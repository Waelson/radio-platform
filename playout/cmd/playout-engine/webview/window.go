//go:build !cli

package webview

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/Waelson/radio-playout-engine/cmd/playout-engine/engine"
)

// OpenPlayerWindow re-launches the current binary with --webview=<url>
// so that the WKWebView runs in an isolated subprocess (own main thread).
func OpenPlayerWindow(url string) {
	self, err := os.Executable()
	if err != nil {
		openBrowser(url)
		return
	}
	cmd := exec.Command(self, "--webview="+url)
	cmd.Env = engine.ExpandedEnv()
	_ = cmd.Start()
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
