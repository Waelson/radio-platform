//go:build linux

package linein

// buildCaptureArgs returns the ffmpeg arguments to capture from a hardware
// input device on Linux using the ALSA input format.
//
// Device IDs are reported by:
//
//	arecord -l
//
// Examples:
//
//	"default"    → "default"
//	"hw:0,0"     → "hw:0,0"
//	"plughw:1,0" → "plughw:1,0"
func buildCaptureArgs(deviceID string) []string {
	id := deviceID
	if id == "" {
		id = "default"
	}
	return []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "alsa",
		"-i", id,
	}
}
