package store_test

import (
	"context"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

func TestCategory_CreateAndGet(t *testing.T) {
	cs := store.NewCategoryStore(openMemDB(t))
	ctx := context.Background()

	cat, err := cs.Create(ctx, "MPB Clássica", "Músicas MPB de 1960–1990", "#3a7bd5")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cat.ID == "" {
		t.Error("ID must be set")
	}
	if cat.Name != "MPB Clássica" {
		t.Errorf("Name = %q", cat.Name)
	}
	if cat.Color != "#3a7bd5" {
		t.Errorf("Color = %q", cat.Color)
	}

	got, err := cs.Get(ctx, cat.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != cat.ID {
		t.Errorf("Get ID mismatch: %q vs %q", got.ID, cat.ID)
	}
}

func TestCategory_CreateEmptyNameFails(t *testing.T) {
	cs := store.NewCategoryStore(openMemDB(t))
	if _, err := cs.Create(context.Background(), "  ", "", ""); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestCategory_List(t *testing.T) {
	cs := store.NewCategoryStore(openMemDB(t))
	ctx := context.Background()

	for _, name := range []string{"Rock", "MPB", "Sertanejo"} {
		if _, err := cs.Create(ctx, name, "", "#000"); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	cats, err := cs.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cats) != 3 {
		t.Errorf("expected 3 categories, got %d", len(cats))
	}
	// should be alphabetical
	if cats[0].Name != "MPB" {
		t.Errorf("first category should be MPB, got %q", cats[0].Name)
	}
}

func TestCategory_Update(t *testing.T) {
	cs := store.NewCategoryStore(openMemDB(t))
	ctx := context.Background()

	cat, _ := cs.Create(ctx, "Old Name", "", "#000")
	if err := cs.Update(ctx, cat.ID, "New Name", "desc", "#fff"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := cs.Get(ctx, cat.ID)
	if got.Name != "New Name" {
		t.Errorf("Name after update = %q", got.Name)
	}
}

func TestCategory_Delete(t *testing.T) {
	cs := store.NewCategoryStore(openMemDB(t))
	ctx := context.Background()

	cat, _ := cs.Create(ctx, "Temp", "", "#000")
	if err := cs.Delete(ctx, cat.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := cs.Get(ctx, cat.ID); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCategory_DeleteBlockedByClockSlot(t *testing.T) {
	db := openMemDB(t)
	cs := store.NewCategoryStore(db)
	cls := store.NewClockStore(db)
	ctx := context.Background()

	cat, _ := cs.Create(ctx, "MPB", "", "#000")
	clk, _ := cls.Create(ctx, "Manhã")
	_, err := cls.AddSlot(ctx, clk.ID, store.ClockSlot{
		SlotType:   "CATEGORY",
		CategoryID: cat.ID,
	})
	if err != nil {
		t.Fatalf("AddSlot: %v", err)
	}

	// Delete should fail because the category is referenced by a slot.
	if err := cs.Delete(ctx, cat.ID); err == nil {
		t.Error("expected error deleting category referenced by clock slot")
	}
}

func TestCategory_AddAndRemoveTracks(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	cs := store.NewCategoryStore(db)
	ctx := context.Background()

	cat, _ := cs.Create(ctx, "Rock", "", "#000")
	t1 := seedTrack(t, ts, "/r/01.mp3", "Track 1", "MUSIC")
	t2 := seedTrack(t, ts, "/r/02.mp3", "Track 2", "MUSIC")

	if err := cs.AddTracks(ctx, cat.ID, []string{t1, t2}); err != nil {
		t.Fatalf("AddTracks: %v", err)
	}

	got, _ := cs.Get(ctx, cat.ID)
	if got.TrackCount != 2 {
		t.Errorf("TrackCount = %d, want 2", got.TrackCount)
	}

	tracks, err := cs.ListTracks(ctx, cat.ID, 50, 0)
	if err != nil {
		t.Fatalf("ListTracks: %v", err)
	}
	if len(tracks) != 2 {
		t.Errorf("expected 2 tracks, got %d", len(tracks))
	}

	if err := cs.RemoveTrack(ctx, cat.ID, t1); err != nil {
		t.Fatalf("RemoveTrack: %v", err)
	}
	got, _ = cs.Get(ctx, cat.ID)
	if got.TrackCount != 1 {
		t.Errorf("TrackCount after remove = %d, want 1", got.TrackCount)
	}
}

func TestCategory_SetTrackCategories(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	cs := store.NewCategoryStore(db)
	ctx := context.Background()

	c1, _ := cs.Create(ctx, "Cat1", "", "#000")
	c2, _ := cs.Create(ctx, "Cat2", "", "#000")
	tid := seedTrack(t, ts, "/m/01.mp3", "Track", "MUSIC")

	// Assign to both categories.
	if err := cs.SetTrackCategories(ctx, tid, []string{c1.ID, c2.ID}); err != nil {
		t.Fatalf("SetTrackCategories: %v", err)
	}
	cats, err := cs.ListByTrack(ctx, tid)
	if err != nil {
		t.Fatalf("ListByTrack: %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d", len(cats))
	}

	// Replace with only c1.
	if err := cs.SetTrackCategories(ctx, tid, []string{c1.ID}); err != nil {
		t.Fatalf("SetTrackCategories replace: %v", err)
	}
	cats, _ = cs.ListByTrack(ctx, tid)
	if len(cats) != 1 || cats[0].ID != c1.ID {
		t.Errorf("after replace, expected only c1, got %v", cats)
	}
}
