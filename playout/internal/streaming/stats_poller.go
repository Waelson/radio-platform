package streaming

import (
	"context"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
)

// statsInterval is the time between listener-count polls for each connected target.
const statsInterval = 30 * time.Second

// pollStatsLoop runs inside Manager.Run() and ticks every statsInterval to
// fetch listener counts from all connected targets.
func (m *Manager) pollStatsLoop(ctx context.Context) {
	ticker := time.NewTicker(statsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.pollAllStats(ctx)
		}
	}
}

// pollAllStats fetches listener counts for every connected target and
// publishes EvtStreamingStats for each successful result.
func (m *Manager) pollAllStats(ctx context.Context) {
	m.mu.RLock()
	type entry struct {
		id  string
		cfg TargetConfig
	}
	var targets []entry
	for id, t := range m.targets {
		if t.IsConnected() {
			targets = append(targets, entry{id: id, cfg: t.cfg})
		}
	}
	m.mu.RUnlock()

	for _, e := range targets {
		go func(e entry) {
			listeners, err := FetchIcecastListeners(ctx, e.cfg)
			if err != nil {
				m.log.Debug("streaming: stats poll failed",
					"target", e.id, "error", err)
				return
			}

			// Update listener count in the target.
			m.mu.RLock()
			t, ok := m.targets[e.id]
			m.mu.RUnlock()
			if !ok {
				return
			}
			t.mu.Lock()
			t.listeners = listeners
			t.mu.Unlock()

			s := t.Status()
			m.evtBus.Publish(events.New(events.EvtStreamingStats,
				events.StreamingStatsPayload{
					TargetID:  e.id,
					Listeners: listeners,
					BytesSent: s.BytesSent,
					UptimeMS:  s.UptimeMS,
				}))
			m.log.Debug("streaming: stats updated",
				"target", e.id, "listeners", listeners)
		}(e)
	}
}
