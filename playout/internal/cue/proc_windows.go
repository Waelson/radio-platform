//go:build windows

package cue

import (
	"os"
	"os/exec"
)

// setCueProcAttr is a no-op on Windows. Orphan prevention relies solely on
// stdin EOF detection (the subprocess exits when it reads EOF from its stdin).
func setCueProcAttr(_ *exec.Cmd) {}

// expandedEnv returns the current environment unchanged on Windows.
// ffmpeg is expected to be on PATH via the system environment.
func expandedEnv() []string {
	return os.Environ()
}
