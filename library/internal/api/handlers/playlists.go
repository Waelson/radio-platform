package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// PlaylistStore is the store subset required by the playlist handlers.
type PlaylistStore interface {
	Create(ctx context.Context, name, category string) (store.Playlist, error)
	FindByID(ctx context.Context, id string) (store.Playlist, error)
	List(ctx context.Context) ([]store.Playlist, error)
	Update(ctx context.Context, id string, patch store.PlaylistPatch) error
	Delete(ctx context.Context, id string) error
	AddItem(ctx context.Context, playlistID, trackID string) (store.PlaylistItem, error)
	RemoveItem(ctx context.Context, itemID string) error
	ReorderItems(ctx context.Context, playlistID string, itemIDs []string) error
}

// ─── JSON shapes ─────────────────────────────────────────────────────────────

type playlistSummaryJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	ItemCount int       `json:"item_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type playlistDetailJSON struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Category  string              `json:"category"`
	Items     []playlistItemJSON  `json:"items"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type playlistItemJSON struct {
	ID       string    `json:"id"`
	Position int       `json:"position"`
	Track    trackJSON `json:"track"`
}

func toPlaylistSummary(p store.Playlist) playlistSummaryJSON {
	return playlistSummaryJSON{
		ID:        p.ID,
		Name:      p.Name,
		Category:  p.Category,
		ItemCount: p.ItemCount,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func toPlaylistDetail(p store.Playlist) playlistDetailJSON {
	items := make([]playlistItemJSON, len(p.Items))
	for i, it := range p.Items {
		items[i] = playlistItemJSON{
			ID:       it.ID,
			Position: it.Position,
			Track:    toTrackJSON(it.Track),
		}
	}
	return playlistDetailJSON{
		ID:        p.ID,
		Name:      p.Name,
		Category:  p.Category,
		Items:     items,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// ListPlaylists handles GET /v1/playlists.
func ListPlaylists(ps PlaylistStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playlists, err := ps.List(r.Context())
		if err != nil {
			slog.Error("ListPlaylists: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "list playlists failed")
			return
		}

		out := make([]playlistSummaryJSON, len(playlists))
		for i, p := range playlists {
			out[i] = toPlaylistSummary(p)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"playlists": out,
			"count":     len(out),
		})
	}
}

// CreatePlaylist handles POST /v1/playlists.
func CreatePlaylist(ps PlaylistStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string `json:"name"`
			Category string `json:"category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}

		pl, err := ps.Create(r.Context(), body.Name, body.Category)
		if err != nil {
			slog.Error("CreatePlaylist: store error", "error", err)
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, toPlaylistDetail(pl))
	}
}

// GetPlaylist handles GET /v1/playlists/{id}.
func GetPlaylist(ps PlaylistStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		pl, err := ps.FindByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "playlist not found")
			return
		}
		if err != nil {
			slog.Error("GetPlaylist: store error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "lookup failed")
			return
		}
		writeJSON(w, http.StatusOK, toPlaylistDetail(pl))
	}
}

// UpdatePlaylist handles PUT /v1/playlists/{id}.
func UpdatePlaylist(ps PlaylistStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var body struct {
			Name     *string `json:"name"`
			Category *string `json:"category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}

		patch := store.PlaylistPatch{Name: body.Name, Category: body.Category}
		if err := ps.Update(r.Context(), id, patch); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "playlist not found")
			return
		} else if err != nil {
			slog.Error("UpdatePlaylist: store error", "id", id, "error", err)
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		pl, err := ps.FindByID(r.Context(), id)
		if err != nil {
			slog.Error("UpdatePlaylist: FindByID after update", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "fetch after update failed")
			return
		}
		writeJSON(w, http.StatusOK, toPlaylistDetail(pl))
	}
}

// DeletePlaylist handles DELETE /v1/playlists/{id}.
func DeletePlaylist(ps PlaylistStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := ps.Delete(r.Context(), id); err != nil {
			slog.Error("DeletePlaylist: store error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "delete failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// AddPlaylistItem handles POST /v1/playlists/{id}/items.
func AddPlaylistItem(ps PlaylistStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playlistID := r.PathValue("id")

		var body struct {
			TrackID string `json:"track_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}
		if body.TrackID == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "track_id is required")
			return
		}

		item, err := ps.AddItem(r.Context(), playlistID, body.TrackID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "playlist or track not found")
			return
		}
		if err != nil {
			slog.Error("AddPlaylistItem: store error", "playlist", playlistID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "add item failed")
			return
		}

		writeJSON(w, http.StatusCreated, playlistItemJSON{
			ID:       item.ID,
			Position: item.Position,
			Track:    toTrackJSON(item.Track),
		})
	}
}

// RemovePlaylistItem handles DELETE /v1/playlists/{id}/items/{item_id}.
func RemovePlaylistItem(ps PlaylistStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemID := r.PathValue("item_id")
		if err := ps.RemoveItem(r.Context(), itemID); err != nil {
			slog.Error("RemovePlaylistItem: store error", "item_id", itemID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "remove item failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ReorderPlaylistItems handles PUT /v1/playlists/{id}/items/reorder.
func ReorderPlaylistItems(ps PlaylistStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playlistID := r.PathValue("id")

		var body struct {
			ItemIDs []string `json:"item_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}
		if len(body.ItemIDs) == 0 {
			writeError(w, http.StatusBadRequest, "bad_request", "item_ids must not be empty")
			return
		}

		if err := ps.ReorderItems(r.Context(), playlistID, body.ItemIDs); err != nil {
			slog.Error("ReorderPlaylistItems: store error", "playlist", playlistID, "error", err)
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		pl, err := ps.FindByID(r.Context(), playlistID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "playlist not found")
			return
		}
		if err != nil {
			slog.Error("ReorderPlaylistItems: FindByID", "id", playlistID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "fetch after reorder failed")
			return
		}
		writeJSON(w, http.StatusOK, toPlaylistDetail(pl))
	}
}
