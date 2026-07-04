//go:build cli

// player_data_cli.go is the CLI-mode stub for playerHTMLBytes.
//
// In CLI builds (go build -tags cli) the binary runs headless: there is no
// systray, no webview window, and no HTTP /player endpoint that serves HTML.
// The embedded player is therefore unnecessary.
//
// However, main.go references playerHTMLBytes unconditionally (it passes the
// value to the API server config). Go requires every referenced symbol to be
// defined in exactly one file per build, so this stub declares the variable
// as a nil slice to satisfy the compiler without pulling in the //go:embed
// machinery or the assets/player.html file.

package main

// playerHTMLBytes is nil in CLI builds. The API server accepts a nil slice
// and simply omits the GET /player route when no content is provided.
var playerHTMLBytes []byte
