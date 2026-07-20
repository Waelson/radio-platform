package queue

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/oklog/ulid/v2"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// Manager is the in-memory playback queue. It is safe for concurrent use.
// All mutations publish a QueueChanged event and update the state snapshot.
type Manager struct {
	mu         sync.Mutex
	pending    []*QueueItem // items waiting to play, in order
	current    *QueueItem   // item currently playing (nil if none)
	evtBus     *events.Bus
	stateMgr   *state.Manager
	store      Store
	log        *slog.Logger
	persistMu  sync.Mutex // serialises disk writes; acquired only in persist goroutines
	persistSeq uint64     // incremented on every mutation; goroutines skip stale writes
}

// NewManager creates a Manager. log may be nil. store may be nil (NopStore used).
func NewManager(evtBus *events.Bus, stateMgr *state.Manager, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Manager{
		evtBus:   evtBus,
		stateMgr: stateMgr,
		store:    NopStore{},
		log:      log,
	}
}

// WithStore sets the persistence store. Call before any queue operations.
func (m *Manager) WithStore(s Store) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s == nil {
		m.store = NopStore{}
	} else {
		m.store = s
	}
}

// --- Command handlers --------------------------------------------------------
// These are registered with the Dispatcher and called after state validation.

// HandleEnqueue handles CmdEnqueue: appends items to the end of the queue.
func (m *Manager) HandleEnqueue(_ context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.EnqueuePayload)
	if !ok {
		return fmt.Errorf("enqueue: unexpected payload type %T", cmd.Payload)
	}
	if len(p.Items) == 0 {
		return fmt.Errorf("enqueue: items list is empty")
	}
	m.Enqueue(p.Items)
	return nil
}

// HandleInsertNext handles CmdInsertNext: inserts item at the front of pending.
func (m *Manager) HandleInsertNext(_ context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.InsertNextPayload)
	if !ok {
		return fmt.Errorf("insert-next: unexpected payload type %T", cmd.Payload)
	}
	m.InsertNext(p.Item)
	return nil
}

// HandleInsertAfter handles CmdInsertAfter: inserts item after a specific item.
func (m *Manager) HandleInsertAfter(_ context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.InsertAfterPayload)
	if !ok {
		return fmt.Errorf("insert-after: unexpected payload type %T", cmd.Payload)
	}
	if _, err := m.InsertAfter(p.AfterQueueItemID, p.Item); err != nil {
		return fmt.Errorf("insert-after: %w", err)
	}
	return nil
}

// HandleEnqueueBreak handles CmdEnqueueBreak: expands a BreakItemInput into
// flat QueueItems with BreakID context and appends them to the END of the
// pending queue.
func (m *Manager) HandleEnqueueBreak(_ context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.EnqueueBreakPayload)
	if !ok {
		return fmt.Errorf("enqueue-break: unexpected payload type %T", cmd.Payload)
	}
	if len(p.Break.Spots) == 0 {
		return fmt.Errorf("enqueue-break: spots list is empty")
	}
	breakID := p.BreakID
	if breakID == "" {
		breakID = "brk_" + ulid.Make().String()
	}
	subItems := expandBreak(p.Break, breakID)
	m.appendQueueItems(subItems, "enqueue_break")
	m.log.Info("enqueued break", "break_id", breakID, "title", p.Break.Title, "sub_items", len(subItems))
	return nil
}

// HandleInsertBreakNext handles CmdInsertBreakNext: expands a BreakItemInput
// and inserts the resulting sub-items at the FRONT of the pending queue,
// preserving break metadata (BreakID, BreakSeq, BreakTotal, BreakRole).
// This is the scheduler-facing counterpart of HandleEnqueueBreak.
func (m *Manager) HandleInsertBreakNext(_ context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.InsertBreakNextPayload)
	if !ok {
		return fmt.Errorf("insert-break-next: unexpected payload type %T", cmd.Payload)
	}
	if len(p.Break.Spots) == 0 {
		return fmt.Errorf("insert-break-next: spots list is empty")
	}
	breakID := p.BreakID
	if breakID == "" {
		breakID = "brk_" + ulid.Make().String()
	}
	subItems := expandBreak(p.Break, breakID)

	m.mu.Lock()
	m.pending = append(subItems, m.pending...)
	m.publishAndUpdateLocked("insert_break_next")
	m.persist()
	m.mu.Unlock()

	m.log.Info("inserted break at front", "break_id", breakID, "title", p.Break.Title, "sub_items", len(subItems))
	return nil
}

// expandBreak converts a BreakItemInput into ordered flat QueueItems with
// break metadata set. The expansion rules are:
//   - Open (if present) → TransitionCrossfade 3s (music → break crossfade)
//   - If no Open, the first spot also gets TransitionCrossfade 3s
//   - All other spots and Close → TransitionCut (hard cut within the block)
func expandBreak(b commands.BreakItemInput, breakID string) []*QueueItem {
	total := len(b.Spots)
	if b.Open != nil {
		total++
	}
	if b.Close != nil {
		total++
	}

	subItems := make([]*QueueItem, 0, total)
	seq := 1

	if b.Open != nil {
		item := newItem(*b.Open)
		item.BreakID = breakID
		item.BreakTitle = b.Title
		item.BreakSeq = seq
		item.BreakTotal = total
		item.BreakRole = "open"
		item.Transition = TransitionSpec{Type: TransitionCrossfade, DurationMS: 3000}
		subItems = append(subItems, item)
		seq++
	}

	for i, s := range b.Spots {
		item := newItem(s)
		item.BreakID = breakID
		item.BreakTitle = b.Title
		item.BreakSeq = seq
		item.BreakTotal = total
		item.BreakRole = "spot"
		if i == 0 && b.Open == nil {
			item.Transition = TransitionSpec{Type: TransitionCrossfade, DurationMS: 3000}
		} else {
			item.Transition = TransitionSpec{Type: TransitionCut}
		}
		subItems = append(subItems, item)
		seq++
	}

	if b.Close != nil {
		item := newItem(*b.Close)
		item.BreakID = breakID
		item.BreakTitle = b.Title
		item.BreakSeq = seq
		item.BreakTotal = total
		item.BreakRole = "close"
		item.Transition = TransitionSpec{Type: TransitionCut}
		subItems = append(subItems, item)
	}

	return subItems
}

// HandleClear handles CmdClearQueue: removes all pending items.
func (m *Manager) HandleClear(_ context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.ClearQueuePayload)
	if !ok {
		return fmt.Errorf("clear-queue: unexpected payload type %T", cmd.Payload)
	}
	m.Clear(p.PreserveCurrent)
	return nil
}

func (m *Manager) HandleRemoveItem(_ context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.RemoveItemPayload)
	if !ok {
		return fmt.Errorf("remove-item: unexpected payload type %T", cmd.Payload)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, it := range m.pending {
		if it.QueueItemID == p.QueueItemID {
			m.pending = append(m.pending[:i], m.pending[i+1:]...)
			m.publishAndUpdateLocked("remove-item")
			m.persist()
			return nil
		}
	}
	return fmt.Errorf("remove-item: item %q not found in pending queue", p.QueueItemID)
}

func (m *Manager) HandleMoveItem(_ context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.MoveItemPayload)
	if !ok {
		return fmt.Errorf("move-item: unexpected payload type %T", cmd.Payload)
	}
	if p.Direction != "up" && p.Direction != "down" {
		return fmt.Errorf("move-item: direction must be \"up\" or \"down\", got %q", p.Direction)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := -1
	for i, it := range m.pending {
		if it.QueueItemID == p.QueueItemID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("move-item: item %q not found in pending queue", p.QueueItemID)
	}
	var swapIdx int
	if p.Direction == "up" {
		if idx == 0 {
			return nil // already at top
		}
		swapIdx = idx - 1
	} else {
		if idx == len(m.pending)-1 {
			return nil // already at bottom
		}
		swapIdx = idx + 1
	}
	m.pending[idx], m.pending[swapIdx] = m.pending[swapIdx], m.pending[idx]
	m.publishAndUpdateLocked("move-item")
	m.persist()
	return nil
}

func (m *Manager) HandleReorderItem(_ context.Context, cmd commands.Command) error {
	p, ok := cmd.Payload.(commands.ReorderItemPayload)
	if !ok {
		return fmt.Errorf("reorder-item: unexpected payload type %T", cmd.Payload)
	}
	if (p.QueueItemID == "") == (p.BreakID == "") {
		return fmt.Errorf("reorder-item: exactly one of queue_item_id or break_id must be set")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var targets, remaining []*QueueItem
	for _, it := range m.pending {
		if (p.QueueItemID != "" && it.QueueItemID == p.QueueItemID) ||
			(p.BreakID != "" && it.BreakID == p.BreakID) {
			targets = append(targets, it)
		} else {
			remaining = append(remaining, it)
		}
	}
	if len(targets) == 0 {
		return fmt.Errorf("reorder-item: no items found for the given id")
	}

	// Find insertion point in remaining
	insertIdx := 0
	if p.AfterID != "" {
		for i, it := range remaining {
			if it.QueueItemID == p.AfterID {
				insertIdx = i + 1
				break
			}
		}
		// AfterID might belong to the moved items — insert at end in that case
		if insertIdx == 0 {
			insertIdx = len(remaining)
		}
	}

	m.pending = make([]*QueueItem, 0, len(targets)+len(remaining))
	m.pending = append(m.pending, remaining[:insertIdx]...)
	m.pending = append(m.pending, targets...)
	m.pending = append(m.pending, remaining[insertIdx:]...)

	m.publishAndUpdateLocked("reorder-item")
	m.persist()
	return nil
}

// --- Public queue operations -------------------------------------------------

// Enqueue adds items to the end of the pending queue and returns the
// created QueueItems with their assigned IDs.
func (m *Manager) Enqueue(inputs []commands.QueueItemInput) []*QueueItem {
	items := make([]*QueueItem, len(inputs))
	for i, inp := range inputs {
		items[i] = newItem(inp)
	}

	m.mu.Lock()
	m.pending = append(m.pending, items...)
	pendingSize := len(m.pending)
	m.publishAndUpdateLocked("enqueue")
	m.persist()
	m.mu.Unlock()

	m.log.Info("enqueued items", "count", len(items), "pending", pendingSize)
	return items
}

// appendQueueItems appends already-built QueueItems to pending.
// Used by HandleEnqueueBreak to add sub-items that need break context set
// before insertion (which newItem cannot do — it has no break context).
func (m *Manager) appendQueueItems(items []*QueueItem, reason string) {
	m.mu.Lock()
	m.pending = append(m.pending, items...)
	pendingSize := len(m.pending)
	m.publishAndUpdateLocked(reason)
	m.persist()
	m.mu.Unlock()

	m.log.Info("appended queue items", "count", len(items), "pending", pendingSize)
}

// InsertNext inserts item at the front of the pending queue (will play next).
func (m *Manager) InsertNext(input commands.QueueItemInput) *QueueItem {
	item := newItem(input)

	m.mu.Lock()
	m.pending = append([]*QueueItem{item}, m.pending...)
	m.publishAndUpdateLocked("insert_next")
	m.persist()
	m.mu.Unlock()

	m.log.Info("inserted item at front", "queue_item_id", item.QueueItemID)
	return item
}

// InsertAfter inserts item immediately after the item with afterID.
// If afterID matches the current item, inserts at the front of pending
// (equivalent to InsertNext). Returns an error if afterID is not found.
func (m *Manager) InsertAfter(afterID string, input commands.QueueItemInput) (*QueueItem, error) {
	item := newItem(input)

	m.mu.Lock()
	defer m.mu.Unlock()

	// If after the current item → insert at front of pending.
	if m.current != nil && m.current.QueueItemID == afterID {
		m.pending = append([]*QueueItem{item}, m.pending...)
		m.publishAndUpdateLocked("insert_after")
		m.persist()
		m.log.Info("inserted after current item", "queue_item_id", item.QueueItemID)
		return item, nil
	}

	// Search pending list.
	for i, qi := range m.pending {
		if qi.QueueItemID == afterID {
			newPending := make([]*QueueItem, 0, len(m.pending)+1)
			newPending = append(newPending, m.pending[:i+1]...)
			newPending = append(newPending, item)
			newPending = append(newPending, m.pending[i+1:]...)
			m.pending = newPending
			m.publishAndUpdateLocked("insert_after")
			m.persist()
			m.log.Info("inserted after item", "after_id", afterID, "queue_item_id", item.QueueItemID)
			return item, nil
		}
	}

	return nil, fmt.Errorf("queue item %q not found", afterID)
}

// Clear removes all pending items. If preserveCurrent is false, also clears the
// current item reference (note: this does not stop audio; that is the
// Playback Manager's responsibility).
func (m *Manager) Clear(preserveCurrent bool) int {
	m.mu.Lock()
	cleared := len(m.pending)
	m.pending = m.pending[:0]
	if !preserveCurrent {
		m.current = nil
	}
	m.publishAndUpdateLocked("clear")
	m.persist()
	m.mu.Unlock()

	m.log.Info("queue cleared", "items_cleared", cleared, "preserve_current", preserveCurrent)
	return cleared
}

// Pop removes and returns the first pending item for the Playback Manager to
// start playing. Returns false if the queue is empty.
func (m *Manager) Pop() (*QueueItem, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.pending) == 0 {
		return nil, false
	}
	item := m.pending[0]
	m.pending = m.pending[1:]
	m.publishAndUpdateLocked("pop")
	m.persist()
	return item, true
}

// PopAsCurrent atomically removes the first pending item and marks it as the
// current playing item in a single critical section. This eliminates the race
// window that exists when Pop() and SetCurrent() are called separately, which
// could cause GET /v1/queue to return without the currently playing item.
func (m *Manager) PopAsCurrent() (*QueueItem, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.pending) == 0 {
		return nil, false
	}
	item := m.pending[0]
	m.pending = m.pending[1:]
	item.Status = ItemStatusPlaying
	m.current = item
	m.publishAndUpdateLocked("pop_as_current")
	m.persist()
	return item, true
}

// SetCurrent marks item as the currently playing item. Called by Playback Manager.
func (m *Manager) SetCurrent(item *QueueItem) {
	m.mu.Lock()
	if item != nil {
		item.Status = ItemStatusPlaying
	}
	m.current = item
	m.publishAndUpdateLocked("set_current")
	m.persist()
	m.mu.Unlock()
}

// ClearCurrent removes the current item reference. Called by Playback Manager
// when an item finishes playing.
func (m *Manager) ClearCurrent() {
	m.mu.Lock()
	m.current = nil
	m.publishAndUpdateLocked("clear_current")
	m.persist()
	m.mu.Unlock()
}

// PrependItem inserts an already-built QueueItem at the front of the pending
// queue with status QUEUED. Used by the playback manager to return a
// crossfade-preloaded item to the queue when stop is issued mid-crossfade.
// Deprecated: prefer CancelPreloading when the item is already in the pending
// list with PRELOADING status.
func (m *Manager) PrependItem(item *QueueItem) {
	m.mu.Lock()
	item.Status = ItemStatusQueued
	m.pending = append([]*QueueItem{item}, m.pending...)
	m.publishAndUpdateLocked("return_to_front")
	m.persist()
	m.mu.Unlock()
}

// MarkPreloading changes the status of a pending item to PRELOADING to signal
// that its decoder has been opened ahead of the currently playing track
// (crossfade pre-roll). The item stays in the pending list so it remains
// visible in GET /v1/queue responses during the crossfade mixing period.
func (m *Manager) MarkPreloading(item *QueueItem) {
	m.mu.Lock()
	item.Status = ItemStatusPreloading
	m.publishAndUpdateLocked("preload")
	m.persist()
	m.mu.Unlock()
}

// PopPreloadingAsCurrent removes a PRELOADING item from the front of the
// pending queue and atomically marks it as the currently playing item. Called
// by the Playback Manager when a crossfade completes and the preloaded item
// takes over as the primary stream. If the item is no longer at the front of
// pending (e.g., user inserted another item ahead of it), the current pointer
// is still updated without a double-removal.
func (m *Manager) PopPreloadingAsCurrent(item *QueueItem) {
	m.mu.Lock()
	if len(m.pending) > 0 && m.pending[0].QueueItemID == item.QueueItemID {
		m.pending = m.pending[1:]
	}
	item.Status = ItemStatusPlaying
	m.current = item
	m.publishAndUpdateLocked("preload_as_current")
	m.persist()
	m.mu.Unlock()
}

// CancelPreloading resets a PRELOADING item back to QUEUED. Used when playback
// is stopped mid-crossfade: the item stays in the pending list (it was never
// removed by Pop) so it will be played on the next Play command.
func (m *Manager) CancelPreloading(item *QueueItem) {
	m.mu.Lock()
	item.Status = ItemStatusQueued
	m.publishAndUpdateLocked("cancel_preload")
	m.persist()
	m.mu.Unlock()
}

// ReturnCurrentToFront puts the current item back at the front of the pending
// queue with status QUEUED. Used when playback is stopped by the operator so
// the item can be played again later.
func (m *Manager) ReturnCurrentToFront() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return
	}
	item := m.current
	item.Status = ItemStatusQueued
	m.pending = append([]*QueueItem{item}, m.pending...)
	m.current = nil
	m.publishAndUpdateLocked("return_to_front")
	m.persist()
}

// Peek returns the first pending item without removing it.
func (m *Manager) Peek() (*QueueItem, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.pending) == 0 {
		return nil, false
	}
	return m.pending[0], true
}

// Current returns the currently playing item (nil if none).
func (m *Manager) Current() *QueueItem {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

// List returns a snapshot of all items: [current (if any), ...pending].
func (m *Manager) List() []*QueueItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]*QueueItem, 0, len(m.pending)+1)
	if m.current != nil {
		cp := *m.current
		out = append(out, &cp)
	}
	for _, it := range m.pending {
		cp := *it
		out = append(out, &cp)
	}
	return out
}

// ListPending returns a copy of the pending queue without the current item.
// Use this when you need to group items by BreakID independently from current.
func (m *Manager) ListPending() []*QueueItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]*QueueItem, len(m.pending))
	for i, it := range m.pending {
		cp := *it
		out[i] = &cp
	}
	return out
}

// Size returns the number of pending items (not counting the current item).
func (m *Manager) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}

// HasCurrent reports whether there is a currently playing item.
func (m *Manager) HasCurrent() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current != nil
}

// --- Private helpers ---------------------------------------------------------

// publishAndUpdateLocked publishes QueueChanged and updates the State Manager.
// Must be called with m.mu held.
func (m *Manager) publishAndUpdateLocked(reason string) {
	size := len(m.pending)

	// Build event payload.
	items := make([]events.QueueItemSummary, 0, size+1)
	if m.current != nil {
		items = append(items, toSummary(m.current))
	}
	for _, it := range m.pending {
		items = append(items, toSummary(it))
	}

	m.evtBus.Publish(events.New(events.EvtQueueChanged, events.QueueChangedPayload{
		Size:   size,
		Reason: reason,
		Items:  items,
	}))

	// Update the State Manager's queue info.
	nextID := ""
	if size > 0 {
		nextID = m.pending[0].QueueItemID
	}
	m.stateMgr.SetQueueInfo(size, nextID)
}

// toSummary converts a QueueItem to a QueueItemSummary for event payloads.
func toSummary(q *QueueItem) events.QueueItemSummary {
	return events.QueueItemSummary{
		QueueItemID: q.QueueItemID,
		AssetID:     q.AssetID,
		Title:       q.Title,
		Type:        string(q.Type),
		DurationMS:  q.DurationMS,
	}
}

// newItem converts a QueueItemInput to a QueueItem with an assigned ID.
func newItem(inp commands.QueueItemInput) *QueueItem {
	t := AssetType(inp.Type)
	if t == "" {
		t = AssetTypeUnknown
	}

	var tr TransitionSpec
	if inp.Transition != nil {
		tr = TransitionSpec{
			Type:       TransitionType(inp.Transition.Type),
			DurationMS: inp.Transition.DurationMS,
		}
	}

	var meta map[string]string
	if len(inp.Metadata) > 0 {
		meta = make(map[string]string, len(inp.Metadata))
		for k, v := range inp.Metadata {
			meta[k] = v
		}
	}

	return &QueueItem{
		QueueItemID: "qi_" + ulid.Make().String(),
		AssetID:     inp.AssetID,
		Path:        inp.Path,
		Type:        t,
		Title:       inp.Title,
		Artist:      inp.Artist,
		ISRC:        inp.ISRC,
		Composer:    inp.Composer,
		Publisher:   inp.Publisher,
		DurationMS:  inp.DurationMS,
		CueInMS:     inp.CueInMS,
		IntroMS:     inp.IntroMS,
		OutroMS:     inp.OutroMS,
		CueOutMS:    inp.CueOutMS,
		GainDB:      inp.GainDB,
		Transition:  tr,
		Mandatory:   inp.Mandatory,
		Metadata:    meta,
		Status:      ItemStatusQueued,
	}
}

// --- Persistence helpers -----------------------------------------------------

// snapshotLocked returns a Snapshot of the current queue state.
// Must be called with m.mu held.
func (m *Manager) snapshotLocked() Snapshot {
	currentID := ""
	if m.current != nil {
		currentID = m.current.QueueItemID
	}

	items := make([]QueueItem, 0, len(m.pending)+1)
	if m.current != nil {
		items = append(items, *m.current)
	}
	for _, it := range m.pending {
		items = append(items, *it)
	}

	return Snapshot{
		SchemaVersion: CurrentSchemaVersion,
		CurrentItemID: currentID,
		Items:         items,
	}
}

// persist saves the current queue state to the store.
// Errors are logged but never propagated — disk failures must not stop playback.
func (m *Manager) persist() {
	// Called with m.mu held.
	m.persistSeq++
	seq := m.persistSeq
	snap := m.snapshotLocked()
	go func() {
		// Serialise all disk writes through persistMu so that only the
		// latest snapshot wins: once this goroutine acquires the lock it
		// rechecks whether a newer request has already been queued and
		// skips the write if so.
		m.persistMu.Lock()
		defer m.persistMu.Unlock()

		m.mu.Lock()
		latest := m.persistSeq
		m.mu.Unlock()

		if seq < latest {
			return // a newer goroutine will write the up-to-date snapshot
		}
		if err := m.store.Save(snap); err != nil {
			m.log.Error("queue persistence: save failed", "error", err)
		}
	}()
}

// RestoreFrom restores queue state from a Snapshot loaded at startup.
// Must be called before any Play command and before any other queue operation.
// The item that was playing (CurrentItemID) is placed at the front of the
// pending queue — not as current — because the engine must issue a new Play
// command to start audio.
func (m *Manager) RestoreFrom(snap Snapshot) {
	if len(snap.Items) == 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pending = m.pending[:0]
	m.current = nil

	for i := range snap.Items {
		item := snap.Items[i] // copy

		// Skip items that already finished.
		switch item.Status {
		case ItemStatusPlayed, ItemStatusSkipped, ItemStatusFailed, ItemStatusMissed:
			continue
		}

		// Items that were playing go to the front of the pending queue
		// (they will restart from the beginning on the next Play).
		if item.QueueItemID == snap.CurrentItemID {
			item.Status = ItemStatusQueued
			m.pending = append([]*QueueItem{&item}, m.pending...)
		} else {
			item.Status = ItemStatusQueued
			m.pending = append(m.pending, &item)
		}
	}

	m.log.Info("queue restored from snapshot",
		"items", len(m.pending),
		"previous_current", snap.CurrentItemID,
	)
}
