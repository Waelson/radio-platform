package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/commands"
)

// enterPanicRequest is the JSON body for POST /v1/panic/enter.
type enterPanicRequest struct {
	Reason string         `json:"reason"`
	Bed    *panicBedInput `json:"bed,omitempty"`
}

// panicBedInput describes the safety audio file to loop during panic.
type panicBedInput struct {
	AssetID string `json:"asset_id,omitempty"`
	Path    string `json:"path"`
}

// exitPanicRequest is the JSON body for POST /v1/panic/exit.
type exitPanicRequest struct {
	Reason string `json:"reason"`
}

// EnterPanic returns a handler for POST /v1/panic/enter.
// ENTER_PANIC has maximum priority and is accepted in any state except STOPPING.
func EnterPanic(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req enterPanicRequest
		if r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
				return
			}
		}

		payload := commands.EnterPanicPayload{Reason: req.Reason}
		if req.Bed != nil {
			if req.Bed.Path == "" {
				writeError(w, http.StatusBadRequest, "invalid_payload", "field bed.path is required when bed is provided")
				return
			}
			payload.Bed = &commands.PanicBedInput{
				AssetID: req.Bed.AssetID,
				Path:    req.Bed.Path,
			}
		}

		cmd, replyCh := commands.NewSync(commands.CmdEnterPanic, payload)
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

// ExitPanic returns a handler for POST /v1/panic/exit.
func ExitPanic(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req exitPanicRequest
		if r.ContentLength != 0 {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		cmd, replyCh := commands.NewSync(commands.CmdExitPanic, commands.ExitPanicPayload{Reason: req.Reason})
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
