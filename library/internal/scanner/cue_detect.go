package scanner

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// reSilenceEnd matches the first "silence_end" line in ffmpeg silencedetect output.
// Example line: "[silencedetect @ 0x...] silence_end: 0.432 | silence_duration: 0.432"
var reSilenceEnd = regexp.MustCompile(`silence_end:\s*([\d.]+)`)

// DetectCueIn runs ffmpeg silencedetect on path and returns the first
// silence_end timestamp in milliseconds. This is the point where silence
// ends at the beginning of the file — i.e. where real audio content starts.
//
// Returns 0 when:
//   - ffmpeg is unavailable
//   - the file has no leading silence (no silence_end line in output)
//   - the output cannot be parsed
//
// ffmpegPath may be an absolute path or "ffmpeg" for PATH lookup.
func DetectCueIn(ffmpegPath, path string) int64 {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	out, _ := exec.Command(ffmpegPath,
		"-hide_banner", "-vn",
		"-i", path,
		"-af", "silencedetect=n=-50dB:d=0.1",
		"-f", "null", "-",
	).CombinedOutput()
	return parseSilenceEnd(string(out))
}

// parseSilenceEnd extracts the first silence_end value (in milliseconds) from
// ffmpeg silencedetect output. Returns 0 when not found.
func parseSilenceEnd(output string) int64 {
	m := reSilenceEnd.FindStringSubmatch(output)
	if m == nil {
		return 0
	}
	s := strings.TrimSpace(m[1])
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return 0
	}
	return int64(f * 1000)
}
