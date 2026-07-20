// Package indexsvc coordinates library scan state and exposes it via Status/TriggerScan.
package indexsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Waelson/radio-library-service/internal/scanner"
)

// ErrAlreadyRunning is returned when TriggerScan is called while a scan is
// already in progress.
var ErrAlreadyRunning = errors.New("scan already in progress")

// Scanner is satisfied by *scanner.Indexer.
type Scanner interface {
	Scan(ctx context.Context) (scanner.ScanResult, error)
	SyncCategories(ctx context.Context, lister scanner.TrackCategoryLister) (scanner.SyncCategoryResult, error)
}

// TrackCategoryLister is satisfied by *store.TrackStore.
type TrackCategoryLister interface {
	scanner.TrackCategoryLister
}

// TrackCounter returns the total number of indexed tracks.
type TrackCounter interface {
	Count(ctx context.Context) (int, error)
}

// Status represents the current state of the indexer.
type Status struct {
	Running     bool       `json:"running"`
	TotalTracks int        `json:"total_tracks"`
	LastRunAt   *time.Time `json:"last_run_at"`
	LastIndexed int        `json:"last_indexed"`
	LastSkipped int        `json:"last_skipped"`
	LastErrors  int        `json:"last_errors"`
}

// Service manages scan state and exposes it through Status and TriggerScan.
type Service struct {
	scanner Scanner
	counter TrackCounter
	lister  TrackCategoryLister
	log     *slog.Logger

	mu         sync.RWMutex
	running    bool
	lastRunAt  *time.Time
	lastResult *scanner.ScanResult
}

// New creates a Service.
func New(sc Scanner, counter TrackCounter, lister TrackCategoryLister, log *slog.Logger) *Service {
	return &Service{scanner: sc, counter: counter, lister: lister, log: log}
}

// Status returns current indexation state and the live track count.
func (s *Service) Status(ctx context.Context) (Status, error) {
	s.mu.RLock()
	running := s.running
	lastRunAt := s.lastRunAt
	var res scanner.ScanResult
	if s.lastResult != nil {
		res = *s.lastResult
	}
	s.mu.RUnlock()

	total, err := s.counter.Count(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("index status: count: %w", err)
	}

	return Status{
		Running:     running,
		TotalTracks: total,
		LastRunAt:   lastRunAt,
		LastIndexed: res.Indexed,
		LastSkipped: res.Skipped,
		LastErrors:  res.ErrCount,
	}, nil
}

// TriggerScan starts a scan in a background goroutine.
// Returns ErrAlreadyRunning if a scan is currently in progress.
func (s *Service) TriggerScan(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		// Use a detached context so the scan is not cancelled when the HTTP
		// request that triggered it completes.
		scanCtx := context.WithoutCancel(ctx)
		result, err := s.scanner.Scan(scanCtx)
		now := time.Now()

		s.mu.Lock()
		s.running = false
		s.lastRunAt = &now
		s.lastResult = &result
		s.mu.Unlock()

		if err != nil {
			s.log.Error("scan failed", "error", err)
		} else {
			s.log.Info("scan complete",
				"indexed", result.Indexed,
				"skipped", result.Skipped,
				"errors", result.ErrCount,
			)
		}
	}()
	return nil
}

// TriggerCategorySync starts a category sync in a background goroutine.
// Returns ErrAlreadyRunning if a scan is currently in progress.
func (s *Service) TriggerCategorySync(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		syncCtx := context.WithoutCancel(ctx)
		defer func() {
			now := time.Now()
			s.mu.Lock()
			s.running = false
			s.lastRunAt = &now
			s.mu.Unlock()
		}()
		result, err := s.scanner.SyncCategories(syncCtx, s.lister)
		if err != nil {
			s.log.Error("category sync failed", "error", err)
		} else {
			s.log.Info("category sync complete",
				"linked", result.Linked,
				"errors", result.Errors,
			)
		}
	}()
	return nil
}

// RunInitialScan triggers a background scan at startup. Errors are logged, not
// returned, because startup should continue regardless.
func (s *Service) RunInitialScan(ctx context.Context) {
	if err := s.TriggerScan(ctx); err != nil {
		s.log.Error("initial scan: could not start", "error", err)
	}
}
