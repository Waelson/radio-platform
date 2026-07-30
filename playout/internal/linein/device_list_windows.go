//go:build windows

package linein

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ListDevices returns the audio input devices available via DirectShow on Windows.
// It runs: ffmpeg -f dshow -list_devices true -i dummy
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
		"-f", "dshow",
		"-list_devices", "true",
		"-i", "dummy",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // always exits with error when listing

	return parseDShowDevices(stderr.String()), nil
}

// dshowAudioHeader matches the DirectShow "DirectShow audio devices" line.
var dshowAudioHeader = regexp.MustCompile(`(?i)DirectShow audio devices`)

// dshowDeviceLine matches:  "Microphone (Realtek Audio)"
var dshowDeviceLine = regexp.MustCompile(`"([^"]+)"`)

func parseDShowDevices(output string) []DeviceInfo {
	var devices []DeviceInfo
	inAudio := false
	for _, line := range strings.Split(output, "\n") {
		if dshowAudioHeader.MatchString(line) {
			inAudio = true
			continue
		}
		if !inAudio {
			continue
		}
		m := dshowDeviceLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := strings.TrimSpace(m[1])
		if name == "" {
			continue
		}
		devices = append(devices, DeviceInfo{
			ID:   name,
			Name: name,
		})
	}
	return devices
}
