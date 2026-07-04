package store_test

import (
	"context"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

// seedTrack inserts a minimal track and returns its ID.
func seedTrack(t *testing.T, ts *store.TrackStore, path, title, typ string) string {
	t.Helper()
	ctx := context.Background()
	tr := store.Track{Path: path, Title: title, Type: typ}
	if err := ts.Upsert(ctx, tr); err != nil {
		t.Fatalf("seedTrack: %v", err)
	}
	got, err := ts.FindByPath(ctx, path)
	if err != nil {
		t.Fatalf("seedTrack FindByPath: %v", err)
	}
	return got.ID
}

// ─── Create ──────────────────────────────────────────────────────────────────

func TestPlaylist_Create(t *testing.T) {
	db := openMemDB(t)
	ps := store.NewPlaylistStore(db)
	ctx := context.Background()

	pl, err := ps.Create(ctx, "Manhã Feliz", "Pop")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if pl.ID == "" {
		t.Error("ID must be set")
	}
	if pl.Name != "Manhã Feliz" {
		t.Errorf("Name = %q", pl.Name)
	}
	if pl.Category != "Pop" {
		t.Errorf("Category = %q", pl.Category)
	}
}

func TestPlaylist_CreateEmptyNameRejected(t *testing.T) {
	ps := store.NewPlaylistStore(openMemDB(t))
	_, err := ps.Create(context.Background(), "  ", "")
	if err == nil {
		t.Error("want error for empty name")
	}
}

// ─── FindByID ────────────────────────────────────────────────────────────────

func TestPlaylist_FindByID_NotFound(t *testing.T) {
	ps := store.NewPlaylistStore(openMemDB(t))
	_, err := ps.FindByID(context.Background(), "ghost")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// ─── List ────────────────────────────────────────────────────────────────────

func TestPlaylist_List(t *testing.T) {
	ps := store.NewPlaylistStore(openMemDB(t))
	ctx := context.Background()

	_, _ = ps.Create(ctx, "Tarde Animada", "")
	_, _ = ps.Create(ctx, "Manhã Feliz", "Pop")

	list, err := ps.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	// sorted alphabetically
	if list[0].Name != "Manhã Feliz" {
		t.Errorf("first = %q, want Manhã Feliz", list[0].Name)
	}
}

// ─── Update ──────────────────────────────────────────────────────────────────

func TestPlaylist_Update(t *testing.T) {
	ps := store.NewPlaylistStore(openMemDB(t))
	ctx := context.Background()

	pl, _ := ps.Create(ctx, "Old Name", "")
	if err := ps.Update(ctx, pl.ID, store.PlaylistPatch{
		Name:     pstr("New Name"),
		Category: pstr("Sertanejo"),
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := ps.FindByID(ctx, pl.ID)
	if got.Name != "New Name" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Category != "Sertanejo" {
		t.Errorf("Category = %q", got.Category)
	}
}

func TestPlaylist_Update_NotFound(t *testing.T) {
	ps := store.NewPlaylistStore(openMemDB(t))
	err := ps.Update(context.Background(), "ghost", store.PlaylistPatch{Name: pstr("X")})
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func TestPlaylist_Delete(t *testing.T) {
	ps := store.NewPlaylistStore(openMemDB(t))
	ctx := context.Background()

	pl, _ := ps.Create(ctx, "Gone", "")
	if err := ps.Delete(ctx, pl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := ps.FindByID(ctx, pl.ID)
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

// ─── AddItem / RemoveItem ────────────────────────────────────────────────────

func TestPlaylist_AddItem(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	ps := store.NewPlaylistStore(db)
	ctx := context.Background()

	trackID := seedTrack(t, ts, "/m/1.mp3", "Detalhes", "MUSIC")
	pl, _ := ps.Create(ctx, "Test PL", "")

	item, err := ps.AddItem(ctx, pl.ID, trackID)
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if item.ID == "" {
		t.Error("item ID must be set")
	}
	if item.Position != 1 {
		t.Errorf("position = %d, want 1", item.Position)
	}
	if item.Track.Title != "Detalhes" {
		t.Errorf("track title = %q", item.Track.Title)
	}

	// Second item gets position 2.
	track2ID := seedTrack(t, ts, "/m/2.mp3", "Emoções", "MUSIC")
	item2, _ := ps.AddItem(ctx, pl.ID, track2ID)
	if item2.Position != 2 {
		t.Errorf("second item position = %d, want 2", item2.Position)
	}

	// FindByID should return both items.
	got, _ := ps.FindByID(ctx, pl.ID)
	if len(got.Items) != 2 {
		t.Errorf("want 2 items, got %d", len(got.Items))
	}
}

func TestPlaylist_AddItem_TrackNotFound(t *testing.T) {
	ps := store.NewPlaylistStore(openMemDB(t))
	ctx := context.Background()
	pl, _ := ps.Create(ctx, "PL", "")
	_, err := ps.AddItem(ctx, pl.ID, "no-such-track")
	if err == nil {
		t.Error("want error for missing track")
	}
}

func TestPlaylist_RemoveItem(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	ps := store.NewPlaylistStore(db)
	ctx := context.Background()

	trackID := seedTrack(t, ts, "/m/1.mp3", "Song", "MUSIC")
	pl, _ := ps.Create(ctx, "PL", "")
	item, _ := ps.AddItem(ctx, pl.ID, trackID)

	if err := ps.RemoveItem(ctx, item.ID); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}
	got, _ := ps.FindByID(ctx, pl.ID)
	if len(got.Items) != 0 {
		t.Errorf("want 0 items, got %d", len(got.Items))
	}
}

// ─── ReorderItems ────────────────────────────────────────────────────────────

func TestPlaylist_ReorderItems(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	ps := store.NewPlaylistStore(db)
	ctx := context.Background()

	t1 := seedTrack(t, ts, "/m/1.mp3", "A", "MUSIC")
	t2 := seedTrack(t, ts, "/m/2.mp3", "B", "MUSIC")
	t3 := seedTrack(t, ts, "/m/3.mp3", "C", "MUSIC")

	pl, _ := ps.Create(ctx, "PL", "")
	i1, _ := ps.AddItem(ctx, pl.ID, t1)
	i2, _ := ps.AddItem(ctx, pl.ID, t2)
	i3, _ := ps.AddItem(ctx, pl.ID, t3)

	// Reverse the order.
	if err := ps.ReorderItems(ctx, pl.ID, []string{i3.ID, i2.ID, i1.ID}); err != nil {
		t.Fatalf("ReorderItems: %v", err)
	}

	got, _ := ps.FindByID(ctx, pl.ID)
	if len(got.Items) != 3 {
		t.Fatalf("want 3 items, got %d", len(got.Items))
	}
	if got.Items[0].TrackID != t3 {
		t.Errorf("first item = %q, want t3", got.Items[0].TrackID)
	}
	if got.Items[2].TrackID != t1 {
		t.Errorf("last item = %q, want t1", got.Items[2].TrackID)
	}
}

func TestPlaylist_ReorderItems_UnknownID(t *testing.T) {
	ps := store.NewPlaylistStore(openMemDB(t))
	ctx := context.Background()
	pl, _ := ps.Create(ctx, "PL", "")
	err := ps.ReorderItems(ctx, pl.ID, []string{"ghost-id"})
	if err == nil {
		t.Error("want error for unknown item ID")
	}
}

// ─── List item_count ─────────────────────────────────────────────────────────

func TestPlaylist_ListItemCount(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	ps := store.NewPlaylistStore(db)
	ctx := context.Background()

	pl, _ := ps.Create(ctx, "PL", "")
	trackID := seedTrack(t, ts, "/m/1.mp3", "Song", "MUSIC")
	_, _ = ps.AddItem(ctx, pl.ID, trackID)

	list, _ := ps.List(ctx)
	if len(list) != 1 {
		t.Fatalf("want 1 playlist, got %d", len(list))
	}
	if list[0].ItemCount != 1 {
		t.Errorf("item_count = %d, want 1", list[0].ItemCount)
	}
}

// helper to get a *string
func pstr(s string) *string { return &s }
