package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Waelson/radio-library-service/internal/indexsvc"
)

// IndexService is the subset of indexsvc.Service required by the index handlers.
type IndexService interface {
	Status(ctx context.Context) (indexsvc.Status, error)
	TriggerScan(ctx context.Context) error
}

// GetIndexStatus handles GET /v1/index/status.
func GetIndexStatus(svc IndexService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := svc.Status(r.Context())
		if err != nil {
			slog.Error("GetIndexStatus: error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "status fetch failed")
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

// TriggerScan handles POST /v1/index/scan.
// Returns 202 Accepted when a scan is started, 409 Conflict if one is already running.
func TriggerScan(svc IndexService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.TriggerScan(r.Context()); errors.Is(err, indexsvc.ErrAlreadyRunning) {
			writeError(w, http.StatusConflict, "already_running", "scan already in progress")
			return
		} else if err != nil {
			slog.Error("TriggerScan: error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "trigger scan failed")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "scan_started"})
	}
}
