// Package scheduler implements timed playback scheduling for the playout engine.
// Entries are evaluated every second (for FireAt) or driven by robfig/cron (for
// CronExpr). Commands are sent to the existing Command Bus — the scheduler never
// touches the audio pipeline directly.
package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Waelson/radio-playout-engine/internal/commands"
)

// TriggerMode defines how the scheduler inserts and starts the scheduled item.
type TriggerMode string

const (
	// TriggerInterrupt inserts the item next and immediately cuts (skips) the
	// current item. Use for time-critical content like news or top-of-hour IDs.
	TriggerInterrupt TriggerMode = "INTERRUPT"

	// TriggerAfterCurrent inserts the item next and waits for the current item
	// to finish naturally. The item plays at the next natural transition.
	TriggerAfterCurrent TriggerMode = "AFTER_CURRENT"

	// TriggerCrossfade inserts the item next and triggers a crossfade skip on
	// the current item. Use for smooth musical transitions.
	TriggerCrossfade TriggerMode = "CROSSFADE"

	// TriggerSkipIfBusy inserts and plays the item only when the engine is idle.
	// If the engine is already playing or paused, the event is marked as MISSED.
	TriggerSkipIfBusy TriggerMode = "SKIP_IF_BUSY"
)

// Entry represents a single scheduled playback event.
// Exactly one of CronExpr or FireAt must be set.
type Entry struct {
	// ID is the unique identifier for this entry (ulid-prefixed).
	ID string
	// Name is a human-readable label shown in logs and events.
	Name string
	// Enabled controls whether the entry is active.
	Enabled bool

	// CronExpr is a 5-field cron expression (minute hour day month weekday).
	// Mutually exclusive with FireAt.
	CronExpr string
	// FireAt is the exact UTC time for a one-shot firing.
	// Mutually exclusive with CronExpr. The entry is automatically disabled
	// after it fires.
	FireAt *time.Time

	// Item is the queue item to insert when the entry fires.
	Item commands.QueueItemInput
	// TriggerMode controls how the item is inserted relative to the current playback.
	TriggerMode TriggerMode

	// internal — not exported; only modified under Manager.mu.
	cronEntryID cron.EntryID // non-zero when registered with the cron scheduler
	lastFiredAt time.Time    // zero = never fired
}
