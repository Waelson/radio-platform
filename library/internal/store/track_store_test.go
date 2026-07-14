package store_test

import (
	"context"
	"database/sql"
	"fmt"
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

// ─── Loudness ────────────────────────────────────────────────────────────────

func TestUpdateLoudness_SetsValuesAndStatus(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	_ = ts.Upsert(ctx, store.Track{Path: "/m/lufs.mp3", Title: "Loud", Type: "MUSIC"})
	tr, _ := ts.FindByPath(ctx, "/m/lufs.mp3")

	// Default status should be "pending" after insert.
	if tr.LoudnessStatus != "pending" {
		t.Errorf("initial LoudnessStatus = %q, want pending", tr.LoudnessStatus)
	}
	if tr.LoudnessLUFS != nil {
		t.Error("initial LoudnessLUFS should be nil")
	}

	if err := ts.UpdateLoudness(ctx, tr.ID, -14.2, -0.5); err != nil {
		t.Fatalf("UpdateLoudness: %v", err)
	}

	got, _ := ts.FindByID(ctx, tr.ID)
	if got.LoudnessStatus != "done" {
		t.Errorf("LoudnessStatus = %q, want done", got.LoudnessStatus)
	}
	if got.LoudnessLUFS == nil || *got.LoudnessLUFS != -14.2 {
		t.Errorf("LoudnessLUFS = %v, want -14.2", got.LoudnessLUFS)
	}
	if got.TruePeakDBTP == nil || *got.TruePeakDBTP != -0.5 {
		t.Errorf("TruePeakDBTP = %v, want -0.5", got.TruePeakDBTP)
	}
	if got.LoudnessAnalyzedAt == nil {
		t.Error("LoudnessAnalyzedAt should be set after UpdateLoudness")
	}
	if got.LoudnessError != "" {
		t.Errorf("LoudnessError should be empty, got %q", got.LoudnessError)
	}
}

func TestUpdateLoudness_NotFound(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	err := ts.UpdateLoudness(context.Background(), "ghost", -16.0, -1.0)
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestUpdateLoudnessStatus_Error(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	_ = ts.Upsert(ctx, store.Track{Path: "/m/broken.mp3", Title: "Broken", Type: "MUSIC"})
	tr, _ := ts.FindByPath(ctx, "/m/broken.mp3")

	if err := ts.UpdateLoudnessStatus(ctx, tr.ID, "error", "ffmpeg exited with code 1"); err != nil {
		t.Fatalf("UpdateLoudnessStatus: %v", err)
	}

	got, _ := ts.FindByID(ctx, tr.ID)
	if got.LoudnessStatus != "error" {
		t.Errorf("LoudnessStatus = %q, want error", got.LoudnessStatus)
	}
	if got.LoudnessError != "ffmpeg exited with code 1" {
		t.Errorf("LoudnessError = %q", got.LoudnessError)
	}
}

func TestUpdateLoudnessStatus_Analyzing(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	_ = ts.Upsert(ctx, store.Track{Path: "/m/wip.mp3", Title: "WIP", Type: "MUSIC"})
	tr, _ := ts.FindByPath(ctx, "/m/wip.mp3")

	if err := ts.UpdateLoudnessStatus(ctx, tr.ID, "analyzing", ""); err != nil {
		t.Fatalf("UpdateLoudnessStatus: %v", err)
	}

	got, _ := ts.FindByID(ctx, tr.ID)
	if got.LoudnessStatus != "analyzing" {
		t.Errorf("LoudnessStatus = %q, want analyzing", got.LoudnessStatus)
	}
}

func TestUpdateLoudnessStatus_NotFound(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	err := ts.UpdateLoudnessStatus(context.Background(), "ghost", "error", "oops")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestCountByLoudnessStatus(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	// Seed 3 tracks.
	for i, path := range []string{"/m/a.mp3", "/m/b.mp3", "/m/c.mp3"} {
		_ = ts.Upsert(ctx, store.Track{Path: path, Title: "T", Type: "MUSIC"})
		tr, _ := ts.FindByPath(ctx, path)
		switch i {
		case 1:
			_ = ts.UpdateLoudness(ctx, tr.ID, -14.0, -1.0) // done
		case 2:
			_ = ts.UpdateLoudnessStatus(ctx, tr.ID, "error", "bad") // error
		}
		// i==0 stays "pending"
	}

	counts, err := ts.CountByLoudnessStatus(ctx)
	if err != nil {
		t.Fatalf("CountByLoudnessStatus: %v", err)
	}
	if counts["pending"] != 1 {
		t.Errorf("pending = %d, want 1", counts["pending"])
	}
	if counts["done"] != 1 {
		t.Errorf("done = %d, want 1", counts["done"])
	}
	if counts["error"] != 1 {
		t.Errorf("error = %d, want 1", counts["error"])
	}
}

func TestListPendingLoudness(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	paths := []string{"/m/p1.mp3", "/m/p2.mp3", "/m/p3.mp3", "/m/p4.mp3"}
	for _, path := range paths {
		_ = ts.Upsert(ctx, store.Track{Path: path, Title: "T", Type: "MUSIC"})
	}

	// Mark p2 as done and p3 as analyzing — should not appear in pending list.
	p2, _ := ts.FindByPath(ctx, "/m/p2.mp3")
	_ = ts.UpdateLoudness(ctx, p2.ID, -16.0, -1.0)
	p3, _ := ts.FindByPath(ctx, "/m/p3.mp3")
	_ = ts.UpdateLoudnessStatus(ctx, p3.ID, "analyzing", "")

	// p1 and p4 are "pending"; p3 is "analyzing" (not pending).
	ids, err := ts.ListPendingLoudness(ctx, 100)
	if err != nil {
		t.Fatalf("ListPendingLoudness: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("want 2 pending IDs, got %d: %v", len(ids), ids)
	}
}

func TestListPendingLoudness_IncludesError(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	_ = ts.Upsert(ctx, store.Track{Path: "/m/err.mp3", Title: "E", Type: "MUSIC"})
	tr, _ := ts.FindByPath(ctx, "/m/err.mp3")
	_ = ts.UpdateLoudnessStatus(ctx, tr.ID, "error", "bad codec")

	ids, err := ts.ListPendingLoudness(ctx, 100)
	if err != nil {
		t.Fatalf("ListPendingLoudness: %v", err)
	}
	if len(ids) != 1 || ids[0] != tr.ID {
		t.Errorf("want [%s], got %v", tr.ID, ids)
	}
}

func TestListPendingLoudness_Limit(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = ts.Upsert(ctx, store.Track{
			Path:  fmt.Sprintf("/m/lim%d.mp3", i),
			Title: "T",
			Type:  "MUSIC",
		})
	}

	ids, err := ts.ListPendingLoudness(ctx, 3)
	if err != nil {
		t.Fatalf("ListPendingLoudness: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("want 3, got %d", len(ids))
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

// ─── CuePoints ───────────────────────────────────────────────────────────────

func ptrI(v int64) *int64 { return &v }

func TestCuePoints_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cp      store.CuePoints
		wantErr bool
	}{
		{
			name: "all nil is valid",
			cp:   store.CuePoints{},
		},
		{
			name: "all set in order is valid",
			cp: store.CuePoints{
				CueInMS:  ptrI(500),
				IntroMS:  ptrI(15000),
				OutroMS:  ptrI(200000),
				CueOutMS: ptrI(213000),
			},
		},
		{
			name:    "negative cue_in_ms",
			cp:      store.CuePoints{CueInMS: ptrI(-1)},
			wantErr: true,
		},
		{
			name:    "cue_in >= intro",
			cp:      store.CuePoints{CueInMS: ptrI(15000), IntroMS: ptrI(500)},
			wantErr: true,
		},
		{
			name:    "intro >= outro",
			cp:      store.CuePoints{IntroMS: ptrI(200000), OutroMS: ptrI(15000)},
			wantErr: true,
		},
		{
			name:    "outro >= cue_out",
			cp:      store.CuePoints{OutroMS: ptrI(213000), CueOutMS: ptrI(200000)},
			wantErr: true,
		},
		{
			name:    "cue_in >= cue_out (no intro/outro)",
			cp:      store.CuePoints{CueInMS: ptrI(213000), CueOutMS: ptrI(500)},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cp.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSaveCuePoints(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	if err := ts.Upsert(ctx, store.Track{
		Path: "/m/cue.mp3", Title: "Cue Test", Type: "MUSIC", DurationMS: 220000,
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	track, _ := ts.FindByPath(ctx, "/m/cue.mp3")

	cp := store.CuePoints{
		CueInMS:  ptrI(500),
		IntroMS:  ptrI(18000),
		OutroMS:  ptrI(210000),
		CueOutMS: ptrI(218000),
	}
	if err := ts.SaveCuePoints(ctx, track.ID, cp); err != nil {
		t.Fatalf("SaveCuePoints: %v", err)
	}

	got, err := ts.FindByID(ctx, track.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.CueInMS == nil || *got.CueInMS != 500 {
		t.Errorf("CueInMS = %v, want 500", got.CueInMS)
	}
	if got.IntroMS == nil || *got.IntroMS != 18000 {
		t.Errorf("IntroMS = %v, want 18000", got.IntroMS)
	}
	if got.OutroMS == nil || *got.OutroMS != 210000 {
		t.Errorf("OutroMS = %v, want 210000", got.OutroMS)
	}
	if got.CueOutMS == nil || *got.CueOutMS != 218000 {
		t.Errorf("CueOutMS = %v, want 218000", got.CueOutMS)
	}

	// Clearing a marker by passing nil.
	if err := ts.SaveCuePoints(ctx, track.ID, store.CuePoints{}); err != nil {
		t.Fatalf("SaveCuePoints (clear): %v", err)
	}
	got2, _ := ts.FindByID(ctx, track.ID)
	if got2.CueInMS != nil {
		t.Errorf("CueInMS should be nil after clear, got %v", got2.CueInMS)
	}
}

func TestSaveCuePoints_NotFound(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	err := ts.SaveCuePoints(ctx, "nonexistent", store.CuePoints{})
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestListNullCueIn(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	// Insert 3 tracks; set cue_in_ms on 1 of them.
	for i, path := range []string{"/a.mp3", "/b.mp3", "/c.mp3"} {
		if err := ts.Upsert(ctx, store.Track{Path: path, Title: path, Type: "MUSIC", DurationMS: int64(i+1) * 1000}); err != nil {
			t.Fatal(err)
		}
	}
	trA, err := ts.FindByPath(ctx, "/a.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.SetCueIn(ctx, trA.ID, 200); err != nil {
		t.Fatal(err)
	}

	ids, err := ts.ListNullCueIn(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Errorf("ListNullCueIn returned %d ids, want 2", len(ids))
	}
}

func TestCountCueInStatus(t *testing.T) {
	ts := store.NewTrackStore(openMemDB(t))
	ctx := context.Background()

	for _, path := range []string{"/x.mp3", "/y.mp3", "/z.mp3"} {
		if err := ts.Upsert(ctx, store.Track{Path: path, Title: path, Type: "MUSIC", DurationMS: 3000}); err != nil {
			t.Fatal(err)
		}
	}
	trX, err := ts.FindByPath(ctx, "/x.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.SetCueIn(ctx, trX.ID, 100); err != nil {
		t.Fatal(err)
	}

	counts, err := ts.CountCueInStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts["pending"] != 2 {
		t.Errorf("pending = %d, want 2", counts["pending"])
	}
	if counts["done"] != 1 {
		t.Errorf("done = %d, want 1", counts["done"])
	}
}
