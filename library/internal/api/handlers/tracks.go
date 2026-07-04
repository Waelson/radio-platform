// Package handlers contains HTTP handler functions for the Library Service API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// TrackStore is the store subset required by the track handlers.
type TrackStore interface {
	FindByID(ctx context.Context, id string) (store.Track, error)
	Search(ctx context.Context, q store.SearchQuery) ([]store.Track, error)
	ListArtists(ctx context.Context, trackType string) ([]string, error)
	UpdateMeta(ctx context.Context, id string, patch store.TrackPatch) error
}

// trackJSON is the JSON representation of a track.
type trackJSON struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Title      string    `json:"title"`
	Artist     string    `json:"artist"`
	Album      string    `json:"album"`
	Type       string    `json:"type"`
	DurationMS int64     `json:"duration_ms"`
	Category   string    `json:"category"`
	IndexedAt  time.Time `json:"indexed_at"`
}

func toTrackJSON(t store.Track) trackJSON {
	return trackJSON{
		ID:         t.ID,
		Path:       t.Path,
		Title:      t.Title,
		Artist:     t.Artist,
		Album:      t.Album,
		Type:       t.Type,
		DurationMS: t.DurationMS,
		Category:   t.Category,
		IndexedAt:  t.IndexedAt,
	}
}

// SearchTracks handles GET /v1/tracks
// Query params: q, type, artist, category, limit (default 50, max 200), offset.
func SearchTracks(ts TrackStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))

		sq := store.SearchQuery{
			Q:        q.Get("q"),
			Type:     q.Get("type"),
			Artist:   q.Get("artist"),
			Category: q.Get("category"),
			Limit:    limit,
			Offset:   offset,
		}

		tracks, err := ts.Search(r.Context(), sq)
		if err != nil {
			slog.Error("SearchTracks: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "search failed")
			return
		}

		out := make([]trackJSON, len(tracks))
		for i, t := range tracks {
			out[i] = toTrackJSON(t)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"tracks": out,
			"count":  len(out),
			"limit":  limit,
			"offset": offset,
		})
	}
}

// GetTrack handles GET /v1/tracks/{id}.
func GetTrack(ts TrackStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "id is required")
			return
		}

		t, err := ts.FindByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "track not found")
			return
		}
		if err != nil {
			slog.Error("GetTrack: store error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "lookup failed")
			return
		}

		writeJSON(w, http.StatusOK, toTrackJSON(t))
	}
}

// PatchTrack handles PATCH /v1/tracks/{id}.
// Accepts a JSON body with any subset of: title, artist, category, type.
func PatchTrack(ts TrackStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "id is required")
			return
		}

		var body struct {
			Title    *string `json:"title"`
			Artist   *string `json:"artist"`
			Category *string `json:"category"`
			Type     *string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}

		if body.Type != nil {
			switch *body.Type {
			case "MUSIC", "VINHETA", "JINGLE", "SPOT":
				// valid
			default:
				writeError(w, http.StatusBadRequest, "bad_request",
					"type must be one of MUSIC, VINHETA, JINGLE, SPOT")
				return
			}
		}

		patch := store.TrackPatch{
			Title:    body.Title,
			Artist:   body.Artist,
			Category: body.Category,
			Type:     body.Type,
		}

		if err := ts.UpdateMeta(r.Context(), id, patch); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "track not found")
			return
		} else if err != nil {
			slog.Error("PatchTrack: store error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "update failed")
			return
		}

		t, err := ts.FindByID(r.Context(), id)
		if err != nil {
			slog.Error("PatchTrack: FindByID after update", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "fetch after update failed")
			return
		}

		writeJSON(w, http.StatusOK, toTrackJSON(t))
	}
}

// ListArtists handles GET /v1/tracks/artists.
// Optional query param: type (MUSIC | VINHETA | JINGLE | SPOT).
func ListArtists(ts TrackStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trackType := r.URL.Query().Get("type")

		artists, err := ts.ListArtists(r.Context(), trackType)
		if err != nil {
			slog.Error("ListArtists: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "list artists failed")
			return
		}

		if artists == nil {
			artists = []string{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"artists": artists,
			"count":   len(artists),
		})
	}
}

// writeJSON and writeError live in the parent api package; re-export here via
// package-level vars so handlers stay in their own package without an import cycle.
var writeJSON = func(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON: encode failed", "error", err)
	}
}

var writeError = func(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}
