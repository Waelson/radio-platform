// Package queue defines the in-memory playback queue types.
package queue

// AssetType classifies the audio content of a queue item.
type AssetType string

const (
	AssetTypeMusic      AssetType = "musicas"
	AssetTypeSpot       AssetType = "spots"
	AssetTypeCommercial AssetType = "COMMERCIAL"
	AssetTypeJingle     AssetType = "jingles"
	AssetTypeBed        AssetType = "BED"
	AssetTypeEffect     AssetType = "EFFECT"
	AssetTypeVoice      AssetType = "VOICE"
	AssetTypeHoraCerta  AssetType = "HORA_CERTA"
	AssetTypeUnknown    AssetType = "UNKNOWN"
)

// TransitionType defines how a queue item transitions to the next one.
type TransitionType string

const (
	TransitionCut       TransitionType = "CUT"
	TransitionFadeOut   TransitionType = "FADE_OUT"
	TransitionCrossfade TransitionType = "CROSSFADE"
	TransitionHard      TransitionType = "HARD"
)

// ItemStatus tracks the lifecycle of a queue item.
type ItemStatus string

const (
	ItemStatusQueued     ItemStatus = "QUEUED"
	ItemStatusPreloading ItemStatus = "PRELOADING"
	ItemStatusPlaying    ItemStatus = "PLAYING"
	ItemStatusFadingOut  ItemStatus = "FADING_OUT"
	ItemStatusPlayed     ItemStatus = "PLAYED"
	ItemStatusSkipped    ItemStatus = "SKIPPED"
	ItemStatusFailed     ItemStatus = "FAILED"
	ItemStatusMissed     ItemStatus = "MISSED"
)

// ItemResult records why and how an item's playback ended.
type ItemResult string

const (
	ItemResultPlayed             ItemResult = "PLAYED"
	ItemResultSkipped            ItemResult = "SKIPPED"
	ItemResultStopped            ItemResult = "STOPPED" // playback stopped by operator; item returns to queue
	ItemResultFailed             ItemResult = "FAILED"
	ItemResultMissed             ItemResult = "MISSED"
	ItemResultInterruptedByPanic ItemResult = "INTERRUPTED_BY_PANIC"
)

// TransitionSpec describes how this item transitions to the next.
type TransitionSpec struct {
	Type       TransitionType
	DurationMS int64
}

// QueueItem is the internal representation of an item in the playback queue.
// The Engine receives items already resolved: asset_id, path, duration, and
// operational metadata. It has no knowledge of the music library.
type QueueItem struct {
	QueueItemID string
	AssetID     string
	Path        string
	Type        AssetType
	Title       string
	Artist      string
	DurationMS  int64
	CueInMS     int64
	CueOutMS    int64 // 0 means use DurationMS
	Transition  TransitionSpec
	Mandatory   bool
	GainDB      float64
	Metadata    map[string]string
	Status      ItemStatus

	// Break fields — non-empty when this item belongs to a commercial break.
	BreakID    string `json:"break_id,omitempty"`
	BreakTitle string `json:"break_title,omitempty"`
	BreakSeq   int    `json:"break_seq,omitempty"`   // 1-based position within the break
	BreakTotal int    `json:"break_total,omitempty"` // total sub-items in the break
	BreakRole  string `json:"break_role,omitempty"`  // "open" | "spot" | "close"
}

// BreakItem represents a commercial break as a hierarchical unit.
// It is used in the API/command layer to receive and return breaks atomically.
// Internally, the queue stores break sub-items as flat QueueItems with BreakID set.
type BreakItem struct {
	BreakID string
	Title   string
	Open    *QueueItem   // opening jingle (optional)
	Spots   []*QueueItem // commercial spots (required, ≥1)
	Close   *QueueItem   // closing jingle (optional)
}

// EffectiveCueOut returns the effective cue-out point in milliseconds.
// When CueOutMS is zero, falls back to DurationMS.
func (q *QueueItem) EffectiveCueOut() int64 {
	if q.CueOutMS > 0 {
		return q.CueOutMS
	}
	return q.DurationMS
}

// EffectiveDuration returns the playable duration respecting cue points.
func (q *QueueItem) EffectiveDuration() int64 {
	return q.EffectiveCueOut() - q.CueInMS
}
