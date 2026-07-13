package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Waelson/radio-library-service/internal/store"
)

// LoudnessWorker is the subset of *loudness.Worker used by loudness handlers.
type LoudnessWorker interface {
	Enqueue(id string)
	IsRunning() bool
	Status(ctx context.Context) (map[string]int, error)
	DrainQueue()
}

// LoudnessTrackStore is the subset of *store.TrackStore used by loudness handlers.
type LoudnessTrackStore interface {
	FindByID(ctx context.Context, id string) (store.Track, error)
	ListPendingLoudness(ctx context.Context, limit int) ([]string, error)
}

// GetLoudnessStatus handles GET /v1/loudness/status.
// Returns per-status track counts and whether the worker is running.
func GetLoudnessStatus(lw LoudnessWorker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		counts, err := lw.Status(r.Context())
		if err != nil {
			slog.Error("GetLoudnessStatus: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "status query failed")
			return
		}
		// Ensure all four statuses are present in the response (even if zero).
		for _, s := range []string{"pending", "analyzing", "done", "error"} {
			if _, ok := counts[s]; !ok {
				counts[s] = 0
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"running": lw.IsRunning(),
			"counts":  counts,
		})
	}
}

// ReanalyzeAll handles POST /v1/loudness/analyze.
// Re-enqueues all tracks whose loudness_status is 'pending' or 'error'.
func ReanalyzeAll(lw LoudnessWorker, ts LoudnessTrackStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, err := ts.ListPendingLoudness(r.Context(), 50_000)
		if err != nil {
			slog.Error("ReanalyzeAll: list pending error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not list pending tracks")
			return
		}
		for _, id := range ids {
			lw.Enqueue(id)
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"enqueued": len(ids),
		})
	}
}

// ReanalyzeTrack handles POST /v1/loudness/analyze/{id}.
// Re-enqueues a single track for loudness analysis.
func ReanalyzeTrack(lw LoudnessWorker, ts LoudnessTrackStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "id is required")
			return
		}
		if _, err := ts.FindByID(r.Context(), id); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "track not found")
			return
		} else if err != nil {
			slog.Error("ReanalyzeTrack: FindByID error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "lookup failed")
			return
		}
		lw.Enqueue(id)
		writeJSON(w, http.StatusAccepted, map[string]any{"id": id})
	}
}

// CancelLoudness handles DELETE /v1/loudness/analyze.
// Drains the in-memory queue, discarding tracks not yet dispatched to ffmpeg.
func CancelLoudness(lw LoudnessWorker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lw.DrainQueue()
		writeJSON(w, http.StatusOK, map[string]any{"cancelled": true})
	}
}
