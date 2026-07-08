//go:build linux

package cue

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// setCueProcAttr configures the subprocess with:
//   - Setpgid: own process group (signals don't propagate from engine group).
//   - Pdeathsig: SIGTERM delivered by the kernel when the parent process dies,
//     regardless of whether the stdin pipe is still open. This provides a
//     second layer of orphan prevention on Linux.
func setCueProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    true,
		Pdeathsig:  syscall.SIGTERM,
	}
}

// expandedEnv returns the current environment with common binary directories
// prepended to PATH.
func expandedEnv() []string {
	extraPaths := []string{
		"/usr/local/bin",
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
