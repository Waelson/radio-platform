package scheduler

import (
	"log/slog"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// fireEntry executes the trigger logic for a single entry given the current
// engine snapshot. It sends commands to the Command Bus and publishes events.
// Returns true if the entry fired, false if it was missed.
func (m *Manager) fireEntry(e *Entry) bool {
	snap := m.stateMgr.Snapshot()
	st := snap.State

	// Line-in stop: fire regardless of PANIC (operator wants to stop capture).
	if e.LineInStop {
		return m.fireLineInStop(e, st)
	}

	// All other entries never fire in PANIC mode — the scheduler would
	// interfere with the safety bed. Mark as missed and bail out.
	if st == state.StatePanic {
		m.publishMissed(e, "engine is in PANIC mode")
		return false
	}

	// Line-in start entry.
	if e.LineIn != nil {
		return m.fireLineIn(e, st)
	}

	// Normal playback entry.
	switch e.TriggerMode {
	case TriggerInterrupt:
		return m.fireInterrupt(e, st)
	case TriggerAfterCurrent:
		return m.fireAfterCurrent(e, st)
	case TriggerCrossfade:
		return m.fireCrossfade(e, st)
	case TriggerSkipIfBusy:
		return m.fireSkipIfBusy(e, st)
	default:
		m.log.Warn("scheduler: unknown trigger mode, using AFTER_CURRENT fallback",
			"entry_id", e.ID, "mode", e.TriggerMode)
		return m.fireAfterCurrent(e, st)
	}
}

// fireLineIn dispatches CmdLineInStart for a line-in entry.
// For INTERRUPT mode, a CmdStop is sent first so that any active playback is
// halted before the capture session opens the audio output device.
func (m *Manager) fireLineIn(e *Entry, st state.PlayerState) bool {
	// SKIP_IF_BUSY: only start when the engine is idle.
	if e.TriggerMode == TriggerSkipIfBusy &&
		(st == state.StatePlaying || st == state.StatePaused || st == state.StateLineIn) {
		m.publishMissed(e, "engine is busy — SKIP_IF_BUSY line-in not started (state="+string(st)+")")
		return false
	}

	// Already in LINE_IN and not INTERRUPT: mark as missed to avoid duplicate sessions.
	if st == state.StateLineIn && e.TriggerMode != TriggerInterrupt {
		m.publishMissed(e, "line-in already active")
		return false
	}

	// INTERRUPT / CROSSFADE: stop current playback before starting line-in so
	// both do not compete for the same audio output device.
	if (e.TriggerMode == TriggerInterrupt || e.TriggerMode == TriggerCrossfade) &&
		(st == state.StatePlaying || st == state.StatePaused) {
		m.cmdBus.TrySend(commands.New(commands.CmdStop, commands.StopPayload{
			Reason: "scheduler: line-in INTERRUPT — stopping playback before capture",
		}))
	}

	payload := *e.LineIn // copy so we can set scheduler-specific fields
	payload.TriggerMode = string(e.TriggerMode)
	payload.TriggeredBy = "scheduler"

	// AFTER_CURRENT: defer CmdLineInStart until after the current item finishes.
	//
	// The engine never transitions through IDLE between consecutive queue items
	// — it goes PLAYING → PLAYING directly. So waiting for EvtPlayerStateChanged
	// To=IDLE would only work when the queue is already empty after the current
	// item, missing the common case where there are more items queued.
	//
	// Two-phase approach:
	//   Phase 1 — watch for EvtItemFinished (current item ended naturally) or
	//             EvtPlayerStateChanged To=IDLE (queue was already empty).
	//   Phase 2 — after EvtItemFinished, immediately send CmdStop to prevent
	//             the next queue item from starting, then wait for IDLE.
	//   Final   — send CmdLineInStart once the engine is idle.
	//
	// A 30-minute safety timeout prevents goroutine leaks.
	if e.TriggerMode == TriggerAfterCurrent &&
		(st == state.StatePlaying || st == state.StatePaused) {
		sub, cancel := m.evtBus.Subscribe(32)
		go func() {
			defer cancel()
			timeout := time.After(30 * time.Minute)

			// Phase 1: wait for the current item to finish (or engine to go idle).
			stopSent := false
			phase1:
			for {
				select {
				case evt := <-sub:
					switch evt.Type {
					case events.EvtItemFinished:
						// Current item ended — send Stop immediately so the next
						// queue item does not start playing before line-in begins.
						m.cmdBus.TrySend(commands.New(commands.CmdStop, commands.StopPayload{
							Reason: "scheduler: AFTER_CURRENT line-in — stopping after current item",
						}))
						stopSent = true
						break phase1
					case events.EvtPlayerStateChanged:
						p, ok := evt.Payload.(events.PlayerStateChangedPayload)
						if ok && p.To == string(state.StateIdle) {
							// Queue was already empty or engine went idle another way —
							// start line-in directly without needing a Stop.
							m.cmdBus.TrySend(commands.New(commands.CmdLineInStart, payload))
							return
						}
					}
				case <-timeout:
					m.log.Warn("scheduler: AFTER_CURRENT line-in timed out waiting for item end",
						"entry_id", e.ID, "name", e.Name)
					return
				}
			}

			if !stopSent {
				return
			}

			// Phase 2: wait for IDLE after the Stop is processed.
			for {
				select {
				case evt := <-sub:
					if evt.Type != events.EvtPlayerStateChanged {
						continue
					}
					p, ok := evt.Payload.(events.PlayerStateChangedPayload)
					if !ok {
						continue
					}
					if p.To == string(state.StateIdle) {
						m.cmdBus.TrySend(commands.New(commands.CmdLineInStart, payload))
						return
					}
				case <-timeout:
					m.log.Warn("scheduler: AFTER_CURRENT line-in timed out waiting for IDLE after stop",
						"entry_id", e.ID, "name", e.Name)
					return
				}
			}
		}()
		m.publishLineInFired(e)
		return true
	}

	m.cmdBus.TrySend(commands.New(commands.CmdLineInStart, payload))
	m.publishLineInFired(e)
	return true
}

// fireLineInStop dispatches CmdLineInStop for a line-in stop entry.
func (m *Manager) fireLineInStop(e *Entry, st state.PlayerState) bool {
	if st != state.StateLineIn {
		m.publishMissed(e, "line-in is not active — stop entry has no effect (state="+string(st)+")")
		return false
	}
	m.cmdBus.TrySend(commands.New(commands.CmdLineInStop, commands.LineInStopPayload{
		Reason: "scheduler: hard-stop entry " + e.ID,
	}))
	m.publishLineInStopFired(e)
	return true
}

// fireInterrupt: insert next + hard-cut skip immediately.
func (m *Manager) fireInterrupt(e *Entry, st state.PlayerState) bool {
	m.dispatchInsert(e)
	if st == state.StatePlaying || st == state.StatePaused {
		m.cmdBus.TrySend(commands.New(commands.CmdSkip, commands.SkipPayload{
			Reason: "scheduler: INTERRUPT entry " + e.ID,
		}))
	} else {
		// Engine is idle/assist — just start playback.
		m.cmdBus.TrySend(commands.New(commands.CmdPlay, commands.PlayPayload{
			Reason: "scheduler: INTERRUPT entry " + e.ID,
		}))
	}
	m.publishFired(e)
	return true
}

// fireAfterCurrent: insert next, let current item finish naturally.
func (m *Manager) fireAfterCurrent(e *Entry, st state.PlayerState) bool {
	m.dispatchInsert(e)
	if st == state.StateIdle || st == state.StateAssist {
		// Nothing playing — trigger playback immediately.
		m.cmdBus.TrySend(commands.New(commands.CmdPlay, commands.PlayPayload{
			Reason: "scheduler: AFTER_CURRENT entry " + e.ID,
		}))
	}
	m.publishFired(e)
	return true
}

// fireCrossfade: insert next + skip with crossfade.
func (m *Manager) fireCrossfade(e *Entry, st state.PlayerState) bool {
	m.dispatchInsert(e)
	if st == state.StatePlaying {
		m.cmdBus.TrySend(commands.New(commands.CmdSkip, commands.SkipPayload{
			Reason: "scheduler: CROSSFADE entry " + e.ID,
			Transition: &commands.TransitionInput{
				Type: "CROSSFADE",
			},
		}))
	} else if st == state.StateIdle || st == state.StateAssist {
		m.cmdBus.TrySend(commands.New(commands.CmdPlay, commands.PlayPayload{
			Reason: "scheduler: CROSSFADE entry " + e.ID,
		}))
	}
	m.publishFired(e)
	return true
}

// fireSkipIfBusy: fire only when the engine is idle; otherwise mark as missed.
func (m *Manager) fireSkipIfBusy(e *Entry, st state.PlayerState) bool {
	if st == state.StatePlaying || st == state.StatePaused {
		m.publishMissed(e, "engine is busy (state="+string(st)+")")
		return false
	}
	m.dispatchInsert(e)
	m.cmdBus.TrySend(commands.New(commands.CmdPlay, commands.PlayPayload{
		Reason: "scheduler: SKIP_IF_BUSY entry " + e.ID,
	}))
	m.publishFired(e)
	return true
}

// dispatchInsert sends the appropriate insert command based on entry type:
// CmdInsertBreakNext when the entry carries a commercial break,
// CmdInsertNext otherwise (single item or HORA_CERTA).
func (m *Manager) dispatchInsert(e *Entry) {
	if e.Break != nil {
		m.cmdBus.TrySend(commands.New(commands.CmdInsertBreakNext, commands.InsertBreakNextPayload{
			Break: *e.Break,
		}))
	} else {
		m.cmdBus.TrySend(commands.New(commands.CmdInsertNext, commands.InsertNextPayload{
			Item: e.Item,
		}))
	}
}

// publishFired emits EvtScheduleEntryFired with item or break metadata.
func (m *Manager) publishFired(e *Entry) {
	payload := events.ScheduleEntryFiredPayload{
		EntryID:     e.ID,
		EntryName:   e.Name,
		TriggerMode: string(e.TriggerMode),
		OneShot:     e.FireAt != nil,
	}
	if e.Break != nil {
		payload.BreakTitle = e.Break.Title
		payload.SpotCount = len(e.Break.Spots)
	} else {
		payload.AssetID = e.Item.AssetID
		payload.Title = e.Item.Title
	}
	m.evtBus.Publish(events.New(events.EvtScheduleEntryFired, payload))
	m.log.Info("scheduler: entry fired",
		"entry_id", e.ID,
		"name", e.Name,
		"mode", e.TriggerMode,
		slog.String("asset_id", e.Item.AssetID),
	)
}

// publishMissed emits EvtScheduleEntryMissed.
func (m *Manager) publishMissed(e *Entry, reason string) {
	m.evtBus.Publish(events.New(events.EvtScheduleEntryMissed, events.ScheduleEntryMissedPayload{
		EntryID:     e.ID,
		EntryName:   e.Name,
		TriggerMode: string(e.TriggerMode),
		Reason:      reason,
	}))
	m.log.Warn("scheduler: entry missed",
		"entry_id", e.ID,
		"name", e.Name,
		"mode", e.TriggerMode,
		"reason", reason,
	)
}

// publishLineInFired emits EvtScheduleEntryFired for a line-in start entry.
func (m *Manager) publishLineInFired(e *Entry) {
	label := ""
	if e.LineIn != nil {
		label = e.LineIn.Label
	}
	m.evtBus.Publish(events.New(events.EvtScheduleEntryFired, events.ScheduleEntryFiredPayload{
		EntryID:     e.ID,
		EntryName:   e.Name,
		TriggerMode: string(e.TriggerMode),
		Title:       label,
		OneShot:     e.FireAt != nil,
		IsLineIn:    true,
	}))
	m.log.Info("scheduler: line-in entry fired",
		"entry_id", e.ID,
		"name", e.Name,
		"mode", string(e.TriggerMode),
		slog.String("label", label),
	)
}

// publishLineInStopFired emits EvtScheduleEntryFired for a line-in stop entry.
func (m *Manager) publishLineInStopFired(e *Entry) {
	m.evtBus.Publish(events.New(events.EvtScheduleEntryFired, events.ScheduleEntryFiredPayload{
		EntryID:     e.ID,
		EntryName:   e.Name,
		TriggerMode: string(e.TriggerMode),
		OneShot:     e.FireAt != nil,
	}))
	m.log.Info("scheduler: line-in stop entry fired",
		"entry_id", e.ID,
		"name", e.Name,
	)
}
