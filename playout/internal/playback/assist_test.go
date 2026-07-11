package playback_test

import (
	"context"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
	"github.com/Waelson/radio-playout-engine/internal/testutil"
)

// assistPlay sends a PLAY command to the manager (used as manual trigger in ASSIST).
func (f *fixture) assistPlay(t *testing.T) {
	t.Helper()
	if err := f.mgr.HandlePlay(context.Background(),
		commands.New(commands.CmdPlay, commands.PlayPayload{})); err != nil {
		t.Fatalf("assistPlay: %v", err)
	}
}

// enterAssist puts the engine into ASSIST mode.
func (f *fixture) enterAssist(t *testing.T) {
	t.Helper()
	if err := f.mgr.HandleEnterAssist(context.Background(),
		commands.New(commands.CmdEnterAssist, commands.EnterAssistPayload{})); err != nil {
		t.Fatalf("HandleEnterAssist: %v", err)
	}
}

// returnAuto returns the engine to AUTO mode.
func (f *fixture) returnAuto(t *testing.T) {
	t.Helper()
	if err := f.mgr.HandleReturnAuto(context.Background(),
		commands.New(commands.CmdReturnAuto, commands.ReturnAutoPayload{})); err != nil {
		t.Fatalf("HandleReturnAuto: %v", err)
	}
}

// waitAssistWaitingN polls until at least n AssistWaiting events appear in the ring buffer.
func waitAssistWaitingN(t *testing.T, evtBus *events.Bus, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		count := 0
		for _, e := range evtBus.Recent(500) {
			if e.Type == events.EvtAssistWaiting {
				count++
			}
		}
		if count >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected %d AssistWaiting events within %s", n, timeout)
}

func waitAssistWaiting(t *testing.T, evtBus *events.Bus, timeout time.Duration) {
	t.Helper()
	waitAssistWaitingN(t, evtBus, 1, timeout)
}

// --- Tests -------------------------------------------------------------------

// TestAssist_EngineWaitsAtItemBoundary verifies that after a single item
// finishes in ASSIST mode the engine does NOT auto-advance to IDLE; it stays
// blocked and the state remains ASSIST (not IDLE).
func TestAssist_EngineWaitsAtItemBoundary(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueue("musicas", 48)
	f.enqueue("musicas", 48) // second item so queue is non-empty after first finishes
	f.enterAssist(t)         // set ASSIST before play so the fast decoder can't race ahead
	f.play(t)

	// Wait until engine is blocked waiting for operator input.
	waitAssistWaiting(t, f.evtBus, 5*time.Second)

	// State must be ASSIST, not IDLE.
	got := f.stateMgr.Snapshot().State
	if got != state.StateAssist {
		t.Errorf("state = %s, want ASSIST", got)
	}
}

// TestAssist_PlayResumesAfterWait verifies that sending a PLAY command while
// the engine is in ASSIST (waiting) causes the next item to start playing.
func TestAssist_PlayResumesAfterWait(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueue("musicas", 48)
	f.enqueue("musicas", 48)
	f.enterAssist(t)
	f.play(t)
	waitAssistWaiting(t, f.evtBus, 5*time.Second)

	// Operator triggers next item.
	f.assistPlay(t)

	// Engine should eventually finish the second item and emit another AssistWaiting
	// (or transition to IDLE if no more items). Either way the state was ASSIST while waiting.
	waitAssistWaitingN(t, f.evtBus, 2, 5*time.Second)

	evts := collectEvents(f.evtBus, events.EvtItemStarted)()
	if len(evts) < 2 {
		t.Errorf("ItemStarted count = %d, want ≥2 (both items played)", len(evts))
	}
}

// TestAssist_AutoAdvancesWithinBreak verifies that the engine advances through
// all spots in the same break without operator intervention.
func TestAssist_AutoAdvancesWithinBreak(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueueBreak(t, "Bloco", 3) // 3 spots, same BreakID
	f.enterAssist(t)
	f.play(t)

	// Trigger the first item of the break.
	waitAssistWaiting(t, f.evtBus, 5*time.Second)
	f.assistPlay(t)

	// Engine must auto-advance through all 3 spots, then wait again (queue empty).
	// Wait for 3 ItemStarted events.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if len(collectEvents(f.evtBus, events.EvtItemStarted)()) >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got := len(collectEvents(f.evtBus, events.EvtItemStarted)())
	if got < 3 {
		t.Errorf("ItemStarted count = %d, want 3 (auto-advanced within break)", got)
	}
}

// TestAssist_WaitsAtBreakEnd verifies that after the last spot of a break the
// engine stops and waits (does not auto-continue to next non-break item).
func TestAssist_WaitsAtBreakEnd(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueueBreak(t, "Bloco", 2)
	f.enqueue("musicas", 48) // music follows the break

	f.enterAssist(t)
	f.play(t)

	// Trigger the break.
	waitAssistWaiting(t, f.evtBus, 5*time.Second)
	f.assistPlay(t)

	// Engine auto-advances through both spots, then waits at break boundary.
	// Collect AssistWaiting events — the second one is the post-break wait.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		count := 0
		for _, e := range f.evtBus.Recent(200) {
			if e.Type == events.EvtAssistWaiting {
				count++
			}
		}
		if count >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	waitingEvts := 0
	for _, e := range f.evtBus.Recent(200) {
		if e.Type == events.EvtAssistWaiting {
			waitingEvts++
		}
	}
	if waitingEvts < 2 {
		t.Errorf("AssistWaiting count = %d, want ≥2 (one before break, one after break)", waitingEvts)
	}

	// State must still be ASSIST (waiting for operator to play the music).
	if got := f.stateMgr.Snapshot().State; got != state.StateAssist {
		t.Errorf("state after break = %s, want ASSIST", got)
	}
}

// TestAssist_ReturnAutoUnblocks verifies that HandleReturnAuto unblocks a
// waiting session and transitions the engine back to PLAYING / AUTO.
func TestAssist_ReturnAutoUnblocks(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueue("musicas", 48)
	f.enqueue("musicas", 48)
	f.enterAssist(t)
	f.play(t)
	waitAssistWaiting(t, f.evtBus, 5*time.Second)

	// Return to AUTO — engine should resume and eventually reach IDLE.
	f.returnAuto(t)
	waitSessionEnd(t, f.evtBus, 5*time.Second)

	if got := f.stateMgr.Snapshot().State; got != state.StateIdle {
		t.Errorf("state after return-auto = %s, want IDLE", got)
	}

	// AssistExited event must have been published.
	found := false
	for _, e := range f.evtBus.Recent(200) {
		if e.Type == events.EvtAssistExited {
			found = true
			break
		}
	}
	if !found {
		t.Error("AssistExited event not published")
	}
}

// TestAssist_StopWhileWaiting verifies that HandleStop cancels a waiting ASSIST
// session and returns the engine to IDLE.
func TestAssist_StopWhileWaiting(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueue("musicas", 48)
	f.enqueue("musicas", 48)
	f.enterAssist(t)
	f.play(t)
	waitAssistWaiting(t, f.evtBus, 5*time.Second)

	f.stop(t)
	waitState(t, f.stateMgr, state.StateIdle, 5*time.Second)
}

// TestAssist_EvtAssistWaitingPublished verifies that EvtAssistWaiting carries
// the expected next-item metadata.
func TestAssist_EvtAssistWaitingPublished(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueue("musicas", 48)
	// Second item is the "next" that should appear in AssistWaiting payload.
	f.queueMgr.Enqueue([]commands.QueueItemInput{{
		Path:       "/fake/second.mp3",
		Type:       "musicas",
		Title:      "Second Track",
		DurationMS: 1000,
	}})
	f.enterAssist(t)
	f.play(t)
	// enterAssist fires one immediate AssistWaiting (before session); after item 1
	// finishes the session loop fires a second one — that's the one with "Second Track".
	waitAssistWaitingN(t, f.evtBus, 2, 5*time.Second)

	// Find LAST AssistWaiting event (oldest-first, so overwrite without break).
	var payload *events.AssistWaitingPayload
	for _, e := range f.evtBus.Recent(200) {
		if e.Type == events.EvtAssistWaiting {
			p := e.Payload.(events.AssistWaitingPayload)
			payload = &p
		}
	}
	if payload == nil {
		t.Fatal("AssistWaiting event not found")
	}
	if payload.NextTitle != "Second Track" {
		t.Errorf("NextTitle = %q, want %q", payload.NextTitle, "Second Track")
	}
	if payload.NextType != "musicas" {
		t.Errorf("NextType = %q, want %q", payload.NextType, "musicas")
	}
}

// TestAssist_PauseResumeRestoresAssistState verifies that pausing while in
// ASSIST mode and then resuming returns the engine to ASSIST (not PLAYING).
func TestAssist_PauseResumeRestoresAssistState(t *testing.T) {
	// Use realtime=true so the item takes real wall-clock time to finish,
	// giving the test goroutine a window to pause mid-playback.
	dec := &testutil.FakeDecoder{Frames: 240000} // ~5 s at 48 kHz
	f := newFixture(t, dec, true, 0)

	f.enqueue("musicas", 240000)
	f.enterAssist(t)
	f.play(t)

	// Wait for the item to start playing (EvtItemStarted).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		found := false
		for _, e := range f.evtBus.Recent(200) {
			if e.Type == events.EvtItemStarted {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Pause while item is playing in ASSIST mode.
	if err := f.mgr.HandlePause(context.Background(),
		commands.New(commands.CmdPause, commands.PausePayload{})); err != nil {
		t.Fatalf("HandlePause: %v", err)
	}
	if got := f.stateMgr.Snapshot().State; got != state.StatePaused {
		t.Errorf("state after pause = %s, want PAUSED", got)
	}

	// Resume — must return to ASSIST, not PLAYING.
	if err := f.mgr.HandleResume(context.Background(),
		commands.New(commands.CmdResume, commands.ResumePayload{})); err != nil {
		t.Fatalf("HandleResume: %v", err)
	}
	if got := f.stateMgr.Snapshot().State; got != state.StateAssist {
		t.Errorf("state after resume = %s, want ASSIST", got)
	}

	// Clean up.
	f.stop(t)
	waitState(t, f.stateMgr, state.StateIdle, 5*time.Second)
}

// TestAssist_EnterDuringPlayback verifies that setting ASSIST before the session
// starts causes the engine to wait after the first item, leaving the second item
// unplayed until the operator triggers it.
func TestAssist_EnterDuringPlayback(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueue("musicas", 48)
	f.enqueue("musicas", 48)
	f.enterAssist(t) // set before play; session starts in ASSIST mode
	f.play(t)

	// enterAssist fires one immediate AssistWaiting (before session); the second
	// fires after item 1 finishes. Wait for that second event so we know item 1
	// has definitely been started before asserting ItemStarted count.
	waitAssistWaitingN(t, f.evtBus, 2, 5*time.Second)

	// Only one item should have been started.
	if got := len(collectEvents(f.evtBus, events.EvtItemStarted)()); got != 1 {
		t.Errorf("ItemStarted count = %d, want 1 (second item must not auto-start)", got)
	}

	// Operator triggers second item.
	f.assistPlay(t)
	// N=3: immediate + after-item-1 + after-item-2
	waitAssistWaitingN(t, f.evtBus, 3, 5*time.Second)

	if got := len(collectEvents(f.evtBus, events.EvtItemStarted)()); got != 2 {
		t.Errorf("ItemStarted count = %d, want 2 after second trigger", got)
	}
}
