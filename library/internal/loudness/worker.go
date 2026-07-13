package loudness

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/Waelson/radio-library-service/internal/store"
)

const defaultQueueSize = 4096

// LoudnessAnalyzer is the interface satisfied by *Analyzer, injectable for tests.
type LoudnessAnalyzer interface {
	Analyze(ctx context.Context, filePath string) (Result, error)
}

// LoudnessStore is the subset of *store.TrackStore the Worker needs.
type LoudnessStore interface {
	FindByID(ctx context.Context, id string) (store.Track, error)
	UpdateLoudness(ctx context.Context, id string, lufs float64, truePeak float64) error
	UpdateLoudnessStatus(ctx context.Context, id string, status string, errMsg string) error
	CountByLoudnessStatus(ctx context.Context) (map[string]int, error)
}

// Worker analyzes track loudness in a bounded concurrency pool.
// Call Start in a goroutine; it blocks until ctx is cancelled.
// Enqueue is safe to call from any goroutine at any time.
type Worker struct {
	analyzer    LoudnessAnalyzer
	store       LoudnessStore
	concurrency int
	queue       chan string
	log         *slog.Logger
	wg          sync.WaitGroup
	running     atomic.Bool
}

// NewWorker creates a Worker.
// concurrency sets the maximum number of parallel ffmpeg processes; 0 defaults to 2.
func NewWorker(analyzer LoudnessAnalyzer, s LoudnessStore, concurrency int, log *slog.Logger) *Worker {
	if concurrency <= 0 {
		concurrency = 2
	}
	return &Worker{
		analyzer:    analyzer,
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
		w.log.Warn("loudness queue full, dropping track", "id", id)
	}
}

// IsRunning reports whether the worker goroutine pool is active.
func (w *Worker) IsRunning() bool { return w.running.Load() }

// Status returns per-status track counts from the database.
func (w *Worker) Status(ctx context.Context) (map[string]int, error) {
	return w.store.CountByLoudnessStatus(ctx)
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
			// Drain in-flight workers.
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

// process analyzes a single track and persists the result.
func (w *Worker) process(ctx context.Context, id string) {
	tr, err := w.store.FindByID(ctx, id)
	if err != nil {
		w.log.Warn("loudness: track not found, skipping", "id", id, "error", err)
		return
	}

	if err := w.store.UpdateLoudnessStatus(ctx, id, "analyzing", ""); err != nil {
		w.log.Warn("loudness: could not set analyzing status", "id", id, "error", err)
	}

	res, err := w.analyzer.Analyze(ctx, tr.Path)
	if err != nil {
		w.log.Warn("loudness: analysis failed", "path", tr.Path, "error", err)
		msg := err.Error()
		if len(msg) > 512 {
			msg = msg[:512]
		}
		_ = w.store.UpdateLoudnessStatus(ctx, id, "error", msg)
		return
	}

	if err := w.store.UpdateLoudness(ctx, id, res.LUFS, res.TruePeak); err != nil {
		w.log.Error("loudness: persist failed", "id", id, "error", err)
		return
	}

	w.log.Debug("loudness: analyzed",
		"path", tr.Path,
		"lufs", res.LUFS,
		"true_peak", res.TruePeak,
	)
}
