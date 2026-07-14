package cuein

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/Waelson/radio-library-service/internal/scanner"
	"github.com/Waelson/radio-library-service/internal/store"
)

const defaultQueueSize = 4096

// CueInStore is the subset of *store.TrackStore the Worker needs.
type CueInStore interface {
	FindByID(ctx context.Context, id string) (store.Track, error)
	SetCueIn(ctx context.Context, id string, ms int64) error
	CountCueInStatus(ctx context.Context) (map[string]int, error)
}

// Worker detects leading silence in tracks (cue_in_ms) using a bounded
// concurrency pool. Call Start in a goroutine; it blocks until ctx is
// cancelled. Enqueue is safe to call from any goroutine at any time.
type Worker struct {
	ffmpegPath  string
	store       CueInStore
	concurrency int
	queue       chan string
	log         *slog.Logger
	wg          sync.WaitGroup
	running     atomic.Bool
	analyzing   atomic.Int64
}

// NewWorker creates a Worker.
// concurrency sets the maximum number of parallel ffmpeg processes; 0 defaults to 2.
func NewWorker(ffmpegPath string, s CueInStore, concurrency int, log *slog.Logger) *Worker {
	if concurrency <= 0 {
		concurrency = 2
	}
	return &Worker{
		ffmpegPath:  ffmpegPath,
		store:       s,
		concurrency: concurrency,
		queue:       make(chan string, defaultQueueSize),
		log:         log,
	}
}

// Enqueue adds a track ID to the analysis queue. Non-blocking: if the queue
// is full the entry is dropped with a warning log.
func (w *Worker) Enqueue(id string) {
	select {
	case w.queue <- id:
	default:
		w.log.Warn("cuein queue full, dropping track", "id", id)
	}
}

// IsRunning reports whether the worker goroutine pool is active.
func (w *Worker) IsRunning() bool { return w.running.Load() }

// DrainQueue discards all IDs currently buffered in the queue channel.
// In-flight analyses already dispatched to goroutines are not cancelled.
func (w *Worker) DrainQueue() {
	for {
		select {
		case <-w.queue:
		default:
			return
		}
	}
}

// Status returns counts of tracks with and without cue_in_ms, plus the
// in-memory count of tracks currently being analyzed.
func (w *Worker) Status(ctx context.Context) (map[string]int, error) {
	counts, err := w.store.CountCueInStatus(ctx)
	if err != nil {
		return nil, err
	}
	counts["analyzing"] = int(w.analyzing.Load())
	counts["error"] = 0 // no persistent error state for cue_in
	return counts, nil
}

// Start launches the analysis pool. It blocks until ctx is cancelled, then
// waits for in-flight analyses to finish before returning.
func (w *Worker) Start(ctx context.Context) {
	w.running.Store(true)
	defer w.running.Store(false)

	sem := make(chan struct{}, w.concurrency)

	for {
		select {
		case <-ctx.Done():
			w.wg.Wait()
			return
		case id := <-w.queue:
			sem <- struct{}{} // acquire slot
			w.wg.Add(1)
			go func(trackID string) {
				defer func() {
					<-sem // release slot
					w.wg.Done()
				}()
				w.process(ctx, trackID)
			}(id)
		}
	}
}

// process detects cue_in for a single track and persists the result.
func (w *Worker) process(ctx context.Context, id string) {
	tr, err := w.store.FindByID(ctx, id)
	if err != nil {
		w.log.Warn("cuein: track not found, skipping", "id", id, "error", err)
		return
	}

	// Skip if already set (operator may have set it between enqueue and process).
	if tr.CueInMS != nil {
		return
	}

	w.analyzing.Add(1)
	defer w.analyzing.Add(-1)

	ms := scanner.DetectCueIn(w.ffmpegPath, tr.Path)
	if ms <= 0 {
		w.log.Debug("cuein: no leading silence detected", "path", tr.Path)
		return
	}

	if err := w.store.SetCueIn(ctx, id, ms); err != nil {
		w.log.Warn("cuein: persist failed", "id", id, "error", err)
		return
	}

	w.log.Debug("cuein: detected", "path", tr.Path, "cue_in_ms", ms)
}
