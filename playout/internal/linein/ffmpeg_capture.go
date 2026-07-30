package linein

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os/exec"
)

// FFmpegCapture implements InputDevice by spawning an ffmpeg subprocess that
// reads from a hardware input device and emits PCM float32 LE 48 kHz stereo
// on stdout — the same PCM format used throughout the engine.
//
// The capture arguments are platform-specific; they are provided by
// buildCaptureArgs (defined in ffmpeg_args_<OS>.go).
type FFmpegCapture struct {
	// FFmpegPath overrides the ffmpeg binary name. Defaults to "ffmpeg".
	FFmpegPath string

	log *slog.Logger
	cmd *exec.Cmd
	out io.ReadCloser
	buf []byte
}

// NewFFmpegCapture creates a new FFmpegCapture. log may be nil.
func NewFFmpegCapture(log *slog.Logger) *FFmpegCapture {
	return &FFmpegCapture{
		FFmpegPath: "ffmpeg",
		log:        log,
	}
}

// Validate checks that ffmpeg is available on PATH (or at FFmpegPath).
// Call this during engine startup.
func (c *FFmpegCapture) Validate() error {
	bin := c.binary()
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("ffmpeg capture: %q not found on PATH: %w", bin, err)
	}
	return nil
}

// Open starts the ffmpeg subprocess for the given device.
// The subprocess is killed when Close is called or ctx is cancelled.
func (c *FFmpegCapture) Open(ctx context.Context, cfg LineInConfig) error {
	bin := c.binary()
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("ffmpeg capture: %q not found on PATH: %w", bin, err)
	}

	args := buildCaptureArgs(cfg.DeviceID)
	// Append common output args: PCM float32 LE stereo 48 kHz → stdout.
	args = append(args,
		"-f", "f32le",
		"-acodec", "pcm_f32le",
		"-ac", "2",
		"-ar", "48000",
		"pipe:1",
	)

	c.cmd = exec.CommandContext(ctx, bin, args...)

	stderr, _ := c.cmd.StderrPipe()
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg capture: stdout pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg capture: start: %w", err)
	}

	// Drain stderr in background to avoid pipe deadlock.
	go func() {
		raw, _ := io.ReadAll(stderr)
		if len(raw) > 0 && c.log != nil {
			c.log.Error("ffmpeg capture stderr", "device", cfg.DeviceID, "output", string(raw))
		}
	}()

	c.out = stdout
	return nil
}

// ReadFrames reads interleaved float32 PCM frames into dst.
// Returns io.EOF when the subprocess closes stdout.
func (c *FFmpegCapture) ReadFrames(_ context.Context, dst []float32) (int, error) {
	if c.out == nil {
		return 0, fmt.Errorf("ffmpeg capture: not opened")
	}
	if len(dst) == 0 {
		return 0, nil
	}

	const channels = 2
	byteCount := len(dst) * 4
	if cap(c.buf) >= byteCount {
		c.buf = c.buf[:byteCount]
	} else {
		c.buf = make([]byte, byteCount)
	}

	n, err := io.ReadFull(c.out, c.buf)
	samplesRead := n / 4
	for i := 0; i < samplesRead; i++ {
		bits := binary.LittleEndian.Uint32(c.buf[i*4:])
		dst[i] = math.Float32frombits(bits)
	}
	framesRead := samplesRead / channels

	if err == io.ErrUnexpectedEOF || err == io.EOF {
		return framesRead, io.EOF
	}
	if err != nil {
		return framesRead, fmt.Errorf("ffmpeg capture: read: %w", err)
	}
	return framesRead, nil
}

// Close kills the ffmpeg subprocess and releases all resources.
func (c *FFmpegCapture) Close() error {
	var firstErr error
	if c.out != nil {
		if err := c.out.Close(); err != nil {
			firstErr = err
		}
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	if c.cmd != nil {
		_ = c.cmd.Wait()
	}
	return firstErr
}

func (c *FFmpegCapture) binary() string {
	if c.FFmpegPath != "" {
		return c.FFmpegPath
	}
	return "ffmpeg"
}
