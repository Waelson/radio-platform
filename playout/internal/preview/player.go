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
func (p *Player) Run(ctx context.Context) {
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
	startSession := func(path string, seekMS, durMS int64) {
		if sess.cancel != nil {
			sess.cancel()
		}
		if durMS == 0 {
			durMS = probeDuration(path)
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
func (p *Player) loop(ctx context.Context, gen int64, path string, startMS int64) {
	send := func(msg intMsg) {
		msg.gen = gen
		select {
		case p.intCh <- msg:
		default:
		}
	}

	stream, err := p.dec.Open(ctx, decoder.Source{Path: path, CueInMS: startMS})
	if err != nil {
		if ctx.Err() == nil {
			send(intMsg{kind: intEnded, err: fmt.Errorf("decoder open: %w", err)})
		}
		return
	}
	defer stream.Close()

	outCfg := output.OutputConfig{
		DeviceID:     "",
		SampleRate:   p.sampleRate,
		Channels:     p.channels,
		BufferFrames: p.bufFrames,
	}
	if err := p.out.Open(ctx, outCfg); err != nil {
		if ctx.Err() == nil {
			send(intMsg{kind: intEnded, err: fmt.Errorf("output open: %w", err)})
		}
		return
	}
	defer p.out.Close()

	if err := p.out.Start(ctx); err != nil {
		if ctx.Err() == nil {
			send(intMsg{kind: intEnded, err: fmt.Errorf("output start: %w", err)})
		}
		return
	}
	defer p.out.Stop(context.Background()) //nolint:contextcheck

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
	intProgress intMsgKind = iota
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
