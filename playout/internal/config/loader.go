package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultConfigFile = "playout-engine.yaml"

// Load builds a Config from multiple sources in order of precedence:
//  1. CLI flags (args — typically os.Args[1:])
//  2. Environment variables (PLAYOUT_* prefix)
//  3. YAML config file (--config flag or playout-engine.yaml if present)
//  4. Built-in defaults
func Load(args []string) (*Config, error) {
	cfg := defaults()

	flags, err := parseFlags(args)
	if err != nil {
		return nil, fmt.Errorf("config: parsing flags: %w", err)
	}

	// Resolve config file path: explicit flag takes precedence, then default.
	configPath := flags.configFile
	if configPath == "" {
		if _, err := os.Stat(defaultConfigFile); err == nil {
			configPath = defaultConfigFile
		}
	}

	if configPath != "" {
		if err := loadYAML(configPath, cfg); err != nil {
			return nil, fmt.Errorf("config: loading yaml %q: %w", configPath, err)
		}
	}

	applyEnv(cfg)
	applyFlags(flags, cfg)

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return cfg, nil
}

// ResolveConfigPath returns the YAML config file path that Load would use for
// the given args. Returns empty string if no file is found or flagged.
func ResolveConfigPath(args []string) string {
	flags, err := parseFlags(args)
	if err != nil {
		return ""
	}
	if flags.configFile != "" {
		return flags.configFile
	}
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}
	return ""
}

// defaults returns a Config populated with safe, sane defaults.
func defaults() *Config {
	return &Config{
		Engine: EngineConfig{
			ID:           "playout-engine",
			InstanceLock: true,
		},
		API: APIConfig{
			Host: "127.0.0.1",
			Port: 8080,
			CORS: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"http://localhost:3000", "http://localhost:3333", "http://localhost:5173"},
			},
		},
		Audio: AudioConfig{
			SampleRate:   48000,
			Channels:     2,
			BufferFrames: 2048,
			Output: OutputConfig{
				DeviceID:        "default",
				AllowNullOutput: true,
			},
		},
		Playback: PlaybackConfig{
			DefaultCrossfadeMS:         8000,
			DefaultStopFadeMS:          300,
			PreloadNextMS:              3000,
			MaxConsecutiveItemFailures: 3,
		},
		Health: HealthConfig{
			ProgressIntervalMS:    500,
			AudioHealthIntervalMS: 500,
			SilenceThresholdDBFS:  -60,
			SilenceDurationMS:     2000,
		},
		Panic: PanicConfig{
			Enabled:              true,
			AutoOnSilence:        false,
			SilenceThresholdDBFS: -60,
			SilenceDurationMS:    2000,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Admin: AdminConfig{
			ShutdownEnabled: false,
		},
		Preview: PreviewConfig{
			Enabled:      false,
			OutputDevice: "",
		},
		Scheduler: SchedulerConfig{
			Enabled:           true,
			Timezone:          "",
			StorePath:         "",
			MissedThresholdMS: 5000,
		},
	}
}

type cliFlags struct {
	configFile string
	apiPort    int
	logLevel   string
}

func parseFlags(args []string) (cliFlags, error) {
	fs := flag.NewFlagSet("playout-engine", flag.ContinueOnError)
	var f cliFlags
	fs.StringVar(&f.configFile, "config", "", "path to YAML config file")
	fs.IntVar(&f.apiPort, "api-port", 0, "API listen port (overrides config)")
	fs.StringVar(&f.logLevel, "log-level", "", "log level: debug|info|warn|error (overrides config)")
	if err := fs.Parse(args); err != nil {
		return cliFlags{}, err
	}
	return f, nil
}

func loadYAML(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parsing yaml: %w", err)
	}
	return nil
}

// applyEnv overlays environment variables (PLAYOUT_* prefix) onto cfg.
// Only the most operationally important fields are exposed via env.
func applyEnv(cfg *Config) {
	if v := os.Getenv("PLAYOUT_ENGINE_ID"); v != "" {
		cfg.Engine.ID = v
	}
	if v := os.Getenv("PLAYOUT_API_HOST"); v != "" {
		cfg.API.Host = v
	}
	if v := os.Getenv("PLAYOUT_API_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.API.Port = p
		}
	}
	if v := os.Getenv("PLAYOUT_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("PLAYOUT_LOG_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}
	if v := os.Getenv("PLAYOUT_AUDIO_SAMPLE_RATE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Audio.SampleRate = n
		}
	}
	if v := os.Getenv("PLAYOUT_AUDIO_CHANNELS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Audio.Channels = n
		}
	}
	if v := os.Getenv("PLAYOUT_AUDIO_OUTPUT_DEVICE"); v != "" {
		cfg.Audio.Output.DeviceID = v
	}
	if v := os.Getenv("PLAYOUT_PANIC_BED_PATH"); v != "" {
		cfg.Panic.BedPath = v
	}
	if v := os.Getenv("RADIOCORE_PREVIEW_ENABLED"); v != "" {
		cfg.Preview.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("RADIOCORE_PREVIEW_OUTPUT_DEVICE"); v != "" {
		cfg.Preview.OutputDevice = v
	}
}

func applyFlags(f cliFlags, cfg *Config) {
	if f.apiPort != 0 {
		cfg.API.Port = f.apiPort
	}
	if f.logLevel != "" {
		cfg.Logging.Level = f.logLevel
	}
}

// Validate checks that cfg fields are within acceptable bounds and enumerations.
func Validate(cfg *Config) error {
	if cfg.API.Port < 1 || cfg.API.Port > 65535 {
		return fmt.Errorf("api.port %d out of range [1, 65535]", cfg.API.Port)
	}
	if cfg.Audio.SampleRate <= 0 {
		return fmt.Errorf("audio.sample_rate must be positive, got %d", cfg.Audio.SampleRate)
	}
	if cfg.Audio.Channels <= 0 {
		return fmt.Errorf("audio.channels must be positive, got %d", cfg.Audio.Channels)
	}
	if cfg.Audio.BufferFrames <= 0 {
		return fmt.Errorf("audio.buffer_frames must be positive, got %d", cfg.Audio.BufferFrames)
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(cfg.Logging.Level)] {
		return fmt.Errorf("logging.level %q is not valid (valid: debug, info, warn, error)",
			cfg.Logging.Level)
	}

	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[strings.ToLower(cfg.Logging.Format)] {
		return fmt.Errorf("logging.format %q is not valid (valid: json, text)",
			cfg.Logging.Format)
	}

	return nil
}
