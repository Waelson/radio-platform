package scanner

import (
	"os/exec"
	"testing"
)

// ─── parseSilenceEnd unit tests ───────────────────────────────────────────────

func TestParseSilenceEnd_TypicalOutput(t *testing.T) {
	output := `
ffmpeg version 6.0
[silencedetect @ 0x600003b042c0] silence_start: 0
[silencedetect @ 0x600003b042c0] silence_end: 0.432 | silence_duration: 0.432
`
	got := parseSilenceEnd(output)
	if got != 432 {
		t.Errorf("parseSilenceEnd = %d, want 432", got)
	}
}

func TestParseSilenceEnd_MultipleEvents(t *testing.T) {
	// Only the first silence_end should be used (leading silence).
	output := `
[silencedetect @ 0x...] silence_end: 1.500 | silence_duration: 1.500
[silencedetect @ 0x...] silence_end: 90.200 | silence_duration: 0.200
`
	got := parseSilenceEnd(output)
	if got != 1500 {
		t.Errorf("parseSilenceEnd = %d, want 1500", got)
	}
}

func TestParseSilenceEnd_NoLeadingSilence(t *testing.T) {
	// No silence_end line — audio starts immediately.
	output := `ffmpeg version 6.0
[silencedetect @ 0x...] silence_start: 240.000
`
	got := parseSilenceEnd(output)
	if got != 0 {
		t.Errorf("parseSilenceEnd = %d, want 0 (no leading silence)", got)
	}
}

func TestParseSilenceEnd_EmptyOutput(t *testing.T) {
	got := parseSilenceEnd("")
	if got != 0 {
		t.Errorf("parseSilenceEnd = %d, want 0", got)
	}
}

func TestParseSilenceEnd_ZeroSilenceEnd(t *testing.T) {
	// silence_end: 0 means no real leading silence — treat as 0.
	output := `[silencedetect @ 0x...] silence_end: 0 | silence_duration: 0`
	got := parseSilenceEnd(output)
	if got != 0 {
		t.Errorf("parseSilenceEnd = %d, want 0", got)
	}
}

func TestParseSilenceEnd_FractionalMs(t *testing.T) {
	output := `[silencedetect @ 0x...] silence_end: 0.001 | silence_duration: 0.001`
	got := parseSilenceEnd(output)
	if got != 1 {
		t.Errorf("parseSilenceEnd = %d, want 1", got)
	}
}

// ─── DetectCueIn integration test (requires ffmpeg) ──────────────────────────

func TestDetectCueIn_Integration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH, skipping integration test")
	}

	// Generate a temporary audio file: 0.5 s silence + 1.0 s 440 Hz sine.
	// ffmpeg lavfi sources produce exactly the content we need without any
	// external fixture file.
	tmp := t.TempDir() + "/cue_test.mp3"
	genCmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono:d=0.5",
		"-f", "lavfi", "-i", "sine=f=440:r=44100:d=1.0",
		"-filter_complex", "[0:a][1:a]concat=n=2:v=0:a=1",
		"-c:a", "libmp3lame", "-q:a", "9",
		tmp,
	)
	if out, err := genCmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg could not generate fixture (%v): %s", err, out)
	}

	ms := DetectCueIn("ffmpeg", tmp)

	// The silence is 500 ms; allow ±200 ms tolerance for encoder overhead.
	if ms < 300 || ms > 700 {
		t.Errorf("DetectCueIn = %d ms, want ~500 ms (300–700 range)", ms)
	}
}
