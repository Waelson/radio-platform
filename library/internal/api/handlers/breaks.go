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

// BreakStore is the store subset required by the break handlers.
type BreakStore interface {
	Create(ctx context.Context, name, openTrackID, closeTrackID string) (store.Break, error)
	FindByID(ctx context.Context, id string) (store.Break, error)
	List(ctx context.Context) ([]store.Break, error)
	Update(ctx context.Context, id string, patch store.BreakPatch) error
	Delete(ctx context.Context, id string) error
	AddItem(ctx context.Context, breakID, trackID string) (store.BreakItem, error)
	RemoveItem(ctx context.Context, itemID string) error
	ReorderItems(ctx context.Context, breakID string, itemIDs []string) error
}

// ─── JSON shapes ─────────────────────────────────────────────────────────────

type breakSummaryJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ItemCount int       `json:"item_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type breakDetailJSON struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	OpenTrack  *trackJSON     `json:"open_track"`
	CloseTrack *trackJSON     `json:"close_track"`
	Items      []breakItemJSON `json:"items"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type breakItemJSON struct {
	ID       string    `json:"id"`
	Position int       `json:"position"`
	Track    trackJSON `json:"track"`
}

// enginePayloadTrack is the track shape expected by the playout engine's
// POST /v1/queue/enqueue-break endpoint.
type enginePayloadTrack struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Title      string `json:"title"`
	Artist     string `json:"artist,omitempty"`
	Type       string `json:"type"`
	DurationMS int64  `json:"duration_ms"`
}

// engineBreakPayload is the full payload for POST /v1/queue/enqueue-break.
type engineBreakPayload struct {
	Name   string               `json:"name"`
	Open   *enginePayloadTrack  `json:"open"`
	Spots  []enginePayloadTrack `json:"spots"`
	Close  *enginePayloadTrack  `json:"close"`
}

func toBreakSummary(b store.Break) breakSummaryJSON {
	return breakSummaryJSON{
		ID: b.ID, Name: b.Name, ItemCount: b.ItemCount,
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}

func toBreakDetail(b store.Break) breakDetailJSON {
	d := breakDetailJSON{
		ID: b.ID, Name: b.Name,
		Items:     make([]breakItemJSON, len(b.Items)),
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
	if b.OpenTrack != nil {
		tj := toTrackJSON(*b.OpenTrack)
		d.OpenTrack = &tj
	}
	if b.CloseTrack != nil {
		tj := toTrackJSON(*b.CloseTrack)
		d.CloseTrack = &tj
	}
	for i, it := range b.Items {
		d.Items[i] = breakItemJSON{
			ID: it.ID, Position: it.Position, Track: toTrackJSON(it.Track),
		}
	}
	return d
}

func toEngineTrack(t store.Track) enginePayloadTrack {
	return enginePayloadTrack{
		ID: t.ID, Path: t.Path, Title: t.Title, Artist: t.Artist,
		Type: t.Type, DurationMS: t.DurationMS,
	}
}

func toEnginePayload(b store.Break) engineBreakPayload {
	p := engineBreakPayload{
		Name:  b.Name,
		Spots: make([]enginePayloadTrack, len(b.Items)),
	}
	if b.OpenTrack != nil {
		et := toEngineTrack(*b.OpenTrack)
		p.Open = &et
	}
	if b.CloseTrack != nil {
		et := toEngineTrack(*b.CloseTrack)
		p.Close = &et
	}
	for i, it := range b.Items {
		p.Spots[i] = toEngineTrack(it.Track)
	}
	return p
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// ListBreaks handles GET /v1/breaks.
func ListBreaks(bs BreakStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		breaks, err := bs.List(r.Context())
		if err != nil {
			slog.Error("ListBreaks: store error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "list breaks failed")
			return
		}
		out := make([]breakSummaryJSON, len(breaks))
		for i, b := range breaks {
			out[i] = toBreakSummary(b)
		}
		writeJSON(w, http.StatusOK, map[string]any{"breaks": out, "count": len(out)})
	}
}

// CreateBreak handles POST /v1/breaks.
func CreateBreak(bs BreakStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name         string `json:"name"`
			OpenTrackID  string `json:"open_track_id"`
			CloseTrackID string `json:"close_track_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}
		brk, err := bs.Create(r.Context(), body.Name, body.OpenTrackID, body.CloseTrackID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, toBreakDetail(brk))
	}
}

// GetBreak handles GET /v1/breaks/{id}.
// With ?format=engine-payload it returns the payload ready for the playout engine.
func GetBreak(bs BreakStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		brk, err := bs.FindByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "break not found")
			return
		}
		if err != nil {
			slog.Error("GetBreak: store error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "lookup failed")
			return
		}

		if r.URL.Query().Get("format") == "engine-payload" {
			writeJSON(w, http.StatusOK, toEnginePayload(brk))
			return
		}
		writeJSON(w, http.StatusOK, toBreakDetail(brk))
	}
}

// UpdateBreak handles PUT /v1/breaks/{id}.
func UpdateBreak(bs BreakStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Name         *string `json:"name"`
			OpenTrackID  *string `json:"open_track_id"`
			CloseTrackID *string `json:"close_track_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}
		patch := store.BreakPatch{
			Name:         body.Name,
			OpenTrackID:  body.OpenTrackID,
			CloseTrackID: body.CloseTrackID,
		}
		if err := bs.Update(r.Context(), id, patch); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "break not found")
			return
		} else if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		brk, err := bs.FindByID(r.Context(), id)
		if err != nil {
			slog.Error("UpdateBreak: FindByID after update", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "fetch after update failed")
			return
		}
		writeJSON(w, http.StatusOK, toBreakDetail(brk))
	}
}

// DeleteBreak handles DELETE /v1/breaks/{id}.
func DeleteBreak(bs BreakStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := bs.Delete(r.Context(), id); err != nil {
			slog.Error("DeleteBreak: store error", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "delete failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// AddBreakItem handles POST /v1/breaks/{id}/items.
func AddBreakItem(bs BreakStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		breakID := r.PathValue("id")
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
		item, err := bs.AddItem(r.Context(), breakID, body.TrackID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "break or track not found")
			return
		}
		if err != nil {
			slog.Error("AddBreakItem: store error", "break", breakID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "add item failed")
			return
		}
		writeJSON(w, http.StatusCreated, breakItemJSON{
			ID: item.ID, Position: item.Position, Track: toTrackJSON(item.Track),
		})
	}
}

// RemoveBreakItem handles DELETE /v1/breaks/{id}/items/{item_id}.
func RemoveBreakItem(bs BreakStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemID := r.PathValue("item_id")
		if err := bs.RemoveItem(r.Context(), itemID); err != nil {
			slog.Error("RemoveBreakItem: store error", "item_id", itemID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "remove item failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ReorderBreakItems handles PUT /v1/breaks/{id}/items/reorder.
func ReorderBreakItems(bs BreakStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		breakID := r.PathValue("id")
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
		if err := bs.ReorderItems(r.Context(), breakID, body.ItemIDs); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		brk, err := bs.FindByID(r.Context(), breakID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "break not found")
			return
		}
		if err != nil {
			slog.Error("ReorderBreakItems: FindByID", "id", breakID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "fetch after reorder failed")
			return
		}
		writeJSON(w, http.StatusOK, toBreakDetail(brk))
	}
}
