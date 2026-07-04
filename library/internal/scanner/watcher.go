package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Waelson/radio-library-service/internal/config"
	"github.com/fsnotify/fsnotify"
)

const debounceDuration = 500 * time.Millisecond

// FileIndexer indexes a single audio file into the track store.
type FileIndexer interface {
	IndexFile(ctx context.Context, path, assetType string) error
}

// TrackDeleter removes a track from the store by file path.
type TrackDeleter interface {
	DeleteByPath(ctx context.Context, path string) error
}

// Watcher monitors the configured library directories for file-system changes
// and keeps the track store in sync automatically.
//
// CREATE and WRITE events are debounced (500 ms) so that partial writes do not
// trigger indexing before the file is complete.
// REMOVE and RENAME events trigger an immediate DeleteByPath.
type Watcher struct {
	cfg      config.ScannerConfig
	indexer  FileIndexer
	deleter  TrackDeleter
	log      *slog.Logger
	mu       sync.Mutex
	timers   map[string]*time.Timer
}

// NewWatcher creates a Watcher.
func NewWatcher(cfg config.ScannerConfig, indexer FileIndexer, deleter TrackDeleter, log *slog.Logger) *Watcher {
	return &Watcher{
		cfg:     cfg,
		indexer: indexer,
		deleter: deleter,
		log:     log,
		timers:  make(map[string]*time.Timer),
	}
}

// Run starts fsnotify watchers on all configured directories and processes
// events until ctx is cancelled. It is safe to call in a goroutine.
func (w *Watcher) Run(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("watcher: create fsnotify: %w", err)
	}
	defer fsw.Close()

	for subdir := range w.cfg.Directories {
		dir := filepath.Join(w.cfg.LibraryRoot, subdir)
		if err := fsw.Add(dir); err != nil {
			w.log.Warn("watcher: cannot watch directory", "dir", dir, "error", err)
		} else {
			w.log.Info("watcher: watching", "dir", dir)
		}
	}

	for {
		select {
		case <-ctx.Done():
			w.cancelAll()
			return nil

		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			w.handleEvent(ctx, event)

		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			w.log.Error("watcher: fsnotify error", "error", err)
		}
	}
}

// handleEvent dispatches a single fsnotify event.
func (w *Watcher) handleEvent(ctx context.Context, ev fsnotify.Event) {
	path := ev.Name
	if !w.isSupportedExt(path) {
		return
	}

	switch {
	case ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write):
		w.debounce(ctx, path)

	case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
		// Cancel any pending index for this path (file is gone).
		w.cancelDebounce(path)
		if err := w.deleter.DeleteByPath(ctx, path); err != nil {
			w.log.Error("watcher: delete failed", "path", path, "error", err)
		} else {
			w.log.Info("watcher: removed", "path", path)
		}
	}
}

// debounce schedules or resets an IndexFile call for path after debounceDuration.
func (w *Watcher) debounce(ctx context.Context, path string) {
	assetType := w.assetTypeForPath(path)
	if assetType == "" {
		w.log.Warn("watcher: no asset type for path, skipping", "path", path)
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if t, ok := w.timers[path]; ok {
		t.Reset(debounceDuration)
		return
	}

	w.timers[path] = time.AfterFunc(debounceDuration, func() {
		w.mu.Lock()
		delete(w.timers, path)
		w.mu.Unlock()

		if err := w.indexer.IndexFile(ctx, path, assetType); err != nil {
			w.log.Error("watcher: index failed", "path", path, "error", err)
		} else {
			w.log.Info("watcher: indexed", "path", path)
		}
	})
}

// cancelDebounce cancels any pending debounce timer for path.
func (w *Watcher) cancelDebounce(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if t, ok := w.timers[path]; ok {
		t.Stop()
		delete(w.timers, path)
	}
}

// cancelAll stops all pending timers (called on shutdown).
func (w *Watcher) cancelAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for path, t := range w.timers {
		t.Stop()
		delete(w.timers, path)
	}
}

// assetTypeForPath returns the asset type associated with the directory that
// contains path, or "" if the path is not inside any configured directory.
func (w *Watcher) assetTypeForPath(path string) string {
	for subdir, assetType := range w.cfg.Directories {
		dir := filepath.Join(w.cfg.LibraryRoot, subdir)
		rel, err := filepath.Rel(dir, path)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return assetType
		}
	}
	return ""
}

// isSupportedExt returns true if the file extension is in the configured list.
func (w *Watcher) isSupportedExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range w.cfg.Extensions {
		if strings.ToLower(e) == ext {
			return true
		}
	}
	return false
}
