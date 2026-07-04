package queue_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/queue"
)

func TestSnapshot_JSON_RoundTrip(t *testing.T) {
	original := queue.Snapshot{
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
				Artist:      "Artist A",
				DurationMS:  240000,
				CueInMS:     0,
				CueOutMS:    235000,
				Mandatory:   false,
				Status:      queue.ItemStatusPlaying,
			},
			{
				QueueItemID: "qi_002",
				AssetID:     "asset-b",
				Path:        "/lib/jingle.mp3",
				Type:        queue.AssetTypeJingle,
				Title:       "Jingle",
				DurationMS:  15000,
				Status:      queue.ItemStatusQueued,
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored queue.Snapshot
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.SchemaVersion != original.SchemaVersion {
		t.Errorf("schema_version: want %d, got %d", original.SchemaVersion, restored.SchemaVersion)
	}
	if restored.CurrentItemID != original.CurrentItemID {
		t.Errorf("current_item_id: want %q, got %q", original.CurrentItemID, restored.CurrentItemID)
	}
	if len(restored.Items) != len(original.Items) {
		t.Fatalf("items len: want %d, got %d", len(original.Items), len(restored.Items))
	}
	for i, item := range original.Items {
		got := restored.Items[i]
		if got.QueueItemID != item.QueueItemID {
			t.Errorf("items[%d].queue_item_id: want %q, got %q", i, item.QueueItemID, got.QueueItemID)
		}
		if got.Path != item.Path {
			t.Errorf("items[%d].path: want %q, got %q", i, item.Path, got.Path)
		}
		if got.Status != item.Status {
			t.Errorf("items[%d].status: want %q, got %q", i, item.Status, got.Status)
		}
	}
}

func TestSnapshot_Empty_SerializesCleanly(t *testing.T) {
	snap := queue.Snapshot{
		SchemaVersion: queue.CurrentSchemaVersion,
		SavedAt:       time.Now().UTC(),
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored queue.Snapshot
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(restored.Items) != 0 {
		t.Errorf("expected empty items, got %d", len(restored.Items))
	}
	if restored.CurrentItemID != "" {
		t.Errorf("expected empty current_item_id, got %q", restored.CurrentItemID)
	}
}

func TestNopStore(t *testing.T) {
	var s queue.NopStore
	snap := queue.Snapshot{SchemaVersion: 1, SavedAt: time.Now()}

	if err := s.Save(snap); err != nil {
		t.Errorf("Save: %v", err)
	}
	loaded, err := s.Load()
	if err != nil {
		t.Errorf("Load: %v", err)
	}
	if loaded.SchemaVersion != 0 {
		t.Errorf("NopStore.Load should return zero Snapshot, got %+v", loaded)
	}
	if err := s.Clear(); err != nil {
		t.Errorf("Clear: %v", err)
	}
}
