// Package playback orchestrates the audio playback loop.
// It pops items from the queue, opens the decoder, drives the read→write
// pipeline, and publishes progress/state events through the Event Bus.
//
// Dependency direction:
//
//	playback → queue, audio/decoder, audio/output, events, state, commands
//
// The API and HTTP packages must not be imported here.
package playback

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/audio"
	"github.com/Waelson/radio-playout-engine/internal/audio/decoder"
	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/health"
	"github.com/Waelson/radio-playout-engine/internal/horacerta"
	"github.com/Waelson/radio-playout-engine/internal/prefs"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// Config holds playback-specific configuration.
type Config struct {
	// DeviceID is the audio output device name. "default" uses the OS default.
	DeviceID string
	// BufferFrames is the PCM read-buffer size in frames. Default: 2048.
	BufferFrames int
	// ProgressIntervalMS controls how often ProgressChanged events fire. Default: 500.
	ProgressIntervalMS int
	// MaxConsecutiveFailures triggers an engine error after N consecutive item
	// failures. Default: 3.
	MaxConsecutiveFailures int
	// DefaultCrossfadeMS is the crossfade duration applied between eligible items.
	// 0 disables crossfade entirely.
	DefaultCrossfadeMS int
	// PanicBedPath is the audio file to loop when panic mode is entered without
	// an explicit bed in the command payload. Loaded from config.panic.bed_path.
	PanicBedPath string

	// Auto crossfade by energy analysis — optional, disabled by default.
	// When enabled, crossfade is triggered as soon as the RMS of the current
	// buffer drops below AutoCrossfadeEnergyThreshDBFS for AutoCrossfadeHoldFrames
	// consecutive read cycles, provided the position is within the detection
	// window. The time-based crossfade remains active as a fallback.
	AutoCrossfadeEnabled          bool
	AutoCrossfadeEnergyThreshDBFS float64 // dBFS, e.g. -18.0
	AutoCrossfadeMinBeforeEndMS   int     // minimum ms before end to start detecting
	AutoCrossfadeMaxBeforeEndMS   int     // maximum ms before end to start detecting
	AutoCrossfadeHoldFrames       int     // consecutive low-energy reads required
}

func (c *Config) setDefaults() {
	if c.BufferFrames <= 0 {
		c.BufferFrames = 2048
	}
	if c.ProgressIntervalMS <= 0 {
		c.ProgressIntervalMS = 500
	}
	if c.MaxConsecutiveFailures <= 0 {
		c.MaxConsecutiveFailures = 3
	}
	// DefaultCrossfadeMS stays 0 (disabled) if not set — no forced default.
	if c.AutoCrossfadeEnabled {
		if c.AutoCrossfadeEnergyThreshDBFS == 0 {
			c.AutoCrossfadeEnergyThreshDBFS = -18.0
		}
		if c.AutoCrossfadeMinBeforeEndMS == 0 {
			c.AutoCrossfadeMinBeforeEndMS = 2000
		}
		if c.AutoCrossfadeMaxBeforeEndMS == 0 {
			c.AutoCrossfadeMaxBeforeEndMS = 20000
		}
		if c.AutoCrossfadeHoldFrames == 0 {
			c.AutoCrossfadeHoldFrames = 8
		}
	}
}

// Manager owns the session loop and exposes command handlers that the
// Dispatcher calls. Only one play session runs at a time.
type Manager struct {
	evtBus    *events.Bus
	stateMgr  *state.Manager
	queueMgr  *queue.Manager
	dec       decoder.Decoder
	out       output.OutputDevice
	cfg       Config
	healthMon *health.Monitor // optional; nil = no health reporting
	log       *slog.Logger

	// Session management — protected by sessionMu.
	sessionMu   sync.Mutex
	sessionStop context.CancelFunc
	sessionDone chan struct{}
	sessionGen  int

	// Skip signal — cap-1 so sending never blocks.
	skipCh chan struct{}

	// Pause/resume — protected by pauseMu.
	pauseMu        sync.Mutex
	paused         bool
	resumeCh       chan struct{}
	pausedFromState state.PlayerState // state to restore on Resume

	// Current item and frame counter — used by the progress loop.
	currentMu      sync.RWMutex
	current        *queue.QueueItem
	currentBreakID string // BreakID of the item currently playing; "" if not in a break
	itemStartFrame int64  // framesTotal value when the current item started

	framesTotal atomic.Int64 // total frames written since session start

	consecutiveFailures int

	// Hora Certa resolver — resolves HORA_CERTA items to audio file paths.
	// nil when the feature is not configured.
	horaCerta *horacerta.Resolver

	// Panic bed — protected by panicMu.
	panicMu   sync.Mutex
	panicStop context.CancelFunc
	panicDone chan struct{}

	// Hot button overlay — a concurrent stream mixed into the main output.
	// duckGain is an atomic percentage (0–100) applied to the main stream.
	// 100 = full volume, 0 = muted.  Protected by hotMu for start/stop;
	// duckGain is read/written atomically in the hot path.
	hotMu    sync.Mutex
	hotStop  context.CancelFunc
	hotDone  chan struct{}
	duckGain atomic.Int32 // 0–100; default 100 (no ducking)

	// overlayMix holds decoded overlay samples that the main loop mixes in
	// before writing to the output device, avoiding concurrent writes to out.
	overlay overlayMix

	// Assist mode — protected by assistMu.
	// assistResumeCh is a buffered cap-1 channel; sending on it unblocks the
	// sessionLoop when it is waiting for the operator to trigger the next item.
	assistMu       sync.Mutex
	assistMode     bool
	assistResumeCh chan struct{}

	// streamingTap, when non-nil, receives a copy of each PCM frame slice
	// written to the output device. The streaming.Manager reads from the other
	// end and fans out to connected Icecast/SHOUTcast targets.
	// Set once via SetStreamingTap before any play session starts; no lock needed.
	streamingTap chan<- []float32
}

// NewManager creates a Manager. healthMon may be nil to disable audio health
// reporting from the playback pipeline.
func NewManager(
	evtBus *events.Bus,
	stateMgr *state.Manager,
	queueMgr *queue.Manager,
	dec decoder.Decoder,
	out output.OutputDevice,
	cfg Config,
	healthMon *health.Monitor,
	log *slog.Logger,
) *Manager {
	cfg.setDefaults()
	if log == nil {
		log = slog.Default()
	}
	m := &Manager{
		evtBus:    evtBus,
		stateMgr:  stateMgr,
		queueMgr:  queueMgr,
		dec:       dec,
		out:       out,
		cfg:       cfg,
		healthMon: healthMon,
		log:       log,
		skipCh:    make(chan struct{}, 1),
	}
	m.duckGain.Store(100)
	return m
}

// WithHoraCerta sets the Hora Certa resolver. Call before any Play command.
// If r is nil, HORA_CERTA items will be marked as failed with a clear log message.
func (m *Manager) WithHoraCerta(r *horacerta.Resolver) {
	m.horaCerta = r
}

// SetStreamingTap sets the channel that receives a copy of each PCM frame
// slice written to the output device. Call once before the first play session.
func (m *Manager) SetStreamingTap(ch chan<- []float32) {
	m.streamingTap = ch
}

// applyGain multiplies every sample in buf by gain.
// Returns immediately when gain == 1.0 to keep the hot path allocation-free.
func applyGain(buf []float32, gain float32) {
	if gain == 1.0 {
		return
	}
	for i := range buf {
		buf[i] *= gain
	}
}

// --- Command handlers --------------------------------------------------------

// HandlePlay starts a new play session if the queue is non-empty.
func (m *Manager) HandlePlay(ctx context.Context, _ commands.Command) error {
	// ASSIST mode: if a session is already running and the engine is in ASSIST,
	// treat PLAY as a manual "advance to next item" signal rather than starting
	// a new session.
	m.sessionMu.Lock()
	sessionRunning := m.sessionStop != nil
	m.sessionMu.Unlock()
	if sessionRunning && m.stateMgr.Snapshot().State == state.StateAssist {
		m.assistMu.Lock()
		ch := m.assistResumeCh
		m.assistMu.Unlock()
		if ch != nil {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
		return nil
	}

	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	if m.sessionStop != nil {
		return &commands.RejectedError{Reason: "already playing"}
	}
	if m.queueMgr.Size() == 0 && !m.queueMgr.HasCurrent() {
		return &commands.RejectedError{Reason: "queue is empty"}
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	m.sessionStop = cancel
	m.sessionDone = done
	m.sessionGen++
	gen := m.sessionGen
	m.framesTotal.Store(0)
	m.consecutiveFailures = 0

	go m.sessionLoop(sessionCtx, cancel, done, gen)
	return nil
}

// outputPauser is implemented by output devices that support hardware-level
// pause (e.g. CoreAudio via AudioQueuePause). Using a type assertion keeps the
// output.OutputDevice interface unchanged.
type outputPauser interface{ PauseAudio() error }

// outputResumer is the counterpart of outputPauser.
type outputResumer interface{ ResumeAudio() error }

// outputRestarter is implemented by output devices that support a clean
// stop-then-start cycle needed after an item drains the queue naturally
// (ASSIST mode wait). Unlike outputResumer (which calls AudioQueueStart on a
// paused queue to preserve buffered data), outputRestarter explicitly stops
// first to avoid the "hungry" auto-stopped queue problem.
type outputRestarter interface{ RestartAudio() error }

// HandlePause pauses an active session.
func (m *Manager) HandlePause(_ context.Context, _ commands.Command) error {
	m.pauseMu.Lock()
	defer m.pauseMu.Unlock()

	if m.paused {
		return &commands.RejectedError{Reason: "already paused"}
	}
	m.paused = true
	m.resumeCh = make(chan struct{})

	// Tell the output device to pause the audio hardware immediately.
	// This also signals any in-progress Write() call to return so the
	// playback loop is not stuck inside the output device.
	if p, ok := m.out.(outputPauser); ok {
		if err := p.PauseAudio(); err != nil {
			m.log.Warn("output pause failed", "error", err)
		}
	}

	prev := m.stateMgr.Snapshot().State
	m.pausedFromState = prev
	m.stateMgr.SetState(state.StatePaused)
	m.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: string(prev),
		To:   string(state.StatePaused),
		Mode: string(m.stateMgr.Snapshot().Mode),
	}))
	return nil
}

// HandleResume resumes a paused session.
func (m *Manager) HandleResume(_ context.Context, _ commands.Command) error {
	m.pauseMu.Lock()
	defer m.pauseMu.Unlock()

	if !m.paused {
		return &commands.RejectedError{Reason: "not paused"}
	}

	// Restart the audio hardware BEFORE unblocking the Go playback loop so
	// that CoreAudio is already running when the loop resumes feeding buffers.
	if r, ok := m.out.(outputResumer); ok {
		if err := r.ResumeAudio(); err != nil {
			m.log.Warn("output resume failed", "error", err)
		}
	}

	restoreState := m.pausedFromState
	m.pausedFromState = ""
	m.paused = false
	ch := m.resumeCh
	m.resumeCh = nil
	if ch != nil {
		close(ch)
	}

	// Restore the state that was active before the pause (e.g. ASSIST or PLAYING).
	// Fall back to PLAYING if no pre-pause state was recorded.
	if restoreState == "" || restoreState == state.StatePaused {
		restoreState = state.StatePlaying
	}
	m.stateMgr.SetState(restoreState)
	m.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: string(state.StatePaused),
		To:   string(restoreState),
		Mode: string(m.stateMgr.Snapshot().Mode),
	}))
	return nil
}

// HandleEnterAssist switches the engine into ASSIST mode. If a session is
// already running, the current item continues playing and the session loop
// will wait for the operator at the next item boundary. If there is no active
// session (engine was IDLE), an AssistWaiting event is published immediately
// so the UI shows the banner and the "▶ PLAY PRÓXIMO" button.
func (m *Manager) HandleEnterAssist(_ context.Context, _ commands.Command) error {
	m.assistMu.Lock()
	m.assistMode = true
	m.assistResumeCh = make(chan struct{}, 1)
	m.assistMu.Unlock()

	prev := m.stateMgr.Snapshot().State
	m.stateMgr.SetState(state.StateAssist)
	m.stateMgr.SetMode(state.ModeAssist)
	m.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: string(prev),
		To:   string(state.StateAssist),
		Mode: string(state.ModeAssist),
	}))
	m.evtBus.Publish(events.New(events.EvtAssistEntered, events.AssistEnteredPayload{}))

	// When entering ASSIST from IDLE (no active session), publish AssistWaiting
	// immediately so the UI shows the play button without waiting for a session.
	m.sessionMu.Lock()
	sessionRunning := m.sessionStop != nil
	m.sessionMu.Unlock()
	if !sessionRunning {
		next, _ := m.queueMgr.Peek()
		nextTitle, nextType := "", ""
		if next != nil {
			nextTitle = next.Title
			nextType = string(next.Type)
		}
		m.evtBus.Publish(events.New(events.EvtAssistWaiting, events.AssistWaitingPayload{
			NextTitle: nextTitle,
			NextType:  nextType,
			QueueSize: m.queueMgr.Size(),
		}))
	}

	return nil
}

// HandleReturnAuto exits ASSIST mode and returns the engine to AUTO (PLAYING).
// If the session loop is currently waiting for the operator, it is unblocked
// and the queue resumes advancing automatically. If there was no active session
// (ASSIST entered from IDLE) and the queue has items, playback starts now.
func (m *Manager) HandleReturnAuto(ctx context.Context, _ commands.Command) error {
	m.assistMu.Lock()
	m.assistMode = false
	ch := m.assistResumeCh
	m.assistResumeCh = nil
	m.assistMu.Unlock()

	prev := m.stateMgr.Snapshot().State

	// Determine correct target state: IDLE when no session is running and the
	// queue is empty; PLAYING otherwise (active session or items waiting to play).
	m.sessionMu.Lock()
	hasSession := m.sessionStop != nil
	m.sessionMu.Unlock()
	targetState := state.StatePlaying
	if !hasSession && m.queueMgr.Size() == 0 && !m.queueMgr.HasCurrent() {
		targetState = state.StateIdle
	}

	m.stateMgr.SetState(targetState)
	m.stateMgr.SetMode(state.ModeAuto)
	m.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: string(prev),
		To:   string(targetState),
		Mode: string(state.ModeAuto),
	}))
	m.evtBus.Publish(events.New(events.EvtAssistExited, events.AssistExitedPayload{}))

	// Unblock sessionLoop if it is waiting for an operator signal.
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	// If no session was running (ASSIST entered from IDLE) and the queue has
	// items, start playback now so the state reflects reality.
	m.sessionMu.Lock()
	if m.sessionStop == nil && (m.queueMgr.Size() > 0 || m.queueMgr.HasCurrent()) {
		sessionCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		m.sessionStop = cancel
		m.sessionDone = done
		m.sessionGen++
		gen := m.sessionGen
		m.framesTotal.Store(0)
		m.consecutiveFailures = 0
		go m.sessionLoop(sessionCtx, cancel, done, gen)
	}
	m.sessionMu.Unlock()

	return nil
}

// HandleStop stops an active session. When called from StatePanic it is
// equivalent to EXIT_PANIC — the panic bed is stopped and the engine
// returns to IDLE.
func (m *Manager) HandleStop(_ context.Context, cmd commands.Command) error {
	p, _ := cmd.Payload.(commands.StopPayload)

	// Stopping while in panic mode exits panic (also stops the bed).
	if m.stateMgr.Snapshot().State == state.StatePanic {
		return m.doExitPanic("stopped")
	}

	m.sessionMu.Lock()
	cancel := m.sessionStop
	done := m.sessionDone
	m.sessionMu.Unlock()

	if cancel == nil {
		return nil // idempotent
	}

	m.forceResume()
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		m.log.Warn("playback session did not stop in time")
	}

	if p.ClearQueue {
		m.queueMgr.Clear(false)
	}
	return nil
}

// HandleSkip skips the current non-mandatory item.
func (m *Manager) HandleSkip(_ context.Context, _ commands.Command) error {
	m.currentMu.RLock()
	cur := m.current
	m.currentMu.RUnlock()

	if cur == nil {
		return &commands.RejectedError{Reason: "nothing is playing"}
	}
	if cur.Mandatory {
		return &commands.RejectedError{Reason: fmt.Sprintf("item %s is mandatory and cannot be skipped", cur.QueueItemID)}
	}

	select {
	case m.skipCh <- struct{}{}:
	default:
	}
	return nil
}

// --- Panic mode handlers -----------------------------------------------------

// HandleEnterPanic activates panic mode. It has maximum priority and is
// allowed in any state except STOPPING (enforced by the Dispatcher).
// Any active session is interrupted. If a bed is specified in the payload it
// is played in a loop until EXIT_PANIC or STOP arrives.
func (m *Manager) HandleEnterPanic(ctx context.Context, cmd commands.Command) error {
	p, _ := cmd.Payload.(commands.EnterPanicPayload)

	// Stop any running normal session first.
	m.sessionMu.Lock()
	cancel := m.sessionStop
	done := m.sessionDone
	m.sessionMu.Unlock()
	if cancel != nil {
		m.forceResume()
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			m.log.Warn("session did not stop before panic")
		}
	}

	// Stop any existing panic bed (re-entering replaces it).
	m.stopPanicBed()

	// Transition to PANIC.
	prev := m.stateMgr.Snapshot().State
	m.stateMgr.SetState(state.StatePanic)
	m.stateMgr.SetMode(state.ModePanic)
	m.stateMgr.ClearNowPlaying()

	bedAssetID := ""
	if p.Bed != nil {
		bedAssetID = p.Bed.AssetID
	}

	m.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: string(prev),
		To:   string(state.StatePanic),
		Mode: string(state.ModePanic),
	}))
	m.evtBus.Publish(events.New(events.EvtPanicEntered, events.PanicEnteredPayload{
		Reason:     p.Reason,
		BedAssetID: bedAssetID,
	}))
	m.log.Warn("panic mode entered", "reason", p.Reason, "bed_asset_id", bedAssetID)

	// Resolve the bed to play: prefer payload, fall back to configured bed_path.
	bed := p.Bed
	if (bed == nil || bed.Path == "") && m.cfg.PanicBedPath != "" {
		bed = &commands.PanicBedInput{Path: m.cfg.PanicBedPath}
	}

	if bed == nil || bed.Path == "" {
		m.log.Warn("panic mode: no bed configured — engine is silent in PANIC state")
	} else {
		m.log.Info("panic mode: starting bed loop", "path", bed.Path)
	}

	// Start the panic bed loop if a bed path is available.
	// Use context.Background() — the bed must outlive the HTTP request context.
	if bed != nil && bed.Path != "" {
		panicCtx, panicCancel := context.WithCancel(context.Background())
		bedDone := make(chan struct{})
		m.panicMu.Lock()
		m.panicStop = panicCancel
		m.panicDone = bedDone
		m.panicMu.Unlock()
		go m.panicBedLoop(panicCtx, *bed, bedDone)
	}

	return nil
}

// HandleExitPanic deactivates panic mode and returns the engine to IDLE.
// If the queue has pending items, playback starts automatically.
func (m *Manager) HandleExitPanic(ctx context.Context, cmd commands.Command) error {
	p, _ := cmd.Payload.(commands.ExitPanicPayload)
	if err := m.doExitPanic(p.Reason); err != nil {
		return err
	}
	// Auto-start playback when the queue is non-empty.
	if m.queueMgr.Size() > 0 || m.queueMgr.HasCurrent() {
		m.sessionMu.Lock()
		if m.sessionStop == nil { // guard: no session already running
			sessionCtx, cancel := context.WithCancel(ctx)
			done := make(chan struct{})
			m.sessionStop = cancel
			m.sessionDone = done
			m.sessionGen++
			gen := m.sessionGen
			m.framesTotal.Store(0)
			m.consecutiveFailures = 0
			go m.sessionLoop(sessionCtx, cancel, done, gen)
		}
		m.sessionMu.Unlock()
	}
	return nil
}

// doExitPanic contains the shared exit-panic logic used by HandleExitPanic
// and HandleStop when called from StatePanic.
func (m *Manager) doExitPanic(reason string) error {
	m.stopPanicBed()

	prev := m.stateMgr.Snapshot().State
	m.stateMgr.SetState(state.StateIdle)
	m.stateMgr.SetMode(state.ModeAuto)
	m.stateMgr.ClearNowPlaying()

	m.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: string(prev),
		To:   string(state.StateIdle),
		Mode: string(state.ModeAuto),
	}))
	m.evtBus.Publish(events.New(events.EvtPanicExited, events.PanicExitedPayload{
		Reason: reason,
	}))
	m.log.Info("panic mode exited", "reason", reason)
	return nil
}

// stopPanicBed cancels the panic bed goroutine and waits for it to finish.
// It is safe to call when no bed is running.
func (m *Manager) stopPanicBed() {
	m.panicMu.Lock()
	cancel := m.panicStop
	done := m.panicDone
	m.panicStop = nil
	m.panicDone = nil
	m.panicMu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		m.log.Warn("panic bed did not stop in time")
	}
}

// panicBedLoop opens the output device and plays bed in an infinite loop
// until ctx is cancelled (by stopPanicBed or engine shutdown).
func (m *Manager) panicBedLoop(ctx context.Context, bed commands.PanicBedInput, done chan struct{}) {
	defer close(done)

	outCfg := output.OutputConfig{
		DeviceID:     m.cfg.DeviceID,
		SampleRate:   audio.DefaultFormat.SampleRate,
		Channels:     audio.DefaultFormat.Channels,
		BufferFrames: m.cfg.BufferFrames,
	}
	m.log.Info("panic bed: opening output device")
	if err := m.out.Open(ctx, outCfg); err != nil {
		m.log.Error("panic bed: output open failed", "error", err)
		return
	}
	m.log.Info("panic bed: output opened")
	defer m.out.Close() //nolint:errcheck

	if err := m.out.Start(ctx); err != nil {
		m.log.Error("panic bed: output start failed", "error", err)
		return
	}
	m.log.Info("panic bed: output started")
	defer m.out.Stop(ctx) //nolint:errcheck

	spf := audio.DefaultFormat.SamplesPerFrame()
	buf := make([]float32, m.cfg.BufferFrames*spf)

	for ctx.Err() == nil {
		m.log.Info("panic bed: opening decoder", "path", bed.Path)
		// Use context.Background() for decoder to ensure it is never cancelled
		// by any parent context chain. panicCtx controls the outer loop only.
		stream, err := m.dec.Open(context.Background(), decoder.Source{Path: bed.Path})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.log.Error("panic bed: decoder open failed", "path", bed.Path, "error", err)
			return
		}
		m.log.Info("panic bed: decoder opened, starting read loop", "ctx_err", ctx.Err())

		frames := 0
		for ctx.Err() == nil {
			n, rerr := stream.ReadFrames(ctx, buf)
			if n > 0 {
				frames += n
				if frames <= spf*10 {
					m.log.Info("panic bed: read frames, writing to output", "n", n, "total_frames", frames)
				}
				applyGain(buf[:n*spf], m.stateMgr.MainVolume())
				if _, werr := m.out.Write(ctx, buf[:n*spf]); werr != nil && ctx.Err() == nil {
					m.log.Error("panic bed: output write failed", "error", werr)
					stream.Close() //nolint:errcheck
					return
				}
				if frames <= spf*10 {
					m.log.Info("panic bed: write to output OK", "total_frames", frames)
				}
			}
			if rerr != nil {
				m.log.Info("panic bed: end of file, looping", "frames_played", frames, "err", rerr, "n_last", n)
				break
			}
		}
		stream.Close() //nolint:errcheck
	}
	m.log.Info("panic bed: loop exited", "ctx_err", ctx.Err())
}

// --- Hot button handlers -----------------------------------------------------

// HandleTriggerHotButton plays a one-shot audio asset according to PlayMode:
//
//   - OVERLAY        — plays the hot button concurrently with the main session,
//     optionally ducking the main stream. Uses inline mixing.
//   - INTERRUPT      — stops the current session, plays the hot button,
//     then returns to IDLE (queue remains intact for a subsequent PLAY).
//   - AFTER_CURRENT  — inserts the hot button asset as the next queue item.
func (m *Manager) HandleTriggerHotButton(ctx context.Context, cmd commands.Command) error {
	p, _ := cmd.Payload.(commands.TriggerHotButtonPayload)

	if p.Asset.Path == "" {
		return &commands.RejectedError{Reason: "hot button asset path is required"}
	}

	m.evtBus.Publish(events.New(events.EvtHotButtonTriggered, events.HotButtonTriggeredPayload{
		ButtonID: p.ButtonID,
		AssetID:  p.Asset.AssetID,
		PlayMode: p.PlayMode,
	}))
	m.log.Info("hot button triggered", "button_id", p.ButtonID, "play_mode", p.PlayMode)

	switch p.PlayMode {
	case "OVERLAY":
		return m.hotButtonOverlay(ctx, p)
	case "INTERRUPT":
		return m.hotButtonInterrupt(ctx, p)
	case "AFTER_CURRENT":
		return m.hotButtonAfterCurrent(p)
	default:
		return &commands.RejectedError{Reason: "play_mode must be OVERLAY, INTERRUPT, or AFTER_CURRENT"}
	}
}

// HandleSetVolume handles CmdSetVolume: updates the main queue gain, publishes
// EvtVolumeChanged, and persists the new level to the preferences file.
func (m *Manager) HandleSetVolume(_ context.Context, cmd commands.Command) error {
	payload, ok := cmd.Payload.(commands.SetVolumePayload)
	if !ok {
		return fmt.Errorf("playback: HandleSetVolume: unexpected payload type %T", cmd.Payload)
	}
	m.stateMgr.SetMainVolume(payload.Level)
	m.evtBus.Publish(events.New(events.EvtVolumeChanged, events.VolumeChangedPayload{Level: payload.Level}))
	p := prefs.Load(prefs.DefaultPath())
	p.MainVolume = payload.Level
	if err := prefs.Save(prefs.DefaultPath(), p); err != nil {
		m.log.Warn("playback: failed to save preferences", "error", err)
	}
	return nil
}

// hotButtonOverlay plays the asset concurrently with the main session.
// If DuckMain is true, the main stream gain is faded down and back up smoothly.
func (m *Manager) hotButtonOverlay(ctx context.Context, p commands.TriggerHotButtonPayload) error {
	// Stop any existing overlay first, then clear leftover samples so that
	// stale audio from the previous hot button does not bleed into the new one.
	m.stopHotOverlay()
	m.overlay.reset()

	// Publish DuckingStarted synchronously so callers (and tests) can observe
	// it before this function returns. The actual gain ramp happens inside the
	// goroutine so it is smooth and does not block the command handler.
	if p.DuckMain {
		m.evtBus.Publish(events.New(events.EvtDuckingStarted, events.DuckingStartedPayload{
			TargetChannel: "main",
			GainDB:        p.DuckGainDB,
		}))
	}

	hotCtx, hotCancel := context.WithCancel(ctx)
	done := make(chan struct{})

	m.hotMu.Lock()
	m.hotStop = hotCancel
	m.hotDone = done
	m.hotMu.Unlock()

	go m.overlayLoop(hotCtx, p, done)
	return nil
}

// overlayLoop decodes the hot button asset and pushes its samples into the
// overlayMix buffer. The main session loop (runPlayLoop) drains that buffer
// and adds the samples to the main PCM stream before each output write,
// ensuring only one goroutine ever calls m.out.Write at a time.
//
// Duck gain: if requested, the main stream gain is faded down smoothly before
// playback starts and faded back up after the carimbo ends, avoiding the
// abrupt "music paused" perception caused by instant gain changes.
//
// Rate-limiting: FFmpeg decodes far faster than real-time. Without throttling
// the buffer would fill instantly and all remaining samples would be dropped.
// overlayLoop sleeps when the buffer already holds more than 250ms of audio,
// keeping the ahead-buffer small while never blocking the main loop.
func (m *Manager) overlayLoop(ctx context.Context, p commands.TriggerHotButtonPayload, done chan struct{}) {
	defer close(done)
	defer func() {
		// Duck-out: fade gain smoothly back to 100%.
		// Uses context.Background() so the ramp completes even when the overlay
		// ctx was cancelled (e.g. by stopHotOverlay during a skip or stop).
		if p.DuckMain {
			m.rampDuckGain(context.Background(), 100, 200)
			m.evtBus.Publish(events.New(events.EvtDuckingEnded, events.DuckingEndedPayload{
				TargetChannel: "main",
			}))
		}
		m.hotMu.Lock()
		m.hotStop = nil
		m.hotDone = nil
		m.hotMu.Unlock()
	}()

	// Duck-in: fade main stream gain down smoothly before the carimbo starts.
	// This runs synchronously inside the goroutine so the ramp always finishes
	// before the decode loop begins, preventing a race with the ramp-out.
	if p.DuckMain {
		gainPct := int32(output.DBToLinear(p.DuckGainDB) * 100)
		if gainPct < 0 {
			gainPct = 0
		}
		if gainPct > 100 {
			gainPct = 100
		}
		m.rampDuckGain(ctx, gainPct, 200)
	}

	stream, err := m.dec.Open(ctx, decoder.Source{Path: p.Asset.Path})
	if err != nil {
		m.log.Error("hot button overlay: decoder open failed", "path", p.Asset.Path, "error", err)
		return
	}
	defer stream.Close() //nolint:errcheck

	spf := audio.DefaultFormat.SamplesPerFrame()
	buf := make([]float32, m.cfg.BufferFrames*spf)

	// Keep at most 250ms of pre-decoded audio ahead of the main loop.
	// At 48 kHz stereo: 48000 * 2 * 0.25 = 24000 samples.
	const maxAheadSamples = 48000 * 2 * 250 / 1000

	for ctx.Err() == nil {
		// Throttle: wait until the buffer has drained below the target.
		for m.overlay.buffered() > maxAheadSamples {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Millisecond):
			}
		}

		n, rerr := stream.ReadFrames(ctx, buf)
		if n > 0 {
			// Push into the mix buffer — the main loop consumes and writes.
			m.overlay.push(buf[:n*spf])
		}
		if rerr != nil {
			return // EOF or decode error — hot button finished
		}
	}
}

// stopHotOverlay cancels any running overlay loop and waits for it to finish.
func (m *Manager) stopHotOverlay() {
	m.hotMu.Lock()
	cancel := m.hotStop
	done := m.hotDone
	m.hotMu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		m.log.Warn("hot button overlay did not stop in time")
	}
}

// hotButtonInterrupt stops the current session, plays the asset, then
// leaves the engine in IDLE (queue remains for a subsequent PLAY).
func (m *Manager) hotButtonInterrupt(ctx context.Context, p commands.TriggerHotButtonPayload) error {
	// Stop the main session.
	m.sessionMu.Lock()
	cancel := m.sessionStop
	done := m.sessionDone
	m.sessionMu.Unlock()
	if cancel != nil {
		m.forceResume()
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			m.log.Warn("interrupt: session did not stop in time")
		}
	}

	// Play the hot button synchronously (blocks until done or ctx cancelled).
	outCfg := output.OutputConfig{
		DeviceID:     m.cfg.DeviceID,
		SampleRate:   audio.DefaultFormat.SampleRate,
		Channels:     audio.DefaultFormat.Channels,
		BufferFrames: m.cfg.BufferFrames,
	}
	if err := m.out.Open(ctx, outCfg); err != nil {
		m.log.Error("interrupt hot button: output open failed", "error", err)
		return nil
	}
	defer m.out.Close() //nolint:errcheck

	if err := m.out.Start(ctx); err != nil {
		m.log.Error("interrupt hot button: output start failed", "error", err)
		return nil
	}
	defer m.out.Stop(ctx) //nolint:errcheck

	stream, err := m.dec.Open(ctx, decoder.Source{Path: p.Asset.Path})
	if err != nil {
		m.log.Error("interrupt hot button: decoder open failed", "path", p.Asset.Path, "error", err)
		return nil
	}
	defer stream.Close() //nolint:errcheck

	spf := audio.DefaultFormat.SamplesPerFrame()
	buf := make([]float32, m.cfg.BufferFrames*spf)
	for ctx.Err() == nil {
		n, rerr := stream.ReadFrames(ctx, buf)
		if n > 0 {
			applyGain(buf[:n*spf], m.stateMgr.MainVolume())
			if _, werr := m.out.Write(ctx, buf[:n*spf]); werr != nil && ctx.Err() == nil {
				m.log.Error("interrupt hot button: output write failed", "error", werr)
				return nil
			}
			// Send a copy to the streaming tap so the interrupt audio reaches
			// connected Icecast/SHOUTcast targets, just like the main loop does.
			if tap := m.streamingTap; tap != nil {
				cp := make([]float32, n*spf)
				copy(cp, buf[:n*spf])
				select {
				case tap <- cp:
				default:
				}
			}
		}
		if rerr != nil {
			break
		}
	}
	return nil
}

// hotButtonAfterCurrent inserts the asset immediately after the current item.
func (m *Manager) hotButtonAfterCurrent(p commands.TriggerHotButtonPayload) error {
	m.queueMgr.InsertNext(commands.QueueItemInput{
		AssetID:    p.Asset.AssetID,
		Path:       p.Asset.Path,
		Type:       p.Asset.Type,
		Title:      p.Asset.Title,
		Artist:     p.Asset.Artist,
		DurationMS: p.Asset.DurationMS,
		CueInMS:    p.Asset.CueInMS,
		IntroMS:    p.Asset.IntroMS,
		OutroMS:    p.Asset.OutroMS,
		CueOutMS:   p.Asset.CueOutMS,
		Mandatory:  p.Asset.Mandatory,
		Metadata:   p.Asset.Metadata,
	})
	return nil
}

// --- Session loop ------------------------------------------------------------

// sessionLoop drives item playback until the queue is exhausted, a stop is
// requested, or too many consecutive failures occur.
func (m *Manager) sessionLoop(ctx context.Context, cancel context.CancelFunc, done chan struct{}, gen int) {
	defer close(done)
	defer cancel()

	// Transition to PLAYING, unless the engine was already put in ASSIST mode
	// before the session started (test scenario or edge case where the operator
	// called ASSIST before PLAY).
	prev := m.stateMgr.Snapshot().State
	m.assistMu.Lock()
	inAssist := m.assistMode
	m.assistMu.Unlock()
	if inAssist {
		m.stateMgr.SetState(state.StateAssist)
		m.stateMgr.SetMode(state.ModeAssist)
	} else {
		m.stateMgr.SetState(state.StatePlaying)
	}
	m.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: string(prev),
		To:   string(m.stateMgr.Snapshot().State),
		Mode: string(m.stateMgr.Snapshot().Mode),
	}))

	// Open output device.
	outCfg := output.OutputConfig{
		DeviceID:     m.cfg.DeviceID,
		SampleRate:   audio.DefaultFormat.SampleRate,
		Channels:     audio.DefaultFormat.Channels,
		BufferFrames: m.cfg.BufferFrames,
	}
	if err := m.out.Open(ctx, outCfg); err != nil {
		m.log.Error("output open failed", "error", err)
		m.evtBus.Publish(events.New(events.EvtOutputOpenFailed, events.OutputFailedPayload{
			Code: "OUTPUT_OPEN_FAILED", Message: err.Error(),
		}))
		m.transitionToIdle(gen)
		return
	}
	defer m.out.Close() //nolint:errcheck

	if err := m.out.Start(ctx); err != nil {
		m.log.Error("output start failed", "error", err)
		m.transitionToIdle(gen)
		return
	}
	defer m.out.Stop(ctx) //nolint:errcheck

	// Progress reporting goroutine.
	pCtx, pCancel := context.WithCancel(ctx)
	pDone := make(chan struct{})
	go m.progressLoop(pCtx, pDone)
	defer func() { pCancel(); <-pDone }()

	// Carry-over from a completed crossfade: the next item's stream is already
	// open and its tracking has been initialised.
	var carryStream decoder.PCMStream
	var carryItem *queue.QueueItem

	for {
		if ctx.Err() != nil {
			break
		}

		var item *queue.QueueItem
		var stream decoder.PCMStream

		if carryStream != nil {
			// Previous iteration completed a crossfade — use the already-open stream.
			item = carryItem
			stream = carryStream
			carryStream = nil
			carryItem = nil
			// State was already announced at crossfade completion; skip setup.
		} else {
			var ok bool
			// PopAsCurrent atomically removes the item from pending and marks it as
			// current, preventing the race window between Pop and SetCurrent that
			// could cause GET /v1/queue to return without the playing item.
			item, ok = m.queueMgr.PopAsCurrent()
			if !ok {
				m.log.Info("queue exhausted, stopping session")
				break
			}

			var err error
			if item.Type == queue.AssetTypeHoraCerta {
				stream, err = m.openHoraCerta(ctx, item)
			} else {
				stream, err = m.dec.Open(ctx, decoder.Source{
					Path:     item.Path,
					CueInMS:  item.CueInMS,
					CueOutMS: item.CueOutMS,
				})
			}
			if err != nil {
				m.log.Error("decoder open failed", "path", item.Path, "error", err)
				m.queueMgr.ClearCurrent() // item was set as current by PopAsCurrent; undo on failure
				m.evtBus.Publish(events.New(events.EvtDecoderError, events.DecoderErrorPayload{
					Code: "DECODER_OPEN_FAILED", Message: err.Error(),
					QueueItemID: item.QueueItemID, AssetID: item.AssetID, Recoverable: true,
				}))
				m.evtBus.Publish(events.New(events.EvtItemFinished, events.ItemFinishedPayload{
					QueueItemID: item.QueueItemID, AssetID: item.AssetID,
					Result: string(queue.ItemResultFailed),
				}))
				if m.handleFailure(ctx, gen) {
					break
				}
				continue
			}

			// Announce the new item.
			m.startItem(item, m.framesTotal.Load())
		}

		// Run the frame-reading loop (with crossfade support).
		result, nextStream, nextItem, xFramesDone := m.runPlayLoop(ctx, item, stream)
		stream.Close() //nolint:errcheck

		playedMS := audio.DefaultFormat.MsFromFrames(m.framesTotal.Load() - m.itemStartFrame)
		if playedMS > item.DurationMS && item.DurationMS > 0 {
			playedMS = item.DurationMS
		}

		// If crossfade completed, set up the next item before clearing the current.
		if nextStream != nil && nextItem != nil && ctx.Err() == nil {
			// Notify break boundary before the next item starts so that
			// SpotEnded / BreakEnded fire before SpotStarted of the next item.
			m.notifyBreakTransition(item, nextItem.BreakID)
			// Back-date the start frame so progress is continuous for the listener.
			m.startItem(nextItem, m.framesTotal.Load()-xFramesDone)
			carryStream = nextStream
			carryItem = nextItem
		}

		// STOP: return item to front of queue and exit loop without ItemFinished.
		if result == queue.ItemResultStopped {
			m.currentMu.Lock()
			m.current = nil
			m.currentMu.Unlock()
			m.queueMgr.ReturnCurrentToFront()
			m.stateMgr.ClearNowPlaying()
			break
		}

		// Clear the finished item from state.
		m.currentMu.Lock()
		if carryStream == nil {
			m.current = nil
		}
		m.currentMu.Unlock()
		m.queueMgr.ClearCurrent()
		if carryStream == nil {
			m.stateMgr.ClearNowPlaying()
			// No crossfade: notify break boundary now (after ClearCurrent so
			// Peek() sees the correct next item in the pending queue).
			next, _ := m.queueMgr.Peek()
			nextBreakID := ""
			if next != nil {
				nextBreakID = next.BreakID
			}
			m.notifyBreakTransition(item, nextBreakID)

			// ASSIST mode: wait for operator trigger at item boundaries.
			// Exception: auto-advance within the same break (spots share BreakID).
			if m.stateMgr.Snapshot().State == state.StateAssist {
				if item.BreakID == "" || nextBreakID != item.BreakID {
					if !m.waitAssistSignal(ctx) {
						break
					}
					// CoreAudio AudioQueue auto-stops when all buffered frames are
					// consumed (~128 ms after the previous item ended). Use
					// RestartAudio (explicit Stop + Start) rather than ResumeAudio
					// (Start only). Without an explicit stop first the queue is in
					// an auto-stopped/"hungry" state where it fires callbacks
					// immediately instead of at the hardware clock rate, causing
					// writes to complete at 10-12× real-time speed and corrupting
					// position tracking.
					if r, ok := m.out.(outputRestarter); ok {
						if err := r.RestartAudio(); err != nil {
							m.log.Warn("assist wait: output restart failed", "error", err)
						}
					}
				}
			}
		}

		m.evtBus.Publish(events.New(events.EvtItemFinished, events.ItemFinishedPayload{
			QueueItemID:      item.QueueItemID,
			AssetID:          item.AssetID,
			Result:           string(result),
			DurationPlayedMS: playedMS,
		}))

		switch result {
		case queue.ItemResultFailed:
			if m.handleFailure(ctx, gen) {
				break
			}
		default:
			m.consecutiveFailures = 0
		}

		if result == queue.ItemResultSkipped && ctx.Err() != nil {
			break
		}
	}

	m.transitionToIdle(gen)
}

// startItem updates all current-item tracking and publishes the announcement
// events. startFrame is the value of framesTotal at which the item started
// (use framesTotal.Load() for a fresh start; subtract xfade frames for carry-over).
func (m *Manager) startItem(item *queue.QueueItem, startFrame int64) {
	m.currentMu.Lock()
	prevBreakID := m.currentBreakID
	m.current = item
	m.currentBreakID = item.BreakID
	m.itemStartFrame = startFrame
	m.currentMu.Unlock()

	m.queueMgr.SetCurrent(item)
	m.stateMgr.SetNowPlaying(&state.NowPlaying{
		QueueItemID:   item.QueueItemID,
		AssetID:       item.AssetID,
		Path:          item.Path,
		Title:         item.Title,
		Artist:        item.Artist,
		Type:          string(item.Type),
		DurationMS:    item.DurationMS,
		BreakID:       item.BreakID,
		BreakTitle:    item.BreakTitle,
		BreakPosition: item.BreakSeq,
		BreakTotal:    item.BreakTotal,
		BreakRole:     item.BreakRole,
	})
	m.evtBus.Publish(events.New(events.EvtNowPlayingChanged, events.NowPlayingChangedPayload{
		QueueItemID:   item.QueueItemID,
		AssetID:       item.AssetID,
		Path:          item.Path,
		Title:         item.Title,
		Artist:        item.Artist,
		ISRC:          item.ISRC,
		Composer:      item.Composer,
		Publisher:     item.Publisher,
		Type:          string(item.Type),
		DurationMS:    item.DurationMS,
		BreakID:       item.BreakID,
		BreakTitle:    item.BreakTitle,
		BreakPosition: item.BreakSeq,
		BreakTotal:    item.BreakTotal,
		BreakRole:     item.BreakRole,
	}))

	// Publish commercial break events when entering a break.
	if item.BreakID != "" {
		if prevBreakID != item.BreakID {
			// First sub-item of this break — announce the break start.
			m.evtBus.Publish(events.New(events.EvtBreakStarted, events.BreakStartedPayload{
				BreakID:    item.BreakID,
				BreakTitle: item.BreakTitle,
				BreakTotal: item.BreakTotal,
			}))
		}
		m.evtBus.Publish(events.New(events.EvtSpotStarted, events.SpotStartedPayload{
			BreakID:     item.BreakID,
			BreakTitle:  item.BreakTitle,
			QueueItemID: item.QueueItemID,
			Title:       item.Title,
			BreakSeq:    item.BreakSeq,
			BreakTotal:  item.BreakTotal,
			BreakRole:   item.BreakRole,
		}))
	}

	m.evtBus.Publish(events.New(events.EvtItemStarted, events.ItemStartedPayload{
		QueueItemID: item.QueueItemID,
		AssetID:     item.AssetID,
	}))
}

// notifyBreakTransition publishes SpotEnded and, when the break is over,
// BreakEnded. Call this when finished is about to stop playing and the
// next item (if any) is known.
// nextBreakID should be "" when there is no next item or the next item is
// not part of any break.
func (m *Manager) notifyBreakTransition(finished *queue.QueueItem, nextBreakID string) {
	if finished == nil || finished.BreakID == "" {
		return
	}
	m.evtBus.Publish(events.New(events.EvtSpotEnded, events.SpotEndedPayload{
		BreakID:     finished.BreakID,
		QueueItemID: finished.QueueItemID,
		BreakSeq:    finished.BreakSeq,
	}))
	if nextBreakID != finished.BreakID {
		m.evtBus.Publish(events.New(events.EvtBreakEnded, events.BreakEndedPayload{
			BreakID:    finished.BreakID,
			BreakTitle: finished.BreakTitle,
		}))
		m.currentMu.Lock()
		m.currentBreakID = ""
		m.currentMu.Unlock()
	}
}

// openHoraCerta resolves the current time to hour/minute audio files and
// opens them as a single chained PCMStream with the configured gain applied.
func (m *Manager) openHoraCerta(ctx context.Context, item *queue.QueueItem) (decoder.PCMStream, error) {
	if m.horaCerta == nil {
		return nil, fmt.Errorf("HORA_CERTA item %q: hora certa resolver not configured (missing hora_certa config block)", item.QueueItemID)
	}
	paths, err := m.horaCerta.Resolve(time.Now())
	if err != nil {
		return nil, fmt.Errorf("HORA_CERTA item %q: %w", item.QueueItemID, err)
	}
	gainDB := m.horaCerta.EffectiveGainDB(item.GainDB)
	m.log.Info("hora certa playing",
		"queue_item_id", item.QueueItemID,
		"paths", paths,
		"gain_db", gainDB,
	)
	return m.horaCerta.OpenChain(ctx, m.dec, paths, gainDB)
}

// handleFailure increments the consecutive-failure counter, waits with
// exponential backoff (100ms × 2^n, capped at 2s), and returns true if the
// session should abort (too many failures).
func (m *Manager) handleFailure(ctx context.Context, gen int) bool {
	m.consecutiveFailures++

	// Exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms → capped 2s.
	backoff := time.Duration(100<<uint(m.consecutiveFailures-1)) * time.Millisecond
	if backoff > 2*time.Second {
		backoff = 2 * time.Second
	}
	m.log.Warn("playback item failed, backing off",
		"failures", m.consecutiveFailures,
		"backoff", backoff)
	select {
	case <-time.After(backoff):
	case <-ctx.Done():
		return true
	}

	if m.consecutiveFailures >= m.cfg.MaxConsecutiveFailures {
		m.log.Error("too many consecutive failures, entering error state",
			"failures", m.consecutiveFailures)
		m.stateMgr.SetError(fmt.Sprintf("playback: %d consecutive failures", m.consecutiveFailures))
		m.sessionMu.Lock()
		if m.sessionGen == gen {
			m.sessionStop = nil
			m.sessionDone = nil
		}
		m.sessionMu.Unlock()
		return true
	}
	return false
}

// --- Frame-reading loop with crossfade ---------------------------------------

// runPlayLoop reads from stream and writes to the output device, applying
// crossfade mixing when the trigger point is reached.
//
// Returns:
//   - result       — PLAYED, SKIPPED, or FAILED
//   - nextStream   — non-nil only when a crossfade completed; caller owns it
//   - nextItem     — the queue item whose stream was opened for the crossfade
//   - xFramesDone  — frames of nextItem that were already mixed into the output
func (m *Manager) runPlayLoop(
	ctx context.Context,
	item *queue.QueueItem,
	stream decoder.PCMStream,
) (result queue.ItemResult, nextStream decoder.PCMStream, nextItem *queue.QueueItem, xFramesDone int64) {
	spf := audio.DefaultFormat.SamplesPerFrame()
	bufSize := m.cfg.BufferFrames * spf
	buf := make([]float32, bufSize)
	nextBuf := make([]float32, bufSize)
	overlayBuf := make([]float32, bufSize) // receives overlay samples for software mixing
	result = queue.ItemResultPlayed

	xfadeDurMS := m.xfadeDurationFor(item)

	// Crossfade working state.
	var xStream decoder.PCMStream
	var xItem *queue.QueueItem
	var xStarted bool
	var xFrames int64 // frames consumed from xStream so far
	var xTotal int64  // total frames in the xfade period

	// Energy-based crossfade trigger state.
	var energyHoldCount int  // consecutive low-energy read cycles
	var energyTriggered bool // latched when hold threshold is reached
	var lastReadSamples int  // valid samples in buf from the previous read

	// Ensure xStream is closed on abnormal exit (normal exit hands it to caller).
	defer func() {
		if xStream != nil {
			xStream.Close()
			xStream = nil
		}
	}()

	for {
		// Stop check (context cancelled by STOP command).
		if ctx.Err() != nil {
			result = queue.ItemResultStopped
			return
		}

		// Skip signal.
		select {
		case <-m.skipCh:
			result = queue.ItemResultSkipped
			return
		default:
		}

		// Pause.
		m.pauseMu.Lock()
		paused, resumeCh := m.paused, m.resumeCh
		m.pauseMu.Unlock()
		if paused {
			select {
			case <-resumeCh:
			case <-ctx.Done():
				result = queue.ItemResultSkipped
				return
			}
		}

		// ---- crossfade trigger check ----
		if !xStarted && xfadeDurMS > 0 && item.DurationMS > 0 {
			posFrames := m.framesTotal.Load() - m.itemStartFrame
			posMS := audio.DefaultFormat.MsFromFrames(posFrames)
			cueOutMS := item.EffectiveCueOut()
			// OutroMS, when set, marks the exact moment to start the crossfade
			// (the musical outro begins). Fall back to the time-based calculation.
			var xStartMS int64
			if item.OutroMS > 0 && item.OutroMS > item.CueInMS {
				xStartMS = item.OutroMS
			} else {
				xStartMS = cueOutMS - int64(xfadeDurMS)
			}

			// Energy-based trigger: evaluate RMS of the previous read buffer.
			if m.cfg.AutoCrossfadeEnabled && !energyTriggered && lastReadSamples > 0 {
				timeRemaining := cueOutMS - posMS
				if timeRemaining >= int64(m.cfg.AutoCrossfadeMinBeforeEndMS) &&
					timeRemaining <= int64(m.cfg.AutoCrossfadeMaxBeforeEndMS) {
					rmsDB := bufRMSdBFS(buf[:lastReadSamples])
					if rmsDB < m.cfg.AutoCrossfadeEnergyThreshDBFS {
						energyHoldCount++
						if energyHoldCount >= m.cfg.AutoCrossfadeHoldFrames {
							energyTriggered = true
							m.log.Debug("crossfade energy trigger latched",
								"queue_item_id", item.QueueItemID,
								"rms_dbfs", fmt.Sprintf("%.1f", rmsDB),
								"threshold_dbfs", m.cfg.AutoCrossfadeEnergyThreshDBFS,
								"time_remaining_ms", timeRemaining,
							)
						}
					} else {
						energyHoldCount = 0
					}
				}
			}

			triggerType := "time"
			if energyTriggered {
				triggerType = "energy"
			}

			if (xStartMS > 0 && posMS >= xStartMS) || energyTriggered {
				if ni, ok := m.queueMgr.Peek(); ok && m.shouldCrossfade(item, ni) {
					ns, err := m.dec.Open(ctx, decoder.Source{
						Path: ni.Path, CueInMS: ni.CueInMS, CueOutMS: ni.CueOutMS,
					})
					if err == nil {
						m.queueMgr.Pop() // consume from queue
						xStream = ns
						xItem = ni
						xStarted = true
						xTotal = audio.DefaultFormat.FramesPerMs(int64(xfadeDurMS))
						if xTotal < 1 {
							xTotal = 1
						}
						m.log.Debug("crossfade started",
							"trigger", triggerType,
							"from", item.QueueItemID,
							"to", ni.QueueItemID,
							"duration_ms", xfadeDurMS,
						)
						m.evtBus.Publish(events.New(events.EvtCrossfadeStarted, events.CrossfadeStartedPayload{
							FromQueueItemID: item.QueueItemID,
							ToQueueItemID:   ni.QueueItemID,
							DurationMS:      int64(xfadeDurMS),
						}))
					}
				}
			}
		}

		// ---- read from main decoder ----
		n, err := stream.ReadFrames(ctx, buf)

		if n > 0 {
			samples := n * spf
			lastReadSamples = samples // used by energy-based crossfade trigger

			// ---- crossfade mixing ----
			if xStarted && xStream != nil {
				nn, _ := xStream.ReadFrames(ctx, nextBuf[:samples])

				progress := float32(xFrames) / float32(xTotal)
				if progress > 1.0 {
					progress = 1.0
				}
				mainGain := 1.0 - progress
				nextGain := progress

				for i := 0; i < samples; i++ {
					var ns float32
					if i < nn*spf {
						ns = nextBuf[i]
					}
					buf[i] = buf[i]*mainGain + ns*nextGain
				}
				xFrames += int64(n)
			}

			// ---- apply duck gain to main stream ----
			if gain := m.duckGain.Load(); gain < 100 {
				g := float32(gain) / 100.0
				for i := 0; i < samples; i++ {
					buf[i] *= g
				}
			}

			// ---- mix in overlay (hot button) at full gain ----
			// overlayLoop writes into m.overlay; we drain and add here so
			// only this goroutine ever calls m.out.Write, avoiding concurrent
			// writes that cause stuttering on real audio devices.
			m.overlay.pop(overlayBuf[:samples])
			for i := 0; i < samples; i++ {
				buf[i] += overlayBuf[i]
			}

			// ---- apply main volume gain ----
			applyGain(buf[:samples], m.stateMgr.MainVolume())

			// ---- write to output ----
			written, werr := m.out.Write(ctx, buf[:samples])
			m.framesTotal.Add(int64(written))
			if m.healthMon != nil {
				m.healthMon.Push(buf[:samples])
			}

			// ---- streaming tap ----
			// Send a copy to the streaming manager (non-blocking).
			// Frames are dropped when the tap channel is full to protect the audio loop.
			if tap := m.streamingTap; tap != nil {
				cp := make([]float32, samples)
				copy(cp, buf[:samples])
				select {
				case tap <- cp:
				default:
				}
			}

			if werr != nil {
				if ctx.Err() != nil {
					result = queue.ItemResultSkipped
					return
				}
				m.log.Error("output write failed", "queue_item_id", item.QueueItemID, "error", werr)
				m.evtBus.Publish(events.New(events.EvtOutputWriteFailed, events.OutputFailedPayload{
					Code: "OUTPUT_WRITE_FAILED", Message: werr.Error(),
				}))
				result = queue.ItemResultFailed
				return
			}

			// ---- check crossfade completion ----
			if xStarted && xFrames >= xTotal {
				// Hand off to next item — transfer ownership of xStream to caller.
				nextStream = xStream
				nextItem = xItem
				xFramesDone = xFrames
				xStream = nil // prevent defer from closing it
				result = queue.ItemResultPlayed
				return
			}
		}

		// ---- handle decoder EOF / error ----
		if err != nil {
			if errors.Is(err, io.EOF) {
				if xStarted && xStream != nil {
					// Main ended mid-crossfade — hand off to next item anyway.
					nextStream = xStream
					nextItem = xItem
					xFramesDone = xFrames
					xStream = nil
				}
				result = queue.ItemResultPlayed
			} else if ctx.Err() != nil {
				result = queue.ItemResultSkipped
			} else {
				m.log.Error("decoder read failed", "queue_item_id", item.QueueItemID, "error", err)
				m.evtBus.Publish(events.New(events.EvtDecoderError, events.DecoderErrorPayload{
					Code: "DECODER_READ_FAILED", Message: err.Error(),
					QueueItemID: item.QueueItemID, AssetID: item.AssetID, Recoverable: true,
				}))
				result = queue.ItemResultFailed
			}
			return
		}
	}
}

// --- Progress loop -----------------------------------------------------------

func (m *Manager) progressLoop(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Duration(m.cfg.ProgressIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.currentMu.RLock()
			cur := m.current
			startFrame := m.itemStartFrame
			m.currentMu.RUnlock()

			if cur == nil {
				continue
			}

			playedFrames := m.framesTotal.Load() - startFrame
			posMS := audio.DefaultFormat.MsFromFrames(playedFrames)
			durMS := cur.DurationMS

			var pct float64
			if durMS > 0 {
				pct = float64(posMS) / float64(durMS) * 100
				if pct > 100 {
					pct = 100
				}
			}
			remainMS := durMS - posMS
			if remainMS < 0 {
				remainMS = 0
			}

			m.stateMgr.UpdateProgress(posMS, pct)
			m.evtBus.Publish(events.New(events.EvtProgressChanged, events.ProgressChangedPayload{
				QueueItemID: cur.QueueItemID,
				PositionMS:  posMS,
				DurationMS:  durMS,
				Percent:     pct,
				RemainingMS: remainMS,
			}))

			// IntroCountdown: published while we're still before the vocal cue.
			if cur.IntroMS > 0 && posMS < cur.IntroMS {
				introRemaining := cur.IntroMS - posMS
				m.evtBus.Publish(events.New(events.EvtIntroCountdown, events.IntroCountdownPayload{
					QueueItemID: cur.QueueItemID,
					PositionMS:  posMS,
					IntroMS:     cur.IntroMS,
					RemainingMS: introRemaining,
				}))
			}
		}
	}
}

// --- Helpers -----------------------------------------------------------------

// xfadeDurationFor returns the crossfade duration in ms for an item, taking
// the item's explicit TransitionSpec into account.
func (m *Manager) xfadeDurationFor(item *queue.QueueItem) int {
	if item.Transition.Type == queue.TransitionCrossfade && item.Transition.DurationMS > 0 {
		return int(item.Transition.DurationMS)
	}
	if item.Transition.Type != "" && item.Transition.Type != queue.TransitionCrossfade {
		return 0 // explicit non-crossfade transition
	}
	return m.cfg.DefaultCrossfadeMS
}

// shouldCrossfade reports whether a crossfade from main to next is appropriate.
// Default rule (MVP): musicas → musicas.
func (m *Manager) shouldCrossfade(main, next *queue.QueueItem) bool {
	// In ASSIST mode the operator controls transitions; no auto-crossfade.
	if m.stateMgr.Snapshot().State == state.StateAssist {
		return false
	}
	return main.Type == queue.AssetTypeMusic && next.Type == queue.AssetTypeMusic
}

// waitAssistSignal blocks the sessionLoop until the operator sends a PLAY
// command (or the context is cancelled). It publishes EvtAssistWaiting before
// blocking so the UI can update. Returns false if ctx was cancelled.
func (m *Manager) waitAssistSignal(ctx context.Context) bool {
	m.assistMu.Lock()
	ch := m.assistResumeCh
	m.assistMu.Unlock()

	next, _ := m.queueMgr.Peek()
	nextTitle, nextType := "", ""
	if next != nil {
		nextTitle = next.Title
		nextType = string(next.Type)
	}
	m.evtBus.Publish(events.New(events.EvtAssistWaiting, events.AssistWaitingPayload{
		NextTitle: nextTitle,
		NextType:  nextType,
		QueueSize: m.queueMgr.Size(),
	}))

	if ch == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-ch:
		return true
	}
}

// forceResume unblocks a paused session so it can observe ctx cancellation.
func (m *Manager) forceResume() {
	m.pauseMu.Lock()
	defer m.pauseMu.Unlock()

	if m.paused {
		m.paused = false
		ch := m.resumeCh
		m.resumeCh = nil
		if ch != nil {
			close(ch)
		}
	}
}

// transitionToIdle returns the engine to IDLE if this session is still current.
func (m *Manager) transitionToIdle(gen int) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	if m.sessionGen != gen {
		return
	}
	m.sessionStop = nil
	m.sessionDone = nil

	prev := m.stateMgr.Snapshot().State
	if prev == state.StateError {
		return
	}
	m.stateMgr.SetState(state.StateIdle)
	m.evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: string(prev),
		To:   string(state.StateIdle),
		Mode: string(m.stateMgr.Snapshot().Mode),
	}))
}

// --- Duck gain ramp ----------------------------------------------------------

// rampDuckGain smoothly transitions m.duckGain from its current value to
// targetPct over durationMS milliseconds (10 ms steps).
// Respects ctx cancellation: if ctx is cancelled mid-ramp the target value is
// applied immediately so the gain never stays at an intermediate level.
func (m *Manager) rampDuckGain(ctx context.Context, targetPct int32, durationMS int) {
	const stepMs = 10
	steps := durationMS / stepMs
	if steps < 1 {
		m.duckGain.Store(targetPct)
		return
	}
	startPct := m.duckGain.Load()
	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			m.duckGain.Store(targetPct)
			return
		case <-time.After(time.Duration(stepMs) * time.Millisecond):
		}
		pct := startPct + int32(float64(targetPct-startPct)*float64(i)/float64(steps))
		m.duckGain.Store(pct)
	}
	m.duckGain.Store(targetPct)
}

// --- overlayMix --------------------------------------------------------------

// overlayMix is a thread-safe FIFO sample buffer used to mix hot button
// overlay audio into the main PCM stream before the output write.
// push is called by overlayLoop; pop is called by runPlayLoop (main loop only).
type overlayMix struct {
	mu   sync.Mutex
	data []float32
}

// push appends samples to the buffer.
// The overlayLoop throttles itself via buffered(), so this cap is a last-resort
// safety guard against unbounded growth (e.g. if the main loop is paused).
// 30 seconds at 48 kHz stereo = 48000 * 2 * 30 = 2 880 000 samples (~11 MB).
func (o *overlayMix) push(samples []float32) {
	const maxSamples = 48000 * 2 * 30 // 30 s safety cap
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.data)+len(samples) > maxSamples {
		return // safety: drop if main loop is stalled for more than 30 s
	}
	o.data = append(o.data, samples...)
}

// pop copies up to len(dst) samples from the buffer into dst.
// Positions not filled (buffer underrun) are zeroed so the caller can safely
// add the result to the main stream without introducing garbage.
func (o *overlayMix) pop(dst []float32) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := copy(dst, o.data)
	o.data = o.data[n:]
	for i := n; i < len(dst); i++ {
		dst[i] = 0
	}
}

// buffered returns the number of samples currently in the buffer.
func (o *overlayMix) buffered() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.data)
}

// reset discards all buffered samples.
// Called by hotButtonOverlay before starting a new overlay, so that leftover
// audio from a cancelled previous overlay does not bleed into the new one.
func (o *overlayMix) reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.data = o.data[:0]
}

// bufRMSdBFS returns the RMS level of a PCM float32 buffer in dBFS.
// Returns -144 for empty buffers or silence (all zeros).
func bufRMSdBFS(buf []float32) float64 {
	if len(buf) == 0 {
		return -144.0
	}
	var sumSq float64
	for _, s := range buf {
		v := float64(s)
		sumSq += v * v
	}
	rms := math.Sqrt(sumSq / float64(len(buf)))
	if rms <= 0 {
		return -144.0
	}
	return 20 * math.Log10(rms)
}
