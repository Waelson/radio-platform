package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// CategoryStore is the store subset required by the category handlers.
type CategoryStore interface {
	List(ctx context.Context) ([]store.Category, error)
	Create(ctx context.Context, name, description, color string) (store.Category, error)
	Get(ctx context.Context, id string) (store.Category, error)
	Update(ctx context.Context, id, name, description, color string) error
	Delete(ctx context.Context, id string) error
	ListTracks(ctx context.Context, categoryID string, limit, offset int) ([]store.Track, error)
	AddTracks(ctx context.Context, categoryID string, trackIDs []string) error
	RemoveTrack(ctx context.Context, categoryID, trackID string) error
	SetTrackCategories(ctx context.Context, trackID string, categoryIDs []string) error
	ListByTrack(ctx context.Context, trackID string) ([]store.Category, error)
}

// ── JSON shapes ───────────────────────────────────────────────────────────────

type categoryJSON struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	TrackCount  int       `json:"track_count"`
	CreatedAt   time.Time `json:"created_at"`
}

func toCategoryJSON(c store.Category) categoryJSON {
	return categoryJSON{
		ID: c.ID, Name: c.Name, Description: c.Description,
		Color: c.Color, TrackCount: c.TrackCount, CreatedAt: c.CreatedAt,
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListCategories handles GET /v1/categories.
func ListCategories(cs CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cats, err := cs.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		out := make([]categoryJSON, len(cats))
		for i, c := range cats {
			out[i] = toCategoryJSON(c)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": out})
	}
}

// CreateCategory handles POST /v1/categories.
func CreateCategory(cs CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Color       string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "missing_name", "name is required")
			return
		}
		if req.Color == "" {
			req.Color = "#888888"
		}
		cat, err := cs.Create(r.Context(), req.Name, req.Description, req.Color)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": toCategoryJSON(cat)})
	}
}

// GetCategory handles GET /v1/categories/{id}.
func GetCategory(cs CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cat, err := cs.Get(r.Context(), r.PathValue("id"))
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "category not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": toCategoryJSON(cat)})
	}
}

// UpdateCategory handles PUT /v1/categories/{id}.
func UpdateCategory(cs CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Color       string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "missing_name", "name is required")
			return
		}
		id := r.PathValue("id")
		if err := cs.Update(r.Context(), id, req.Name, req.Description, req.Color); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "category not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		cat, _ := cs.Get(r.Context(), id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": toCategoryJSON(cat)})
	}
}

// DeleteCategory handles DELETE /v1/categories/{id}.
func DeleteCategory(cs CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := cs.Delete(r.Context(), r.PathValue("id")); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "category not found")
				return
			}
			writeError(w, http.StatusConflict, "conflict", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// ListCategoryTracks handles GET /v1/categories/{id}/tracks.
func ListCategoryTracks(cs CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 {
			limit = 50
		}

		tracks, err := cs.ListTracks(r.Context(), id, limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		out := make([]trackJSON, len(tracks))
		for i, t := range tracks {
			out[i] = toTrackJSON(t)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": out, "count": len(out)})
	}
}

// AddCategoryTracks handles POST /v1/categories/{id}/tracks.
func AddCategoryTracks(cs CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TrackIDs []string `json:"track_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if len(req.TrackIDs) == 0 {
			writeError(w, http.StatusBadRequest, "missing_track_ids", "track_ids must not be empty")
			return
		}
		if err := cs.AddTracks(r.Context(), r.PathValue("id"), req.TrackIDs); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// RemoveCategoryTrack handles DELETE /v1/categories/{id}/tracks/{track_id}.
func RemoveCategoryTrack(cs CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := cs.RemoveTrack(r.Context(), r.PathValue("id"), r.PathValue("track_id")); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// SetTrackCategories handles PUT /v1/tracks/{id}/categories.
func SetTrackCategories(cs CategoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CategoryIDs []string `json:"category_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if err := cs.SetTrackCategories(r.Context(), r.PathValue("id"), req.CategoryIDs); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		cats, err := cs.ListByTrack(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		out := make([]categoryJSON, len(cats))
		for i, c := range cats {
			out[i] = toCategoryJSON(c)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": out})
	}
}
