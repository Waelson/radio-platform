package linein

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
)

const (
	// bufferFrames is the number of PCM frames per read/write iteration.
	// At 48 kHz stereo this is ~42 ms of audio per loop iteration.
	bufferFrames = 2048

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
	input  InputDevice
	out    output.OutputDevice
	log    *slog.Logger
	cancel context.CancelFunc
	mu     sync.Mutex
	active bool
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

// Stop signals the active session to stop.
// It is safe to call Stop even when no session is active.
func (m *LineInManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}

// run is the main goroutine. It opens the input device, routes PCM to the
// output device, runs the silence watchdog, and emits events.
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

	buf := make([]float32, bufferFrames*2) // stereo

	silenceThresholdLinear := dbfsToLinear(cfg.silenceThresholdDBFS())
	silenceThresholdDur := time.Duration(cfg.silenceThresholdMS()) * time.Millisecond

	var silenceSince *time.Time
	var silenceAlerted bool

	lastLevelUpdate := time.Now()

	for {
		// Duration cap: stop when configured duration has elapsed.
		if cfg.DurationMS > 0 {
			elapsed := time.Since(startedAt)
			if elapsed >= time.Duration(cfg.DurationMS)*time.Millisecond {
				m.logf("line-in duration cap reached", "duration_ms", cfg.DurationMS)
				m.emit(events, Event{
					Type:      EventStopped,
					ElapsedMS: elapsed.Milliseconds(),
				})
				return
			}
		}

		// Check for external stop signal.
		select {
		case <-ctx.Done():
			elapsed := time.Since(startedAt)
			m.emit(events, Event{Type: EventStopped, ElapsedMS: elapsed.Milliseconds()})
			return
		default:
		}

		n, err := m.input.ReadFrames(ctx, buf)
		if err == io.EOF || (err != nil && ctx.Err() != nil) {
			elapsed := time.Since(startedAt)
			m.emit(events, Event{Type: EventStopped, ElapsedMS: elapsed.Milliseconds()})
			return
		}
		if err != nil {
			m.emit(events, Event{Type: EventError, Err: fmt.Errorf("linein: read: %w", err)})
			return
		}
		if n == 0 {
			continue
		}

		frames := buf[:n*2]

		// Write to output device.
		if _, werr := m.out.Write(ctx, frames); werr != nil {
			m.emit(events, Event{Type: EventError, Err: fmt.Errorf("linein: write output: %w", werr)})
			return
		}

		// ── Silence watchdog ──────────────────────────────────────────────
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

		// ── Level update ──────────────────────────────────────────────────
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
		// Drop non-critical level events if the channel is full.
		// Critical events (error, stopped) are always retried via a blocking send.
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

// computeRMS returns the root mean square amplitude of the frame buffer.
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

// linearToDBFS converts a linear amplitude to dBFS.
// Returns -144 for zero input (treated as silence floor).
func linearToDBFS(linear float64) float64 {
	if linear <= 0 {
		return -144
	}
	return 20 * math.Log10(linear)
}

// dbfsToLinear converts a dBFS value to linear amplitude.
func dbfsToLinear(dbfs float64) float64 {
	if dbfs <= -144 {
		return 0
	}
	return math.Pow(10, dbfs/20)
}
