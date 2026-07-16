package streaming

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Waelson/radio-playout-engine/internal/events"
)


// Manager fans out PCM audio from the playback loop to one or more streaming
// targets (Icecast/SHOUTcast). It is safe for concurrent use.
type Manager struct {
	mu      sync.RWMutex
	targets map[string]*Target

	// reconnecting tracks which target IDs have an active reconnect loop.
	// Protected by mu.
	reconnecting map[string]bool

	// runCtx is set when Run() is called; used by reconnect goroutines.
	// Protected by mu.
	runCtx context.Context

	// audioIn receives the mixed PCM stream from the MixBus.
	// Set once via SetAudioIn before Run is called.
	audioIn <-chan []float32
	evtBus  *events.Bus
	log     *slog.Logger
}

// NewManager creates a Manager. evtBus and log may not be nil.
func NewManager(evtBus *events.Bus, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		targets:      make(map[string]*Target),
		reconnecting: make(map[string]bool),
		evtBus:       evtBus,
		log:          log,
	}
}

// SetAudioIn sets the channel from which fanOut reads mixed PCM frames.
// Must be called before Run.
func (m *Manager) SetAudioIn(ch <-chan []float32) { m.audioIn = ch }

// Run starts the fanOut goroutine, subscribes to NowPlayingChanged events
// for metadata updates, and blocks until ctx is cancelled.
// On shutdown it disconnects all targets gracefully.
// Call this in its own goroutine.
func (m *Manager) Run(ctx context.Context) {
	// Store the context so reconnect goroutines can use it.
	m.mu.Lock()
	m.runCtx = ctx
	m.mu.Unlock()

	// Subscribe to engine events before starting the fan-out goroutine so no
	// NowPlayingChanged event is missed between Run() and the first select.
	evtCh, cancelSub := m.evtBus.Subscribe(32)
	defer cancelSub()

	fanDone := make(chan struct{})
	go func() {
		defer close(fanDone)
		m.fanOut(ctx)
	}()

	statsDone := make(chan struct{})
	go func() {
		defer close(statsDone)
		m.pollStatsLoop(ctx)
	}()

	for {
		select {
		case <-ctx.Done():
			<-fanDone
			<-statsDone
			m.mu.Lock()
			for _, t := range m.targets {
				t.Disconnect()
			}
			m.mu.Unlock()
			return
		case evt := <-evtCh:
			if evt.Type == events.EvtNowPlayingChanged {
				if p, ok := evt.Payload.(events.NowPlayingChangedPayload); ok {
					// Run in a goroutine — HTTP requests must not stall the event loop.
					go m.updateAllMetadata(ctx, p.Title, p.Artist)
				}
			}
		}
	}
}

// updateAllMetadata sends title/artist to every connected target that has
// SendMetadata enabled, and publishes EvtStreamingMetadataUpdated on success.
func (m *Manager) updateAllMetadata(ctx context.Context, title, artist string) {
	m.mu.RLock()
	type entry struct {
		id  string
		cfg TargetConfig
	}
	var targets []entry
	for id, t := range m.targets {
		if t.IsConnected() && t.cfg.SendMetadata {
			targets = append(targets, entry{id: id, cfg: t.cfg})
		}
	}
	m.mu.RUnlock()

	for _, e := range targets {
		if err := UpdateMetadata(ctx, e.cfg, title, artist); err != nil {
			m.log.Warn("streaming: metadata update failed",
				"target", e.id, "error", err)
			continue
		}
		m.evtBus.Publish(events.New(events.EvtStreamingMetadataUpdated,
			events.StreamingMetadataUpdatedPayload{
				TargetID: e.id,
				Title:    title,
				Artist:   artist,
			}))
		m.log.Debug("streaming: metadata updated", "target", e.id,
			"title", title, "artist", artist)
	}
}

// fanOut reads frame slices from tapCh and writes them to every connected
// target. Target.Write is non-blocking and copies the slice internally, so
// passing the same slice to multiple targets is safe.
//
// Silence keepalive: Icecast/SHOUTcast servers close source connections that
// send no data for several seconds. When the playout engine is idle (no music
// playing), fanOut generates silence frames at 48 kHz stereo to keep every
// connected target's FFmpeg process alive. Silence stops immediately when real
// audio resumes.
func (m *Manager) fanOut(ctx context.Context) {
	send := func(frames []float32) {
		m.mu.RLock()
		for _, t := range m.targets {
			t.Write(frames)
		}
		m.mu.RUnlock()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case frames, ok := <-m.audioIn:
			if !ok {
				return
			}
			send(frames)
		}
	}
}


// AddTarget connects a new streaming target. If a target with the same ID
// already exists an error is returned.
func (m *Manager) AddTarget(ctx context.Context, cfg TargetConfig) error {
	m.mu.Lock()
	if _, exists := m.targets[cfg.ID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("streaming: target %q already exists", cfg.ID)
	}
	t := NewTarget(cfg, m.log)
	t.SetOnDisconnect(func(id, reason string) {
		m.log.Warn("streaming: target disconnected unexpectedly", "target", id, "reason", reason)
		// Reconnect handles its own disconnect events; only publish here when
		// reconnect is disabled so the client still receives the event.
		if !cfg.Reconnect.Enabled {
			m.evtBus.Publish(events.New(events.EvtStreamingDisconnected, events.StreamingDisconnectedPayload{
				TargetID: id,
				Reason:   reason,
			}))
		}
		m.scheduleReconnect(id, cfg)
	})
	m.targets[cfg.ID] = t
	m.mu.Unlock()

	if err := t.Connect(ctx); err != nil {
		m.mu.Lock()
		delete(m.targets, cfg.ID)
		m.mu.Unlock()
		return fmt.Errorf("streaming: connect %q: %w", cfg.ID, err)
	}

	m.evtBus.Publish(events.New(events.EvtStreamingConnected, events.StreamingConnectedPayload{
		TargetID:    cfg.ID,
		Name:        cfg.Name,
		Host:        cfg.Host,
		Mount:       cfg.Mount,
		Format:      cfg.Format,
		BitrateKbps: cfg.BitrateKbps,
	}))
	m.log.Info("streaming: target connected", "target", cfg.ID, "host", cfg.Host, "format", cfg.Format)
	return nil
}

// RemoveTarget disconnects and removes a target by ID.
// If the target does not exist the call is a no-op.
func (m *Manager) RemoveTarget(id string) {
	m.mu.Lock()
	t, ok := m.targets[id]
	if ok {
		delete(m.targets, id)
	}
	m.mu.Unlock()

	if !ok {
		return
	}
	t.Disconnect()
	m.evtBus.Publish(events.New(events.EvtStreamingDisconnected, events.StreamingDisconnectedPayload{
		TargetID: id,
		Reason:   "removed",
	}))
	m.log.Info("streaming: target removed", "target", id)
}

// ListStatuses returns a snapshot of the status of all registered targets.
func (m *Manager) ListStatuses() []TargetStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]TargetStatus, 0, len(m.targets))
	for _, t := range m.targets {
		out = append(out, t.Status())
	}
	return out
}

// Status returns the status of a single target or an error if not found.
func (m *Manager) Status(id string) (TargetStatus, error) {
	m.mu.RLock()
	t, ok := m.targets[id]
	m.mu.RUnlock()
	if !ok {
		return TargetStatus{}, fmt.Errorf("streaming: target %q not found", id)
	}
	return t.Status(), nil
}
