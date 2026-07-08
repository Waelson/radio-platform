package cue

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/Waelson/radio-playout-engine/internal/audio/decoder"
	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/preview"
)

// RunCuePlayer is the entry point for --mode=cue-player.
// It builds an isolated preview.Player using the provided output device,
// forwards its events to stdout as newline-delimited JSON, and reads
// commands from stdin. Blocks until stdin closes (parent died) or
// {"cmd":"quit"} is received.
//
// Orphan prevention: when the main engine dies for any reason, the OS closes
// the write-end of the stdin pipe, causing bufio.Scanner.Scan() to return
// false (EOF). RunCuePlayer then returns and the process exits cleanly.
func RunCuePlayer(out output.OutputDevice, audioCfg preview.AudioConfig, log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Local EventBus — only preview events are published on it in this process.
	evtBus := events.NewBus(nil)
	dec := decoder.NewFFmpegDecoder(log)
	player := preview.New(evtBus, dec, out, audioCfg, log)

	// Subscribe to preview events and forward them to stdout.
	evtCh, unsub := evtBus.Subscribe(256)
	defer unsub()
	go forwardEventsToStdout(ctx, evtCh)

	// Run the player state machine in a background goroutine.
	go player.Run(ctx)

	// Announce readiness; the proxy waits for this before sending commands.
	writeStdoutEvt(subEvt{Event: "ready"})

	// Read and dispatch commands from stdin.
	// Scan returns false on EOF (parent died) or pipe error.
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		var cmd subCmd
		if err := json.Unmarshal(sc.Bytes(), &cmd); err != nil {
			continue
		}
		if cmd.Cmd == "quit" {
			return
		}
		dispatchToPlayer(ctx, player, cmd)
	}
	// stdin closed — parent process died; exit immediately.
}

// forwardEventsToStdout converts EventBus preview events into subEvt JSON and
// writes them to stdout. Runs until ctx is cancelled or the channel closes.
func forwardEventsToStdout(ctx context.Context, ch <-chan events.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			var se subEvt
			switch evt.Type {
			case events.EvtPreviewStarted:
				p := evt.Payload.(events.PreviewStartedPayload)
				se = subEvt{Event: "started", Path: p.Path, DurationMS: p.DurationMS, SeekMS: p.SeekMS}
			case events.EvtPreviewPaused:
				p := evt.Payload.(events.PreviewPausedPayload)
				se = subEvt{Event: "paused", PositionMS: p.PositionMS, DurationMS: p.DurationMS}
			case events.EvtPreviewResumed:
				p := evt.Payload.(events.PreviewResumedPayload)
				se = subEvt{Event: "resumed", PositionMS: p.PositionMS, DurationMS: p.DurationMS}
			case events.EvtPreviewStopped:
				p := evt.Payload.(events.PreviewStoppedPayload)
				se = subEvt{Event: "stopped", Reason: p.Reason, PositionMS: p.PositionMS}
			case events.EvtPreviewProgress:
				p := evt.Payload.(events.PreviewProgressPayload)
				se = subEvt{Event: "progress", PositionMS: p.PositionMS, DurationMS: p.DurationMS}
			case events.EvtPreviewSeeked:
				p := evt.Payload.(events.PreviewSeekedPayload)
				se = subEvt{Event: "seeked", PositionMS: p.PositionMS, DurationMS: p.DurationMS}
			default:
				continue
			}
			writeStdoutEvt(se)
		}
	}
}

// writeStdoutEvt serialises e as JSON and writes it to stdout followed by \n.
func writeStdoutEvt(e subEvt) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = os.Stdout.Write(data)
}

// dispatchToPlayer translates a subCmd into a preview.Player handler call.
func dispatchToPlayer(ctx context.Context, player *preview.Player, cmd subCmd) {
	switch cmd.Cmd {
	case "play":
		_ = player.HandlePlay(ctx, commands.New(commands.CmdPreviewPlay,
			commands.PreviewPlayPayload{Path: cmd.Path, SeekMS: cmd.SeekMS}))
	case "pause":
		_ = player.HandlePause(ctx, commands.New(commands.CmdPreviewPause, nil))
	case "resume":
		_ = player.HandleResume(ctx, commands.New(commands.CmdPreviewResume, nil))
	case "stop":
		_ = player.HandleStop(ctx, commands.New(commands.CmdPreviewStop, nil))
	case "seek":
		_ = player.HandleSeek(ctx, commands.New(commands.CmdPreviewSeek,
			commands.PreviewSeekPayload{PositionMS: cmd.PositionMS}))
	}
}
