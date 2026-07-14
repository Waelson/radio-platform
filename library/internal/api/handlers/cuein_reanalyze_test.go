package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── fakes ───────────────────────────────────────────────────────────────────

type fakeCueInWorker struct {
	enqueued  []string
	running   bool
	drained   bool
	statusMap map[string]int
	statusErr error
}

func (f *fakeCueInWorker) Enqueue(id string)                                  { f.enqueued = append(f.enqueued, id) }
func (f *fakeCueInWorker) IsRunning() bool                                    { return f.running }
func (f *fakeCueInWorker) DrainQueue()                                        { f.drained = true }
func (f *fakeCueInWorker) Status(_ context.Context) (map[string]int, error)   { return f.statusMap, f.statusErr }

type fakeCueInTrackStore struct {
	ids []string
	err error
}

func (f *fakeCueInTrackStore) ListNullCueIn(_ context.Context, _ int) ([]string, error) {
	return f.ids, f.err
}

// ─── GetCueInReanalyzeStatus ─────────────────────────────────────────────────

func TestGetCueInReanalyzeStatus_Running(t *testing.T) {
	w := &fakeCueInWorker{
		running:   true,
		statusMap: map[string]int{"pending": 10, "done": 5, "analyzing": 2, "error": 0},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/tracks/reanalyze-cuepoints/status", nil)
	rec := httptest.NewRecorder()
	GetCueInReanalyzeStatus(w).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"running":true`) {
		t.Errorf("body missing running=true: %s", body)
	}
}

func TestGetCueInReanalyzeStatus_EnsuresAllKeys(t *testing.T) {
	// Status returns only pending+done; handler must fill in analyzing+error.
	w := &fakeCueInWorker{
		statusMap: map[string]int{"pending": 3, "done": 7},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/tracks/reanalyze-cuepoints/status", nil)
	rec := httptest.NewRecorder()
	GetCueInReanalyzeStatus(w).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	counts := body["counts"].(map[string]any)
	for _, key := range []string{"pending", "analyzing", "done", "error"} {
		if _, ok := counts[key]; !ok {
			t.Errorf("counts missing key %q", key)
		}
	}
}

// ─── TriggerCueInReanalyze ───────────────────────────────────────────────────

func TestTriggerCueInReanalyze_EnqueuesNullTracks(t *testing.T) {
	cw := &fakeCueInWorker{}
	ts := &fakeCueInTrackStore{ids: []string{"id-1", "id-2", "id-3"}}

	req := httptest.NewRequest(http.MethodPost, "/v1/tracks/reanalyze-cuepoints", nil)
	rec := httptest.NewRecorder()
	TriggerCueInReanalyze(cw, ts).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if len(cw.enqueued) != 3 {
		t.Errorf("enqueued = %d, want 3", len(cw.enqueued))
	}
}

func TestTriggerCueInReanalyze_EmptyResult(t *testing.T) {
	cw := &fakeCueInWorker{}
	ts := &fakeCueInTrackStore{ids: nil}

	req := httptest.NewRequest(http.MethodPost, "/v1/tracks/reanalyze-cuepoints", nil)
	rec := httptest.NewRecorder()
	TriggerCueInReanalyze(cw, ts).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"enqueued":0`) {
		t.Errorf("expected enqueued:0 in body: %s", body)
	}
}

// ─── CancelCueInReanalyze ────────────────────────────────────────────────────

func TestCancelCueInReanalyze(t *testing.T) {
	cw := &fakeCueInWorker{}

	req := httptest.NewRequest(http.MethodDelete, "/v1/tracks/reanalyze-cuepoints", nil)
	rec := httptest.NewRecorder()
	CancelCueInReanalyze(cw).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !cw.drained {
		t.Error("expected DrainQueue to be called")
	}
}
