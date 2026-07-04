package commands

import (
	"context"
	"fmt"
)

const busBuffer = 64

// Bus is the internal command channel. The API sends commands; the Dispatcher
// consumes them. It is safe for concurrent use.
type Bus struct {
	ch chan Command
}

// NewBus creates a Command Bus with a buffer of busBuffer slots.
func NewBus() *Bus {
	return &Bus{ch: make(chan Command, busBuffer)}
}

// Send enqueues cmd for the Dispatcher. It blocks while the buffer is full;
// ctx cancellation causes an immediate return with the context error.
func (b *Bus) Send(ctx context.Context, cmd Command) error {
	select {
	case b.ch <- cmd:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("command bus: send %s: %w", cmd.Type, ctx.Err())
	}
}

// TrySend enqueues cmd without blocking. Returns false when the buffer is full.
// Use this from non-blocking callbacks (e.g. the audio health monitor).
func (b *Bus) TrySend(cmd Command) bool {
	select {
	case b.ch <- cmd:
		return true
	default:
		return false
	}
}

// Receive returns the read-only channel consumed exclusively by the Dispatcher.
func (b *Bus) Receive() <-chan Command {
	return b.ch
}
