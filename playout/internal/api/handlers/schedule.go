package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/scheduler"
)

// ScheduleManager is the interface that schedule handlers require from the
// scheduler.Manager. Using an interface keeps handlers independently testable.
type ScheduleManager interface {
	Add(e scheduler.Entry) (string, error)
	Update(id string, e scheduler.Entry) error
	Remove(id string)
	Enable(id string) bool
	Disable(id string) bool
	Get(id string) (scheduler.Entry, bool)
	List() []scheduler.Entry
	NextFireAt(id string) *time.Time
}

// --- request / response DTOs -------------------------------------------------

type scheduleAddRequest struct {
	Name        string         `json:"name"`
	Enabled     bool           `json:"enabled"`
	CronExpr    string         `json:"cron_expr"`
	FireAt      *time.Time     `json:"fire_at"`
	TriggerMode string         `json:"trigger_mode"`
	Item        queueItemInput `json:"item"`
}

type scheduleEntryView struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Enabled     bool           `json:"enabled"`
	CronExpr    string         `json:"cron_expr,omitempty"`
	FireAt      *time.Time     `json:"fire_at,omitempty"`
	TriggerMode string         `json:"trigger_mode"`
	Item        queueItemInput `json:"item"`
	CreatedAt   time.Time      `json:"created_at"`
	LastFiredAt *time.Time     `json:"last_fired_at,omitempty"`
	NextFireAt  *time.Time     `json:"next_fire_at,omitempty"`
}

// --- helpers -----------------------------------------------------------------

var validTriggerModes = map[string]bool{
	string(scheduler.TriggerInterrupt):    true,
	string(scheduler.TriggerAfterCurrent): true,
	string(scheduler.TriggerCrossfade):    true,
	string(scheduler.TriggerSkipIfBusy):   true,
}

func validateScheduleRequest(req scheduleAddRequest) string {
	if req.CronExpr == "" && req.FireAt == nil {
		return "exactly one of cron_expr or fire_at must be set"
	}
	if req.CronExpr != "" && req.FireAt != nil {
		return "cron_expr and fire_at are mutually exclusive"
	}
	if req.Item.Path == "" && req.Item.Type != "HORA_CERTA" {
		return "field item.path is required"
	}
	if req.TriggerMode != "" && !validTriggerModes[req.TriggerMode] {
		return "trigger_mode must be one of: INTERRUPT, AFTER_CURRENT, CROSSFADE, SKIP_IF_BUSY"
	}
	return ""
}

func toScheduleEntry(req scheduleAddRequest) scheduler.Entry {
	mode := scheduler.TriggerMode(req.TriggerMode)
	if mode == "" {
		mode = scheduler.TriggerAfterCurrent
	}
	return scheduler.Entry{
		Name:        req.Name,
		Enabled:     req.Enabled,
		CronExpr:    req.CronExpr,
		FireAt:      req.FireAt,
		TriggerMode: mode,
		Item:        toCommandItem(req.Item),
	}
}

func toScheduleView(e scheduler.Entry, nextFireAt *time.Time) scheduleEntryView {
	v := scheduleEntryView{
		ID:          e.ID,
		Name:        e.Name,
		Enabled:     e.Enabled,
		CronExpr:    e.CronExpr,
		FireAt:      e.FireAt,
		TriggerMode: string(e.TriggerMode),
		CreatedAt:   e.CreatedAt,
		NextFireAt:  nextFireAt,
		Item: queueItemInput{
			AssetID:    e.Item.AssetID,
			Path:       e.Item.Path,
			Type:       e.Item.Type,
			Title:      e.Item.Title,
			Artist:     e.Item.Artist,
			DurationMS: e.Item.DurationMS,
			CueInMS:    e.Item.CueInMS,
			CueOutMS:   e.Item.CueOutMS,
			GainDB:     e.Item.GainDB,
			Mandatory:  e.Item.Mandatory,
			Metadata:   e.Item.Metadata,
		},
	}
	if !e.LastFiredAt.IsZero() {
		t := e.LastFiredAt
		v.LastFiredAt = &t
	}
	if e.Item.Transition != nil {
		v.Item.Transition = &transitionInput{
			Type:       e.Item.Transition.Type,
			DurationMS: e.Item.Transition.DurationMS,
		}
	}
	return v
}

// --- handlers ----------------------------------------------------------------

// ScheduleAdd handles POST /v1/schedule.
func ScheduleAdd(mgr ScheduleManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req scheduleAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
			return
		}
		if reason := validateScheduleRequest(req); reason != "" {
			writeError(w, http.StatusBadRequest, "invalid_payload", reason)
			return
		}

		id, err := mgr.Add(toScheduleEntry(req))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_entry", err.Error())
			return
		}

		entry, _ := mgr.Get(id)
		writeJSON(w, http.StatusCreated, map[string]any{
			"ok":    true,
			"entry": toScheduleView(entry, mgr.NextFireAt(id)),
		})
	}
}

// ScheduleList handles GET /v1/schedule.
func ScheduleList(mgr ScheduleManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries := mgr.List()
		views := make([]scheduleEntryView, len(entries))
		for i, e := range entries {
			views[i] = toScheduleView(e, mgr.NextFireAt(e.ID))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"entries": views,
			"count":   len(views),
		})
	}
}

// ScheduleGet handles GET /v1/schedule/{id}.
func ScheduleGet(mgr ScheduleManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		entry, ok := mgr.Get(id)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "schedule entry not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    true,
			"entry": toScheduleView(entry, mgr.NextFireAt(id)),
		})
	}
}

// ScheduleUpdate handles PUT /v1/schedule/{id}.
func ScheduleUpdate(mgr ScheduleManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req scheduleAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
			return
		}
		if reason := validateScheduleRequest(req); reason != "" {
			writeError(w, http.StatusBadRequest, "invalid_payload", reason)
			return
		}

		if err := mgr.Update(id, toScheduleEntry(req)); err != nil {
			// Distinguish not-found from validation errors.
			if _, exists := mgr.Get(id); !exists {
				writeError(w, http.StatusNotFound, "not_found", "schedule entry not found")
			} else {
				writeError(w, http.StatusBadRequest, "invalid_entry", err.Error())
			}
			return
		}

		entry, _ := mgr.Get(id)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    true,
			"entry": toScheduleView(entry, mgr.NextFireAt(id)),
		})
	}
}

// ScheduleDelete handles DELETE /v1/schedule/{id}.
func ScheduleDelete(mgr ScheduleManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := mgr.Get(id); !ok {
			writeError(w, http.StatusNotFound, "not_found", "schedule entry not found")
			return
		}
		mgr.Remove(id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// ScheduleEnable handles POST /v1/schedule/{id}/enable.
func ScheduleEnable(mgr ScheduleManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !mgr.Enable(id) {
			writeError(w, http.StatusNotFound, "not_found", "schedule entry not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "entry_id": id, "enabled": true})
	}
}

// ScheduleDisable handles POST /v1/schedule/{id}/disable.
func ScheduleDisable(mgr ScheduleManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !mgr.Disable(id) {
			writeError(w, http.StatusNotFound, "not_found", "schedule entry not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "entry_id": id, "enabled": false})
	}
}
