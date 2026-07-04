package queue_test

import (
	"path/filepath"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// TestIntegration_CrashRecovery simulates a crash-and-restart cycle:
// 1. Engine A enqueues items and saves state.
// 2. Engine A "crashes" (Manager is discarded without clean shutdown).
// 3. Engine B starts, loads the snapshot, and restores the queue.
// 4. Engine B's queue must match what Engine A had enqueued.
func TestIntegration_CrashRecovery(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "queue.json")

	// --- Engine A: enqueue then "crash" (no clear) ---
	{
		mgr, _ := newManagerWithStore(t, storePath)
		enqueueItem(mgr, "/lib/a.mp3", "A")
		enqueueItem(mgr, "/lib/b.mp3", "B")
		enqueueItem(mgr, "/lib/c.mp3", "C")
		// Simulate that item A started playing: pop it from pending first,
		// then mark it as current (mirrors how the playback manager behaves).
		popped, _ := mgr.Pop()
		mgr.SetCurrent(popped)
		waitPersist(t, storePath, 3)
		// Engine A crashes — no Clear, no clean shutdown.
	}

	// --- Engine B: restore from snapshot ---
	{
		mgr, _ := newManagerWithStore(t, storePath)
		store := queue.NewFileStore(storePath)

		snap, err := store.Load()
		if err != nil {
			t.Fatalf("Engine B: Load snapshot: %v", err)
		}
		mgr.RestoreFrom(snap)

		items := mgr.List()
		if len(items) != 3 {
			t.Fatalf("Engine B: want 3 items after restore, got %d", len(items))
		}
		// The item that was PLAYING must be at the front.
		if items[0].Path != "/lib/a.mp3" {
			t.Errorf("Engine B: items[0] should be /lib/a.mp3 (was playing), got %s", items[0].Path)
		}
		if items[0].Status != queue.ItemStatusQueued {
			t.Errorf("Engine B: restored current item must be QUEUED, got %s", items[0].Status)
		}
	}
}

// TestIntegration_CleanShutdown_ClearOnStop verifies that after a clean
// shutdown (clear_on_stop=true), the snapshot file is removed and Engine B
// starts with an empty queue.
func TestIntegration_CleanShutdown_ClearOnStop(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "queue.json")

	// Engine A: enqueue, then clean shutdown → clear snapshot.
	{
		mgr, store := newManagerWithStore(t, storePath)
		enqueueItem(mgr, "/lib/a.mp3", "A")
		waitPersist(t, storePath, 1)
		// Clean shutdown.
		if err := store.Clear(); err != nil {
			t.Fatalf("Clear: %v", err)
		}
	}

	// Engine B: snapshot absent → empty queue.
	{
		store := queue.NewFileStore(storePath)
		snap, err := store.Load()
		if err != nil {
			t.Fatalf("Load after clear: %v", err)
		}
		if len(snap.Items) != 0 {
			t.Errorf("after clear, expect empty snapshot, got %d items", len(snap.Items))
		}
	}
}

// TestIntegration_SchemaVersion_Future verifies that a snapshot written by a
// newer engine version is rejected with a clear error.
func TestIntegration_SchemaVersion_Future(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "queue.json")

	// Write a snapshot with a future schema version directly.
	futureSnap := `{"schema_version":9999,"saved_at":"2030-01-01T00:00:00Z","items":[]}`
	if err := writeFile(t, storePath, futureSnap); err != nil {
		t.Fatal(err)
	}

	store := queue.NewFileStore(storePath)
	_, err := store.Load()
	if err == nil {
		t.Fatal("expected error for future schema version, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// TestIntegration_RestoreSkipsFinishedItems verifies that items with terminal
// statuses (PLAYED, FAILED, SKIPPED, MISSED) are dropped on restore.
func TestIntegration_RestoreSkipsFinishedItems(t *testing.T) {
	snap := queue.Snapshot{
		SchemaVersion: queue.CurrentSchemaVersion,
		Items: []queue.QueueItem{
			{QueueItemID: "qi_1", Path: "/a.mp3", Status: queue.ItemStatusQueued},
			{QueueItemID: "qi_2", Path: "/b.mp3", Status: queue.ItemStatusPlayed},
			{QueueItemID: "qi_3", Path: "/c.mp3", Status: queue.ItemStatusFailed},
			{QueueItemID: "qi_4", Path: "/d.mp3", Status: queue.ItemStatusSkipped},
			{QueueItemID: "qi_5", Path: "/e.mp3", Status: queue.ItemStatusMissed},
			{QueueItemID: "qi_6", Path: "/f.mp3", Status: queue.ItemStatusQueued},
		},
	}

	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test")
	mgr := queue.NewManager(evtBus, stateMgr, nil)
	mgr.RestoreFrom(snap)

	items := mgr.List()
	if len(items) != 2 {
		t.Fatalf("want 2 items (qi_1, qi_6), got %d: %+v", len(items), items)
	}
	if items[0].QueueItemID != "qi_1" || items[1].QueueItemID != "qi_6" {
		t.Errorf("unexpected order: %s, %s", items[0].QueueItemID, items[1].QueueItemID)
	}
}

// --- helpers -----------------------------------------------------------------

func newManagerWithStore(t *testing.T, storePath string) (*queue.Manager, *queue.FileStore) {
	t.Helper()
	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test")
	mgr := queue.NewManager(evtBus, stateMgr, nil)
	store := queue.NewFileStore(storePath)
	mgr.WithStore(store)
	return mgr, store
}

// waitPersist waits until the store contains at least wantItems items.
func waitPersist(t *testing.T, storePath string, wantItems int) {
	t.Helper()
	store := queue.NewFileStore(storePath)
	deadline := nowPlus(500)
	for nowMs() < deadline {
		snap, err := store.Load()
		if err == nil && len(snap.Items) >= wantItems {
			return
		}
		sleepMs(5)
	}
	t.Fatalf("snapshot not persisted with %d items within 500ms", wantItems)
}

func writeFile(t *testing.T, path, content string) error {
	t.Helper()
	return writeFileBytes(path, []byte(content))
}
