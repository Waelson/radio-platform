package loudness_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/loudness"
	"github.com/Waelson/radio-library-service/internal/store"
)

// ─── mocks ───────────────────────────────────────────────────────────────────

type mockAnalyzer struct {
	result loudness.Result
	err    error
}

func (m *mockAnalyzer) Analyze(_ context.Context, _ string) (loudness.Result, error) {
	return m.result, m.err
}

type mockStore struct {
	tracks   map[string]store.Track
	statuses map[string]string
	errors   map[string]string
	lufs     map[string]float64
	peak     map[string]float64
	counts   map[string]int
}

func newMockStore() *mockStore {
	return &mockStore{
		tracks:   make(map[string]store.Track),
		statuses: make(map[string]string),
		errors:   make(map[string]string),
		lufs:     make(map[string]float64),
		peak:     make(map[string]float64),
		counts:   make(map[string]int),
	}
}

func (m *mockStore) FindByID(_ context.Context, id string) (store.Track, error) {
	t, ok := m.tracks[id]
	if !ok {
		return store.Track{}, store.ErrNotFound
	}
	return t, nil
}

func (m *mockStore) UpdateLoudness(_ context.Context, id string, lufs, peak float64) error {
	m.lufs[id] = lufs
	m.peak[id] = peak
	m.statuses[id] = "done"
	return nil
}

func (m *mockStore) UpdateLoudnessStatus(_ context.Context, id, status, errMsg string) error {
	m.statuses[id] = status
	m.errors[id] = errMsg
	return nil
}

func (m *mockStore) CountByLoudnessStatus(_ context.Context) (map[string]int, error) {
	return m.counts, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func runWorkerAndProcess(t *testing.T, ms *mockStore, ma *mockAnalyzer, ids ...string) {
	t.Helper()
	w := loudness.NewWorker(ma, ms, 1, nopLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for _, id := range ids {
		w.Enqueue(id)
	}

	// Run worker until all items are processed (signalled by context timeout
	// being longer than needed, or done when queue drains).
	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	// Give the worker time to process then cancel.
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestWorker_Success(t *testing.T) {
	ms := newMockStore()
	ms.tracks["t1"] = store.Track{ID: "t1", Path: "/music/song.mp3"}

	ma := &mockAnalyzer{result: loudness.Result{LUFS: -14.2, TruePeak: -0.5}}

	runWorkerAndProcess(t, ms, ma, "t1")

	if ms.statuses["t1"] != "done" {
		t.Errorf("status = %q, want done", ms.statuses["t1"])
	}
	if ms.lufs["t1"] != -14.2 {
		t.Errorf("LUFS = %v, want -14.2", ms.lufs["t1"])
	}
	if ms.peak["t1"] != -0.5 {
		t.Errorf("TruePeak = %v, want -0.5", ms.peak["t1"])
	}
}

func TestWorker_AnalysisError(t *testing.T) {
	ms := newMockStore()
	ms.tracks["t2"] = store.Track{ID: "t2", Path: "/music/broken.mp3"}

	ma := &mockAnalyzer{err: errors.New("ffmpeg exited with code 1")}

	runWorkerAndProcess(t, ms, ma, "t2")

	if ms.statuses["t2"] != "error" {
		t.Errorf("status = %q, want error", ms.statuses["t2"])
	}
	if ms.errors["t2"] == "" {
		t.Error("error message should be set")
	}
}

func TestWorker_TrackNotFound(t *testing.T) {
	ms := newMockStore() // empty — no tracks
	ma := &mockAnalyzer{result: loudness.Result{LUFS: -16.0}}

	// Should not panic or set any status — just skip.
	runWorkerAndProcess(t, ms, ma, "ghost-id")

	if _, ok := ms.statuses["ghost-id"]; ok {
		t.Error("no status should be written for a track that doesn't exist")
	}
}

func TestWorker_MultipleTracksSequential(t *testing.T) {
	ms := newMockStore()
	ids := []string{"a", "b", "c"}
	for _, id := range ids {
		ms.tracks[id] = store.Track{ID: id, Path: "/music/" + id + ".mp3"}
	}
	ma := &mockAnalyzer{result: loudness.Result{LUFS: -16.0, TruePeak: -1.0}}

	runWorkerAndProcess(t, ms, ma, ids...)

	for _, id := range ids {
		if ms.statuses[id] != "done" {
			t.Errorf("[%s] status = %q, want done", id, ms.statuses[id])
		}
	}
}

func TestWorker_IsRunning(t *testing.T) {
	ms := newMockStore()
	ma := &mockAnalyzer{}
	w := loudness.NewWorker(ma, ms, 1, nopLogger())

	if w.IsRunning() {
		t.Error("IsRunning should be false before Start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	go func() {
		close(started)
		w.Start(ctx)
	}()
	<-started
	time.Sleep(50 * time.Millisecond)

	if !w.IsRunning() {
		t.Error("IsRunning should be true after Start")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
	if w.IsRunning() {
		t.Error("IsRunning should be false after cancel")
	}
}

func TestWorker_EnqueueFull_DoesNotBlock(t *testing.T) {
	ms := newMockStore()
	// Slow analyzer so the queue fills up.
	ma := &mockAnalyzer{result: loudness.Result{LUFS: -16.0}}
	w := loudness.NewWorker(ma, ms, 1, nopLogger())

	// Enqueue more items than the buffer without starting the worker.
	// Should not deadlock.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			w.Enqueue("id")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Enqueue blocked — queue should be non-blocking")
	}
}
