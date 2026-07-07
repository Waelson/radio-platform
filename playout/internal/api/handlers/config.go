package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Waelson/radio-playout-engine/internal/config"
	"gopkg.in/yaml.v3"
)

// GetCurrentConfig returns the engine's startup config snapshot as JSON.
// The snapshot is fixed at startup; changes to the YAML file after startup
// are NOT reflected here until the engine restarts.
func GetCurrentConfig(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, cfg)
	}
}

// UpdateConfig validates the incoming JSON config, writes a .bak of the current
// YAML, then rewrites the YAML file. Only the last backup is kept.
// Returns 503 when the engine was not started with a YAML config file.
func UpdateConfig(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if configPath == "" {
			writeError(w, http.StatusServiceUnavailable, "no_config_file",
				"engine was not started with a YAML config file — cannot save")
			return
		}

		// 1. Decode body.
		var incoming config.Config
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		// 2. Validate before touching the filesystem.
		if err := config.Validate(&incoming); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_value", err.Error())
			return
		}

		// 3. Read current YAML for backup.
		current, err := os.ReadFile(configPath)
		if err != nil && !os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, "read_error", err.Error())
			return
		}

		// 4. Write .bak (overwrites any previous backup).
		if len(current) > 0 {
			if err := os.WriteFile(configPath+".bak", current, 0o644); err != nil {
				writeError(w, http.StatusInternalServerError, "backup_error", err.Error())
				return
			}
		}

		// 5. Marshal to YAML and write.
		data, err := yaml.Marshal(&incoming)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "marshal_error", err.Error())
			return
		}
		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, "write_error", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// BrowsePath opens a native OS file/folder dialog and returns the selected path.
// Body: { "type": "file" } or { "type": "dir" }
// Response: { "path": "/selected/path" } or { "path": "" } when cancelled.
func BrowsePath() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Type != "file" && req.Type != "dir" {
			writeError(w, http.StatusBadRequest, "invalid_type", `type must be "file" or "dir"`)
			return
		}

		path, _ := openNativeDialog(req.Type)
		writeJSON(w, http.StatusOK, map[string]string{"path": path})
	}
}

// openNativeDialog opens a platform-specific file or folder picker dialog.
// Returns the selected path (trimmed) or empty string if the user cancelled
// or the dialog tool is unavailable.
func openNativeDialog(kind string) (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		script := "POSIX path of (choose file)"
		if kind == "dir" {
			script = "POSIX path of (choose folder)"
		}
		cmd = exec.Command("osascript", "-e", script)

	case "linux":
		if kind == "dir" {
			cmd = exec.Command("zenity", "--file-selection", "--directory", "--title=Selecionar pasta")
		} else {
			cmd = exec.Command("zenity", "--file-selection", "--title=Selecionar arquivo")
		}

	case "windows":
		ps := `Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.OpenFileDialog; if ($d.ShowDialog() -eq 'OK') { $d.FileName }`
		if kind == "dir" {
			ps = `Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.FolderBrowserDialog; if ($d.ShowDialog() -eq 'OK') { $d.SelectedPath }`
		}
		cmd = exec.Command("powershell", "-NoProfile", "-Command", ps)

	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	out, err := cmd.Output()
	if err != nil {
		// User cancelled or tool not available — not a fatal error.
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
