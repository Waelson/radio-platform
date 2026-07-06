package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/commands"
)

// previewUnavailable writes the standard 503 response when preview is disabled.
func previewUnavailable(w http.ResponseWriter) {
	writeError(w, http.StatusServiceUnavailable, "preview_disabled",
		"audio preview is not enabled — set preview.enabled: true in config")
}

// PreviewPlay returns a handler for POST /v1/preview/play.
func PreviewPlay(bus queueBus, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			previewUnavailable(w)
			return
		}
		var req struct {
			Path   string `json:"path"`
			SeekMS int64  `json:"seek_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Path == "" {
			writeError(w, http.StatusBadRequest, "missing_path", "path is required")
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdPreviewPlay, commands.PreviewPlayPayload{
			Path:   req.Path,
			SeekMS: req.SeekMS,
		})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
		})
	}
}

// PreviewPause returns a handler for POST /v1/preview/pause.
func PreviewPause(bus queueBus, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			previewUnavailable(w)
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdPreviewPause, nil)
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
		})
	}
}

// PreviewResume returns a handler for POST /v1/preview/resume.
func PreviewResume(bus queueBus, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			previewUnavailable(w)
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdPreviewResume, nil)
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
		})
	}
}

// PreviewStop returns a handler for POST /v1/preview/stop.
func PreviewStop(bus queueBus, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			previewUnavailable(w)
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdPreviewStop, nil)
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
		})
	}
}

// PreviewSeek returns a handler for POST /v1/preview/seek.
func PreviewSeek(bus queueBus, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			previewUnavailable(w)
			return
		}
		var req struct {
			PositionMS int64 `json:"position_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdPreviewSeek, commands.PreviewSeekPayload{
			PositionMS: req.PositionMS,
		})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
		})
	}
}

// PreviewStatus returns a handler for GET /v1/preview/status.
// getStatus is a closure that returns the current preview state as any
// (compatible with preview.Status JSON shape).
func PreviewStatus(getStatus func() any, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			previewUnavailable(w)
			return
		}
		writeJSON(w, http.StatusOK, getStatus())
	}
}
