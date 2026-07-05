//go:build !cli

package webview

import (
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/Waelson/radio-playout-engine/cmd/playout-engine/engine"
)

var (
	webviewMu    sync.Mutex
	webviewProcs []*os.Process
)

// OpenPlayerWindow re-launches the current binary with --webview=<url>
// so that the WKWebView runs in an isolated subprocess (own main thread).
func OpenPlayerWindow(url, title string) {
	self, err := os.Executable()
	if err != nil {
		openBrowser(url)
		return
	}
	cmd := exec.Command(self, "--webview="+url, "--webview-title="+title)
	cmd.Env = engine.ExpandedEnv()
	if err := cmd.Start(); err != nil {
		return
	}
	webviewMu.Lock()
	webviewProcs = append(webviewProcs, cmd.Process)
	webviewMu.Unlock()

	// Remove from list when process exits naturally.
	go func(p *os.Process) {
		_ = cmd.Wait()
		webviewMu.Lock()
		for i, proc := range webviewProcs {
			if proc == p {
				webviewProcs = append(webviewProcs[:i], webviewProcs[i+1:]...)
				break
			}
		}
		webviewMu.Unlock()
	}(cmd.Process)
}

// KillAll terminates all open webview subprocesses. Called on systray exit.
func KillAll() {
	webviewMu.Lock()
	procs := webviewProcs
	webviewProcs = nil
	webviewMu.Unlock()
	for _, p := range procs {
		_ = p.Kill()
	}
}

// openBrowser opens url in the default system browser (cross-platform fallback).
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
