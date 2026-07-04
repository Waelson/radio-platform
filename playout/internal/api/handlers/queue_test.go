package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// Ensure the real queue.Manager implements the queueReader interface used by handlers.
// This is a compile-time check only.
var _ interface {
	Current() *queue.QueueItem
	ListPending() []*queue.QueueItem
	Size() int
} = (*queue.Manager)(nil)

// --- Fake command bus that auto-accepts all commands --------------------------

type fakeAcceptBus struct {
	lastCmd *commands.Command
}

func (f *fakeAcceptBus) Send(_ context.Context, cmd commands.Command) error {
	f.lastCmd = &cmd
	// Auto-accept: fill the reply channel if present.
	if cmd.Reply != nil {
		go func() {
			time.Sleep(5 * time.Millisecond) // simulate dispatcher latency
			cmd.Reply <- commands.Result{CommandID: cmd.ID, Accepted: true}
		}()
	}
	return nil
}

// fakeRejectBus auto-rejects all commands.
type fakeRejectBus struct{}

func (f *fakeRejectBus) Send(_ context.Context, cmd commands.Command) error {
	if cmd.Reply != nil {
		go func() {
			time.Sleep(5 * time.Millisecond)
			cmd.Reply <- commands.Result{
				CommandID: cmd.ID,
				Accepted:  false,
				Reason:    "command rejected by engine",
			}
		}()
	}
	return nil
}

// --- Real queue manager as read source ----------------------------------------

func newRealQueueMgr(t *testing.T) *queue.Manager {
	t.Helper()
	return queue.NewManager(events.NewBus(nil), state.NewManager("test"), nil)
}

// --- Helper ------------------------------------------------------------------

func postJSON(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// --- GET /v1/queue -----------------------------------------------------------

func TestQueueList_EmptyQueue(t *testing.T) {
	qMgr := newRealQueueMgr(t)
	h := handlers.QueueList(qMgr)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/queue", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	pending, _ := resp["pending"].([]any)
	if len(pending) != 0 {
		t.Errorf("pending len = %d, want 0", len(pending))
	}
	if resp["current"] != nil {
		t.Errorf("current = %v, want nil", resp["current"])
	}
}

func TestQueueList_WithItems(t *testing.T) {
	qMgr := newRealQueueMgr(t)
	qMgr.Enqueue([]commands.QueueItemInput{
		{Path: "/a.mp3", Title: "Song A", Type: "musicas"},
		{Path: "/b.mp3", Title: "Song B", Type: "musicas"},
	})
	h := handlers.QueueList(qMgr)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/queue", nil)
	h.ServeHTTP(rr, req)

	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	pending := resp["pending"].([]any)
	if len(pending) != 2 {
		t.Fatalf("pending len = %d, want 2", len(pending))
	}
	first := pending[0].(map[string]any)
	if first["kind"] != "item" {
		t.Errorf("first entry kind = %v, want item", first["kind"])
	}
	item := first["item"].(map[string]any)
	if item["title"] != "Song A" {
		t.Errorf("first item title = %v", item["title"])
	}
	if item["status"] != "QUEUED" {
		t.Errorf("first item status = %v", item["status"])
	}
	if resp["total"] != float64(2) {
		t.Errorf("total = %v, want 2", resp["total"])
	}
}

func TestQueueList_WithBreak(t *testing.T) {
	qMgr := newRealQueueMgr(t)
	// Enqueue a music item then a break via HandleEnqueueBreak directly.
	qMgr.Enqueue([]commands.QueueItemInput{
		{Path: "/music.mp3", Title: "Song A", Type: "musicas", DurationMS: 180000},
	})
	_ = qMgr.HandleEnqueueBreak(context.Background(), commands.New(
		commands.CmdEnqueueBreak,
		commands.EnqueueBreakPayload{
			BreakID: "brk_TEST01",
			Break: commands.BreakItemInput{
				Title: "Bloco Teste",
				Spots: []commands.QueueItemInput{
					{Path: "/spot1.mp3", Title: "Spot 1", Type: "spots", DurationMS: 30000},
					{Path: "/spot2.mp3", Title: "Spot 2", Type: "spots", DurationMS: 30000},
				},
			},
		},
	))

	h := handlers.QueueList(qMgr)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/queue", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	pending := resp["pending"].([]any)
	if len(pending) != 2 {
		t.Fatalf("pending len = %d, want 2 (1 item + 1 break)", len(pending))
	}

	// First entry: plain music item.
	first := pending[0].(map[string]any)
	if first["kind"] != "item" {
		t.Errorf("pending[0].kind = %v, want item", first["kind"])
	}

	// Second entry: break block.
	second := pending[1].(map[string]any)
	if second["kind"] != "break" {
		t.Fatalf("pending[1].kind = %v, want break", second["kind"])
	}
	bk := second["break"].(map[string]any)
	if bk["break_id"] != "brk_TEST01" {
		t.Errorf("break_id = %v, want brk_TEST01", bk["break_id"])
	}
	if bk["title"] != "Bloco Teste" {
		t.Errorf("break title = %v", bk["title"])
	}
	bkItems := bk["items"].([]any)
	if len(bkItems) != 2 {
		t.Errorf("break items len = %d, want 2", len(bkItems))
	}

	if resp["break_count"] != float64(1) {
		t.Errorf("break_count = %v, want 1", resp["break_count"])
	}
	if resp["total"] != float64(3) { // 1 music + 2 spots
		t.Errorf("total = %v, want 3", resp["total"])
	}
}

// --- POST /v1/queue/enqueue --------------------------------------------------

func TestEnqueue_ValidItems_Accepted(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)

	// Simulate the real-world flow: the bus handler also does the actual enqueue.
	// For the handler test, we just verify it returns accepted=true.
	h := handlers.Enqueue(bus, qMgr)

	body := map[string]any{
		"items": []map[string]any{
			{"path": "/music/a.mp3", "type": "musicas", "title": "Song A", "duration_ms": 180000},
		},
	}
	rr := postJSON(t, h, "/v1/queue/enqueue", body)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Error("ok should be true")
	}
	if resp["accepted"] != true {
		t.Errorf("accepted = %v, want true", resp["accepted"])
	}
	if resp["command_id"] == "" {
		t.Error("command_id should be set")
	}
	if resp["queue_size"] == nil {
		t.Error("queue_size should be present")
	}
}

func TestEnqueue_EmptyItems_400(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.Enqueue(bus, qMgr)

	rr := postJSON(t, h, "/v1/queue/enqueue", map[string]any{"items": []any{}})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEnqueue_MissingPath_400(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.Enqueue(bus, qMgr)

	rr := postJSON(t, h, "/v1/queue/enqueue", map[string]any{
		"items": []map[string]any{{"title": "No path", "type": "musicas"}},
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEnqueue_InvalidJSON_400(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.Enqueue(bus, qMgr)

	req := httptest.NewRequest(http.MethodPost, "/v1/queue/enqueue", bytes.NewBufferString("not-json"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEnqueue_Rejected_ByEngine(t *testing.T) {
	bus := &fakeRejectBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.Enqueue(bus, qMgr)

	body := map[string]any{
		"items": []map[string]any{{"path": "/a.mp3", "type": "musicas"}},
	}
	rr := postJSON(t, h, "/v1/queue/enqueue", body)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["accepted"] != false {
		t.Errorf("accepted = %v, want false", resp["accepted"])
	}
	if resp["reason"] == "" {
		t.Error("reason should be non-empty when rejected")
	}
}

// --- POST /v1/queue/insert-next ----------------------------------------------

func TestInsertNext_Valid(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.InsertNext(bus, qMgr)

	body := map[string]any{
		"item": map[string]any{"path": "/x.mp3", "type": "jingles"},
	}
	rr := postJSON(t, h, "/v1/queue/insert-next", body)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["accepted"] != true {
		t.Errorf("accepted = %v", resp["accepted"])
	}
}

func TestInsertNext_MissingPath_400(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.InsertNext(bus, qMgr)

	rr := postJSON(t, h, "/v1/queue/insert-next", map[string]any{
		"item": map[string]any{"type": "jingles"},
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// --- POST /v1/queue/insert-after ---------------------------------------------

func TestInsertAfter_Valid(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.InsertAfter(bus, qMgr)

	body := map[string]any{
		"after_queue_item_id": "qi_001",
		"item":                map[string]any{"path": "/x.mp3", "type": "musicas"},
	}
	rr := postJSON(t, h, "/v1/queue/insert-after", body)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
}

func TestInsertAfter_MissingAfterID_400(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.InsertAfter(bus, qMgr)

	body := map[string]any{
		"item": map[string]any{"path": "/x.mp3", "type": "musicas"},
	}
	rr := postJSON(t, h, "/v1/queue/insert-after", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// --- POST /v1/queue/clear ----------------------------------------------------

func TestClearQueue_Valid(t *testing.T) {
	bus := &fakeAcceptBus{}
	h := handlers.ClearQueue(bus)

	body := map[string]any{"preserve_current": true}
	rr := postJSON(t, h, "/v1/queue/clear", body)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["accepted"] != true {
		t.Errorf("accepted = %v", resp["accepted"])
	}
}

// --- POST /v1/queue/enqueue-break --------------------------------------------

func TestEnqueueBreak_Valid_Accepted(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.EnqueueBreak(bus, qMgr)

	body := map[string]any{
		"title": "Bloco 14h",
		"spots": []map[string]any{
			{"path": "/spot1.mp3", "type": "spots", "title": "Spot 1", "duration_ms": 30000},
			{"path": "/spot2.mp3", "type": "spots", "title": "Spot 2", "duration_ms": 30000},
		},
	}
	rr := postJSON(t, h, "/v1/queue/enqueue-break", body)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Error("ok should be true")
	}
	if resp["accepted"] != true {
		t.Errorf("accepted = %v, want true", resp["accepted"])
	}
	breakID, _ := resp["break_id"].(string)
	if breakID == "" {
		t.Error("break_id should be non-empty")
	}
	if resp["total_items"] != float64(2) {
		t.Errorf("total_items = %v, want 2", resp["total_items"])
	}
	if resp["total_duration_ms"] != float64(60000) {
		t.Errorf("total_duration_ms = %v, want 60000", resp["total_duration_ms"])
	}
	if resp["command_id"] == "" {
		t.Error("command_id should be set")
	}
}

func TestEnqueueBreak_WithOpenAndClose(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.EnqueueBreak(bus, qMgr)

	body := map[string]any{
		"title": "Bloco Completo",
		"open":  map[string]any{"path": "/open.mp3", "type": "jingles", "duration_ms": 8000},
		"spots": []map[string]any{
			{"path": "/spot1.mp3", "type": "spots", "duration_ms": 30000},
		},
		"close": map[string]any{"path": "/close.mp3", "type": "jingles", "duration_ms": 7000},
	}
	rr := postJSON(t, h, "/v1/queue/enqueue-break", body)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["total_items"] != float64(3) {
		t.Errorf("total_items = %v, want 3", resp["total_items"])
	}
	if resp["total_duration_ms"] != float64(45000) {
		t.Errorf("total_duration_ms = %v, want 45000", resp["total_duration_ms"])
	}
}

func TestEnqueueBreak_NoSpots_400(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.EnqueueBreak(bus, qMgr)

	rr := postJSON(t, h, "/v1/queue/enqueue-break", map[string]any{
		"title": "Vazio",
		"spots": []any{},
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEnqueueBreak_NoTitle_400(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.EnqueueBreak(bus, qMgr)

	rr := postJSON(t, h, "/v1/queue/enqueue-break", map[string]any{
		"spots": []map[string]any{{"path": "/s.mp3", "type": "spots"}},
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEnqueueBreak_MissingSpotPath_400(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.EnqueueBreak(bus, qMgr)

	rr := postJSON(t, h, "/v1/queue/enqueue-break", map[string]any{
		"title": "Break",
		"spots": []map[string]any{{"title": "No path", "type": "spots"}},
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEnqueueBreak_InvalidJSON_400(t *testing.T) {
	bus := &fakeAcceptBus{}
	qMgr := newRealQueueMgr(t)
	h := handlers.EnqueueBreak(bus, qMgr)

	req := httptest.NewRequest(http.MethodPost, "/v1/queue/enqueue-break", bytes.NewBufferString("not-json"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// --- POST /v1/queue/clear ----------------------------------------------------

func TestClearQueue_EmptyBody_DefaultsToPreserveCurrent(t *testing.T) {
	bus := &fakeAcceptBus{}
	h := handlers.ClearQueue(bus)

	req := httptest.NewRequest(http.MethodPost, "/v1/queue/clear", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	// Verify the command was sent.
	if bus.lastCmd == nil {
		t.Fatal("no command was sent")
	}
	p := bus.lastCmd.Payload.(commands.ClearQueuePayload)
	if !p.PreserveCurrent {
		t.Error("preserve_current should default to true")
	}
}
