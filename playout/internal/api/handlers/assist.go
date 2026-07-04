package handlers

import (
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/commands"
)

// EnterAssist returns a handler for POST /v1/playback/enter-assist.
func EnterAssist(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, replyCh := commands.NewSync(commands.CmdEnterAssist, commands.EnterAssistPayload{})
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

// ReturnAuto returns a handler for POST /v1/playback/return-auto.
func ReturnAuto(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, replyCh := commands.NewSync(commands.CmdReturnAuto, commands.ReturnAutoPayload{})
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
