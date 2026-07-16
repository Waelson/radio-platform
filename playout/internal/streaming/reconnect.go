package streaming

import (
	"context"
	"math"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
)

// calcBackoff returns the exponential backoff delay for a given retry number
// (1-based). The result is clamped to [initial, MaxDelaySec].
// If InitialDelaySec ≤ 0 it defaults to 2 s; MaxDelaySec ≤ 0 defaults to 60 s;
// BackoffMultiplier ≤ 1 defaults to 2.0.
func calcBackoff(rc ReconnectConfig, retryCount int) time.Duration {
	initial := time.Duration(rc.InitialDelaySec) * time.Second
	if initial <= 0 {
		initial = 2 * time.Second
	}
	max := time.Duration(rc.MaxDelaySec) * time.Second
	if max <= 0 {
		max = 60 * time.Second
	}
	mult := rc.BackoffMultiplier
	if mult <= 1.0 {
		mult = 2.0
	}
	d := time.Duration(float64(initial) * math.Pow(mult, float64(retryCount-1)))
	if d > max {
		return max
	}
	return d
}

// scheduleReconnect starts a background goroutine that reconnects the target
// identified by id using exponential backoff. It is a no-op when:
//   - reconnect is disabled in the target config
//   - a reconnect loop is already running for that target
//   - Run has not yet been called (runCtx is nil)
func (m *Manager) scheduleReconnect(id string, cfg TargetConfig) {
	if !cfg.Reconnect.Enabled {
		return
	}

	m.mu.Lock()
	if m.reconnecting[id] {
		m.mu.Unlock()
		return
	}
	m.reconnecting[id] = true
	ctx := m.runCtx
	m.mu.Unlock()

	if ctx == nil {
		m.mu.Lock()
		delete(m.reconnecting, id)
		m.mu.Unlock()
		return
	}

	go m.reconnectLoop(ctx, id, cfg)
}

// reconnectLoop retries Connect with exponential backoff until the target
// reconnects, the manager context is cancelled, the target is removed, or
// MaxRetries is exhausted.
func (m *Manager) reconnectLoop(ctx context.Context, id string, cfg TargetConfig) {
	defer func() {
		m.mu.Lock()
		delete(m.reconnecting, id)
		m.mu.Unlock()
	}()

	rc := cfg.Reconnect

	for retryCount := 1; ; retryCount++ {
		if rc.MaxRetries > 0 && retryCount > rc.MaxRetries {
			m.log.Error("streaming: max reconnect retries exhausted",
				"target", id, "retries", retryCount-1)
			m.evtBus.Publish(events.New(events.EvtStreamingError, events.StreamingErrorPayload{
				TargetID:   id,
				Error:      "max reconnect retries exhausted",
				RetryCount: retryCount - 1,
			}))
			m.mu.RLock()
			t, ok := m.targets[id]
			m.mu.RUnlock()
			if ok {
				t.mu.Lock()
				t.state = StateError
				t.lastError = "max reconnect retries exhausted"
				t.nextRetryAt = nil
				t.mu.Unlock()
			}
			return
		}

		delay := calcBackoff(rc, retryCount)
		nextAt := time.Now().Add(delay)

		// Update target state to reconnecting.
		m.mu.RLock()
		t, ok := m.targets[id]
		m.mu.RUnlock()
		if !ok {
			return // target was removed while we were waiting
		}
		t.mu.Lock()
		t.state = StateReconnecting
		t.retryCount = retryCount
		t.nextRetryAt = &nextAt
		t.mu.Unlock()

		// Publish disconnect event with countdown so clients can show a timer.
		m.evtBus.Publish(events.New(events.EvtStreamingDisconnected, events.StreamingDisconnectedPayload{
			TargetID:  id,
			Reason:    "reconnecting",
			RetryInMS: delay.Milliseconds(),
		}))
		m.log.Info("streaming: waiting before reconnect",
			"target", id, "retry", retryCount, "delay", delay)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		// Recheck after the wait — target may have been removed.
		m.mu.RLock()
		_, ok = m.targets[id]
		m.mu.RUnlock()
		if !ok {
			return
		}

		if err := t.Connect(ctx); err != nil {
			m.log.Warn("streaming: reconnect attempt failed",
				"target", id, "retry", retryCount, "error", err)
			continue
		}

		// Reconnected successfully — reset retry state.
		t.mu.Lock()
		t.retryCount = 0
		t.nextRetryAt = nil
		t.mu.Unlock()

		m.log.Info("streaming: reconnected successfully", "target", id, "retry", retryCount)
		m.evtBus.Publish(events.New(events.EvtStreamingConnected, events.StreamingConnectedPayload{
			TargetID:    id,
			Name:        cfg.Name,
			Host:        cfg.Host,
			Mount:       cfg.Mount,
			Format:      cfg.Format,
			BitrateKbps: cfg.BitrateKbps,
		}))
		return
	}
}

// ExportCalcBackoff is exported for testing only.
// Returns the backoff duration for the given retry count (1-based).
func ExportCalcBackoff(rc ReconnectConfig, retryCount int) time.Duration {
	return calcBackoff(rc, retryCount)
}
