//go:build cli

package apptray

// RunSystray is a no-op stub used when the binary is built with the cli tag.
// In CLI mode the systray UI is excluded and the engine always runs headless.
func RunSystray() {}
