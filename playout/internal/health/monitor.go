// Package health computes audio health metrics from raw PCM data and
// publishes AudioHealthChanged events on the Event Bus. It also detects
// sustained silence and raises/clears AlertRaised/AlertCleared events.
//
// Dependency direction:
//
//	health → events, state
//
// Playback and API packages must not be imported here.
package health

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
	"github.com/oklog/ulid/v2"
)

// windowDurationMS is the size of each RMS/peak measurement window.
const windowDurationMS = 50

// Config holds audio health monitoring parameters.
type Config struct {
	// IntervalMS controls how often AudioHealthChanged events are published.
	// Default: 500.
	IntervalMS int
	// SilenceThresholdDBFS is the level (in dBFS) below which audio is
	// considered silent. Default: -60.
	SilenceThresholdDBFS float64
	// SilenceDurationMS is the minimum duration of silence before
	// AlertRaised fires. Default: 2000.
	SilenceDurationMS int
	// SampleRate is the audio sample rate in Hz. Default: 48000.
	SampleRate int
	// Channels is the number of audio channels. Default: 2.
	Channels int

	// AutoPanicSilenceDurationMS is the silence duration in ms after which
	// OnAutoPanic is called. 0 disables auto-panic. Must be >= SilenceDurationMS
	// for auto-panic to trigger after the alert.
	AutoPanicSilenceDurationMS int
	// OnAutoPanic is called once per silence event when silence exceeds
	// AutoPanicSilenceDurationMS. It is reset when audio resumes.
	// The callback is invoked from the Run goroutine; implementations must be
	// non-blocking (e.g. a non-blocking channel send or goroutine dispatch).
	OnAutoPanic func(reason string)

	// VUMeterEnabled enables the advanced VU meter (EvtVUMeter events).
	VUMeterEnabled bool
	// VUMeterIntervalMS controls how often EvtVUMeter is published. Default: 100.
	VUMeterIntervalMS int
	// PeakHoldMS is how long (ms) the peak hold value is retained. Default: 3000.
	PeakHoldMS int
}

func (c *Config) setDefaults() {
	if c.IntervalMS <= 0 {
		c.IntervalMS = 500
	}
	if c.SilenceThresholdDBFS == 0 {
		c.SilenceThresholdDBFS = -60
	}
	if c.SilenceDurationMS <= 0 {
		c.SilenceDurationMS = 2000
	}
	if c.SampleRate <= 0 {
		c.SampleRate = 48000
	}
	if c.Channels <= 0 {
		c.Channels = 2
	}
}

// Monitor computes audio health metrics from raw PCM data.
// Push is designed for the hot audio path and avoids allocation.
type Monitor struct {
	cfg      Config
	evtBus   *events.Bus
	stateMgr *state.Manager
	log      *slog.Logger

	// windowSamples is the number of PCM samples in a 50 ms window.
	windowSamples int

	// Window accumulator. Guarded by mu; only Push writes these fields.
	mu          sync.Mutex
	sumSq       float64
	peak        float64
	sampleCount int

	// latest holds the most recently completed window result.
	// Written by Push (audio goroutine), read by Run (timer goroutine).
	latest atomic.Value // *windowResult

	// underrunCount is incremented by the output device on buffer underruns.
	underrunCount atomic.Int64

	// silenceAlertID is non-empty when an AlertRaised has been published.
	// Only accessed from the Run goroutine so no mutex is needed.
	silenceAlertID string

	// vu holds the advanced VU meter state. nil when VUMeterEnabled is false.
	vu *vuState
}

// windowResult is an immutable snapshot of a completed measurement window.
type windowResult struct {
	levelDBFS float64
	peakDBFS  float64
}

// NewMonitor creates a Monitor. evtBus and stateMgr may be nil (useful in tests).
func NewMonitor(cfg Config, evtBus *events.Bus, stateMgr *state.Manager, log *slog.Logger) *Monitor {
	cfg.setDefaults()
	if log == nil {
		log = slog.Default()
	}
	m := &Monitor{
		cfg:           cfg,
		evtBus:        evtBus,
		stateMgr:      stateMgr,
		log:           log,
		windowSamples: cfg.SampleRate * windowDurationMS / 1000 * cfg.Channels,
	}
	if cfg.VUMeterEnabled {
		peakHoldMS := cfg.PeakHoldMS
		if peakHoldMS <= 0 {
			peakHoldMS = 3000
		}
		m.vu = newVUState(cfg.Channels, cfg.SampleRate, peakHoldMS)
	}
	return m
}

// Push ingests PCM samples from the audio pipeline. It is safe to call from
// the hot path: it holds a mutex only for scalar arithmetic, never allocates,
// and never blocks on I/O.
func (m *Monitor) Push(samples []float32) {
	m.mu.Lock()
	for _, s := range samples {
		v := float64(s)
		m.sumSq += v * v
		if v < 0 {
			v = -v
		}
		if v > m.peak {
			m.peak = v
		}
		m.sampleCount++
		if m.sampleCount >= m.windowSamples {
			// Window complete — compute level and hand off atomically.
			rms := math.Sqrt(m.sumSq / float64(m.sampleCount))
			peak := m.peak
			m.sumSq = 0
			m.peak = 0
			m.sampleCount = 0
			m.mu.Unlock()
			m.latest.Store(&windowResult{
				levelDBFS: linearToDBFS(rms),
				peakDBFS:  linearToDBFS(peak),
			})
			m.mu.Lock()
		}
	}
	m.mu.Unlock()
	// VU meter: runs independently with its own lock, outside m.mu.
	if m.vu != nil {
		m.vu.push(samples)
	}
}

// IncUnderrun records one output buffer underrun. Thread-safe.
func (m *Monitor) IncUnderrun() {
	m.underrunCount.Add(1)
}

// Run starts the health-reporting loop. It blocks until ctx is cancelled.
// Typically run in its own goroutine:
//
//	go mon.Run(ctx)
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.cfg.IntervalMS) * time.Millisecond)
	defer ticker.Stop()

	// VU meter ticker — nil channel is never selected, so no guard needed.
	var vuTickerC <-chan time.Time
	if m.vu != nil {
		vuIntervalMS := m.cfg.VUMeterIntervalMS
		if vuIntervalMS <= 0 {
			vuIntervalMS = 100
		}
		vuTicker := time.NewTicker(time.Duration(vuIntervalMS) * time.Millisecond)
		defer vuTicker.Stop()
		vuTickerC = vuTicker.C
	}

	var silenceStart time.Time
	var inSilence bool
	var autoPanicFired bool

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			m.report(now, &silenceStart, &inSilence, &autoPanicFired)
		case <-vuTickerC:
			if m.evtBus != nil {
				m.evtBus.Publish(events.New(events.EvtVUMeter, m.vu.snapshot()))
			}
		}
	}
}

// report is called by Run on each tick. All pointer parameters are owned
// exclusively by the Run goroutine — no locking is needed for them.
func (m *Monitor) report(now time.Time, silenceStart *time.Time, inSilence *bool, autoPanicFired *bool) {
	wr, _ := m.latest.Load().(*windowResult)
	if wr == nil {
		// No audio has been pushed yet; nothing meaningful to report.
		return
	}

	isSilent := wr.levelDBFS < m.cfg.SilenceThresholdDBFS
	silenceMS := int64(0)

	if isSilent {
		if !*inSilence {
			*inSilence = true
			*silenceStart = now
		}
		silenceMS = now.Sub(*silenceStart).Milliseconds()

		// Raise alert when silence has persisted long enough.
		if silenceMS >= int64(m.cfg.SilenceDurationMS) && m.silenceAlertID == "" {
			m.silenceAlertID = "alert_" + ulid.Make().String()
			if m.evtBus != nil {
				m.evtBus.Publish(events.New(events.EvtAlertRaised, events.AlertRaisedPayload{
					AlertID:  m.silenceAlertID,
					Severity: "WARNING",
					Source:   "audio_health",
					Message:  fmt.Sprintf("silence detected for %dms", silenceMS),
				}))
			}
			m.log.Warn("silence alert raised", "duration_ms", silenceMS)
		}

		// Trigger auto-panic once per silence event when the extended threshold
		// is exceeded. The callback is non-blocking by contract.
		if m.cfg.AutoPanicSilenceDurationMS > 0 &&
			!*autoPanicFired &&
			silenceMS >= int64(m.cfg.AutoPanicSilenceDurationMS) &&
			m.cfg.OnAutoPanic != nil {
			*autoPanicFired = true
			m.log.Warn("auto-panic triggered by silence", "duration_ms", silenceMS)
			m.cfg.OnAutoPanic(fmt.Sprintf("sustained silence detected for %dms", silenceMS))
		}
	} else if *inSilence {
		// Silence has ended — clear the active alert if one was raised.
		*inSilence = false
		*silenceStart = time.Time{}
		*autoPanicFired = false
		if m.silenceAlertID != "" {
			if m.evtBus != nil {
				m.evtBus.Publish(events.New(events.EvtAlertCleared, events.AlertClearedPayload{
					AlertID: m.silenceAlertID,
				}))
			}
			m.log.Info("silence alert cleared")
			m.silenceAlertID = ""
		}
	}

	payload := events.AudioHealthChangedPayload{
		LevelDBFS:         wr.levelDBFS,
		PeakDBFS:          wr.peakDBFS,
		Silence:           *inSilence && silenceMS >= int64(m.cfg.SilenceDurationMS),
		SilenceDurationMS: silenceMS,
		BufferPct:         0, // populated by output adapter in future phases
		UnderrunCount:     m.underrunCount.Load(),
	}

	if m.evtBus != nil {
		m.evtBus.Publish(events.New(events.EvtAudioHealthChanged, payload))
	}
	if m.stateMgr != nil {
		m.stateMgr.UpdateAudioHealth(state.AudioHealth{
			LevelDBFS:     wr.levelDBFS,
			PeakDBFS:      wr.peakDBFS,
			Silence:       payload.Silence,
			BufferPct:     0,
			UnderrunCount: m.underrunCount.Load(),
		})
	}
}

// linearToDBFS converts a linear amplitude in [0.0, 1.0] to dBFS.
// Returns -120 for near-zero values to avoid log(0).
func linearToDBFS(linear float64) float64 {
	if linear < 1e-10 {
		return -120
	}
	return 20 * math.Log10(linear)
}
