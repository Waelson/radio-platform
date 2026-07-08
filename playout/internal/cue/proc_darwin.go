//go:build darwin

package cue

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// setCueProcAttr configures the subprocess to run in its own process group
// (Setpgid) so that signals sent to the engine's group do not propagate to
// the CUE subprocess — the proxy handles shutdown explicitly via stdin EOF.
func setCueProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// expandedEnv returns the current environment with common tool directories
// prepended to PATH so that ffmpeg/ffprobe are found when the subprocess runs
// inside a macOS .app bundle (which inherits a minimal LaunchServices PATH).
func expandedEnv() []string {
	extraPaths := []string{
		"/opt/homebrew/bin", // Homebrew — Apple Silicon
		"/usr/local/bin",    // Homebrew — Intel / manual installs
		"/opt/local/bin",    // MacPorts
		"/usr/bin",
		"/bin",
	}
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			current := strings.TrimPrefix(e, "PATH=")
			env[i] = "PATH=" + strings.Join(extraPaths, ":") + ":" + current
			return env
		}
	}
	return append(env, "PATH="+strings.Join(extraPaths, ":"))
}
