package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
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

// breakItemInput is the schedule-handler representation of a commercial break.
// It mirrors commands.BreakItemInput but uses queueItemInput for JSON decoding.
type breakItemInput struct {
	Title string           `json:"title"`
	Open  *queueItemInput  `json:"open,omitempty"`
	Spots []queueItemInput `json:"spots"`
	Close *queueItemInput  `json:"close,omitempty"`
}

// lineInInput is the schedule-handler DTO for a line-in entry.
type lineInInput struct {
	DeviceID             string  `json:"device_id,omitempty"`
	Label                string  `json:"label,omitempty"`
	DurationMS           int64   `json:"duration_ms,omitempty"`
	OnSilence            string  `json:"on_silence,omitempty"`
	SilenceThresholdDBFS float64 `json:"silence_threshold_dbfs,omitempty"`
	SilenceThresholdMS   int64   `json:"silence_threshold_ms,omitempty"`
	Record               bool    `json:"record,omitempty"`
	RecordPath           string  `json:"record_path,omitempty"`
	RecordFormat         string  `json:"record_format,omitempty"`
}

type scheduleAddRequest struct {
	Name        string          `json:"name"`
	Enabled     bool            `json:"enabled"`
	CronExpr    string          `json:"cron_expr"`
	FireAt      *time.Time      `json:"fire_at"`
	TriggerMode string          `json:"trigger_mode"`
	Item        *queueItemInput `json:"item,omitempty"`       // mutually exclusive with Break, LineIn, LineInStop
	Break       *breakItemInput `json:"break,omitempty"`      // mutually exclusive with Item, LineIn, LineInStop
	LineIn      *lineInInput    `json:"line_in,omitempty"`    // mutually exclusive with Item, Break, LineInStop
	LineInStop  bool            `json:"line_in_stop,omitempty"` // mutually exclusive with Item, Break, LineIn
}

type scheduleEntryView struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Enabled     bool            `json:"enabled"`
	CronExpr    string          `json:"cron_expr,omitempty"`
	FireAt      *time.Time      `json:"fire_at,omitempty"`
	TriggerMode string          `json:"trigger_mode"`
	Item        *queueItemInput `json:"item,omitempty"`
	Break       *breakItemInput `json:"break,omitempty"`
	LineIn      *lineInInput    `json:"line_in,omitempty"`
	LineInStop  bool            `json:"line_in_stop,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	LastFiredAt *time.Time      `json:"last_fired_at,omitempty"`
	NextFireAt  *time.Time      `json:"next_fire_at,omitempty"`
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
	// Count how many mutually exclusive payload types are set.
	payloadCount := 0
	if req.Item != nil {
		payloadCount++
	}
	if req.Break != nil {
		payloadCount++
	}
	if req.LineIn != nil {
		payloadCount++
	}
	if req.LineInStop {
		payloadCount++
	}
	if payloadCount > 1 {
		return "item, break, line_in and line_in_stop are mutually exclusive"
	}
	if payloadCount == 0 {
		return "exactly one of item, break, line_in or line_in_stop must be set"
	}
	if req.Item != nil {
		if req.Item.Path == "" && req.Item.Type != "HORA_CERTA" {
			return "field item.path is required"
		}
	}
	if req.Break != nil {
		if len(req.Break.Spots) == 0 {
			return "break.spots must have at least one item"
		}
		for i, s := range req.Break.Spots {
			if s.Path == "" {
				return fmt.Sprintf("break.spots[%d].path is required", i)
			}
		}
	}
	if req.TriggerMode != "" && !validTriggerModes[req.TriggerMode] {
		return "trigger_mode must be one of: INTERRUPT, AFTER_CURRENT, CROSSFADE, SKIP_IF_BUSY"
	}
	return ""
}

func toCommandBreak(b *breakItemInput) *commands.BreakItemInput {
	if b == nil {
		return nil
	}
	out := &commands.BreakItemInput{
		Title: b.Title,
		Spots: make([]commands.QueueItemInput, len(b.Spots)),
	}
	for i, s := range b.Spots {
		out.Spots[i] = toCommandItem(s)
	}
	if b.Open != nil {
		v := toCommandItem(*b.Open)
		out.Open = &v
	}
	if b.Close != nil {
		v := toCommandItem(*b.Close)
		out.Close = &v
	}
	return out
}

func toScheduleEntry(req scheduleAddRequest) scheduler.Entry {
	mode := scheduler.TriggerMode(req.TriggerMode)
	if mode == "" {
		mode = scheduler.TriggerAfterCurrent
	}
	e := scheduler.Entry{
		Name:        req.Name,
		Enabled:     req.Enabled,
		CronExpr:    req.CronExpr,
		FireAt:      req.FireAt,
		TriggerMode: mode,
		Break:       toCommandBreak(req.Break),
		LineInStop:  req.LineInStop,
	}
	if req.Item != nil {
		e.Item = toCommandItem(*req.Item)
	}
	if req.LineIn != nil {
		e.LineIn = &commands.LineInStartPayload{
			DeviceID:             req.LineIn.DeviceID,
			Label:                req.LineIn.Label,
			DurationMS:           req.LineIn.DurationMS,
			OnSilence:            req.LineIn.OnSilence,
			SilenceThresholdDBFS: req.LineIn.SilenceThresholdDBFS,
			SilenceThresholdMS:   req.LineIn.SilenceThresholdMS,
			Record:               req.LineIn.Record,
			RecordPath:           req.LineIn.RecordPath,
			RecordFormat:         req.LineIn.RecordFormat,
			TriggeredBy:          "schedule",
		}
	}
	return e
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
	}
	if !e.LastFiredAt.IsZero() {
		t := e.LastFiredAt
		v.LastFiredAt = &t
	}
	if e.Break != nil {
		bv := &breakItemInput{Title: e.Break.Title}
		bv.Spots = make([]queueItemInput, len(e.Break.Spots))
		for i, s := range e.Break.Spots {
			bv.Spots[i] = fromCommandItem(s)
		}
		if e.Break.Open != nil {
			ov := fromCommandItem(*e.Break.Open)
			bv.Open = &ov
		}
		if e.Break.Close != nil {
			cv := fromCommandItem(*e.Break.Close)
			bv.Close = &cv
		}
		v.Break = bv
	} else if e.LineIn != nil {
		v.LineIn = &lineInInput{
			DeviceID:             e.LineIn.DeviceID,
			Label:                e.LineIn.Label,
			DurationMS:           e.LineIn.DurationMS,
			OnSilence:            e.LineIn.OnSilence,
			SilenceThresholdDBFS: e.LineIn.SilenceThresholdDBFS,
			SilenceThresholdMS:   e.LineIn.SilenceThresholdMS,
			Record:               e.LineIn.Record,
			RecordPath:           e.LineIn.RecordPath,
			RecordFormat:         e.LineIn.RecordFormat,
		}
	} else if e.LineInStop {
		v.LineInStop = true
	} else {
		item := fromCommandItem(e.Item)
		v.Item = &item
	}
	return v
}

// fromCommandItem converts a commands.QueueItemInput to the handler-level DTO.
func fromCommandItem(c commands.QueueItemInput) queueItemInput {
	out := queueItemInput{
		AssetID:    c.AssetID,
		Path:       c.Path,
		Type:       c.Type,
		Title:      c.Title,
		Artist:     c.Artist,
		DurationMS: c.DurationMS,
		CueInMS:    c.CueInMS,
		CueOutMS:   c.CueOutMS,
		GainDB:     c.GainDB,
		Mandatory:  c.Mandatory,
		Metadata:   c.Metadata,
	}
	if c.Transition != nil {
		out.Transition = &transitionInput{
			Type:       c.Transition.Type,
			DurationMS: c.Transition.DurationMS,
		}
	}
	return out
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
