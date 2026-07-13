package store_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

func finAt(t time.Time) *time.Time { return &t }

var baseTime = time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)

func seedTLEntry(t *testing.T, s *store.TransmissionLogStore, overrides store.TransmissionLogEntry) store.TransmissionLogEntry {
	t.Helper()
	e := store.TransmissionLogEntry{
		QueueItemID:      "qi_" + overrides.QueueItemID,
		AssetID:          "asset-1",
		Title:            "Track A",
		Artist:           "Artist X",
		Type:             "MUSIC",
		ISRC:             "BR-ABC-26-00001",
		Composer:         "Comp A",
		DurationMS:       240000,
		DurationPlayedMS: 240000,
		Result:           "finished",
		StartedAt:        baseTime,
		FinishedAt:       finAt(baseTime.Add(4 * time.Minute)),
		ImportFileName:   "transmission_20260720_08.jsonl",
	}
	if overrides.Type != "" {
		e.Type = overrides.Type
	}
	if overrides.StartedAt != (time.Time{}) {
		e.StartedAt = overrides.StartedAt
		e.FinishedAt = finAt(overrides.StartedAt.Add(4 * time.Minute))
	}
	if overrides.Title != "" {
		e.Title = overrides.Title
	}
	if overrides.Artist != "" {
		e.Artist = overrides.Artist
	}
	if err := s.BulkInsert(context.Background(), []store.TransmissionLogEntry{e}); err != nil {
		t.Fatalf("BulkInsert seed: %v", err)
	}
	return e
}

// ── BulkInsert ────────────────────────────────────────────────────────────────

func TestTransmissionLog_BulkInsert_Idempotent(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionLogStore(openMemDB(t))

	e := store.TransmissionLogEntry{
		QueueItemID:    "qi_idem",
		AssetID:        "a1",
		Title:          "T",
		Artist:         "A",
		Type:           "MUSIC",
		DurationMS:     100,
		Result:         "finished",
		StartedAt:      baseTime,
		ImportFileName: "f.jsonl",
	}

	// First insert.
	if err := s.BulkInsert(ctx, []store.TransmissionLogEntry{e}); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// Second insert — same queue_item_id — must be no-op (INSERT OR IGNORE).
	if err := s.BulkInsert(ctx, []store.TransmissionLogEntry{e}); err != nil {
		t.Fatalf("second insert: %v", err)
	}

	_, total, err := s.List(ctx, store.TransmissionLogQuery{Limit: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1 (idempotent)", total)
	}
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestTransmissionLog_List_FilterByType(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionLogStore(openMemDB(t))

	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "1", Type: "MUSIC"})
	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "2", Type: "SPOT"})

	entries, total, err := s.List(ctx, store.TransmissionLogQuery{Type: "MUSIC", Limit: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(entries) != 1 || entries[0].Type != "MUSIC" {
		t.Errorf("unexpected entries: %v", entries)
	}
}

func TestTransmissionLog_List_FilterBySearch(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionLogStore(openMemDB(t))

	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "a", Title: "Garota de Ipanema"})
	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "b", Title: "Aquarela do Brasil"})

	entries, total, err := s.List(ctx, store.TransmissionLogQuery{Search: "Ipanema", Limit: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(entries) == 0 || !strings.Contains(entries[0].Title, "Ipanema") {
		t.Errorf("unexpected title: %v", entries)
	}
}

func TestTransmissionLog_List_FilterByDateRange(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionLogStore(openMemDB(t))

	day1 := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "d1", StartedAt: day1})
	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "d2", StartedAt: day2})

	entries, total, err := s.List(ctx, store.TransmissionLogQuery{
		From:  day1,
		To:    day1.Add(23 * time.Hour),
		Limit: 100,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	_ = entries
}

// ── Summary ───────────────────────────────────────────────────────────────────

func TestTransmissionLog_Summary(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionLogStore(openMemDB(t))

	day := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "s1", Type: "MUSIC", StartedAt: day.Add(8 * time.Hour)})
	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "s2", Type: "MUSIC", StartedAt: day.Add(9 * time.Hour)})
	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "s3", Type: "SPOT", StartedAt: day.Add(10 * time.Hour)})

	sum, err := s.Summary(ctx, day)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.Total != 3 {
		t.Errorf("Total = %d, want 3", sum.Total)
	}
	if sum.ByType["MUSIC"] != 2 {
		t.Errorf("ByType[MUSIC] = %d, want 2", sum.ByType["MUSIC"])
	}
	if sum.ByType["SPOT"] != 1 {
		t.Errorf("ByType[SPOT] = %d, want 1", sum.ByType["SPOT"])
	}
}

// ── ExportCSV ─────────────────────────────────────────────────────────────────

func TestTransmissionLog_ExportCSV(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionLogStore(openMemDB(t))

	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "csv1"})

	var buf bytes.Buffer
	from := baseTime.Add(-time.Hour)
	to := baseTime.Add(time.Hour)
	if err := s.ExportCSV(ctx, from, to, &buf); err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "started_at") {
		t.Errorf("missing CSV header in output: %q", out[:min2(100, len(out))])
	}
	if !strings.Contains(out, "Track A") {
		t.Errorf("missing track title in CSV output: %q", out)
	}
}

// ── ExportECAD ────────────────────────────────────────────────────────────────

func TestTransmissionLog_ExportECAD(t *testing.T) {
	ctx := context.Background()
	s := store.NewTransmissionLogStore(openMemDB(t))

	// ECAD only includes MUSIC/JINGLE/VINHETA.
	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "e1", Type: "MUSIC"})
	seedTLEntry(t, s, store.TransmissionLogEntry{QueueItemID: "e2", Type: "SPOT"}) // must be excluded

	station := store.StationInfo{
		Name: "Radio Exemplo FM", CNPJ: "12.345.678/0001-90",
		City: "São Paulo", State: "SP", Frequency: "98.5 MHz", Type: "FM",
	}

	var buf bytes.Buffer
	from := baseTime.Add(-time.Hour)
	to := baseTime.Add(time.Hour)
	if err := s.ExportECAD(ctx, from, to, station, &buf); err != nil {
		t.Fatalf("ExportECAD: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (header + 1 detail), got %d:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "H;") {
		t.Errorf("first line must start with H;, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "Radio Exemplo FM") {
		t.Errorf("header missing station name: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "D;") {
		t.Errorf("detail line must start with D;, got %q", lines[1])
	}
	// SPOT must not appear.
	for _, l := range lines[1:] {
		if strings.Contains(l, "SPOT") {
			t.Errorf("SPOT entry must not appear in ECAD export: %q", l)
		}
	}
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
