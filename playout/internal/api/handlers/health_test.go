package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

func TestHealth_AlwaysReturns200(t *testing.T) {
	for _, st := range []state.PlayerState{
		state.StateIdle,
		state.StatePlaying,
		state.StateError,
		state.StatePanic,
	} {
		t.Run(string(st), func(t *testing.T) {
			mgr := state.NewManager("test")
			mgr.SetState(st)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
			handlers.Health(mgr).ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rr.Code)
			}
			if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}

			var body map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["status"] != "ok" {
				t.Errorf("status field = %q, want ok", body["status"])
			}
		})
	}
}

func TestHealth_AudioOutput_Error_WhenStateError(t *testing.T) {
	mgr := state.NewManager("test")
	mgr.SetError("decoder died")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	handlers.Health(mgr).ServeHTTP(rr, req)

	var body map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["audio_output"] != "error" {
		t.Errorf("audio_output = %q, want error", body["audio_output"])
	}
}

func TestReady_503_WhenStarting(t *testing.T) {
	mgr := state.NewManager("test")
	// Initial state is STARTING.

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	handlers.Ready(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["ready"] != false {
		t.Error("ready should be false")
	}
}

func TestReady_200_WhenIdle(t *testing.T) {
	mgr := state.NewManager("test")
	mgr.SetState(state.StateIdle)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	handlers.Ready(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["ready"] != true {
		t.Error("ready should be true")
	}
}
