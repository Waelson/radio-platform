package scheduler

import (
	"log/slog"

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

	// Entries never fire in PANIC mode — the scheduler would interfere with the
	// safety bed. Mark as missed and bail out.
	if st == state.StatePanic {
		m.publishMissed(e, "engine is in PANIC mode")
		return false
	}

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

// fireInterrupt: insert next + hard-cut skip immediately.
func (m *Manager) fireInterrupt(e *Entry, st state.PlayerState) bool {
	m.insertNext(e)
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
	m.insertNext(e)
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
	m.insertNext(e)
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
	m.insertNext(e)
	m.cmdBus.TrySend(commands.New(commands.CmdPlay, commands.PlayPayload{
		Reason: "scheduler: SKIP_IF_BUSY entry " + e.ID,
	}))
	m.publishFired(e)
	return true
}

// insertNext sends a CmdInsertNext with the entry's item to the Command Bus.
func (m *Manager) insertNext(e *Entry) {
	m.cmdBus.TrySend(commands.New(commands.CmdInsertNext, commands.InsertNextPayload{
		Item: e.Item,
	}))
}

// publishFired emits EvtScheduleEntryFired.
func (m *Manager) publishFired(e *Entry) {
	m.evtBus.Publish(events.New(events.EvtScheduleEntryFired, events.ScheduleEntryFiredPayload{
		EntryID:     e.ID,
		EntryName:   e.Name,
		TriggerMode: string(e.TriggerMode),
		AssetID:     e.Item.AssetID,
		Title:       e.Item.Title,
		OneShot:     e.FireAt != nil,
	}))
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
