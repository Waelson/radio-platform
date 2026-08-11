package linein

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

const (
	// bufferFrames is the number of PCM frames per read/write iteration.
	// At 48 kHz stereo this is ~42 ms of audio per loop iteration.
	bufferFrames = 2048

	// pcmChanSize is the capacity of the Go channel that decouples the
	// avfoundation reader goroutine from the ring-buffer writer loop.
	// 64 chunks × ~42 ms = ~2.7 s of headroom for avfoundation delivery
	// stalls (USB packet jitter, system load spikes, etc.) without any
	// audio underrun reaching the CoreAudio ring buffer.
	pcmChanSize = 64

	// preBufferChunks is the number of PCM chunks written to the ring buffer
	// before the AudioQueue is started. With the producer-consumer pattern
	// the Go channel is already full from the avfoundation burst, so the
	// consumer writes these chunks nearly instantly (no perceptible startup
	// delay). After Start() the 3 initial AudioQueue callbacks drain 3 chunks,
	// leaving ~7 chunks (~294 ms) of steady-state headroom that absorbs
	// delivery jitter without causing ring-empty silence callbacks.
	preBufferChunks = 1

	// levelUpdateInterval controls how often EventLevelUpdate is emitted.
	levelUpdateInterval = 500 * time.Millisecond
)

// LineInManager captures audio from an InputDevice and routes it directly to
// an OutputDevice. It also runs a silence watchdog and enforces a duration cap.
//
// Lifecycle:
//
//	manager := NewLineInManager(input, output, log)
//	events := manager.Start(ctx, cfg)   // starts goroutine; returns event channel
//	// ... read events ...
//	manager.Stop()                       // signals goroutine to stop
//	// channel is closed when goroutine exits
type LineInManager struct {
	input    InputDevice
	out      output.OutputDevice
	recorder output.OutputDevice // optional; simultaneous recording (caller owns lifecycle)
	stateMgr *state.Manager      // optional; provides MainVolume for gain control
	log      *slog.Logger
	cancel   context.CancelFunc
	mu       sync.Mutex
	active   bool
	paused   atomic.Bool // when true, frames are zeroed before writing (silence)
}

// NewLineInManager creates a manager with the given devices.
// log may be nil.
func NewLineInManager(input InputDevice, out output.OutputDevice, log *slog.Logger) *LineInManager {
	return &LineInManager{input: input, out: out, log: log}
}

// Start begins line-in capture with the given config.
// It returns a channel that receives Event notifications; the channel is
// closed when the session ends (either via Stop, duration expiry, or error).
// Returns an error immediately if a session is already active.
func (m *LineInManager) Start(ctx context.Context, cfg LineInConfig) (<-chan Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active {
		return nil, fmt.Errorf("linein manager: session already active")
	}

	capCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.active = true

	events := make(chan Event, 16)

	go m.run(capCtx, cfg, events)

	return events, nil
}

// SetRecorder attaches an optional secondary output that receives the same
// PCM frames as the main output (simultaneous recording).
// Must be called before Start. The caller is responsible for closing the
// recorder after the session ends.
func (m *LineInManager) SetRecorder(r output.OutputDevice) {
	m.recorder = r
}

// SetStateManager attaches the state manager so that the main volume level is
// applied to line-in frames before they are written to the output device.
// Must be called before Start.
func (m *LineInManager) SetStateManager(s *state.Manager) {
	m.stateMgr = s
}

// Stop signals the active session to stop.
// It is safe to call Stop even when no session is active.
func (m *LineInManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}

// Pause silences the line-in output without stopping the session.
// The capture device keeps running; frames are replaced with silence so the
// output buffer stays fed and resuming is glitch-free.
func (m *LineInManager) Pause() { m.paused.Store(true) }

// Resume restores live audio output after a Pause.
func (m *LineInManager) Resume() { m.paused.Store(false) }

// run is the main goroutine. It opens the input device, routes PCM to the
// output device, runs the silence watchdog, and emits events.
//
// Architecture (two-stage pipeline):
//
//	[FFmpeg/avfoundation] → producer goroutine → [Go channel, pcmChanSize]
//	                      → consumer loop      → [C ring buffer]
//	                      → CoreAudio pullCallback (always re-enqueues)
//
// The Go channel decouples avfoundation delivery jitter (USB packet bursts,
// system load spikes) from the CoreAudio ring-buffer write rate. The ring
// buffer itself decouples the Go writer from the CoreAudio hardware clock.
// With both buffers combined, audio remains glitch-free even if avfoundation
// stalls for up to ~2.7 s.
func (m *LineInManager) run(ctx context.Context, cfg LineInConfig, events chan<- Event) {
	defer func() {
		m.mu.Lock()
		m.active = false
		m.mu.Unlock()
		close(events)
	}()

	startedAt := time.Now()

	if err := m.input.Open(ctx, cfg); err != nil {
		m.emit(events, Event{Type: EventError, Err: fmt.Errorf("linein: open input: %w", err)})
		return
	}
	defer m.input.Close()

	m.emit(events, Event{Type: EventStarted})
	m.logf("line-in session started", "label", cfg.Label, "device", cfg.DeviceID)

	// ── Producer goroutine ──────────────────────────────────────────────────
	// Reads from FFmpeg as fast as avfoundation delivers (including the
	// startup burst) and forwards chunks to pcmCh. The channel buffer absorbs
	// delivery jitter so the consumer loop never has to wait on avfoundation.
	errCh := make(chan error, 1)
	pcmCh := make(chan []float32, pcmChanSize)
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		defer close(pcmCh)
		for {
			b := make([]float32, bufferFrames*2)
			n, err := m.input.ReadFrames(ctx, b)
			if n > 0 {
				select {
				case pcmCh <- b[:n*2]:
				case <-ctx.Done():
					return
				}
			}
			if err == io.EOF || (err != nil && ctx.Err() != nil) {
				return
			}
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}()

	// Ensure the producer goroutine exits BEFORE m.input.Close() is called.
	// Go defers run LIFO, so registering these two after m.input.Close() means
	// they execute in this order on any return:
	//   1. m.cancel()       — signals the producer to exit via ctx.Done()
	//   2. <-producerDone  — waits for the producer to fully exit
	//   3. m.input.Close() — now safe: no concurrent ReadFrames in progress
	// Without this ordering, duration_ms expiry caused a use-after-free crash:
	// run() returned without cancelling ctx, defer m.input.Close() freed the
	// C session, and the still-running producer called caInputAvail/caInputRead
	// on the freed pointer → SIGSEGV.
	defer func() { <-producerDone }()
	defer m.cancel()

	silenceThresholdLinear := dbfsToLinear(cfg.silenceThresholdDBFS())
	silenceThresholdDur := time.Duration(cfg.silenceThresholdMS()) * time.Millisecond

	var silenceSince *time.Time
	var silenceAlerted bool
	lastLevelUpdate := time.Now()

	queueStarted := false
	preBuffered := 0

	for {
		// Priority stop check: when ctx is cancelled, exit immediately
		// without draining buffered frames from pcmCh. Without this,
		// the select below would randomly pick pcmCh over ctx.Done()
		// for up to ~2.7 s while the channel drains (pcmChanSize = 64
		// chunks × ~42 ms each), causing audible delay after Stop/Pause/Skip.
		select {
		case <-ctx.Done():
			elapsed := time.Since(startedAt)
			m.emit(events, Event{Type: EventStopped, ElapsedMS: elapsed.Milliseconds()})
			return
		default:
		}

		// Duration cap.
		if cfg.DurationMS > 0 {
			elapsed := time.Since(startedAt)
			if elapsed >= time.Duration(cfg.DurationMS)*time.Millisecond {
				m.logf("line-in duration cap reached", "duration_ms", cfg.DurationMS)
				m.emit(events, Event{Type: EventStopped, ElapsedMS: elapsed.Milliseconds()})
				return
			}
		}

		// Wait for the next PCM chunk, an input error, or a stop signal.
		var frames []float32
		select {
		case <-ctx.Done():
			elapsed := time.Since(startedAt)
			m.emit(events, Event{Type: EventStopped, ElapsedMS: elapsed.Milliseconds()})
			return
		case err := <-errCh:
			m.emit(events, Event{Type: EventError, Err: fmt.Errorf("linein: read: %w", err)})
			return
		case f, ok := <-pcmCh:
			if !ok {
				// Producer closed — FFmpeg reached EOF.
				elapsed := time.Since(startedAt)
				m.emit(events, Event{Type: EventStopped, ElapsedMS: elapsed.Milliseconds()})
				return
			}
			frames = f
		}

		// When paused, replace frames with silence so the output buffer stays
		// fed and the device does not underrun. The capture device keeps running
		// so resuming is glitch-free (no startup delay or buffer refill needed).
		if m.paused.Load() {
			for i := range frames {
				frames[i] = 0
			}
		}

		// Apply main volume gain so the line-in level respects the same
		// volume control used by regular playback.
		if m.stateMgr != nil {
			applyGain(frames, m.stateMgr.MainVolume())
		}

		// Write to ring buffer via the Mixer. The Mixer fans out to the
		// health monitor and the streaming tap automatically.
		if _, werr := m.out.Write(ctx, frames); werr != nil {
			m.emit(events, Event{Type: EventError, Err: fmt.Errorf("linein: write output: %w", werr)})
			return
		}

		// Start the AudioQueue only after preBufferChunks chunks are in the
		// ring. The producer-consumer pattern fills the Go channel from the
		// avfoundation burst, so the consumer writes these chunks almost
		// instantly. After Start() the ring has ~7 chunks of headroom
		// (~294 ms) that absorbs delivery jitter without silence callbacks.
		if !queueStarted {
			preBuffered++
			if preBuffered >= preBufferChunks {
				queueStarted = true
				if serr := m.out.Start(ctx); serr != nil {
					m.emit(events, Event{Type: EventError, Err: fmt.Errorf("linein: start output: %w", serr)})
					return
				}
				m.logf("line-in output started", "label", cfg.Label)
			}
		}

		// Write to recorder (non-fatal).
		if m.recorder != nil {
			if _, werr := m.recorder.Write(ctx, frames); werr != nil {
				m.logf("linein: recorder write error", "err", werr)
			}
		}

		// ── Silence watchdog ─────────────────────────────────────────────
		rms := computeRMS(frames)
		if rms < silenceThresholdLinear {
			now := time.Now()
			if silenceSince == nil {
				silenceSince = &now
				silenceAlerted = false
			} else if !silenceAlerted && time.Since(*silenceSince) >= silenceThresholdDur {
				silenceAlerted = true
				silenceDurMS := time.Since(*silenceSince).Milliseconds()
				if cfg.onSilenceOrDefault() == "stop" {
					m.emit(events, Event{
						Type:              EventSilenceStop,
						SilenceDurationMS: silenceDurMS,
						ElapsedMS:         time.Since(startedAt).Milliseconds(),
					})
					return
				}
				m.emit(events, Event{
					Type:              EventSilenceAlert,
					SilenceDurationMS: silenceDurMS,
				})
			}
		} else {
			silenceSince = nil
			silenceAlerted = false
		}

		// ── Level update ─────────────────────────────────────────────────
		if time.Since(lastLevelUpdate) >= levelUpdateInterval {
			lastLevelUpdate = time.Now()
			m.emit(events, Event{
				Type:    EventLevelUpdate,
				RMSDBFS: linearToDBFS(rms),
			})
		}
	}
}

func (m *LineInManager) emit(ch chan<- Event, e Event) {
	select {
	case ch <- e:
	default:
		if e.Type == EventError || e.Type == EventStopped || e.Type == EventSilenceStop {
			ch <- e
		}
	}
}

func (m *LineInManager) logf(msg string, args ...any) {
	if m.log != nil {
		m.log.Info(msg, args...)
	}
}

// ── Audio helpers ─────────────────────────────────────────────────────────────

// applyGain multiplies every sample in buf by gain. Returns immediately when
// gain == 1.0 to keep the hot path allocation-free.
func applyGain(buf []float32, gain float32) {
	if gain == 1.0 {
		return
	}
	for i := range buf {
		buf[i] *= gain
	}
}

func computeRMS(frames []float32) float64 {
	if len(frames) == 0 {
		return 0
	}
	var sum float64
	for _, s := range frames {
		v := float64(s)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(frames)))
}

func linearToDBFS(linear float64) float64 {
	if linear <= 0 {
		return -144
	}
	return 20 * math.Log10(linear)
}

func dbfsToLinear(dbfs float64) float64 {
	if dbfs <= -144 {
		return 0
	}
	return math.Pow(10, dbfs/20)
}
