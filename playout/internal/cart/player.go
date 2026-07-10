// Package cart implements the isolated cart player — a dedicated audio channel
// for hotkey-triggered playback. It is completely decoupled from the main
// playback pipeline and the preview/CUE channel.
//
// The cart player supports one active cart at a time. Triggering a new cart
// while one is already playing stops the previous one immediately ("replaced")
// and starts the new one without any gap.
package cart

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Waelson/radio-playout-engine/internal/audio/decoder"
	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/oklog/ulid/v2"
)

// AudioConfig carries audio format parameters for the cart player.
type AudioConfig struct {
	DeviceID     string // platform-specific device ID; empty = system default
	SampleRate   int
	Channels     int
	BufferFrames int
}

// Player is an isolated cart audio player for hotkey-triggered playback.
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
	vol        *atomic.Uint32 // float32 bits — read lock-free in the audio hot path
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
	vol *atomic.Uint32,
	log *slog.Logger,
) *Player {
	if log == nil {
		log = slog.Default()
	}
	if vol == nil {
		v := &atomic.Uint32{}
		v.Store(math.Float32bits(1.0))
		vol = v
	}
	return &Player{
		evtBus:     evtBus,
		dec:        dec,
		out:        out,
		deviceID:   audioCfg.DeviceID,
		sampleRate: audioCfg.SampleRate,
		channels:   audioCfg.Channels,
		bufFrames:  audioCfg.BufferFrames,
		vol:        vol,
		log:        log,
		status:     Status{State: StateIdle},
		cmdCh:      make(chan extCmd, 8),
		intCh:      make(chan intMsg, 64),
	}
}

// GetStatus returns a point-in-time snapshot of the cart player state.
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
	outCfg := output.OutputConfig{
		DeviceID:     p.deviceID,
		SampleRate:   p.sampleRate,
		Channels:     p.channels,
		BufferFrames: p.bufFrames,
	}
	if err := p.out.Open(ctx, outCfg); err != nil {
		p.log.Warn("cart: output device unavailable, falling back to null",
			"device", p.deviceID, "error", err)
		p.out = &output.NullOutput{}
		_ = p.out.Open(ctx, outCfg)
	}

	if err := p.out.Start(ctx); err != nil {
		p.log.Warn("cart: output start failed", "error", err)
	}
	if pa, ok := p.out.(interface{ PauseAudio() error }); ok {
		_ = pa.PauseAudio()
	}
	defer func() {
		_ = p.out.Stop(context.Background())
		_ = p.out.Close()
	}()

	type playSession struct {
		cancel context.CancelFunc
		cartID string
		path   string
		title  string
		artist string
		posMS  int64
		durMS  int64
	}

	var (
		state State = StateIdle
		sess  playSession
		gen   int64
	)

	startSession := func(cartID, path, title, artist string) {
		if sess.cancel != nil {
			sess.cancel()
		}
		gen++
		pCtx, pCancel := context.WithCancel(ctx)
		sess = playSession{
			cancel: pCancel,
			cartID: cartID,
			path:   path,
			title:  title,
			artist: artist,
		}
		state = StatePlaying
		p.setStatus(Status{
			State:  state,
			CartID: cartID,
			Path:   path,
			Title:  title,
			Artist: artist,
		})
		go p.loop(pCtx, gen, cartID, path)
	}

	stopSession := func(reason string) {
		if state == StateIdle {
			return
		}
		if sess.cancel != nil {
			sess.cancel()
			sess.cancel = nil
		}
		gen++
		prevCartID := sess.cartID
		sess = playSession{}
		state = StateIdle
		p.setStatus(Status{State: StateIdle})
		p.evtBus.Publish(events.New(events.EvtCartStopped, events.CartStoppedPayload{
			CartID: prevCartID,
			Reason: reason,
		}))
	}

	for {
		select {
		case <-ctx.Done():
			if state != StateIdle {
				if sess.cancel != nil {
					sess.cancel()
				}
			}
			return

		case cmd := <-p.cmdCh:
			switch cmd.kind {

			case extPlay:
				// If already playing, stop with reason "replaced" before starting new.
				if state == StatePlaying {
					stopSession("replaced")
				}
				cartID := "cart_" + ulid.Make().String()
				startSession(cartID, cmd.path, cmd.title, cmd.artist)
				p.evtBus.Publish(events.New(events.EvtCartStarted, events.CartStartedPayload{
					CartID:     cartID,
					Path:       sess.path,
					Title:      sess.title,
					Artist:     sess.artist,
					DurationMS: sess.durMS,
				}))

			case extStop:
				stopSession("manual")
			}

		case msg := <-p.intCh:
			if msg.gen != gen {
				break
			}
			switch msg.kind {

			case intDuration:
				sess.durMS = msg.posMS // posMS field reused to carry durMS
				p.setStatus(Status{
					State:      state,
					CartID:     sess.cartID,
					Path:       sess.path,
					Title:      sess.title,
					Artist:     sess.artist,
					PositionMS: sess.posMS,
					DurationMS: sess.durMS,
				})
				// Re-publish CartStarted with the now-known duration.
				p.evtBus.Publish(events.New(events.EvtCartStarted, events.CartStartedPayload{
					CartID:     sess.cartID,
					Path:       sess.path,
					Title:      sess.title,
					Artist:     sess.artist,
					DurationMS: sess.durMS,
				}))

			case intProgress:
				sess.posMS = msg.posMS
				p.setStatus(Status{
					State:      state,
					CartID:     sess.cartID,
					Path:       sess.path,
					Title:      sess.title,
					Artist:     sess.artist,
					PositionMS: sess.posMS,
					DurationMS: sess.durMS,
				})
				p.evtBus.Publish(events.New(events.EvtCartProgress, events.CartProgressPayload{
					CartID:     sess.cartID,
					PositionMS: sess.posMS,
					DurationMS: sess.durMS,
				}))

			case intEnded:
				if msg.err != nil {
					p.log.Warn("cart: playback ended with error",
						"error", msg.err, "path", sess.path, "cart_id", sess.cartID)
				}
				reason := "finished"
				if msg.err != nil {
					reason = "error"
				}
				gen++
				prevCartID := sess.cartID
				sess = playSession{}
				state = StateIdle
				p.setStatus(Status{State: StateIdle})
				p.evtBus.Publish(events.New(events.EvtCartStopped, events.CartStoppedPayload{
					CartID: prevCartID,
					Reason: reason,
				}))
			}
		}
	}
}

// loop is the playback goroutine. It decodes path and writes PCM to the output
// device, sending progress/ended messages to intCh.
func (p *Player) loop(ctx context.Context, gen int64, cartID, path string) {
	send := func(msg intMsg) {
		msg.gen = gen
		select {
		case p.intCh <- msg:
		default:
		}
	}

	// Probe duration in the background so the event loop is never blocked.
	go func() {
		dur := probeDuration(path)
		if dur > 0 {
			send(intMsg{kind: intDuration, posMS: dur})
		}
	}()

	stream, err := p.dec.Open(ctx, decoder.Source{Path: path})
	if err != nil {
		if ctx.Err() == nil {
			send(intMsg{kind: intEnded, err: fmt.Errorf("cart: decoder open: %w", err)})
		}
		return
	}
	defer stream.Close()

	if r, ok := p.out.(interface{ ResumeAudio() error }); ok {
		if err := r.ResumeAudio(); err != nil {
			if ctx.Err() == nil {
				send(intMsg{kind: intEnded, err: fmt.Errorf("cart: output resume: %w", err)})
			}
			return
		}
	} else {
		if err := p.out.Start(ctx); err != nil {
			if ctx.Err() == nil {
				send(intMsg{kind: intEnded, err: fmt.Errorf("cart: output start: %w", err)})
			}
			return
		}
	}
	defer func() {
		if pa, ok := p.out.(interface{ PauseAudio() error }); ok {
			_ = pa.PauseAudio()
		} else {
			_ = p.out.Stop(context.Background()) //nolint:contextcheck
		}
	}()

	_ = cartID // cartID carried in parent scope for event payloads
	buf := make([]float32, p.bufFrames*p.channels)
	posMS := int64(0)
	lastProgressMS := int64(-101)

	for {
		if ctx.Err() != nil {
			return
		}

		n, readErr := stream.ReadFrames(ctx, buf)
		if n > 0 {
			gain := math.Float32frombits(p.vol.Load())
			if gain != 1.0 {
				for i := range buf[:n*p.channels] {
					buf[i] *= gain
				}
			}
			if _, writeErr := p.out.Write(ctx, buf[:n*p.channels]); writeErr != nil {
				if ctx.Err() == nil {
					send(intMsg{kind: intEnded, err: fmt.Errorf("cart: output write: %w", writeErr)})
				}
				return
			}
			posMS += int64(n) * 1000 / int64(p.sampleRate)
		}

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
				send(intMsg{kind: intEnded, err: fmt.Errorf("cart: decoder read: %w", readErr)})
			}
			return
		}
	}
}

// --- Dispatcher handler funcs ------------------------------------------------

// HandlePlay handles CmdCartPlay.
func (p *Player) HandlePlay(_ context.Context, cmd commands.Command) error {
	payload, ok := cmd.Payload.(commands.CartPlayPayload)
	if !ok {
		return fmt.Errorf("cart: HandlePlay: unexpected payload type %T", cmd.Payload)
	}
	p.send(extCmd{kind: extPlay, path: payload.Path, title: payload.Title, artist: payload.Artist})
	return nil
}

// HandleStop handles CmdCartStop.
func (p *Player) HandleStop(_ context.Context, _ commands.Command) error {
	p.send(extCmd{kind: extStop})
	return nil
}

func (p *Player) send(cmd extCmd) {
	select {
	case p.cmdCh <- cmd:
	default:
		p.log.Warn("cart: command dropped (channel full)", "kind", cmd.kind)
	}
}

// --- Internal types ----------------------------------------------------------

type extCmdKind int

const (
	extPlay extCmdKind = iota
	extStop
)

type extCmd struct {
	kind   extCmdKind
	path   string
	title  string
	artist string
}

type intMsgKind int

const (
	intDuration intMsgKind = iota
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
