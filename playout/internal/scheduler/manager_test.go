package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// --- test doubles ------------------------------------------------------------

// fakeState implements stateReader and allows tests to control the engine state.
type fakeState struct {
	mu  sync.Mutex
	st  state.PlayerState
}

func (f *fakeState) setState(s state.PlayerState) {
	f.mu.Lock()
	f.st = s
	f.mu.Unlock()
}

func (f *fakeState) Snapshot() state.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return state.Snapshot{State: f.st}
}

// fakeClock lets tests advance time deterministically.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// cmdCollector wraps commands.Bus and collects sent commands for assertions.
type cmdCollector struct {
	*commands.Bus
	mu   sync.Mutex
	cmds []commands.Command
}

func newCollector() *cmdCollector {
	c := &cmdCollector{Bus: commands.NewBus()}
	return c
}

// drainAll reads all buffered commands without blocking.
func (c *cmdCollector) drainAll() []commands.Command {
	var out []commands.Command
	for {
		select {
		case cmd := <-c.Bus.Receive():
			out = append(out, cmd)
		default:
			return out
		}
	}
}

// newTestManager builds a Manager wired to test doubles.
func newTestManager(fs *fakeState, clk *fakeClock) (*Manager, *cmdCollector) {
	col := newCollector()
	evtBus := events.NewBus(slog.Default())
	m := New(col.Bus, evtBus, fs, slog.Default())
	m.withClock(clk)
	return m, col
}

// --- fire logic tests --------------------------------------------------------

func TestFireInterrupt_WhenPlaying(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	clk := newFakeClock(time.Now())
	m, col := newTestManager(fs, clk)

	e := &Entry{
		ID:          "e1",
		Name:        "test",
		Enabled:     true,
		TriggerMode: TriggerInterrupt,
		Item:        commands.QueueItemInput{AssetID: "a1", Title: "Song A"},
	}

	fired := m.fireEntry(e)
	if !fired {
		t.Fatal("expected fired=true")
	}

	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands (InsertNext + Skip), got %d", len(cmds))
	}
	if cmds[0].Type != commands.CmdInsertNext {
		t.Errorf("cmds[0].Type = %s, want %s", cmds[0].Type, commands.CmdInsertNext)
	}
	if cmds[1].Type != commands.CmdSkip {
		t.Errorf("cmds[1].Type = %s, want %s", cmds[1].Type, commands.CmdSkip)
	}
}

func TestFireInterrupt_WhenIdle(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	clk := newFakeClock(time.Now())
	m, col := newTestManager(fs, clk)

	e := &Entry{
		ID:          "e2",
		Name:        "test",
		Enabled:     true,
		TriggerMode: TriggerInterrupt,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	}

	fired := m.fireEntry(e)
	if !fired {
		t.Fatal("expected fired=true")
	}

	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands (InsertNext + Play), got %d", len(cmds))
	}
	if cmds[1].Type != commands.CmdPlay {
		t.Errorf("cmds[1].Type = %s, want %s", cmds[1].Type, commands.CmdPlay)
	}
}

func TestFireAfterCurrent_WhenPlaying(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	clk := newFakeClock(time.Now())
	m, col := newTestManager(fs, clk)

	e := &Entry{
		ID:          "e3",
		Enabled:     true,
		TriggerMode: TriggerAfterCurrent,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	}

	fired := m.fireEntry(e)
	if !fired {
		t.Fatal("expected fired=true")
	}

	cmds := col.drainAll()
	// Only InsertNext — no skip, no play.
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command (InsertNext only), got %d", len(cmds))
	}
	if cmds[0].Type != commands.CmdInsertNext {
		t.Errorf("cmds[0].Type = %s, want %s", cmds[0].Type, commands.CmdInsertNext)
	}
}

func TestFireAfterCurrent_WhenIdle(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	clk := newFakeClock(time.Now())
	m, col := newTestManager(fs, clk)

	e := &Entry{
		ID:          "e4",
		Enabled:     true,
		TriggerMode: TriggerAfterCurrent,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	}

	m.fireEntry(e)
	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands (InsertNext + Play), got %d", len(cmds))
	}
	if cmds[1].Type != commands.CmdPlay {
		t.Errorf("cmds[1].Type = %s, want %s", cmds[1].Type, commands.CmdPlay)
	}
}

func TestFireCrossfade_WhenPlaying(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	clk := newFakeClock(time.Now())
	m, col := newTestManager(fs, clk)

	e := &Entry{
		ID:          "e5",
		Enabled:     true,
		TriggerMode: TriggerCrossfade,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	}

	m.fireEntry(e)
	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	skip, ok := cmds[1].Payload.(commands.SkipPayload)
	if !ok {
		t.Fatal("cmds[1].Payload should be SkipPayload")
	}
	if skip.Transition == nil || skip.Transition.Type != "CROSSFADE" {
		t.Errorf("expected CROSSFADE transition, got %+v", skip.Transition)
	}
}

func TestFireSkipIfBusy_WhenPlaying_Missed(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	clk := newFakeClock(time.Now())
	m, col := newTestManager(fs, clk)

	e := &Entry{
		ID:          "e6",
		Enabled:     true,
		TriggerMode: TriggerSkipIfBusy,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	}

	fired := m.fireEntry(e)
	if fired {
		t.Fatal("expected fired=false (missed)")
	}
	cmds := col.drainAll()
	if len(cmds) != 0 {
		t.Fatalf("expected 0 commands when missed, got %d", len(cmds))
	}
}

func TestFireSkipIfBusy_WhenIdle_Fires(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	clk := newFakeClock(time.Now())
	m, col := newTestManager(fs, clk)

	e := &Entry{
		ID:          "e7",
		Enabled:     true,
		TriggerMode: TriggerSkipIfBusy,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	}

	fired := m.fireEntry(e)
	if !fired {
		t.Fatal("expected fired=true")
	}
	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
}

func TestFire_PanicState_AlwaysMissed(t *testing.T) {
	for _, mode := range []TriggerMode{TriggerInterrupt, TriggerAfterCurrent, TriggerCrossfade, TriggerSkipIfBusy} {
		fs := &fakeState{st: state.StatePanic}
		clk := newFakeClock(time.Now())
		m, col := newTestManager(fs, clk)

		e := &Entry{
			ID:          "epanic",
			Enabled:     true,
			TriggerMode: mode,
			Item:        commands.QueueItemInput{AssetID: "a1"},
		}

		fired := m.fireEntry(e)
		if fired {
			t.Errorf("mode=%s: expected fired=false in PANIC state", mode)
		}
		if cmds := col.drainAll(); len(cmds) != 0 {
			t.Errorf("mode=%s: expected 0 commands in PANIC state, got %d", mode, len(cmds))
		}
	}
}

// --- Manager lifecycle tests -------------------------------------------------

func TestAdd_Remove(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	clk := newFakeClock(time.Now())
	m, _ := newTestManager(fs, clk)

	id, err := m.Add(Entry{
		Name:        "jingle",
		Enabled:     true,
		CronExpr:    "0 * * * *",
		TriggerMode: TriggerAfterCurrent,
		Item:        commands.QueueItemInput{AssetID: "j1"},
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	if _, ok := m.Get(id); !ok {
		t.Fatal("Get should find the added entry")
	}
	if len(m.List()) != 1 {
		t.Fatal("List should return 1 entry")
	}

	m.Remove(id)
	if _, ok := m.Get(id); ok {
		t.Fatal("Get should not find removed entry")
	}
	if len(m.List()) != 0 {
		t.Fatal("List should be empty after Remove")
	}
}

func TestAdd_InvalidCronExpr(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	clk := newFakeClock(time.Now())
	m, _ := newTestManager(fs, clk)

	_, err := m.Add(Entry{
		Name:     "bad",
		Enabled:  true,
		CronExpr: "not a cron expression",
	})
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

func TestEnable_Disable(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	clk := newFakeClock(time.Now())
	m, _ := newTestManager(fs, clk)

	id, _ := m.Add(Entry{
		Name:        "e",
		Enabled:     true,
		CronExpr:    "0 * * * *",
		TriggerMode: TriggerAfterCurrent,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	})

	m.Disable(id)
	e, _ := m.Get(id)
	if e.Enabled {
		t.Error("expected Enabled=false after Disable")
	}

	m.Enable(id)
	e, _ = m.Get(id)
	if !e.Enabled {
		t.Error("expected Enabled=true after Enable")
	}
}

// TestFireAt_TickFires checks that a one-shot FireAt entry fires once when the
// clock crosses its target time.
func TestFireAt_TickFires(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := newFakeClock(base)
	m, col := newTestManager(fs, clk)

	fireTime := base.Add(2 * time.Second)
	_, err := m.Add(Entry{
		Name:        "one-shot",
		Enabled:     true,
		FireAt:      &fireTime,
		TriggerMode: TriggerAfterCurrent,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// t=0: before fireTime — should not fire.
	m.tickFireAt()
	if cmds := col.drainAll(); len(cmds) != 0 {
		t.Fatalf("t=0: expected 0 commands, got %d", len(cmds))
	}

	// t=2s: at fireTime — should fire.
	clk.advance(2 * time.Second)
	m.tickFireAt()
	cmds := col.drainAll()
	if len(cmds) == 0 {
		t.Fatal("t=2s: expected commands to be sent after FireAt")
	}

	// t=3s: second tick — entry is disabled, must NOT fire again.
	clk.advance(time.Second)
	m.tickFireAt()
	if cmds := col.drainAll(); len(cmds) != 0 {
		t.Fatalf("t=3s: one-shot entry fired again (expected 0 commands), got %d", len(cmds))
	}

	// Entry should now be disabled.
	entries := m.List()
	if len(entries) == 0 {
		t.Fatal("entry should still exist (just disabled)")
	}
	if entries[0].Enabled {
		t.Error("one-shot entry should be disabled after firing")
	}
}

// TestRun_CancelStops verifies Run exits promptly on context cancellation.
func TestRun_CancelStops(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	clk := newFakeClock(time.Now())
	m, _ := newTestManager(fs, clk)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Run(ctx)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit within 2 seconds after context cancellation")
	}
}
