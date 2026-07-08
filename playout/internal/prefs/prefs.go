// Package prefs manages user-adjustable runtime preferences that survive
// engine restarts. Preferences are stored in a JSON file separate from the
// YAML configuration so that operational adjustments (e.g. volume) never
// modify the structural configuration file.
package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Preferences holds runtime preferences persisted across restarts.
type Preferences struct {
	MainVolume    float32 `json:"main_volume"`
	PreviewVolume float32 `json:"preview_volume"`
}

// DefaultPath returns the default preferences file path: ~/.radiocore/preferences.json.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".radiocore", "preferences.json")
}

// Load reads preferences from path. Returns defaults (1.0, 1.0) if the file
// does not exist or cannot be parsed — never returns an error to the caller.
func Load(path string) Preferences {
	p := Preferences{MainVolume: 1.0, PreviewVolume: 1.0}
	data, err := os.ReadFile(path)
	if err != nil {
		return p
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p
	}
	p.MainVolume = clamp01(p.MainVolume)
	p.PreviewVolume = clamp01(p.PreviewVolume)
	return p
}

// Save writes preferences atomically (temp file + rename) to avoid corruption
// on crash. Errors are returned so callers can log them, but they must not
// interrupt the engine.
func Save(path string, p Preferences) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
