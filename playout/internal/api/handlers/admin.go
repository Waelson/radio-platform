package handlers

import (
	"net/http"
	"os"
	"runtime"
	"syscall"
)

// BuildInfo is the response for GET /v1/build.
type BuildInfo struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Shutdown handles POST /v1/admin/shutdown.
// It sends SIGTERM to the current process, triggering the engine's graceful
// shutdown sequence (same as pressing Ctrl-C or killing the process).
func Shutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"ok":      true,
			"message": "shutdown initiated",
		})
		// Signal after the response is written.
		go func() { _ = syscall.Kill(os.Getpid(), syscall.SIGTERM) }()
	}
}

// Build returns a handler for GET /v1/build that exposes build-time metadata.
func Build(version string) http.HandlerFunc {
	info := BuildInfo{
		Version:   version,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, info)
	}
}
