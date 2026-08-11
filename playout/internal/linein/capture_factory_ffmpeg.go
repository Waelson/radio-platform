//go:build !coreaudio

package linein

import "log/slog"

// NewCapture returns an FFmpegCapture on non-coreaudio platforms (Linux, Windows).
func NewCapture(log *slog.Logger) InputDevice {
	return NewFFmpegCapture(log)
}
