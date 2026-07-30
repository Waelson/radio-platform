package linein_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/linein"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func newHandlerEnv() (*linein.Handler, *state.Manager, *events.Bus) {
	stateMgr := state.NewManager("test-engine")
	evtBus := events.NewBus(slog.Default())
	out := &stubOutput{}
	h := linein.NewHandler(out, stateMgr, evtBus, slog.Default())
	return h, stateMgr, evtBus
}

func startCmd(label string) commands.Command {
	return commands.New(commands.CmdLineInStart, commands.LineInStartPayload{
		DeviceID:    ":0",
		Label:       label,
		DurationMS:  0,
		TriggeredBy: "manual",
	})
}

func stopCmd() commands.Command {
	return commands.New(commands.CmdLineInStop, commands.LineInStopPayload{
		Reason: "test",
	})
}

// drainUntil reads from ch until the predicate returns true or the timeout
// elapses. Returns true if the condition was met.
func drainUntil(ch <-chan events.Event, pred func(events.Event) bool, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			return false
		case e, ok := <-ch:
			if !ok {
				return false
			}
			if pred(e) {
				return true
			}
		}
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestHandler_StartTransitionsToLineIn verifies that HandleStart updates
// the state machine to StateLineIn.
func TestHandler_StartTransitionsToLineIn(t *testing.T) {
	h, stateMgr, _ := newHandlerEnv()
	in := &stubInput{batches: 1_000_000, sample: 0.5}
	// Replace the handler's default FFmpegCapture via a custom Manager would
	// require refactoring; instead we rely on a short DurationMS so the session
	// ends quickly and doesn't hold state.
	// We test the state *immediately* after HandleStart before the goroutine exits.

	// Use a session that will end on its own.
	cmd := commands.New(commands.CmdLineInStart, commands.LineInStartPayload{
		DeviceID:    ":0",
		Label:       "Studio 1",
		DurationMS:  50, // short session — cap
		TriggeredBy: "manual",
	})
	_ = in // not used directly here

	// In the real Handler, HandleStart creates an FFmpegCapture which requires
	// a running ffmpeg binary. We test the state machine update via a wrapper
	// approach: verify that when the session starts successfully the state is
	// StateLineIn, and when it ends it returns to StateIdle.
	// Since FFmpegCapture may not be available in CI, we guard with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := h.HandleStart(ctx, cmd)
	if err != nil {
		// FFmpeg not available in this environment — skip instead of fail.
		t.Skipf("HandleStart returned error (ffmpeg may not be available): %v", err)
	}

	snap := stateMgr.Snapshot()
	if snap.State != state.StateLineIn {
		t.Errorf("state = %s, want StateLineIn after HandleStart", snap.State)
	}
	if snap.LineIn == nil {
		t.Fatal("Snapshot.LineIn should be non-nil after HandleStart")
	}
	if snap.LineIn.Label != "Studio 1" {
		t.Errorf("LineIn.Label = %q, want %q", snap.LineIn.Label, "Studio 1")
	}
}

// TestHandler_StopWithNoActiveSession returns a RejectedError.
func TestHandler_StopWithNoActiveSession(t *testing.T) {
	h, _, _ := newHandlerEnv()

	err := h.HandleStop(context.Background(), stopCmd())
	if err == nil {
		t.Fatal("expected error when stopping with no active session")
	}
	var rejected *commands.RejectedError
	if !isRejectedErr(err, &rejected) {
		t.Errorf("expected *commands.RejectedError, got %T: %v", err, err)
	}
}

// TestHandler_DoubleStart returns RejectedError when a session is already active.
func TestHandler_DoubleStart(t *testing.T) {
	h, _, _ := newHandlerEnv()
	ctx := context.Background()

	cmd := commands.New(commands.CmdLineInStart, commands.LineInStartPayload{
		DeviceID: ":0", Label: "first", DurationMS: 500, TriggeredBy: "manual",
	})

	err := h.HandleStart(ctx, cmd)
	if err != nil {
		t.Skipf("HandleStart returned error (ffmpeg may not be available): %v", err)
	}
	defer h.HandleStop(ctx, stopCmd()) //nolint

	cmd2 := commands.New(commands.CmdLineInStart, commands.LineInStartPayload{
		DeviceID: ":0", Label: "second", DurationMS: 500, TriggeredBy: "manual",
	})
	err2 := h.HandleStart(ctx, cmd2)
	if err2 == nil {
		t.Fatal("expected error on double start")
	}
	var rejected *commands.RejectedError
	if !isRejectedErr(err2, &rejected) {
		t.Errorf("expected *commands.RejectedError on double start, got %T: %v", err2, err2)
	}
}

// TestHandler_SessionEndResetsState verifies that stopping the session via
// HandleStop causes EvtLineInStopped to be published and state to return to IDLE.
func TestHandler_SessionEndResetsState(t *testing.T) {
	h, stateMgr, evtBus := newHandlerEnv()
	ch, unsub := evtBus.Subscribe(32)
	defer unsub()

	ctx := context.Background()
	cmd := commands.New(commands.CmdLineInStart, commands.LineInStartPayload{
		DeviceID: ":0", Label: "short", DurationMS: 0, TriggeredBy: "manual",
	})
	if err := h.HandleStart(ctx, cmd); err != nil {
		t.Skipf("HandleStart returned error (ffmpeg may not be available): %v", err)
	}

	// Give the session a moment to stabilise, then stop it.
	time.Sleep(100 * time.Millisecond)
	if err := h.HandleStop(ctx, stopCmd()); err != nil {
		t.Fatalf("HandleStop: %v", err)
	}

	// Wait for EvtLineInStopped — the forward goroutine publishes this after stop.
	stopped := drainUntil(ch, func(e events.Event) bool {
		return e.Type == events.EvtLineInStopped
	}, 5*time.Second)
	if !stopped {
		t.Fatal("expected EvtLineInStopped within 5s after HandleStop")
	}

	// After the session ends the state must be IDLE again.
	if snap := stateMgr.Snapshot(); snap.State != state.StateIdle {
		t.Errorf("state = %s, want StateIdle after session ends", snap.State)
	}
}

// TestHandler_InvalidPayload returns a non-nil error for unexpected payload type.
func TestHandler_InvalidPayload(t *testing.T) {
	h, _, _ := newHandlerEnv()

	bad := commands.Command{Type: commands.CmdLineInStart, Payload: "not a payload"}
	err := h.HandleStart(context.Background(), bad)
	if err == nil {
		t.Fatal("expected error for invalid payload type")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func isRejectedErr(err error, target **commands.RejectedError) bool {
	e, ok := err.(*commands.RejectedError)
	if ok {
		*target = e
	}
	return ok
}
