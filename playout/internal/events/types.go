// Package events defines the internal event types published on the Event Bus.
// This package imports nothing from the project to prevent import cycles.
package events

import "time"

// EventType identifies the kind of event.
type EventType string

const (
	// EvtStateSnapshot is a synthetic event sent only to newly connected
	// WebSocket clients to provide an initial state before event streaming begins.
	// It is never published to the Event Bus.
	EvtStateSnapshot EventType = "StateSnapshot"

	EvtEngineStarted      EventType = "EngineStarted"
	EvtEngineStopping     EventType = "EngineStopping"
	EvtPlayerStateChanged EventType = "PlayerStateChanged"
	EvtNowPlayingChanged  EventType = "NowPlayingChanged"
	EvtProgressChanged    EventType = "ProgressChanged"
	EvtAudioHealthChanged EventType = "AudioHealthChanged"
	EvtQueueChanged       EventType = "QueueChanged"
	EvtCommandAccepted    EventType = "CommandAccepted"
	EvtCommandRejected    EventType = "CommandRejected"
	EvtItemStarted        EventType = "ItemStarted"
	EvtItemFinished       EventType = "ItemFinished"
	EvtCrossfadeStarted   EventType = "CrossfadeStarted"
	EvtPanicEntered       EventType = "PanicEntered"
	EvtPanicExited        EventType = "PanicExited"
	EvtAlertRaised        EventType = "AlertRaised"
	EvtAlertCleared       EventType = "AlertCleared"
	EvtHotButtonTriggered EventType = "HotButtonTriggered"
	EvtDuckingStarted     EventType = "DuckingStarted"
	EvtDuckingEnded       EventType = "DuckingEnded"
	EvtPlaybackError      EventType = "PlaybackError"
	EvtDecoderError       EventType = "DecoderError"
	EvtOutputOpenFailed   EventType = "OutputOpenFailed"
	EvtOutputWriteFailed  EventType = "OutputWriteFailed"
	EvtVUMeter            EventType = "VUMeter"

	// Commercial break events.
	EvtBreakStarted EventType = "BreakStarted"
	EvtBreakEnded   EventType = "BreakEnded"
	EvtSpotStarted  EventType = "SpotStarted"
	EvtSpotEnded    EventType = "SpotEnded"

	// Assist mode events.
	EvtAssistEntered EventType = "AssistEntered"
	EvtAssistExited  EventType = "AssistExited"
	EvtAssistWaiting EventType = "AssistWaiting"

	// Scheduler events.
	EvtScheduleEntryFired   EventType = "ScheduleEntryFired"
	EvtScheduleEntryMissed  EventType = "ScheduleEntryMissed"
	EvtScheduleEntryAdded   EventType = "ScheduleEntryAdded"
	EvtScheduleEntryRemoved EventType = "ScheduleEntryRemoved"
	EvtScheduleEntryUpdated EventType = "ScheduleEntryUpdated"

	// Preview (cue) events — isolated from the main playback pipeline.
	EvtPreviewStarted  EventType = "PreviewStarted"
	EvtPreviewPaused   EventType = "PreviewPaused"
	EvtPreviewResumed  EventType = "PreviewResumed"
	EvtPreviewStopped  EventType = "PreviewStopped"
	EvtPreviewProgress EventType = "PreviewProgress"
	EvtPreviewSeeked   EventType = "PreviewSeeked"
)

// Priority classifies events for backpressure decisions in the WebSocket hub.
// Low-priority events (progress, audio health) may be dropped under load.
// High-priority events (panic, errors, rejections) must never be dropped.
type Priority int

const (
	PriorityLow  Priority = 0
	PriorityHigh Priority = 1
)

// IsCritical reports whether the event type must never be silently dropped.
func IsCritical(t EventType) bool {
	switch t {
	case EvtPanicEntered, EvtPanicExited,
		EvtCommandRejected, EvtAlertRaised,
		EvtEngineStarted, EvtEngineStopping,
		EvtPlaybackError, EvtDecoderError,
		EvtOutputOpenFailed, EvtOutputWriteFailed:
		return true
	default:
		return false
	}
}

// Event is the envelope for every event published on the Event Bus and
// delivered to WebSocket clients.
type Event struct {
	EventID   string    `json:"event_id"`
	Type      EventType `json:"type"`
	Version   int       `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

// --- Payload types -----------------------------------------------------------

// EngineStartedPayload is the payload for EvtEngineStarted.
type EngineStartedPayload struct {
	EngineID string `json:"engine_id"`
	Version  string `json:"version"`
}

// EngineStoppingPayload is the payload for EvtEngineStopping.
type EngineStoppingPayload struct {
	Reason string `json:"reason"`
}

// PlayerStateChangedPayload is the payload for EvtPlayerStateChanged.
// From/To/Mode are strings matching the constants defined in the state package.
type PlayerStateChangedPayload struct {
	From string `json:"from"`
	To   string `json:"to"`
	Mode string `json:"mode"`
}

// NowPlayingChangedPayload is the payload for EvtNowPlayingChanged.
type NowPlayingChangedPayload struct {
	QueueItemID string `json:"queue_item_id"`
	AssetID     string `json:"asset_id"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Type        string `json:"type"`
	DurationMS  int64  `json:"duration_ms"`

	// Break fields — non-empty when the item belongs to a commercial break.
	BreakID       string `json:"break_id,omitempty"`
	BreakTitle    string `json:"break_title,omitempty"`
	BreakPosition int    `json:"break_position,omitempty"`
	BreakTotal    int    `json:"break_total,omitempty"`
	BreakRole     string `json:"break_role,omitempty"` // "open" | "spot" | "close"
}

// ProgressChangedPayload is the payload for EvtProgressChanged.
type ProgressChangedPayload struct {
	QueueItemID string  `json:"queue_item_id"`
	PositionMS  int64   `json:"position_ms"`
	DurationMS  int64   `json:"duration_ms"`
	Percent     float64 `json:"percent"`
	RemainingMS int64   `json:"remaining_ms"`
}

// AudioHealthChangedPayload is the payload for EvtAudioHealthChanged.
type AudioHealthChangedPayload struct {
	LevelDBFS         float64 `json:"level_dbfs"`
	PeakDBFS          float64 `json:"peak_dbfs"`
	Silence           bool    `json:"silence"`
	SilenceDurationMS int64   `json:"silence_duration_ms"`
	BufferPct         int     `json:"buffer_pct"`
	UnderrunCount     int64   `json:"underrun_count"`
}

// QueueChangedPayload is the payload for EvtQueueChanged.
type QueueChangedPayload struct {
	Size   int                `json:"size"`
	Reason string             `json:"reason"`
	Items  []QueueItemSummary `json:"items"`
}

// QueueItemSummary is the abbreviated item shape used in QueueChangedPayload.
type QueueItemSummary struct {
	QueueItemID string `json:"queue_item_id"`
	AssetID     string `json:"asset_id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	DurationMS  int64  `json:"duration_ms"`
}

// CommandAcceptedPayload is the payload for EvtCommandAccepted.
type CommandAcceptedPayload struct {
	CommandID string `json:"command_id"`
	Command   string `json:"command"`
	Reason    string `json:"reason,omitempty"`
}

// CommandRejectedPayload is the payload for EvtCommandRejected.
type CommandRejectedPayload struct {
	CommandID string `json:"command_id"`
	Command   string `json:"command"`
	Reason    string `json:"reason"`
}

// ItemStartedPayload is the payload for EvtItemStarted.
type ItemStartedPayload struct {
	QueueItemID string `json:"queue_item_id"`
	AssetID     string `json:"asset_id"`
}

// ItemFinishedPayload is the payload for EvtItemFinished.
type ItemFinishedPayload struct {
	QueueItemID      string `json:"queue_item_id"`
	AssetID          string `json:"asset_id"`
	Result           string `json:"result"`
	DurationPlayedMS int64  `json:"duration_played_ms"`
}

// CrossfadeStartedPayload is the payload for EvtCrossfadeStarted.
type CrossfadeStartedPayload struct {
	FromQueueItemID string `json:"from_queue_item_id"`
	ToQueueItemID   string `json:"to_queue_item_id"`
	DurationMS      int64  `json:"duration_ms"`
}

// PanicEnteredPayload is the payload for EvtPanicEntered.
type PanicEnteredPayload struct {
	Reason     string `json:"reason"`
	BedAssetID string `json:"bed_asset_id,omitempty"`
}

// PanicExitedPayload is the payload for EvtPanicExited.
type PanicExitedPayload struct {
	Reason string `json:"reason"`
}

// AlertRaisedPayload is the payload for EvtAlertRaised.
type AlertRaisedPayload struct {
	AlertID  string `json:"alert_id"`
	Severity string `json:"severity"` // INFO | WARNING | CRITICAL
	Source   string `json:"source"`
	Message  string `json:"message"`
}

// AlertClearedPayload is the payload for EvtAlertCleared.
type AlertClearedPayload struct {
	AlertID string `json:"alert_id"`
}

// HotButtonTriggeredPayload is the payload for EvtHotButtonTriggered.
type HotButtonTriggeredPayload struct {
	ButtonID string `json:"button_id"`
	AssetID  string `json:"asset_id"`
	PlayMode string `json:"play_mode"`
}

// DuckingStartedPayload is the payload for EvtDuckingStarted.
type DuckingStartedPayload struct {
	TargetChannel string  `json:"target_channel"`
	GainDB        float64 `json:"gain_db"`
}

// DuckingEndedPayload is the payload for EvtDuckingEnded.
type DuckingEndedPayload struct {
	TargetChannel string `json:"target_channel"`
}

// PlaybackErrorPayload is the payload for EvtPlaybackError.
type PlaybackErrorPayload struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	QueueItemID string `json:"queue_item_id,omitempty"`
	Recoverable bool   `json:"recoverable"`
}

// DecoderErrorPayload is the payload for EvtDecoderError.
type DecoderErrorPayload struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	QueueItemID string `json:"queue_item_id,omitempty"`
	AssetID     string `json:"asset_id,omitempty"`
	Recoverable bool   `json:"recoverable"`
}

// OutputFailedPayload is the payload for EvtOutputOpenFailed and EvtOutputWriteFailed.
type OutputFailedPayload struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

// VUMeterPayload is the payload for EvtVUMeter, published periodically when
// vu_meter_enabled is true. Provides professional broadcast-grade audio metrics.
type VUMeterPayload struct {
	RMSDbfs        float64            `json:"rms_dbfs"`
	PeakDbfs       float64            `json:"peak_dbfs"`
	PeakHoldDbfs   float64            `json:"peak_hold_dbfs"`
	LUFSMomentary  float64            `json:"lufs_momentary"`
	LUFSIntegrated float64            `json:"lufs_integrated"`
	Clip           bool               `json:"clip"`
	Channels       []VUChannelPayload `json:"channels"`
}

// VUChannelPayload holds per-channel metrics within VUMeterPayload.
type VUChannelPayload struct {
	RMSDbfs  float64 `json:"rms_dbfs"`
	PeakDbfs float64 `json:"peak_dbfs"`
}

// --- Commercial break payloads -----------------------------------------------

// BreakStartedPayload is the payload for EvtBreakStarted.
// Published once when the first sub-item of a break begins playing.
type BreakStartedPayload struct {
	BreakID    string `json:"break_id"`
	BreakTitle string `json:"break_title"`
	BreakTotal int    `json:"break_total"`
}

// BreakEndedPayload is the payload for EvtBreakEnded.
// Published after the last sub-item of a break finishes playing.
type BreakEndedPayload struct {
	BreakID    string `json:"break_id"`
	BreakTitle string `json:"break_title"`
}

// SpotStartedPayload is the payload for EvtSpotStarted.
// Published for every sub-item that starts within a break.
type SpotStartedPayload struct {
	BreakID     string `json:"break_id"`
	BreakTitle  string `json:"break_title"`
	QueueItemID string `json:"queue_item_id"`
	Title       string `json:"title"`
	BreakSeq    int    `json:"break_seq"`
	BreakTotal  int    `json:"break_total"`
	BreakRole   string `json:"break_role"` // "open" | "spot" | "close"
}

// SpotEndedPayload is the payload for EvtSpotEnded.
// Published when a sub-item within a break finishes playing.
type SpotEndedPayload struct {
	BreakID     string `json:"break_id"`
	QueueItemID string `json:"queue_item_id"`
	BreakSeq    int    `json:"break_seq"`
}

// AssistEnteredPayload is published when the engine enters ASSIST mode.
type AssistEnteredPayload struct {
	Reason string `json:"reason,omitempty"`
}

// AssistExitedPayload is published when the engine returns to AUTO mode.
type AssistExitedPayload struct{}

// AssistWaitingPayload is published when the engine is in ASSIST mode and
// has finished the current item, waiting for the operator to trigger the next.
type AssistWaitingPayload struct {
	NextTitle string `json:"next_title,omitempty"`
	NextType  string `json:"next_type,omitempty"`
	QueueSize int    `json:"queue_size"`
}

// --- Preview event payloads --------------------------------------------------

// PreviewStartedPayload is published when a preview begins playback.
type PreviewStartedPayload struct {
	Path       string `json:"path"`
	DurationMS int64  `json:"duration_ms"`
	SeekMS     int64  `json:"seek_ms"`
}

// PreviewPausedPayload is published when a preview is paused.
type PreviewPausedPayload struct {
	PositionMS int64 `json:"position_ms"`
	DurationMS int64 `json:"duration_ms"`
}

// PreviewResumedPayload is published when a paused preview resumes.
type PreviewResumedPayload struct {
	PositionMS int64 `json:"position_ms"`
	DurationMS int64 `json:"duration_ms"`
}

// PreviewStoppedPayload is published when a preview stops for any reason.
// Reason values: "stop" (explicit), "seek" (restarting at new position),
// "end" (reached end of file), "error" (decode/output failure).
type PreviewStoppedPayload struct {
	Reason     string `json:"reason"`
	PositionMS int64  `json:"position_ms"`
}

// PreviewProgressPayload is published periodically (~100ms) during playback.
type PreviewProgressPayload struct {
	PositionMS int64 `json:"position_ms"`
	DurationMS int64 `json:"duration_ms"`
}

// PreviewSeekedPayload is published after a seek completes and playback restarts.
type PreviewSeekedPayload struct {
	PositionMS int64 `json:"position_ms"`
	DurationMS int64 `json:"duration_ms"`
}

// --- Scheduler event payloads ------------------------------------------------

// ScheduleEntryFiredPayload is published when a scheduler entry fires successfully.
type ScheduleEntryFiredPayload struct {
	EntryID     string `json:"entry_id"`
	EntryName   string `json:"entry_name"`
	TriggerMode string `json:"trigger_mode"`
	AssetID     string `json:"asset_id,omitempty"`
	Title       string `json:"title,omitempty"`
	OneShot     bool   `json:"one_shot"` // true if the entry is auto-disabled after firing
}

// ScheduleEntryMissedPayload is published when a scheduler entry fires but
// the engine state prevents execution (e.g. PANIC, or SKIP_IF_BUSY while playing).
type ScheduleEntryMissedPayload struct {
	EntryID     string `json:"entry_id"`
	EntryName   string `json:"entry_name"`
	TriggerMode string `json:"trigger_mode"`
	Reason      string `json:"reason"` // human-readable reason for the miss
}

// ScheduleEntryAddedPayload is published when a new entry is registered.
type ScheduleEntryAddedPayload struct {
	EntryID  string `json:"entry_id"`
	Name     string `json:"name"`
	CronExpr string `json:"cron_expr,omitempty"`
	OneShot  bool   `json:"one_shot"`
}

// ScheduleEntryRemovedPayload is published when an entry is removed.
type ScheduleEntryRemovedPayload struct {
	EntryID string `json:"entry_id"`
}

// ScheduleEntryUpdatedPayload is published when an entry's enabled state changes.
type ScheduleEntryUpdatedPayload struct {
	EntryID string `json:"entry_id"`
	Enabled bool   `json:"enabled"`
}
