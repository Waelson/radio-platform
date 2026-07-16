// Package mixbus implements a software mixing bus for streaming audio.
//
// It aggregates PCM frames from multiple independent audio sources (main
// playback, cart player) into a single continuous, fixed-rate output stream
// suitable for the streaming manager / FFmpeg targets.
//
// Each source writes frames at its own hardware clock rate into a dedicated
// ring buffer. A fixed 20 ms ticker reads exactly 1920 samples (48 kHz ×
// 0.020 s × 2 channels) from every ring buffer, sums them, and publishes
// the mixed frame to the output channel. When a ring buffer is empty, silence
// (zeros) is used for that source — this also replaces the silence keepalive
// that was previously handled inside the streaming fanOut loop.
package mixbus

import (
	"context"
	"sync"
	"time"
)

const (
	sampleRate   = 48000
	channels     = 2
	tickInterval = 20 * time.Millisecond
	// tickSamples: samples per tick = 48000 * 2 * 0.020 = 1920
	tickSamples = sampleRate * channels * int(tickInterval/time.Millisecond) / 1000
	// ringCap: 2 seconds of stereo float32 per source
	ringCap = sampleRate * channels * 2
)

// ── ring buffer ───────────────────────────────────────────────────────────────

type ringBuf struct {
	mu   sync.Mutex
	data []float32
	r, w int
	n    int // number of samples currently held
}

func newRingBuf() *ringBuf {
	return &ringBuf{data: make([]float32, ringCap)}
}

// write appends frames to the ring buffer. If the buffer would overflow, the
// oldest samples are silently discarded to make room — keeping the stream live
// at the cost of a brief glitch (preferable to blocking the audio loop).
func (rb *ringBuf) write(frames []float32) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for _, s := range frames {
		if rb.n == ringCap {
			// Overflow: advance read pointer to discard oldest sample.
			rb.r = (rb.r + 1) % ringCap
			rb.n--
		}
		rb.data[rb.w] = s
		rb.w = (rb.w + 1) % ringCap
		rb.n++
	}
}

// read fills out with exactly len(out) samples. Missing samples are zero-filled
// (silence). Returns the number of real samples consumed (may be < len(out)).
func (rb *ringBuf) read(out []float32) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for i := range out {
		if rb.n == 0 {
			out[i] = 0
			continue
		}
		out[i] = rb.data[rb.r]
		rb.r = (rb.r + 1) % ringCap
		rb.n--
	}
}

// ── MixBus ────────────────────────────────────────────────────────────────────

// MixBus aggregates audio from multiple sources and outputs a single mixed
// stream at a fixed 20 ms rate.
type MixBus struct {
	mainRing *ringBuf
	cartRing *ringBuf

	mainIn chan []float32 // playback manager writes here
	cartIn chan []float32 // cart player writes here
	outCh  chan []float32 // streaming manager reads here
}

// New creates a MixBus ready to Run.
func New() *MixBus {
	return &MixBus{
		mainRing: newRingBuf(),
		cartRing: newRingBuf(),
		mainIn:   make(chan []float32, 64),
		cartIn:   make(chan []float32, 64),
		outCh:    make(chan []float32, 32),
	}
}

// MainIn returns the write-only channel for the main playback source.
// Pass this to playback.Manager.SetStreamingTap.
func (m *MixBus) MainIn() chan<- []float32 { return m.mainIn }

// CartIn returns the write-only channel for the cart player source.
// Pass this to cart.Player.SetStreamingTap.
func (m *MixBus) CartIn() chan<- []float32 { return m.cartIn }

// OutCh returns the read-only channel of mixed frames consumed by the
// streaming manager's fanOut loop.
func (m *MixBus) OutCh() <-chan []float32 { return m.outCh }

// Run starts the mixing loop. It must be called in a dedicated goroutine and
// returns when ctx is cancelled.
func (m *MixBus) Run(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	mainBuf := make([]float32, tickSamples)
	cartBuf := make([]float32, tickSamples)
	mixed := make([]float32, tickSamples)

	drainInputs := func() {
		for {
			select {
			case frames := <-m.mainIn:
				m.mainRing.write(frames)
			case frames := <-m.cartIn:
				m.cartRing.write(frames)
			default:
				return
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return

		// Accept frames between ticks to keep ring buffers fed.
		case frames := <-m.mainIn:
			m.mainRing.write(frames)
		case frames := <-m.cartIn:
			m.cartRing.write(frames)

		case <-ticker.C:
			// Drain any frames that arrived since the last tick.
			drainInputs()

			// Read one tick-worth from each source (silence-padded if empty).
			m.mainRing.read(mainBuf)
			m.cartRing.read(cartBuf)

			// Mix: sum and soft-clip to [-1, 1].
			for i := range mixed {
				v := mainBuf[i] + cartBuf[i]
				if v > 1.0 {
					v = 1.0
				} else if v < -1.0 {
					v = -1.0
				}
				mixed[i] = v
			}

			// Publish mixed frame — copy so the next tick can reuse mixed[].
			cp := make([]float32, tickSamples)
			copy(cp, mixed)
			select {
			case m.outCh <- cp:
			default:
				// Streaming manager is slow — drop frame to protect this loop.
			}
		}
	}
}
