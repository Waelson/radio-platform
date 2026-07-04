package decoder

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/Waelson/radio-playout-engine/internal/audio"
)

// FFmpegDecoder opens audio files by spawning an ffmpeg subprocess.
// It converts any format supported by ffmpeg to the engine's internal PCM
// (float32 LE, 48 kHz, stereo).
//
// Requires ffmpeg to be available on PATH (or at FFmpegPath).
type FFmpegDecoder struct {
	// FFmpegPath overrides the binary name/path. Defaults to "ffmpeg".
	FFmpegPath string

	// Format overrides the output PCM format. Defaults to audio.DefaultFormat.
	Format audio.AudioFormat

	log *slog.Logger
}

// NewFFmpegDecoder creates a decoder with optional logger. log may be nil.
func NewFFmpegDecoder(log *slog.Logger) *FFmpegDecoder {
	return &FFmpegDecoder{
		FFmpegPath: "ffmpeg",
		Format:     audio.DefaultFormat,
		log:        log,
	}
}

// Validate checks that ffmpeg is available on PATH (or at FFmpegPath).
// Call this during engine startup.
func (d *FFmpegDecoder) Validate() error {
	bin := d.binary()
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("ffmpeg decoder: %q not found on PATH: %w", bin, err)
	}
	return nil
}

// Open starts an ffmpeg subprocess that writes PCM float32 LE stereo 48 kHz
// to stdout, and returns a PCMStream backed by that pipe.
// The subprocess is killed when the returned PCMStream is Closed, or when ctx
// is cancelled.
func (d *FFmpegDecoder) Open(ctx context.Context, src Source) (PCMStream, error) {
	if src.Path == "" {
		return nil, fmt.Errorf("ffmpeg decoder: source path is empty")
	}

	path := filepath.Clean(src.Path)
	args := d.buildArgs(path, src.CueInMS, src.CueOutMS)

	cmd := exec.CommandContext(ctx, d.binary(), args...)

	// Capture stderr for debug logging.
	stderr, _ := cmd.StderrPipe()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg decoder: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg decoder: start: %w", err)
	}

	// Drain stderr in background to avoid pipe deadlock. Always log if non-empty.
	go func() {
		raw, _ := io.ReadAll(stderr)
		if len(raw) > 0 && d.log != nil {
			d.log.Error("ffmpeg stderr", "path", path, "output", string(raw))
		}
	}()

	return &ffmpegStream{
		cmd:    cmd,
		stdout: stdout,
		format: d.outputFormat(),
		buf:    make([]byte, 4096*d.outputFormat().BytesPerFrame()),
	}, nil
}

func (d *FFmpegDecoder) binary() string {
	if d.FFmpegPath != "" {
		return d.FFmpegPath
	}
	return "ffmpeg"
}

func (d *FFmpegDecoder) outputFormat() audio.AudioFormat {
	if d.Format.SampleRate == 0 {
		return audio.DefaultFormat
	}
	return d.Format
}

// buildArgs constructs the ffmpeg argument list.
func (d *FFmpegDecoder) buildArgs(path string, cueInMS, cueOutMS int64) []string {
	args := []string{"-hide_banner", "-loglevel", "error"}

	// Seek before -i for fast stream seek (ss before input = fast keyframe seek).
	if cueInMS > 0 {
		args = append(args, "-ss", formatDuration(cueInMS))
	}

	args = append(args, "-i", path)

	// Limit duration if cue-out is specified.
	if cueOutMS > 0 {
		durationMS := cueOutMS - cueInMS
		if durationMS > 0 {
			args = append(args, "-t", formatDuration(durationMS))
		}
	}

	format := d.outputFormat()
	args = append(args,
		"-f", "f32le",
		"-acodec", "pcm_f32le",
		"-ac", strconv.Itoa(format.Channels),
		"-ar", strconv.Itoa(format.SampleRate),
		"pipe:1",
	)
	return args
}

// formatDuration converts milliseconds to a seconds string with millisecond
// precision, e.g. 3500 → "3.500".
func formatDuration(ms int64) string {
	sec := ms / 1000
	frac := ms % 1000
	return strconv.FormatInt(sec, 10) + "." + fmt.Sprintf("%03d", frac)
}

// --- ffmpegStream ------------------------------------------------------------

type ffmpegStream struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	format audio.AudioFormat
	buf    []byte // byte buffer for batch reads
}

// ReadFrames reads audio frames from the ffmpeg stdout pipe.
// dst must be a multiple of format.Channels in length.
// Returns io.EOF when the stream is exhausted.
func (s *ffmpegStream) ReadFrames(_ context.Context, dst []float32) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}

	channels := s.format.Channels
	if channels == 0 {
		channels = 2
	}
	if len(dst)%channels != 0 {
		return 0, fmt.Errorf("ffmpeg stream: dst length %d not divisible by channels %d",
			len(dst), channels)
	}

	// We need exactly len(dst)*4 bytes.
	byteCount := len(dst) * 4
	rawBuf := s.growBuf(byteCount)

	n, err := io.ReadFull(s.stdout, rawBuf[:byteCount])
	samplesRead := n / 4

	for i := 0; i < samplesRead; i++ {
		bits := binary.LittleEndian.Uint32(rawBuf[i*4:])
		dst[i] = math.Float32frombits(bits)
	}

	framesRead := samplesRead / channels

	if err == io.ErrUnexpectedEOF || err == io.EOF {
		return framesRead, io.EOF
	}
	if err != nil {
		return framesRead, fmt.Errorf("ffmpeg stream: read: %w", err)
	}
	return framesRead, nil
}

// growBuf returns a slice of at least n bytes from the pre-allocated buffer,
// growing s.buf if necessary (avoids per-call allocation in the common case).
func (s *ffmpegStream) growBuf(n int) []byte {
	if cap(s.buf) >= n {
		return s.buf[:n]
	}
	s.buf = make([]byte, n)
	return s.buf
}

func (s *ffmpegStream) Close() error {
	err := s.stdout.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_ = s.cmd.Wait()
	return err
}

func (s *ffmpegStream) Format() audio.AudioFormat {
	return s.format
}
