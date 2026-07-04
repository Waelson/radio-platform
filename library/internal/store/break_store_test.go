package store_test

import (
	"context"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

// ─── Create ──────────────────────────────────────────────────────────────────

func TestBreak_Create(t *testing.T) {
	bs := store.NewBreakStore(openMemDB(t))
	ctx := context.Background()

	brk, err := bs.Create(ctx, "Break Comercial", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if brk.ID == "" {
		t.Error("ID must be set")
	}
	if brk.Name != "Break Comercial" {
		t.Errorf("Name = %q", brk.Name)
	}
	if brk.OpenTrack != nil || brk.CloseTrack != nil {
		t.Error("open/close track should be nil when not set")
	}
}

func TestBreak_CreateWithOpenClose(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	bs := store.NewBreakStore(db)
	ctx := context.Background()

	openID := seedTrack(t, ts, "/j/open.mp3", "Abertura", "JINGLE")
	closeID := seedTrack(t, ts, "/j/close.mp3", "Fechamento", "JINGLE")

	brk, err := bs.Create(ctx, "Break", openID, closeID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if brk.OpenTrack == nil || brk.OpenTrack.ID != openID {
		t.Errorf("OpenTrack = %v", brk.OpenTrack)
	}
	if brk.CloseTrack == nil || brk.CloseTrack.ID != closeID {
		t.Errorf("CloseTrack = %v", brk.CloseTrack)
	}
}

func TestBreak_CreateEmptyName(t *testing.T) {
	bs := store.NewBreakStore(openMemDB(t))
	_, err := bs.Create(context.Background(), "  ", "", "")
	if err == nil {
		t.Error("want error for empty name")
	}
}

// ─── FindByID ────────────────────────────────────────────────────────────────

func TestBreak_FindByID_NotFound(t *testing.T) {
	bs := store.NewBreakStore(openMemDB(t))
	_, err := bs.FindByID(context.Background(), "ghost")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// ─── List ────────────────────────────────────────────────────────────────────

func TestBreak_List(t *testing.T) {
	bs := store.NewBreakStore(openMemDB(t))
	ctx := context.Background()

	_, _ = bs.Create(ctx, "Tarde", "", "")
	_, _ = bs.Create(ctx, "Manhã", "", "")

	list, err := bs.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2, got %d", len(list))
	}
	if list[0].Name != "Manhã" {
		t.Errorf("sorted: first = %q", list[0].Name)
	}
}

// ─── Update ──────────────────────────────────────────────────────────────────

func TestBreak_Update(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	bs := store.NewBreakStore(db)
	ctx := context.Background()

	openID := seedTrack(t, ts, "/j/open.mp3", "Abertura", "JINGLE")
	brk, _ := bs.Create(ctx, "Old", "", "")

	if err := bs.Update(ctx, brk.ID, store.BreakPatch{
		Name:        pstr("New Name"),
		OpenTrackID: &openID,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := bs.FindByID(ctx, brk.ID)
	if got.Name != "New Name" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.OpenTrack == nil || got.OpenTrack.ID != openID {
		t.Errorf("OpenTrack = %v", got.OpenTrack)
	}
}

func TestBreak_Update_ClearOpenTrack(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	bs := store.NewBreakStore(db)
	ctx := context.Background()

	openID := seedTrack(t, ts, "/j/open.mp3", "Abertura", "JINGLE")
	brk, _ := bs.Create(ctx, "Break", openID, "")

	empty := ""
	if err := bs.Update(ctx, brk.ID, store.BreakPatch{OpenTrackID: &empty}); err != nil {
		t.Fatalf("Update (clear): %v", err)
	}

	got, _ := bs.FindByID(ctx, brk.ID)
	if got.OpenTrack != nil {
		t.Errorf("OpenTrack should be nil after clear, got %v", got.OpenTrack)
	}
}

func TestBreak_Update_NotFound(t *testing.T) {
	bs := store.NewBreakStore(openMemDB(t))
	err := bs.Update(context.Background(), "ghost", store.BreakPatch{Name: pstr("X")})
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func TestBreak_Delete(t *testing.T) {
	bs := store.NewBreakStore(openMemDB(t))
	ctx := context.Background()

	brk, _ := bs.Create(ctx, "Gone", "", "")
	if err := bs.Delete(ctx, brk.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := bs.FindByID(ctx, brk.ID)
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

// ─── AddItem / RemoveItem ────────────────────────────────────────────────────

func TestBreak_AddItem(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	bs := store.NewBreakStore(db)
	ctx := context.Background()

	spotID := seedTrack(t, ts, "/s/spot1.mp3", "Farmácia XYZ", "SPOT")
	brk, _ := bs.Create(ctx, "Break", "", "")

	item, err := bs.AddItem(ctx, brk.ID, spotID)
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if item.Position != 1 {
		t.Errorf("position = %d, want 1", item.Position)
	}
	if item.Track.Title != "Farmácia XYZ" {
		t.Errorf("track title = %q", item.Track.Title)
	}

	// Second spot → position 2.
	spot2ID := seedTrack(t, ts, "/s/spot2.mp3", "Banco Y", "SPOT")
	item2, _ := bs.AddItem(ctx, brk.ID, spot2ID)
	if item2.Position != 2 {
		t.Errorf("second item position = %d, want 2", item2.Position)
	}

	got, _ := bs.FindByID(ctx, brk.ID)
	if len(got.Items) != 2 {
		t.Errorf("want 2 items, got %d", len(got.Items))
	}
}

func TestBreak_AddItem_TrackNotFound(t *testing.T) {
	bs := store.NewBreakStore(openMemDB(t))
	ctx := context.Background()
	brk, _ := bs.Create(ctx, "Break", "", "")
	_, err := bs.AddItem(ctx, brk.ID, "no-track")
	if err == nil {
		t.Error("want error for missing track")
	}
}

func TestBreak_RemoveItem(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	bs := store.NewBreakStore(db)
	ctx := context.Background()

	spotID := seedTrack(t, ts, "/s/spot.mp3", "Spot", "SPOT")
	brk, _ := bs.Create(ctx, "Break", "", "")
	item, _ := bs.AddItem(ctx, brk.ID, spotID)

	if err := bs.RemoveItem(ctx, item.ID); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}
	got, _ := bs.FindByID(ctx, brk.ID)
	if len(got.Items) != 0 {
		t.Errorf("want 0 items, got %d", len(got.Items))
	}
}

// ─── ReorderItems ────────────────────────────────────────────────────────────

func TestBreak_ReorderItems(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	bs := store.NewBreakStore(db)
	ctx := context.Background()

	s1 := seedTrack(t, ts, "/s/1.mp3", "A", "SPOT")
	s2 := seedTrack(t, ts, "/s/2.mp3", "B", "SPOT")
	s3 := seedTrack(t, ts, "/s/3.mp3", "C", "SPOT")

	brk, _ := bs.Create(ctx, "Break", "", "")
	i1, _ := bs.AddItem(ctx, brk.ID, s1)
	i2, _ := bs.AddItem(ctx, brk.ID, s2)
	i3, _ := bs.AddItem(ctx, brk.ID, s3)

	if err := bs.ReorderItems(ctx, brk.ID, []string{i3.ID, i1.ID, i2.ID}); err != nil {
		t.Fatalf("ReorderItems: %v", err)
	}

	got, _ := bs.FindByID(ctx, brk.ID)
	if got.Items[0].TrackID != s3 {
		t.Errorf("first = %q, want s3", got.Items[0].TrackID)
	}
}

func TestBreak_ReorderItems_UnknownID(t *testing.T) {
	bs := store.NewBreakStore(openMemDB(t))
	ctx := context.Background()
	brk, _ := bs.Create(ctx, "Break", "", "")
	err := bs.ReorderItems(ctx, brk.ID, []string{"ghost"})
	if err == nil {
		t.Error("want error for unknown item ID")
	}
}

// ─── List item_count ─────────────────────────────────────────────────────────

func TestBreak_ListItemCount(t *testing.T) {
	db := openMemDB(t)
	ts := store.NewTrackStore(db)
	bs := store.NewBreakStore(db)
	ctx := context.Background()

	brk, _ := bs.Create(ctx, "Break", "", "")
	spotID := seedTrack(t, ts, "/s/spot.mp3", "Spot", "SPOT")
	_, _ = bs.AddItem(ctx, brk.ID, spotID)

	list, _ := bs.List(ctx)
	if list[0].ItemCount != 1 {
		t.Errorf("item_count = %d, want 1", list[0].ItemCount)
	}
}
