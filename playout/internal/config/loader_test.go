package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.API.Host != "127.0.0.1" {
		t.Errorf("API.Host = %q, want 127.0.0.1", cfg.API.Host)
	}
	if cfg.API.Port != 8080 {
		t.Errorf("API.Port = %d, want 8080", cfg.API.Port)
	}
	if cfg.Audio.SampleRate != 48000 {
		t.Errorf("Audio.SampleRate = %d, want 48000", cfg.Audio.SampleRate)
	}
	if cfg.Audio.Channels != 2 {
		t.Errorf("Audio.Channels = %d, want 2", cfg.Audio.Channels)
	}
	if cfg.Audio.BufferFrames != 2048 {
		t.Errorf("Audio.BufferFrames = %d, want 2048", cfg.Audio.BufferFrames)
	}
	if cfg.Playback.DefaultCrossfadeMS != 8000 {
		t.Errorf("Playback.DefaultCrossfadeMS = %d, want 8000", cfg.Playback.DefaultCrossfadeMS)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want info", cfg.Logging.Level)
	}
}

func TestLoad_FromYAML(t *testing.T) {
	yaml := `
engine:
  id: "test-studio"
api:
  port: 9090
logging:
  level: "debug"
  format: "text"
audio:
  output:
    driver: "null"
`
	cfg, err := config.Load([]string{"--config", writeTempYAML(t, yaml)})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Engine.ID != "test-studio" {
		t.Errorf("Engine.ID = %q, want test-studio", cfg.Engine.ID)
	}
	if cfg.API.Port != 9090 {
		t.Errorf("API.Port = %d, want 9090", cfg.API.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want debug", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %q, want text", cfg.Logging.Format)
	}
}

func TestLoad_FlagOverridesYAML(t *testing.T) {
	yaml := `
api:
  port: 9090
logging:
  level: "info"
audio:
  output:
    driver: "null"
`
	cfg, err := config.Load([]string{
		"--config", writeTempYAML(t, yaml),
		"--api-port", "7070",
		"--log-level", "warn",
	})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.API.Port != 7070 {
		t.Errorf("API.Port = %d, want 7070 (flag must override yaml)", cfg.API.Port)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Logging.Level = %q, want warn (flag must override yaml)", cfg.Logging.Level)
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	yaml := `
api:
  port: 9090
audio:
  output:
    driver: "null"
`
	t.Setenv("PLAYOUT_API_PORT", "6060")
	t.Setenv("PLAYOUT_LOG_LEVEL", "debug")

	cfg, err := config.Load([]string{"--config", writeTempYAML(t, yaml)})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.API.Port != 6060 {
		t.Errorf("API.Port = %d, want 6060 (env must override yaml)", cfg.API.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want debug (env must override yaml)", cfg.Logging.Level)
	}
}

func TestLoad_FlagOverridesEnv(t *testing.T) {
	t.Setenv("PLAYOUT_API_PORT", "5000")

	cfg, err := config.Load([]string{"--api-port", "4000"})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.API.Port != 4000 {
		t.Errorf("API.Port = %d, want 4000 (flag must override env)", cfg.API.Port)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	_, err := config.Load([]string{"--api-port", "99999"})
	if err == nil {
		t.Error("expected error for port 99999, got nil")
	}
}

func TestLoad_InvalidDriver(t *testing.T) {
	yaml := `
audio:
  output:
    driver: "magic_driver"
`
	_, err := config.Load([]string{"--config", writeTempYAML(t, yaml)})
	if err == nil {
		t.Error("expected error for unknown driver, got nil")
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	yaml := `
audio:
  output:
    driver: "null"
logging:
  level: "verbose"
`
	_, err := config.Load([]string{"--config", writeTempYAML(t, yaml)})
	if err == nil {
		t.Error("expected error for invalid log level, got nil")
	}
}

func TestLoad_MissingConfigFile(t *testing.T) {
	_, err := config.Load([]string{"--config", filepath.Join(t.TempDir(), "nonexistent.yaml")})
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}
}

// writeTempYAML writes content to a temp file and returns its path.
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}
