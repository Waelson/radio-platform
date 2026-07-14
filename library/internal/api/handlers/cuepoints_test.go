package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
)

func ptr64(v int64) *int64 { return &v }

func TestSaveCuePoints_Success(t *testing.T) {
	fs := seedStore() // track "id1" has DurationMS: 214000

	body := `{"cue_in_ms": 500, "intro_ms": 15000, "outro_ms": 200000, "cue_out_ms": 213000}`
	req := httptest.NewRequest(http.MethodPut, "/v1/tracks/id1/cuepoints", bytes.NewBufferString(body))
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()

	handlers.SaveCuePoints(fs).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
	if resp["track_id"] != "id1" {
		t.Errorf("expected track_id=id1, got %v", resp["track_id"])
	}
	if resp["cue_in_ms"] == nil {
		t.Error("cue_in_ms should not be nil")
	}
}

func TestSaveCuePoints_ClearAll(t *testing.T) {
	fs := seedStore()
	// Set some values first.
	fs.tracks[0].CueInMS = ptr64(500)
	fs.tracks[0].IntroMS = ptr64(15000)

	body := `{"cue_in_ms": null, "intro_ms": null, "outro_ms": null, "cue_out_ms": null}`
	req := httptest.NewRequest(http.MethodPut, "/v1/tracks/id1/cuepoints", bytes.NewBufferString(body))
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()

	handlers.SaveCuePoints(fs).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Verify cleared in store.
	if fs.tracks[0].CueInMS != nil {
		t.Error("expected CueInMS to be nil after clear")
	}
}

func TestSaveCuePoints_InvalidOrdering(t *testing.T) {
	fs := seedStore()

	// intro_ms > outro_ms — invalid
	body := `{"intro_ms": 200000, "outro_ms": 15000}`
	req := httptest.NewRequest(http.MethodPut, "/v1/tracks/id1/cuepoints", bytes.NewBufferString(body))
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()

	handlers.SaveCuePoints(fs).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "invalid_cuepoints" {
		t.Errorf("expected error=invalid_cuepoints, got %v", resp["error"])
	}
}

func TestSaveCuePoints_NegativeValue(t *testing.T) {
	fs := seedStore()

	body := `{"cue_in_ms": -1}`
	req := httptest.NewRequest(http.MethodPut, "/v1/tracks/id1/cuepoints", bytes.NewBufferString(body))
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()

	handlers.SaveCuePoints(fs).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaveCuePoints_CueOutExceedsDuration(t *testing.T) {
	fs := seedStore() // id1 DurationMS: 214000

	body := `{"cue_out_ms": 300000}` // > 214000
	req := httptest.NewRequest(http.MethodPut, "/v1/tracks/id1/cuepoints", bytes.NewBufferString(body))
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()

	handlers.SaveCuePoints(fs).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaveCuePoints_TrackNotFound(t *testing.T) {
	fs := seedStore()

	body := `{"cue_in_ms": 500}`
	req := httptest.NewRequest(http.MethodPut, "/v1/tracks/nonexistent/cuepoints", bytes.NewBufferString(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handlers.SaveCuePoints(fs).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSaveCuePoints_InvalidJSON(t *testing.T) {
	fs := seedStore()

	req := httptest.NewRequest(http.MethodPut, "/v1/tracks/id1/cuepoints", bytes.NewBufferString("not json"))
	req.SetPathValue("id", "id1")
	w := httptest.NewRecorder()

	handlers.SaveCuePoints(fs).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
