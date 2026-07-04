package handlers

import (
	"net/http"

	"github.com/Waelson/radio-playout-engine/internal/state"
)

// healthResponse is the payload for GET /v1/health.
type healthResponse struct {
	Status      string `json:"status"`
	Engine      string `json:"engine"`
	AudioOutput string `json:"audio_output"`
}

// Health returns a handler for GET /v1/health.
// It always returns 200 as long as the process is alive.
func Health(stateMgr *state.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := stateMgr.Snapshot()

		audioOutput := "ready"
		if snap.State == state.StateError {
			audioOutput = "error"
		}

		writeJSON(w, http.StatusOK, healthResponse{
			Status:      "ok",
			Engine:      "running",
			AudioOutput: audioOutput,
		})
	}
}

// readyResponse is the payload for GET /v1/ready.
type readyResponse struct {
	Ready  bool   `json:"ready"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// Ready returns a handler for GET /v1/ready.
// It returns 200 only when the engine has left the STARTING state.
func Ready(stateMgr *state.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := stateMgr.Snapshot()

		if snap.State == state.StateStarting {
			writeJSON(w, http.StatusServiceUnavailable, readyResponse{
				Ready:  false,
				State:  string(snap.State),
				Reason: "engine is still starting",
			})
			return
		}

		writeJSON(w, http.StatusOK, readyResponse{
			Ready: true,
			State: string(snap.State),
		})
	}
}
