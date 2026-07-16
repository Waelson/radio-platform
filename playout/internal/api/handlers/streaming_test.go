package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/state"
	"github.com/Waelson/radio-playout-engine/internal/streaming"
)

// --- fake StreamingManager ---------------------------------------------------

type fakeStreamingMgr struct {
	targets map[string]streaming.TargetStatus
	addErr  error
}

func newFakeStreamingMgr() *fakeStreamingMgr {
	return &fakeStreamingMgr{targets: make(map[string]streaming.TargetStatus)}
}

func (f *fakeStreamingMgr) AddTarget(_ context.Context, cfg streaming.TargetConfig) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.targets[cfg.ID] = streaming.TargetStatus{
		ID:    cfg.ID,
		State: streaming.StateConnected,
	}
	return nil
}

func (f *fakeStreamingMgr) RemoveTarget(id string) {
	delete(f.targets, id)
}

func (f *fakeStreamingMgr) ListStatuses() []streaming.TargetStatus {
	out := make([]streaming.TargetStatus, 0, len(f.targets))
	for _, s := range f.targets {
		out = append(out, s)
	}
	return out
}

func (f *fakeStreamingMgr) Status(id string) (streaming.TargetStatus, error) {
	s, ok := f.targets[id]
	if !ok {
		return streaming.TargetStatus{}, fmt.Errorf("not found")
	}
	return s, nil
}

// --- helpers -----------------------------------------------------------------

func streamingGetRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// --- StreamingList -----------------------------------------------------------

func TestStreamingList_Empty(t *testing.T) {
	mgr := newFakeStreamingMgr()
	rr := streamingGetRequest(handlers.StreamingList(mgr), "/v1/streaming")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	targets, ok := body["targets"].([]any)
	if !ok {
		t.Fatal("targets field missing")
	}
	if len(targets) != 0 {
		t.Errorf("len(targets) = %d, want 0", len(targets))
	}
}

func TestStreamingList_WithTargets(t *testing.T) {
	mgr := newFakeStreamingMgr()
	now := time.Now()
	mgr.targets["t1"] = streaming.TargetStatus{ID: "t1", State: streaming.StateConnected, ConnectedAt: &now}
	mgr.targets["t2"] = streaming.TargetStatus{ID: "t2", State: streaming.StateReconnecting}

	rr := streamingGetRequest(handlers.StreamingList(mgr), "/v1/streaming")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	targets := body["targets"].([]any)
	if len(targets) != 2 {
		t.Errorf("len(targets) = %d, want 2", len(targets))
	}
}

// --- StreamingConnect --------------------------------------------------------

func TestStreamingConnect_Created(t *testing.T) {
	mgr := newFakeStreamingMgr()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/streaming/{id}/connect", handlers.StreamingConnect(mgr))

	body := map[string]any{
		"name": "Rádio Teste", "type": "icecast",
		"host": "127.0.0.1", "port": 8000,
		"password": "secret", "format": "mp3",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/streaming/icecast-1/connect", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["id"] != "icecast-1" {
		t.Errorf("id = %v, want icecast-1", resp["id"])
	}
	if resp["state"] != "connected" {
		t.Errorf("state = %v, want connected", resp["state"])
	}
}

func TestStreamingConnect_MissingHostPort(t *testing.T) {
	mgr := newFakeStreamingMgr()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/streaming/{id}/connect", handlers.StreamingConnect(mgr))

	body := map[string]any{"name": "bad"} // missing host/port
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/streaming/bad-target/connect", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestStreamingConnect_DuplicateID_Conflict(t *testing.T) {
	mgr := newFakeStreamingMgr()
	mgr.addErr = fmt.Errorf("streaming: target %q already exists", "dup")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/streaming/{id}/connect", handlers.StreamingConnect(mgr))

	body := map[string]any{"host": "127.0.0.1", "port": 8000}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/streaming/dup/connect", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

// --- StreamingDisconnect -----------------------------------------------------

func TestStreamingDisconnect_NoContent(t *testing.T) {
	mgr := newFakeStreamingMgr()
	mgr.targets["t1"] = streaming.TargetStatus{ID: "t1", State: streaming.StateConnected}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/streaming/{id}/disconnect", handlers.StreamingDisconnect(mgr))

	req := httptest.NewRequest(http.MethodPost, "/v1/streaming/t1/disconnect", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if _, ok := mgr.targets["t1"]; ok {
		t.Error("target should have been removed")
	}
}

func TestStreamingDisconnect_Idempotent(t *testing.T) {
	mgr := newFakeStreamingMgr()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/streaming/{id}/disconnect", handlers.StreamingDisconnect(mgr))

	req := httptest.NewRequest(http.MethodPost, "/v1/streaming/nonexistent/disconnect", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

// --- StreamingTest -----------------------------------------------------------

func TestStreamingTest_Reachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	body := map[string]any{"host": "127.0.0.1", "port": port}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/streaming/any/test", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handlers.StreamingTest().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Errorf("ok = %v, want true", resp["ok"])
	}
}

func TestStreamingTest_Unreachable(t *testing.T) {
	body := map[string]any{"host": "127.0.0.1", "port": 1}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/streaming/any/test", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handlers.StreamingTest().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["ok"] != false {
		t.Errorf("ok = %v, want false", resp["ok"])
	}
	if resp["error"] == nil || resp["error"] == "" {
		t.Error("error field should be non-empty for unreachable host")
	}
}

func TestStreamingTest_MissingBody(t *testing.T) {
	body := map[string]any{"host": ""} // no port
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/streaming/any/test", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handlers.StreamingTest().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// --- GET /v1/status streaming block ------------------------------------------

func TestStatus_StreamingBlock_Present(t *testing.T) {
	stateMgr := state.NewManager("test")
	streamMgr := newFakeStreamingMgr()
	now := time.Now()
	streamMgr.targets["t1"] = streaming.TargetStatus{
		ID:          "t1",
		State:       streaming.StateConnected,
		ConnectedAt: &now,
		BytesSent:   12345,
	}

	rr := streamingGetRequest(handlers.Status(stateMgr, streamMgr), "/v1/status")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)

	streamingBlock, ok := body["streaming"].([]any)
	if !ok {
		t.Fatal("streaming field missing")
	}
	if len(streamingBlock) != 1 {
		t.Fatalf("len(streaming) = %d, want 1", len(streamingBlock))
	}
	entry := streamingBlock[0].(map[string]any)
	if entry["id"] != "t1" {
		t.Errorf("id = %v, want t1", entry["id"])
	}
	if entry["state"] != "connected" {
		t.Errorf("state = %v, want connected", entry["state"])
	}
}

func TestStatus_StreamingBlock_EmptyWhenNoManager(t *testing.T) {
	stateMgr := state.NewManager("test")

	// No streaming manager — backward compat.
	rr := streamingGetRequest(handlers.Status(stateMgr), "/v1/status")
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)

	streamingBlock, ok := body["streaming"].([]any)
	if !ok {
		t.Fatal("streaming field missing")
	}
	if len(streamingBlock) != 0 {
		t.Errorf("len(streaming) = %d, want 0 when no manager", len(streamingBlock))
	}
}
