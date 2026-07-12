package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// RotationLogStore is the store subset required by the rotation log handlers.
type RotationLogStore interface {
	Append(ctx context.Context, e store.RotationLogEntry) error
	ListByDate(ctx context.Context, date time.Time) ([]store.RotationLogEntry, error)
}

// ── JSON shapes ───────────────────────────────────────────────────────────────

type rotationLogEntryJSON struct {
	ID         string    `json:"id,omitempty"`
	TrackID    string    `json:"track_id"`
	PlayedAt   time.Time `json:"played_at"`
	ClockID    string    `json:"clock_id,omitempty"`
	SlotType   string    `json:"slot_type,omitempty"`
	CategoryID string    `json:"category_id,omitempty"`
	Artist     string    `json:"artist,omitempty"`
	Title      string    `json:"title,omitempty"`
	Album      string    `json:"album,omitempty"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// AppendRotationLog handles POST /v1/rotation-log.
// Accepts a JSON array of entries. Called by the Player UI after enqueueing
// generated items, to record what was programmed.
func AppendRotationLog(rls RotationLogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req []rotationLogEntryJSON
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		for _, e := range req {
			if e.TrackID == "" {
				writeError(w, http.StatusBadRequest, "missing_track_id", "track_id is required for all entries")
				return
			}
			played := e.PlayedAt
			if played.IsZero() {
				played = time.Now().UTC()
			}
			if err := rls.Append(r.Context(), store.RotationLogEntry{
				TrackID:    e.TrackID,
				PlayedAt:   played,
				ClockID:    e.ClockID,
				SlotType:   e.SlotType,
				CategoryID: e.CategoryID,
				Artist:     e.Artist,
				Title:      e.Title,
				Album:      e.Album,
			}); err != nil {
				writeError(w, http.StatusInternalServerError, "store_error", err.Error())
				return
			}
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "count": len(req)})
	}
}

// GetRotationLog handles GET /v1/rotation-log?date=YYYY-MM-DD.
func GetRotationLog(rls RotationLogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dateStr := r.URL.Query().Get("date")
		var date time.Time
		if dateStr == "" {
			date = time.Now().UTC()
		} else {
			var err error
			date, err = time.Parse("2006-01-02", dateStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_date", "date must be YYYY-MM-DD")
				return
			}
		}

		entries, err := rls.ListByDate(r.Context(), date)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}

		out := make([]rotationLogEntryJSON, len(entries))
		for i, e := range entries {
			out[i] = rotationLogEntryJSON{
				ID:         e.ID,
				TrackID:    e.TrackID,
				PlayedAt:   e.PlayedAt,
				ClockID:    e.ClockID,
				SlotType:   e.SlotType,
				CategoryID: e.CategoryID,
				Artist:     e.Artist,
				Title:      e.Title,
				Album:      e.Album,
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"data": out,
			"date": date.Format("2006-01-02"),
		})
	}
}
