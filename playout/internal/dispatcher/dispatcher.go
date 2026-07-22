// Package dispatcher reads commands from the Command Bus, validates them
// against the current engine state, routes them to registered handlers, and
// publishes CommandAccepted / CommandRejected events.
//
// Dependency direction: dispatcher → commands, events, state.
// The API and audio packages must not be imported here.
package dispatcher

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// HandlerFunc is the signature for per-command business-logic handlers.
// A handler is called only after the command passes state-machine validation.
// Errors are logged but do not alter the acceptance status already published.
type HandlerFunc func(ctx context.Context, cmd commands.Command) error

// Dispatcher is the central command router of the engine.
type Dispatcher struct {
	cmdBus   *commands.Bus
	evtBus   *events.Bus
	stateMgr *state.Manager
	handlers map[commands.CommandType]HandlerFunc
	log      *slog.Logger
}

// New creates a Dispatcher wired to the given buses and state manager.
// Register command handlers with Handle() before calling Run().
// log may be nil; a no-op logger will be used in that case.
func New(
	cmdBus *commands.Bus,
	evtBus *events.Bus,
	stateMgr *state.Manager,
	log *slog.Logger,
) *Dispatcher {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Dispatcher{
		cmdBus:   cmdBus,
		evtBus:   evtBus,
		stateMgr: stateMgr,
		handlers: make(map[commands.CommandType]HandlerFunc),
		log:      log,
	}
}

// Handle registers h as the handler for command type t.
// Must be called before Run().
func (d *Dispatcher) Handle(t commands.CommandType, h HandlerFunc) {
	d.handlers[t] = h
}

// Run starts the dispatch loop, reading commands until ctx is cancelled.
// Intended to be called in a dedicated goroutine.
func (d *Dispatcher) Run(ctx context.Context) {
	d.log.Info("dispatcher started")
	for {
		select {
		case <-ctx.Done():
			d.log.Info("dispatcher stopped")
			return
		case cmd, ok := <-d.cmdBus.Receive():
			if !ok {
				return
			}
			d.dispatch(ctx, cmd)
		}
	}
}

func (d *Dispatcher) dispatch(ctx context.Context, cmd commands.Command) {
	snap := d.stateMgr.Snapshot()

	// 1. State-machine validation: command allowed in current state?
	if reason := d.validate(cmd.Type, snap.State); reason != "" {
		d.log.Info("command rejected by state machine",
			"command_id", cmd.ID,
			"command", string(cmd.Type),
			"state", string(snap.State),
			"reason", reason,
		)
		d.evtBus.Publish(events.New(events.EvtCommandRejected, events.CommandRejectedPayload{
			CommandID: cmd.ID,
			Command:   string(cmd.Type),
			Reason:    reason,
		}))
		d.stateMgr.RecordLastCommand(string(cmd.Type), false)
		d.reply(cmd, commands.Result{CommandID: cmd.ID, Accepted: false, Reason: reason})
		return
	}

	// 2. Call the registered handler BEFORE publishing CommandAccepted so that
	//    business-logic rejections (RejectedError) can still result in a
	//    CommandRejected event (not CommandAccepted followed by a silent failure).
	var handlerErr error
	if h, ok := d.handlers[cmd.Type]; ok {
		handlerErr = h(ctx, cmd)
	}

	// 3. Check whether the handler signalled a business-logic rejection.
	var rejected *commands.RejectedError
	if handlerErr != nil {
		// Use errors.As so wrapped RejectedErrors are also detected.
		if isRejected(handlerErr, &rejected) {
			reason := rejected.Reason
			d.log.Info("command rejected by handler",
				"command_id", cmd.ID,
				"command", string(cmd.Type),
				"reason", reason,
			)
			d.evtBus.Publish(events.New(events.EvtCommandRejected, events.CommandRejectedPayload{
				CommandID: cmd.ID,
				Command:   string(cmd.Type),
				Reason:    reason,
			}))
			d.stateMgr.RecordLastCommand(string(cmd.Type), false)
			d.reply(cmd, commands.Result{CommandID: cmd.ID, Accepted: false, Reason: reason})
			return
		}
		// Non-rejection handler error: command is accepted but handler logged an error.
		d.log.Error("command handler error",
			"command_id", cmd.ID,
			"command", string(cmd.Type),
			"error", handlerErr,
		)
	}

	// 4. Command accepted (handler succeeded or returned a non-rejection error).
	d.log.Debug("command accepted",
		"command_id", cmd.ID,
		"command", string(cmd.Type),
		"state", string(snap.State),
	)
	d.evtBus.Publish(events.New(events.EvtCommandAccepted, events.CommandAcceptedPayload{
		CommandID: cmd.ID,
		Command:   string(cmd.Type),
	}))
	d.stateMgr.RecordLastCommand(string(cmd.Type), true)

	result := commands.Result{CommandID: cmd.ID, Accepted: true}
	if handlerErr != nil {
		result.Reason = handlerErr.Error()
	}
	d.reply(cmd, result)
}

// isRejected reports whether err (or an error it wraps) is a *RejectedError
// and, if so, sets target to that error.
func isRejected(err error, target **commands.RejectedError) bool {
	e, ok := err.(*commands.RejectedError)
	if ok {
		*target = e
	}
	return ok
}

// reply sends result to cmd.Reply when it is non-nil. It never blocks.
func (d *Dispatcher) reply(cmd commands.Command, result commands.Result) {
	if cmd.Reply == nil {
		return
	}
	select {
	case cmd.Reply <- result:
	default:
	}
}

// validate returns an empty string when cmdType is permitted in currentState,
// or a human-readable rejection reason otherwise.
func (d *Dispatcher) validate(cmdType commands.CommandType, currentState state.PlayerState) string {
	// Preview commands are fully isolated from the main playback pipeline and
	// are allowed in any engine state.
	switch cmdType {
	case commands.CmdPreviewPlay, commands.CmdPreviewPause, commands.CmdPreviewResume,
		commands.CmdPreviewStop, commands.CmdPreviewSeek:
		return ""
	}

	// ENTER_PANIC has maximum priority — allowed in any state except STOPPING.
	if cmdType == commands.CmdEnterPanic {
		if currentState == state.StateStopping {
			return "cannot enter panic while engine is stopping"
		}
		return ""
	}

	allowed, ok := allowedCommands[currentState]
	if !ok {
		return fmt.Sprintf("engine is not accepting commands in state %s", currentState)
	}
	if !allowed[cmdType] {
		return fmt.Sprintf("command %s is not allowed in state %s", cmdType, currentState)
	}
	return ""
}

// allowedCommands maps each PlayerState to the set of commands it accepts.
// ENTER_PANIC is handled separately in validate() and not listed here.
var allowedCommands = map[state.PlayerState]map[commands.CommandType]bool{
	state.StateIdle: {
		commands.CmdEnqueue:            true,
		commands.CmdEnqueueBreak:       true,
		commands.CmdPlay:               true,
		commands.CmdClearQueue:         true,
		commands.CmdInsertNext:         true,
		commands.CmdInsertBreakNext:    true,
		commands.CmdRemoveItem:         true,
		commands.CmdMoveItem:           true,
		commands.CmdReorderItem:        true,
		commands.CmdPlayNow:            true,
		commands.CmdEnterAssist:        true,
		commands.CmdSetVolume:          true,
		commands.CmdPreviewSetVolume:   true,
		commands.CmdCartPlay:           true,
		commands.CmdCartStop:           true,
		commands.CmdCartSetVolume:      true,
	},
	state.StatePlaying: {
		commands.CmdPause:              true,
		commands.CmdStop:               true,
		commands.CmdSkip:               true,
		commands.CmdEnqueue:            true,
		commands.CmdEnqueueBreak:       true,
		commands.CmdInsertNext:         true,
		commands.CmdInsertBreakNext:    true,
		commands.CmdInsertAfter:        true,
		commands.CmdClearQueue:         true,
		commands.CmdRemoveItem:         true,
		commands.CmdMoveItem:           true,
		commands.CmdReorderItem:        true,
		commands.CmdPlayNow:            true,
		commands.CmdEnterAssist:        true,
		commands.CmdTriggerHotButton:   true,
		commands.CmdSetVolume:          true,
		commands.CmdPreviewSetVolume:   true,
		commands.CmdCartPlay:           true,
		commands.CmdCartStop:           true,
		commands.CmdCartSetVolume:      true,
	},
	state.StatePaused: {
		commands.CmdResume:             true,
		commands.CmdStop:               true,
		commands.CmdEnqueue:            true,
		commands.CmdEnqueueBreak:       true,
		commands.CmdInsertNext:         true,
		commands.CmdInsertBreakNext:    true,
		commands.CmdInsertAfter:        true,
		commands.CmdClearQueue:         true,
		commands.CmdRemoveItem:         true,
		commands.CmdMoveItem:           true,
		commands.CmdReorderItem:        true,
		commands.CmdPlayNow:            true,
		commands.CmdSetVolume:          true,
		commands.CmdPreviewSetVolume:   true,
		commands.CmdCartPlay:           true,
		commands.CmdCartStop:           true,
		commands.CmdCartSetVolume:      true,
	},
	state.StateAssist: {
		commands.CmdPlay:               true, // manual advance: sends signal to waiting sessionLoop
		commands.CmdPause:              true, // allowed while an item is playing in ASSIST mode
		commands.CmdReturnAuto:         true,
		commands.CmdSkip:               true,
		commands.CmdStop:               true,
		commands.CmdEnqueue:            true,
		commands.CmdEnqueueBreak:       true,
		commands.CmdInsertNext:         true,
		commands.CmdInsertBreakNext:    true,
		commands.CmdInsertAfter:        true,
		commands.CmdClearQueue:         true,
		commands.CmdRemoveItem:         true,
		commands.CmdMoveItem:           true,
		commands.CmdReorderItem:        true,
		commands.CmdPlayNow:            true,
		commands.CmdTriggerHotButton:   true,
		commands.CmdSetVolume:          true,
		commands.CmdPreviewSetVolume:   true,
		commands.CmdCartPlay:           true,
		commands.CmdCartStop:           true,
		commands.CmdCartSetVolume:      true,
	},
	state.StatePanic: {
		commands.CmdExitPanic:          true,
		commands.CmdStop:               true,
		commands.CmdEnqueue:            true,
		commands.CmdInsertNext:         true,
		commands.CmdSetVolume:          true,
		commands.CmdPreviewSetVolume:   true,
		commands.CmdCartPlay:           true,
		commands.CmdCartStop:           true,
		commands.CmdCartSetVolume:      true,
		// CmdInsertBreakNext not allowed in PANIC — scheduler never fires breaks during PANIC
	},
	state.StateError: {
		commands.CmdReset:              true,
		commands.CmdCartPlay:           true,
		commands.CmdCartStop:           true,
		commands.CmdCartSetVolume:      true,
	},
	// StateStarting and StateStopping: no commands accepted (except ENTER_PANIC above).
}
