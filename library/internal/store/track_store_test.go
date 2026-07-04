package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

// openMemDB opens an in-memory SQLite database with the full schema applied.
func openMemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("openMemDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func ptr(s string) *string { return &s }

// ─── Upsert ──────────────────────────────────────────────────────────────────

func TestUpsert_InsertNew(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	track := store.Track{
		Path:       "/music/song.mp3",
		Title:      "Detalhes",
		Artist:     "Roberto Carlos",
		Type:       "MUSIC",
		DurationMS: 214293,
		Category:   "Sertanejo",
	}
	if err := ts.Upsert(ctx, track); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := ts.FindByPath(ctx, track.Path)
	if err != nil {
		t.Fatalf("FindByPath: %v", err)
	}
	if got.Title != track.Title {
		t.Errorf("Title = %q, want %q", got.Title, track.Title)
	}
	if got.Artist != track.Artist {
		t.Errorf("Artist = %q, want %q", got.Artist, track.Artist)
	}
	if got.DurationMS != track.DurationMS {
		t.Errorf("DurationMS = %d, want %d", got.DurationMS, track.DurationMS)
	}
	if got.ID == "" {
		t.Error("ID should be set after insert")
	}
}

func TestUpsert_UpdatePreservesID(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	original := store.Track{
		Path:       "/music/song.mp3",
		Title:      "Old Title",
		Artist:     "Old Artist",
		Type:       "MUSIC",
		DurationMS: 10000,
	}
	if err := ts.Upsert(ctx, original); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	first, _ := ts.FindByPath(ctx, original.Path)

	updated := original
	updated.Title = "New Title"
	updated.DurationMS = 20000
	if err := ts.Upsert(ctx, updated); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	second, _ := ts.FindByPath(ctx, original.Path)

	if first.ID != second.ID {
		t.Errorf("ID changed after upsert: %q → %q", first.ID, second.ID)
	}
	if second.Title != "New Title" {
		t.Errorf("Title not updated: %q", second.Title)
	}
	if second.DurationMS != 20000 {
		t.Errorf("DurationMS not updated: %d", second.DurationMS)
	}
}

// ─── FindByID ────────────────────────────────────────────────────────────────

func TestFindByID_NotFound(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	_, err := ts.FindByID(context.Background(), "nonexistent")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestFindByID_Found(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	_ = ts.Upsert(ctx, store.Track{
		Path: "/jingles/spot.mp3", Title: "Promo", Type: "JINGLE",
	})
	inserted, _ := ts.FindByPath(ctx, "/jingles/spot.mp3")

	got, err := ts.FindByID(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Title != "Promo" {
		t.Errorf("Title = %q", got.Title)
	}
}

// ─── Search ──────────────────────────────────────────────────────────────────

func seedTracks(t *testing.T, ts *store.TrackStore) {
	t.Helper()
	ctx := context.Background()
	tracks := []store.Track{
		{Path: "/m/1.mp3", Title: "Detalhes", Artist: "Roberto Carlos", Type: "MUSIC", Category: "Pop"},
		{Path: "/m/2.mp3", Title: "Emoções", Artist: "Roberto Carlos", Type: "MUSIC", Category: "Pop"},
		{Path: "/v/1.mp3", Title: "Abertura Manhã", Type: "VINHETA", Category: "Entrada"},
		{Path: "/j/1.mp3", Title: "Você está na melhor", Artist: "Rádio X", Type: "JINGLE", Category: "Institucional"},
		{Path: "/s/1.mp3", Title: "Promoção 30s", Artist: "Farmácia XYZ", Type: "SPOT", Category: "Comercial"},
	}
	for _, tr := range tracks {
		if err := ts.Upsert(ctx, tr); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func TestSearch_All(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	seedTracks(t, ts)

	results, err := ts.Search(context.Background(), store.SearchQuery{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("want 5 results, got %d", len(results))
	}
}

func TestSearch_ByType(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	seedTracks(t, ts)

	results, err := ts.Search(context.Background(), store.SearchQuery{Type: "MUSIC"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 MUSIC tracks, got %d", len(results))
	}
}

func TestSearch_ByArtist(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	seedTracks(t, ts)

	results, err := ts.Search(context.Background(), store.SearchQuery{Artist: "Roberto Carlos"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 tracks for Roberto Carlos, got %d", len(results))
	}
}

func TestSearch_FullText(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	seedTracks(t, ts)

	results, err := ts.Search(context.Background(), store.SearchQuery{Q: "abertura"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("want 1 result for 'abertura', got %d", len(results))
	}
	if results[0].Title != "Abertura Manhã" {
		t.Errorf("unexpected title: %q", results[0].Title)
	}
}

func TestSearch_LimitOffset(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	seedTracks(t, ts)

	first, _ := ts.Search(context.Background(), store.SearchQuery{Limit: 2, Offset: 0})
	second, _ := ts.Search(context.Background(), store.SearchQuery{Limit: 2, Offset: 2})

	if len(first) != 2 {
		t.Errorf("first page: want 2, got %d", len(first))
	}
	if len(second) != 2 {
		t.Errorf("second page: want 2, got %d", len(second))
	}
	if first[0].Path == second[0].Path {
		t.Error("pages must not overlap")
	}
}

// ─── Count ───────────────────────────────────────────────────────────────────

func TestCount(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	n, _ := ts.Count(ctx)
	if n != 0 {
		t.Errorf("empty DB: want 0, got %d", n)
	}

	seedTracks(t, ts)
	n, _ = ts.Count(ctx)
	if n != 5 {
		t.Errorf("after seed: want 5, got %d", n)
	}
}

// ─── ListArtists ─────────────────────────────────────────────────────────────

func TestListArtists(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	seedTracks(t, ts)
	ctx := context.Background()

	artists, err := ts.ListArtists(ctx, "")
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	// Roberto Carlos, Rádio X, Farmácia XYZ (sorted)
	if len(artists) != 3 {
		t.Errorf("want 3 artists, got %d: %v", len(artists), artists)
	}

	musicArtists, _ := ts.ListArtists(ctx, "MUSIC")
	if len(musicArtists) != 1 {
		t.Errorf("want 1 MUSIC artist, got %d", len(musicArtists))
	}
	if musicArtists[0] != "Roberto Carlos" {
		t.Errorf("unexpected artist: %q", musicArtists[0])
	}
}

// ─── UpdateMeta ──────────────────────────────────────────────────────────────

func TestUpdateMeta_NotFound(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	err := ts.UpdateMeta(context.Background(), "ghost", store.TrackPatch{Title: ptr("X")})
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestUpdateMeta_PartialPatch(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	_ = ts.Upsert(ctx, store.Track{
		Path: "/m/patch.mp3", Title: "Original", Artist: "Artist A", Type: "MUSIC",
	})
	tr, _ := ts.FindByPath(ctx, "/m/patch.mp3")

	if err := ts.UpdateMeta(ctx, tr.ID, store.TrackPatch{
		Title:    ptr("Updated Title"),
		Category: ptr("Sertanejo"),
	}); err != nil {
		t.Fatalf("UpdateMeta: %v", err)
	}

	got, _ := ts.FindByID(ctx, tr.ID)
	if got.Title != "Updated Title" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Artist != "Artist A" {
		t.Errorf("Artist changed unexpectedly: %q", got.Artist)
	}
	if got.Category != "Sertanejo" {
		t.Errorf("Category = %q", got.Category)
	}
}

// ─── DeleteByPath ────────────────────────────────────────────────────────────

func TestDeleteByPath_Idempotent(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	_ = ts.Upsert(ctx, store.Track{Path: "/del/me.mp3", Title: "Gone", Type: "MUSIC"})

	if err := ts.DeleteByPath(ctx, "/del/me.mp3"); err != nil {
		t.Fatalf("DeleteByPath: %v", err)
	}
	// Second call must not error.
	if err := ts.DeleteByPath(ctx, "/del/me.mp3"); err != nil {
		t.Errorf("second DeleteByPath: %v", err)
	}

	_, err := ts.FindByPath(ctx, "/del/me.mp3")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}
