// Package preview implements the isolated audio preview (cue) player.
// It is completely decoupled from the main playback pipeline and uses its own
// output device so the presenter can monitor audio without affecting the air signal.
package preview

// State represents the operational state of the preview player.
type State string

const (
	// StateIdle means no preview is active.
	StateIdle State = "idle"
	// StatePlaying means a preview is currently playing.
	StatePlaying State = "playing"
	// StatePaused means a preview is paused at a known position.
	StatePaused State = "paused"
)

// Status is a point-in-time snapshot of the preview player state.
type Status struct {
	State      State  `json:"state"`
	Path       string `json:"path,omitempty"`
	PositionMS int64  `json:"position_ms"`
	DurationMS int64  `json:"duration_ms"`
}
