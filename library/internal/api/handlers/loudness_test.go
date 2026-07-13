package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

// ─── mocks ────────────────────────────────────────────────────────────────────

type mockLoudnessWorker struct {
	enqueued  []string
	running   bool
	counts    map[string]int
	countErr  error
	drained   bool
}

func (m *mockLoudnessWorker) Enqueue(id string)        { m.enqueued = append(m.enqueued, id) }
func (m *mockLoudnessWorker) IsRunning() bool           { return m.running }
func (m *mockLoudnessWorker) DrainQueue()               { m.drained = true }
func (m *mockLoudnessWorker) Status(_ context.Context) (map[string]int, error) {
	return m.counts, m.countErr
}

type mockLoudnessTrackStore struct {
	tracks  map[string]store.Track
	pending []string
}

func (m *mockLoudnessTrackStore) FindByID(_ context.Context, id string) (store.Track, error) {
	t, ok := m.tracks[id]
	if !ok {
		return store.Track{}, store.ErrNotFound
	}
	return t, nil
}

func (m *mockLoudnessTrackStore) ListPendingLoudness(_ context.Context, _ int) ([]string, error) {
	return m.pending, nil
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestGetLoudnessStatus_OK(t *testing.T) {
	lw := &mockLoudnessWorker{
		running: true,
		counts:  map[string]int{"done": 10, "pending": 3},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/loudness/status", nil)
	rec := httptest.NewRecorder()

	GetLoudnessStatus(lw)(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"running":true`) {
		t.Errorf("body missing running=true: %s", body)
	}
	if !strings.Contains(body, `"done":10`) {
		t.Errorf("body missing done:10: %s", body)
	}
	// All four statuses should be present.
	for _, s := range []string{"pending", "analyzing", "done", "error"} {
		if !strings.Contains(body, `"`+s+`"`) {
			t.Errorf("body missing status %q: %s", s, body)
		}
	}
}

func TestGetLoudnessStatus_StoreError(t *testing.T) {
	lw := &mockLoudnessWorker{countErr: errors.New("db dead")}
	req := httptest.NewRequest(http.MethodGet, "/v1/loudness/status", nil)
	rec := httptest.NewRecorder()

	GetLoudnessStatus(lw)(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestReanalyzeAll_EnqueuesAllPending(t *testing.T) {
	lw := &mockLoudnessWorker{}
	ts := &mockLoudnessTrackStore{pending: []string{"a", "b", "c"}}

	req := httptest.NewRequest(http.MethodPost, "/v1/loudness/analyze", nil)
	rec := httptest.NewRecorder()

	ReanalyzeAll(lw, ts)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
	if len(lw.enqueued) != 3 {
		t.Errorf("enqueued %d, want 3", len(lw.enqueued))
	}
}

func TestReanalyzeTrack_OK(t *testing.T) {
	lw := &mockLoudnessWorker{}
	ts := &mockLoudnessTrackStore{
		tracks: map[string]store.Track{"t1": {ID: "t1"}},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/loudness/analyze/t1", nil)
	req.SetPathValue("id", "t1")
	rec := httptest.NewRecorder()

	ReanalyzeTrack(lw, ts)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
	if len(lw.enqueued) != 1 || lw.enqueued[0] != "t1" {
		t.Errorf("enqueued = %v, want [t1]", lw.enqueued)
	}
}

func TestReanalyzeTrack_NotFound(t *testing.T) {
	lw := &mockLoudnessWorker{}
	ts := &mockLoudnessTrackStore{tracks: map[string]store.Track{}}

	req := httptest.NewRequest(http.MethodPost, "/v1/loudness/analyze/ghost", nil)
	req.SetPathValue("id", "ghost")
	rec := httptest.NewRecorder()

	ReanalyzeTrack(lw, ts)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if len(lw.enqueued) != 0 {
		t.Error("should not enqueue unknown track")
	}
}

func TestCancelLoudness_DrainsQueue(t *testing.T) {
	lw := &mockLoudnessWorker{}

	req := httptest.NewRequest(http.MethodDelete, "/v1/loudness/analyze", nil)
	rec := httptest.NewRecorder()

	CancelLoudness(lw)(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !lw.drained {
		t.Error("DrainQueue was not called")
	}
}
