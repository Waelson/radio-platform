// Package config defines and loads the Library Service configuration.
// Precedence (highest to lowest): CLI flags > YAML file > defaults.
package config

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration for the Library Service.
type Config struct {
	Service ServiceConfig `yaml:"service"`
	API     APIConfig     `yaml:"api"`
	DB      DBConfig      `yaml:"database"`
	Scanner ScannerConfig `yaml:"scanner"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServiceConfig holds process-level settings.
type ServiceConfig struct {
	ID      string `yaml:"id"`
	Version string `yaml:"-"` // injected at build time
}

// APIConfig holds HTTP API settings.
type APIConfig struct {
	Host string     `yaml:"host"`
	Port int        `yaml:"port"`
	CORS CORSConfig `yaml:"cors"`
}

// CORSConfig controls cross-origin behaviour.
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// DBConfig holds database settings.
type DBConfig struct {
	Path string `yaml:"path"`
}

// ScannerConfig holds media library scanner settings.
type ScannerConfig struct {
	// LibraryRoot is the base directory that contains the type subdirectories.
	LibraryRoot string `yaml:"library_root"`
	// Directories maps subdirectory names to asset types.
	// e.g. {"musicas": "MUSIC", "vinhetas": "VINHETA"}
	Directories map[string]string `yaml:"directories"`
	// Extensions is the list of file extensions to index (e.g. ".mp3").
	Extensions []string `yaml:"extensions"`
	// FFprobePath is the path to the ffprobe binary. Defaults to "ffprobe" (PATH lookup).
	FFprobePath string `yaml:"ffprobe_path"`
	// WatchEnabled enables automatic re-indexing via filesystem events.
	WatchEnabled bool `yaml:"watch_enabled"`
	// MetadataSource controls where title, artist, album and category are extracted from.
	// "filename" (default): parsed from the file name pattern [Category] Artist - Album - Title.
	// "tags": read from embedded audio tags (ID3/Vorbis) via ffprobe.
	MetadataSource string `yaml:"metadata_source"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// defaults returns a Config with sensible defaults.
func defaults() Config {
	return Config{
		Service: ServiceConfig{
			ID: "library-01",
		},
		API: APIConfig{
			Host: "0.0.0.0",
			Port: 8081,
			CORS: CORSConfig{
				AllowedOrigins: []string{"*"},
			},
		},
		DB: DBConfig{
			Path: "./library.db",
		},
		Scanner: ScannerConfig{
			LibraryRoot: "/audio-library",
			Directories: map[string]string{
				"musicas":  "MUSIC",
				"vinhetas": "VINHETA",
				"jingles":  "JINGLE",
				"spots":    "SPOT",
				"efeitos":  "EFEITOS",
			},
			Extensions:     []string{".mp3", ".wav", ".flac", ".ogg", ".aac"},
			FFprobePath:    "ffprobe",
			WatchEnabled:   true,
			MetadataSource: "filename",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// Load reads configuration from CLI flags and an optional YAML file.
// The -config flag specifies the path to the YAML file; it is optional.
func Load(args []string) (*Config, error) {
	cfg := defaults()

	fs := flag.NewFlagSet("library-service", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to YAML configuration file")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("config: parse flags: %w", err)
	}

	if *configPath != "" {
		data, err := os.ReadFile(*configPath)
		if err != nil {
			return nil, fmt.Errorf("config: read file %q: %w", *configPath, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("config: parse YAML: %w", err)
		}
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.API.Port <= 0 || cfg.API.Port > 65535 {
		return fmt.Errorf("api.port must be between 1 and 65535, got %d", cfg.API.Port)
	}
	if cfg.DB.Path == "" {
		return fmt.Errorf("database.path must not be empty")
	}
	return nil
}
