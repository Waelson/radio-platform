package handlers

import (
	"context"
	"log/slog"
	"net/http"
)

// CueInWorker is the subset of *cuein.Worker used by cue_in handlers.
type CueInWorker interface {
	Enqueue(id string)
	IsRunning() bool
	Status(ctx context.Context) (map[string]int, error)
	DrainQueue()
}

// CueInTrackStore is the subset of *store.TrackStore used by cue_in handlers.
type CueInTrackStore interface {
	ListNullCueIn(ctx context.Context, limit int) ([]string, error)
}

// GetCueInReanalyzeStatus handles GET /v1/tracks/reanalyze-cuepoints/status.
// Returns per-state track counts and whether the worker is running.
func GetCueInReanalyzeStatus(cw CueInWorker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		counts, err := cw.Status(r.Context())
		if err != nil {
			slog.Error("GetCueInReanalyzeStatus: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "status query failed")
			return
		}
		// Ensure all expected keys are present even if zero.
		for _, s := range []string{"pending", "analyzing", "done", "error"} {
			if _, ok := counts[s]; !ok {
				counts[s] = 0
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"running": cw.IsRunning(),
			"counts":  counts,
		})
	}
}

// TriggerCueInReanalyze handles POST /v1/tracks/reanalyze-cuepoints.
// Enqueues all tracks whose cue_in_ms is NULL for (re)analysis.
func TriggerCueInReanalyze(cw CueInWorker, ts CueInTrackStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, err := ts.ListNullCueIn(r.Context(), 50_000)
		if err != nil {
			slog.Error("TriggerCueInReanalyze: list error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not list pending tracks")
			return
		}
		for _, id := range ids {
			cw.Enqueue(id)
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"enqueued": len(ids),
		})
	}
}

// CancelCueInReanalyze handles DELETE /v1/tracks/reanalyze-cuepoints.
// Drains the in-memory queue, discarding tracks not yet dispatched to ffmpeg.
func CancelCueInReanalyze(cw CueInWorker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cw.DrainQueue()
		writeJSON(w, http.StatusOK, map[string]any{"cancelled": true})
	}
}
