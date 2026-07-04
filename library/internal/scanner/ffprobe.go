package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Metadata holds the fields extracted from an audio file via ffprobe.
type Metadata struct {
	Title      string
	Artist     string
	Album      string // from "album" tag
	Genre      string // from "genre" tag — used as category in "tags" strategy
	DurationMS int64
}

// ffprobeOutput mirrors the JSON structure returned by ffprobe.
type ffprobeOutput struct {
	Format struct {
		Duration string            `json:"duration"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
}

// Probe extracts audio metadata from the file at filePath using ffprobe.
// If tags are absent or empty in the file, the corresponding fields in the
// returned Metadata will be empty strings — the caller should use nameparser
// as a fallback.
//
// ffprobePath may be an absolute path or just "ffprobe" for PATH lookup.
func Probe(ctx context.Context, ffprobePath, filePath string) (Metadata, error) {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}

	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		filePath,
	)

	out, err := cmd.Output()
	if err != nil {
		return Metadata{}, fmt.Errorf("ffprobe %q: %w", filePath, err)
	}

	var result ffprobeOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return Metadata{}, fmt.Errorf("ffprobe: parse output for %q: %w", filePath, err)
	}

	meta := Metadata{
		DurationMS: parseDurationMS(result.Format.Duration),
	}

	// Tags are case-insensitive in practice; normalise to lowercase lookup.
	tags := make(map[string]string, len(result.Format.Tags))
	for k, v := range result.Format.Tags {
		tags[strings.ToLower(k)] = v
	}
	meta.Title = strings.TrimSpace(tags["title"])
	meta.Artist = strings.TrimSpace(tags["artist"])
	meta.Album = strings.TrimSpace(tags["album"])
	meta.Genre = strings.TrimSpace(tags["genre"])

	return meta, nil
}

// parseDurationMS converts a duration string in seconds (e.g. "214.293") to
// milliseconds. Returns 0 on parse failure.
func parseDurationMS(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(f * 1000)
}
