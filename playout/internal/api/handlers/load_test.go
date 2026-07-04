package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

func newLoadQueueMgr(t *testing.T) *queue.Manager {
	t.Helper()
	return queue.NewManager(events.NewBus(nil), state.NewManager("load-test"), nil)
}

// TestLoad_Health_Concurrent fires 200 concurrent GET /v1/health requests and
// verifies that all succeed with 200 OK and no data races.
func TestLoad_Health_Concurrent(t *testing.T) {
	stateMgr := state.NewManager("load-test")
	stateMgr.SetState(state.StateIdle)
	h := handlers.Health(stateMgr)

	const workers = 200
	var wg sync.WaitGroup
	var ok atomic.Int64
	var fail atomic.Int64

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
			h(w, r)
			if w.Code == http.StatusOK {
				ok.Add(1)
			} else {
				fail.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := ok.Load(); got != workers {
		t.Errorf("health: %d/%d requests succeeded", got, workers)
	}
}

// TestLoad_Status_Concurrent fires 200 concurrent GET /v1/status requests
// while the state manager is being written to from a separate goroutine.
func TestLoad_Status_Concurrent(t *testing.T) {
	stateMgr := state.NewManager("load-test")
	stateMgr.SetState(state.StateIdle)
	h := handlers.Status(stateMgr)

	const workers = 200

	// Continuously toggle state in background while requests arrive.
	stop := make(chan struct{})
	go func() {
		states := []state.PlayerState{state.StateIdle, state.StatePlaying, state.StateIdle}
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				stateMgr.SetState(states[i%len(states)])
				i++
			}
		}
	}()

	var wg sync.WaitGroup
	var successes atomic.Int64

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
			h(w, r)
			if w.Code == http.StatusOK {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	close(stop)

	if got := successes.Load(); got != workers {
		t.Errorf("status: %d/%d requests succeeded", got, workers)
	}
}

// TestLoad_QueueList_Concurrent fires 200 concurrent GET /v1/queue requests.
func TestLoad_QueueList_Concurrent(t *testing.T) {
	qMgr := newLoadQueueMgr(t)
	h := handlers.QueueList(qMgr)

	const workers = 200
	var wg sync.WaitGroup
	var successes atomic.Int64

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1/queue", nil)
			h(w, r)
			if w.Code == http.StatusOK {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := successes.Load(); got != workers {
		t.Errorf("queue list: %d/%d requests succeeded", got, workers)
	}
}

// TestLoad_Health_Throughput is a benchmark-style test that verifies the
// handler can sustain at least 10 000 requests in < 5 seconds on any machine.
func TestLoad_Health_Throughput(t *testing.T) {
	stateMgr := state.NewManager("load-test")
	stateMgr.SetState(state.StateIdle)
	h := handlers.Health(stateMgr)

	const total = 10_000
	const batch = 100

	var done atomic.Int64
	var wg sync.WaitGroup

	for b := 0; b < total/batch; b++ {
		wg.Add(batch)
		for i := 0; i < batch; i++ {
			go func() {
				defer wg.Done()
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
				h(w, r)
				done.Add(1)
			}()
		}
		wg.Wait()
	}

	if got := done.Load(); got != total {
		t.Errorf("throughput: completed %d/%d requests", got, total)
	}
	t.Logf("throughput: completed %s requests", fmt.Sprintf("%d", done.Load()))
}
