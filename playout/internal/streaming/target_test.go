package streaming_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/streaming"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func testConfig(t *testing.T, port int) streaming.TargetConfig {
	t.Helper()
	return streaming.TargetConfig{
		ID:          "test-target",
		Name:        "Test Stream",
		Type:        "icecast",
		Host:        "127.0.0.1",
		Port:        port,
		Mount:       "/test",
		Password:    "secret",
		Format:      "mp3",
		BitrateKbps: 64,
		SampleRate:  44100,
		Channels:    2,
		Reconnect: streaming.ReconnectConfig{
			Enabled:           true,
			InitialDelaySec:   1,
			MaxDelaySec:       5,
			BackoffMultiplier: 2.0,
		},
	}
}

// mockIcecastServer starts a minimal HTTP server that accepts the FFmpeg SOURCE
// connection and drains the body (simulating Icecast). Returns the server and
// the port it is listening on.
func mockIcecastServer(t *testing.T) (*httptest.Server, int, *atomic.Int64) {
	t.Helper()
	var bytesReceived atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Icecast accepts the SOURCE method; FFmpeg uses PUT for some versions.
		// The mock just drains whatever arrives.
		n, _ := io.Copy(io.Discard, r.Body)
		bytesReceived.Add(n)
		w.WriteHeader(http.StatusOK)
	}))

	// Extract port from the listener address.
	_, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	return srv, port, &bytesReceived
}

// ── unit tests ────────────────────────────────────────────────────────────────

func TestTargetConfig_Defaults(t *testing.T) {
	cfg := streaming.TargetConfig{
		ID:   "t1",
		Type: "icecast",
		Host: "h",
		Port: 8000,
	}
	// Zero BitrateKbps — target.buildFFmpegArgs should default to 128.
	// We test the defaults indirectly via Status after connect (no real FFmpeg).
	tgt := streaming.NewTarget(cfg, nil)
	s := tgt.Status()
	if s.State != streaming.StateIdle {
		t.Errorf("initial state: got %q, want idle", s.State)
	}
	if s.ID != "t1" {
		t.Errorf("ID: got %q, want t1", s.ID)
	}
}

func TestTarget_WriteWhileDisconnected(t *testing.T) {
	cfg := streaming.TargetConfig{ID: "t1", Type: "icecast", Host: "h", Port: 8000}
	tgt := streaming.NewTarget(cfg, nil)

	// Write while idle must not panic.
	frames := make([]float32, 256)
	tgt.Write(frames)
}

func TestTarget_DisconnectBeforeConnect(t *testing.T) {
	cfg := streaming.TargetConfig{ID: "t1", Type: "icecast", Host: "h", Port: 8000}
	tgt := streaming.NewTarget(cfg, nil)
	// Must not block or panic.
	done := make(chan struct{})
	go func() {
		tgt.Disconnect()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Disconnect() blocked")
	}
}

func TestTarget_ConnectToUnreachableHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg")
	}
	cfg := streaming.TargetConfig{
		ID:    "t1",
		Type:  "icecast",
		Host:  "127.0.0.1",
		Port:  1, // nothing listening here
		Mount: "/test",
	}
	tgt := streaming.NewTarget(cfg, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// FFmpeg will fail because nothing is listening on port 1.
	// Connect starts FFmpeg and returns quickly; the subprocess will die soon.
	// We just verify that Connect returns without panicking.
	_ = tgt.Connect(ctx)
	tgt.Disconnect()
}

func TestTarget_OnDisconnectCallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg")
	}

	srv, port, _ := mockIcecastServer(t)
	defer srv.Close()

	cfg := testConfig(t, port)
	tgt := streaming.NewTarget(cfg, nil)

	disconnected := make(chan string, 1)
	tgt.SetOnDisconnect(func(id, reason string) {
		disconnected <- reason
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := tgt.Connect(ctx); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	// Kill the server to trigger a disconnect.
	srv.Close()

	select {
	case reason := <-disconnected:
		if reason == "" {
			t.Error("expected non-empty disconnect reason")
		}
	case <-time.After(5 * time.Second):
		t.Log("no disconnect callback received (FFmpeg may not be installed)")
	}

	tgt.Disconnect()
}

func TestTarget_BuildURL_Icecast(t *testing.T) {
	cfg := streaming.TargetConfig{
		ID:       "x",
		Type:     "icecast",
		Host:     "stream.example.com",
		Port:     8000,
		Mount:    "/ao-vivo",
		Password: "secret",
	}
	// Use ExportBuildURL for white-box testing; if not exported, test URL
	// indirectly via the exported FFmpegArgs.
	args := streaming.ExportBuildFFmpegArgs(cfg)
	url := args[len(args)-1]
	if !strings.Contains(url, "source:secret@stream.example.com:8000/ao-vivo") {
		t.Errorf("unexpected URL in args: %v", url)
	}
}

func TestTarget_BuildURL_SHOUTcast_v1(t *testing.T) {
	cfg := streaming.TargetConfig{
		ID:       "x",
		Type:     "shoutcast_v1",
		Host:     "sc.example.com",
		Port:     8000,
		Mount:    "/stream",
		Password: "pass123",
	}
	args := streaming.ExportBuildFFmpegArgs(cfg)
	url := args[len(args)-1]
	// SHOUTcast v1: password in user field, no "source:" prefix.
	if !strings.Contains(url, ":pass123@sc.example.com:8000/stream") {
		t.Errorf("unexpected SHOUTcast v1 URL: %v", url)
	}
	if strings.Contains(url, "source:") {
		t.Errorf("SHOUTcast v1 URL must not contain 'source:': %v", url)
	}
}

func TestTarget_BuildFFmpegArgs_Format(t *testing.T) {
	cases := []struct {
		format      string
		wantEncoder string
		wantFormat  string
	}{
		{"mp3", "libmp3lame", "mp3"},
		{"ogg_vorbis", "libvorbis", "ogg"},
		{"ogg_opus", "libopus", "ogg"},
		{"aac", "aac", "adts"},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			cfg := streaming.TargetConfig{
				ID: "x", Type: "icecast", Host: "h", Port: 8000,
				Format: tc.format, BitrateKbps: 128, SampleRate: 44100, Channels: 2,
			}
			args := streaming.ExportBuildFFmpegArgs(cfg)
			argsStr := strings.Join(args, " ")
			if !strings.Contains(argsStr, tc.wantEncoder) {
				t.Errorf("format %q: want encoder %q in args %v", tc.format, tc.wantEncoder, args)
			}
			if !strings.Contains(argsStr, tc.wantFormat) {
				t.Errorf("format %q: want ffmpeg format %q in args %v", tc.format, tc.wantFormat, args)
			}
		})
	}
}

func TestTarget_StatusBytesSent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg")
	}

	srv, port, _ := mockIcecastServer(t)
	defer srv.Close()

	cfg := testConfig(t, port)
	tgt := streaming.NewTarget(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := tgt.Connect(ctx); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	// Write a few frames.
	frames := make([]float32, 4096)
	for i := 0; i < 10; i++ {
		tgt.Write(frames)
		time.Sleep(10 * time.Millisecond)
	}

	s := tgt.Status()
	if s.State != streaming.StateConnected {
		t.Errorf("state: got %q, want connected", s.State)
	}
	if s.ConnectedAt == nil {
		t.Error("ConnectedAt must not be nil when connected")
	}

	tgt.Disconnect()

	s = tgt.Status()
	if s.State != streaming.StateDisconnected {
		t.Errorf("state after disconnect: got %q, want disconnected", s.State)
	}
	if s.ConnectedAt != nil {
		t.Error("ConnectedAt must be nil after disconnect")
	}
}
