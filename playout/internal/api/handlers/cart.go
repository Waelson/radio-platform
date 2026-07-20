package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/commands"
)

// CartPlay returns a handler for POST /v1/cart/play.
func CartPlay(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path     string  `json:"path"`
			Title    string  `json:"title"`
			Artist   string  `json:"artist"`
			GainDB   float64 `json:"gain_db"`
			CueInMS  int64   `json:"cue_in_ms"`
			IntroMS  int64   `json:"intro_ms"`
			OutroMS  int64   `json:"outro_ms"`
			CueOutMS int64   `json:"cue_out_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Path == "" {
			writeError(w, http.StatusBadRequest, "missing_path", "path is required")
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdCartPlay, commands.CartPlayPayload{
			Path:     req.Path,
			Title:    req.Title,
			Artist:   req.Artist,
			GainDB:   req.GainDB,
			CueInMS:  req.CueInMS,
			IntroMS:  req.IntroMS,
			OutroMS:  req.OutroMS,
			CueOutMS: req.CueOutMS,
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

// CartStop returns a handler for POST /v1/cart/stop.
func CartStop(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, replyCh := commands.NewSync(commands.CmdCartStop, nil)
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

// CartStatus returns a handler for GET /v1/cart/status.
// getStatus is a closure that returns the current cart state as any
// (compatible with cart.Status JSON shape).
func CartStatus(getStatus func() any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, getStatus())
	}
}
