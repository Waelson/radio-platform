package scheduler

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
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
	mu sync.Mutex
	st state.PlayerState
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

// cmdCollector wraps commands.Bus and exposes a drain helper for assertions.
type cmdCollector struct {
	*commands.Bus
}

func newCollector() *cmdCollector {
	return &cmdCollector{Bus: commands.NewBus()}
}

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
// cfg is merged on top of a zero Config (no store, 5s missed threshold).
func newTestManager(fs *fakeState, clk *fakeClock, cfg ...Config) (*Manager, *cmdCollector) {
	c := Config{MissedThresholdMS: 5000}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	col := newCollector()
	evtBus := events.NewBus(slog.Default())
	m, err := New(c, col.Bus, evtBus, fs, slog.Default())
	if err != nil {
		panic("newTestManager: " + err.Error())
	}
	m.withClock(clk)
	return m, col
}

// --- fire logic tests --------------------------------------------------------

func TestFireInterrupt_WhenPlaying(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	fired := m.fireEntry(&Entry{
		ID: "e1", Name: "test", Enabled: true,
		TriggerMode: TriggerInterrupt,
		Item:        commands.QueueItemInput{AssetID: "a1", Title: "Song A"},
	})
	if !fired {
		t.Fatal("expected fired=true")
	}
	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands (InsertNext+Skip), got %d", len(cmds))
	}
	if cmds[0].Type != commands.CmdInsertNext {
		t.Errorf("cmds[0]=%s, want CmdInsertNext", cmds[0].Type)
	}
	if cmds[1].Type != commands.CmdSkip {
		t.Errorf("cmds[1]=%s, want CmdSkip", cmds[1].Type)
	}
}

func TestFireInterrupt_WhenIdle(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	m.fireEntry(&Entry{ID: "e2", Enabled: true, TriggerMode: TriggerInterrupt,
		Item: commands.QueueItemInput{AssetID: "a1"}})

	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 (InsertNext+Play), got %d", len(cmds))
	}
	if cmds[1].Type != commands.CmdPlay {
		t.Errorf("cmds[1]=%s, want CmdPlay", cmds[1].Type)
	}
}

func TestFireAfterCurrent_WhenPlaying(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	m.fireEntry(&Entry{ID: "e3", Enabled: true, TriggerMode: TriggerAfterCurrent,
		Item: commands.QueueItemInput{AssetID: "a1"}})

	cmds := col.drainAll()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command (InsertNext only), got %d", len(cmds))
	}
	if cmds[0].Type != commands.CmdInsertNext {
		t.Errorf("cmds[0]=%s, want CmdInsertNext", cmds[0].Type)
	}
}

func TestFireAfterCurrent_WhenIdle(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	m.fireEntry(&Entry{ID: "e4", Enabled: true, TriggerMode: TriggerAfterCurrent,
		Item: commands.QueueItemInput{AssetID: "a1"}})

	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 (InsertNext+Play), got %d", len(cmds))
	}
	if cmds[1].Type != commands.CmdPlay {
		t.Errorf("cmds[1]=%s, want CmdPlay", cmds[1].Type)
	}
}

func TestFireCrossfade_WhenPlaying(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	m.fireEntry(&Entry{ID: "e5", Enabled: true, TriggerMode: TriggerCrossfade,
		Item: commands.QueueItemInput{AssetID: "a1"}})

	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	skip, ok := cmds[1].Payload.(commands.SkipPayload)
	if !ok {
		t.Fatal("cmds[1] should be SkipPayload")
	}
	if skip.Transition == nil || skip.Transition.Type != "CROSSFADE" {
		t.Errorf("expected CROSSFADE transition, got %+v", skip.Transition)
	}
}

func TestFireSkipIfBusy_WhenPlaying_Missed(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	fired := m.fireEntry(&Entry{ID: "e6", Enabled: true, TriggerMode: TriggerSkipIfBusy,
		Item: commands.QueueItemInput{AssetID: "a1"}})

	if fired {
		t.Fatal("expected fired=false (missed)")
	}
	if cmds := col.drainAll(); len(cmds) != 0 {
		t.Fatalf("expected 0 commands when missed, got %d", len(cmds))
	}
}

func TestFireSkipIfBusy_WhenIdle_Fires(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	fired := m.fireEntry(&Entry{ID: "e7", Enabled: true, TriggerMode: TriggerSkipIfBusy,
		Item: commands.QueueItemInput{AssetID: "a1"}})

	if !fired {
		t.Fatal("expected fired=true")
	}
	if len(col.drainAll()) != 2 {
		t.Fatal("expected 2 commands (InsertNext+Play)")
	}
}

func TestFire_PanicState_AlwaysMissed(t *testing.T) {
	for _, mode := range []TriggerMode{TriggerInterrupt, TriggerAfterCurrent, TriggerCrossfade, TriggerSkipIfBusy} {
		fs := &fakeState{st: state.StatePanic}
		m, col := newTestManager(fs, newFakeClock(time.Now()))

		fired := m.fireEntry(&Entry{ID: "ep", Enabled: true, TriggerMode: mode,
			Item: commands.QueueItemInput{AssetID: "a1"}})

		if fired {
			t.Errorf("mode=%s: expected fired=false in PANIC", mode)
		}
		if cmds := col.drainAll(); len(cmds) != 0 {
			t.Errorf("mode=%s: expected 0 commands in PANIC, got %d", mode, len(cmds))
		}
	}
}

// --- Manager lifecycle tests -------------------------------------------------

func TestAdd_Remove(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	m, _ := newTestManager(fs, newFakeClock(time.Now()))

	id, err := m.Add(Entry{
		Name: "jingle", Enabled: true,
		CronExpr: "0 * * * *", TriggerMode: TriggerAfterCurrent,
		Item: commands.QueueItemInput{AssetID: "j1"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
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
	m, _ := newTestManager(fs, newFakeClock(time.Now()))

	_, err := m.Add(Entry{Name: "bad", Enabled: true, CronExpr: "not a valid cron"})
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

func TestEnable_Disable(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	m, _ := newTestManager(fs, newFakeClock(time.Now()))

	id, _ := m.Add(Entry{
		Name: "e", Enabled: true, CronExpr: "0 * * * *",
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

// --- FireAt tick tests -------------------------------------------------------

func TestFireAt_TickFires(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := newFakeClock(base)
	m, col := newTestManager(fs, clk)

	fireTime := base.Add(2 * time.Second)
	_, err := m.Add(Entry{
		Name: "one-shot", Enabled: true, FireAt: &fireTime,
		TriggerMode: TriggerAfterCurrent,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// t=0: before fireTime — must not fire.
	m.tickFireAt()
	if cmds := col.drainAll(); len(cmds) != 0 {
		t.Fatalf("t=0: expected 0 commands, got %d", len(cmds))
	}

	// t=2s: at fireTime — must fire.
	clk.advance(2 * time.Second)
	m.tickFireAt()
	if cmds := col.drainAll(); len(cmds) == 0 {
		t.Fatal("t=2s: expected commands after FireAt")
	}

	// t=3s: second tick — entry is disabled, must NOT fire again.
	clk.advance(time.Second)
	m.tickFireAt()
	if cmds := col.drainAll(); len(cmds) != 0 {
		t.Fatalf("t=3s: one-shot re-fired (want 0 commands), got %d", len(cmds))
	}

	// Entry must be disabled.
	entries := m.List()
	if len(entries) == 0 {
		t.Fatal("entry should still exist (just disabled)")
	}
	if entries[0].Enabled {
		t.Error("one-shot entry should be disabled after firing")
	}
}

func TestFireAt_MissedThreshold(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := newFakeClock(base)
	// threshold = 5s
	m, col := newTestManager(fs, clk, Config{MissedThresholdMS: 5000})

	fireTime := base.Add(1 * time.Second) // fire at +1s
	_, err := m.Add(Entry{
		Name: "late-shot", Enabled: true, FireAt: &fireTime,
		TriggerMode: TriggerAfterCurrent,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Advance clock by 10s — 9s past fireTime, well beyond 5s threshold.
	clk.advance(10 * time.Second)
	m.tickFireAt()

	// Must NOT have sent any commands (marked as MISSED, not fired).
	if cmds := col.drainAll(); len(cmds) != 0 {
		t.Fatalf("expected 0 commands (missed), got %d", len(cmds))
	}

	// Entry must be disabled.
	entries := m.List()
	if entries[0].Enabled {
		t.Error("missed entry should be disabled")
	}
	if entries[0].LastFiredAt.IsZero() {
		t.Error("LastFiredAt should be set even when entry is missed")
	}
}

func TestFireAt_NoThreshold_FiresLate(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := newFakeClock(base)
	// threshold = 0 means always fire, no matter how late.
	m, col := newTestManager(fs, clk, Config{MissedThresholdMS: 0})

	fireTime := base.Add(1 * time.Second)
	m.Add(Entry{ //nolint
		Name: "no-threshold", Enabled: true, FireAt: &fireTime,
		TriggerMode: TriggerAfterCurrent,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	})

	clk.advance(60 * time.Second) // 59s late
	m.tickFireAt()

	if cmds := col.drainAll(); len(cmds) == 0 {
		t.Fatal("expected commands to fire (threshold=0 means always fire)")
	}
}

// --- Store tests -------------------------------------------------------------

func TestStore_SaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")
	store := NewFileStore(path)

	fireAt := time.Date(2026, 7, 7, 20, 0, 0, 0, time.UTC)
	original := []Entry{
		{
			ID: "sched_001", Name: "Test", Enabled: true,
			CronExpr: "0 10 * * *", TriggerMode: TriggerCrossfade,
			Item:      commands.QueueItemInput{AssetID: "a1", Title: "Song"},
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "sched_002", Name: "OneShot", Enabled: false,
			FireAt: &fireAt, TriggerMode: TriggerInterrupt,
			Item:        commands.QueueItemInput{AssetID: "a2"},
			LastFiredAt: time.Date(2026, 7, 7, 20, 0, 1, 0, time.UTC),
		},
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != len(original) {
		t.Fatalf("expected %d entries, got %d", len(original), len(loaded))
	}
	if loaded[0].ID != original[0].ID {
		t.Errorf("entry[0].ID = %q, want %q", loaded[0].ID, original[0].ID)
	}
	if loaded[1].FireAt == nil {
		t.Error("entry[1].FireAt should not be nil after round-trip")
	}
	if !loaded[1].FireAt.Equal(fireAt) {
		t.Errorf("entry[1].FireAt = %v, want %v", loaded[1].FireAt, fireAt)
	}
}

func TestStore_Load_FileNotExist(t *testing.T) {
	store := NewFileStore("/tmp/scheduler_nonexistent_test_" + time.Now().Format("20060102150405") + ".json")
	entries, err := store.Load()
	if err != nil {
		t.Fatalf("Load of missing file should return nil error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("Load of missing file should return nil entries, got %v", entries)
	}
}

func TestStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")
	store := NewFileStore(path)

	entries := []Entry{{ID: "x1", Name: "Atomic", Enabled: true}}
	if err := store.Save(entries); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// .tmp file must not remain after a successful save.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after successful save")
	}

	// Verify final file is valid JSON.
	data, _ := os.ReadFile(path)
	var doc storeDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if doc.Version != 1 {
		t.Errorf("version = %d, want 1", doc.Version)
	}
}

// --- Manager persist-on-mutate tests -----------------------------------------

func TestManager_PersistsOnAdd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")
	fs := &fakeState{st: state.StateIdle}
	m, _ := newTestManager(fs, newFakeClock(time.Now()), Config{
		StorePath:         path,
		MissedThresholdMS: 5000,
	})

	m.Add(Entry{ //nolint
		Name: "persist-me", Enabled: true, CronExpr: "0 * * * *",
		TriggerMode: TriggerAfterCurrent,
		Item:        commands.QueueItemInput{AssetID: "a1"},
	})

	entries, err := NewFileStore(path).Load()
	if err != nil {
		t.Fatalf("Load after Add: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 persisted entry, got %d", len(entries))
	}
	if entries[0].Name != "persist-me" {
		t.Errorf("persisted name = %q, want %q", entries[0].Name, "persist-me")
	}
}

func TestManager_RestoresFromStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")

	// Pre-populate the store.
	store := NewFileStore(path)
	if err := store.Save([]Entry{
		{ID: "sched_restored", Name: "Restored", Enabled: true,
			CronExpr: "0 * * * *", TriggerMode: TriggerAfterCurrent,
			Item: commands.QueueItemInput{AssetID: "a1"}},
	}); err != nil {
		t.Fatalf("pre-populate store: %v", err)
	}

	// Create a new Manager pointing at the same store.
	fs := &fakeState{st: state.StateIdle}
	m, _ := newTestManager(fs, newFakeClock(time.Now()), Config{
		StorePath:         path,
		MissedThresholdMS: 5000,
	})

	entries := m.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 restored entry, got %d", len(entries))
	}
	if entries[0].ID != "sched_restored" {
		t.Errorf("restored ID = %q, want sched_restored", entries[0].ID)
	}
}

// --- HORA_CERTA tests --------------------------------------------------------

// TestFireHoraCerta confirms that an entry with item.Type="HORA_CERTA" and
// empty path is accepted by Add and fires a CmdInsertNext with the correct item.
func TestFireHoraCerta_Add_AcceptsEmptyPath(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	m, _ := newTestManager(fs, newFakeClock(time.Now()))

	_, err := m.Add(Entry{
		Name:        "Hora Certa",
		Enabled:     true,
		CronExpr:    "0 * * * *",
		TriggerMode: TriggerInterrupt,
		Item: commands.QueueItemInput{
			Type:  "HORA_CERTA",
			Title: "Hora Certa",
			// Path intentionally empty — resolved by the playback manager at play time.
		},
	})
	if err != nil {
		t.Fatalf("Add HORA_CERTA entry: unexpected error: %v", err)
	}
}

// TestFireHoraCerta_Fire_SendsInsertNext confirms that firing a HORA_CERTA entry
// dispatches CmdInsertNext with type=HORA_CERTA and an empty path.
func TestFireHoraCerta_Fire_SendsInsertNext(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	fired := m.fireEntry(&Entry{
		ID:          "hc1",
		Name:        "Hora Certa",
		Enabled:     true,
		TriggerMode: TriggerInterrupt,
		Item: commands.QueueItemInput{
			Type:  "HORA_CERTA",
			Title: "Hora Certa",
		},
	})

	if !fired {
		t.Fatal("expected fired=true")
	}

	cmds := col.drainAll()
	// INTERRUPT + IDLE → CmdInsertNext + CmdPlay
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands (InsertNext+Play), got %d", len(cmds))
	}

	insertCmd := cmds[0]
	if insertCmd.Type != commands.CmdInsertNext {
		t.Fatalf("cmds[0] = %s, want CmdInsertNext", insertCmd.Type)
	}

	payload, ok := insertCmd.Payload.(commands.InsertNextPayload)
	if !ok {
		t.Fatalf("payload type = %T, want InsertNextPayload", insertCmd.Payload)
	}
	if payload.Item.Type != "HORA_CERTA" {
		t.Errorf("item.Type = %q, want HORA_CERTA", payload.Item.Type)
	}
	if payload.Item.Path != "" {
		t.Errorf("item.Path = %q, want empty (resolved at play time)", payload.Item.Path)
	}
}


// --- Break entry fire tests --------------------------------------------------

func makeBreak() *commands.BreakItemInput {
	return &commands.BreakItemInput{
		Title: "Bloco Teste",
		Open:  &commands.QueueItemInput{Path: "/open.mp3", Type: "jingle"},
		Spots: []commands.QueueItemInput{
			{Path: "/spot-a.mp3", Type: "spot", Title: "Spot A"},
			{Path: "/spot-b.mp3", Type: "spot", Title: "Spot B"},
		},
		Close: &commands.QueueItemInput{Path: "/close.mp3", Type: "jingle"},
	}
}

// TestSchedulerBreak_AfterCurrent confirms that a break entry with
// TriggerAfterCurrent dispatches CmdInsertBreakNext (not CmdInsertNext).
func TestSchedulerBreak_AfterCurrent(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	fired := m.fireEntry(&Entry{
		ID:          "b1",
		Name:        "bloco",
		Enabled:     true,
		TriggerMode: TriggerAfterCurrent,
		Break:       makeBreak(),
	})
	if !fired {
		t.Fatal("expected fired=true")
	}
	cmds := col.drainAll()
	// AFTER_CURRENT + PLAYING → only InsertBreakNext (no skip, no play)
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Type != commands.CmdInsertBreakNext {
		t.Errorf("cmd = %s, want CmdInsertBreakNext", cmds[0].Type)
	}
	p, ok := cmds[0].Payload.(commands.InsertBreakNextPayload)
	if !ok {
		t.Fatalf("payload type = %T, want InsertBreakNextPayload", cmds[0].Payload)
	}
	if p.Break.Title != "Bloco Teste" {
		t.Errorf("break.Title = %q, want 'Bloco Teste'", p.Break.Title)
	}
	if len(p.Break.Spots) != 2 {
		t.Errorf("break.Spots len = %d, want 2", len(p.Break.Spots))
	}
}

// TestSchedulerBreak_Interrupt confirms break + INTERRUPT → CmdInsertBreakNext + CmdSkip.
func TestSchedulerBreak_Interrupt(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	fired := m.fireEntry(&Entry{
		ID:          "b2",
		Name:        "bloco-interrupt",
		Enabled:     true,
		TriggerMode: TriggerInterrupt,
		Break:       makeBreak(),
	})
	if !fired {
		t.Fatal("expected fired=true")
	}
	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands (InsertBreakNext+Skip), got %d", len(cmds))
	}
	if cmds[0].Type != commands.CmdInsertBreakNext {
		t.Errorf("cmds[0] = %s, want CmdInsertBreakNext", cmds[0].Type)
	}
	if cmds[1].Type != commands.CmdSkip {
		t.Errorf("cmds[1] = %s, want CmdSkip", cmds[1].Type)
	}
}

// TestSchedulerBreak_Crossfade confirms break + CROSSFADE → CmdInsertBreakNext + CmdSkip{CROSSFADE}.
func TestSchedulerBreak_Crossfade(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	fired := m.fireEntry(&Entry{
		ID:          "b3",
		Name:        "bloco-crossfade",
		Enabled:     true,
		TriggerMode: TriggerCrossfade,
		Break:       makeBreak(),
	})
	if !fired {
		t.Fatal("expected fired=true")
	}
	cmds := col.drainAll()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[0].Type != commands.CmdInsertBreakNext {
		t.Errorf("cmds[0] = %s, want CmdInsertBreakNext", cmds[0].Type)
	}
	skip, ok := cmds[1].Payload.(commands.SkipPayload)
	if !ok {
		t.Fatal("cmds[1] should be SkipPayload")
	}
	if skip.Transition == nil || skip.Transition.Type != "CROSSFADE" {
		t.Errorf("expected CROSSFADE transition, got %+v", skip.Transition)
	}
}

// TestSchedulerBreak_SkipIfBusy_Missed confirms break + SKIP_IF_BUSY when PLAYING → MISSED.
func TestSchedulerBreak_SkipIfBusy_Missed(t *testing.T) {
	fs := &fakeState{st: state.StatePlaying}
	m, col := newTestManager(fs, newFakeClock(time.Now()))

	fired := m.fireEntry(&Entry{
		ID:          "b4",
		Name:        "bloco-skip",
		Enabled:     true,
		TriggerMode: TriggerSkipIfBusy,
		Break:       makeBreak(),
	})
	if fired {
		t.Fatal("expected fired=false (missed when busy)")
	}
	if cmds := col.drainAll(); len(cmds) != 0 {
		t.Fatalf("expected 0 commands when missed, got %d", len(cmds))
	}
}

// TestSchedulerBreak_Persist confirms that an entry with Break persists and
// restores correctly via FileStore (round-trip through JSON).
func TestSchedulerBreak_Persist(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/schedule.json"

	fs := &fakeState{st: state.StateIdle}
	m, _ := newTestManager(fs, newFakeClock(time.Now()), Config{
		StorePath:         path,
		MissedThresholdMS: 5000,
	})

	_, err := m.Add(Entry{
		Name:        "Bloco Persistido",
		Enabled:     true,
		CronExpr:    "30 10 * * *",
		TriggerMode: TriggerAfterCurrent,
		Break:       makeBreak(),
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Reload from disk via a new Manager.
	m2, _ := newTestManager(fs, newFakeClock(time.Now()), Config{
		StorePath:         path,
		MissedThresholdMS: 5000,
	})
	entries := m2.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 restored entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Break == nil {
		t.Fatal("restored entry should have Break set")
	}
	if e.Break.Title != "Bloco Teste" {
		t.Errorf("Break.Title = %q, want 'Bloco Teste'", e.Break.Title)
	}
	if len(e.Break.Spots) != 2 {
		t.Errorf("Break.Spots len = %d, want 2", len(e.Break.Spots))
	}
	if e.Break.Open == nil || e.Break.Close == nil {
		t.Error("Break.Open and Break.Close should be non-nil after restore")
	}
}

// --- Timezone and Run tests --------------------------------------------------

func TestNew_InvalidTimezone(t *testing.T) {
	evtBus := events.NewBus(slog.Default())
	_, err := New(Config{Timezone: "NotA/Timezone"}, commands.NewBus(), evtBus,
		&fakeState{}, slog.Default())
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestNew_ValidTimezone(t *testing.T) {
	evtBus := events.NewBus(slog.Default())
	_, err := New(Config{Timezone: "America/Sao_Paulo", MissedThresholdMS: 5000},
		commands.NewBus(), evtBus, &fakeState{}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error for valid timezone: %v", err)
	}
}

func TestRun_CancelStops(t *testing.T) {
	fs := &fakeState{st: state.StateIdle}
	m, _ := newTestManager(fs, newFakeClock(time.Now()))

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
		t.Fatal("Run did not exit within 2s after context cancellation")
	}
}
