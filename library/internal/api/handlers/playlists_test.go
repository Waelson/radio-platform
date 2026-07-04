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

// ─── fake playlist store ──────────────────────────────────────────────────────

type fakePlaylistStore struct {
	playlists map[string]*store.Playlist
	nextID    int
	err       error
}

func newFakePlaylistStore() *fakePlaylistStore {
	return &fakePlaylistStore{playlists: make(map[string]*store.Playlist)}
}

func (f *fakePlaylistStore) genID() string {
	f.nextID++
	return fmt.Sprintf("pl-%d", f.nextID)
}

func (f *fakePlaylistStore) Create(_ context.Context, name, category string) (store.Playlist, error) {
	if f.err != nil {
		return store.Playlist{}, f.err
	}
	if strings.TrimSpace(name) == "" {
		return store.Playlist{}, fmt.Errorf("name required")
	}
	pl := store.Playlist{
		ID:        f.genID(),
		Name:      name,
		Category:  category,
		Items:     []store.PlaylistItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.playlists[pl.ID] = &pl
	return pl, nil
}

func (f *fakePlaylistStore) FindByID(_ context.Context, id string) (store.Playlist, error) {
	if f.err != nil {
		return store.Playlist{}, f.err
	}
	p, ok := f.playlists[id]
	if !ok {
		return store.Playlist{}, store.ErrNotFound
	}
	return *p, nil
}

func (f *fakePlaylistStore) List(_ context.Context) ([]store.Playlist, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]store.Playlist, 0, len(f.playlists))
	for _, p := range f.playlists {
		out = append(out, *p)
	}
	return out, nil
}

func (f *fakePlaylistStore) Update(_ context.Context, id string, patch store.PlaylistPatch) error {
	if f.err != nil {
		return f.err
	}
	p, ok := f.playlists[id]
	if !ok {
		return store.ErrNotFound
	}
	if patch.Name != nil {
		p.Name = *patch.Name
	}
	if patch.Category != nil {
		p.Category = *patch.Category
	}
	return nil
}

func (f *fakePlaylistStore) Delete(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	delete(f.playlists, id)
	return nil
}

func (f *fakePlaylistStore) AddItem(_ context.Context, playlistID, trackID string) (store.PlaylistItem, error) {
	if f.err != nil {
		return store.PlaylistItem{}, f.err
	}
	p, ok := f.playlists[playlistID]
	if !ok {
		return store.PlaylistItem{}, store.ErrNotFound
	}
	item := store.PlaylistItem{
		ID:       fmt.Sprintf("item-%d", len(p.Items)+1),
		TrackID:  trackID,
		Position: len(p.Items) + 1,
		Track:    store.Track{ID: trackID, Title: "Track " + trackID, Type: "MUSIC"},
	}
	p.Items = append(p.Items, item)
	p.ItemCount = len(p.Items)
	return item, nil
}

func (f *fakePlaylistStore) RemoveItem(_ context.Context, itemID string) error {
	if f.err != nil {
		return f.err
	}
	for _, p := range f.playlists {
		for i, it := range p.Items {
			if it.ID == itemID {
				p.Items = append(p.Items[:i], p.Items[i+1:]...)
				p.ItemCount = len(p.Items)
				return nil
			}
		}
	}
	return nil // idempotent
}

func (f *fakePlaylistStore) ReorderItems(_ context.Context, playlistID string, itemIDs []string) error {
	if f.err != nil {
		return f.err
	}
	p, ok := f.playlists[playlistID]
	if !ok {
		return store.ErrNotFound
	}
	byID := map[string]store.PlaylistItem{}
	for _, it := range p.Items {
		byID[it.ID] = it
	}
	reordered := make([]store.PlaylistItem, 0, len(itemIDs))
	for i, id := range itemIDs {
		it, ok := byID[id]
		if !ok {
			return fmt.Errorf("item %q not found", id)
		}
		it.Position = i + 1
		reordered = append(reordered, it)
	}
	p.Items = reordered
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func doPlaylist(t *testing.T, handler http.HandlerFunc, method, target, body string, pathValues map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func decodeAny(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v — %s", err, w.Body.String())
	}
	return m
}

// ─── ListPlaylists ────────────────────────────────────────────────────────────

func TestListPlaylists_Empty(t *testing.T) {
	w := doPlaylist(t, handlers.ListPlaylists(newFakePlaylistStore()), "GET", "/v1/playlists", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	b := decodeAny(t, w)
	if b["playlists"] == nil {
		t.Error("playlists must not be null")
	}
}

func TestListPlaylists_WithData(t *testing.T) {
	fs := newFakePlaylistStore()
	ctx := context.Background()
	_, _ = fs.Create(ctx, "PL A", "")
	_, _ = fs.Create(ctx, "PL B", "Rock")

	w := doPlaylist(t, handlers.ListPlaylists(fs), "GET", "/v1/playlists", "", nil)
	b := decodeAny(t, w)
	if b["count"].(float64) != 2 {
		t.Errorf("count = %v", b["count"])
	}
}

// ─── CreatePlaylist ───────────────────────────────────────────────────────────

func TestCreatePlaylist_Success(t *testing.T) {
	w := doPlaylist(t, handlers.CreatePlaylist(newFakePlaylistStore()),
		"POST", "/v1/playlists", `{"name":"Manhã Feliz","category":"Pop"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	b := decodeAny(t, w)
	if b["name"] != "Manhã Feliz" {
		t.Errorf("name = %v", b["name"])
	}
	if b["items"] == nil {
		t.Error("items must be present")
	}
}

func TestCreatePlaylist_EmptyName(t *testing.T) {
	w := doPlaylist(t, handlers.CreatePlaylist(newFakePlaylistStore()),
		"POST", "/v1/playlists", `{"name":""}`, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCreatePlaylist_BadJSON(t *testing.T) {
	w := doPlaylist(t, handlers.CreatePlaylist(newFakePlaylistStore()),
		"POST", "/v1/playlists", `{bad}`, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ─── GetPlaylist ──────────────────────────────────────────────────────────────

func TestGetPlaylist_Found(t *testing.T) {
	fs := newFakePlaylistStore()
	pl, _ := fs.Create(context.Background(), "Test", "")
	w := doPlaylist(t, handlers.GetPlaylist(fs), "GET", "/v1/playlists/"+pl.ID, "", map[string]string{"id": pl.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	b := decodeAny(t, w)
	if b["id"] != pl.ID {
		t.Errorf("id = %v", b["id"])
	}
}

func TestGetPlaylist_NotFound(t *testing.T) {
	w := doPlaylist(t, handlers.GetPlaylist(newFakePlaylistStore()),
		"GET", "/v1/playlists/ghost", "", map[string]string{"id": "ghost"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

// ─── UpdatePlaylist ───────────────────────────────────────────────────────────

func TestUpdatePlaylist_Success(t *testing.T) {
	fs := newFakePlaylistStore()
	pl, _ := fs.Create(context.Background(), "Old", "")
	w := doPlaylist(t, handlers.UpdatePlaylist(fs), "PUT", "/v1/playlists/"+pl.ID,
		`{"name":"New","category":"Sertanejo"}`, map[string]string{"id": pl.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	b := decodeAny(t, w)
	if b["name"] != "New" {
		t.Errorf("name = %v", b["name"])
	}
}

func TestUpdatePlaylist_NotFound(t *testing.T) {
	w := doPlaylist(t, handlers.UpdatePlaylist(newFakePlaylistStore()),
		"PUT", "/v1/playlists/ghost", `{"name":"X"}`, map[string]string{"id": "ghost"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

// ─── DeletePlaylist ───────────────────────────────────────────────────────────

func TestDeletePlaylist(t *testing.T) {
	fs := newFakePlaylistStore()
	pl, _ := fs.Create(context.Background(), "Gone", "")
	w := doPlaylist(t, handlers.DeletePlaylist(fs), "DELETE", "/v1/playlists/"+pl.ID, "", map[string]string{"id": pl.ID})
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}
}

// ─── AddPlaylistItem ──────────────────────────────────────────────────────────

func TestAddPlaylistItem_Success(t *testing.T) {
	fs := newFakePlaylistStore()
	pl, _ := fs.Create(context.Background(), "PL", "")
	w := doPlaylist(t, handlers.AddPlaylistItem(fs), "POST", "/v1/playlists/"+pl.ID+"/items",
		`{"track_id":"track-abc"}`, map[string]string{"id": pl.ID})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	b := decodeAny(t, w)
	if b["id"] == nil {
		t.Error("item id must be present")
	}
	if b["position"].(float64) != 1 {
		t.Errorf("position = %v", b["position"])
	}
}

func TestAddPlaylistItem_MissingTrackID(t *testing.T) {
	fs := newFakePlaylistStore()
	pl, _ := fs.Create(context.Background(), "PL", "")
	w := doPlaylist(t, handlers.AddPlaylistItem(fs), "POST", "/v1/playlists/"+pl.ID+"/items",
		`{}`, map[string]string{"id": pl.ID})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ─── RemovePlaylistItem ───────────────────────────────────────────────────────

func TestRemovePlaylistItem(t *testing.T) {
	fs := newFakePlaylistStore()
	pl, _ := fs.Create(context.Background(), "PL", "")
	item, _ := fs.AddItem(context.Background(), pl.ID, "t1")
	w := doPlaylist(t, handlers.RemovePlaylistItem(fs), "DELETE",
		"/v1/playlists/"+pl.ID+"/items/"+item.ID, "",
		map[string]string{"id": pl.ID, "item_id": item.ID})
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}
}

// ─── ReorderPlaylistItems ─────────────────────────────────────────────────────

func TestReorderPlaylistItems_Success(t *testing.T) {
	fs := newFakePlaylistStore()
	ctx := context.Background()
	pl, _ := fs.Create(ctx, "PL", "")
	i1, _ := fs.AddItem(ctx, pl.ID, "t1")
	i2, _ := fs.AddItem(ctx, pl.ID, "t2")

	body := fmt.Sprintf(`{"item_ids":[%q,%q]}`, i2.ID, i1.ID)
	w := doPlaylist(t, handlers.ReorderPlaylistItems(fs), "PUT",
		"/v1/playlists/"+pl.ID+"/items/reorder", body, map[string]string{"id": pl.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	b := decodeAny(t, w)
	items := b["items"].([]any)
	first := items[0].(map[string]any)
	if first["id"] != i2.ID {
		t.Errorf("first item after reorder = %v, want %v", first["id"], i2.ID)
	}
}

func TestReorderPlaylistItems_EmptyIDs(t *testing.T) {
	fs := newFakePlaylistStore()
	pl, _ := fs.Create(context.Background(), "PL", "")
	w := doPlaylist(t, handlers.ReorderPlaylistItems(fs), "PUT",
		"/v1/playlists/"+pl.ID+"/items/reorder", `{"item_ids":[]}`,
		map[string]string{"id": pl.ID})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}
