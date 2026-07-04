package handlers

import (
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/commands"
)

// Play returns a handler for POST /v1/playback/play.
func Play(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, replyCh := commands.NewSync(commands.CmdPlay, commands.PlayPayload{})
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

// Pause returns a handler for POST /v1/playback/pause.
func Pause(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, replyCh := commands.NewSync(commands.CmdPause, commands.PausePayload{})
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

// Resume returns a handler for POST /v1/playback/resume.
func Resume(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, replyCh := commands.NewSync(commands.CmdResume, commands.ResumePayload{})
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

// Stop returns a handler for POST /v1/playback/stop.
func Stop(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, replyCh := commands.NewSync(commands.CmdStop, commands.StopPayload{})
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

// Skip returns a handler for POST /v1/playback/skip.
func Skip(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, replyCh := commands.NewSync(commands.CmdSkip, commands.SkipPayload{})
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
