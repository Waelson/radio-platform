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

// ─── fake break store ─────────────────────────────────────────────────────────

type fakeBreakStore struct {
	breaks map[string]*store.Break
	nextID int
	err    error
}

func newFakeBreakStore() *fakeBreakStore {
	return &fakeBreakStore{breaks: make(map[string]*store.Break)}
}

func (f *fakeBreakStore) genID() string {
	f.nextID++
	return fmt.Sprintf("brk-%d", f.nextID)
}

func (f *fakeBreakStore) Create(_ context.Context, name, openID, closeID string) (store.Break, error) {
	if f.err != nil {
		return store.Break{}, f.err
	}
	if strings.TrimSpace(name) == "" {
		return store.Break{}, fmt.Errorf("name required")
	}
	brk := store.Break{
		ID: f.genID(), Name: name,
		Items: []store.BreakItem{}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if openID != "" {
		brk.OpenTrack = &store.Track{ID: openID, Title: "Open", Type: "JINGLE"}
	}
	if closeID != "" {
		brk.CloseTrack = &store.Track{ID: closeID, Title: "Close", Type: "JINGLE"}
	}
	f.breaks[brk.ID] = &brk
	return brk, nil
}

func (f *fakeBreakStore) FindByID(_ context.Context, id string) (store.Break, error) {
	if f.err != nil {
		return store.Break{}, f.err
	}
	b, ok := f.breaks[id]
	if !ok {
		return store.Break{}, store.ErrNotFound
	}
	return *b, nil
}

func (f *fakeBreakStore) List(_ context.Context) ([]store.Break, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]store.Break, 0, len(f.breaks))
	for _, b := range f.breaks {
		out = append(out, *b)
	}
	return out, nil
}

func (f *fakeBreakStore) Update(_ context.Context, id string, patch store.BreakPatch) error {
	if f.err != nil {
		return f.err
	}
	b, ok := f.breaks[id]
	if !ok {
		return store.ErrNotFound
	}
	if patch.Name != nil {
		b.Name = *patch.Name
	}
	if patch.OpenTrackID != nil {
		if *patch.OpenTrackID == "" {
			b.OpenTrack = nil
		} else {
			b.OpenTrack = &store.Track{ID: *patch.OpenTrackID, Title: "Open", Type: "JINGLE"}
		}
	}
	if patch.CloseTrackID != nil {
		if *patch.CloseTrackID == "" {
			b.CloseTrack = nil
		} else {
			b.CloseTrack = &store.Track{ID: *patch.CloseTrackID, Title: "Close", Type: "JINGLE"}
		}
	}
	return nil
}

func (f *fakeBreakStore) Delete(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	delete(f.breaks, id)
	return nil
}

func (f *fakeBreakStore) AddItem(_ context.Context, breakID, trackID string) (store.BreakItem, error) {
	if f.err != nil {
		return store.BreakItem{}, f.err
	}
	b, ok := f.breaks[breakID]
	if !ok {
		return store.BreakItem{}, store.ErrNotFound
	}
	item := store.BreakItem{
		ID:       fmt.Sprintf("bi-%d", len(b.Items)+1),
		TrackID:  trackID,
		Position: len(b.Items) + 1,
		Track:    store.Track{ID: trackID, Title: "Spot " + trackID, Type: "SPOT"},
	}
	b.Items = append(b.Items, item)
	b.ItemCount = len(b.Items)
	return item, nil
}

func (f *fakeBreakStore) RemoveItem(_ context.Context, itemID string) error {
	if f.err != nil {
		return f.err
	}
	for _, b := range f.breaks {
		for i, it := range b.Items {
			if it.ID == itemID {
				b.Items = append(b.Items[:i], b.Items[i+1:]...)
				b.ItemCount = len(b.Items)
				return nil
			}
		}
	}
	return nil
}

func (f *fakeBreakStore) ReorderItems(_ context.Context, breakID string, itemIDs []string) error {
	if f.err != nil {
		return f.err
	}
	b, ok := f.breaks[breakID]
	if !ok {
		return store.ErrNotFound
	}
	byID := map[string]store.BreakItem{}
	for _, it := range b.Items {
		byID[it.ID] = it
	}
	reordered := make([]store.BreakItem, 0, len(itemIDs))
	for i, id := range itemIDs {
		it, ok := byID[id]
		if !ok {
			return fmt.Errorf("item %q not found", id)
		}
		it.Position = i + 1
		reordered = append(reordered, it)
	}
	b.Items = reordered
	return nil
}

// ─── helper ──────────────────────────────────────────────────────────────────

func doBreak(t *testing.T, handler http.HandlerFunc, method, target, body string, pv map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range pv {
		req.SetPathValue(k, v)
	}
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func decodeBreak(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v — %s", err, w.Body.String())
	}
	return m
}

// ─── ListBreaks ───────────────────────────────────────────────────────────────

func TestListBreaks_Empty(t *testing.T) {
	w := doBreak(t, handlers.ListBreaks(newFakeBreakStore()), "GET", "/v1/breaks", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	b := decodeBreak(t, w)
	if b["breaks"] == nil {
		t.Error("breaks must not be null")
	}
}

func TestListBreaks_WithData(t *testing.T) {
	fs := newFakeBreakStore()
	_, _ = fs.Create(context.Background(), "Break A", "", "")
	_, _ = fs.Create(context.Background(), "Break B", "", "")
	w := doBreak(t, handlers.ListBreaks(fs), "GET", "/v1/breaks", "", nil)
	b := decodeBreak(t, w)
	if b["count"].(float64) != 2 {
		t.Errorf("count = %v", b["count"])
	}
}

// ─── CreateBreak ──────────────────────────────────────────────────────────────

func TestCreateBreak_Success(t *testing.T) {
	w := doBreak(t, handlers.CreateBreak(newFakeBreakStore()),
		"POST", "/v1/breaks", `{"name":"Break Manhã"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	b := decodeBreak(t, w)
	if b["name"] != "Break Manhã" {
		t.Errorf("name = %v", b["name"])
	}
	if b["items"] == nil {
		t.Error("items must be present")
	}
}

func TestCreateBreak_WithOpenClose(t *testing.T) {
	w := doBreak(t, handlers.CreateBreak(newFakeBreakStore()),
		"POST", "/v1/breaks",
		`{"name":"Break","open_track_id":"ot1","close_track_id":"ct1"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	b := decodeBreak(t, w)
	if b["open_track"] == nil {
		t.Error("open_track must be present")
	}
	if b["close_track"] == nil {
		t.Error("close_track must be present")
	}
}

func TestCreateBreak_EmptyName(t *testing.T) {
	w := doBreak(t, handlers.CreateBreak(newFakeBreakStore()),
		"POST", "/v1/breaks", `{"name":""}`, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ─── GetBreak ────────────────────────────────────────────────────────────────

func TestGetBreak_Found(t *testing.T) {
	fs := newFakeBreakStore()
	brk, _ := fs.Create(context.Background(), "Break", "", "")
	w := doBreak(t, handlers.GetBreak(fs, fakeNR), "GET", "/v1/breaks/"+brk.ID, "",
		map[string]string{"id": brk.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestGetBreak_NotFound(t *testing.T) {
	w := doBreak(t, handlers.GetBreak(newFakeBreakStore(), fakeNR), "GET", "/v1/breaks/ghost", "",
		map[string]string{"id": "ghost"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestGetBreak_EnginePayload(t *testing.T) {
	fs := newFakeBreakStore()
	brk, _ := fs.Create(context.Background(), "Break Comercial", "ot1", "ct1")
	_, _ = fs.AddItem(context.Background(), brk.ID, "spot1")

	req := httptest.NewRequest("GET", "/v1/breaks/"+brk.ID+"?format=engine-payload", nil)
	req.SetPathValue("id", brk.ID)
	w := httptest.NewRecorder()
	handlers.GetBreak(fs, fakeNR)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["name"] != "Break Comercial" {
		t.Errorf("name = %v", payload["name"])
	}
	if payload["open"] == nil {
		t.Error("open must be present")
	}
	if payload["close"] == nil {
		t.Error("close must be present")
	}
	spots, ok := payload["spots"].([]any)
	if !ok || len(spots) != 1 {
		t.Errorf("spots = %v", payload["spots"])
	}
	// Engine payload must have path, title, type, duration_ms fields.
	open := payload["open"].(map[string]any)
	if _, ok := open["path"]; !ok {
		t.Error("engine payload open must have 'path'")
	}
	if _, ok := open["duration_ms"]; !ok {
		t.Error("engine payload open must have 'duration_ms'")
	}
}

func TestGetBreak_EnginePayload_NullOpenClose(t *testing.T) {
	fs := newFakeBreakStore()
	brk, _ := fs.Create(context.Background(), "Break Simples", "", "")

	req := httptest.NewRequest("GET", "/v1/breaks/"+brk.ID+"?format=engine-payload", nil)
	req.SetPathValue("id", brk.ID)
	w := httptest.NewRecorder()
	handlers.GetBreak(fs, fakeNR)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var payload map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload["open"] != nil {
		t.Errorf("open should be null when not set, got %v", payload["open"])
	}
	if payload["close"] != nil {
		t.Errorf("close should be null when not set, got %v", payload["close"])
	}
}

// ─── UpdateBreak ──────────────────────────────────────────────────────────────

func TestUpdateBreak_Success(t *testing.T) {
	fs := newFakeBreakStore()
	brk, _ := fs.Create(context.Background(), "Old", "", "")
	w := doBreak(t, handlers.UpdateBreak(fs), "PUT", "/v1/breaks/"+brk.ID,
		`{"name":"New"}`, map[string]string{"id": brk.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	b := decodeBreak(t, w)
	if b["name"] != "New" {
		t.Errorf("name = %v", b["name"])
	}
}

func TestUpdateBreak_NotFound(t *testing.T) {
	w := doBreak(t, handlers.UpdateBreak(newFakeBreakStore()), "PUT", "/v1/breaks/ghost",
		`{"name":"X"}`, map[string]string{"id": "ghost"})
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

// ─── DeleteBreak ──────────────────────────────────────────────────────────────

func TestDeleteBreak(t *testing.T) {
	fs := newFakeBreakStore()
	brk, _ := fs.Create(context.Background(), "Gone", "", "")
	w := doBreak(t, handlers.DeleteBreak(fs), "DELETE", "/v1/breaks/"+brk.ID, "",
		map[string]string{"id": brk.ID})
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}
}

// ─── AddBreakItem ─────────────────────────────────────────────────────────────

func TestAddBreakItem_Success(t *testing.T) {
	fs := newFakeBreakStore()
	brk, _ := fs.Create(context.Background(), "Break", "", "")
	w := doBreak(t, handlers.AddBreakItem(fs), "POST", "/v1/breaks/"+brk.ID+"/items",
		`{"track_id":"spot-abc"}`, map[string]string{"id": brk.ID})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	b := decodeBreak(t, w)
	if b["position"].(float64) != 1 {
		t.Errorf("position = %v", b["position"])
	}
}

func TestAddBreakItem_MissingTrackID(t *testing.T) {
	fs := newFakeBreakStore()
	brk, _ := fs.Create(context.Background(), "Break", "", "")
	w := doBreak(t, handlers.AddBreakItem(fs), "POST", "/v1/breaks/"+brk.ID+"/items",
		`{}`, map[string]string{"id": brk.ID})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ─── RemoveBreakItem ──────────────────────────────────────────────────────────

func TestRemoveBreakItem(t *testing.T) {
	fs := newFakeBreakStore()
	brk, _ := fs.Create(context.Background(), "Break", "", "")
	item, _ := fs.AddItem(context.Background(), brk.ID, "spot1")
	w := doBreak(t, handlers.RemoveBreakItem(fs), "DELETE",
		"/v1/breaks/"+brk.ID+"/items/"+item.ID, "",
		map[string]string{"id": brk.ID, "item_id": item.ID})
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}
}

// ─── ReorderBreakItems ────────────────────────────────────────────────────────

func TestReorderBreakItems_Success(t *testing.T) {
	fs := newFakeBreakStore()
	ctx := context.Background()
	brk, _ := fs.Create(ctx, "Break", "", "")
	i1, _ := fs.AddItem(ctx, brk.ID, "s1")
	i2, _ := fs.AddItem(ctx, brk.ID, "s2")

	body := fmt.Sprintf(`{"item_ids":[%q,%q]}`, i2.ID, i1.ID)
	w := doBreak(t, handlers.ReorderBreakItems(fs), "PUT",
		"/v1/breaks/"+brk.ID+"/items/reorder", body, map[string]string{"id": brk.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	b := decodeBreak(t, w)
	items := b["items"].([]any)
	first := items[0].(map[string]any)
	if first["id"] != i2.ID {
		t.Errorf("first after reorder = %v, want %v", first["id"], i2.ID)
	}
}

func TestReorderBreakItems_EmptyIDs(t *testing.T) {
	fs := newFakeBreakStore()
	brk, _ := fs.Create(context.Background(), "Break", "", "")
	w := doBreak(t, handlers.ReorderBreakItems(fs), "PUT",
		"/v1/breaks/"+brk.ID+"/items/reorder", `{"item_ids":[]}`,
		map[string]string{"id": brk.ID})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}
