package queue_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/queue"
)

func newTempStore(t *testing.T) *queue.FileStore {
	t.Helper()
	dir := t.TempDir()
	return queue.NewFileStore(filepath.Join(dir, "queue.json"))
}

func sampleSnapshot() queue.Snapshot {
	return queue.Snapshot{
		SchemaVersion: queue.CurrentSchemaVersion,
		SavedAt:       time.Now().UTC().Truncate(time.Millisecond),
		CurrentItemID: "qi_001",
		Items: []queue.QueueItem{
			{
				QueueItemID: "qi_001",
				AssetID:     "asset-a",
				Path:        "/lib/music-a.mp3",
				Type:        queue.AssetTypeMusic,
				Title:       "Music A",
				DurationMS:  240000,
				Status:      queue.ItemStatusPlaying,
			},
			{
				QueueItemID: "qi_002",
				AssetID:     "asset-b",
				Path:        "/lib/music-b.mp3",
				Type:        queue.AssetTypeMusic,
				Title:       "Music B",
				DurationMS:  200000,
				Status:      queue.ItemStatusQueued,
			},
		},
	}
}

func TestFileStore_SaveLoad_RoundTrip(t *testing.T) {
	fs := newTempStore(t)
	original := sampleSnapshot()

	if err := fs.Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.SchemaVersion != queue.CurrentSchemaVersion {
		t.Errorf("schema_version: want %d, got %d", queue.CurrentSchemaVersion, loaded.SchemaVersion)
	}
	if loaded.CurrentItemID != original.CurrentItemID {
		t.Errorf("current_item_id: want %q, got %q", original.CurrentItemID, loaded.CurrentItemID)
	}
	if len(loaded.Items) != len(original.Items) {
		t.Fatalf("items len: want %d, got %d", len(original.Items), len(loaded.Items))
	}
	for i, want := range original.Items {
		got := loaded.Items[i]
		if got.QueueItemID != want.QueueItemID {
			t.Errorf("items[%d].queue_item_id: want %q, got %q", i, want.QueueItemID, got.QueueItemID)
		}
		if got.Path != want.Path {
			t.Errorf("items[%d].path: want %q, got %q", i, want.Path, got.Path)
		}
		if got.Status != want.Status {
			t.Errorf("items[%d].status: want %s, got %s", i, want.Status, got.Status)
		}
	}
}

func TestFileStore_Load_Missing_ReturnsEmpty(t *testing.T) {
	fs := newTempStore(t)

	snap, err := fs.Load()
	if err != nil {
		t.Fatalf("Load on missing file should return empty, got error: %v", err)
	}
	if len(snap.Items) != 0 {
		t.Errorf("expected empty items, got %d", len(snap.Items))
	}
	if snap.SchemaVersion != 0 {
		t.Errorf("expected zero schema_version, got %d", snap.SchemaVersion)
	}
}

func TestFileStore_Load_Corrupted_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "queue.json")
	if err := os.WriteFile(path, []byte("{not valid json{{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs := queue.NewFileStore(path)

	_, err := fs.Load()
	if err == nil {
		t.Fatal("expected error for corrupted file, got nil")
	}
}

func TestFileStore_Load_FutureSchema_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "queue.json")
	data := []byte(`{"schema_version":9999,"saved_at":"2026-01-01T00:00:00Z","items":[]}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	fs := queue.NewFileStore(path)

	_, err := fs.Load()
	if err == nil {
		t.Fatal("expected error for future schema version, got nil")
	}
}

func TestFileStore_Clear_RemovesFile(t *testing.T) {
	fs := newTempStore(t)

	if err := fs.Save(sampleSnapshot()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(fs.Path()); err != nil {
		t.Fatalf("file should exist after Save: %v", err)
	}

	if err := fs.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(fs.Path()); !os.IsNotExist(err) {
		t.Error("file should not exist after Clear")
	}
}

func TestFileStore_Clear_MissingFile_NoError(t *testing.T) {
	fs := newTempStore(t)
	if err := fs.Clear(); err != nil {
		t.Errorf("Clear on missing file should not error: %v", err)
	}
}

func TestFileStore_Save_IsAtomic(t *testing.T) {
	// Verify that no .tmp file remains after a successful Save.
	fs := newTempStore(t)

	if err := fs.Save(sampleSnapshot()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tmpPath := fs.Path() + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not remain after successful Save")
	}
}

func TestFileStore_Save_CreatesParentDir(t *testing.T) {
	base := t.TempDir()
	deep := filepath.Join(base, "a", "b", "c", "queue.json")
	fs := queue.NewFileStore(deep)

	if err := fs.Save(sampleSnapshot()); err != nil {
		t.Fatalf("Save with nested dir: %v", err)
	}
	if _, err := os.Stat(deep); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestFileStore_ConcurrentSaves_NoCorruption(t *testing.T) {
	fs := newTempStore(t)
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = fs.Save(sampleSnapshot())
		}()
	}
	wg.Wait()

	// File must be valid JSON after all concurrent writes.
	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("concurrent saves left corrupted file: %v", err)
	}
	if loaded.SchemaVersion != queue.CurrentSchemaVersion {
		t.Errorf("schema_version after concurrent saves: %d", loaded.SchemaVersion)
	}
}

func TestFileStore_Overwrite_PreservesLastWrite(t *testing.T) {
	fs := newTempStore(t)

	snap1 := sampleSnapshot()
	snap1.CurrentItemID = "first"
	if err := fs.Save(snap1); err != nil {
		t.Fatalf("Save 1: %v", err)
	}

	snap2 := sampleSnapshot()
	snap2.CurrentItemID = "second"
	if err := fs.Save(snap2); err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.CurrentItemID != "second" {
		t.Errorf("want current_item_id=second, got %q", loaded.CurrentItemID)
	}
}
