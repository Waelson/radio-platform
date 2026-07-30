package linein_test

import (
	"context"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/linein"
)

func TestFFmpegCapture_ValidateNotFound(t *testing.T) {
	cap := linein.NewFFmpegCapture(nil)
	cap.FFmpegPath = "/nonexistent/ffmpeg-binary-does-not-exist"

	if err := cap.Validate(); err == nil {
		t.Error("expected error when ffmpeg binary does not exist")
	}
}

func TestFFmpegCapture_OpenNotFound(t *testing.T) {
	cap := linein.NewFFmpegCapture(nil)
	cap.FFmpegPath = "/nonexistent/ffmpeg-binary-does-not-exist"

	err := cap.Open(context.Background(), linein.LineInConfig{DeviceID: "default"})
	if err == nil {
		t.Error("expected error opening capture when ffmpeg not found")
	}
}

func TestFFmpegCapture_ReadBeforeOpen(t *testing.T) {
	cap := linein.NewFFmpegCapture(nil)

	buf := make([]float32, 64)
	_, err := cap.ReadFrames(context.Background(), buf)
	if err == nil {
		t.Error("expected error when ReadFrames called before Open")
	}
}

func TestFFmpegCapture_CloseBeforeOpen(t *testing.T) {
	cap := linein.NewFFmpegCapture(nil)
	// Should not panic or return error.
	if err := cap.Close(); err != nil {
		t.Logf("Close before Open returned (acceptable): %v", err)
	}
}
