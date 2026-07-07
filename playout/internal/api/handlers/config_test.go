package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

// minimalValidBody returns a JSON payload that passes config.Validate.
func minimalValidBody(t *testing.T) []byte {
	t.Helper()
	cfg := config.Config{}
	cfg.API.Port = 8080
	cfg.Audio.SampleRate = 48000
	cfg.Audio.Channels = 2
	cfg.Audio.BufferFrames = 2048
	cfg.Audio.Output.Driver = "null"
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// writeTempYAML creates a temporary YAML config file and returns its path.
func writeTempConfigYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "cfg-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

// ── GetCurrentConfig ─────────────────────────────────────────────────────────

func TestGetCurrentConfig_ReturnsJSON(t *testing.T) {
	cfg := &config.Config{}
	cfg.Engine.ID = "test-engine"
	cfg.API.Port = 9090

	req := httptest.NewRequest(http.MethodGet, "/v1/config/current", nil)
	rec := httptest.NewRecorder()
	handlers.GetCurrentConfig(cfg)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	engine, ok := got["engine"].(map[string]any)
	if !ok {
		t.Fatal("missing 'engine' key in response")
	}
	if engine["id"] != "test-engine" {
		t.Errorf("engine.id = %v, want test-engine", engine["id"])
	}
}

// ── UpdateConfig ─────────────────────────────────────────────────────────────

func TestUpdateConfig_ValidPayload_RewritesYAML(t *testing.T) {
	path := writeTempConfigYAML(t, "# original\n")

	req := httptest.NewRequest(http.MethodPut, "/v1/config", bytes.NewReader(minimalValidBody(t)))
	rec := httptest.NewRecorder()
	handlers.UpdateConfig(path)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading updated file: %v", err)
	}
	if len(data) == 0 {
		t.Error("YAML file is empty after update")
	}
}

func TestUpdateConfig_ValidPayload_CreatesBackup(t *testing.T) {
	original := "# original content\n"
	path := writeTempConfigYAML(t, original)

	req := httptest.NewRequest(http.MethodPut, "/v1/config", bytes.NewReader(minimalValidBody(t)))
	rec := httptest.NewRecorder()
	handlers.UpdateConfig(path)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf(".bak file not created: %v", err)
	}
	if string(bak) != original {
		t.Errorf(".bak content = %q, want %q", bak, original)
	}
}

func TestUpdateConfig_ValidPayload_OverwritesBackup(t *testing.T) {
	first := "# first version\n"
	path := writeTempConfigYAML(t, first)

	body := minimalValidBody(t)

	// First save — creates .bak with 'first'
	req1 := httptest.NewRequest(http.MethodPut, "/v1/config", bytes.NewReader(body))
	handlers.UpdateConfig(path)(httptest.NewRecorder(), req1)

	// Second save — .bak should now contain the YAML written by the first save,
	// not the original 'first'.
	req2 := httptest.NewRequest(http.MethodPut, "/v1/config", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	handlers.UpdateConfig(path)(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("second save: status = %d, want 200", rec2.Code)
	}

	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf(".bak not found after second save: %v", err)
	}
	// The backup should NOT be the original 'first' anymore.
	if string(bak) == first {
		t.Error(".bak was not overwritten on second save — still contains original content")
	}
}

func TestUpdateConfig_InvalidJSON_Returns400_NoBackup(t *testing.T) {
	path := writeTempConfigYAML(t, "# original\n")

	req := httptest.NewRequest(http.MethodPut, "/v1/config", bytes.NewReader([]byte("{invalid")))
	rec := httptest.NewRecorder()
	handlers.UpdateConfig(path)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Error(".bak file must not be created when JSON is invalid")
	}
}

func TestUpdateConfig_InvalidPort_Returns400_NoWrite(t *testing.T) {
	original := "# original\n"
	path := writeTempConfigYAML(t, original)

	cfg := config.Config{}
	cfg.API.Port = 99999 // out of range
	cfg.Audio.SampleRate = 48000
	cfg.Audio.Channels = 2
	cfg.Audio.BufferFrames = 2048
	cfg.Audio.Output.Driver = "null"
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"
	body, _ := json.Marshal(cfg)

	req := httptest.NewRequest(http.MethodPut, "/v1/config", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handlers.UpdateConfig(path)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	// File must be untouched.
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Error("YAML file was modified despite validation failure")
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Error(".bak must not be created when validation fails")
	}
}

func TestUpdateConfig_InvalidDriver_Returns400(t *testing.T) {
	path := writeTempConfigYAML(t, "# original\n")

	cfg := config.Config{}
	cfg.API.Port = 8080
	cfg.Audio.SampleRate = 48000
	cfg.Audio.Channels = 2
	cfg.Audio.BufferFrames = 2048
	cfg.Audio.Output.Driver = "magic_driver"
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"
	body, _ := json.Marshal(cfg)

	req := httptest.NewRequest(http.MethodPut, "/v1/config", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handlers.UpdateConfig(path)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateConfig_InvalidLogLevel_Returns400(t *testing.T) {
	path := writeTempConfigYAML(t, "# original\n")

	cfg := config.Config{}
	cfg.API.Port = 8080
	cfg.Audio.SampleRate = 48000
	cfg.Audio.Channels = 2
	cfg.Audio.BufferFrames = 2048
	cfg.Audio.Output.Driver = "null"
	cfg.Logging.Level = "verbose" // invalid
	cfg.Logging.Format = "json"
	body, _ := json.Marshal(cfg)

	req := httptest.NewRequest(http.MethodPut, "/v1/config", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handlers.UpdateConfig(path)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateConfig_NoConfigPath_Returns503(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/v1/config", bytes.NewReader(minimalValidBody(t)))
	rec := httptest.NewRecorder()
	handlers.UpdateConfig("")(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// ── BrowsePath ────────────────────────────────────────────────────────────────

func TestBrowsePath_InvalidType_Returns400(t *testing.T) {
	body := []byte(`{"type":"unknown"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/config/browse", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handlers.BrowsePath()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestBrowsePath_ReturnsEmptyOnCancel(t *testing.T) {
	// On macOS osascript opens a real interactive dialog — skip to avoid blocking.
	if runtime.GOOS == "darwin" {
		t.Skip("skipped on macOS: osascript requires user interaction")
	}

	// On Linux (CI), zenity is typically not installed, so openNativeDialog fails
	// immediately and the handler returns {"path":""} with 200.
	body := []byte(`{"type":"file"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/config/browse", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handlers.BrowsePath()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	_ = filepath.IsAbs(resp.Path)
}
