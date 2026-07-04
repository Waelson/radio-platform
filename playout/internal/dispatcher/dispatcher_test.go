package dispatcher_test

import (
	"context"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/dispatcher"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// runDispatcher starts the dispatcher in a goroutine and returns a cancel func.
func runDispatcher(t *testing.T, d *dispatcher.Dispatcher) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go d.Run(ctx)
	return cancel
}

// sendAndWait sends a command and waits for an event matching predicate or
// returns the first received event within timeout.
func waitEvent(t *testing.T, ch <-chan events.Event, want events.EventType) events.Event {
	t.Helper()
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case evt := <-ch:
			if evt.Type == want {
				return evt
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event %s", want)
		}
	}
}

func setup() (*commands.Bus, *events.Bus, *state.Manager, *dispatcher.Dispatcher) {
	cmdBus := commands.NewBus()
	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test-engine")
	d := dispatcher.New(cmdBus, evtBus, stateMgr, nil)
	return cmdBus, evtBus, stateMgr, d
}

func TestDispatcher_AcceptsCommandInValidState(t *testing.T) {
	cmdBus, evtBus, stateMgr, d := setup()
	stateMgr.SetState(state.StateIdle)

	ch, cancel := evtBus.Subscribe(16)
	defer cancel()

	stopDispatcher := runDispatcher(t, d)
	defer stopDispatcher()

	cmd := commands.Command{ID: "cmd_1", Type: commands.CmdPlay}
	_ = cmdBus.Send(context.Background(), cmd)

	evt := waitEvent(t, ch, events.EvtCommandAccepted)
	p := evt.Payload.(events.CommandAcceptedPayload)
	if p.CommandID != "cmd_1" {
		t.Errorf("CommandID = %q, want cmd_1", p.CommandID)
	}
}

func TestDispatcher_RejectsCommandInWrongState(t *testing.T) {
	cmdBus, evtBus, stateMgr, d := setup()
	// PAUSED state: only RESUME and STOP are allowed.
	stateMgr.SetState(state.StatePaused)

	ch, cancel := evtBus.Subscribe(16)
	defer cancel()

	stopDispatcher := runDispatcher(t, d)
	defer stopDispatcher()

	cmd := commands.Command{ID: "cmd_2", Type: commands.CmdPlay}
	_ = cmdBus.Send(context.Background(), cmd)

	evt := waitEvent(t, ch, events.EvtCommandRejected)
	p := evt.Payload.(events.CommandRejectedPayload)
	if p.CommandID != "cmd_2" {
		t.Errorf("CommandID = %q, want cmd_2", p.CommandID)
	}
	if p.Reason == "" {
		t.Error("Reason should not be empty")
	}
}

func TestDispatcher_EnterPanicAllowedInAnyState(t *testing.T) {
	for _, st := range []state.PlayerState{
		state.StateIdle,
		state.StatePlaying,
		state.StatePaused,
		state.StateAssist,
		state.StateError,
	} {
		t.Run(string(st), func(t *testing.T) {
			cmdBus, evtBus, stateMgr, d := setup()
			stateMgr.SetState(st)

			ch, cancel := evtBus.Subscribe(16)
			defer cancel()

			stopDispatcher := runDispatcher(t, d)
			defer stopDispatcher()

			cmd := commands.Command{ID: "panic_cmd", Type: commands.CmdEnterPanic}
			_ = cmdBus.Send(context.Background(), cmd)

			waitEvent(t, ch, events.EvtCommandAccepted)
		})
	}
}

func TestDispatcher_EnterPanicRejectedInStopping(t *testing.T) {
	cmdBus, evtBus, stateMgr, d := setup()
	stateMgr.SetState(state.StateStopping)

	ch, cancel := evtBus.Subscribe(16)
	defer cancel()

	stopDispatcher := runDispatcher(t, d)
	defer stopDispatcher()

	cmd := commands.Command{ID: "panic_stop", Type: commands.CmdEnterPanic}
	_ = cmdBus.Send(context.Background(), cmd)

	waitEvent(t, ch, events.EvtCommandRejected)
}

func TestDispatcher_HandlerCalledOnAccept(t *testing.T) {
	cmdBus, evtBus, stateMgr, d := setup()
	stateMgr.SetState(state.StateIdle)

	called := make(chan commands.Command, 1)
	d.Handle(commands.CmdPlay, func(_ context.Context, cmd commands.Command) error {
		called <- cmd
		return nil
	})

	ch, cancel := evtBus.Subscribe(16)
	defer cancel()

	stopDispatcher := runDispatcher(t, d)
	defer stopDispatcher()

	cmd := commands.Command{ID: "cmd_play", Type: commands.CmdPlay}
	_ = cmdBus.Send(context.Background(), cmd)

	waitEvent(t, ch, events.EvtCommandAccepted)

	select {
	case got := <-called:
		if got.ID != "cmd_play" {
			t.Errorf("handler received ID %q, want cmd_play", got.ID)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("handler was not called")
	}
}

func TestDispatcher_RecordsLastCommand(t *testing.T) {
	cmdBus, evtBus, stateMgr, d := setup()
	stateMgr.SetState(state.StateIdle)

	ch, cancel := evtBus.Subscribe(16)
	defer cancel()

	stopDispatcher := runDispatcher(t, d)
	defer stopDispatcher()

	cmd := commands.Command{ID: "cmd_enq", Type: commands.CmdEnqueue}
	_ = cmdBus.Send(context.Background(), cmd)

	waitEvent(t, ch, events.EvtCommandAccepted)
	// Give handler time to record.
	time.Sleep(10 * time.Millisecond)

	s := stateMgr.Snapshot()
	if s.LastCommand == nil {
		t.Fatal("LastCommand should be set")
	}
	if s.LastCommand.Command != string(commands.CmdEnqueue) {
		t.Errorf("LastCommand.Command = %q", s.LastCommand.Command)
	}
	if !s.LastCommand.Accepted {
		t.Error("LastCommand.Accepted should be true")
	}
}

func TestDispatcher_NoCommandsInStartingState(t *testing.T) {
	cmdBus, evtBus, stateMgr, d := setup()
	// StateStarting is the default initial state.
	_ = stateMgr

	ch, cancel := evtBus.Subscribe(16)
	defer cancel()

	stopDispatcher := runDispatcher(t, d)
	defer stopDispatcher()

	cmd := commands.Command{ID: "cmd_x", Type: commands.CmdPlay}
	_ = cmdBus.Send(context.Background(), cmd)

	waitEvent(t, ch, events.EvtCommandRejected)
}
