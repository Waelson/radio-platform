//go:build windows

package linein

// buildCaptureArgs returns the ffmpeg arguments to capture from a hardware
// input device on Windows using the DirectShow (dshow) input format.
//
// Device IDs are reported by:
//
//	ffmpeg -f dshow -list_devices true -i dummy
//
// Examples:
//
//	"default"                        → "audio=Microphone (Realtek Audio)"
//	"Microphone (Realtek Audio)"     → "audio=Microphone (Realtek Audio)"
func buildCaptureArgs(deviceID string) []string {
	id := deviceID
	if id == "" || id == "default" {
		// dshow has no built-in "default" alias; use a well-known name as
		// placeholder. In practice the operator must configure a real device.
		id = "audio=default"
	} else {
		id = "audio=" + id
	}
	return []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "dshow",
		"-i", id,
	}
}
