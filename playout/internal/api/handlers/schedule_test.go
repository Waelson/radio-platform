package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/scheduler"
)

// --- fake schedule manager ---------------------------------------------------

type fakeScheduleMgr struct {
	entries   map[string]scheduler.Entry
	addErr    error
	updateErr error
	seq       int
}

func newFakeScheduleMgr() *fakeScheduleMgr {
	return &fakeScheduleMgr{entries: make(map[string]scheduler.Entry)}
}

func (f *fakeScheduleMgr) Add(e scheduler.Entry) (string, error) {
	if f.addErr != nil {
		return "", f.addErr
	}
	f.seq++
	e.ID = fmt.Sprintf("sched_%03d", f.seq)
	e.CreatedAt = time.Now()
	f.entries[e.ID] = e
	return e.ID, nil
}

func (f *fakeScheduleMgr) Update(id string, e scheduler.Entry) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	old, ok := f.entries[id]
	if !ok {
		return fmt.Errorf("entry %q not found", id)
	}
	e.ID = id
	e.CreatedAt = old.CreatedAt
	f.entries[id] = e
	return nil
}

func (f *fakeScheduleMgr) Remove(id string)          { delete(f.entries, id) }
func (f *fakeScheduleMgr) Enable(id string) bool     { return f.setEnabled(id, true) }
func (f *fakeScheduleMgr) Disable(id string) bool    { return f.setEnabled(id, false) }
func (f *fakeScheduleMgr) setEnabled(id string, v bool) bool {
	e, ok := f.entries[id]
	if !ok {
		return false
	}
	e.Enabled = v
	f.entries[id] = e
	return true
}
func (f *fakeScheduleMgr) Get(id string) (scheduler.Entry, bool) {
	e, ok := f.entries[id]
	return e, ok
}
func (f *fakeScheduleMgr) List() []scheduler.Entry {
	out := make([]scheduler.Entry, 0, len(f.entries))
	for _, e := range f.entries {
		out = append(out, e)
	}
	return out
}
func (f *fakeScheduleMgr) NextFireAt(id string) *time.Time { return nil }

// seed adds a pre-built entry and returns its ID.
func (f *fakeScheduleMgr) seed(name string) string {
	e := scheduler.Entry{
		Name:        name,
		Enabled:     true,
		CronExpr:    "0 * * * *",
		TriggerMode: scheduler.TriggerAfterCurrent,
		Item:        commands.QueueItemInput{Path: "/media/jingle.mp3", Type: "JINGLE", Title: name},
		CreatedAt:   time.Now(),
	}
	id, _ := f.Add(e)
	return id
}

// --- helper ------------------------------------------------------------------

func schedReq(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		data, _ := json.Marshal(body)
		buf = bytes.NewBuffer(data)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	// Simulate PathValue for {id} patterns used in handlers.
	// Go 1.22+ mux sets these; in tests we set them manually.
	return req
}

func withPathValue(r *http.Request, key, val string) *http.Request {
	r.SetPathValue(key, val)
	return r
}

// --- POST /v1/schedule -------------------------------------------------------

func TestScheduleAdd_OK(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":         "Jingle",
		"enabled":      true,
		"cron_expr":    "0 * * * *",
		"trigger_mode": "AFTER_CURRENT",
		"item":         map[string]any{"path": "/media/jingle.mp3", "type": "JINGLE"},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["ok"] != true {
		t.Errorf("ok = %v, want true", resp["ok"])
	}
	entry := resp["entry"].(map[string]any)
	if entry["id"] == "" {
		t.Error("entry.id should not be empty")
	}
	if entry["trigger_mode"] != "AFTER_CURRENT" {
		t.Errorf("trigger_mode = %v, want AFTER_CURRENT", entry["trigger_mode"])
	}
}

func TestScheduleAdd_DefaultTriggerMode(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":      "No mode",
		"enabled":   true,
		"cron_expr": "0 * * * *",
		"item":      map[string]any{"path": "/media/jingle.mp3"},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	entry := resp["entry"].(map[string]any)
	if entry["trigger_mode"] != "AFTER_CURRENT" {
		t.Errorf("default trigger_mode = %v, want AFTER_CURRENT", entry["trigger_mode"])
	}
}

func TestScheduleAdd_MissingCronAndFireAt(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":    "bad",
		"enabled": true,
		"item":    map[string]any{"path": "/media/x.mp3"},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestScheduleAdd_BothCronAndFireAt(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	fireAt := time.Now().Add(time.Hour)
	body := map[string]any{
		"name":      "conflict",
		"enabled":   true,
		"cron_expr": "0 * * * *",
		"fire_at":   fireAt,
		"item":      map[string]any{"path": "/media/x.mp3"},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestScheduleAdd_MissingItemPath(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":      "no path",
		"cron_expr": "0 * * * *",
		"item":      map[string]any{"type": "JINGLE"},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestScheduleAdd_InvalidTriggerMode(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":         "bad mode",
		"cron_expr":    "0 * * * *",
		"trigger_mode": "NOT_VALID",
		"item":         map[string]any{"path": "/media/x.mp3"},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestScheduleAdd_ManagerError(t *testing.T) {
	mgr := newFakeScheduleMgr()
	mgr.addErr = fmt.Errorf("invalid cron expression \"bad\": ...")
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":      "err",
		"cron_expr": "0 * * * *",
		"item":      map[string]any{"path": "/media/x.mp3"},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// --- GET /v1/schedule --------------------------------------------------------

func TestScheduleList_Empty(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()
	handlers.ScheduleList(mgr).ServeHTTP(rr, schedReq(t, http.MethodGet, "/v1/schedule", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["count"].(float64) != 0 {
		t.Errorf("count = %v, want 0", resp["count"])
	}
}

func TestScheduleList_WithEntries(t *testing.T) {
	mgr := newFakeScheduleMgr()
	mgr.seed("Jingle")
	mgr.seed("News")
	rr := httptest.NewRecorder()
	handlers.ScheduleList(mgr).ServeHTTP(rr, schedReq(t, http.MethodGet, "/v1/schedule", nil))

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["count"].(float64) != 2 {
		t.Errorf("count = %v, want 2", resp["count"])
	}
}

// --- GET /v1/schedule/{id} ---------------------------------------------------

func TestScheduleGet_Found(t *testing.T) {
	mgr := newFakeScheduleMgr()
	id := mgr.seed("Jingle")
	rr := httptest.NewRecorder()
	req := withPathValue(schedReq(t, http.MethodGet, "/v1/schedule/"+id, nil), "id", id)
	handlers.ScheduleGet(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Errorf("ok = %v, want true", resp["ok"])
	}
	entry := resp["entry"].(map[string]any)
	if entry["id"] != id {
		t.Errorf("entry.id = %v, want %v", entry["id"], id)
	}
}

func TestScheduleGet_NotFound(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()
	req := withPathValue(schedReq(t, http.MethodGet, "/v1/schedule/missing", nil), "id", "missing")
	handlers.ScheduleGet(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// --- PUT /v1/schedule/{id} ---------------------------------------------------

func TestScheduleUpdate_OK(t *testing.T) {
	mgr := newFakeScheduleMgr()
	id := mgr.seed("Old Name")
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":         "New Name",
		"enabled":      true,
		"cron_expr":    "0 8 * * *",
		"trigger_mode": "INTERRUPT",
		"item":         map[string]any{"path": "/media/news.mp3", "type": "SPOT"},
	}
	req := withPathValue(schedReq(t, http.MethodPut, "/v1/schedule/"+id, body), "id", id)
	handlers.ScheduleUpdate(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	entry := resp["entry"].(map[string]any)
	if entry["name"] != "New Name" {
		t.Errorf("name = %v, want 'New Name'", entry["name"])
	}
	if entry["trigger_mode"] != "INTERRUPT" {
		t.Errorf("trigger_mode = %v, want INTERRUPT", entry["trigger_mode"])
	}
}

func TestScheduleUpdate_NotFound(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":      "X",
		"cron_expr": "0 * * * *",
		"item":      map[string]any{"path": "/media/x.mp3"},
	}
	req := withPathValue(schedReq(t, http.MethodPut, "/v1/schedule/ghost", body), "id", "ghost")
	handlers.ScheduleUpdate(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestScheduleUpdate_InvalidPayload(t *testing.T) {
	mgr := newFakeScheduleMgr()
	id := mgr.seed("Entry")
	rr := httptest.NewRecorder()

	// Missing both cron_expr and fire_at.
	body := map[string]any{
		"name": "bad",
		"item": map[string]any{"path": "/media/x.mp3"},
	}
	req := withPathValue(schedReq(t, http.MethodPut, "/v1/schedule/"+id, body), "id", id)
	handlers.ScheduleUpdate(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// --- DELETE /v1/schedule/{id} ------------------------------------------------

func TestScheduleDelete_OK(t *testing.T) {
	mgr := newFakeScheduleMgr()
	id := mgr.seed("Spot")
	rr := httptest.NewRecorder()
	req := withPathValue(schedReq(t, http.MethodDelete, "/v1/schedule/"+id, nil), "id", id)
	handlers.ScheduleDelete(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	// Verify the entry was actually removed.
	if _, ok := mgr.Get(id); ok {
		t.Error("entry should have been removed")
	}
}

func TestScheduleDelete_NotFound(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()
	req := withPathValue(schedReq(t, http.MethodDelete, "/v1/schedule/nope", nil), "id", "nope")
	handlers.ScheduleDelete(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// --- POST /v1/schedule/{id}/enable & /disable --------------------------------

func TestScheduleEnable_OK(t *testing.T) {
	mgr := newFakeScheduleMgr()
	id := mgr.seed("Entry")
	mgr.Disable(id) // start disabled

	rr := httptest.NewRecorder()
	req := withPathValue(schedReq(t, http.MethodPost, "/v1/schedule/"+id+"/enable", nil), "id", id)
	handlers.ScheduleEnable(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["enabled"] != true {
		t.Errorf("enabled = %v, want true", resp["enabled"])
	}

	e, _ := mgr.Get(id)
	if !e.Enabled {
		t.Error("entry should be enabled")
	}
}

func TestScheduleDisable_OK(t *testing.T) {
	mgr := newFakeScheduleMgr()
	id := mgr.seed("Entry")

	rr := httptest.NewRecorder()
	req := withPathValue(schedReq(t, http.MethodPost, "/v1/schedule/"+id+"/disable", nil), "id", id)
	handlers.ScheduleDisable(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["enabled"] != false {
		t.Errorf("enabled = %v, want false", resp["enabled"])
	}
}

func TestScheduleEnable_NotFound(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()
	req := withPathValue(schedReq(t, http.MethodPost, "/v1/schedule/ghost/enable", nil), "id", "ghost")
	handlers.ScheduleEnable(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestScheduleDisable_NotFound(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()
	req := withPathValue(schedReq(t, http.MethodPost, "/v1/schedule/ghost/disable", nil), "id", "ghost")
	handlers.ScheduleDisable(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// --- Break entries -----------------------------------------------------------

func TestScheduleAdd_Break_Valid(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":         "Bloco Comercial 10h30",
		"enabled":      true,
		"cron_expr":    "30 10 * * *",
		"trigger_mode": "AFTER_CURRENT",
		"break": map[string]any{
			"title": "Bloco das 10h30",
			"open":  map[string]any{"path": "/lib/open.mp3", "type": "jingle"},
			"spots": []any{
				map[string]any{"path": "/lib/spot-a.mp3", "type": "spot", "title": "Spot A"},
				map[string]any{"path": "/lib/spot-b.mp3", "type": "spot", "title": "Spot B"},
			},
			"close": map[string]any{"path": "/lib/close.mp3", "type": "jingle"},
		},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["ok"] != true {
		t.Errorf("ok = %v, want true", resp["ok"])
	}
	entry := resp["entry"].(map[string]any)
	if entry["item"] != nil {
		t.Errorf("break entry should not have 'item' in view, got: %v", entry["item"])
	}
	brk, ok := entry["break"].(map[string]any)
	if !ok || brk == nil {
		t.Fatalf("entry.break should be present, got: %v", entry["break"])
	}
	if brk["title"] != "Bloco das 10h30" {
		t.Errorf("break.title = %v, want 'Bloco das 10h30'", brk["title"])
	}
	spots, _ := brk["spots"].([]any)
	if len(spots) != 2 {
		t.Errorf("break.spots len = %d, want 2", len(spots))
	}
}

func TestScheduleAdd_ItemAndBreak_Rejected(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":      "conflict",
		"cron_expr": "0 * * * *",
		"item":      map[string]any{"path": "/media/x.mp3"},
		"break": map[string]any{
			"title": "Bloco",
			"spots": []any{map[string]any{"path": "/lib/spot.mp3"}},
		},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestScheduleAdd_Break_NoSpots_Rejected(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":      "empty break",
		"cron_expr": "0 * * * *",
		"break": map[string]any{
			"title": "Bloco Vazio",
			"spots": []any{},
		},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestScheduleAdd_Break_SpotMissingPath_Rejected(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":      "spot no path",
		"cron_expr": "0 * * * *",
		"break": map[string]any{
			"title": "Bloco",
			"spots": []any{
				map[string]any{"type": "spot", "title": "No path here"},
			},
		},
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestScheduleAdd_NeitherItemNorBreak_Rejected(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()

	body := map[string]any{
		"name":      "nothing",
		"cron_expr": "0 * * * *",
	}
	handlers.ScheduleAdd(mgr).ServeHTTP(rr, schedReq(t, http.MethodPost, "/v1/schedule", body))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// --- Content-Type ------------------------------------------------------------

func TestScheduleHandlers_ContentType(t *testing.T) {
	mgr := newFakeScheduleMgr()
	rr := httptest.NewRecorder()
	handlers.ScheduleList(mgr).ServeHTTP(rr, schedReq(t, http.MethodGet, "/v1/schedule", nil))

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
