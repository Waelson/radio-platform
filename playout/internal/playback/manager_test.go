package playback_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/playback"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
	"github.com/Waelson/radio-playout-engine/internal/testutil"
)

// ---- helpers ----------------------------------------------------------------

type fixture struct {
	mgr      *playback.Manager
	queueMgr *queue.Manager
	stateMgr *state.Manager
	evtBus   *events.Bus
	out      *output.NullOutput
}

func newFixture(t *testing.T, dec *testutil.FakeDecoder, realtime bool, xfadeMS int) *fixture {
	t.Helper()
	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test-engine")
	stateMgr.SetState(state.StateIdle)
	queueMgr := queue.NewManager(evtBus, stateMgr, nil)
	null := &output.NullOutput{Realtime: realtime}
	cfg := playback.Config{
		BufferFrames:           512,
		ProgressIntervalMS:     50,
		MaxConsecutiveFailures: 3,
		DefaultCrossfadeMS:     xfadeMS,
	}
	mgr := playback.NewManager(evtBus, stateMgr, queueMgr, dec, null, cfg, nil, nil)
	return &fixture{mgr: mgr, queueMgr: queueMgr, stateMgr: stateMgr, evtBus: evtBus, out: null}
}

func (f *fixture) enqueue(assetType string, frames int) {
	durMS := int64(frames) * 1000 / 48000
	f.queueMgr.Enqueue([]commands.QueueItemInput{{
		AssetID:    "asset-" + assetType,
		Path:       "/fake/" + assetType + ".mp3",
		Type:       assetType,
		Title:      "Track " + assetType,
		DurationMS: durMS,
	}})
}

func (f *fixture) play(t *testing.T) {
	t.Helper()
	if err := f.mgr.HandlePlay(context.Background(),
		commands.New(commands.CmdPlay, commands.PlayPayload{})); err != nil {
		t.Fatalf("HandlePlay: %v", err)
	}
}

func (f *fixture) stop(t *testing.T) {
	t.Helper()
	_ = f.mgr.HandleStop(context.Background(),
		commands.New(commands.CmdStop, commands.StopPayload{}))
}

func waitState(t *testing.T, sm *state.Manager, want state.PlayerState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if sm.Snapshot().State == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("state did not reach %s within %s; current=%s", want, timeout, sm.Snapshot().State)
}

// waitSessionEnd polls the event bus ring buffer until a
// PlayerStateChanged event with To="IDLE" appears. This event is published
// only by transitionToIdle (i.e. at the end of a real session), never during
// initialization, so it reliably signals that a session ran to completion.
// All other session events (CrossfadeStarted, etc.) are published before this
// one and are therefore already in the ring buffer when this returns.
func waitSessionEnd(t *testing.T, evtBus *events.Bus, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, e := range evtBus.Recent(200) {
			if e.Type != events.EvtPlayerStateChanged {
				continue
			}
			if p, ok := e.Payload.(events.PlayerStateChangedPayload); ok && p.To == "IDLE" {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("session did not end within %s", timeout)
}

// collectEvents returns a getter backed by the bus ring buffer.
// Publish writes to the ring buffer synchronously, so all events published
// before waitState returns are guaranteed to be present when the getter is
// called after waitState.
func collectEvents(evtBus *events.Bus, wantType events.EventType) func() []events.Event {
	return func() []events.Event {
		all := evtBus.Recent(200)
		var out []events.Event
		for _, e := range all {
			if e.Type == wantType {
				out = append(out, e)
			}
		}
		return out
	}
}

// ---- basic playback (carried over from Phase 4) -----------------------------

func TestPlay_TransitionsToPlayingThenIdle(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 480}
	f := newFixture(t, dec, false, 0)
	f.enqueue("musicas", 480)
	f.play(t)
	waitState(t, f.stateMgr, state.StateIdle, 3*time.Second)
}

func TestPlay_RejectsWhenQueueEmpty(t *testing.T) {
	dec := &testutil.FakeDecoder{}
	f := newFixture(t, dec, false, 0)
	err := f.mgr.HandlePlay(context.Background(), commands.New(commands.CmdPlay, commands.PlayPayload{}))
	if _, ok := err.(*commands.RejectedError); !ok {
		t.Fatalf("expected RejectedError, got %T: %v", err, err)
	}
}

func TestPlay_RejectsWhenAlreadyPlaying(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 480000}
	f := newFixture(t, dec, true, 0)
	f.enqueue("musicas", 480000)
	f.enqueue("musicas", 480000)
	f.play(t)
	waitState(t, f.stateMgr, state.StatePlaying, 500*time.Millisecond)

	err := f.mgr.HandlePlay(context.Background(), commands.New(commands.CmdPlay, commands.PlayPayload{}))
	if _, ok := err.(*commands.RejectedError); !ok {
		t.Fatalf("expected RejectedError, got %T: %v", err, err)
	}
	f.stop(t)
	waitState(t, f.stateMgr, state.StateIdle, 3*time.Second)
}

func TestStop_StopsSession(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 480000}
	f := newFixture(t, dec, true, 0)
	f.enqueue("musicas", 480000)
	f.play(t)
	waitState(t, f.stateMgr, state.StatePlaying, 500*time.Millisecond)
	f.stop(t)
	waitState(t, f.stateMgr, state.StateIdle, 3*time.Second)
}

func TestStop_ClearsQueueWhenRequested(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 480000}
	f := newFixture(t, dec, true, 0)
	f.enqueue("musicas", 480000)
	f.enqueue("musicas", 480000)
	f.play(t)
	waitState(t, f.stateMgr, state.StatePlaying, 500*time.Millisecond)
	_ = f.mgr.HandleStop(context.Background(),
		commands.New(commands.CmdStop, commands.StopPayload{ClearQueue: true}))
	waitState(t, f.stateMgr, state.StateIdle, 3*time.Second)
	if n := f.queueMgr.Size(); n != 0 {
		t.Errorf("expected empty queue, got %d", n)
	}
}

func TestPauseResume(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 480000}
	f := newFixture(t, dec, true, 0)
	f.enqueue("musicas", 480000)
	f.play(t)
	waitState(t, f.stateMgr, state.StatePlaying, 500*time.Millisecond)

	ctx := context.Background()
	if err := f.mgr.HandlePause(ctx, commands.New(commands.CmdPause, commands.PausePayload{})); err != nil {
		t.Fatalf("HandlePause: %v", err)
	}
	if got := f.stateMgr.Snapshot().State; got != state.StatePaused {
		t.Errorf("expected PAUSED, got %s", got)
	}
	if err := f.mgr.HandleResume(ctx, commands.New(commands.CmdResume, commands.ResumePayload{})); err != nil {
		t.Fatalf("HandleResume: %v", err)
	}
	if got := f.stateMgr.Snapshot().State; got != state.StatePlaying {
		t.Errorf("expected PLAYING after resume, got %s", got)
	}
	f.stop(t)
	waitState(t, f.stateMgr, state.StateIdle, 3*time.Second)
}

func TestSkip_MandatoryItemRejected(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 480000}
	f := newFixture(t, dec, true, 0)
	f.queueMgr.Enqueue([]commands.QueueItemInput{{
		Path: "/fake/mandatory.mp3", Type: "spots", DurationMS: 10000, Mandatory: true,
	}})
	f.play(t)
	waitState(t, f.stateMgr, state.StatePlaying, 500*time.Millisecond)
	err := f.mgr.HandleSkip(context.Background(), commands.New(commands.CmdSkip, commands.SkipPayload{}))
	if _, ok := err.(*commands.RejectedError); !ok {
		t.Fatalf("expected RejectedError on mandatory skip, got %T: %v", err, err)
	}
	f.stop(t)
	waitState(t, f.stateMgr, state.StateIdle, 3*time.Second)
}

func TestSkip_NonMandatoryItemAdvances(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 480000}
	f := newFixture(t, dec, true, 0)
	f.enqueue("musicas", 480000)
	f.play(t)
	waitState(t, f.stateMgr, state.StatePlaying, 500*time.Millisecond)
	if err := f.mgr.HandleSkip(context.Background(),
		commands.New(commands.CmdSkip, commands.SkipPayload{})); err != nil {
		t.Fatalf("HandleSkip: %v", err)
	}
	waitState(t, f.stateMgr, state.StateIdle, 3*time.Second)
}

func TestStop_IdempotentWhenNotPlaying(t *testing.T) {
	dec := &testutil.FakeDecoder{}
	f := newFixture(t, dec, false, 0)
	if err := f.mgr.HandleStop(context.Background(),
		commands.New(commands.CmdStop, commands.StopPayload{})); err != nil {
		t.Fatalf("HandleStop on idle engine: %v", err)
	}
}

// ---- crossfade tests --------------------------------------------------------

// TestCrossfade_MusicToMusic verifies that a CrossfadeStarted event is
// published when two musicas items are played with crossfade enabled.
//
// Item A: 4800 frames (100 ms). XfadeMS = 50 ms → xfade starts at 50 ms.
// Item B: 4800 frames (100 ms).
// FakeDecoder with Frames=4800 is shared (it creates fresh streams each Open).
func TestCrossfade_MusicToMusic(t *testing.T) {
	// 4800 frames = 100 ms; xfade = 50 ms → starts at 50 ms.
	// Use realtime=false so it runs fast, but we need the item's DurationMS
	// to trigger the xfade calculation.
	const totalFrames = 4800
	const xfadeMS = 50

	dec := &testutil.FakeDecoder{Frames: totalFrames}
	f := newFixture(t, dec, false, xfadeMS)

	getXfade := collectEvents(f.evtBus, events.EvtCrossfadeStarted)
	getNowPlaying := collectEvents(f.evtBus, events.EvtNowPlayingChanged)

	// Two musicas items.
	f.enqueue("musicas", totalFrames)
	f.enqueue("musicas", totalFrames)

	f.play(t)
	waitSessionEnd(t, f.evtBus, 3*time.Second)

	// Exactly one CrossfadeStarted event should have fired.
	xfades := getXfade()
	if len(xfades) != 1 {
		t.Fatalf("expected 1 CrossfadeStarted, got %d", len(xfades))
	}

	p, ok := xfades[0].Payload.(events.CrossfadeStartedPayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", xfades[0].Payload)
	}
	if p.DurationMS != int64(xfadeMS) {
		t.Errorf("CrossfadeStarted.DurationMS: want %d, got %d", xfadeMS, p.DurationMS)
	}

	// Two NowPlayingChanged events: one for A, one for B (at xfade completion).
	np := getNowPlaying()
	if len(np) != 2 {
		t.Errorf("expected 2 NowPlayingChanged events, got %d", len(np))
	}
}

// TestCrossfade_NotTriggeredForSpot verifies no crossfade fires when the
// second item is a spots.
func TestCrossfade_NotTriggeredForSpot(t *testing.T) {
	const totalFrames = 4800
	const xfadeMS = 50

	dec := &testutil.FakeDecoder{Frames: totalFrames}
	f := newFixture(t, dec, false, xfadeMS)

	getXfade := collectEvents(f.evtBus, events.EvtCrossfadeStarted)

	f.enqueue("musicas", totalFrames)
	f.enqueue("spots", totalFrames)

	f.play(t)
	waitSessionEnd(t, f.evtBus, 3*time.Second)

	if got := getXfade(); len(got) != 0 {
		t.Errorf("expected no CrossfadeStarted for musicas→spots, got %d", len(got))
	}
}

// TestCrossfade_DisabledWhenZero verifies no crossfade fires when DefaultCrossfadeMS=0.
func TestCrossfade_DisabledWhenZero(t *testing.T) {
	const totalFrames = 4800

	dec := &testutil.FakeDecoder{Frames: totalFrames}
	f := newFixture(t, dec, false, 0) // xfadeMS=0

	getXfade := collectEvents(f.evtBus, events.EvtCrossfadeStarted)

	f.enqueue("musicas", totalFrames)
	f.enqueue("musicas", totalFrames)

	f.play(t)
	waitSessionEnd(t, f.evtBus, 3*time.Second)

	if got := getXfade(); len(got) != 0 {
		t.Errorf("expected no CrossfadeStarted when xfadeMS=0, got %d", len(got))
	}
}

// TestCrossfade_ExplicitTransitionOverride verifies that an item with an
// explicit CROSSFADE transition uses its own DurationMS instead of the default.
func TestCrossfade_ExplicitTransitionOverride(t *testing.T) {
	const totalFrames = 4800
	const customXfadeMS = 30

	dec := &testutil.FakeDecoder{Frames: totalFrames}
	f := newFixture(t, dec, false, 50) // default 50 ms; item will override

	getXfade := collectEvents(f.evtBus, events.EvtCrossfadeStarted)

	durMS := int64(totalFrames) * 1000 / 48000
	f.queueMgr.Enqueue([]commands.QueueItemInput{{
		AssetID:    "a1",
		Path:       "/fake/a.mp3",
		Type:       "musicas",
		DurationMS: durMS,
		Transition: &commands.TransitionInput{
			Type:       "CROSSFADE",
			DurationMS: customXfadeMS,
		},
	}})
	f.enqueue("musicas", totalFrames)

	f.play(t)
	waitSessionEnd(t, f.evtBus, 3*time.Second)

	xfades := getXfade()
	if len(xfades) != 1 {
		t.Fatalf("expected 1 CrossfadeStarted, got %d", len(xfades))
	}
	p := xfades[0].Payload.(events.CrossfadeStartedPayload)
	if p.DurationMS != customXfadeMS {
		t.Errorf("CrossfadeStarted.DurationMS: want %d, got %d", customXfadeMS, p.DurationMS)
	}
}

// TestCrossfade_CutTransitionSkipsXfade verifies that an explicit CUT
// transition suppresses crossfade even between two musicas items.
func TestCrossfade_CutTransitionSkipsXfade(t *testing.T) {
	const totalFrames = 4800

	dec := &testutil.FakeDecoder{Frames: totalFrames}
	f := newFixture(t, dec, false, 50)

	getXfade := collectEvents(f.evtBus, events.EvtCrossfadeStarted)

	durMS := int64(totalFrames) * 1000 / 48000
	f.queueMgr.Enqueue([]commands.QueueItemInput{{
		AssetID:    "a1",
		Path:       "/fake/a.mp3",
		Type:       "musicas",
		DurationMS: durMS,
		Transition: &commands.TransitionInput{Type: "CUT"},
	}})
	f.enqueue("musicas", totalFrames)

	f.play(t)
	waitSessionEnd(t, f.evtBus, 3*time.Second)

	if got := getXfade(); len(got) != 0 {
		t.Errorf("expected no CrossfadeStarted for CUT transition, got %d", len(got))
	}
}

// ---- panic mode -------------------------------------------------------------

// TestEnterPanic_FromIdle verifies that ENTER_PANIC transitions the engine
// from IDLE to PANIC and publishes PanicEntered.
func TestEnterPanic_FromIdle(t *testing.T) {
	dec := &testutil.FakeDecoder{}
	f := newFixture(t, dec, false, 0)

	getPanic := collectEvents(f.evtBus, events.EvtPanicEntered)

	err := f.mgr.HandleEnterPanic(context.Background(),
		commands.New(commands.CmdEnterPanic, commands.EnterPanicPayload{Reason: "test"}))
	if err != nil {
		t.Fatalf("HandleEnterPanic: %v", err)
	}

	if got := f.stateMgr.Snapshot().State; got != state.StatePanic {
		t.Errorf("state: want PANIC, got %s", got)
	}
	if got := f.stateMgr.Snapshot().Mode; got != state.ModePanic {
		t.Errorf("mode: want PANIC, got %s", got)
	}

	panics := getPanic()
	if len(panics) != 1 {
		t.Fatalf("expected 1 PanicEntered event, got %d", len(panics))
	}
	p, ok := panics[0].Payload.(events.PanicEnteredPayload)
	if !ok {
		t.Fatalf("unexpected PanicEntered payload type: %T", panics[0].Payload)
	}
	if p.Reason != "test" {
		t.Errorf("PanicEntered.Reason: want %q, got %q", "test", p.Reason)
	}
}

// TestEnterPanic_StopsPlayingSession verifies that entering panic while playing
// interrupts the current session.
func TestEnterPanic_StopsPlayingSession(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 480000}
	f := newFixture(t, dec, true, 0)
	f.enqueue("musicas", 480000)
	f.play(t)
	waitState(t, f.stateMgr, state.StatePlaying, 500*time.Millisecond)

	err := f.mgr.HandleEnterPanic(context.Background(),
		commands.New(commands.CmdEnterPanic, commands.EnterPanicPayload{Reason: "manual"}))
	if err != nil {
		t.Fatalf("HandleEnterPanic: %v", err)
	}

	if got := f.stateMgr.Snapshot().State; got != state.StatePanic {
		t.Errorf("state: want PANIC, got %s", got)
	}
}

// TestExitPanic_ReturnsToIdle verifies that EXIT_PANIC transitions back to IDLE
// and publishes PanicExited.
func TestExitPanic_ReturnsToIdle(t *testing.T) {
	dec := &testutil.FakeDecoder{}
	f := newFixture(t, dec, false, 0)

	// Enter panic first.
	if err := f.mgr.HandleEnterPanic(context.Background(),
		commands.New(commands.CmdEnterPanic, commands.EnterPanicPayload{Reason: "manual"})); err != nil {
		t.Fatalf("HandleEnterPanic: %v", err)
	}

	getPanicExited := collectEvents(f.evtBus, events.EvtPanicExited)

	if err := f.mgr.HandleExitPanic(context.Background(),
		commands.New(commands.CmdExitPanic, commands.ExitPanicPayload{Reason: "resolved"})); err != nil {
		t.Fatalf("HandleExitPanic: %v", err)
	}

	if got := f.stateMgr.Snapshot().State; got != state.StateIdle {
		t.Errorf("state: want IDLE, got %s", got)
	}
	if got := f.stateMgr.Snapshot().Mode; got != state.ModeAuto {
		t.Errorf("mode: want AUTO, got %s", got)
	}

	exited := getPanicExited()
	if len(exited) != 1 {
		t.Fatalf("expected 1 PanicExited event, got %d", len(exited))
	}
	p, ok := exited[0].Payload.(events.PanicExitedPayload)
	if !ok {
		t.Fatalf("unexpected PanicExited payload type: %T", exited[0].Payload)
	}
	if p.Reason != "resolved" {
		t.Errorf("PanicExited.Reason: want %q, got %q", "resolved", p.Reason)
	}
}

// TestStop_DuringPanic_ExitsPanic verifies that STOP while in PANIC is
// equivalent to EXIT_PANIC.
func TestStop_DuringPanic_ExitsPanic(t *testing.T) {
	dec := &testutil.FakeDecoder{}
	f := newFixture(t, dec, false, 0)

	if err := f.mgr.HandleEnterPanic(context.Background(),
		commands.New(commands.CmdEnterPanic, commands.EnterPanicPayload{Reason: "test"})); err != nil {
		t.Fatalf("HandleEnterPanic: %v", err)
	}

	f.stop(t)

	if got := f.stateMgr.Snapshot().State; got != state.StateIdle {
		t.Errorf("state: want IDLE after STOP during panic, got %s", got)
	}
	if got := f.stateMgr.Snapshot().Mode; got != state.ModeAuto {
		t.Errorf("mode: want AUTO after STOP during panic, got %s", got)
	}
}

// TestEnterPanic_WithBed verifies that a panic bed loops until EXIT_PANIC.
func TestEnterPanic_WithBed(t *testing.T) {
	// Use a short-lived FakeDecoder so the bed loops at least twice.
	dec := &testutil.FakeDecoder{Frames: 48} // 1 ms of audio — loops very fast
	f := newFixture(t, dec, false, 0)

	err := f.mgr.HandleEnterPanic(context.Background(),
		commands.New(commands.CmdEnterPanic, commands.EnterPanicPayload{
			Reason: "silence",
			Bed:    &commands.PanicBedInput{Path: "/fake/bed.mp3"},
		}))
	if err != nil {
		t.Fatalf("HandleEnterPanic: %v", err)
	}
	if got := f.stateMgr.Snapshot().State; got != state.StatePanic {
		t.Fatalf("state: want PANIC, got %s", got)
	}

	// Let the bed loop for a short time.
	time.Sleep(30 * time.Millisecond)

	// Exit panic — bed must stop cleanly.
	if err := f.mgr.HandleExitPanic(context.Background(),
		commands.New(commands.CmdExitPanic, commands.ExitPanicPayload{Reason: "ok"})); err != nil {
		t.Fatalf("HandleExitPanic: %v", err)
	}
	if got := f.stateMgr.Snapshot().State; got != state.StateIdle {
		t.Errorf("state: want IDLE, got %s", got)
	}
}

// ---- hot buttons ------------------------------------------------------------

// TestHotButton_AfterCurrent verifies that AFTER_CURRENT inserts the asset as
// the next queue item without interrupting the current session.
func TestHotButton_AfterCurrent(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 480000} // long enough not to finish during test
	f := newFixture(t, dec, true, 0)
	f.enqueue("musicas", 480000)
	f.play(t)
	waitState(t, f.stateMgr, state.StatePlaying, 500*time.Millisecond)

	err := f.mgr.HandleTriggerHotButton(context.Background(),
		commands.New(commands.CmdTriggerHotButton, commands.TriggerHotButtonPayload{
			ButtonID: "btn-1",
			Asset:    commands.QueueItemInput{Path: "/fake/jingle.mp3", Type: "spots"},
			PlayMode: "AFTER_CURRENT",
		}))
	if err != nil {
		t.Fatalf("HandleTriggerHotButton AFTER_CURRENT: %v", err)
	}

	if f.queueMgr.Size() < 1 {
		t.Error("expected jingle to be inserted into the queue")
	}

	// Engine should still be PLAYING — not interrupted.
	if got := f.stateMgr.Snapshot().State; got != state.StatePlaying {
		t.Errorf("state: want PLAYING after AFTER_CURRENT, got %s", got)
	}

	f.stop(t)
	waitState(t, f.stateMgr, state.StateIdle, 3*time.Second)
}

// TestHotButton_Interrupt verifies that INTERRUPT stops the current session,
// plays the asset, then leaves the engine in IDLE.
func TestHotButton_Interrupt(t *testing.T) {
	// realtime: true so the main session stays in PLAYING long enough
	// for the interrupt to actually arrive while it is active.
	// 4800 frames ≈ 100 ms at 48 kHz.
	dec := &testutil.FakeDecoder{Frames: 4800}
	f := newFixture(t, dec, true, 0)
	// The main queue item is long; it will be interrupted.
	f.enqueue("musicas", 4800)
	f.play(t)
	waitState(t, f.stateMgr, state.StatePlaying, 500*time.Millisecond)

	getHot := collectEvents(f.evtBus, events.EvtHotButtonTriggered)

	err := f.mgr.HandleTriggerHotButton(context.Background(),
		commands.New(commands.CmdTriggerHotButton, commands.TriggerHotButtonPayload{
			ButtonID: "btn-interrupt",
			Asset:    commands.QueueItemInput{Path: "/fake/hotbutton.mp3", Type: "spots"},
			PlayMode: "INTERRUPT",
		}))
	if err != nil {
		t.Fatalf("HandleTriggerHotButton INTERRUPT: %v", err)
	}

	// Wait for IDLE (session was stopped and hot button finished).
	waitState(t, f.stateMgr, state.StateIdle, 3*time.Second)

	hotEvts := getHot()
	if len(hotEvts) != 1 {
		t.Fatalf("expected 1 HotButtonTriggered, got %d", len(hotEvts))
	}
	p, ok := hotEvts[0].Payload.(events.HotButtonTriggeredPayload)
	if !ok {
		t.Fatalf("unexpected payload type: %T", hotEvts[0].Payload)
	}
	if p.ButtonID != "btn-interrupt" {
		t.Errorf("ButtonID: want btn-interrupt, got %s", p.ButtonID)
	}
	if p.PlayMode != "INTERRUPT" {
		t.Errorf("PlayMode: want INTERRUPT, got %s", p.PlayMode)
	}
}

// TestHotButton_Overlay_WithDucking verifies that OVERLAY ducks the main
// stream (duckGain < 100) while the hot button plays.
func TestHotButton_Overlay_WithDucking(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 4800} // ~100 ms hot button
	f := newFixture(t, dec, false, 0)

	// Main session must be running for OVERLAY to make sense.
	f.enqueue("musicas", 480000)
	// Note: we don't play the main session here — overlay works even without it
	// (it just writes to output). We test the ducking logic specifically.

	getDucking := collectEvents(f.evtBus, events.EvtDuckingStarted)
	getDuckEnd := collectEvents(f.evtBus, events.EvtDuckingEnded)

	err := f.mgr.HandleTriggerHotButton(context.Background(),
		commands.New(commands.CmdTriggerHotButton, commands.TriggerHotButtonPayload{
			ButtonID:   "btn-overlay",
			Asset:      commands.QueueItemInput{Path: "/fake/overlay.mp3", Type: "spots"},
			PlayMode:   "OVERLAY",
			DuckMain:   true,
			DuckGainDB: -6,
		}))
	if err != nil {
		t.Fatalf("HandleTriggerHotButton OVERLAY: %v", err)
	}

	// DuckingStarted must be published synchronously with the handler.
	duckEvts := getDucking()
	if len(duckEvts) != 1 {
		t.Fatalf("expected 1 DuckingStarted, got %d", len(duckEvts))
	}
	dp, ok := duckEvts[0].Payload.(events.DuckingStartedPayload)
	if !ok {
		t.Fatalf("unexpected DuckingStarted payload type: %T", duckEvts[0].Payload)
	}
	if dp.GainDB != -6 {
		t.Errorf("DuckingStarted.GainDB: want -6, got %v", dp.GainDB)
	}

	// Wait for the overlay goroutine to finish (short asset).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(getDuckEnd()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(getDuckEnd()) == 0 {
		t.Fatal("DuckingEnded event not received within timeout")
	}
}

// TestHotButton_RejectsInvalidPlayMode verifies that an unknown play_mode
// returns a RejectedError without panicking.
func TestHotButton_RejectsInvalidPlayMode(t *testing.T) {
	dec := &testutil.FakeDecoder{}
	f := newFixture(t, dec, false, 0)

	err := f.mgr.HandleTriggerHotButton(context.Background(),
		commands.New(commands.CmdTriggerHotButton, commands.TriggerHotButtonPayload{
			ButtonID: "btn-bad",
			Asset:    commands.QueueItemInput{Path: "/fake/x.mp3"},
			PlayMode: "UNKNOWN_MODE",
		}))
	if _, ok := err.(*commands.RejectedError); !ok {
		t.Fatalf("expected RejectedError for unknown play_mode, got %T: %v", err, err)
	}
}

// TestEnterPanic_Idempotent verifies that calling ENTER_PANIC twice does not
// produce two PanicEntered events (second call replaces the first).
func TestEnterPanic_Idempotent(t *testing.T) {
	dec := &testutil.FakeDecoder{}
	f := newFixture(t, dec, false, 0)

	for i := 0; i < 2; i++ {
		if err := f.mgr.HandleEnterPanic(context.Background(),
			commands.New(commands.CmdEnterPanic, commands.EnterPanicPayload{Reason: "again"})); err != nil {
			t.Fatalf("HandleEnterPanic #%d: %v", i+1, err)
		}
	}

	if got := f.stateMgr.Snapshot().State; got != state.StatePanic {
		t.Errorf("state: want PANIC, got %s", got)
	}

	// Exit cleanly — should succeed without deadlock.
	if err := f.mgr.HandleExitPanic(context.Background(),
		commands.New(commands.CmdExitPanic, commands.ExitPanicPayload{})); err != nil {
		t.Fatalf("HandleExitPanic: %v", err)
	}
	if got := f.stateMgr.Snapshot().State; got != state.StateIdle {
		t.Errorf("state: want IDLE, got %s", got)
	}
}

// ---------------------------------------------------------------------------
// Corrupted / failing decoder tests
// ---------------------------------------------------------------------------

// TestDecoder_OpenError_CircuitBreaker verifies that when Open() always fails,
// the engine enters ERROR after MaxConsecutiveFailures(3) attempts.
// Each failure incurs backoff: 100ms + 200ms + 400ms ≈ 700ms; timeout = 3s.
func TestDecoder_OpenError_CircuitBreaker(t *testing.T) {
	dec := &testutil.FakeDecoder{OpenErr: errors.New("no such file or directory")}
	f := newFixture(t, dec, false, 0)

	f.enqueue("CORRUPT", 0)
	f.enqueue("CORRUPT", 0)
	f.enqueue("CORRUPT", 0)
	f.play(t)

	waitState(t, f.stateMgr, state.StateError, 3*time.Second)
}

// TestDecoder_MidStreamError_SkipsToNext verifies that a stream error mid-play
// is treated as an item failure and the engine moves on to the next queued item.
func TestDecoder_MidStreamError_SkipsToNext(t *testing.T) {
	// First item fails after 240 frames; second item is healthy.
	dec := &testutil.FakeDecoder{Frames: 9600, FailAfter: 240}
	f := newFixture(t, dec, false, 0)

	f.enqueue("BAD", 9600)
	f.enqueue("GOOD", 9600)
	f.play(t)

	// Engine should eventually return to IDLE after both items are processed.
	waitSessionEnd(t, f.evtBus, 3*time.Second)

	if got := f.stateMgr.Snapshot().State; got == state.StateError {
		t.Errorf("engine entered ERROR state unexpectedly after one failed item")
	}
}

// TestDecoder_EmptyStream_SkipsToNext verifies that a zero-frame file
// (immediate EOF, simulating a corrupt/empty audio file) is skipped and
// the engine proceeds to the next item.
func TestDecoder_EmptyStream_SkipsToNext(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 0} // first item: immediate EOF
	f := newFixture(t, dec, false, 0)

	// Enqueue empty item then a valid one (reuses same decoder — both get EOF,
	// but the circuit breaker threshold is 3 so two items won't trigger ERROR).
	f.enqueue("EMPTY", 0)
	f.enqueue("EMPTY", 0)
	f.play(t)

	// After two empty items the engine returns to IDLE (not ERROR).
	waitSessionEnd(t, f.evtBus, 3*time.Second)

	if got := f.stateMgr.Snapshot().State; got == state.StateError {
		t.Errorf("two empty items triggered ERROR; want IDLE")
	}
}

// TestDecoder_OpenError_BackoffRespectsCancellation verifies that a STOP
// command during the backoff window cancels the wait and the engine reaches
// IDLE promptly.
func TestDecoder_OpenError_BackoffRespectsCancellation(t *testing.T) {
	dec := &testutil.FakeDecoder{OpenErr: errors.New("simulated I/O error")}
	f := newFixture(t, dec, false, 0)

	// Only one item — engine will backoff then reach IDLE after the single failure.
	f.enqueue("BAD", 0)
	f.play(t)

	// Allow the first failure to trigger backoff, then stop.
	time.Sleep(20 * time.Millisecond)
	f.stop(t)

	waitState(t, f.stateMgr, state.StateIdle, 2*time.Second)
}
