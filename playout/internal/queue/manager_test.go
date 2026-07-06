package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

func newManager(t *testing.T) (*queue.Manager, *events.Bus) {
	t.Helper()
	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test")
	return queue.NewManager(evtBus, stateMgr, nil), evtBus
}

func itemInput(title string) commands.QueueItemInput {
	return commands.QueueItemInput{
		AssetID:    "asset_" + title,
		Path:       "/audio/" + title + ".mp3",
		Type:       "musicas",
		Title:      title,
		DurationMS: 180000,
	}
}

func waitQueueChanged(t *testing.T, ch <-chan events.Event) events.QueueChangedPayload {
	t.Helper()
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case evt := <-ch:
			if evt.Type == events.EvtQueueChanged {
				return evt.Payload.(events.QueueChangedPayload)
			}
		case <-deadline:
			t.Fatal("timed out waiting for QueueChanged event")
		}
	}
}

// --- Enqueue -----------------------------------------------------------------

func TestEnqueue_AppendsItems(t *testing.T) {
	mgr, _ := newManager(t)

	items := mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})

	if len(items) != 2 {
		t.Fatalf("Enqueue returned %d items, want 2", len(items))
	}
	if mgr.Size() != 2 {
		t.Errorf("Size = %d, want 2", mgr.Size())
	}
	for _, it := range items {
		if it.QueueItemID == "" {
			t.Error("QueueItemID should be assigned")
		}
		if it.Status != queue.ItemStatusQueued {
			t.Errorf("Status = %s, want QUEUED", it.Status)
		}
	}
}

func TestEnqueue_AssignsUniqueIDs(t *testing.T) {
	mgr, _ := newManager(t)

	items := mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B"), itemInput("C")})
	ids := map[string]bool{}
	for _, it := range items {
		if ids[it.QueueItemID] {
			t.Errorf("duplicate QueueItemID %q", it.QueueItemID)
		}
		ids[it.QueueItemID] = true
	}
}

func TestEnqueue_PublishesQueueChanged(t *testing.T) {
	mgr, evtBus := newManager(t)

	ch, cancel := evtBus.Subscribe(8)
	defer cancel()

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A")})

	p := waitQueueChanged(t, ch)
	if p.Size != 1 {
		t.Errorf("QueueChanged.Size = %d, want 1", p.Size)
	}
	if p.Reason != "enqueue" {
		t.Errorf("QueueChanged.Reason = %q", p.Reason)
	}
	if len(p.Items) != 1 {
		t.Errorf("QueueChanged.Items len = %d, want 1", len(p.Items))
	}
}

func TestEnqueue_OrderPreserved(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B"), itemInput("C")})

	list := mgr.List()
	titles := make([]string, len(list))
	for i, it := range list {
		titles[i] = it.Title
	}
	want := []string{"A", "B", "C"}
	for i, w := range want {
		if titles[i] != w {
			t.Errorf("item[%d].Title = %q, want %q", i, titles[i], w)
		}
	}
}

// --- InsertNext --------------------------------------------------------------

func TestInsertNext_PlacesAtFront(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	mgr.InsertNext(itemInput("X"))

	list := mgr.List()
	if list[0].Title != "X" {
		t.Errorf("first item = %q, want X", list[0].Title)
	}
}

func TestInsertNext_PublishesQueueChanged(t *testing.T) {
	mgr, evtBus := newManager(t)

	ch, cancel := evtBus.Subscribe(8)
	defer cancel()

	mgr.InsertNext(itemInput("X"))
	waitQueueChanged(t, ch)
}

// --- InsertAfter -------------------------------------------------------------

func TestInsertAfter_PlacesAfterTarget(t *testing.T) {
	mgr, _ := newManager(t)

	items := mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B"), itemInput("C")})
	bID := items[1].QueueItemID

	_, err := mgr.InsertAfter(bID, itemInput("X"))
	if err != nil {
		t.Fatalf("InsertAfter: %v", err)
	}

	list := mgr.List()
	titles := make([]string, len(list))
	for i, it := range list {
		titles[i] = it.Title
	}
	// Expected order: A, B, X, C
	want := []string{"A", "B", "X", "C"}
	for i, w := range want {
		if titles[i] != w {
			t.Errorf("item[%d] = %q, want %q", i, titles[i], w)
		}
	}
}

func TestInsertAfter_EquivalentToInsertNextWhenAfterCurrent(t *testing.T) {
	mgr, _ := newManager(t)

	items := mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	// Pop A so it becomes current.
	popped, ok := mgr.Pop()
	if !ok {
		t.Fatal("Pop returned false")
	}
	mgr.SetCurrent(popped)

	aID := items[0].QueueItemID
	_, err := mgr.InsertAfter(aID, itemInput("X"))
	if err != nil {
		t.Fatalf("InsertAfter current: %v", err)
	}

	// Pending should be [X, B].
	list := mgr.List()
	if list[0].Title != "A" { // current
		t.Errorf("list[0] = %q, want A (current)", list[0].Title)
	}
	if list[1].Title != "X" {
		t.Errorf("list[1] = %q, want X", list[1].Title)
	}
	if list[2].Title != "B" {
		t.Errorf("list[2] = %q, want B", list[2].Title)
	}
}

func TestInsertAfter_ErrorOnUnknownID(t *testing.T) {
	mgr, _ := newManager(t)

	_, err := mgr.InsertAfter("qi_unknown", itemInput("X"))
	if err == nil {
		t.Fatal("expected error for unknown ID, got nil")
	}
}

// --- Clear -------------------------------------------------------------------

func TestClear_RemovesPendingItems(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	n := mgr.Clear(true)

	if n != 2 {
		t.Errorf("Clear returned %d, want 2", n)
	}
	if mgr.Size() != 0 {
		t.Errorf("Size after clear = %d, want 0", mgr.Size())
	}
}

func TestClear_PreservesCurrentWhenTrue(t *testing.T) {
	mgr, _ := newManager(t)

	items := mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	popped, _ := mgr.Pop()
	_ = items
	mgr.SetCurrent(popped)

	mgr.Clear(true)

	if !mgr.HasCurrent() {
		t.Error("current should be preserved when preserveCurrent=true")
	}
	if mgr.Size() != 0 {
		t.Errorf("pending size = %d, want 0", mgr.Size())
	}
}

func TestClear_ClearsCurrentWhenFalse(t *testing.T) {
	mgr, _ := newManager(t)

	items := mgr.Enqueue([]commands.QueueItemInput{itemInput("A")})
	_ = items
	popped, _ := mgr.Pop()
	mgr.SetCurrent(popped)

	mgr.Clear(false)

	if mgr.HasCurrent() {
		t.Error("current should be cleared when preserveCurrent=false")
	}
}

func TestClear_PublishesQueueChanged(t *testing.T) {
	mgr, evtBus := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A")})

	ch, cancel := evtBus.Subscribe(8)
	defer cancel()

	mgr.Clear(true)
	p := waitQueueChanged(t, ch)
	if p.Size != 0 {
		t.Errorf("QueueChanged.Size = %d after clear, want 0", p.Size)
	}
}

// --- Pop / SetCurrent / ClearCurrent -----------------------------------------

func TestPop_RemovesFirstItem(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	item, ok := mgr.Pop()
	if !ok {
		t.Fatal("Pop returned false")
	}
	if item.Title != "A" {
		t.Errorf("popped item = %q, want A", item.Title)
	}
	if mgr.Size() != 1 {
		t.Errorf("Size after pop = %d, want 1", mgr.Size())
	}
}

func TestPop_EmptyQueue(t *testing.T) {
	mgr, _ := newManager(t)

	_, ok := mgr.Pop()
	if ok {
		t.Error("Pop on empty queue should return false")
	}
}

func TestPopAsCurrent_AtomicallySetsCurrentAndRemovesPending(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	item, ok := mgr.PopAsCurrent()
	if !ok {
		t.Fatal("PopAsCurrent returned false")
	}
	if item.Title != "A" {
		t.Errorf("popped item = %q, want A", item.Title)
	}
	if item.Status != queue.ItemStatusPlaying {
		t.Errorf("item.Status = %q, want PLAYING", item.Status)
	}
	// List must include the current item (A) at the top, and pending item (B)
	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2", len(list))
	}
	if list[0].Title != "A" || list[0].Status != queue.ItemStatusPlaying {
		t.Errorf("list[0] = %q/%q, want A/PLAYING", list[0].Title, list[0].Status)
	}
	if list[1].Title != "B" {
		t.Errorf("list[1] = %q, want B", list[1].Title)
	}
	if mgr.Size() != 1 {
		t.Errorf("Size after PopAsCurrent = %d, want 1 (only pending)", mgr.Size())
	}
}

func TestPopAsCurrent_EmptyQueue(t *testing.T) {
	mgr, _ := newManager(t)

	_, ok := mgr.PopAsCurrent()
	if ok {
		t.Error("PopAsCurrent on empty queue should return false")
	}
}

func TestSetCurrent_SetsStatusPlaying(t *testing.T) {
	mgr, _ := newManager(t)

	items := mgr.Enqueue([]commands.QueueItemInput{itemInput("A")})
	popped, _ := mgr.Pop()
	_ = items
	mgr.SetCurrent(popped)

	cur := mgr.Current()
	if cur == nil {
		t.Fatal("Current() should not be nil after SetCurrent")
	}
	if cur.Status != queue.ItemStatusPlaying {
		t.Errorf("Status = %s, want PLAYING", cur.Status)
	}
}

func TestClearCurrent(t *testing.T) {
	mgr, _ := newManager(t)

	items := mgr.Enqueue([]commands.QueueItemInput{itemInput("A")})
	_ = items
	popped, _ := mgr.Pop()
	mgr.SetCurrent(popped)
	mgr.ClearCurrent()

	if mgr.HasCurrent() {
		t.Error("HasCurrent should be false after ClearCurrent")
	}
}

// --- Peek / List / Size / HasCurrent -----------------------------------------

func TestPeek_DoesNotRemoveItem(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	item, ok := mgr.Peek()
	if !ok {
		t.Fatal("Peek returned false")
	}
	if item.Title != "A" {
		t.Errorf("Peek item = %q, want A", item.Title)
	}
	if mgr.Size() != 2 {
		t.Error("Peek should not remove item")
	}
}

func TestList_IncludesCurrent(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	popped, _ := mgr.Pop()
	mgr.SetCurrent(popped)

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2 (current + 1 pending)", len(list))
	}
	if list[0].Title != "A" {
		t.Errorf("list[0] should be current item A")
	}
}

func TestList_ReturnsSnapshot(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A")})
	list := mgr.List()

	// Mutating the returned slice should not affect the manager.
	list[0].Title = "mutated"
	if mgr.List()[0].Title == "mutated" {
		t.Error("List snapshot mutation leaked into manager state")
	}
}

// --- Command handlers --------------------------------------------------------

func TestHandleEnqueue_Success(t *testing.T) {
	mgr, _ := newManager(t)

	cmd := commands.Command{
		Type: commands.CmdEnqueue,
		Payload: commands.EnqueuePayload{
			Items: []commands.QueueItemInput{itemInput("A")},
		},
	}
	err := mgr.HandleEnqueue(context.Background(), cmd)
	if err != nil {
		t.Fatalf("HandleEnqueue: %v", err)
	}
	if mgr.Size() != 1 {
		t.Error("expected 1 item after HandleEnqueue")
	}
}

func TestHandleEnqueue_EmptyItems_Error(t *testing.T) {
	mgr, _ := newManager(t)

	cmd := commands.Command{
		Type:    commands.CmdEnqueue,
		Payload: commands.EnqueuePayload{Items: nil},
	}
	err := mgr.HandleEnqueue(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error for empty items")
	}
}

func TestHandleInsertNext_Success(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A")})
	cmd := commands.Command{
		Type:    commands.CmdInsertNext,
		Payload: commands.InsertNextPayload{Item: itemInput("X")},
	}
	if err := mgr.HandleInsertNext(context.Background(), cmd); err != nil {
		t.Fatalf("HandleInsertNext: %v", err)
	}
	list := mgr.List()
	if list[0].Title != "X" {
		t.Errorf("first item = %q, want X", list[0].Title)
	}
}

func TestHandleInsertAfter_Success(t *testing.T) {
	mgr, _ := newManager(t)

	items := mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	cmd := commands.Command{
		Type: commands.CmdInsertAfter,
		Payload: commands.InsertAfterPayload{
			AfterQueueItemID: items[0].QueueItemID,
			Item:             itemInput("X"),
		},
	}
	if err := mgr.HandleInsertAfter(context.Background(), cmd); err != nil {
		t.Fatalf("HandleInsertAfter: %v", err)
	}
	list := mgr.List()
	if list[1].Title != "X" {
		t.Errorf("list[1] = %q, want X", list[1].Title)
	}
}

func TestHandleClear_Success(t *testing.T) {
	mgr, _ := newManager(t)

	mgr.Enqueue([]commands.QueueItemInput{itemInput("A"), itemInput("B")})
	cmd := commands.Command{
		Type:    commands.CmdClearQueue,
		Payload: commands.ClearQueuePayload{PreserveCurrent: true},
	}
	if err := mgr.HandleClear(context.Background(), cmd); err != nil {
		t.Fatalf("HandleClear: %v", err)
	}
	if mgr.Size() != 0 {
		t.Error("expected empty queue after HandleClear")
	}
}

// --- InsertBreakNext ---------------------------------------------------------

func breakInput(title string, spots ...string) commands.BreakItemInput {
	b := commands.BreakItemInput{Title: title}
	for _, s := range spots {
		b.Spots = append(b.Spots, commands.QueueItemInput{
			Path:       "/audio/" + s + ".mp3",
			Type:       "spots",
			Title:      s,
			DurationMS: 30000,
		})
	}
	return b
}

func TestInsertBreakNext_Order(t *testing.T) {
	mgr, _ := newManager(t)

	// Pre-existing items in the queue.
	mgr.Enqueue([]commands.QueueItemInput{itemInput("Existing")})

	open := commands.QueueItemInput{Path: "/open.mp3", Type: "jingles", Title: "Open"}
	close_ := commands.QueueItemInput{Path: "/close.mp3", Type: "jingles", Title: "Close"}
	b := commands.BreakItemInput{
		Title: "Bloco",
		Open:  &open,
		Spots: []commands.QueueItemInput{
			{Path: "/spot1.mp3", Type: "spots", Title: "Spot1", DurationMS: 30000},
			{Path: "/spot2.mp3", Type: "spots", Title: "Spot2", DurationMS: 30000},
		},
		Close: &close_,
	}

	cmd := commands.Command{
		Type:    commands.CmdInsertBreakNext,
		Payload: commands.InsertBreakNextPayload{Break: b},
	}
	if err := mgr.HandleInsertBreakNext(context.Background(), cmd); err != nil {
		t.Fatalf("HandleInsertBreakNext: %v", err)
	}

	// Queue must be: Open, Spot1, Spot2, Close, Existing
	list := mgr.List()
	if len(list) != 5 {
		t.Fatalf("expected 5 items, got %d", len(list))
	}
	wantOrder := []string{"Open", "Spot1", "Spot2", "Close", "Existing"}
	for i, title := range wantOrder {
		if list[i].Title != title {
			t.Errorf("list[%d].Title = %q, want %q", i, list[i].Title, title)
		}
	}
}

func TestInsertBreakNext_SharedBreakID(t *testing.T) {
	mgr, _ := newManager(t)

	cmd := commands.Command{
		Type:    commands.CmdInsertBreakNext,
		Payload: commands.InsertBreakNextPayload{Break: breakInput("Bloco", "Spot1", "Spot2")},
	}
	if err := mgr.HandleInsertBreakNext(context.Background(), cmd); err != nil {
		t.Fatalf("HandleInsertBreakNext: %v", err)
	}

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	if list[0].BreakID == "" {
		t.Error("BreakID should not be empty")
	}
	if list[0].BreakID != list[1].BreakID {
		t.Errorf("BreakID mismatch: %q != %q", list[0].BreakID, list[1].BreakID)
	}
}

func TestInsertBreakNext_BreakRolesAndSeq(t *testing.T) {
	mgr, _ := newManager(t)

	open := commands.QueueItemInput{Path: "/open.mp3", Type: "jingles", Title: "Open"}
	close_ := commands.QueueItemInput{Path: "/close.mp3", Type: "jingles", Title: "Close"}
	b := commands.BreakItemInput{
		Title: "Bloco",
		Open:  &open,
		Spots: []commands.QueueItemInput{{Path: "/spot.mp3", Type: "spots", Title: "Spot"}},
		Close: &close_,
	}
	cmd := commands.Command{
		Type:    commands.CmdInsertBreakNext,
		Payload: commands.InsertBreakNextPayload{Break: b},
	}
	if err := mgr.HandleInsertBreakNext(context.Background(), cmd); err != nil {
		t.Fatalf("HandleInsertBreakNext: %v", err)
	}

	list := mgr.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 items, got %d", len(list))
	}

	wantRoles := []string{"open", "spot", "close"}
	for i, role := range wantRoles {
		if list[i].BreakRole != role {
			t.Errorf("list[%d].BreakRole = %q, want %q", i, list[i].BreakRole, role)
		}
		if list[i].BreakSeq != i+1 {
			t.Errorf("list[%d].BreakSeq = %d, want %d", i, list[i].BreakSeq, i+1)
		}
		if list[i].BreakTotal != 3 {
			t.Errorf("list[%d].BreakTotal = %d, want 3", i, list[i].BreakTotal)
		}
	}
}

func TestInsertBreakNext_NoSpots_ReturnsError(t *testing.T) {
	mgr, _ := newManager(t)

	cmd := commands.Command{
		Type: commands.CmdInsertBreakNext,
		Payload: commands.InsertBreakNextPayload{
			Break: commands.BreakItemInput{Title: "Empty"},
		},
	}
	if err := mgr.HandleInsertBreakNext(context.Background(), cmd); err == nil {
		t.Fatal("expected error for break with no spots")
	}
}

func TestInsertBreakNext_FirstSpotGetsCrossfadeWhenNoOpen(t *testing.T) {
	mgr, _ := newManager(t)

	cmd := commands.Command{
		Type:    commands.CmdInsertBreakNext,
		Payload: commands.InsertBreakNextPayload{Break: breakInput("Bloco", "Spot1", "Spot2")},
	}
	if err := mgr.HandleInsertBreakNext(context.Background(), cmd); err != nil {
		t.Fatalf("HandleInsertBreakNext: %v", err)
	}

	list := mgr.List()
	if list[0].Transition.Type != queue.TransitionCrossfade {
		t.Errorf("first spot transition = %q, want CROSSFADE", list[0].Transition.Type)
	}
	if list[1].Transition.Type != queue.TransitionCut {
		t.Errorf("second spot transition = %q, want CUT", list[1].Transition.Type)
	}
}
