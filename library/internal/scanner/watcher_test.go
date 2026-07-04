package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/config"
	"github.com/Waelson/radio-library-service/internal/scanner"
)

// ─── fakes ───────────────────────────────────────────────────────────────────

type fakeIndexer struct {
	mu      sync.Mutex
	indexed []string
}

func (f *fakeIndexer) IndexFile(_ context.Context, path, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.indexed = append(f.indexed, path)
	return nil
}

func (f *fakeIndexer) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.indexed))
	copy(cp, f.indexed)
	return cp
}

type fakeDeleter struct {
	mu      sync.Mutex
	deleted []string
}

func (f *fakeDeleter) DeleteByPath(_ context.Context, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, path)
	return nil
}

func (f *fakeDeleter) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.deleted))
	copy(cp, f.deleted)
	return cp
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// setupWatcher creates a temporary library root, starts a Watcher in the
// background and returns the root dir, fake stores and a cancel func.
func setupWatcher(t *testing.T, subdirs map[string]string) (root string, idx *fakeIndexer, del *fakeDeleter, cancel context.CancelFunc) {
	t.Helper()

	root = t.TempDir()
	for subdir := range subdirs {
		if err := os.MkdirAll(filepath.Join(root, subdir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	cfg := config.ScannerConfig{
		LibraryRoot:  root,
		Directories:  subdirs,
		Extensions:   []string{".mp3", ".ogg"},
		WatchEnabled: true,
	}

	idx = &fakeIndexer{}
	del = &fakeDeleter{}

	w := scanner.NewWatcher(cfg, idx, del, noopLogger())

	ctx, cancel2 := context.WithCancel(context.Background())
	ready := make(chan struct{})
	go func() {
		close(ready)
		_ = w.Run(ctx)
	}()
	<-ready

	// Small pause so fsnotify has time to add watches before we act on files.
	time.Sleep(50 * time.Millisecond)

	return root, idx, del, cancel2
}

// waitFor polls until cond() returns true or timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestWatcher_IndexOnCreate(t *testing.T) {
	root, idx, _, cancel := setupWatcher(t, map[string]string{"musicas": "MUSIC"})
	defer cancel()

	file := filepath.Join(root, "musicas", "song.mp3")
	if err := os.WriteFile(file, []byte("fake-audio"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Debounce is 500ms; allow up to 2s total.
	if !waitFor(t, 2*time.Second, func() bool {
		for _, p := range idx.paths() {
			if p == file {
				return true
			}
		}
		return false
	}) {
		t.Errorf("IndexFile not called for %q within timeout", file)
	}
}

func TestWatcher_DeleteOnRemove(t *testing.T) {
	root, _, del, cancel := setupWatcher(t, map[string]string{"musicas": "MUSIC"})
	defer cancel()

	file := filepath.Join(root, "musicas", "gone.mp3")
	// Create first so the watcher is aware of it, then delete.
	_ = os.WriteFile(file, []byte("fake"), 0o644)
	time.Sleep(20 * time.Millisecond)

	if err := os.Remove(file); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if !waitFor(t, 2*time.Second, func() bool {
		for _, p := range del.paths() {
			if p == file {
				return true
			}
		}
		return false
	}) {
		t.Errorf("DeleteByPath not called for %q within timeout", file)
	}
}

func TestWatcher_IgnoreUnsupportedExtension(t *testing.T) {
	root, idx, del, cancel := setupWatcher(t, map[string]string{"musicas": "MUSIC"})
	defer cancel()

	file := filepath.Join(root, "musicas", "readme.txt")
	_ = os.WriteFile(file, []byte("text"), 0o644)
	_ = os.Remove(file)

	// Wait a bit more than the debounce window.
	time.Sleep(700 * time.Millisecond)

	if len(idx.paths()) > 0 {
		t.Errorf("IndexFile should not be called for .txt, got %v", idx.paths())
	}
	if len(del.paths()) > 0 {
		t.Errorf("DeleteByPath should not be called for .txt, got %v", del.paths())
	}
}

func TestWatcher_DebounceBurstyWrites(t *testing.T) {
	root, idx, _, cancel := setupWatcher(t, map[string]string{"musicas": "MUSIC"})
	defer cancel()

	file := filepath.Join(root, "musicas", "bursting.mp3")

	// Simulate three rapid writes — only one IndexFile call expected.
	for i := 0; i < 3; i++ {
		_ = os.WriteFile(file, []byte("chunk"), 0o644)
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to fire.
	if !waitFor(t, 2*time.Second, func() bool {
		return len(idx.paths()) > 0
	}) {
		t.Fatal("IndexFile never called")
	}

	time.Sleep(200 * time.Millisecond) // allow any extra calls to arrive
	if n := len(idx.paths()); n != 1 {
		t.Errorf("expected 1 IndexFile call after burst, got %d", n)
	}
}

func TestWatcher_MultipleDirectories(t *testing.T) {
	dirs := map[string]string{
		"musicas":  "MUSIC",
		"vinhetas": "VINHETA",
	}
	root, idx, _, cancel := setupWatcher(t, dirs)
	defer cancel()

	musicFile  := filepath.Join(root, "musicas", "track.mp3")
	vinhetaFile := filepath.Join(root, "vinhetas", "aberta.mp3")

	_ = os.WriteFile(musicFile, []byte("m"), 0o644)
	_ = os.WriteFile(vinhetaFile, []byte("v"), 0o644)

	if !waitFor(t, 3*time.Second, func() bool {
		return len(idx.paths()) >= 2
	}) {
		t.Errorf("expected 2 indexed files, got %d: %v", len(idx.paths()), idx.paths())
	}
}
