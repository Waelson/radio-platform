package scheduler_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/scheduler"
)

// ── Stubs ─────────────────────────────────────────────────────────────────────

type stubClocks struct {
	clock *scheduler.Clock
}

func (s *stubClocks) GetClockForHour(_ context.Context, _, _ int) (*scheduler.Clock, error) {
	return s.clock, nil
}

type stubTracks struct {
	byCategory map[string][]scheduler.TrackRef
	byType     map[string][]scheduler.TrackRef
	byID       map[string]scheduler.TrackRef
}

func (s *stubTracks) TracksByCategory(_ context.Context, categoryID string) ([]scheduler.TrackRef, error) {
	return s.byCategory[categoryID], nil
}

func (s *stubTracks) TracksByType(_ context.Context, trackType string) ([]scheduler.TrackRef, error) {
	return s.byType[trackType], nil
}

func (s *stubTracks) TrackByID(_ context.Context, id string) (scheduler.TrackRef, error) {
	t, ok := s.byID[id]
	if !ok {
		return scheduler.TrackRef{}, errors.New("not found")
	}
	return t, nil
}

type stubSepRules struct {
	rules []scheduler.SeparationRule
}

func (s *stubSepRules) ListRules(_ context.Context) ([]scheduler.SeparationRule, error) {
	return s.rules, nil
}

type stubRotLog struct {
	recentIDs map[string]time.Time
	oldest    map[string]string
}

func (s *stubRotLog) RecentTrackIDs(_ context.Context, _ time.Time) (map[string]time.Time, error) {
	if s.recentIDs == nil {
		return map[string]time.Time{}, nil
	}
	return s.recentIDs, nil
}

func (s *stubRotLog) RecentByField(_ context.Context, _, _ string, _ time.Time) ([]scheduler.LogEntry, error) {
	return nil, nil
}

func (s *stubRotLog) OldestInCategory(_ context.Context, categoryID string) (string, error) {
	if s.oldest == nil {
		return "", nil
	}
	return s.oldest[categoryID], nil
}

// ── Helper ────────────────────────────────────────────────────────────────────

func makeTrack(id, artist, title string) scheduler.TrackRef {
	return scheduler.TrackRef{ID: id, Artist: artist, Title: title, DurationMS: 180000}
}

func simpleClock(slots []scheduler.Slot) *scheduler.Clock {
	return &scheduler.Clock{ID: "clk1", Name: "Test Clock", Slots: slots}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestGenerate_SimpleCategory(t *testing.T) {
	tracks := []scheduler.TrackRef{
		makeTrack("t1", "Elis Regina", "Como Nossos Pais"),
		makeTrack("t2", "Caetano Veloso", "Sozinho"),
		makeTrack("t3", "Maria Bethânia", "A Barca"),
	}
	gen := scheduler.New(
		&stubClocks{clock: simpleClock([]scheduler.Slot{
			{ID: "s1", Position: 1, SlotType: "CATEGORY", CategoryID: "cat1", CategoryName: "MPB"},
		})},
		&stubTracks{byCategory: map[string][]scheduler.TrackRef{"cat1": tracks}},
		&stubSepRules{},
		&stubRotLog{},
	)

	from := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	items, warnings, err := gen.Generate(context.Background(), from, 1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].SlotType != "CATEGORY" {
		t.Errorf("SlotType = %q", items[0].SlotType)
	}
	if items[0].ClockName != "Test Clock" {
		t.Errorf("ClockName = %q", items[0].ClockName)
	}
	t.Logf("warnings: %v", warnings)
}

func TestGenerate_NoClock_Warning(t *testing.T) {
	gen := scheduler.New(
		&stubClocks{clock: nil}, // no clock for this hour
		&stubTracks{},
		&stubSepRules{},
		&stubRotLog{},
	)

	from := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	items, warnings, err := gen.Generate(context.Background(), from, 1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items when no clock, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Error("expected a warning for missing clock")
	}
}

func TestGenerate_EmptyCategory_Warning(t *testing.T) {
	gen := scheduler.New(
		&stubClocks{clock: simpleClock([]scheduler.Slot{
			{ID: "s1", Position: 1, SlotType: "CATEGORY", CategoryID: "cat1"},
		})},
		&stubTracks{byCategory: map[string][]scheduler.TrackRef{}}, // empty
		&stubSepRules{},
		&stubRotLog{},
	)

	from := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	items, warnings, err := gen.Generate(context.Background(), from, 1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Error("expected warning for empty category")
	}
}

// TestGenerate_ArtistSeparation_Normalized verifica que variações de maiúsculo,
// acento e espaço no nome do artista são tratadas como o mesmo artista para fins
// de separação mínima.
func TestGenerate_ArtistSeparation_Normalized(t *testing.T) {
	// t1 e t2 são do mesmo artista, mas com grafias diferentes.
	tracks := []scheduler.TrackRef{
		makeTrack("t1", "Elis Regina", "Song A"),
		makeTrack("t2", "ÉLIS REGINA", "Song B"), // maiúsculo + acento diferente
	}
	gen := scheduler.New(
		&stubClocks{clock: simpleClock([]scheduler.Slot{
			{ID: "s1", Position: 1, SlotType: "CATEGORY", CategoryID: "cat1"},
			{ID: "s2", Position: 2, SlotType: "CATEGORY", CategoryID: "cat1"},
		})},
		&stubTracks{byCategory: map[string][]scheduler.TrackRef{"cat1": tracks}},
		&stubSepRules{rules: []scheduler.SeparationRule{
			{ID: "r1", Field: "artist", MinSepMinutes: 60},
		}},
		&stubRotLog{},
	)

	from := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	items, warnings, err := gen.Generate(context.Background(), from, 1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// Ambos os slots devem ser preenchidos (fallback), e deve haver aviso de separação.
	if len(items) != 2 {
		t.Errorf("expected 2 items (fallback), got %d", len(items))
	}
	if len(warnings) == 0 {
		t.Error("expected separation warning when same artist (different casing/accent) fills both slots")
	}
}

func TestGenerate_ArtistSeparation(t *testing.T) {
	// Only 2 tracks, same artist. With strict 60min artist separation and
	// session artist tracking, the second slot in the same hour should
	// still pick a different track (or fall back).
	tracks := []scheduler.TrackRef{
		makeTrack("t1", "Elis Regina", "Song A"),
		makeTrack("t2", "Elis Regina", "Song B"),
	}
	gen := scheduler.New(
		&stubClocks{clock: simpleClock([]scheduler.Slot{
			{ID: "s1", Position: 1, SlotType: "CATEGORY", CategoryID: "cat1"},
			{ID: "s2", Position: 2, SlotType: "CATEGORY", CategoryID: "cat1"},
		})},
		&stubTracks{byCategory: map[string][]scheduler.TrackRef{"cat1": tracks}},
		&stubSepRules{rules: []scheduler.SeparationRule{
			{ID: "r1", Field: "artist", MinSepMinutes: 60},
		}},
		&stubRotLog{},
	)

	from := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	items, _, err := gen.Generate(context.Background(), from, 1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// Both slots should still get a track (fallback kicks in).
	if len(items) != 2 {
		t.Errorf("expected 2 items (with fallback), got %d", len(items))
	}
}

func TestGenerate_FixedSlot(t *testing.T) {
	fixed := makeTrack("fixed1", "Station", "Jingle Manhã")
	gen := scheduler.New(
		&stubClocks{clock: simpleClock([]scheduler.Slot{
			{ID: "s1", Position: 1, SlotType: "FIXED", FixedTrackID: "fixed1"},
		})},
		&stubTracks{byID: map[string]scheduler.TrackRef{"fixed1": fixed}},
		&stubSepRules{},
		&stubRotLog{},
	)

	from := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	items, _, err := gen.Generate(context.Background(), from, 1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Track.ID != "fixed1" {
		t.Errorf("expected fixed1, got %q", items[0].Track.ID)
	}
}

func TestGenerate_MultipleHours(t *testing.T) {
	tracks := []scheduler.TrackRef{
		makeTrack("t1", "A1", "S1"),
		makeTrack("t2", "A2", "S2"),
		makeTrack("t3", "A3", "S3"),
	}
	gen := scheduler.New(
		&stubClocks{clock: simpleClock([]scheduler.Slot{
			{ID: "s1", Position: 1, SlotType: "CATEGORY", CategoryID: "cat1"},
			{ID: "s2", Position: 2, SlotType: "JINGLE"},
		})},
		&stubTracks{
			byCategory: map[string][]scheduler.TrackRef{"cat1": tracks},
			byType:     map[string][]scheduler.TrackRef{"JINGLE": {makeTrack("j1", "", "Jingle")}},
		},
		&stubSepRules{},
		&stubRotLog{},
	)

	from := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	items, _, err := gen.Generate(context.Background(), from, 3)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// 3 hours × 2 slots = 6 items.
	if len(items) != 6 {
		t.Errorf("expected 6 items, got %d", len(items))
	}
}

// TestGenerate_DurationCap verifies that the safety cap stops adding slots once
// accumulated + next_dur would exceed the limit (lookahead).
// Clock has 25 slots × 3 min each = 75 min, but only 1 hour is requested.
// Lookahead rejects slot 21 because 20×3 min = 60 min exactly at the limit.
func TestGenerate_DurationCap(t *testing.T) {
	track := makeTrack("t1", "Artist", "Song") // DurationMS = 180_000 (3 min)
	slots := make([]scheduler.Slot, 25)
	for i := range slots {
		slots[i] = scheduler.Slot{
			ID:         fmt.Sprintf("s%d", i+1),
			Position:   i + 1,
			SlotType:   "CATEGORY",
			CategoryID: "cat1",
		}
	}
	gen := scheduler.New(
		&stubClocks{clock: simpleClock(slots)},
		&stubTracks{byCategory: map[string][]scheduler.TrackRef{"cat1": {track}}},
		&stubSepRules{},
		&stubRotLog{},
	)

	from := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	items, _, err := gen.Generate(context.Background(), from, 1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(items) != 20 {
		t.Errorf("expected 20 items (duration cap), got %d", len(items))
	}
	total := int64(0)
	for _, it := range items {
		total += it.Track.DurationMS
	}
	if total > 3_600_000 {
		t.Errorf("total duration %dms exceeds 1-hour cap (3_600_000ms)", total)
	}
}

// TestGenerate_DurationCapLookahead verifies that the lookahead cap prevents a
// long track from pushing the playlist past the requested time window.
// Clock has 25 slots × 7 min each = 175 min, but only 1 hour is requested.
// After 8 tracks (56 min), the 9th (7 min) would reach 63 min > 60 min → rejected.
func TestGenerate_DurationCapLookahead(t *testing.T) {
	track := scheduler.TrackRef{ID: "t1", Artist: "Artist", Title: "Long Song", DurationMS: 420_000} // 7 min
	slots := make([]scheduler.Slot, 25)
	for i := range slots {
		slots[i] = scheduler.Slot{
			ID:         fmt.Sprintf("s%d", i+1),
			Position:   i + 1,
			SlotType:   "CATEGORY",
			CategoryID: "cat1",
		}
	}
	gen := scheduler.New(
		&stubClocks{clock: simpleClock(slots)},
		&stubTracks{byCategory: map[string][]scheduler.TrackRef{"cat1": {track}}},
		&stubSepRules{},
		&stubRotLog{},
	)

	from := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	items, _, err := gen.Generate(context.Background(), from, 1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// 8 × 7 min = 56 min ≤ 60 min; 9 × 7 min = 63 min > 60 min → stops at 8.
	if len(items) != 8 {
		t.Errorf("expected 8 items (lookahead cap), got %d", len(items))
	}
	total := int64(0)
	for _, it := range items {
		total += it.Track.DurationMS
	}
	if total > 3_600_000 {
		t.Errorf("total duration %dms exceeds 1-hour cap (3_600_000ms)", total)
	}
}

func TestGenerate_HoursCapAt24(t *testing.T) {
	gen := scheduler.New(
		&stubClocks{clock: nil},
		&stubTracks{},
		&stubSepRules{},
		&stubRotLog{},
	)
	from := time.Now().UTC()
	_, warnings, err := gen.Generate(context.Background(), from, 999)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(warnings) != 24 {
		t.Errorf("expected 24 warnings (one per hour, capped), got %d", len(warnings))
	}
}
