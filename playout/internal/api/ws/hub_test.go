package ws_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"

	apiws "github.com/Waelson/radio-playout-engine/internal/api/ws"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// startTestServer creates a minimal HTTP test server with the /v1/events
// WebSocket endpoint, a running Hub, and returns the server URL and a cancel
// function to shut everything down.
func startTestServer(t *testing.T) (serverURL string, evtBus *events.Bus, cancel context.CancelFunc) {
	t.Helper()

	evtBus = events.NewBus(nil)
	stateMgr := state.NewManager("test")
	stateMgr.SetState(state.StateIdle)

	hub := apiws.NewHub(evtBus, stateMgr, nil)

	ctx, cancelFn := context.WithCancel(context.Background())
	go hub.Run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/events", func(w http.ResponseWriter, r *http.Request) {
		apiws.ServeWS(hub, w, r)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return wsURL, evtBus, cancelFn
}

// readEvent reads one JSON event from the WebSocket connection with a timeout.
func readEvent(t *testing.T, conn *websocket.Conn, timeout time.Duration) events.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("websocket read: %v", err)
	}

	var evt events.Event
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("unmarshal event: %v (raw: %s)", err, string(msg))
	}
	return evt
}

// TestHub_ClientReceivesSnapshot verifies that a newly connected client
// immediately receives a StateSnapshot event.
func TestHub_ClientReceivesSnapshot(t *testing.T) {
	url, _, cancel := startTestServer(t)
	defer cancel()

	conn, _, err := websocket.Dial(context.Background(), url+"/v1/events", nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	evt := readEvent(t, conn, 2*time.Second)
	if evt.Type != events.EvtStateSnapshot {
		t.Errorf("expected StateSnapshot as first event, got %s", evt.Type)
	}
}

// TestHub_ClientReceivesPublishedEvent verifies that events published to the
// Event Bus are delivered to connected clients.
func TestHub_ClientReceivesPublishedEvent(t *testing.T) {
	url, evtBus, cancel := startTestServer(t)
	defer cancel()

	conn, _, err := websocket.Dial(context.Background(), url+"/v1/events", nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Consume the snapshot.
	_ = readEvent(t, conn, 5*time.Second)

	// Publish a QueueChanged event.
	evtBus.Publish(events.New(events.EvtQueueChanged, events.QueueChangedPayload{
		Size:   1,
		Reason: "enqueue",
		Items: []events.QueueItemSummary{
			{QueueItemID: "qi_test", AssetID: "a1", Title: "Track", Type: "MUSIC", DurationMS: 180000},
		},
	}))

	// Client should receive it.
	evt := readEvent(t, conn, 5*time.Second)
	if evt.Type != events.EvtQueueChanged {
		t.Errorf("expected QueueChanged, got %s", evt.Type)
	}
}

// TestHub_MultipleClients verifies all connected clients receive the event.
func TestHub_MultipleClients(t *testing.T) {
	url, evtBus, cancel := startTestServer(t)
	defer cancel()

	const numClients = 3
	conns := make([]*websocket.Conn, numClients)
	for i := range conns {
		c, _, err := websocket.Dial(context.Background(), url+"/v1/events", nil)
		if err != nil {
			t.Fatalf("dial client %d: %v", i, err)
		}
		conns[i] = c
		defer c.Close(websocket.StatusNormalClosure, "")
		// Drain snapshot.
		_ = readEvent(t, c, 2*time.Second)
	}

	// Publish one event.
	evtBus.Publish(events.New(events.EvtPlayerStateChanged, events.PlayerStateChangedPayload{
		From: "IDLE", To: "PLAYING", Mode: "AUTO",
	}))

	// Each client should receive it.
	for i, c := range conns {
		evt := readEvent(t, c, 5*time.Second)
		if evt.Type != events.EvtPlayerStateChanged {
			t.Errorf("client %d: expected PlayerStateChanged, got %s", i, evt.Type)
		}
	}
}

// TestHub_QueueChangedAfterEnqueue exercises the full integration path:
// enqueue a command via queue.Manager → QueueChanged event → WebSocket client.
func TestHub_QueueChangedAfterEnqueue(t *testing.T) {
	url, evtBus, cancel := startTestServer(t)
	defer cancel()

	conn, _, err := websocket.Dial(context.Background(), url+"/v1/events", nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	stateMgr := state.NewManager("test2")
	stateMgr.SetState(state.StateIdle)
	queueMgr := queue.NewManager(evtBus, stateMgr, nil)

	// Drain snapshot.
	_ = readEvent(t, conn, 2*time.Second)

	// Enqueue an item via the queue manager (which publishes QueueChanged).
	queueMgr.Enqueue([]commands.QueueItemInput{
		{AssetID: "a1", Path: "/music/track.mp3", Type: "MUSIC", DurationMS: 200000},
	})

	evt := readEvent(t, conn, 5*time.Second)
	if evt.Type != events.EvtQueueChanged {
		t.Errorf("expected QueueChanged, got %s", evt.Type)
	}
}
