package queue_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// newPersistentManager creates a Manager wired to a temporary FileStore.
func newPersistentManager(t *testing.T) (*queue.Manager, *queue.FileStore) {
	t.Helper()
	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test")
	mgr := queue.NewManager(evtBus, stateMgr, nil)
	store := queue.NewFileStore(filepath.Join(t.TempDir(), "queue.json"))
	mgr.WithStore(store)
	return mgr, store
}

func enqueueItem(mgr *queue.Manager, path, title string) {
	mgr.Enqueue([]commands.QueueItemInput{{
		Path:       path,
		Title:      title,
		AssetID:    "asset-" + title,
		DurationMS: 240000,
	}})
}

// TestPersist_AfterEnqueue verifies that the snapshot is saved after Enqueue.
func TestPersist_AfterEnqueue(t *testing.T) {
	mgr, store := newPersistentManager(t)
	enqueueItem(mgr, "/lib/a.mp3", "A")
	enqueueItem(mgr, "/lib/b.mp3", "B")

	// Give the background goroutine time to write.
	time.Sleep(50 * time.Millisecond)

	snap, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(snap.Items) != 2 {
		t.Errorf("want 2 items in snapshot, got %d", len(snap.Items))
	}
}

// TestPersist_AfterClear verifies that the snapshot is updated after Clear.
func TestPersist_AfterClear(t *testing.T) {
	mgr, store := newPersistentManager(t)
	enqueueItem(mgr, "/lib/a.mp3", "A")
	time.Sleep(50 * time.Millisecond)

	mgr.Clear(false)
	time.Sleep(50 * time.Millisecond)

	snap, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(snap.Items) != 0 {
		t.Errorf("want 0 items after clear, got %d", len(snap.Items))
	}
}

// TestRestore_PreservesOrder verifies that RestoreFrom puts items in queue in
// the original order, skipping PLAYED/FAILED items.
func TestRestore_PreservesOrder(t *testing.T) {
	snap := queue.Snapshot{
		SchemaVersion: queue.CurrentSchemaVersion,
		Items: []queue.QueueItem{
			{QueueItemID: "qi_1", Path: "/lib/a.mp3", Title: "A", Status: queue.ItemStatusQueued},
			{QueueItemID: "qi_2", Path: "/lib/b.mp3", Title: "B", Status: queue.ItemStatusPlayed}, // skip
			{QueueItemID: "qi_3", Path: "/lib/c.mp3", Title: "C", Status: queue.ItemStatusQueued},
		},
	}

	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test")
	mgr := queue.NewManager(evtBus, stateMgr, nil)
	mgr.RestoreFrom(snap)

	items := mgr.List()
	if len(items) != 2 {
		t.Fatalf("want 2 items (skipping PLAYED), got %d", len(items))
	}
	if items[0].QueueItemID != "qi_1" {
		t.Errorf("items[0]: want qi_1, got %s", items[0].QueueItemID)
	}
	if items[1].QueueItemID != "qi_3" {
		t.Errorf("items[1]: want qi_3, got %s", items[1].QueueItemID)
	}
}

// TestRestore_CurrentItemGoesToFront verifies that the item that was PLAYING
// when the snapshot was taken is moved to the front of the queue.
func TestRestore_CurrentItemGoesToFront(t *testing.T) {
	snap := queue.Snapshot{
		SchemaVersion: queue.CurrentSchemaVersion,
		CurrentItemID: "qi_2",
		Items: []queue.QueueItem{
			{QueueItemID: "qi_1", Path: "/lib/a.mp3", Title: "A", Status: queue.ItemStatusQueued},
			{QueueItemID: "qi_2", Path: "/lib/b.mp3", Title: "B", Status: queue.ItemStatusPlaying},
			{QueueItemID: "qi_3", Path: "/lib/c.mp3", Title: "C", Status: queue.ItemStatusQueued},
		},
	}

	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test")
	mgr := queue.NewManager(evtBus, stateMgr, nil)
	mgr.RestoreFrom(snap)

	items := mgr.List()
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}
	// qi_2 (was PLAYING) should be first.
	if items[0].QueueItemID != "qi_2" {
		t.Errorf("items[0]: want qi_2 (was current), got %s", items[0].QueueItemID)
	}
	// All restored items must have status QUEUED.
	for i, it := range items {
		if it.Status != queue.ItemStatusQueued {
			t.Errorf("items[%d].status: want QUEUED, got %s", i, it.Status)
		}
	}
}

// TestRestore_EmptySnapshot_NoOp verifies that restoring an empty snapshot
// leaves the queue unchanged.
func TestRestore_EmptySnapshot_NoOp(t *testing.T) {
	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test")
	mgr := queue.NewManager(evtBus, stateMgr, nil)
	enqueueItem(mgr, "/lib/a.mp3", "A")

	mgr.RestoreFrom(queue.Snapshot{})

	if mgr.Size() != 1 {
		t.Errorf("empty restore should be a no-op, want 1 item, got %d", mgr.Size())
	}
}

// TestPersist_StoreError_DoesNotPropagateToQueue verifies that a failing store
// does not block or panic the queue.
func TestPersist_StoreError_DoesNotPropagateToQueue(t *testing.T) {
	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test")
	mgr := queue.NewManager(evtBus, stateMgr, nil)
	mgr.WithStore(&alwaysFailStore{})

	// These must not panic or block.
	enqueueItem(mgr, "/lib/a.mp3", "A")
	enqueueItem(mgr, "/lib/b.mp3", "B")
	mgr.Clear(false)

	time.Sleep(50 * time.Millisecond) // let background goroutines complete
}

// TestPersist_ConcurrentEnqueue_NoPanic fires concurrent Enqueue calls and
// verifies the manager and store remain consistent.
func TestPersist_ConcurrentEnqueue_NoPanic(t *testing.T) {
	mgr, store := newPersistentManager(t)
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			enqueueItem(mgr, "/lib/a.mp3", "A")
		}()
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	snap, err := store.Load()
	if err != nil {
		t.Fatalf("store corrupted after concurrent enqueues: %v", err)
	}
	if len(snap.Items) != goroutines {
		t.Errorf("want %d items in snapshot, got %d", goroutines, len(snap.Items))
	}
}

// alwaysFailStore is a Store that always returns an error from Save.
type alwaysFailStore struct{}

func (alwaysFailStore) Save(_ queue.Snapshot) error { return errForcedStoreFailure }
func (alwaysFailStore) Load() (queue.Snapshot, error) {
	return queue.Snapshot{}, errForcedStoreFailure
}
func (alwaysFailStore) Clear() error { return errForcedStoreFailure }

var errForcedStoreFailure = fmt.Errorf("forced store failure")
