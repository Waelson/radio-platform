package indexsvc_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/indexsvc"
	"github.com/Waelson/radio-library-service/internal/scanner"
)

// ─── fakes ───────────────────────────────────────────────────────────────────

type mockScanner struct {
	mu     sync.Mutex
	result scanner.ScanResult
	err    error
	done   chan struct{} // closed when Scan is called
	block  chan struct{} // Scan blocks until this is closed (nil = no block)
}

func newMockScanner(result scanner.ScanResult, err error) *mockScanner {
	return &mockScanner{result: result, err: err, done: make(chan struct{})}
}

func (m *mockScanner) Scan(_ context.Context) (scanner.ScanResult, error) {
	if m.block != nil {
		<-m.block
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-m.done:
	default:
		close(m.done)
	}
	return m.result, m.err
}

func (m *mockScanner) waitDone(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-m.done:
	case <-time.After(timeout):
		t.Fatal("scan did not complete within timeout")
	}
}

type mockCounter struct {
	n   int
	err error
}

func (m *mockCounter) Count(_ context.Context) (int, error) { return m.n, m.err }

func noopLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// ─── tests ───────────────────────────────────────────────────────────────────

func TestStatus_InitialState(t *testing.T) {
	svc := indexsvc.New(newMockScanner(scanner.ScanResult{}, nil), &mockCounter{n: 42}, noopLog())
	st, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Running {
		t.Error("should not be running initially")
	}
	if st.TotalTracks != 42 {
		t.Errorf("TotalTracks = %d, want 42", st.TotalTracks)
	}
	if st.LastRunAt != nil {
		t.Error("LastRunAt should be nil before first scan")
	}
}

func TestStatus_CounterError(t *testing.T) {
	svc := indexsvc.New(
		newMockScanner(scanner.ScanResult{}, nil),
		&mockCounter{err: errors.New("db down")},
		noopLog(),
	)
	_, err := svc.Status(context.Background())
	if err == nil {
		t.Error("want error when counter fails")
	}
}

func TestTriggerScan_UpdatesStatusAfterCompletion(t *testing.T) {
	ms := newMockScanner(scanner.ScanResult{Indexed: 5, Skipped: 2, ErrCount: 1}, nil)
	svc := indexsvc.New(ms, &mockCounter{n: 5}, noopLog())

	if err := svc.TriggerScan(context.Background()); err != nil {
		t.Fatalf("TriggerScan: %v", err)
	}

	ms.waitDone(t, 3*time.Second)
	// Small pause to allow the goroutine to update state after Scan returns.
	time.Sleep(20 * time.Millisecond)

	st, _ := svc.Status(context.Background())
	if st.Running {
		t.Error("should not be running after scan completes")
	}
	if st.LastRunAt == nil {
		t.Error("LastRunAt should be set after scan")
	}
	if st.LastIndexed != 5 {
		t.Errorf("LastIndexed = %d, want 5", st.LastIndexed)
	}
	if st.LastSkipped != 2 {
		t.Errorf("LastSkipped = %d, want 2", st.LastSkipped)
	}
	if st.LastErrors != 1 {
		t.Errorf("LastErrors = %d, want 1", st.LastErrors)
	}
}

func TestTriggerScan_RunningFlagDuringExecution(t *testing.T) {
	block := make(chan struct{})
	ms := &mockScanner{
		result: scanner.ScanResult{Indexed: 1},
		done:   make(chan struct{}),
		block:  block,
	}
	svc := indexsvc.New(ms, &mockCounter{}, noopLog())

	if err := svc.TriggerScan(context.Background()); err != nil {
		t.Fatalf("TriggerScan: %v", err)
	}

	// Give goroutine time to start and enter Scan (blocked).
	time.Sleep(30 * time.Millisecond)

	st, _ := svc.Status(context.Background())
	if !st.Running {
		t.Error("should be running while scan is blocked")
	}

	close(block) // unblock scan
	ms.waitDone(t, 3*time.Second)
}

func TestTriggerScan_ErrAlreadyRunning(t *testing.T) {
	block := make(chan struct{})
	ms := &mockScanner{result: scanner.ScanResult{}, done: make(chan struct{}), block: block}
	svc := indexsvc.New(ms, &mockCounter{}, noopLog())

	_ = svc.TriggerScan(context.Background())
	time.Sleep(20 * time.Millisecond)

	// Second call must be rejected.
	err := svc.TriggerScan(context.Background())
	if !errors.Is(err, indexsvc.ErrAlreadyRunning) {
		t.Errorf("want ErrAlreadyRunning, got %v", err)
	}

	close(block)
}

func TestRunInitialScan_DoesNotPanic(t *testing.T) {
	ms := newMockScanner(scanner.ScanResult{Indexed: 3}, nil)
	svc := indexsvc.New(ms, &mockCounter{n: 3}, noopLog())
	svc.RunInitialScan(context.Background())
	ms.waitDone(t, 3*time.Second)
}
