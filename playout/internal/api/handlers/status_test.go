package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

func TestStatus_BasicFields(t *testing.T) {
	mgr := state.NewManager("studio-a")
	mgr.SetState(state.StatePlaying)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	handlers.Status(mgr).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if body["engine_id"] != "studio-a" {
		t.Errorf("engine_id = %v", body["engine_id"])
	}
	if body["state"] != "PLAYING" {
		t.Errorf("state = %v", body["state"])
	}
	if body["mode"] != "AUTO" {
		t.Errorf("mode = %v", body["mode"])
	}
}

func TestStatus_NowPlaying_Present(t *testing.T) {
	mgr := state.NewManager("test")
	mgr.SetNowPlaying(&state.NowPlaying{
		QueueItemID: "qi_1",
		AssetID:     "a1",
		Title:       "Track A",
		Artist:      "Artist A",
		Type:        "MUSIC",
		DurationMS:  240000,
		PositionMS:  60000,
		Percent:     25.0,
		Transition:  &state.TransitionInfo{Type: "CROSSFADE", DurationMS: 8000},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	handlers.Status(mgr).ServeHTTP(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)

	np, ok := body["now_playing"].(map[string]any)
	if !ok {
		t.Fatal("now_playing field missing or wrong type")
	}
	if np["queue_item_id"] != "qi_1" {
		t.Errorf("queue_item_id = %v", np["queue_item_id"])
	}
	if np["title"] != "Track A" {
		t.Errorf("title = %v", np["title"])
	}
	if np["percent"] != 25.0 {
		t.Errorf("percent = %v", np["percent"])
	}
	tr, ok := np["transition"].(map[string]any)
	if !ok {
		t.Fatal("transition missing")
	}
	if tr["type"] != "CROSSFADE" {
		t.Errorf("transition.type = %v", tr["type"])
	}
}

func TestStatus_NowPlaying_Absent(t *testing.T) {
	mgr := state.NewManager("test")
	mgr.SetState(state.StateIdle)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	handlers.Status(mgr).ServeHTTP(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)

	if _, exists := body["now_playing"]; exists {
		if body["now_playing"] != nil {
			t.Error("now_playing should be absent or null when not playing")
		}
	}
}

func TestStatus_LastCommand_Accepted(t *testing.T) {
	mgr := state.NewManager("test")
	mgr.RecordLastCommand("PLAY", true)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	handlers.Status(mgr).ServeHTTP(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)

	lc, ok := body["last_command"].(map[string]any)
	if !ok {
		t.Fatal("last_command missing")
	}
	if lc["command"] != "PLAY" {
		t.Errorf("command = %v", lc["command"])
	}
	if lc["status"] != "ACCEPTED" {
		t.Errorf("status = %v", lc["status"])
	}
}

func TestStatus_LastCommand_Rejected(t *testing.T) {
	mgr := state.NewManager("test")
	mgr.RecordLastCommand("SKIP", false)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	handlers.Status(mgr).ServeHTTP(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)

	lc := body["last_command"].(map[string]any)
	if lc["status"] != "REJECTED" {
		t.Errorf("status = %v, want REJECTED", lc["status"])
	}
}

func TestStatus_PanicFlag(t *testing.T) {
	mgr := state.NewManager("test")
	mgr.SetState(state.StatePanic)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	handlers.Status(mgr).ServeHTTP(rr, req)

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)

	if body["panic"] != true {
		t.Errorf("panic = %v, want true", body["panic"])
	}
}
