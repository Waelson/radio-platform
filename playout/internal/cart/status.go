// Package cart implements the isolated cart player — a dedicated audio channel
// for hotkey-triggered playback. It is completely decoupled from the main
// playback pipeline and the preview/CUE channel.
package cart

// State represents the operational state of the cart player.
type State string

const (
	// StateIdle means no cart is active.
	StateIdle State = "idle"
	// StatePlaying means a cart is currently playing.
	StatePlaying State = "playing"
)

// Status is a point-in-time snapshot of the cart player state.
type Status struct {
	State      State  `json:"state"`
	CartID     string `json:"cart_id,omitempty"`
	Path       string `json:"path,omitempty"`
	Title      string `json:"title,omitempty"`
	Artist     string `json:"artist,omitempty"`
	PositionMS int64  `json:"position_ms"`
	DurationMS int64  `json:"duration_ms"`
}
