package loudness

import (
	"math"
	"testing"
)

// ─── parseEbur128 ────────────────────────────────────────────────────────────

func TestParseEbur128_Valid(t *testing.T) {
	// Typical ffmpeg ebur128=peak=true summary output.
	output := `
[Parsed_ebur128_0 @ 0x600003b042c0] Summary:

  Integrated loudness:
    I:         -14.2 LUFS
    Threshold:  -24.2 LUFS

  Loudness range:
    LRA:         4.1 LU
    Threshold:  -34.2 LUFS
    LRA low:   -16.5 LUFS
    LRA high:  -12.4 LUFS

  True peak:
    Peak:       -0.5 dBFS
`
	res, err := parseEbur128(output)
	if err != nil {
		t.Fatalf("parseEbur128: %v", err)
	}
	if res.LUFS != -14.2 {
		t.Errorf("LUFS = %v, want -14.2", res.LUFS)
	}
	if res.TruePeak != -0.5 {
		t.Errorf("TruePeak = %v, want -0.5", res.TruePeak)
	}
}

func TestParseEbur128_NegativeInf(t *testing.T) {
	// Silent audio: ffmpeg reports -inf.
	output := `
  Integrated loudness:
    I:         -inf LUFS
    Threshold:  -70.0 LUFS

  True peak:
    Peak:       -inf dBFS
`
	res, err := parseEbur128(output)
	if err != nil {
		t.Fatalf("parseEbur128: %v", err)
	}
	if !math.IsInf(res.LUFS, -1) {
		t.Errorf("LUFS = %v, want -Inf", res.LUFS)
	}
	if !math.IsInf(res.TruePeak, -1) {
		t.Errorf("TruePeak = %v, want -Inf", res.TruePeak)
	}
}

func TestParseEbur128_NoTruePeak(t *testing.T) {
	// ebur128 without peak=true: no "Peak:" line.
	output := `
  Integrated loudness:
    I:         -16.0 LUFS
    Threshold:  -26.0 LUFS
`
	res, err := parseEbur128(output)
	if err != nil {
		t.Fatalf("parseEbur128: %v", err)
	}
	if res.LUFS != -16.0 {
		t.Errorf("LUFS = %v, want -16.0", res.LUFS)
	}
	// TruePeak defaults to -Inf when the line is absent.
	if !math.IsInf(res.TruePeak, -1) {
		t.Errorf("TruePeak = %v, want -Inf (absent)", res.TruePeak)
	}
}

func TestParseEbur128_NoMatch(t *testing.T) {
	_, err := parseEbur128("ffmpeg: No such file or directory")
	if err == nil {
		t.Error("expected error when output has no LUFS line")
	}
}

func TestParseEbur128_EmptyOutput(t *testing.T) {
	_, err := parseEbur128("")
	if err == nil {
		t.Error("expected error for empty output")
	}
}
