//go:build darwin

package linein

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ListDevices returns the audio input devices available via avfoundation on macOS.
// It runs: ffmpeg -f avfoundation -list_devices true -i ""
// and parses the stderr output.
func ListDevices(ffmpegPath string) ([]DeviceInfo, error) {
	bin := ffmpegPath
	if bin == "" {
		bin = "ffmpeg"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("linein list devices: %q not found on PATH: %w", bin, err)
	}

	cmd := exec.Command(bin,
		"-hide_banner",
		"-f", "avfoundation",
		"-list_devices", "true",
		"-i", "",
	)
	// avfoundation always exits with code 1 when listing devices; capture stderr.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run()

	return parseAVFoundationDevices(stderr.String()), nil
}

// audioSection matches the AVFoundation "AVFoundation audio devices:" header.
var avfAudioHeader = regexp.MustCompile(`(?i)AVFoundation audio devices`)

// avfDeviceLine matches lines like:  [AVFoundation indev @ ...] [0] Built-in Microphone
var avfDeviceLine = regexp.MustCompile(`\[(\d+)\]\s+(.+)`)

func parseAVFoundationDevices(output string) []DeviceInfo {
	var devices []DeviceInfo
	inAudio := false
	for _, line := range strings.Split(output, "\n") {
		if avfAudioHeader.MatchString(line) {
			inAudio = true
			continue
		}
		if !inAudio {
			continue
		}
		m := avfDeviceLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		idx := m[1]
		name := strings.TrimSpace(m[2])
		devices = append(devices, DeviceInfo{
			ID:   idx,
			Name: name,
		})
	}
	return devices
}
