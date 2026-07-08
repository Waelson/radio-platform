package cue

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/preview"
)

// Proxy manages a CUE player subprocess and exposes the same handler interface
// as preview.Player. Commands are forwarded to the subprocess via stdin;
// subprocess events arrive via stdout and are re-published on the main engine's
// EventBus so that WebSocket clients and the state machine see identical events
// regardless of whether the player is embedded or isolated.
//
// Orphan prevention operates in three layers:
//  1. stdin EOF: when the main engine dies (any cause), the OS closes the
//     stdin pipe's write-end; the subprocess detects EOF and exits immediately.
//  2. Pdeathsig SIGTERM (Linux): the kernel kills the child when the parent dies.
//  3. SIGKILL fallback in gracefulStop: if the subprocess does not exit within
//     5 seconds after receiving {"cmd":"quit"}, it is force-killed.
type Proxy struct {
	evtBus    *events.Bus
	spawnArgs []string // CLI args forwarded verbatim to subprocess (e.g. --config=path)
	log       *slog.Logger

	mu     sync.RWMutex
	status preview.Status
	proc   *os.Process // nil when subprocess is not running

	encMu sync.Mutex
	enc   *json.Encoder // writes to subprocess stdin; nil when not running
	stdin io.WriteCloser
}

// NewProxy creates a Proxy. spawnArgs should be the filtered CLI args the main
// engine was started with (they are forwarded to the subprocess so it loads the
// same config file, e.g. --config=/etc/playout.yaml).
func NewProxy(evtBus *events.Bus, spawnArgs []string, log *slog.Logger) *Proxy {
	if log == nil {
		log = slog.Default()
	}
	return &Proxy{
		evtBus:    evtBus,
		spawnArgs: spawnArgs,
		log:       log,
		status:    preview.Status{State: preview.StateIdle},
	}
}

// GetStatus returns the current preview state derived from subprocess events.
// Safe for concurrent use.
func (p *Proxy) GetStatus() preview.Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// Run spawns the CUE subprocess and blocks until ctx is cancelled.
// Must be called in a dedicated goroutine (same contract as preview.Player.Run).
func (p *Proxy) Run(ctx context.Context) {
	done := p.spawn()
	select {
	case <-ctx.Done():
		p.gracefulStop(done)
	case <-done:
		// Subprocess crashed or exited before ctx was cancelled.
		p.log.Warn("cue proxy: subprocess exited unexpectedly")
		p.mu.Lock()
		p.status = preview.Status{State: preview.StateIdle}
		p.mu.Unlock()
	}
}

// spawn launches the CUE subprocess and returns a channel that is closed when
// the subprocess exits (after cmd.Wait returns).
func (p *Proxy) spawn() <-chan struct{} {
	done := make(chan struct{})

	self, err := os.Executable()
	if err != nil {
		p.log.Error("cue proxy: locate self binary", "error", err)
		close(done)
		return done
	}

	// Build subprocess args: mode flag + original engine args (includes --config= etc.)
	args := make([]string, 0, len(p.spawnArgs)+1)
	args = append(args, "--mode=cue-player")
	args = append(args, p.spawnArgs...)

	cmd := exec.Command(self, args...)
	cmd.Env = expandedEnv() // ensure ffmpeg/ffprobe are on PATH in .app bundles
	cmd.Stderr = os.Stderr  // CUE subprocess logs appear alongside engine logs

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		p.log.Error("cue proxy: create stdin pipe", "error", err)
		close(done)
		return done
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		p.log.Error("cue proxy: create stdout pipe", "error", err)
		close(done)
		return done
	}

	setCueProcAttr(cmd) // Setpgid (+ Pdeathsig on Linux)

	if err := cmd.Start(); err != nil {
		p.log.Error("cue proxy: spawn subprocess", "error", err)
		close(done)
		return done
	}

	p.log.Info("cue proxy: subprocess started", "pid", cmd.Process.Pid)

	p.mu.Lock()
	p.proc = cmd.Process
	p.mu.Unlock()

	p.encMu.Lock()
	p.stdin = stdinPipe
	p.enc = json.NewEncoder(stdinPipe)
	p.encMu.Unlock()

	go func() {
		defer close(done)
		p.readEvents(stdoutPipe)     // blocks until stdout closes
		_ = stdinPipe.Close()        // close our write-end if not already closed
		_ = cmd.Wait()               // reap zombie
		p.mu.Lock()
		p.proc = nil
		p.mu.Unlock()
		p.log.Info("cue proxy: subprocess stopped")
	}()

	return done
}

// gracefulStop sends {"cmd":"quit"}, closes stdin (EOF signal), then waits up
// to 5 seconds for the subprocess to exit before force-killing it.
func (p *Proxy) gracefulStop(done <-chan struct{}) {
	p.encMu.Lock()
	if p.enc != nil {
		_ = p.enc.Encode(subCmd{Cmd: "quit"})
	}
	if p.stdin != nil {
		_ = p.stdin.Close()
		p.stdin = nil
		p.enc = nil
	}
	p.encMu.Unlock()

	select {
	case <-done:
		// clean exit
	case <-time.After(5 * time.Second):
		p.mu.RLock()
		proc := p.proc
		p.mu.RUnlock()
		if proc != nil {
			p.log.Warn("cue proxy: subprocess did not exit in time, force-killing")
			_ = proc.Kill()
		}
		<-done
	}
}

// readEvents reads newline-delimited JSON events from the subprocess stdout
// and dispatches them to the main engine's EventBus. Blocks until stdout
// closes (subprocess exited or pipe broken).
func (p *Proxy) readEvents(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		var se subEvt
		if err := json.Unmarshal(sc.Bytes(), &se); err != nil {
			continue
		}
		p.dispatchSubEvt(se)
	}
}

// dispatchSubEvt updates local status and re-publishes the event on the
// main engine's EventBus so all downstream consumers (WebSocket, metrics)
// receive identical events regardless of player mode.
func (p *Proxy) dispatchSubEvt(se subEvt) {
	switch se.Event {
	case "ready":
		p.log.Debug("cue proxy: subprocess ready")

	case "started":
		p.mu.Lock()
		p.status = preview.Status{
			State:      preview.StatePlaying,
			Path:       se.Path,
			PositionMS: se.SeekMS,
			DurationMS: se.DurationMS,
		}
		p.mu.Unlock()
		p.evtBus.Publish(events.New(events.EvtPreviewStarted, events.PreviewStartedPayload{
			Path: se.Path, DurationMS: se.DurationMS, SeekMS: se.SeekMS,
		}))

	case "progress":
		p.mu.Lock()
		p.status.PositionMS = se.PositionMS
		p.status.DurationMS = se.DurationMS
		p.mu.Unlock()
		p.evtBus.Publish(events.New(events.EvtPreviewProgress, events.PreviewProgressPayload{
			PositionMS: se.PositionMS, DurationMS: se.DurationMS,
		}))

	case "paused":
		p.mu.Lock()
		p.status.State = preview.StatePaused
		p.status.PositionMS = se.PositionMS
		p.status.DurationMS = se.DurationMS
		p.mu.Unlock()
		p.evtBus.Publish(events.New(events.EvtPreviewPaused, events.PreviewPausedPayload{
			PositionMS: se.PositionMS, DurationMS: se.DurationMS,
		}))

	case "resumed":
		p.mu.Lock()
		p.status.State = preview.StatePlaying
		p.status.PositionMS = se.PositionMS
		p.status.DurationMS = se.DurationMS
		p.mu.Unlock()
		p.evtBus.Publish(events.New(events.EvtPreviewResumed, events.PreviewResumedPayload{
			PositionMS: se.PositionMS, DurationMS: se.DurationMS,
		}))

	case "stopped":
		p.mu.Lock()
		p.status = preview.Status{State: preview.StateIdle}
		p.mu.Unlock()
		p.evtBus.Publish(events.New(events.EvtPreviewStopped, events.PreviewStoppedPayload{
			Reason: se.Reason, PositionMS: se.PositionMS,
		}))

	case "seeked":
		p.mu.Lock()
		p.status.PositionMS = se.PositionMS
		p.status.DurationMS = se.DurationMS
		p.mu.Unlock()
		p.evtBus.Publish(events.New(events.EvtPreviewSeeked, events.PreviewSeekedPayload{
			PositionMS: se.PositionMS, DurationMS: se.DurationMS,
		}))

	case "error":
		p.log.Warn("cue proxy: subprocess error", "message", se.Message)
	}
}

// sendCmd encodes sc as JSON and writes it to the subprocess stdin.
func (p *Proxy) sendCmd(sc subCmd) error {
	p.encMu.Lock()
	defer p.encMu.Unlock()
	if p.enc == nil {
		return fmt.Errorf("cue proxy: subprocess not running")
	}
	return p.enc.Encode(sc)
}

// --- Dispatcher handlers — identical contract to preview.Player --------------

// HandlePlay handles CmdPreviewPlay.
func (p *Proxy) HandlePlay(_ context.Context, cmd commands.Command) error {
	payload, ok := cmd.Payload.(commands.PreviewPlayPayload)
	if !ok {
		return fmt.Errorf("cue proxy: HandlePlay: unexpected payload type %T", cmd.Payload)
	}
	return p.sendCmd(subCmd{Cmd: "play", Path: payload.Path, SeekMS: payload.SeekMS})
}

// HandlePause handles CmdPreviewPause.
func (p *Proxy) HandlePause(_ context.Context, _ commands.Command) error {
	return p.sendCmd(subCmd{Cmd: "pause"})
}

// HandleResume handles CmdPreviewResume.
func (p *Proxy) HandleResume(_ context.Context, _ commands.Command) error {
	return p.sendCmd(subCmd{Cmd: "resume"})
}

// HandleStop handles CmdPreviewStop.
func (p *Proxy) HandleStop(_ context.Context, _ commands.Command) error {
	return p.sendCmd(subCmd{Cmd: "stop"})
}

// HandleSeek handles CmdPreviewSeek.
func (p *Proxy) HandleSeek(_ context.Context, cmd commands.Command) error {
	payload, ok := cmd.Payload.(commands.PreviewSeekPayload)
	if !ok {
		return fmt.Errorf("cue proxy: HandleSeek: unexpected payload type %T", cmd.Payload)
	}
	return p.sendCmd(subCmd{Cmd: "seek", PositionMS: payload.PositionMS})
}
