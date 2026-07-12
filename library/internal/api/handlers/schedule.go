package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Waelson/radio-library-service/internal/scheduler"
	"github.com/Waelson/radio-library-service/internal/store"
)

// SeparationRuleStore is the store subset required by the separation-rule handlers.
type SeparationRuleStore interface {
	List(ctx context.Context) ([]store.SeparationRule, error)
	Create(ctx context.Context, field string, minSepMinutes int) (store.SeparationRule, error)
	Update(ctx context.Context, id, field string, minSepMinutes int) error
	Delete(ctx context.Context, id string) error
}

// SchedulerService is the interface for the playlist generator.
type SchedulerService interface {
	Generate(ctx context.Context, from time.Time, hours int) ([]scheduler.GeneratedItem, []string, error)
}

// ── JSON shapes ───────────────────────────────────────────────────────────────

type separationRuleJSON struct {
	ID            string `json:"id"`
	Field         string `json:"field"`
	MinSepMinutes int    `json:"min_sep_minutes"`
}

type generatedItemJSON struct {
	Hour         int            `json:"hour"`
	Position     int            `json:"position"`
	SlotID       string         `json:"slot_id"`
	SlotType     string         `json:"slot_type"`
	ClockID      string         `json:"clock_id"`
	ClockName    string         `json:"clock_name"`
	CategoryID   string         `json:"category_id,omitempty"`
	CategoryName string         `json:"category_name,omitempty"`
	Track        generatedTrackJSON `json:"track"`
}

type generatedTrackJSON struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	DurationMS int64  `json:"duration_ms"`
}

// ── Separation rule handlers ──────────────────────────────────────────────────

// ListSeparationRules handles GET /v1/schedule/separation-rules.
func ListSeparationRules(ss SeparationRuleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rules, err := ss.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		out := make([]separationRuleJSON, len(rules))
		for i, rule := range rules {
			out[i] = separationRuleJSON{ID: rule.ID, Field: rule.Field, MinSepMinutes: rule.MinSepMinutes}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": out})
	}
}

// CreateSeparationRule handles POST /v1/schedule/separation-rules.
func CreateSeparationRule(ss SeparationRuleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Field         string `json:"field"`
			MinSepMinutes int    `json:"min_sep_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		rule, err := ss.Create(r.Context(), req.Field, req.MinSepMinutes)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_rule", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"ok":   true,
			"data": separationRuleJSON{ID: rule.ID, Field: rule.Field, MinSepMinutes: rule.MinSepMinutes},
		})
	}
}

// UpdateSeparationRule handles PUT /v1/schedule/separation-rules/{id}.
func UpdateSeparationRule(ss SeparationRuleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Field         string `json:"field"`
			MinSepMinutes int    `json:"min_sep_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if err := ss.Update(r.Context(), r.PathValue("id"), req.Field, req.MinSepMinutes); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "rule not found")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid_rule", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// DeleteSeparationRule handles DELETE /v1/schedule/separation-rules/{id}.
func DeleteSeparationRule(ss SeparationRuleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := ss.Delete(r.Context(), r.PathValue("id")); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "rule not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// ── Generator handler ─────────────────────────────────────────────────────────

// GenerateSchedule handles POST /v1/schedule/generate.
func GenerateSchedule(svc SchedulerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			From  string `json:"from"`  // ISO 8601, e.g. "2026-07-19T08:00:00"
			Hours int    `json:"hours"` // 1–24, default 1
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		from := time.Now().UTC().Truncate(time.Hour)
		if req.From != "" {
			var err error
			from, err = time.Parse("2006-01-02T15:04:05", req.From)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_from",
					"from must be ISO 8601 without timezone, e.g. 2026-07-19T08:00:00")
				return
			}
		}
		if req.Hours <= 0 {
			req.Hours = 1
		}

		items, warnings, err := svc.Generate(r.Context(), from, req.Hours)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "generator_error", err.Error())
			return
		}

		out := make([]generatedItemJSON, len(items))
		for i, item := range items {
			out[i] = generatedItemJSON{
				Hour: item.Hour, Position: item.Position,
				SlotID: item.SlotID, SlotType: item.SlotType,
				ClockID: item.ClockID, ClockName: item.ClockName,
				CategoryID: item.CategoryID, CategoryName: item.CategoryName,
				Track: generatedTrackJSON{
					ID:         item.Track.ID,
					Path:       item.Track.Path,
					Title:      item.Track.Title,
					Artist:     item.Track.Artist,
					Album:      item.Track.Album,
					DurationMS: item.Track.DurationMS,
				},
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": map[string]any{
				"from":     from.Format("2006-01-02T15:04:05"),
				"to":       from.Add(time.Duration(req.Hours) * time.Hour).Format("2006-01-02T15:04:05"),
				"hours":    req.Hours,
				"items":    out,
				"warnings": warnings,
			},
		})
	}
}
