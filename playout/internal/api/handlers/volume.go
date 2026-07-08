package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

type volumeResponse struct {
	Level float32 `json:"level"`
}

// GetVolume returns a handler for GET /v1/playback/volume.
func GetVolume(stateMgr *state.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, volumeResponse{Level: stateMgr.MainVolume()})
	}
}

// SetVolume returns a handler for PUT /v1/playback/volume.
func SetVolume(bus queueBus, stateMgr *state.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Level float32 `json:"level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Level < 0 || req.Level > 1 {
			writeError(w, http.StatusBadRequest, "invalid_level", "level must be between 0.0 and 1.0")
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdSetVolume, commands.SetVolumePayload{Level: req.Level})
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

// GetPreviewVolume returns a handler for GET /v1/preview/volume.
func GetPreviewVolume(stateMgr *state.Manager, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			previewUnavailable(w)
			return
		}
		writeJSON(w, http.StatusOK, volumeResponse{Level: stateMgr.PreviewVolume()})
	}
}

// SetPreviewVolume returns a handler for PUT /v1/preview/volume.
func SetPreviewVolume(bus queueBus, stateMgr *state.Manager, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			previewUnavailable(w)
			return
		}
		var req struct {
			Level float32 `json:"level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Level < 0 || req.Level > 1 {
			writeError(w, http.StatusBadRequest, "invalid_level", "level must be between 0.0 and 1.0")
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdPreviewSetVolume, commands.PreviewSetVolumePayload{Level: req.Level})
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
