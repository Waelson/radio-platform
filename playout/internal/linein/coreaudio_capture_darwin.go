//go:build coreaudio

package linein

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/linein/cainput"
)

const (
	// caCaptureChannels matches the engine's internal audio format.
	caCaptureChannels = 2
	// caCaptureRate matches the engine's internal audio format.
	caCaptureRate = 48000
	// caReadPollInterval is the sleep duration when the ring buffer is empty.
	// At 512 samples/callback ≈ 10 ms, polling at 1 ms means we check ~10×
	// per hardware callback — low CPU cost, sub-millisecond response time.
	caReadPollInterval = time.Millisecond
)

// CoreAudioCapture implements InputDevice using a native CoreAudio AudioQueue.
//
// The AudioQueue callback fires every ~10 ms (512 frames @ 48 kHz) and writes
// PCM float32 data directly into an SPSC ring buffer in C. ReadFrames polls
// that ring with a 1 ms sleep, delivering hardware-timed audio to the engine
// without any subprocess, pipe, or FFmpeg overhead.
type CoreAudioCapture struct {
	log     *slog.Logger
	session *cainput.Session
}

// NewCoreAudioCapture creates a CoreAudioCapture. log may be nil.
func NewCoreAudioCapture(log *slog.Logger) *CoreAudioCapture {
	return &CoreAudioCapture{log: log}
}

// Open opens the CoreAudio input queue for the device specified in cfg.
// DeviceID "" or "default" selects the system default input device.
// Numeric strings ("0", "1", ...) select by avfoundation device index.
func (c *CoreAudioCapture) Open(ctx context.Context, cfg LineInConfig) error {
	deviceID := cfg.DeviceID
	if deviceID == "" {
		deviceID = "default"
	}

	sess, err := cainput.Open(deviceID, caCaptureRate, caCaptureChannels)
	if err != nil {
		return err
	}

	if err := sess.Start(); err != nil {
		sess.Close()
		return err
	}

	c.session = sess
	if c.log != nil {
		c.log.Info("coreaudio capture: opened", "device", deviceID)
	}
	return nil
}

// ReadFrames reads interleaved PCM float32 frames into dst.
// It polls the C ring buffer at 1 ms intervals until at least one frame
// is available, then returns as many as fit in dst.
// Returns io.EOF when ctx is cancelled (session ending).
func (c *CoreAudioCapture) ReadFrames(ctx context.Context, dst []float32) (int, error) {
	if c.session == nil {
		return 0, io.EOF
	}

	want := len(dst)
	for {
		avail := c.session.Avail()
		if avail > 0 {
			if avail > want {
				avail = want
			}
			n := c.session.Read(dst[:avail])
			// Return frames (samples / channels).
			return n / caCaptureChannels, nil
		}

		// Ring is empty — wait one poll interval or bail on context cancel.
		select {
		case <-ctx.Done():
			return 0, io.EOF
		case <-time.After(caReadPollInterval):
		}
	}
}

// Close stops the AudioQueue and releases all C resources.
func (c *CoreAudioCapture) Close() error {
	if c.session == nil {
		return nil
	}
	c.session.Stop()
	c.session.Close()
	c.session = nil
	return nil
}
