package decoder_test

import (
	"context"
	"fmt"
	"io"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/audio"
	"github.com/Waelson/radio-playout-engine/internal/audio/decoder"
)

// ffmpegAvailable returns true if ffmpeg is found on PATH.
func ffmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// generateSilenceWAV creates a minimal silent WAV file using ffmpeg lavfi.
func generateSilenceWAV(t *testing.T, durationMS int) string {
	t.Helper()
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not available")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "silence.wav")
	dur := fmt.Sprintf("%.3f", float64(durationMS)/1000.0)
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo",
		"-t", dur,
		"-ar", "48000", "-ac", "2",
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate WAV: %v\n%s", err, out)
	}
	return path
}

func TestFFmpegDecoder_Validate(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not available")
	}
	d := decoder.NewFFmpegDecoder(nil)
	if err := d.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestFFmpegDecoder_Validate_NotFound(t *testing.T) {
	d := &decoder.FFmpegDecoder{FFmpegPath: "/nonexistent/ffmpeg"}
	if err := d.Validate(); err == nil {
		t.Error("expected error for nonexistent ffmpeg, got nil")
	}
}

func TestFFmpegDecoder_Open_EmptyPath(t *testing.T) {
	d := decoder.NewFFmpegDecoder(nil)
	_, err := d.Open(context.Background(), decoder.Source{Path: ""})
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

func TestFFmpegDecoder_Open_NonExistentFile(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not available")
	}
	d := decoder.NewFFmpegDecoder(nil)
	stream, err := d.Open(context.Background(), decoder.Source{Path: "/nonexistent/file.mp3"})
	if err != nil {
		// Some platforms may fail at Open; that is acceptable.
		return
	}
	defer stream.Close()
	// On most platforms the subprocess starts but fails on the first read.
	buf := make([]float32, 1024)
	_, readErr := stream.ReadFrames(context.Background(), buf)
	if readErr == nil {
		t.Error("expected read error for nonexistent file, got nil")
	}
}

func TestFFmpegDecoder_ReadsFrames(t *testing.T) {
	path := generateSilenceWAV(t, 1000) // 1 second silence

	d := decoder.NewFFmpegDecoder(nil)
	stream, err := d.Open(context.Background(), decoder.Source{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer stream.Close()

	// Verify format.
	format := stream.Format()
	if format.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", format.SampleRate)
	}
	if format.Channels != 2 {
		t.Errorf("Channels = %d, want 2", format.Channels)
	}

	// Read all frames.
	buf := make([]float32, 4096)
	totalFrames := int64(0)
	for {
		n, err := stream.ReadFrames(context.Background(), buf)
		totalFrames += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadFrames: %v", err)
		}
	}

	// 1 second at 48000 Hz. Allow ±5% for codec rounding.
	expectedFrames := int64(48000)
	tolerance := expectedFrames / 20
	if math.Abs(float64(totalFrames-expectedFrames)) > float64(tolerance) {
		t.Errorf("totalFrames = %d, expected ~%d (±%d)", totalFrames, expectedFrames, tolerance)
	}
}

func TestFFmpegDecoder_ReadsCorrectFormat(t *testing.T) {
	path := generateSilenceWAV(t, 500)

	d := decoder.NewFFmpegDecoder(nil)
	stream, err := d.Open(context.Background(), decoder.Source{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer stream.Close()

	want := audio.DefaultFormat
	got := stream.Format()
	if got.SampleRate != want.SampleRate || got.Channels != want.Channels {
		t.Errorf("Format = %+v, want %+v", got, want)
	}
}

func TestFFmpegDecoder_Close_DoesNotPanic(t *testing.T) {
	path := generateSilenceWAV(t, 200)

	d := decoder.NewFFmpegDecoder(nil)
	stream, err := d.Open(context.Background(), decoder.Source{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Close without reading.
	if err := stream.Close(); err != nil {
		t.Logf("Close: %v (non-fatal)", err)
	}
}

func TestFFmpegDecoder_ContextCancel(t *testing.T) {
	path := generateSilenceWAV(t, 5000) // 5 seconds — we cancel early

	d := decoder.NewFFmpegDecoder(nil)
	ctx, cancel := context.WithCancel(context.Background())

	stream, err := d.Open(ctx, decoder.Source{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer stream.Close()

	buf := make([]float32, 4096)
	_, _ = stream.ReadFrames(ctx, buf)

	cancel()
	// After cancel the process is killed; further reads may return errors.
	// We just verify there is no panic.
	_, _ = stream.ReadFrames(ctx, buf)
}

func TestFFmpegDecoder_CueIn(t *testing.T) {
	path := generateSilenceWAV(t, 2000) // 2 seconds

	d := decoder.NewFFmpegDecoder(nil)
	// Start at 1000ms → should yield ~1 second of audio.
	stream, err := d.Open(context.Background(), decoder.Source{
		Path:    path,
		CueInMS: 1000,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer stream.Close()

	total := countFrames(t, stream)

	expected := int64(48000)
	tol := expected / 10
	if math.Abs(float64(total-expected)) > float64(tol) {
		t.Errorf("after cue_in=1000ms: total frames = %d, expected ~%d", total, expected)
	}
}

func TestFFmpegDecoder_CueInCueOut(t *testing.T) {
	path := generateSilenceWAV(t, 5000) // 5 seconds

	d := decoder.NewFFmpegDecoder(nil)
	// Play from 1000ms to 2000ms → ~1 second.
	stream, err := d.Open(context.Background(), decoder.Source{
		Path:     path,
		CueInMS:  1000,
		CueOutMS: 2000,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer stream.Close()

	total := countFrames(t, stream)

	expected := int64(48000)
	tol := expected / 10
	if math.Abs(float64(total-expected)) > float64(tol) {
		t.Errorf("cue_in=1000 cue_out=2000: frames = %d, expected ~%d", total, expected)
	}
}

// countFrames drains a PCMStream and returns the total number of frames read.
func countFrames(t *testing.T, stream decoder.PCMStream) int64 {
	t.Helper()
	buf := make([]float32, 4096)
	total := int64(0)
	for {
		n, err := stream.ReadFrames(context.Background(), buf)
		total += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadFrames: %v", err)
		}
	}
	return total
}

// Ensure strconv is used (used in formatDuration inside the package).
var _ = strconv.Itoa
