package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
	"github.com/Waelson/radio-library-service/internal/store"
)

// ─── fake store ──────────────────────────────────────────────────────────────

type fakeTrackStore struct {
	tracks []store.Track
	err    error
}

func (f *fakeTrackStore) FindByID(_ context.Context, id string) (store.Track, error) {
	if f.err != nil {
		return store.Track{}, f.err
	}
	for _, t := range f.tracks {
		if t.ID == id {
			return t, nil
		}
	}
	return store.Track{}, store.ErrNotFound
}

func (f *fakeTrackStore) Search(_ context.Context, q store.SearchQuery) ([]store.Track, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []store.Track
	for _, t := range f.tracks {
		if q.Type != "" && t.Type != q.Type {
			continue
		}
		if q.Artist != "" && t.Artist != q.Artist {
			continue
		}
		if q.Q != "" && !strings.Contains(strings.ToLower(t.Title), strings.ToLower(q.Q)) &&
			!strings.Contains(strings.ToLower(t.Artist), strings.ToLower(q.Q)) {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeTrackStore) CountFiltered(_ context.Context, q store.SearchQuery) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	tracks, _ := f.Search(context.Background(), q)
	return len(tracks), nil
}

func (f *fakeTrackStore) ListArtists(_ context.Context, trackType string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	seen := map[string]struct{}{}
	var out []string
	for _, t := range f.tracks {
		if t.Artist == "" {
			continue
		}
		if trackType != "" && t.Type != trackType {
			continue
		}
		if _, ok := seen[t.Artist]; !ok {
			seen[t.Artist] = struct{}{}
			out = append(out, t.Artist)
		}
	}
	return out, nil
}

func (f *fakeTrackStore) UpdateMeta(_ context.Context, id string, patch store.TrackPatch) error {
	if f.err != nil {
		return f.err
	}
	for i, t := range f.tracks {
		if t.ID != id {
			continue
		}
		if patch.Title != nil {
			f.tracks[i].Title = *patch.Title
		}
		if patch.Artist != nil {
			f.tracks[i].Artist = *patch.Artist
		}
		if patch.Category != nil {
			f.tracks[i].Category = *patch.Category
		}
		if patch.Type != nil {
			f.tracks[i].Type = *patch.Type
		}
		return nil
	}
	return store.ErrNotFound
}

// ─── fake normalization reader ────────────────────────────────────────────────

type fakeNormalizationReader struct{}

func (f *fakeNormalizationReader) NormalizationSettings(_ context.Context) (store.NormalizationSettings, error) {
	return store.NormalizationSettings{
		Enabled:    true,
		TargetLUFS: -16.0,
		MaxGainDB:  12.0,
	}, nil
}

var fakeNR = &fakeNormalizationReader{}

// ─── helpers ─────────────────────────────────────────────────────────────────

func seedStore() *fakeTrackStore {
	return &fakeTrackStore{tracks: []store.Track{
		{ID: "id1", Path: "/m/1.mp3", Title: "Detalhes", Artist: "Roberto Carlos", Type: "MUSIC", DurationMS: 214000, IndexedAt: time.Now()},
		{ID: "id2", Path: "/m/2.mp3", Title: "Emoções", Artist: "Roberto Carlos", Type: "MUSIC", DurationMS: 180000, IndexedAt: time.Now()},
		{ID: "id3", Path: "/v/1.mp3", Title: "Abertura Manhã", Type: "VINHETA", DurationMS: 10000, IndexedAt: time.Now()},
	}}
}

func do(t *testing.T, handler http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body: %v — raw: %s", err, w.Body.String())
	}
	return m
}

// ─── SearchTracks ────────────────────────────────────────────────────────────

func TestSearchTracks_All(t *testing.T) {
	w := do(t, handlers.SearchTracks(seedStore(), fakeNR), "GET", "/v1/tracks", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := decodeBody(t, w)
	tracks := body["tracks"].([]any)
	if len(tracks) != 3 {
		t.Errorf("want 3 tracks, got %d", len(tracks))
	}
}

func TestSearchTracks_FilterByType(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/tracks?type=MUSIC", nil)
	w := httptest.NewRecorder()
	handlers.SearchTracks(seedStore(), fakeNR)(w, req)
	body := decodeBody(t, w)
	tracks := body["tracks"].([]any)
	if len(tracks) != 2 {
		t.Errorf("want 2 MUSIC tracks, got %d", len(tracks))
	}
}

func TestSearchTracks_FilterByQ(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/tracks?q=abertura", nil)
	w := httptest.NewRecorder()
	handlers.SearchTracks(seedStore(), fakeNR)(w, req)
	body := decodeBody(t, w)
	tracks := body["tracks"].([]any)
	if len(tracks) != 1 {
		t.Errorf("want 1 track matching 'abertura', got %d", len(tracks))
	}
}

func TestSearchTracks_StoreError(t *testing.T) {
	fs := &fakeTrackStore{err: fmt.Errorf("db down")}
	w := do(t, handlers.SearchTracks(fs, fakeNR), "GET", "/v1/tracks", "")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ─── GetTrack ────────────────────────────────────────────────────────────────

func TestGetTrack_Found(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/tracks/id1", nil)
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()
	handlers.GetTrack(seedStore(), fakeNR)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := decodeBody(t, w)
	if body["id"] != "id1" {
		t.Errorf("id = %v", body["id"])
	}
	if body["title"] != "Detalhes" {
		t.Errorf("title = %v", body["title"])
	}
}

func TestGetTrack_NotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/tracks/ghost", nil)
	req.SetPathValue("id", "ghost")
	w := httptest.NewRecorder()
	handlers.GetTrack(seedStore(), fakeNR)(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

// ─── PatchTrack ──────────────────────────────────────────────────────────────

func TestPatchTrack_Success(t *testing.T) {
	fs := seedStore()
	req := httptest.NewRequest("PATCH", "/v1/tracks/id1",
		strings.NewReader(`{"title":"Novo Título","category":"Sertanejo"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()
	handlers.PatchTrack(fs, fakeNR)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d — body: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["title"] != "Novo Título" {
		t.Errorf("title = %v", body["title"])
	}
	if body["category"] != "Sertanejo" {
		t.Errorf("category = %v", body["category"])
	}
	// Artist should be unchanged.
	if body["artist"] != "Roberto Carlos" {
		t.Errorf("artist changed unexpectedly: %v", body["artist"])
	}
}

func TestPatchTrack_InvalidType(t *testing.T) {
	req := httptest.NewRequest("PATCH", "/v1/tracks/id1",
		strings.NewReader(`{"type":"INVALID"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()
	handlers.PatchTrack(seedStore(), fakeNR)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestPatchTrack_NotFound(t *testing.T) {
	req := httptest.NewRequest("PATCH", "/v1/tracks/ghost",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "ghost")
	w := httptest.NewRecorder()
	handlers.PatchTrack(seedStore(), fakeNR)(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestPatchTrack_BadJSON(t *testing.T) {
	req := httptest.NewRequest("PATCH", "/v1/tracks/id1",
		strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()
	handlers.PatchTrack(seedStore(), fakeNR)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ─── ListArtists ─────────────────────────────────────────────────────────────

func TestListArtists_All(t *testing.T) {
	w := do(t, handlers.ListArtists(seedStore()), "GET", "/v1/tracks/artists", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := decodeBody(t, w)
	artists := body["artists"].([]any)
	// Roberto Carlos appears in MUSIC; VINHETA has no artist.
	if len(artists) != 1 {
		t.Errorf("want 1 artist, got %d: %v", len(artists), artists)
	}
}

func TestListArtists_FilterByType(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/tracks/artists?type=VINHETA", nil)
	w := httptest.NewRecorder()
	handlers.ListArtists(seedStore())(w, req)
	body := decodeBody(t, w)
	artists := body["artists"].([]any)
	if len(artists) != 0 {
		t.Errorf("want 0 VINHETA artists, got %d", len(artists))
	}
}

func TestListArtists_EmptyResult(t *testing.T) {
	fs := &fakeTrackStore{}
	w := do(t, handlers.ListArtists(fs), "GET", "/v1/tracks/artists", "")
	body := decodeBody(t, w)
	// Must return an array, not null.
	if body["artists"] == nil {
		t.Error("artists must not be null")
	}
}
