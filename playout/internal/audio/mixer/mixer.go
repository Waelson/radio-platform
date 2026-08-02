// Package mixer provides a transparent audio mixing stage that sits between
// audio sources (playback, line-in) and the underlying OutputDevice.
//
// The Mixer implements output.OutputDevice so it is a drop-in replacement
// everywhere an OutputDevice is used. Its Write() method fans out to three
// destinations in a single call:
//
//  1. The real hardware OutputDevice.
//  2. The audio health monitor (VU meter, silence detection).
//  3. The streaming tap channel (Icecast/SHOUTcast).
//
// Lifecycle
//
// The Mixer keeps the underlying device open across sessions. Open() and
// Start() are idempotent — only the first call activates the device; all
// subsequent calls are no-ops. Stop() and Close() are also no-ops so that
// individual session managers cannot accidentally silence the hardware between
// sessions. Use Shutdown() for the final, real close at engine exit.
//
// Pause / Resume / Restart
//
// The Mixer implements the optional PauseAudio, ResumeAudio and RestartAudio
// methods used by the playback manager. It delegates them directly to the
// underlying device (when the device supports them).
package mixer

import (
	"context"
	"sync"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/health"
)

// Mixer wraps an OutputDevice and fans out every Write to the health monitor
// and the streaming tap. It also provides idempotent Open/Start and no-op
// Stop/Close so the hardware stays active across session boundaries.
type Mixer struct {
	out output.OutputDevice

	mu        sync.Mutex
	opened    bool
	started   bool
	lastCfg   output.OutputConfig

	healthMon *health.Monitor
	streamTap chan<- []float32
}

// New creates a Mixer wrapping the given OutputDevice.
func New(out output.OutputDevice) *Mixer {
	return &Mixer{out: out}
}

// SetHealthMonitor attaches the audio health monitor. Frames written via
// Write() are pushed to the monitor so VU meter and health events reflect all
// audio sources routed through the Mixer.
func (m *Mixer) SetHealthMonitor(mon *health.Monitor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthMon = mon
}

// SetStreamingTap sets the channel that receives a non-blocking copy of each
// PCM frame slice written through the Mixer. Used to feed the streaming
// pipeline (Icecast/SHOUTcast) with audio from all sources.
func (m *Mixer) SetStreamingTap(ch chan<- []float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamTap = ch
}

// ── output.OutputDevice implementation ───────────────────────────────────────

// Open opens the underlying device if not already open. Subsequent calls with
// different configs are ignored — the device keeps the first config. Returns
// the underlying device error only on the first call.
func (m *Mixer) Open(ctx context.Context, cfg output.OutputConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.opened {
		return nil
	}
	if err := m.out.Open(ctx, cfg); err != nil {
		return err
	}
	m.lastCfg = cfg
	m.opened = true
	return nil
}

// Start starts the underlying device if not already started. Subsequent calls
// are no-ops.
func (m *Mixer) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}
	if err := m.out.Start(ctx); err != nil {
		return err
	}
	m.started = true
	return nil
}

// Write writes frames to the underlying device and fans out to the health
// monitor and the streaming tap. The fan-out happens after the hardware write
// so the health and streaming data reflects the actual output signal.
func (m *Mixer) Write(ctx context.Context, frames []float32) (int, error) {
	n, err := m.out.Write(ctx, frames)

	m.mu.Lock()
	mon := m.healthMon
	tap := m.streamTap
	m.mu.Unlock()

	if mon != nil {
		mon.Push(frames)
	}
	if tap != nil {
		cp := make([]float32, len(frames))
		copy(cp, frames)
		select {
		case tap <- cp:
		default:
		}
	}

	return n, err
}

// Stop is a no-op. The Mixer keeps the device running between sessions so
// silence is output while no source is active (ring buffer drains to zeros).
// Use Shutdown to stop for real.
func (m *Mixer) Stop(_ context.Context) error { return nil }

// Close is a no-op. See Stop.
func (m *Mixer) Close() error { return nil }

// Info delegates to the underlying device.
func (m *Mixer) Info() output.OutputDeviceInfo { return m.out.Info() }

// Shutdown performs the real Stop + Close on the underlying device. Call this
// once at engine exit, after all sessions have ended.
func (m *Mixer) Shutdown() error {
	m.mu.Lock()
	wasStarted := m.started
	wasOpened := m.opened
	m.started = false
	m.opened = false
	m.mu.Unlock()

	if wasStarted {
		_ = m.out.Stop(context.Background())
	}
	if wasOpened {
		return m.out.Close()
	}
	return nil
}

// ── Optional device capabilities — delegated to underlying device ─────────────

// PauseAudio pauses the hardware AudioQueue (if supported).
// Called by the playback manager via type assertion (outputPauser).
func (m *Mixer) PauseAudio() error {
	type pauser interface{ PauseAudio() error }
	if p, ok := m.out.(pauser); ok {
		return p.PauseAudio()
	}
	return nil
}

// ResumeAudio resumes the hardware AudioQueue (if supported).
// Called by the playback manager via type assertion (outputResumer).
func (m *Mixer) ResumeAudio() error {
	type resumer interface{ ResumeAudio() error }
	if r, ok := m.out.(resumer); ok {
		return r.ResumeAudio()
	}
	return nil
}

// RestartAudio performs a hardware stop+start cycle (if supported).
// Called by the playback manager after ASSIST mode wait via type assertion
// (outputRestarter).
func (m *Mixer) RestartAudio() error {
	type restarter interface{ RestartAudio() error }
	if r, ok := m.out.(restarter); ok {
		return r.RestartAudio()
	}
	return nil
}

// FlushAudio drains the hardware ring buffer immediately (if supported).
// Called after a session ends to eliminate the tail of pre-buffered audio
// that would otherwise continue playing for up to ~2.7 s.
func (m *Mixer) FlushAudio() error {
	type flusher interface{ FlushAudio() error }
	if f, ok := m.out.(flusher); ok {
		return f.FlushAudio()
	}
	return nil
}

// RingOccupancy returns the number of float32 samples currently buffered in the
// underlying hardware ring (written by Go but not yet played by CoreAudio).
// Returns 0 if the device does not support this query.
func (m *Mixer) RingOccupancy() int64 {
	type occupancyReader interface{ RingOccupancy() int64 }
	if r, ok := m.out.(occupancyReader); ok {
		return r.RingOccupancy()
	}
	return 0
}

// ListDevices delegates to the underlying device (if it implements DeviceLister).
func (m *Mixer) ListDevices() ([]output.DeviceInfo, error) {
	if lister, ok := m.out.(output.DeviceLister); ok {
		return lister.ListDevices()
	}
	return nil, nil
}
