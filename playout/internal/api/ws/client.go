package ws

import (
	"context"
	"net/http"
	"time"

	"nhooyr.io/websocket"
)

// writeTimeout is the per-message write deadline for WebSocket clients.
const writeTimeout = 10 * time.Second

// pingInterval controls how often the server sends a ping frame to detect
// dead connections.
const pingInterval = 30 * time.Second

// Client represents a single connected WebSocket peer.
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte // buffered channel of outbound JSON messages
	remoteAddr string
}

// ServeWS upgrades the HTTP connection to WebSocket and manages the client
// lifecycle: snapshot-on-connect, read loop (ping/pong keepalive), write loop.
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Allow any origin in development; production should restrict this.
		InsecureSkipVerify: true,
	})
	if err != nil {
		hub.log.Error("websocket upgrade failed", "error", err)
		return
	}

	c := &Client{
		hub:        hub,
		conn:       conn,
		send:       make(chan []byte, clientBufSize),
		remoteAddr: r.RemoteAddr,
	}

	hub.register(c)
	defer hub.unregister(c)

	// Send state snapshot immediately so the client can render current state.
	if snap, err := hub.Snapshot(); err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), writeTimeout)
		_ = conn.Write(ctx, websocket.MessageText, snap)
		cancel()
	}

	// Run write loop in a goroutine; this function runs the read/keepalive loop.
	writeDone := make(chan struct{})
	go c.writeLoop(r.Context(), writeDone)

	c.readLoop(r.Context())

	// readLoop exited — close the connection and wait for writeLoop to finish.
	conn.Close(websocket.StatusNormalClosure, "")
	<-writeDone
}

// writeLoop drains c.send and writes messages to the WebSocket connection.
// It also sends periodic pings to keep the connection alive.
func (c *Client) writeLoop(ctx context.Context, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// Hub closed the channel — client was unregistered.
				return
			}
			wCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(wCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				c.hub.log.Debug("websocket write error", "remote", c.remoteAddr, "error", err)
				return
			}

		case <-ticker.C:
			pCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Ping(pCtx)
			cancel()
			if err != nil {
				c.hub.log.Debug("websocket ping failed", "remote", c.remoteAddr, "error", err)
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// readLoop reads and discards incoming frames from the client.
// WebSocket keepalive (ping/pong) is handled automatically by the library.
// The loop exits when the connection is closed or ctx is cancelled.
func (c *Client) readLoop(ctx context.Context) {
	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			// Any error here means the connection is gone.
			return
		}
		// Client messages are intentionally ignored; the Engine is write-only.
	}
}
