package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// ClockStore is the store subset required by the clock handlers.
type ClockStore interface {
	List(ctx context.Context) ([]store.Clock, error)
	Create(ctx context.Context, name string) (store.Clock, error)
	Get(ctx context.Context, id string) (store.Clock, error)
	Update(ctx context.Context, id, name string) error
	Delete(ctx context.Context, id string) error
	AddSlot(ctx context.Context, clockID string, slot store.ClockSlot) (store.ClockSlot, error)
	UpdateSlot(ctx context.Context, slotID string, slot store.ClockSlot) error
	DeleteSlot(ctx context.Context, slotID string) error
	ReorderSlots(ctx context.Context, clockID string, orderedSlotIDs []string) error
	GetGrid(ctx context.Context) ([]store.ScheduleCell, error)
	SetGridCells(ctx context.Context, cells []store.ScheduleCell) error
}

// ── JSON shapes ───────────────────────────────────────────────────────────────

type clockSummaryJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SlotCount int       `json:"slot_count"`
	CreatedAt time.Time `json:"created_at"`
}

type clockDetailJSON struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Slots     []clockSlotJSON `json:"slots"`
	CreatedAt time.Time     `json:"created_at"`
}

type clockSlotJSON struct {
	ID              string `json:"id"`
	ClockID         string `json:"clock_id"`
	Position        int    `json:"position"`
	SlotType        string `json:"slot_type"`
	CategoryID      string `json:"category_id,omitempty"`
	CategoryName    string `json:"category_name,omitempty"`
	FixedTrackID    string `json:"fixed_track_id,omitempty"`
	DurationHintMS  int64  `json:"duration_hint_ms"`
}

type scheduleCellJSON struct {
	Weekday   int    `json:"weekday"`
	Hour      int    `json:"hour"`
	ClockID   string `json:"clock_id"`
	ClockName string `json:"clock_name,omitempty"`
}

func toClockSummary(c store.Clock) clockSummaryJSON {
	slotCount := c.SlotCount
	if len(c.Slots) > slotCount {
		slotCount = len(c.Slots)
	}
	return clockSummaryJSON{
		ID: c.ID, Name: c.Name, SlotCount: slotCount, CreatedAt: c.CreatedAt,
	}
}

func toClockDetail(c store.Clock) clockDetailJSON {
	slots := make([]clockSlotJSON, len(c.Slots))
	for i, s := range c.Slots {
		slots[i] = toSlotJSON(s)
	}
	return clockDetailJSON{ID: c.ID, Name: c.Name, Slots: slots, CreatedAt: c.CreatedAt}
}

func toSlotJSON(s store.ClockSlot) clockSlotJSON {
	return clockSlotJSON{
		ID: s.ID, ClockID: s.ClockID, Position: s.Position, SlotType: s.SlotType,
		CategoryID: s.CategoryID, CategoryName: s.CategoryName,
		FixedTrackID: s.FixedTrackID, DurationHintMS: s.DurationHintMS,
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListClocks handles GET /v1/clocks.
func ListClocks(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clocks, err := cs.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		out := make([]clockSummaryJSON, len(clocks))
		for i, c := range clocks {
			out[i] = toClockSummary(c)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": out})
	}
}

// CreateClock handles POST /v1/clocks.
func CreateClock(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "missing_name", "name is required")
			return
		}
		clk, err := cs.Create(r.Context(), req.Name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": toClockDetail(clk)})
	}
}

// GetClock handles GET /v1/clocks/{id}.
func GetClock(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clk, err := cs.Get(r.Context(), r.PathValue("id"))
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "clock not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": toClockDetail(clk)})
	}
}

// UpdateClock handles PUT /v1/clocks/{id}.
func UpdateClock(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		id := r.PathValue("id")
		if err := cs.Update(r.Context(), id, req.Name); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "clock not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		clk, _ := cs.Get(r.Context(), id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": toClockDetail(clk)})
	}
}

// DeleteClock handles DELETE /v1/clocks/{id}.
func DeleteClock(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := cs.Delete(r.Context(), r.PathValue("id")); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "clock not found")
				return
			}
			writeError(w, http.StatusConflict, "conflict", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// AddClockSlot handles POST /v1/clocks/{id}/slots.
func AddClockSlot(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SlotType       string `json:"slot_type"`
			CategoryID     string `json:"category_id"`
			FixedTrackID   string `json:"fixed_track_id"`
			DurationHintMS int64  `json:"duration_hint_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		validTypes := map[string]bool{
			"CATEGORY": true, "JINGLE": true, "SPOT": true,
			"VINHETA": true, "HORA_CERTA": true, "FIXED": true,
		}
		if !validTypes[req.SlotType] {
			writeError(w, http.StatusBadRequest, "invalid_slot_type",
				"slot_type must be one of: CATEGORY, JINGLE, SPOT, VINHETA, HORA_CERTA, FIXED")
			return
		}
		slot, err := cs.AddSlot(r.Context(), r.PathValue("id"), store.ClockSlot{
			SlotType:       req.SlotType,
			CategoryID:     req.CategoryID,
			FixedTrackID:   req.FixedTrackID,
			DurationHintMS: req.DurationHintMS,
		})
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "clock not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": toSlotJSON(slot)})
	}
}

// UpdateClockSlot handles PUT /v1/clocks/{id}/slots/{slot_id}.
func UpdateClockSlot(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SlotType       string `json:"slot_type"`
			CategoryID     string `json:"category_id"`
			FixedTrackID   string `json:"fixed_track_id"`
			DurationHintMS int64  `json:"duration_hint_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if err := cs.UpdateSlot(r.Context(), r.PathValue("slot_id"), store.ClockSlot{
			SlotType:       req.SlotType,
			CategoryID:     req.CategoryID,
			FixedTrackID:   req.FixedTrackID,
			DurationHintMS: req.DurationHintMS,
		}); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "slot not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// DeleteClockSlot handles DELETE /v1/clocks/{id}/slots/{slot_id}.
func DeleteClockSlot(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := cs.DeleteSlot(r.Context(), r.PathValue("slot_id")); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "slot not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// ReorderClockSlots handles PUT /v1/clocks/{id}/slots/reorder.
func ReorderClockSlots(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SlotIDs []string `json:"slot_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if len(req.SlotIDs) == 0 {
			writeError(w, http.StatusBadRequest, "missing_slot_ids", "slot_ids must not be empty")
			return
		}
		if err := cs.ReorderSlots(r.Context(), r.PathValue("id"), req.SlotIDs); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		clk, _ := cs.Get(r.Context(), r.PathValue("id"))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": toClockDetail(clk)})
	}
}

// GetClockGrid handles GET /v1/schedule/clock-grid.
func GetClockGrid(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cells, err := cs.GetGrid(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		out := make([]scheduleCellJSON, len(cells))
		for i, c := range cells {
			out[i] = scheduleCellJSON{
				Weekday:   c.Weekday,
				Hour:      c.Hour,
				ClockID:   c.ClockID,
				ClockName: c.ClockName,
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": map[string]any{"grid": out}})
	}
}

// SetClockGrid handles PUT /v1/schedule/clock-grid.
func SetClockGrid(cs ClockStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req []scheduleCellJSON
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		cells := make([]store.ScheduleCell, len(req))
		for i, c := range req {
			cells[i] = store.ScheduleCell{
				Weekday: c.Weekday, Hour: c.Hour, ClockID: c.ClockID,
			}
		}
		if err := cs.SetGridCells(r.Context(), cells); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
