package linein

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// Handler bridges the command bus and the LineInManager.
// It handles CmdLineInStart and CmdLineInStop, manages the session lifecycle,
// updates the state machine, and forwards LineInManager events to the Event Bus.
//
// When Record is true in the start payload, Handler opens a FileOutput in
// parallel with the main output. After the session ends it closes the file
// (patching the WAV header) and optionally converts it to MP3 via FFmpeg.
// Completed recordings are kept in memory and exposed via ListRecordings.
type Handler struct {
	out        output.OutputDevice
	stateMgr   *state.Manager
	evtBus     *events.Bus
	log        *slog.Logger
	recordsDir string // directory for auto-generated recording filenames; "" → "."

	mu  sync.Mutex
	mgr *LineInManager

	recMu      sync.RWMutex
	recordings []RecordingEntry
}

// NewHandler creates a Handler. out is the main audio output device that
// line-in audio will be routed to during a session.
func NewHandler(out output.OutputDevice, stateMgr *state.Manager, evtBus *events.Bus, log *slog.Logger) *Handler {
	return &Handler{
		out:      out,
		stateMgr: stateMgr,
		evtBus:   evtBus,
		log:      log,
	}
}

// SetRecordingsDir configures the directory where auto-generated recording
// files are written. Defaults to the current working directory if not set.
func (h *Handler) SetRecordingsDir(dir string) {
	h.recordsDir = dir
}

// ListRecordings returns a snapshot of all completed recording entries.
func (h *Handler) ListRecordings() []RecordingEntry {
	h.recMu.RLock()
	defer h.recMu.RUnlock()
	cp := make([]RecordingEntry, len(h.recordings))
	copy(cp, h.recordings)
	return cp
}

// HandleStart handles CmdLineInStart. It creates an FFmpegCapture and
// LineInManager, starts the session, transitions the engine to StateLineIn,
// and spawns a goroutine that forwards session events to the Event Bus.
func (h *Handler) HandleStart(ctx context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.LineInStartPayload)
	if !ok {
		return fmt.Errorf("linein handler: unexpected payload type %T", cmd.Payload)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.mgr != nil {
		return &commands.RejectedError{Reason: "line-in session already active"}
	}

	capture := NewFFmpegCapture(h.log)
	mgr := NewLineInManager(capture, h.out, h.log)

	cfg := LineInConfig{
		DeviceID:             p.DeviceID,
		Label:                p.Label,
		DurationMS:           p.DurationMS,
		OnSilence:            p.OnSilence,
		SilenceThresholdDBFS: p.SilenceThresholdDBFS,
		SilenceThresholdMS:   p.SilenceThresholdMS,
		Record:               p.Record,
		RecordPath:           p.RecordPath,
		RecordFormat:         p.RecordFormat,
		TriggeredBy:          p.TriggeredBy,
	}

	// Set up simultaneous recording if requested.
	var recorder *output.FileOutput
	if cfg.Record {
		path := cfg.RecordPath
		if path == "" {
			path = h.recordingPath(cfg.Label, time.Now().UTC(), cfg.RecordFormat)
		}
		fo := &output.FileOutput{Path: path}
		ocfg := output.OutputConfig{SampleRate: 48000, Channels: 2}
		if err := fo.Open(ctx, ocfg); err != nil {
			// Non-fatal: log and proceed without recording.
			h.logf("linein: recording unavailable, session continues without recording",
				"path", path, "err", err)
		} else {
			recorder = fo
			mgr.SetRecorder(fo)
		}
	}

	ch, err := mgr.Start(ctx, cfg)
	if err != nil {
		if recorder != nil {
			_ = recorder.Close()
		}
		return fmt.Errorf("linein handler: start: %w", err)
	}

	h.mgr = mgr

	startedAt := time.Now().UTC()
	h.stateMgr.SetLineIn(state.LineInStatus{
		DeviceID:    p.DeviceID,
		Label:       p.Label,
		StartedAt:   startedAt,
		DurationMS:  p.DurationMS,
		TriggeredBy: p.TriggeredBy,
	})

	go h.forward(cfg, ch, recorder, startedAt)
	return nil
}

// HandleStop handles CmdLineInStop. It signals the active session to stop.
// The state machine is reset to Idle once the session goroutine drains.
func (h *Handler) HandleStop(_ context.Context, _ commands.Command) error {
	h.mu.Lock()
	mgr := h.mgr
	h.mu.Unlock()

	if mgr == nil {
		return &commands.RejectedError{Reason: "no active line-in session"}
	}
	mgr.Stop()
	return nil
}

// forward reads from the LineInManager event channel and publishes
// corresponding events to the Event Bus. It always cleans up state when the
// channel closes, regardless of whether the session ended cleanly or with an error.
// If recorder is non-nil, it closes it after the channel drains and (if needed)
// converts the WAV to MP3, then records the entry in the in-memory history.
func (h *Handler) forward(cfg LineInConfig, ch <-chan Event, recorder *output.FileOutput, startedAt time.Time) {
	cleanStop := false
	stateCleared := false
	var elapsedMS int64

	// clearState resets the engine state back to IDLE and releases the mgr
	// reference. It is idempotent so safe to call multiple times.
	clearState := func() {
		if stateCleared {
			return
		}
		stateCleared = true
		h.stateMgr.ClearLineIn()
		h.stateMgr.SetState(state.StateIdle)
		h.mu.Lock()
		h.mgr = nil
		h.mu.Unlock()
	}

	for e := range ch {
		switch e.Type {
		case EventStarted:
			h.evtBus.Publish(events.New(events.EvtLineInStarted, events.LineInStartedPayload{
				DeviceID:    cfg.DeviceID,
				Label:       cfg.Label,
				DurationMS:  cfg.DurationMS,
				TriggeredBy: "engine",
			}))

		case EventStopped:
			cleanStop = true
			elapsedMS = e.ElapsedMS
			reason := "manual"
			if cfg.DurationMS > 0 && e.ElapsedMS >= cfg.DurationMS {
				reason = "duration_expired"
			}
			// Clear state BEFORE publishing so that observers querying the
			// snapshot upon receiving EvtLineInStopped see StateIdle.
			clearState()
			h.evtBus.Publish(events.New(events.EvtLineInStopped, events.LineInStoppedPayload{
				Label:     cfg.Label,
				ElapsedMS: e.ElapsedMS,
				Reason:    reason,
			}))

		case EventSilenceStop:
			h.evtBus.Publish(events.New(events.EvtLineInError, events.LineInErrorPayload{
				Label:             cfg.Label,
				Error:             "silence_detected",
				SilenceDurationMS: e.SilenceDurationMS,
			}))

		case EventSilenceAlert:
			h.evtBus.Publish(events.New(events.EvtLineInError, events.LineInErrorPayload{
				Label:             cfg.Label,
				Error:             "silence_detected",
				SilenceDurationMS: e.SilenceDurationMS,
			}))

		case EventError:
			errMsg := "unknown_error"
			if e.Err != nil {
				errMsg = e.Err.Error()
			}
			h.evtBus.Publish(events.New(events.EvtLineInError, events.LineInErrorPayload{
				Label: cfg.Label,
				Error: errMsg,
			}))

		case EventLevelUpdate:
			h.evtBus.Publish(events.New(events.EvtLineInLevel, events.LineInLevelPayload{
				RMSDBFS: e.RMSDBFS,
			}))
		}
	}

	// Channel closed — ensure state is reset for sessions that ended via error.
	clearState()

	// Publish EvtLineInStopped for sessions that terminated without a clean stop
	// (error, silence_stop) so observers always see a matching Started/Stopped pair.
	if !cleanStop {
		elapsedMS = time.Since(startedAt).Milliseconds()
		h.evtBus.Publish(events.New(events.EvtLineInStopped, events.LineInStoppedPayload{
			Label:     cfg.Label,
			ElapsedMS: elapsedMS,
			Reason:    "error",
		}))
	}

	// ── Recording finalisation ────────────────────────────────────────────────
	if recorder != nil {
		stoppedAt := time.Now().UTC()

		// Close the FileOutput — this patches the WAV header with final sizes.
		if err := recorder.Close(); err != nil {
			h.logf("linein: failed to close recording file", "path", recorder.Path, "err", err)
		}

		finalPath := recorder.Path
		format := cfg.RecordFormat
		if format == "" {
			format = "wav"
		}

		// Post-stop MP3 conversion.
		if format == "mp3" {
			mp3Path := strings.TrimSuffix(recorder.Path, ".wav") + ".mp3"
			if err := h.convertWAVtoMP3(recorder.Path, mp3Path); err != nil {
				h.logf("linein: wav→mp3 conversion failed, keeping WAV", "err", err)
				format = "wav" // downgrade to wav since conversion failed
			} else {
				_ = os.Remove(recorder.Path) // remove the temp WAV
				finalPath = mp3Path
			}
		}

		var sizeBytes int64
		if fi, err := os.Stat(finalPath); err == nil {
			sizeBytes = fi.Size()
		}

		triggeredBy := cfg.TriggeredBy
		if triggeredBy == "" {
			triggeredBy = "manual"
		}

		entry := RecordingEntry{
			Label:       cfg.Label,
			StartedAt:   startedAt,
			StoppedAt:   stoppedAt,
			DurationMS:  elapsedMS,
			Path:        finalPath,
			Format:      format,
			SizeBytes:   sizeBytes,
			TriggeredBy: triggeredBy,
		}
		h.recMu.Lock()
		h.recordings = append(h.recordings, entry)
		h.recMu.Unlock()

		h.logf("line-in recording saved", "path", finalPath, "format", format,
			"size_bytes", sizeBytes, "duration_ms", elapsedMS)
	}

	h.log.Info("line-in session ended", "label", cfg.Label)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// recordingPath generates an automatic filename for a recording.
// Format: linein_<YYYY-MM-DD_HHhMM>_<label-slug>.wav  (always .wav — MP3
// conversion happens post-stop and renames the file to .mp3 if needed).
func (h *Handler) recordingPath(label string, t time.Time, _ string) string {
	dir := h.recordsDir
	if dir == "" {
		dir = "."
	}
	slug := slugify(label)
	if slug == "" {
		slug = "linein"
	}
	filename := fmt.Sprintf("linein_%s_%s.wav", t.Format("2006-01-02_15h04"), slug)
	return filepath.Join(dir, filename)
}

// slugify converts a label to a safe lowercase ASCII slug for use in filenames.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '_':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// convertWAVtoMP3 runs FFmpeg to convert src (WAV) to dst (MP3 @ 128 kbps).
func (h *Handler) convertWAVtoMP3(src, dst string) error {
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", src,
		"-codec:a", "libmp3lame", "-b:a", "128k",
		"-y", dst,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (h *Handler) logf(msg string, args ...any) {
	if h.log != nil {
		h.log.Info(msg, args...)
	}
}
