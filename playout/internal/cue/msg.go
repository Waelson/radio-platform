// Package cue implements the isolated CUE (preview) player as a subprocess.
// The main engine manages a Proxy that spawns a child binary with
// --mode=cue-player and communicates over stdin/stdout using newline-delimited
// JSON. This separates the CUE CoreAudio client into its own OS process (Mach
// task), preventing HAL notifications from the BT/A2DP device from disrupting
// the main engine's AudioQueue.
package cue

// subCmd is a command sent from the main engine to the CUE subprocess via stdin.
// Each message is a JSON object followed by a newline (\n).
//
// cmd values: play | pause | resume | stop | seek | set_volume | quit
type subCmd struct {
	Cmd        string  `json:"cmd"`
	Path       string  `json:"path,omitempty"`        // play: absolute file path
	SeekMS     int64   `json:"seek_ms,omitempty"`     // play: start position
	PositionMS int64   `json:"position_ms,omitempty"` // seek: target position
	Volume     float32 `json:"volume,omitempty"`      // set_volume: 0.0–1.0
}

// subEvt is an event sent from the CUE subprocess to the main engine via stdout.
// Each message is a JSON object followed by a newline (\n).
//
// event values: ready | started | progress | paused | resumed | stopped | seeked | error
type subEvt struct {
	Event      string `json:"event"`
	Path       string `json:"path,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	PositionMS int64  `json:"position_ms,omitempty"`
	SeekMS     int64  `json:"seek_ms,omitempty"`
	Reason     string `json:"reason,omitempty"`  // stopped: end | stop | error
	Message    string `json:"message,omitempty"` // error: human-readable description
}
