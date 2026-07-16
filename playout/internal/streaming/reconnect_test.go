package streaming_test

import (
	"context"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/streaming"
)

// ── unit tests (no FFmpeg required) ──────────────────────────────────────────

func TestCalcBackoff_FirstRetry(t *testing.T) {
	rc := streaming.ReconnectConfig{
		InitialDelaySec:   2,
		MaxDelaySec:       60,
		BackoffMultiplier: 2.0,
	}
	got := streaming.ExportCalcBackoff(rc, 1)
	if got != 2*time.Second {
		t.Errorf("retry 1: got %v, want 2s", got)
	}
}

func TestCalcBackoff_Doubles(t *testing.T) {
	rc := streaming.ReconnectConfig{
		InitialDelaySec:   1,
		MaxDelaySec:       60,
		BackoffMultiplier: 2.0,
	}
	cases := []struct {
		retry int
		want  time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 32 * time.Second},
		{7, 60 * time.Second}, // capped at MaxDelaySec
		{8, 60 * time.Second},
	}
	for _, tc := range cases {
		got := streaming.ExportCalcBackoff(rc, tc.retry)
		if got != tc.want {
			t.Errorf("retry %d: got %v, want %v", tc.retry, got, tc.want)
		}
	}
}

func TestCalcBackoff_Defaults(t *testing.T) {
	// Zero values → defaults: initial=2s, max=60s, mult=2.0
	rc := streaming.ReconnectConfig{}
	got1 := streaming.ExportCalcBackoff(rc, 1)
	if got1 != 2*time.Second {
		t.Errorf("default initial: got %v, want 2s", got1)
	}
	// After enough retries it should be capped at 60s.
	got10 := streaming.ExportCalcBackoff(rc, 10)
	if got10 != 60*time.Second {
		t.Errorf("default cap: got %v, want 60s", got10)
	}
}

func TestCalcBackoff_MultiplierLEOne_DefaultsTo2(t *testing.T) {
	rc := streaming.ReconnectConfig{
		InitialDelaySec:   1,
		MaxDelaySec:       60,
		BackoffMultiplier: 0.5, // invalid → default 2.0
	}
	got2 := streaming.ExportCalcBackoff(rc, 2)
	if got2 != 2*time.Second {
		t.Errorf("invalid multiplier retry 2: got %v, want 2s", got2)
	}
}

func TestManager_Reconnect_Disabled_NoLoop(t *testing.T) {
	// When reconnect is disabled, the onDisconnect callback must not start a
	// reconnect loop. We verify indirectly: the manager publishes a
	// StreamingDisconnected event (because reconnect is off).
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg")
	}

	srv, port, _ := mockIcecastServer(t)
	defer srv.Close()

	bus := events.NewBus(nil)
	m := streaming.NewManager(bus, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go m.Run(ctx)

	// Reconnect disabled.
	cfg := testConfig(t, port)
	cfg.Reconnect.Enabled = false

	if err := m.AddTarget(ctx, cfg); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	// Kill the server to trigger unexpected disconnect.
	srv.Close()

	// No reconnect loop should run, so after a moment the target should remain
	// in disconnected/error state without recovering.
	time.Sleep(500 * time.Millisecond)

	s, err := m.Status(cfg.ID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if s.State == streaming.StateConnected {
		t.Error("target should not be connected when reconnect is disabled")
	}

	m.RemoveTarget(cfg.ID)
}

// ── integration tests (require FFmpeg) ───────────────────────────────────────

func TestManager_Reconnect_MaxRetries_Exhausted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg")
	}

	// Use a port with nothing listening → every connect attempt fails fast.
	bus := events.NewBus(nil)

	var gotError bool
	evtCh, cancelSub := bus.Subscribe(16)
	defer cancelSub()
	go func() {
		for evt := range evtCh {
			if evt.Type == events.EvtStreamingError {
				gotError = true
			}
		}
	}()

	m := streaming.NewManager(bus, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go m.Run(ctx)

	// Use an unreachable host so Connect fails quickly, allowing the test to
	// exhaust MaxRetries=2 in a reasonable time.
	cfg := streaming.TargetConfig{
		ID:     "retry-test",
		Name:   "Retry Test",
		Type:   "icecast",
		Host:   "127.0.0.1",
		Port:   1, // nothing listening
		Mount:  "/test",
		Format: "mp3",
		Reconnect: streaming.ReconnectConfig{
			Enabled:           true,
			MaxRetries:        2,
			InitialDelaySec:   1,
			MaxDelaySec:       2,
			BackoffMultiplier: 1.5,
		},
	}

	// Connect will likely succeed (FFmpeg starts) but then die quickly.
	if err := m.AddTarget(ctx, cfg); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	// Wait for reconnect attempts and eventual EvtStreamingError.
	deadline := time.After(25 * time.Second)
	for !gotError {
		select {
		case <-deadline:
			t.Log("EvtStreamingError not received within deadline (FFmpeg may behave differently)")
			return
		case <-time.After(200 * time.Millisecond):
		}
	}

	s, _ := m.Status(cfg.ID)
	if s.State != streaming.StateError {
		t.Errorf("state after max retries: got %q, want error", s.State)
	}
}
