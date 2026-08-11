//go:build coreaudio

package linein

import "log/slog"

// NewCapture returns a CoreAudioCapture on macOS coreaudio builds.
// Hardware-timed callbacks eliminate subprocess and pipe jitter.
func NewCapture(log *slog.Logger) InputDevice {
	return NewCoreAudioCapture(log)
}
