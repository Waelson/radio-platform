package events

import (
	"log/slog"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

const ringCap = 200

// Bus is a non-blocking, fan-out event bus. Publish never blocks: slow
// consumers have low-priority events dropped silently; critical-event drops
// are logged as warnings. The Bus is safe for concurrent use.
type Bus struct {
	mu   sync.Mutex
	subs []*sub

	// ring buffer – stores the last ringCap published events.
	ring  [ringCap]Event
	wHead int // next write position (0–ringCap-1)
	rLen  int // number of valid entries (0–ringCap)

	log *slog.Logger
}

type sub struct {
	ch chan Event
}

// NewBus creates an Event Bus. log may be nil (drops are not warned).
func NewBus(log *slog.Logger) *Bus {
	return &Bus{log: log}
}

// New creates a fully-populated Event: ULID-based ID, current UTC timestamp,
// version 1. Use this to build events before calling Publish.
func New(t EventType, payload any) Event {
	return Event{
		EventID:   "evt_" + ulid.Make().String(),
		Type:      t,
		Version:   1,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
}

// Publish delivers evt to all current subscribers and appends it to the ring
// buffer. It never blocks; full-buffer subscribers lose low-priority events.
func (b *Bus) Publish(evt Event) {
	b.mu.Lock()
	// append to ring
	b.ring[b.wHead] = evt
	b.wHead = (b.wHead + 1) % ringCap
	if b.rLen < ringCap {
		b.rLen++
	}
	// snapshot subscriber list while holding the lock so that concurrent
	// cancel() calls don't race with our iteration below.
	subs := make([]*sub, len(b.subs))
	copy(subs, b.subs)
	b.mu.Unlock()

	for _, s := range subs {
		select {
		case s.ch <- evt:
		default:
			if IsCritical(evt.Type) && b.log != nil {
				b.log.Warn("event bus: slow consumer dropped critical event",
					"event_type", string(evt.Type),
					"event_id", evt.EventID,
				)
			}
		}
	}
}

// Subscribe returns a read-only channel of future events and a cancel func.
// Calling cancel removes the subscriber from the bus; the consumer's goroutine
// should stop reading via its own context — the channel is not closed, so no
// panic occurs if Publish delivers to a just-cancelled subscriber.
// bufSize controls the channel buffer depth (≤0 → 128).
func (b *Bus) Subscribe(bufSize int) (<-chan Event, func()) {
	if bufSize <= 0 {
		bufSize = 128
	}
	s := &sub{ch: make(chan Event, bufSize)}

	b.mu.Lock()
	b.subs = append(b.subs, s)
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		for i, candidate := range b.subs {
			if candidate == s {
				last := len(b.subs) - 1
				b.subs[i] = b.subs[last]
				b.subs[last] = nil
				b.subs = b.subs[:last]
				break
			}
		}
		b.mu.Unlock()
	}

	return s.ch, cancel
}

// Recent returns the last n events in the ring buffer, oldest-first.
// If n ≤ 0 or n exceeds stored events, all stored events are returned.
func (b *Bus) Recent(n int) []Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	if n <= 0 || n > b.rLen {
		n = b.rLen
	}
	if n == 0 {
		return nil
	}

	out := make([]Event, n)
	// oldest of the last-n entries sits at:
	start := (b.wHead - n + ringCap) % ringCap
	for i := 0; i < n; i++ {
		out[i] = b.ring[(start+i)%ringCap]
	}
	return out
}
