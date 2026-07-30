package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// ── Interfaces ────────────────────────────────────────────────────────────────

// lineInStateReader is the subset of state.Manager used by line-in handlers.
type lineInStateReader interface {
	Snapshot() state.Snapshot
}

// LineInConfigStore persists and retrieves the default line-in device.
type LineInConfigStore interface {
	GetLineInDefaultDeviceID() string
	SetLineInDefaultDeviceID(deviceID string) error
}

// ── DTOs ─────────────────────────────────────────────────────────────────────

// LineInDevice is the API DTO for a single audio input device.
type LineInDevice struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

// lineInStartRequest is the JSON body for POST /v1/linein/start.
type lineInStartRequest struct {
	DeviceID             string  `json:"device_id"`
	Label                string  `json:"label"`
	DurationMS           int64   `json:"duration_ms"`
	OnSilence            string  `json:"on_silence"`
	SilenceThresholdDBFS float64 `json:"silence_threshold_dbfs"`
	SilenceThresholdMS   int64   `json:"silence_threshold_ms"`
	TriggerMode          string  `json:"trigger_mode"`
}

// lineInStopRequest is the JSON body for POST /v1/linein/stop.
type lineInStopRequest struct {
	Reason string `json:"reason"`
}

// lineInConfigRequest is the JSON body for PATCH /v1/linein/config.
type lineInConfigRequest struct {
	DefaultDeviceID string `json:"default_device_id"`
}

// lineInStartData is the data field in the 200 response for POST /v1/linein/start.
type lineInStartData struct {
	Active     bool      `json:"active"`
	DeviceID   string    `json:"device_id"`
	Label      string    `json:"label"`
	StartedAt  time.Time `json:"started_at"`
	DurationMS int64     `json:"duration_ms,omitempty"`
}

// lineInStatusData is the data field in the 200 response for GET /v1/linein/status.
type lineInStatusData struct {
	Active    bool      `json:"active"`
	DeviceID  string    `json:"device_id,omitempty"`
	Label     string    `json:"label,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	DurationMS int64    `json:"duration_ms,omitempty"`
	ElapsedMS  int64    `json:"elapsed_ms,omitempty"`
}

// lineInStopData is the data field in the 200 response for POST /v1/linein/stop.
type lineInStopData struct {
	Active    bool      `json:"active"`
	StoppedAt time.Time `json:"stopped_at"`
}

// lineInDevicesData is the response data for GET /v1/linein/devices.
type lineInDevicesData struct {
	Devices []LineInDevice `json:"devices"`
	Count   int            `json:"count"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// LineInStart returns a handler for POST /v1/linein/start.
// It validates the current engine state, then dispatches CmdLineInStart.
func LineInStart(bus queueBus, stateMgr lineInStateReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse body (optional — all fields have defaults).
		var req lineInStartRequest
		if r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
				return
			}
		}

		snap := stateMgr.Snapshot()

		// Pre-condition checks (synchronous — no need to wait for dispatcher).
		if snap.State == state.StateLineIn {
			writeError(w, http.StatusConflict, "linein_already_active", "a line-in session is already active")
			return
		}
		if snap.State == state.StatePanic {
			writeError(w, http.StatusServiceUnavailable, "engine_in_panic", "engine is in PANIC mode — line-in refused")
			return
		}

		deviceID := req.DeviceID
		if deviceID == "" {
			deviceID = "default"
		}

		payload := commands.LineInStartPayload{
			DeviceID:             deviceID,
			Label:                req.Label,
			DurationMS:           req.DurationMS,
			OnSilence:            req.OnSilence,
			SilenceThresholdDBFS: req.SilenceThresholdDBFS,
			SilenceThresholdMS:   req.SilenceThresholdMS,
			TriggerMode:          req.TriggerMode,
			TriggeredBy:          "manual",
		}

		cmd, replyCh := commands.NewSync(commands.CmdLineInStart, payload)
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		if !result.Accepted {
			writeError(w, http.StatusConflict, "command_rejected", result.Reason)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": lineInStartData{
				Active:     true,
				DeviceID:   deviceID,
				Label:      req.Label,
				StartedAt:  time.Now().UTC(),
				DurationMS: req.DurationMS,
			},
		})
	}
}

// LineInStop returns a handler for POST /v1/linein/stop.
func LineInStop(bus queueBus, stateMgr lineInStateReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req lineInStopRequest
		if r.ContentLength != 0 {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		snap := stateMgr.Snapshot()
		if snap.State != state.StateLineIn {
			writeError(w, http.StatusConflict, "linein_not_active", "no line-in session is currently active")
			return
		}

		cmd, replyCh := commands.NewSync(commands.CmdLineInStop, commands.LineInStopPayload{Reason: req.Reason})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		if !result.Accepted {
			writeError(w, http.StatusConflict, "command_rejected", result.Reason)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": lineInStopData{
				Active:    false,
				StoppedAt: time.Now().UTC(),
			},
		})
	}
}

// LineInStatus returns a handler for GET /v1/linein/status.
func LineInStatus(stateMgr lineInStateReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := stateMgr.Snapshot()

		if snap.State != state.StateLineIn || snap.LineIn == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":   true,
				"data": lineInStatusData{Active: false},
			})
			return
		}

		li := snap.LineIn
		elapsed := time.Since(li.StartedAt).Milliseconds()

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"data": lineInStatusData{
				Active:     true,
				DeviceID:   li.DeviceID,
				Label:      li.Label,
				StartedAt:  li.StartedAt,
				DurationMS: li.DurationMS,
				ElapsedMS:  elapsed,
			},
		})
	}
}

// LineInDevices returns a handler for GET /v1/linein/devices.
// listDevices is called on every request (no caching).
// If nil, returns an empty list with 200 OK.
func LineInDevices(listDevices func() ([]LineInDevice, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		if listDevices == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":   true,
				"data": lineInDevicesData{Devices: []LineInDevice{}, Count: 0},
			})
			return
		}

		devs, err := listDevices()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "device_enumeration_failed", err.Error())
			return
		}
		if devs == nil {
			devs = []LineInDevice{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"data": lineInDevicesData{Devices: devs, Count: len(devs)},
		})
	}
}

// LineInPatchConfig returns a handler for PATCH /v1/linein/config.
// It persists the default input device ID via the provided store.
// If store is nil, returns 501 Not Implemented.
func LineInPatchConfig(store LineInConfigStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusNotImplemented, "prefs_unavailable", "prefs store is not configured")
			return
		}

		var req lineInConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
			return
		}
		if req.DefaultDeviceID == "" {
			writeError(w, http.StatusBadRequest, "missing_field", "field default_device_id is required")
			return
		}

		if err := store.SetLineInDefaultDeviceID(req.DefaultDeviceID); err != nil {
			writeError(w, http.StatusInternalServerError, "save_failed", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"data": map[string]any{"saved": true, "default_device_id": req.DefaultDeviceID},
		})
	}
}
