package linein

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// Handler bridges the command bus and the LineInManager.
// It handles CmdLineInStart and CmdLineInStop, manages the session lifecycle,
// updates the state machine, and forwards LineInManager events to the Event Bus.
type Handler struct {
	out      output.OutputDevice
	stateMgr *state.Manager
	evtBus   *events.Bus
	log      *slog.Logger

	mu  sync.Mutex
	mgr *LineInManager
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
	}

	ch, err := mgr.Start(ctx, cfg)
	if err != nil {
		return fmt.Errorf("linein handler: start: %w", err)
	}

	h.mgr = mgr

	h.stateMgr.SetLineIn(state.LineInStatus{
		DeviceID:    p.DeviceID,
		Label:       p.Label,
		StartedAt:   time.Now().UTC(),
		DurationMS:  p.DurationMS,
		TriggeredBy: p.TriggeredBy,
	})

	go h.forward(cfg, ch)
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
func (h *Handler) forward(cfg LineInConfig, ch <-chan Event) {
	cleanStop := false
	stateCleared := false

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

	// Channel closed — ensure state is reset for sessions that ended via error
	// (no EventStopped was emitted before the channel closed).
	clearState()

	// Publish EvtLineInStopped for sessions that terminated without a clean stop
	// (error, silence_stop) so observers always see a matching Started/Stopped pair.
	if !cleanStop {
		h.evtBus.Publish(events.New(events.EvtLineInStopped, events.LineInStoppedPayload{
			Label:  cfg.Label,
			Reason: "error",
		}))
	}

	h.log.Info("line-in session ended", "label", cfg.Label)
}
