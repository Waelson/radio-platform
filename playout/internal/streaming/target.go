package streaming

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// writeChanCap is the number of PCM frame slices buffered per target.
	// At 48 kHz stereo, one 4096-sample slice ≈ 85 ms. 64 slices ≈ 5.4 s.
	writeChanCap = 64

	// icecastScheme is the FFmpeg URL prefix for both Icecast and SHOUTcast.
	icecastScheme = "icecast"
)

// Target manages a single FFmpeg subprocess that encodes and streams PCM audio
// to an Icecast or SHOUTcast server.
//
// The audio hot path (Write) never blocks — frames are discarded if the
// internal buffer is full, ensuring the main audio loop is unaffected.
type Target struct {
	cfg TargetConfig
	log *slog.Logger

	mu          sync.Mutex
	state       TargetState
	connectedAt *time.Time
	lastError   string
	retryCount  int
	nextRetryAt *time.Time
	listeners   int

	// FFmpeg subprocess
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	writeCh chan []float32

	bytesSent atomic.Int64

	// stopCh is closed by Disconnect to cancel reconnection loops.
	stopCh chan struct{}
	// doneCh is closed when the watchProcess goroutine exits.
	doneCh chan struct{}

	// onDisconnect is called when the subprocess exits unexpectedly.
	// It is set by the manager to trigger reconnection.
	onDisconnect func(id string, reason string)
}

// NewTarget creates a Target from config. log may be nil.
func NewTarget(cfg TargetConfig, log *slog.Logger) *Target {
	if log == nil {
		log = slog.Default()
	}
	return &Target{
		cfg:    cfg,
		log:    log,
		state:  StateIdle,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Connect starts the FFmpeg subprocess and begins streaming.
// It is safe to call Connect again after Disconnect.
func (t *Target) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == StateConnected || t.state == StateConnecting {
		return fmt.Errorf("streaming target %s: already %s", t.cfg.ID, t.state)
	}

	// Validate format string and codec availability before starting FFmpeg.
	if err := ValidateFormat(t.cfg.Format); err != nil {
		return err
	}
	if err := CheckCodecAvailable(t.cfg.Format); err != nil {
		return err
	}

	t.setState(StateConnecting)
	t.stopCh = make(chan struct{})
	t.doneCh = make(chan struct{})

	args := t.buildFFmpegArgs()
	t.log.Info("streaming: starting ffmpeg",
		"target", t.cfg.ID,
		"host", t.cfg.Host,
		"format", t.cfg.Format,
		"bitrate", t.cfg.BitrateKbps,
	)

	cmd := exec.Command(ffmpegBin(), args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.setState(StateError)
		t.lastError = err.Error()
		return fmt.Errorf("streaming target %s: stdin pipe: %w", t.cfg.ID, err)
	}

	// Capture stderr into a buffer. When cmd.Stderr is set to an io.Writer,
	// cmd.Wait() guarantees all stderr output is written before returning,
	// so watchProcess can read the buffer safely after Wait completes.
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		t.setState(StateError)
		t.lastError = err.Error()
		return fmt.Errorf("streaming target %s: start ffmpeg: %w", t.cfg.ID, err)
	}

	t.cmd = cmd
	t.stdin = stdin
	t.writeCh = make(chan []float32, writeChanCap)
	now := time.Now()
	t.connectedAt = &now
	t.bytesSent.Store(0)
	t.setState(StateConnected)

	go t.writeLoop()
	go t.watchProcess(ctx, &stderrBuf)

	return nil
}

// Disconnect stops the FFmpeg subprocess and cancels any pending reconnection.
func (t *Target) Disconnect() {
	t.mu.Lock()
	select {
	case <-t.stopCh:
		// Already stopped.
		t.mu.Unlock()
		return
	default:
		close(t.stopCh)
	}
	t.setState(StateDisconnected)
	t.connectedAt = nil
	cmd := t.cmd
	stdin := t.stdin
	doneCh := t.doneCh
	started := cmd != nil
	t.mu.Unlock()

	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	// Wait for watchProcess to exit only if a subprocess was started.
	if started {
		<-doneCh
	}
}

// Write sends PCM frames to the FFmpeg subprocess. It never blocks.
// Frames are silently dropped when the internal buffer is full.
func (t *Target) Write(frames []float32) {
	t.mu.Lock()
	ch := t.writeCh
	st := t.state
	t.mu.Unlock()

	if ch == nil || st != StateConnected {
		return
	}

	// Copy the slice so the caller can reuse their buffer immediately.
	cp := make([]float32, len(frames))
	copy(cp, frames)

	select {
	case ch <- cp:
	default: // buffer full — discard to protect the audio loop
	}
}

// Status returns a snapshot of the current status.
func (t *Target) Status() TargetStatus {
	t.mu.Lock()
	defer t.mu.Unlock()

	s := TargetStatus{
		ID:          t.cfg.ID,
		State:       t.state,
		LastError:   t.lastError,
		RetryCount:  t.retryCount,
		ConnectedAt: t.connectedAt,
		NextRetryAt: t.nextRetryAt,
		BytesSent:   t.bytesSent.Load(),
		Listeners:   t.listeners,
	}
	if t.connectedAt != nil {
		s.UptimeMS = time.Since(*t.connectedAt).Milliseconds()
	}
	return s
}

// ID returns the target identifier.
func (t *Target) ID() string { return t.cfg.ID }

// IsConnected reports whether the target is actively streaming.
func (t *Target) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state == StateConnected
}

// SetOnDisconnect registers the callback invoked when the subprocess exits
// unexpectedly (not via Disconnect). Used by the manager for reconnection.
func (t *Target) SetOnDisconnect(fn func(id string, reason string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDisconnect = fn
}

// ── internal ──────────────────────────────────────────────────────────────────

// writeLoop drains writeCh and encodes PCM as little-endian bytes to FFmpeg stdin.
func (t *Target) writeLoop() {
	for frames := range t.writeCh {
		if err := t.writeFrames(frames); err != nil {
			// If stdin is closed, exit quietly.
			return
		}
	}
}

func (t *Target) writeFrames(frames []float32) error {
	buf := make([]byte, len(frames)*4)
	for i, f := range frames {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	n, err := t.stdin.Write(buf)
	t.bytesSent.Add(int64(n))
	return err
}

// watchProcess waits for the FFmpeg process to exit and notifies the manager.
// stderrBuf is fully populated by cmd.Wait() (set via cmd.Stderr in Connect).
func (t *Target) watchProcess(ctx context.Context, stderrBuf *bytes.Buffer) {
	defer close(t.doneCh)

	cmd := t.cmd
	if cmd == nil {
		return
	}
	err := cmd.Wait() // also drains stderrBuf (set as cmd.Stderr)

	// Check whether Disconnect was called intentionally.
	select {
	case <-t.stopCh:
		// Intentional — do not trigger reconnection.
		return
	default:
	}

	// Build a human-readable reason that includes the FFmpeg error output
	// so the UI can show exactly why the connection dropped.
	stderrStr := strings.TrimSpace(stderrBuf.String())
	if stderrStr != "" {
		t.log.Warn("streaming: ffmpeg stderr", "target", t.cfg.ID, "output", stderrStr)
	}

	reason := "connection_lost"
	if err != nil {
		reason = fmt.Sprintf("ffmpeg exited: %v", err)
		// Append the last meaningful stderr line for quick diagnosis.
		if stderrStr != "" {
			lastLine := stderrStr
			if idx := strings.LastIndex(stderrStr, "\n"); idx >= 0 {
				if l := strings.TrimSpace(stderrStr[idx+1:]); l != "" {
					lastLine = l
				}
			}
			reason += " — " + lastLine
		}
	}

	t.mu.Lock()
	t.setState(StateDisconnected)
	t.lastError = reason
	t.connectedAt = nil
	cb := t.onDisconnect
	// Close writeCh so writeLoop exits.
	if t.writeCh != nil {
		close(t.writeCh)
		t.writeCh = nil
	}
	t.mu.Unlock()

	t.log.Warn("streaming: ffmpeg process exited",
		"target", t.cfg.ID, "reason", reason)

	if cb != nil {
		cb(t.cfg.ID, reason)
	}
}

// setState updates the target state. Must be called with t.mu held.
func (t *Target) setState(s TargetState) {
	t.state = s
}

// ── FFmpeg args ───────────────────────────────────────────────────────────────

func (t *Target) buildFFmpegArgs() []string {
	sr := t.cfg.SampleRate
	if sr == 0 {
		sr = 44100
	}
	ch := t.cfg.Channels
	if ch == 0 {
		ch = 2
	}
	bitrate := t.cfg.BitrateKbps
	if bitrate == 0 {
		bitrate = 128
	}

	args := []string{
		"-hide_banner", "-loglevel", "warning",
		// Input: raw PCM float32 LE stereo 48 kHz from stdin.
		"-f", "f32le", "-ar", "48000", "-ac", "2", "-i", "pipe:0",
		// Encoder.
		"-c:a", t.encoder(),
		"-b:a", strconv.Itoa(bitrate) + "k",
		"-ar", strconv.Itoa(sr),
	}

	// SHOUTcast v1 uses a legacy HTTP-like handshake; FFmpeg requires this flag.
	if t.cfg.Type == "shoutcast_v1" {
		args = append(args, "-legacy_icecast", "1")
	}

	// Icecast station metadata.
	if t.cfg.StationName != "" {
		args = append(args, "-ice_name", t.cfg.StationName)
	}
	if t.cfg.StationGenre != "" {
		args = append(args, "-ice_genre", t.cfg.StationGenre)
	}
	if t.cfg.StationDescription != "" {
		args = append(args, "-ice_description", t.cfg.StationDescription)
	}
	if t.cfg.StationURL != "" {
		args = append(args, "-ice_url", t.cfg.StationURL)
	}

	args = append(args,
		"-content_type", t.contentType(),
		"-f", t.ffmpegFormat(),
		t.buildURL(),
	)
	return args
}

func (t *Target) encoder() string {
	switch t.cfg.Format {
	case "ogg_vorbis":
		return "libvorbis"
	case "ogg_opus":
		return "libopus"
	case "aac":
		return "aac"
	default: // "mp3"
		return "libmp3lame"
	}
}

func (t *Target) contentType() string {
	switch t.cfg.Format {
	case "ogg_vorbis", "ogg_opus":
		return "application/ogg"
	case "aac":
		return "audio/aac"
	default:
		return "audio/mpeg"
	}
}

func (t *Target) ffmpegFormat() string {
	switch t.cfg.Format {
	case "ogg_vorbis", "ogg_opus":
		return "ogg"
	case "aac":
		return "adts"
	default:
		return "mp3"
	}
}

// buildURL constructs the icecast:// URL understood by FFmpeg for both
// Icecast and SHOUTcast targets.
func (t *Target) buildURL() string {
	mount := t.cfg.Mount
	switch t.cfg.Type {
	case "shoutcast_v1":
		// SHOUTcast v1 servers are typically single-stream and expect "/" as
		// the mount point in the ICY SOURCE handshake. Fall back to "/" when
		// the operator did not configure an explicit mount.
		if mount == "" {
			mount = "/"
		}
		// Password in the user field, no "source:" prefix (ICY legacy auth).
		return fmt.Sprintf("icecast://:%s@%s:%d%s",
			t.cfg.Password, t.cfg.Host, t.cfg.Port, mount)
	default: // "icecast", "shoutcast_v2"
		if mount == "" {
			mount = "/stream"
		}
		return fmt.Sprintf("icecast://source:%s@%s:%d%s",
			t.cfg.Password, t.cfg.Host, t.cfg.Port, mount)
	}
}

func ffmpegBin() string { return "ffmpeg" }

// ExportBuildFFmpegArgs is exported for testing only.
// It returns the FFmpeg argument list that would be used for the given config.
func ExportBuildFFmpegArgs(cfg TargetConfig) []string {
	t := NewTarget(cfg, nil)
	return t.buildFFmpegArgs()
}
