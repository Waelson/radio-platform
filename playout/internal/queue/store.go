package queue

import "time"

// CurrentSchemaVersion is the version stamped on every saved Snapshot.
// Increment when the Snapshot structure changes in a breaking way.
// CurrentSchemaVersion is bumped to 2 to document that QueueItem now carries
// BreakID / BreakTitle / BreakSeq / BreakTotal / BreakRole fields.
// Snapshots written by schema v1 are still readable: missing break fields
// unmarshal to zero values, which is semantically correct (no break context).
const CurrentSchemaVersion = 2

// Snapshot is a point-in-time copy of the queue state that can be persisted
// and restored across engine restarts.
type Snapshot struct {
	SchemaVersion int         `json:"schema_version"`
	SavedAt       time.Time   `json:"saved_at"`
	CurrentItemID string      `json:"current_item_id,omitempty"` // QueueItemID of the item that was playing
	Items         []QueueItem `json:"items"`
}

// Store is the persistence contract for the queue.
// Implementations must be safe for concurrent use.
type Store interface {
	// Save atomically persists the current queue snapshot.
	// A failure must not corrupt any previously saved state.
	Save(snap Snapshot) error

	// Load reads the last saved snapshot.
	// Returns an empty Snapshot (no error) if no snapshot exists yet.
	Load() (Snapshot, error)

	// Clear removes any persisted snapshot (e.g. on clean shutdown).
	Clear() error
}

// NopStore is a no-op Store used when persistence is disabled.
// All operations succeed immediately without touching disk.
type NopStore struct{}

func (NopStore) Save(_ Snapshot) error        { return nil }
func (NopStore) Load() (Snapshot, error)      { return Snapshot{}, nil }
func (NopStore) Clear() error                 { return nil }
