//go:build cli

package webview

// RunWebview and OpenPlayerWindow are no-ops in CLI mode (no UI).
func RunWebview(url string)      {}
func OpenPlayerWindow(url string) {}
