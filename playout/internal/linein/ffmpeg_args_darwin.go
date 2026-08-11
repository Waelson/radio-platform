//go:build darwin

package linein

// buildCaptureArgs returns the ffmpeg arguments to capture from a hardware
// input device on macOS using the avfoundation input format.
//
// Device IDs are reported by:
//
//	ffmpeg -f avfoundation -list_devices true -i ""
//
// Examples:
//
//	"default"  → ":0" (first audio input)
//	"0"        → ":0"
//	":1"       → ":1" (second audio input)
func buildCaptureArgs(deviceID string) []string {
	id := resolveAVFoundationDevice(deviceID)
	return []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "avfoundation",
		"-thread_queue_size", "512",
		"-i", id,
	}
}

// resolveAVFoundationDevice normalises the deviceID to an avfoundation
// input string of the form ":<audio_index>".
// avfoundation uses "video:audio" notation; we only need audio, so
// video is left empty (e.g. ":0").
func resolveAVFoundationDevice(deviceID string) string {
	if deviceID == "" || deviceID == "default" {
		return ":0"
	}
	// If the caller already supplied the colon prefix (e.g. ":1"), keep it.
	if len(deviceID) > 0 && deviceID[0] == ':' {
		return deviceID
	}
	return ":" + deviceID
}
