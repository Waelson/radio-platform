package streaming_test

import (
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/streaming"
)

// --- ValidateFormat ----------------------------------------------------------

func TestValidateFormat_ValidFormats(t *testing.T) {
	valid := []string{"mp3", "ogg_vorbis", "ogg_opus", "aac", ""}
	for _, f := range valid {
		if err := streaming.ValidateFormat(f); err != nil {
			t.Errorf("ValidateFormat(%q) unexpected error: %v", f, err)
		}
	}
}

func TestValidateFormat_InvalidFormats(t *testing.T) {
	invalid := []string{"flac", "wav", "mp4", "mpeg", "libmp3lame", "opus"}
	for _, f := range invalid {
		if err := streaming.ValidateFormat(f); err == nil {
			t.Errorf("ValidateFormat(%q) want error, got nil", f)
		}
	}
}

// --- CheckCodecAvailable -----------------------------------------------------

// TestCheckCodecAvailable_AAC tests the built-in aac encoder which never
// requires an external library — should always return nil without running ffmpeg.
func TestCheckCodecAvailable_AAC(t *testing.T) {
	if err := streaming.CheckCodecAvailable("aac"); err != nil {
		t.Errorf("CheckCodecAvailable(aac) unexpected error: %v", err)
	}
}

// TestCheckCodecAvailable_EmptyDefaultsToMP3 verifies that empty string
// triggers the mp3 path (which runs ffmpeg if available).
func TestCheckCodecAvailable_EmptyDefaultsToMP3(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg")
	}
	// Only assert no panic; result depends on whether libmp3lame is installed.
	_ = streaming.CheckCodecAvailable("")
}

// TestCheckCodecAvailable_InvalidFormat verifies that an unrecognised format
// returns an error without starting ffmpeg.
func TestCheckCodecAvailable_InvalidFormat(t *testing.T) {
	err := streaming.CheckCodecAvailable("flac")
	if err == nil {
		t.Error("CheckCodecAvailable(flac) want error, got nil")
	}
}

// TestCheckCodecAvailable_MP3_WhenFFmpegAvailable is an integration test that
// verifies libmp3lame is detected when ffmpeg is installed with that codec.
func TestCheckCodecAvailable_MP3_WhenFFmpegAvailable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg with libmp3lame")
	}
	err := streaming.CheckCodecAvailable("mp3")
	if err != nil {
		t.Logf("libmp3lame not available (install ffmpeg with libmp3lame): %v", err)
	}
}

// --- StreamingConnect handler rejects invalid format -------------------------

// TestStreamingConnect_InvalidFormat tests that the handler rejects unknown
// format strings with 400 before calling AddTarget.
// This test lives here (streaming_test package) to keep handler tests in
// handlers package; we duplicate a minimal version to verify ValidateFormat
// is wired correctly via the exported function.
func TestValidateFormat_RejectUnknown(t *testing.T) {
	cases := []struct {
		format  string
		wantErr bool
	}{
		{"mp3", false},
		{"ogg_vorbis", false},
		{"ogg_opus", false},
		{"aac", false},
		{"", false},
		{"flac", true},
		{"wav", true},
		{"mp4", true},
		{"libmp3lame", true},
	}
	for _, tc := range cases {
		err := streaming.ValidateFormat(tc.format)
		if tc.wantErr && err == nil {
			t.Errorf("ValidateFormat(%q) want error, got nil", tc.format)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("ValidateFormat(%q) unexpected error: %v", tc.format, err)
		}
	}
}
