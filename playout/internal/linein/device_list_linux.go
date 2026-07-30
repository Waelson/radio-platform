//go:build linux

package linein

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ListDevices returns the audio input devices available via ALSA on Linux.
// It runs: arecord -l and parses the output.
func ListDevices(_ string) ([]DeviceInfo, error) {
	if _, err := exec.LookPath("arecord"); err != nil {
		return nil, fmt.Errorf("linein list devices: \"arecord\" not found on PATH: %w", err)
	}

	var out bytes.Buffer
	cmd := exec.Command("arecord", "-l")
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("linein list devices: arecord -l: %w", err)
	}
	return parseARecordDevices(out.String()), nil
}

// cardLine matches:  card 0: PCH [HDA Intel PCH], device 0: ALC269VC Analog [ALC269VC Analog]
var arecordCardLine = regexp.MustCompile(`card\s+(\d+):[^,]+,\s+device\s+(\d+):\s+(.+?)\s*\[`)

func parseARecordDevices(output string) []DeviceInfo {
	var devices []DeviceInfo
	for _, line := range strings.Split(output, "\n") {
		m := arecordCardLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		card := m[1]
		dev := m[2]
		name := strings.TrimSpace(m[3])
		devices = append(devices, DeviceInfo{
			ID:   fmt.Sprintf("hw:%s,%s", card, dev),
			Name: name,
		})
	}
	return devices
}
