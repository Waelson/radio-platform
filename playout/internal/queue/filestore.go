package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileStore persists queue snapshots as JSON files using atomic writes
// (write to a temporary file then os.Rename to the target path).
// Rename is atomic on Unix and Windows — a crash mid-write never leaves
// a corrupt snapshot file.
// FileStore is safe for concurrent use.
type FileStore struct {
	mu   sync.Mutex
	path string // target file path
	tmp  string // temporary file path (same directory as path)
}

// NewFileStore creates a FileStore that persists snapshots at path.
// The parent directory is created automatically on the first Save.
func NewFileStore(path string) *FileStore {
	return &FileStore{
		path: path,
		tmp:  path + ".tmp",
	}
}

var _ Store = (*FileStore)(nil)

// Save atomically persists snap to disk.
// It writes JSON to a .tmp file in the same directory, then renames it to the
// target path. A crash between write and rename leaves only the .tmp file,
// which is ignored by Load.
func (fs *FileStore) Save(snap Snapshot) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	snap.SchemaVersion = CurrentSchemaVersion
	snap.SavedAt = time.Now().UTC()

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("queue filestore: marshal: %w", err)
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(fs.path), 0o755); err != nil {
		return fmt.Errorf("queue filestore: mkdir: %w", err)
	}

	// Write to temp file first.
	if err := os.WriteFile(fs.tmp, data, 0o644); err != nil {
		return fmt.Errorf("queue filestore: write temp: %w", err)
	}

	// Atomic rename to target path.
	if err := os.Rename(fs.tmp, fs.path); err != nil {
		_ = os.Remove(fs.tmp)
		return fmt.Errorf("queue filestore: rename: %w", err)
	}

	return nil
}

// Load reads and validates the last saved snapshot.
// Returns an empty Snapshot (no error) when no file exists yet.
// Returns an error if the file exists but is corrupted or uses an unknown schema version.
func (fs *FileStore) Load() (Snapshot, error) {
	data, err := os.ReadFile(fs.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, nil
		}
		return Snapshot{}, fmt.Errorf("queue filestore: read: %w", err)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("queue filestore: unmarshal (corrupted file at %s): %w", fs.path, err)
	}

	if snap.SchemaVersion > CurrentSchemaVersion {
		return Snapshot{}, fmt.Errorf(
			"queue filestore: snapshot schema version %d is newer than supported %d — upgrade the engine",
			snap.SchemaVersion, CurrentSchemaVersion,
		)
	}

	return snap, nil
}

// Clear removes the snapshot file. A missing file is not an error.
func (fs *FileStore) Clear() error {
	if err := os.Remove(fs.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("queue filestore: clear: %w", err)
	}
	_ = os.Remove(fs.tmp) // clean up any leftover temp file
	return nil
}

// Path returns the target file path.
func (fs *FileStore) Path() string { return fs.path }

// DefaultStorePath returns the conventional snapshot path for an engine ID.
func DefaultStorePath(engineID string) string {
	return fmt.Sprintf("/tmp/playout-%s-queue.json", engineID)
}
