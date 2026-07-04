//go:build !cli

package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// EngineProc manages the playout engine as a child process.
// The systray UI starts/stops/restarts the engine and polls its HTTP health endpoint.
type EngineProc struct {
	cmd     *exec.Cmd
	port    int
	started time.Time
	lastErr string
}

// NewEngineProc creates an EngineProc configured to communicate on the given port.
func NewEngineProc(port int) *EngineProc {
	return &EngineProc{port: port}
}

// Start launches the engine as a child process using the same binary path.
// It passes --startup=cli so the child runs in headless mode.
// Additional args (e.g. --config, --api-port) are forwarded transparently.
func (e *EngineProc) Start(extraArgs []string) error {
	if e.IsRunning() {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("engine_proc: locate self: %w", err)
	}

	args := append([]string{"--startup=cli"}, extraArgs...)
	cmd := exec.Command(self, args...)
	cmd.Env = ExpandedEnv()
	if lf, lerr := openEngineLog(); lerr == nil {
		cmd.Stdout = lf
		cmd.Stderr = lf
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("engine_proc: start: %w", err)
	}
	e.cmd = cmd
	e.started = time.Now()
	e.lastErr = ""
	return nil
}

// Stop sends SIGTERM to the engine and waits up to 10 seconds before SIGKILL.
func (e *EngineProc) Stop() error {
	if e.cmd == nil || e.cmd.Process == nil {
		return nil
	}
	_ = e.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- e.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = e.cmd.Process.Kill()
		<-done
	}
	e.cmd = nil
	return nil
}

// Restart stops the engine (if running) and starts it again with the same args.
func (e *EngineProc) Restart(extraArgs []string) error {
	if err := e.Stop(); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	return e.Start(extraArgs)
}

// IsRunning reports whether the child process is alive.
func (e *EngineProc) IsRunning() bool {
	if e.cmd == nil || e.cmd.Process == nil {
		return false
	}
	err := e.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// Pid returns the child PID (0 if not running).
func (e *EngineProc) Pid() int {
	if e.cmd == nil || e.cmd.Process == nil {
		return 0
	}
	return e.cmd.Process.Pid
}

// Uptime returns a human-readable uptime string.
func (e *EngineProc) Uptime() string {
	if e.cmd == nil {
		return "—"
	}
	d := time.Since(e.started).Truncate(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// ExpandedEnv returns the current environment with extra directories prepended
// to PATH so that tools like ffmpeg are found when the engine runs as a
// subprocess of a macOS .app bundle (which inherits a minimal system PATH).
func ExpandedEnv() []string {
	extraPaths := []string{
		"/opt/homebrew/bin", // Homebrew — Apple Silicon
		"/usr/local/bin",    // Homebrew — Intel / manual installs
		"/opt/local/bin",    // MacPorts
		"/usr/bin",          // macOS system tools
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

// openEngineLog opens (or creates) ~/RadioFlow/logs/engine.log for appending.
func openEngineLog() (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, "RadioFlow", "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "engine.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}

type healthResp struct {
	Status string `json:"status"`
}

// Poll performs a GET /v1/health request and returns whether the engine is healthy.
func (e *EngineProc) Poll() bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/health", e.port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var h healthResp
	_ = json.NewDecoder(resp.Body).Decode(&h)
	return resp.StatusCode == http.StatusOK
}
