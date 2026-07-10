package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/commands"
)

// cartUnavailable writes the standard 503 response when the cart player is disabled.
func cartUnavailable(w http.ResponseWriter) {
	writeError(w, http.StatusServiceUnavailable, "cart_disabled",
		"cart player is not enabled — set cart.enabled: true in config")
}

// CartPlay returns a handler for POST /v1/cart/play.
func CartPlay(bus queueBus, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			cartUnavailable(w)
			return
		}
		var req struct {
			Path   string `json:"path"`
			Title  string `json:"title"`
			Artist string `json:"artist"`
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
			Path:   req.Path,
			Title:  req.Title,
			Artist: req.Artist,
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
func CartStop(bus queueBus, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			cartUnavailable(w)
			return
		}
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
func CartStatus(getStatus func() any, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			cartUnavailable(w)
			return
		}
		writeJSON(w, http.StatusOK, getStatus())
	}
}
