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
	"sync/atomic"
	"time"
	"unicode"

	"github.com/oklog/ulid/v2"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/queue"
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
	out             output.OutputDevice
	stateMgr        *state.Manager
	evtBus          *events.Bus
	log             *slog.Logger
	recordsDir      string // directory for auto-generated recording filenames; "" → "."
	defaultDeviceID string // fallback capture device when payload DeviceID is empty
	queueMgr        *queue.Manager
	cmdBus          interface{ TrySend(commands.Command) bool }

	// playOnStop: when true, forward() sends CmdPlay after the session ends
	// and state transitions to IDLE. Set by SetPlayOnStop() before HandleStop.
	playOnStop atomic.Bool

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

// SetQueueManager wires the queue manager so that a virtual LINE_IN item
// appears as the current item in the playlist during an active session.
func (h *Handler) SetQueueManager(qm *queue.Manager) {
	h.queueMgr = qm
}

// SetCmdBus wires the command bus so that forward() can dispatch CmdPlay
// after a skip-triggered stop once the state has transitioned to IDLE.
func (h *Handler) SetCmdBus(bus interface{ TrySend(commands.Command) bool }) {
	h.cmdBus = bus
}

// SetPlayOnStop schedules a CmdPlay to be sent by forward() after the
// current session ends and state reaches IDLE. Call this before HandleStop
// when the operator presses Skip during a LINE_IN session.
func (h *Handler) SetPlayOnStop() {
	h.playOnStop.Store(true)
}

// SetDefaultDeviceID configures the fallback capture device used when a
// LineInStart command arrives without an explicit device_id.
func (h *Handler) SetDefaultDeviceID(id string) {
	h.defaultDeviceID = id
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

	capture := NewCapture(h.log)
	mgr := NewLineInManager(capture, h.out, h.log)
	mgr.SetStateManager(h.stateMgr)

	deviceID := p.DeviceID
	if deviceID == "" {
		deviceID = h.defaultDeviceID
	}

	cfg := LineInConfig{
		DeviceID:             deviceID,
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

	// The Mixer owns the OutputDevice and keeps it open across sessions — no
	// Open/Start/Close needed here. The Mixer's idempotent Open/Start ensure
	// the hardware is ready before the first Write, and the no-op Close/Stop
	// prevent a session end from silencing the device for other sessions.
	ch, err := mgr.Start(ctx, cfg)
	if err != nil {
		if recorder != nil {
			_ = recorder.Close()
		}
		return fmt.Errorf("linein handler: start: %w", err)
	}

	h.mgr = mgr

	// Register a virtual LINE_IN queue item as the current item so the session
	// appears in the playlist with the correct label and type.
	if h.queueMgr != nil {
		label := cfg.Label
		if label == "" {
			label = "Line-In"
		}
		virtualItem := &queue.QueueItem{
			QueueItemID: "qi_linein_" + ulid.Make().String(),
			Type:        queue.AssetTypeLiveInput,
			Title:       label,
			Status:      queue.ItemStatusPlaying,
		}
		h.queueMgr.SetCurrent(virtualItem)
	}

	prevState := h.stateMgr.Snapshot().State
	startedAt := time.Now().UTC()
	h.stateMgr.SetLineIn(state.LineInStatus{
		DeviceID:    p.DeviceID,
		Label:       p.Label,
		StartedAt:   startedAt,
		DurationMS:  p.DurationMS,
		TriggeredBy: p.TriggeredBy,
	})
	h.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: string(prevState),
		To:   string(state.StateLineIn),
		Mode: string(h.stateMgr.Snapshot().Mode),
	}))

	// When the scheduler triggered line-in via INTERRUPT or CROSSFADE it
	// first sent CmdStop, which returned the interrupted item (Music A) to the
	// front of the queue. When the session ends naturally (duration_ms), we
	// must discard that item so the queue advances past it instead of replaying it.
	skipFrontOnResume := p.TriggeredBy == "scheduler" &&
		(p.TriggerMode == "INTERRUPT" || p.TriggerMode == "CROSSFADE")

	go h.forward(cfg, ch, recorder, startedAt, skipFrontOnResume)
	return nil
}

// HandlePauseSession pauses an active line-in session without stopping it.
// The capture device keeps running; output is silenced until ResumeSession.
func (h *Handler) HandlePauseSession(_ context.Context, _ commands.Command) error {
	h.mu.Lock()
	mgr := h.mgr
	h.mu.Unlock()
	if mgr == nil {
		return &commands.RejectedError{Reason: "no active line-in session"}
	}
	mgr.Pause()
	h.evtBus.Publish(events.New(events.EvtLineInPaused, nil))
	return nil
}

// HandleResumeSession resumes a paused line-in session.
func (h *Handler) HandleResumeSession(_ context.Context, _ commands.Command) error {
	h.mu.Lock()
	mgr := h.mgr
	h.mu.Unlock()
	if mgr == nil {
		return &commands.RejectedError{Reason: "no active line-in session"}
	}
	mgr.Resume()
	h.evtBus.Publish(events.New(events.EvtLineInResumed, nil))
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
func (h *Handler) forward(cfg LineInConfig, ch <-chan Event, recorder *output.FileOutput, startedAt time.Time, skipFrontOnResume bool) {
	cleanStop := false
	stateCleared := false
	autoPlay := false // set when the session ends naturally (duration_ms) so queue resumes
	var elapsedMS int64

	// clearState resets the engine state back to IDLE and releases the mgr
	// reference. It is idempotent so safe to call multiple times.
	clearState := func() {
		if stateCleared {
			return
		}
		stateCleared = true
		// Transition the virtual LINE_IN item to PLAYED so it remains
		// visible in the queue list after the session ends.
		// It will be replaced when the next item starts (SetCurrent/PopAsCurrent).
		if h.queueMgr != nil {
			h.queueMgr.MarkCurrentPlayed()
		}
		// Drain the hardware ring buffer so buffered line-in audio stops immediately.
		type flusher interface{ FlushAudio() error }
		if f, ok := h.out.(flusher); ok {
			_ = f.FlushAudio()
		}
		h.stateMgr.ClearLineIn()
		h.stateMgr.SetState(state.StateIdle)
		h.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
			From: string(state.StateLineIn),
			To:   string(state.StateIdle),
			Mode: string(state.ModeAuto),
		}))
		h.mu.Lock()
		h.mgr = nil
		h.mu.Unlock()
	}

	// Progress loop: publish EvtProgressChanged every 250 ms when a duration
	// cap is configured so the frontend progress bar advances during line-in.
	var progressDone chan struct{}
	if cfg.DurationMS > 0 {
		progressDone = make(chan struct{})
		go func() {
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-progressDone:
					return
				case t := <-ticker.C:
					posMS := t.Sub(startedAt).Milliseconds()
					if posMS < 0 {
						posMS = 0
					}
					if posMS > cfg.DurationMS {
						posMS = cfg.DurationMS
					}
					remMS := cfg.DurationMS - posMS
					pct := float64(posMS) / float64(cfg.DurationMS) * 100
					h.evtBus.Publish(events.New(events.EvtProgressChanged, events.ProgressChangedPayload{
						PositionMS:  posMS,
						DurationMS:  cfg.DurationMS,
						Percent:     pct,
						RemainingMS: remMS,
					}))
				}
			}
		}()
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
				autoPlay = true // resume queue automatically after duration cap
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

	// Stop the progress ticker goroutine (if running).
	if progressDone != nil {
		close(progressDone)
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

	// Resume queue playback when the session ended via Skip or duration_ms expiry.
	// This is done after clearState() so the dispatcher sees StateIdle and
	// accepts CmdPlay — sending it earlier would race the state transition.
	// Manual Stop leaves the queue paused (operator explicitly stopped).
	if (h.playOnStop.Swap(false) || autoPlay) && h.cmdBus != nil && h.queueMgr != nil && h.queueMgr.Size() > 0 {
		// When the scheduler stopped active playback to start line-in
		// (INTERRUPT/CROSSFADE), the interrupted item was returned to the
		// front of the queue by CmdStop. Discard it so we advance past it
		// rather than replaying it after line-in ends.
		if autoPlay && skipFrontOnResume {
			h.queueMgr.Pop()
		}
		if h.queueMgr.Size() > 0 {
			reason := "duration_expired_linein"
			if !autoPlay {
				reason = "skip_linein"
			}
			h.cmdBus.TrySend(commands.New(commands.CmdPlay, commands.PlayPayload{Reason: reason}))
		}
	}
	// The Mixer owns the OutputDevice — no Stop/Close here.
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
