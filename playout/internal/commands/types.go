// Package commands defines the internal command types carried through the
// Command Bus. This package imports nothing from the project to prevent
// import cycles.
package commands

import (
	"time"

	"github.com/oklog/ulid/v2"
)

// CommandType identifies the type of an internal command.
type CommandType string

const (
	CmdEnqueue          CommandType = "ENQUEUE"
	CmdEnqueueBreak     CommandType = "ENQUEUE_BREAK"
	CmdPlay             CommandType = "PLAY"
	CmdPause            CommandType = "PAUSE"
	CmdResume           CommandType = "RESUME"
	CmdStop             CommandType = "STOP"
	CmdSkip             CommandType = "SKIP"
	CmdClearQueue       CommandType = "CLEAR_QUEUE"
	CmdInsertNext       CommandType = "INSERT_NEXT"
	CmdInsertBreakNext  CommandType = "INSERT_BREAK_NEXT"
	CmdInsertAfter      CommandType = "INSERT_AFTER"
	CmdRemoveItem       CommandType = "REMOVE_ITEM"
	CmdMoveItem         CommandType = "MOVE_ITEM"
	CmdReorderItem      CommandType = "REORDER_ITEM"
	CmdEnterPanic       CommandType = "ENTER_PANIC"
	CmdExitPanic        CommandType = "EXIT_PANIC"
	CmdEnterAssist      CommandType = "ENTER_ASSIST"
	CmdReturnAuto       CommandType = "RETURN_AUTO"
	CmdTriggerHotButton CommandType = "TRIGGER_HOT_BUTTON"
	CmdReset            CommandType = "RESET"
	CmdShutdown         CommandType = "SHUTDOWN"

	// Volume commands.
	CmdSetVolume        CommandType = "SET_VOLUME"
	CmdPreviewSetVolume CommandType = "PREVIEW_SET_VOLUME"
	CmdCartSetVolume    CommandType = "CART_SET_VOLUME"

	// Preview (cue) commands — isolated from the main playback pipeline.
	CmdPreviewPlay   CommandType = "PREVIEW_PLAY"
	CmdPreviewPause  CommandType = "PREVIEW_PAUSE"
	CmdPreviewResume CommandType = "PREVIEW_RESUME"
	CmdPreviewStop   CommandType = "PREVIEW_STOP"
	CmdPreviewSeek   CommandType = "PREVIEW_SEEK"

	// Cart player commands — dedicated hotkey audio channel.
	CmdCartPlay CommandType = "CART_PLAY"
	CmdCartStop CommandType = "CART_STOP"
)

// Command is the internal envelope carried through the Command Bus.
// ID is a ULID prefixed with "cmd_". Payload is one of the typed
// payload structs below.
// Reply is optional: when non-nil the Dispatcher sends the Result after
// processing, allowing the caller to wait for a synchronous acknowledgment.
type Command struct {
	ID       string
	Type     CommandType
	Payload  any
	IssuedAt time.Time
	Reply    chan Result // nil means fire-and-forget
}

// Result is returned by the Dispatcher after processing a command.
type Result struct {
	CommandID string
	Accepted  bool
	Reason    string // non-empty when rejected
}

// New creates a fire-and-forget command with a ULID-based ID.
func New(t CommandType, payload any) Command {
	return Command{
		ID:       "cmd_" + ulid.Make().String(),
		Type:     t,
		Payload:  payload,
		IssuedAt: time.Now().UTC(),
	}
}

// NewSync creates a command with a buffered Reply channel so the caller can
// wait for the Dispatcher's acceptance decision. The returned channel is closed
// by nobody — the Dispatcher sends exactly one Result and the caller reads it.
func NewSync(t CommandType, payload any) (Command, <-chan Result) {
	ch := make(chan Result, 1)
	return Command{
		ID:       "cmd_" + ulid.Make().String(),
		Type:     t,
		Payload:  payload,
		IssuedAt: time.Now().UTC(),
		Reply:    ch,
	}, ch
}

// --- Payload types (one per command) ----------------------------------------

// PlayPayload carries the payload for CmdPlay.
type PlayPayload struct {
	Reason string
}

// PausePayload carries the payload for CmdPause.
type PausePayload struct {
	Reason string
}

// ResumePayload carries the payload for CmdResume.
type ResumePayload struct {
	Reason string
}

// StopPayload carries the payload for CmdStop.
type StopPayload struct {
	ClearQueue bool
	FadeMS     int64
	Reason     string
}

// SkipPayload carries the payload for CmdSkip.
type SkipPayload struct {
	Reason     string
	Transition *TransitionInput
}

// TransitionInput describes a transition between items as received from the API.
type TransitionInput struct {
	Type       string // CUT | FADE_OUT | CROSSFADE | HARD
	DurationMS int64
}

// EnqueuePayload carries the payload for CmdEnqueue.
type EnqueuePayload struct {
	Items []QueueItemInput
}

// BreakItemInput is the API-level representation of a commercial break received
// in POST /v1/queue/enqueue-break. The handler converts it into flat QueueItems.
type BreakItemInput struct {
	Title string           `json:"title"`
	Open  *QueueItemInput  `json:"open,omitempty"`
	Spots []QueueItemInput `json:"spots"` // required, ≥1
	Close *QueueItemInput  `json:"close,omitempty"`
}

// EnqueueBreakPayload carries the payload for CmdEnqueueBreak.
// BreakID may be pre-computed by the caller (API handler) so it can be
// included in the HTTP response before the command is processed.
// If empty, HandleEnqueueBreak generates one.
type EnqueueBreakPayload struct {
	Break   BreakItemInput
	BreakID string // optional: pre-computed by the handler
}

// InsertNextPayload carries the payload for CmdInsertNext.
type InsertNextPayload struct {
	Item QueueItemInput
}

// InsertBreakNextPayload carries the payload for CmdInsertBreakNext.
// The break is expanded and inserted at the front of the pending queue,
// exactly like CmdInsertNext but for a full commercial break.
type InsertBreakNextPayload struct {
	Break   BreakItemInput
	BreakID string // optional: pre-computed by caller
}

// InsertAfterPayload carries the payload for CmdInsertAfter.
type InsertAfterPayload struct {
	AfterQueueItemID string
	Item             QueueItemInput
}

// ClearQueuePayload carries the payload for CmdClearQueue.
type ClearQueuePayload struct {
	PreserveCurrent bool
}

// RemoveItemPayload carries the payload for CmdRemoveItem.
type RemoveItemPayload struct {
	QueueItemID string
}

// MoveItemPayload carries the payload for CmdMoveItem.
// Direction must be "up" or "down".
type MoveItemPayload struct {
	QueueItemID string
	Direction   string // "up" | "down"
}

// ReorderItemPayload carries the payload for CmdReorderItem.
// Exactly one of QueueItemID or BreakID must be set.
// AfterID is the queue_item_id of the item after which to insert; "" means move to the front.
type ReorderItemPayload struct {
	QueueItemID string
	BreakID     string
	AfterID     string
}

// EnterPanicPayload carries the payload for CmdEnterPanic.
type EnterPanicPayload struct {
	Reason string
	Bed    *PanicBedInput
}

// PanicBedInput describes the safety audio bed to play during panic.
type PanicBedInput struct {
	AssetID string
	Path    string
}

// ExitPanicPayload carries the payload for CmdExitPanic.
type ExitPanicPayload struct {
	Reason string
}

// EnterAssistPayload carries the payload for CmdEnterAssist.
type EnterAssistPayload struct {
	Reason string
}

// ReturnAutoPayload carries the payload for CmdReturnAuto.
type ReturnAutoPayload struct{}

// TriggerHotButtonPayload carries the payload for CmdTriggerHotButton.
type TriggerHotButtonPayload struct {
	ButtonID   string
	Asset      QueueItemInput
	PlayMode   string // OVERLAY | INTERRUPT | AFTER_CURRENT
	DuckMain   bool
	DuckGainDB float64
	Reason     string
}

// ResetPayload carries the payload for CmdReset.
type ResetPayload struct{}

// ShutdownPayload carries the payload for CmdShutdown.
type ShutdownPayload struct {
	Reason string
}

// QueueItemInput carries queue item fields received from an external API
// request. It lives in the commands package (not in queue) so that the
// commands package remains free of internal imports.
type QueueItemInput struct {
	AssetID    string
	Path       string
	Type       string
	Title      string
	Artist     string
	ISRC       string
	Composer   string
	Publisher  string
	DurationMS int64
	CueInMS    int64
	CueOutMS   int64
	GainDB     float64
	Transition *TransitionInput
	Mandatory  bool
	Metadata   map[string]string
}

// --- Preview payloads --------------------------------------------------------

// PreviewPlayPayload is the payload for CmdPreviewPlay.
type PreviewPlayPayload struct {
	// Path is the absolute path to the audio file to preview.
	Path string
	// SeekMS is the playback start position in milliseconds.
	// Zero means start from the beginning.
	SeekMS int64
}

// PreviewSeekPayload is the payload for CmdPreviewSeek.
// Seek stops the current preview and restarts it from PositionMS.
type PreviewSeekPayload struct {
	PositionMS int64
}

// SetVolumePayload carries the new volume level for the main output.
type SetVolumePayload struct {
	Level float32 // 0.0–1.0
}

// PreviewSetVolumePayload carries the new volume level for the preview/CUE output.
type PreviewSetVolumePayload struct {
	Level float32 // 0.0–1.0
}

// CmdPreviewPause, CmdPreviewResume and CmdPreviewStop carry no payload.

// --- Cart payloads -----------------------------------------------------------

// CartPlayPayload is the payload for CmdCartPlay.
// If a cart is already playing it is stopped with reason "replaced" before
// the new one starts.
type CartPlayPayload struct {
	Path   string
	Title  string
	Artist string
	GainDB float64 // EBU R128 normalization gain in dB; 0 = unity
}

// CartSetVolumePayload carries the new volume level for the cart output.
type CartSetVolumePayload struct {
	Level float32 // 0.0–1.0
}

// CmdCartStop carries no payload.
