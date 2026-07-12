package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

func TestRotationLog_AppendAndListByDate(t *testing.T) {
	rls := store.NewRotationLogStore(openMemDB(t))
	ctx := context.Background()

	now := time.Now().UTC()
	entries := []store.RotationLogEntry{
		{TrackID: "t1", PlayedAt: now, ClockID: "clk1", SlotType: "CATEGORY", CategoryID: "cat1", Artist: "Elis Regina", Title: "Como Nossos Pais", Album: "Falso Brilhante"},
		{TrackID: "t2", PlayedAt: now.Add(5 * time.Minute), ClockID: "clk1", SlotType: "JINGLE", CategoryID: "", Artist: "", Title: "Jingle Manhã", Album: ""},
	}
	for _, e := range entries {
		if err := rls.Append(ctx, e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	got, err := rls.ListByDate(ctx, now)
	if err != nil {
		t.Fatalf("ListByDate: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
}

func TestRotationLog_RecentByField(t *testing.T) {
	rls := store.NewRotationLogStore(openMemDB(t))
	ctx := context.Background()

	now := time.Now().UTC()
	entries := []store.RotationLogEntry{
		{TrackID: "t1", PlayedAt: now.Add(-90 * time.Minute), Artist: "Elis Regina", Title: "Song A", CategoryID: "cat1"},
		{TrackID: "t2", PlayedAt: now.Add(-30 * time.Minute), Artist: "Elis Regina", Title: "Song B", CategoryID: "cat1"},
		{TrackID: "t3", PlayedAt: now.Add(-10 * time.Minute), Artist: "Caetano Veloso", Title: "Song C", CategoryID: "cat1"},
	}
	for _, e := range entries {
		if err := rls.Append(ctx, e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	// Artist "Elis Regina" since 60 min ago → only t2 qualifies.
	since := now.Add(-60 * time.Minute)
	got, err := rls.RecentByField(ctx, "artist", "Elis Regina", since)
	if err != nil {
		t.Fatalf("RecentByField: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 entry, got %d", len(got))
	}
	if len(got) > 0 && got[0].TrackID != "t2" {
		t.Errorf("expected t2, got %q", got[0].TrackID)
	}
}

func TestRotationLog_RecentTrackIDs(t *testing.T) {
	rls := store.NewRotationLogStore(openMemDB(t))
	ctx := context.Background()

	now := time.Now().UTC()
	for i, tid := range []string{"t1", "t2", "t3"} {
		e := store.RotationLogEntry{
			TrackID:  tid,
			PlayedAt: now.Add(time.Duration(-i*20) * time.Minute),
		}
		if err := rls.Append(ctx, e); err != nil {
			t.Fatalf("Append %q: %v", tid, err)
		}
	}

	since := now.Add(-30 * time.Minute)
	ids, err := rls.RecentTrackIDs(ctx, since)
	if err != nil {
		t.Fatalf("RecentTrackIDs: %v", err)
	}
	// t1 (0 min ago) and t2 (20 min ago) qualify; t3 (40 min ago) doesn't.
	if _, ok := ids["t1"]; !ok {
		t.Error("t1 should be in recent ids")
	}
	if _, ok := ids["t2"]; !ok {
		t.Error("t2 should be in recent ids")
	}
	if _, ok := ids["t3"]; ok {
		t.Error("t3 should NOT be in recent ids")
	}
}
