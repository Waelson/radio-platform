package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/commands"
)

// triggerHotButtonRequest is the JSON body for POST /v1/hotbuttons/trigger.
type triggerHotButtonRequest struct {
	ButtonID   string         `json:"button_id"`
	Asset      queueItemInput `json:"asset"`
	PlayMode   string         `json:"play_mode"`   // OVERLAY | INTERRUPT | AFTER_CURRENT
	DuckMain   bool           `json:"duck_main"`
	DuckGainDB float64        `json:"duck_gain_db"`
	Reason     string         `json:"reason,omitempty"`
}

// TriggerHotButton returns a handler for POST /v1/hotbuttons/trigger.
func TriggerHotButton(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req triggerHotButtonRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
			return
		}
		if req.Asset.Path == "" {
			writeError(w, http.StatusBadRequest, "invalid_payload", "field asset.path is required")
			return
		}
		validModes := map[string]bool{"OVERLAY": true, "INTERRUPT": true, "AFTER_CURRENT": true}
		if !validModes[req.PlayMode] {
			writeError(w, http.StatusBadRequest, "invalid_payload",
				"field play_mode must be OVERLAY, INTERRUPT, or AFTER_CURRENT")
			return
		}

		payload := commands.TriggerHotButtonPayload{
			ButtonID:   req.ButtonID,
			Asset:      toCommandItem(req.Asset),
			PlayMode:   req.PlayMode,
			DuckMain:   req.DuckMain,
			DuckGainDB: req.DuckGainDB,
			Reason:     req.Reason,
		}

		cmd, replyCh := commands.NewSync(commands.CmdTriggerHotButton, payload)
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
