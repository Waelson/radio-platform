package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
)

func TestBus_SendReceive(t *testing.T) {
	bus := commands.NewBus()

	cmd := commands.Command{
		ID:   "cmd_test",
		Type: commands.CmdPlay,
	}

	ctx := context.Background()
	if err := bus.Send(ctx, cmd); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case got := <-bus.Receive():
		if got.ID != cmd.ID {
			t.Errorf("got ID %q, want %q", got.ID, cmd.ID)
		}
		if got.Type != cmd.Type {
			t.Errorf("got Type %s, want %s", got.Type, cmd.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for command")
	}
}

func TestBus_SendCancelledContext(t *testing.T) {
	// Fill the bus buffer so the next Send would block.
	bus := commands.NewBus()
	ctx := context.Background()
	for i := 0; i < 64; i++ {
		if err := bus.Send(ctx, commands.Command{Type: commands.CmdEnqueue}); err != nil {
			t.Fatalf("pre-fill Send: %v", err)
		}
	}

	// Now cancel the context before sending.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := bus.Send(cancelCtx, commands.Command{Type: commands.CmdPlay})
	if err == nil {
		t.Fatal("Send with cancelled context: expected error, got nil")
	}
}

func TestBus_SendWithDeadline(t *testing.T) {
	bus := commands.NewBus()
	ctx := context.Background()
	// Fill the bus buffer.
	for i := 0; i < 64; i++ {
		_ = bus.Send(ctx, commands.Command{Type: commands.CmdEnqueue})
	}

	// Attempt a send with a tight deadline.
	deadlineCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := bus.Send(deadlineCtx, commands.Command{Type: commands.CmdPlay})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected deadline error, got nil")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Send blocked for %v, expected ~20ms", elapsed)
	}
}

func TestBus_Receive_ReadOnly(t *testing.T) {
	bus := commands.NewBus()
	ch := bus.Receive()
	// Verify the returned channel is read-only at compile time (type assertion).
	var _ <-chan commands.Command = ch
}
