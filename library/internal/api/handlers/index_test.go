package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
	"github.com/Waelson/radio-library-service/internal/indexsvc"
)

// ─── fake index service ───────────────────────────────────────────────────────

type fakeIndexService struct {
	status indexsvc.Status
	err    error
	scanErr error
}

func (f *fakeIndexService) Status(_ context.Context) (indexsvc.Status, error) {
	return f.status, f.err
}

func (f *fakeIndexService) TriggerScan(_ context.Context) error {
	return f.scanErr
}

// ─── GetIndexStatus ───────────────────────────────────────────────────────────

func TestGetIndexStatus_OK(t *testing.T) {
	now := time.Now()
	svc := &fakeIndexService{
		status: indexsvc.Status{
			Running:     false,
			TotalTracks: 120,
			LastRunAt:   &now,
			LastIndexed: 115,
			LastSkipped: 5,
			LastErrors:  0,
		},
	}

	req := httptest.NewRequest("GET", "/v1/index/status", nil)
	w := httptest.NewRecorder()
	handlers.GetIndexStatus(svc)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}

	body := decodeBody(t, w)
	if body["total_tracks"].(float64) != 120 {
		t.Errorf("total_tracks = %v", body["total_tracks"])
	}
	if body["running"].(bool) {
		t.Error("running should be false")
	}
	if body["last_indexed"].(float64) != 115 {
		t.Errorf("last_indexed = %v", body["last_indexed"])
	}
}

func TestGetIndexStatus_ServiceError(t *testing.T) {
	svc := &fakeIndexService{err: errors.New("db error")}
	req := httptest.NewRequest("GET", "/v1/index/status", nil)
	w := httptest.NewRecorder()
	handlers.GetIndexStatus(svc)(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestGetIndexStatus_Running(t *testing.T) {
	svc := &fakeIndexService{status: indexsvc.Status{Running: true, TotalTracks: 50}}
	req := httptest.NewRequest("GET", "/v1/index/status", nil)
	w := httptest.NewRecorder()
	handlers.GetIndexStatus(svc)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := decodeBody(t, w)
	if !body["running"].(bool) {
		t.Error("running should be true")
	}
}

// ─── TriggerScan ─────────────────────────────────────────────────────────────

func TestTriggerScan_Started(t *testing.T) {
	svc := &fakeIndexService{}
	req := httptest.NewRequest("POST", "/v1/index/scan", nil)
	w := httptest.NewRecorder()
	handlers.TriggerScan(svc)(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d — %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["status"] != "scan_started" {
		t.Errorf("status = %v", body["status"])
	}
}

func TestTriggerScan_AlreadyRunning(t *testing.T) {
	svc := &fakeIndexService{scanErr: indexsvc.ErrAlreadyRunning}
	req := httptest.NewRequest("POST", "/v1/index/scan", nil)
	w := httptest.NewRecorder()
	handlers.TriggerScan(svc)(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
	body := decodeBody(t, w)
	if body["error"] != "already_running" {
		t.Errorf("error code = %v", body["error"])
	}
}

func TestTriggerScan_ServiceError(t *testing.T) {
	svc := &fakeIndexService{scanErr: errors.New("internal")}
	req := httptest.NewRequest("POST", "/v1/index/scan", nil)
	w := httptest.NewRecorder()
	handlers.TriggerScan(svc)(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}
