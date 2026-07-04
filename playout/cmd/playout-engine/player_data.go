//go:build !cli

// player_data.go embeds the player UI into the binary at compile time.
//
// The //go:embed directive instructs the Go toolchain to read assets/player.html
// during `go build` and store its bytes directly inside the compiled executable.
// At runtime, playerHTMLBytes is already populated — no file I/O required.
//
// This makes the .app bundle self-contained: the player HTML is served by the
// engine's HTTP server at GET /player without any dependency on files on disk.
//
// Build constraint (!cli): this file is compiled only for the UI binary
// (systray + webview). The CLI counterpart (player_data_cli.go) provides
// an empty stub so that main.go compiles in headless mode as well.
//
// Note: //go:embed paths are relative to the source file's directory and
// cannot use "../". That is why assets/ lives alongside this file rather
// than in the project root.

package main

import _ "embed"

//go:embed assets/player.html
var playerHTMLBytes []byte
