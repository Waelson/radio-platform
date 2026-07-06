package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileStore persists scheduled entries to a JSON file on disk.
// Writes are atomic: data is written to a .tmp file, then renamed to the
// target path. This prevents corruption on crash during a write.
type FileStore struct {
	path string
}

// NewFileStore creates a FileStore that reads/writes the given path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// DefaultStorePath returns the default schedule persistence path:
// ~/RadioFlow/schedule.json (platform-independent via os.UserHomeDir).
func DefaultStorePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "RadioFlow", "schedule.json")
}

type storeDoc struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

// Load reads entries from the file.
// Returns an empty (nil) slice when the file does not exist — not an error.
func (s *FileStore) Load() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scheduler store: read %s: %w", s.path, err)
	}
	var doc storeDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("scheduler store: parse %s: %w", s.path, err)
	}
	return doc.Entries, nil
}

// Save atomically writes entries to the configured path.
// The parent directory must exist; this function does not create it.
func (s *FileStore) Save(entries []Entry) error {
	doc := storeDoc{Version: 1, Entries: entries}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("scheduler store: marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("scheduler store: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("scheduler store: rename %s → %s: %w", tmp, s.path, err)
	}
	return nil
}
