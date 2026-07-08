package preview

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/Waelson/radio-playout-engine/internal/audio/decoder"
	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
)

// AudioConfig carries audio format parameters for the preview player.
type AudioConfig struct {
	DeviceID     string // platform-specific device ID; empty = system default
	SampleRate   int
	Channels     int
	BufferFrames int
}

// Player is an isolated preview (cue) audio player.
// It manages its own state machine and communicates via the event bus.
// All public Handle* methods are safe for concurrent use and match
// the dispatcher.HandlerFunc signature.
type Player struct {
	evtBus     *events.Bus
	dec        decoder.Decoder
	out        output.OutputDevice
	deviceID   string
	sampleRate int
	channels   int
	bufFrames  int
	log        *slog.Logger

	mu     sync.RWMutex
	status Status

	cmdCh chan extCmd // external commands from dispatcher handlers
	intCh chan intMsg // internal messages from the playback goroutine
}

// New creates a Player. out must not yet be opened — the player opens
// and closes it around each playback session.
func New(
	evtBus *events.Bus,
	dec decoder.Decoder,
	out output.OutputDevice,
	audioCfg AudioConfig,
	log *slog.Logger,
) *Player {
	if log == nil {
		log = slog.Default()
	}
	return &Player{
		evtBus:     evtBus,
		dec:        dec,
		out:        out,
		deviceID:   audioCfg.DeviceID,
		sampleRate: audioCfg.SampleRate,
		channels:   audioCfg.Channels,
		bufFrames:  audioCfg.BufferFrames,
		log:        log,
		status:     Status{State: StateIdle},
		cmdCh:      make(chan extCmd, 8),
		intCh:      make(chan intMsg, 64),
	}
}

// GetStatus returns a snapshot of the current preview state.
// Safe for concurrent use.
func (p *Player) GetStatus() Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *Player) setStatus(s Status) {
	p.mu.Lock()
	p.status = s
	p.mu.Unlock()
}

// Run is the player event loop. Must be called in a dedicated goroutine.
// It returns when ctx is cancelled.
//
// The output device is opened once here and kept open for the lifetime of the
// engine, so that repeated preview plays do not trigger device
// initialisation (particularly expensive for Bluetooth / A2DP devices).
// Only Start/Stop are called around individual playback sessions.
func (p *Player) Run(ctx context.Context) {
	// Pre-open the output device once. If the configured device is
	// unavailable, fall back to NullOutput so the engine keeps running.
	outCfg := output.OutputConfig{
		DeviceID:     p.deviceID,
		SampleRate:   p.sampleRate,
		Channels:     p.channels,
		BufferFrames: p.bufFrames,
	}
	if err := p.out.Open(ctx, outCfg); err != nil {
		p.log.Warn("preview: output device unavailable, falling back to null",
			"device", p.deviceID, "error", err)
		p.out = &output.NullOutput{}
		_ = p.out.Open(ctx, outCfg)
	}

	// Start the queue once so the device (e.g. BT/A2DP) enters active state,
	// then immediately pause it. Keeping the device in paused state (rather
	// than stopped) means ResumeAudio between sessions is much lighter than
	// a full Start — avoids the HAL notification storm that causes the break.
	if err := p.out.Start(ctx); err != nil {
		p.log.Warn("preview: output start failed", "error", err)
	}
	if pa, ok := p.out.(interface{ PauseAudio() error }); ok {
		_ = pa.PauseAudio()
	}
	defer func() {
		_ = p.out.Stop(context.Background())
		_ = p.out.Close()
	}()

	// playSession holds the mutable state of the current playback session.
	type playSession struct {
		cancel context.CancelFunc
		path   string
		posMS  int64
		durMS  int64
	}

	var (
		state State = StateIdle
		sess  playSession
		gen   int64 // incremented on every new/cancelled session to discard stale msgs
	)

	// startSession cancels any existing session and starts a new one.
	// Duration probing is deferred to the loop goroutine (Phase 3) so this
	// function returns immediately without blocking the event loop.
	startSession := func(path string, seekMS, durMS int64) {
		if sess.cancel != nil {
			sess.cancel()
		}
		gen++
		pCtx, pCancel := context.WithCancel(ctx)
		sess = playSession{
			cancel: pCancel,
			path:   path,
			posMS:  seekMS,
			durMS:  durMS,
		}
		state = StatePlaying
		p.setStatus(Status{State: state, Path: path, PositionMS: seekMS, DurationMS: durMS})
		go p.loop(pCtx, gen, path, seekMS)
	}

	// stopSession cancels the current playback goroutine (if any).
	stopSession := func() {
		if sess.cancel != nil {
			sess.cancel()
			sess.cancel = nil
		}
		gen++ // invalidate any in-flight messages from the old goroutine
	}

	for {
		select {
		case <-ctx.Done():
			stopSession()
			return

		case cmd := <-p.cmdCh:
			switch cmd.kind {

			case extPlay:
				startSession(cmd.path, cmd.seekMS, 0)
				p.evtBus.Publish(events.New(events.EvtPreviewStarted, events.PreviewStartedPayload{
					Path:       sess.path,
					DurationMS: sess.durMS,
					SeekMS:     sess.posMS,
				}))

			case extPause:
				if state != StatePlaying {
					break
				}
				stopSession()
				state = StatePaused
				p.setStatus(Status{State: state, Path: sess.path, PositionMS: sess.posMS, DurationMS: sess.durMS})
				p.evtBus.Publish(events.New(events.EvtPreviewPaused, events.PreviewPausedPayload{
					PositionMS: sess.posMS,
					DurationMS: sess.durMS,
				}))

			case extResume:
				if state != StatePaused {
					break
				}
				startSession(sess.path, sess.posMS, sess.durMS)
				p.evtBus.Publish(events.New(events.EvtPreviewResumed, events.PreviewResumedPayload{
					PositionMS: sess.posMS,
					DurationMS: sess.durMS,
				}))

			case extStop:
				stopSession()
				prevPos := sess.posMS
				sess = playSession{}
				state = StateIdle
				p.setStatus(Status{State: state})
				p.evtBus.Publish(events.New(events.EvtPreviewStopped, events.PreviewStoppedPayload{
					Reason:     "stop",
					PositionMS: prevPos,
				}))

			case extSeek:
				if state == StateIdle {
					break
				}
				startSession(sess.path, cmd.seekMS, sess.durMS)
				p.evtBus.Publish(events.New(events.EvtPreviewSeeked, events.PreviewSeekedPayload{
					PositionMS: cmd.seekMS,
					DurationMS: sess.durMS,
				}))
			}

		case msg := <-p.intCh:
			// Discard messages from previous/cancelled sessions.
			if msg.gen != gen {
				break
			}
			switch msg.kind {

			case intDuration:
				// Duration arrived from the background probe in loop().
				sess.durMS = msg.posMS // posMS field reused to carry durMS
				p.setStatus(Status{State: state, Path: sess.path, PositionMS: sess.posMS, DurationMS: sess.durMS})

			case intProgress:
				sess.posMS = msg.posMS
				p.setStatus(Status{State: state, Path: sess.path, PositionMS: sess.posMS, DurationMS: sess.durMS})
				p.evtBus.Publish(events.New(events.EvtPreviewProgress, events.PreviewProgressPayload{
					PositionMS: sess.posMS,
					DurationMS: sess.durMS,
				}))

			case intEnded:
				stopSession()
				reason := "end"
				if msg.err != nil {
					reason = "error"
					p.log.Warn("preview: playback ended with error", "error", msg.err, "path", sess.path)
				}
				prevPos := sess.posMS
				sess = playSession{}
				state = StateIdle
				p.setStatus(Status{State: state})
				p.evtBus.Publish(events.New(events.EvtPreviewStopped, events.PreviewStoppedPayload{
					Reason:     reason,
					PositionMS: prevPos,
				}))
			}
		}
	}
}

// loop is the playback goroutine. It opens the decoder at startMS, writes PCM
// to the output device, and sends progress/ended messages to intCh.
// It returns when ctx is cancelled or the stream reaches EOF.
//
// The output device is already open (opened once in Run). This function only
// calls Start/Stop around the session, never Open/Close.
func (p *Player) loop(ctx context.Context, gen int64, path string, startMS int64) {
	send := func(msg intMsg) {
		msg.gen = gen
		select {
		case p.intCh <- msg:
		default:
		}
	}

	// Phase 3: probe duration in a background goroutine so the event loop
	// is never blocked waiting for ffprobe to finish.
	go func() {
		dur := probeDuration(path)
		if dur > 0 {
			send(intMsg{kind: intDuration, posMS: dur}) // posMS field carries durMS
		}
	}()

	stream, err := p.dec.Open(ctx, decoder.Source{Path: path, CueInMS: startMS})
	if err != nil {
		if ctx.Err() == nil {
			send(intMsg{kind: intEnded, err: fmt.Errorf("decoder open: %w", err)})
		}
		return
	}
	defer stream.Close()

	// Phase 2: device is already open and in paused state (from Run).
	// Use ResumeAudio if available (lighter than Start for BT/A2DP — avoids
	// re-establishing the A2DP stream). Fall back to Start otherwise.
	if r, ok := p.out.(interface{ ResumeAudio() error }); ok {
		if err := r.ResumeAudio(); err != nil {
			if ctx.Err() == nil {
				send(intMsg{kind: intEnded, err: fmt.Errorf("output resume: %w", err)})
			}
			return
		}
	} else {
		if err := p.out.Start(ctx); err != nil {
			if ctx.Err() == nil {
				send(intMsg{kind: intEnded, err: fmt.Errorf("output start: %w", err)})
			}
			return
		}
	}
	// At session end: pause (not stop) so the device stays warm for the next session.
	defer func() {
		if pa, ok := p.out.(interface{ PauseAudio() error }); ok {
			_ = pa.PauseAudio()
		} else {
			_ = p.out.Stop(context.Background()) //nolint:contextcheck
		}
	}()

	buf := make([]float32, p.bufFrames*p.channels)
	posMS := startMS
	lastProgressMS := startMS - 101 // ensure a progress event fires on the first iteration

	for {
		if ctx.Err() != nil {
			return
		}

		n, readErr := stream.ReadFrames(ctx, buf)
		if n > 0 {
			if _, writeErr := p.out.Write(ctx, buf[:n*p.channels]); writeErr != nil {
				if ctx.Err() == nil {
					send(intMsg{kind: intEnded, err: fmt.Errorf("output write: %w", writeErr)})
				}
				return
			}
			posMS += int64(n) * 1000 / int64(p.sampleRate)
		}

		// Publish progress at ~100 ms intervals.
		if posMS-lastProgressMS >= 100 {
			send(intMsg{kind: intProgress, posMS: posMS})
			lastProgressMS = posMS
		}

		if readErr == io.EOF {
			send(intMsg{kind: intEnded})
			return
		}
		if readErr != nil {
			if ctx.Err() == nil {
				send(intMsg{kind: intEnded, err: fmt.Errorf("decoder read: %w", readErr)})
			}
			return
		}
	}
}

// --- Dispatcher handler funcs ------------------------------------------------

// HandlePlay handles CmdPreviewPlay.
func (p *Player) HandlePlay(_ context.Context, cmd commands.Command) error {
	payload, ok := cmd.Payload.(commands.PreviewPlayPayload)
	if !ok {
		return fmt.Errorf("preview: HandlePlay: unexpected payload type %T", cmd.Payload)
	}
	p.send(extCmd{kind: extPlay, path: payload.Path, seekMS: payload.SeekMS})
	return nil
}

// HandlePause handles CmdPreviewPause.
func (p *Player) HandlePause(_ context.Context, _ commands.Command) error {
	p.send(extCmd{kind: extPause})
	return nil
}

// HandleResume handles CmdPreviewResume.
func (p *Player) HandleResume(_ context.Context, _ commands.Command) error {
	p.send(extCmd{kind: extResume})
	return nil
}

// HandleStop handles CmdPreviewStop.
func (p *Player) HandleStop(_ context.Context, _ commands.Command) error {
	p.send(extCmd{kind: extStop})
	return nil
}

// HandleSeek handles CmdPreviewSeek.
func (p *Player) HandleSeek(_ context.Context, cmd commands.Command) error {
	payload, ok := cmd.Payload.(commands.PreviewSeekPayload)
	if !ok {
		return fmt.Errorf("preview: HandleSeek: unexpected payload type %T", cmd.Payload)
	}
	p.send(extCmd{kind: extSeek, seekMS: payload.PositionMS})
	return nil
}

func (p *Player) send(cmd extCmd) {
	select {
	case p.cmdCh <- cmd:
	default:
		p.log.Warn("preview: command dropped (channel full)", "kind", cmd.kind)
	}
}

// --- Internal types ----------------------------------------------------------

type extCmdKind int

const (
	extPlay   extCmdKind = iota
	extPause
	extResume
	extStop
	extSeek
)

type extCmd struct {
	kind   extCmdKind
	path   string
	seekMS int64
}

type intMsgKind int

const (
	intDuration intMsgKind = iota // duration probed in background; posMS field carries durMS
	intProgress
	intEnded
)

type intMsg struct {
	gen   int64
	kind  intMsgKind
	posMS int64
	err   error
}

// probeDuration probes the duration of path using ffprobe.
// Returns 0 when ffprobe is unavailable or the file cannot be probed.
func probeDuration(path string) int64 {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		path,
	).Output()
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(out))
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(f * 1000)
}
