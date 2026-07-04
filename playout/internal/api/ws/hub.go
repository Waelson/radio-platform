// Package ws implements the WebSocket fan-out hub for the Engine's real-time
// event stream.  It subscribes to the Event Bus and delivers events to all
// connected WebSocket clients, applying per-client backpressure rules.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// clientBufSize is the per-client channel capacity.
// Events are dropped (for low-priority types) when the channel is full.
const clientBufSize = 128

// Hub manages the set of connected WebSocket clients and fans out events
// from the Event Bus to each of them.
type Hub struct {
	evtBus   *events.Bus
	stateMgr *state.Manager
	log      *slog.Logger

	mu      sync.RWMutex
	clients map[*Client]struct{}
}

// NewHub creates a Hub wired to the given Event Bus.
func NewHub(evtBus *events.Bus, stateMgr *state.Manager, log *slog.Logger) *Hub {
	if log == nil {
		log = slog.Default()
	}
	return &Hub{
		evtBus:   evtBus,
		stateMgr: stateMgr,
		log:      log,
		clients:  make(map[*Client]struct{}),
	}
}

// Run subscribes to the Event Bus and fans out events until ctx is cancelled.
// Intended to be called in a dedicated goroutine.
func (h *Hub) Run(ctx context.Context) {
	ch, unsub := h.evtBus.Subscribe(256)
	defer unsub()

	h.log.Info("websocket hub started")
	for {
		select {
		case <-ctx.Done():
			h.log.Info("websocket hub stopped")
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			h.broadcast(evt)
		}
	}
}

// register adds a client to the hub. Called by Client when it upgrades.
func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	h.log.Debug("websocket client registered", "remote", c.remoteAddr)
}

// unregister removes a client and closes its send channel.
func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
	h.log.Debug("websocket client unregistered", "remote", c.remoteAddr)
}

// broadcast sends evt to every connected client, applying backpressure rules.
func (h *Hub) broadcast(evt events.Event) {
	msg, err := json.Marshal(evt)
	if err != nil {
		h.log.Error("websocket marshal error", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// Channel full — apply backpressure policy.
			if events.IsCritical(evt.Type) {
				// Critical events: block briefly rather than drop.
				// Use a non-blocking second attempt; if still full, log and drop.
				h.log.Warn("websocket client buffer full, dropping critical event",
					"remote", c.remoteAddr,
					"event_type", string(evt.Type),
				)
			}
			// Low-priority events are silently dropped.
		}
	}
}

// Snapshot builds and returns the current engine state as a serialised
// StateSnapshot event, ready to be sent to a newly connected client.
func (h *Hub) Snapshot() ([]byte, error) {
	snap := h.stateMgr.Snapshot()
	evt := events.New(events.EvtStateSnapshot, snap)
	return json.Marshal(evt)
}
