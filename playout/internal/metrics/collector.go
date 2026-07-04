// Package metrics collects runtime counters for the Playout Engine.
// It subscribes to the Event Bus and increments atomic counters as events
// arrive — no locks, no blocking, safe to call from any goroutine.
package metrics

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
)

// Snapshot is a point-in-time copy of all counters.
type Snapshot struct {
	UptimeSeconds        int64 `json:"uptime_seconds"`
	ItemsPlayedTotal     int64 `json:"items_played_total"`
	ItemsFailedTotal     int64 `json:"items_failed_total"`
	CommandsTotal        int64 `json:"commands_total"`
	CommandsRejectedTotal int64 `json:"commands_rejected_total"`
	UnderrunTotal        int64 `json:"underrun_total"`
	PanicTotal           int64 `json:"panic_total"`
	DecoderErrorsTotal   int64 `json:"decoder_errors_total"`
	OutputErrorsTotal    int64 `json:"output_errors_total"`
}

// Collector subscribes to the event bus and maintains runtime counters.
type Collector struct {
	startedAt time.Time

	itemsPlayed     atomic.Int64
	itemsFailed     atomic.Int64
	commands        atomic.Int64
	commandsRejected atomic.Int64
	underruns       atomic.Int64
	panics          atomic.Int64
	decoderErrors   atomic.Int64
	outputErrors    atomic.Int64
}

// New creates a Collector. Call Run to start consuming events.
func New() *Collector {
	return &Collector{startedAt: time.Now()}
}

// Run subscribes to the event bus and processes events until ctx is cancelled.
// Must be called in its own goroutine.
func (c *Collector) Run(ctx context.Context, bus *events.Bus) {
	ch, cancel := bus.Subscribe(256)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			c.handle(evt)
		}
	}
}

// Snapshot returns a point-in-time copy of all counters.
func (c *Collector) Snapshot() Snapshot {
	return Snapshot{
		UptimeSeconds:         int64(time.Since(c.startedAt).Seconds()),
		ItemsPlayedTotal:      c.itemsPlayed.Load(),
		ItemsFailedTotal:      c.itemsFailed.Load(),
		CommandsTotal:         c.commands.Load(),
		CommandsRejectedTotal: c.commandsRejected.Load(),
		UnderrunTotal:         c.underruns.Load(),
		PanicTotal:            c.panics.Load(),
		DecoderErrorsTotal:    c.decoderErrors.Load(),
		OutputErrorsTotal:     c.outputErrors.Load(),
	}
}

func (c *Collector) handle(evt events.Event) {
	switch evt.Type {
	case events.EvtItemFinished:
		if p, ok := evt.Payload.(events.ItemFinishedPayload); ok {
			switch p.Result {
			case "PLAYED":
				c.itemsPlayed.Add(1)
			case "FAILED":
				c.itemsFailed.Add(1)
			}
		}
	case events.EvtCommandAccepted:
		c.commands.Add(1)
	case events.EvtCommandRejected:
		c.commands.Add(1)
		c.commandsRejected.Add(1)
	case events.EvtAudioHealthChanged:
		if p, ok := evt.Payload.(events.AudioHealthChangedPayload); ok {
			// UnderrunCount in the payload is a cumulative total from the health
			// monitor; we store it directly (not increment) via a compare-and-swap
			// so the metric stays accurate even after reconnect.
			current := c.underruns.Load()
			if p.UnderrunCount > current {
				c.underruns.Store(p.UnderrunCount)
			}
		}
	case events.EvtPanicEntered:
		c.panics.Add(1)
	case events.EvtDecoderError:
		c.decoderErrors.Add(1)
	case events.EvtOutputOpenFailed, events.EvtOutputWriteFailed:
		c.outputErrors.Add(1)
	}
}
