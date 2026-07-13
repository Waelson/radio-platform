// Package loudness provides EBU R128 loudness analysis via ffmpeg ebur128.
package loudness

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Result holds the EBU R128 loudness measurement returned by Analyze.
type Result struct {
	LUFS     float64 // integrated loudness in LUFS (ITU-R BS.1770)
	TruePeak float64 // true peak in dBFS (math.Inf(-1) when not measurable)
}

// Analyzer measures loudness using ffmpeg with the ebur128 filter.
type Analyzer struct {
	ffmpegPath string
	timeout    time.Duration
}

// NewAnalyzer creates an Analyzer.
// ffmpegPath may be an absolute path or "ffmpeg" for PATH lookup.
// timeout is applied per-file; 0 defaults to 120 s.
func NewAnalyzer(ffmpegPath string, timeout time.Duration) *Analyzer {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Analyzer{ffmpegPath: ffmpegPath, timeout: timeout}
}

// ffmpeg ebur128 summary lines:
//
//	  I:         -16.8 LUFS     ← integrated loudness
//	  Peak:       -1.2 dBFS     ← true peak (only with peak=true)
var (
	reLUFS     = regexp.MustCompile(`I:\s+([-\d.]+|-inf)\s+LUFS`)
	reTruePeak = regexp.MustCompile(`Peak:\s+([-\d.]+|-inf)\s+dBFS`)
)

// Analyze runs ffmpeg ebur128=peak=true on filePath and returns the measurement.
// ffmpeg writes the summary to stderr; we rely on parse success rather than
// the exit code because the null muxer may return non-zero.
func (a *Analyzer) Analyze(ctx context.Context, filePath string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// CombinedOutput captures both stdout and stderr (ebur128 summary is on stderr).
	out, _ := exec.CommandContext(ctx, a.ffmpegPath,
		"-hide_banner",
		"-i", filePath,
		"-af", "ebur128=peak=true",
		"-f", "null", "-",
	).CombinedOutput()

	res, err := parseEbur128(string(out))
	if err != nil {
		// Include the first 512 bytes of ffmpeg output in the error so that
		// callers can log it for diagnosis without reading the whole buffer.
		snippet := string(out)
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		return Result{}, fmt.Errorf("analyze %q: %w\nffmpeg output: %s", filePath, err, snippet)
	}
	return res, nil
}

// parseEbur128 extracts integrated loudness and true peak from ffmpeg output.
// Handles -inf values (completely silent audio).
func parseEbur128(output string) (Result, error) {
	m := reLUFS.FindStringSubmatch(output)
	if m == nil {
		return Result{}, fmt.Errorf("integrated loudness not found in ffmpeg output")
	}
	lufs, err := parseFloatOrInf(m[1])
	if err != nil {
		return Result{}, fmt.Errorf("parse LUFS %q: %w", m[1], err)
	}

	// True peak is optional — very short or silent files may not report it.
	truePeak := math.Inf(-1)
	if mp := reTruePeak.FindStringSubmatch(output); mp != nil {
		if v, e := parseFloatOrInf(mp[1]); e == nil {
			truePeak = v
		}
	}

	return Result{LUFS: lufs, TruePeak: truePeak}, nil
}

// parseFloatOrInf converts "-inf" to math.Inf(-1) and parses regular floats.
func parseFloatOrInf(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "-inf" {
		return math.Inf(-1), nil
	}
	return strconv.ParseFloat(s, 64)
}
