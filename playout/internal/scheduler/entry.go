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
// Exactly one of Item or Break must carry the content to play.
type Entry struct {
	// ID is the unique identifier for this entry (ulid-prefixed).
	ID string `json:"id"`
	// Name is a human-readable label shown in logs and events.
	Name string `json:"name"`
	// Enabled controls whether the entry is active.
	Enabled bool `json:"enabled"`

	// CronExpr is a 5-field cron expression (minute hour day month weekday).
	// Mutually exclusive with FireAt.
	CronExpr string `json:"cron_expr,omitempty"`
	// FireAt is the exact UTC time for a one-shot firing.
	// Mutually exclusive with CronExpr. The entry is automatically disabled
	// after it fires (or is marked as missed when past missed_threshold_ms).
	FireAt *time.Time `json:"fire_at,omitempty"`

	// Item is the queue item to insert when the entry fires.
	// Mutually exclusive with Break.
	Item commands.QueueItemInput `json:"item,omitempty"`
	// Break is the commercial break to insert when the entry fires.
	// Mutually exclusive with Item. When set, CmdInsertBreakNext is used
	// instead of CmdInsertNext so the full block is placed at the front.
	Break *commands.BreakItemInput `json:"break,omitempty"`
	// TriggerMode controls how the item/break is inserted relative to current playback.
	TriggerMode TriggerMode `json:"trigger_mode"`

	// CreatedAt is the time the entry was first registered.
	CreatedAt time.Time `json:"created_at"`
	// LastFiredAt records the last successful (or missed) firing time.
	// Zero value means the entry has never been evaluated.
	LastFiredAt time.Time `json:"last_fired_at,omitempty"`

	// cronEntryID is the robfig/cron job handle; not persisted.
	cronEntryID cron.EntryID `json:"-"`
}
