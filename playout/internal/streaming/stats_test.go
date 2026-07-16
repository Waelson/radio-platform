package streaming_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/streaming"
)

// icecastStatsXML is a minimal /admin/stats XML response from a real Icecast server.
const icecastStatsXML = `<?xml version="1.0" encoding="UTF-8"?>
<icestats>
  <source mount="/stream">
    <listeners>42</listeners>
    <listener_peak>100</listener_peak>
  </source>
  <source mount="/backup">
    <listeners>5</listeners>
  </source>
</icestats>`

// newStatsServer starts a test HTTP server that serves mock Icecast XML.
func newStatsServer(t *testing.T, xmlBody string, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/stats" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(xmlBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// cfgForServer builds a TargetConfig pointing at the given test server.
func cfgForServer(t *testing.T, srv *httptest.Server, mount string) streaming.TargetConfig {
	t.Helper()
	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, _ := strconv.Atoi(portStr)
	return streaming.TargetConfig{
		ID:       "test",
		Host:     host,
		Port:     port,
		Password: "hackme",
		Mount:    mount,
	}
}

func TestFetchIcecastListeners_MatchingMount(t *testing.T) {
	srv := newStatsServer(t, icecastStatsXML, http.StatusOK)
	cfg := cfgForServer(t, srv, "/stream")

	listeners, err := streaming.FetchIcecastListeners(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listeners != 42 {
		t.Errorf("listeners = %d, want 42", listeners)
	}
}

func TestFetchIcecastListeners_SecondMount(t *testing.T) {
	srv := newStatsServer(t, icecastStatsXML, http.StatusOK)
	cfg := cfgForServer(t, srv, "/backup")

	listeners, err := streaming.FetchIcecastListeners(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listeners != 5 {
		t.Errorf("listeners = %d, want 5", listeners)
	}
}

func TestFetchIcecastListeners_MountNotFound(t *testing.T) {
	srv := newStatsServer(t, icecastStatsXML, http.StatusOK)
	cfg := cfgForServer(t, srv, "/nonexistent")

	listeners, err := streaming.FetchIcecastListeners(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Mount not found → 0 listeners, no error.
	if listeners != 0 {
		t.Errorf("listeners = %d, want 0 for unknown mount", listeners)
	}
}

func TestFetchIcecastListeners_DefaultMount(t *testing.T) {
	srv := newStatsServer(t, icecastStatsXML, http.StatusOK)
	// Empty mount → defaults to "/stream"
	cfg := cfgForServer(t, srv, "")

	listeners, err := streaming.FetchIcecastListeners(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listeners != 42 {
		t.Errorf("listeners = %d, want 42 for default mount", listeners)
	}
}

func TestFetchIcecastListeners_HTTPError(t *testing.T) {
	srv := newStatsServer(t, "", http.StatusUnauthorized)
	cfg := cfgForServer(t, srv, "/stream")

	_, err := streaming.FetchIcecastListeners(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
}

func TestFetchIcecastListeners_InvalidXML(t *testing.T) {
	srv := newStatsServer(t, "<not valid xml><<<", http.StatusOK)
	cfg := cfgForServer(t, srv, "/stream")

	_, err := streaming.FetchIcecastListeners(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for invalid XML, got nil")
	}
}

func TestFetchIcecastListeners_ShoutcastV1Skipped(t *testing.T) {
	// SHOUTcast v1 should return 0, nil without making any HTTP request.
	cfg := streaming.TargetConfig{
		ID:   "shout1",
		Type: "shoutcast_v1",
		Host: "127.0.0.1",
		Port: 1, // unreachable — proves no request is made
	}
	listeners, err := streaming.FetchIcecastListeners(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error for shoutcast_v1: %v", err)
	}
	if listeners != 0 {
		t.Errorf("listeners = %d, want 0 for shoutcast_v1", listeners)
	}
}

func TestFetchIcecastListeners_UnreachableServer(t *testing.T) {
	cfg := streaming.TargetConfig{
		ID:    "unreachable",
		Host:  "127.0.0.1",
		Port:  1, // nothing listening here
		Mount: "/stream",
	}
	_, err := streaming.FetchIcecastListeners(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}
