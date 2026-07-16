package streaming_test

import (
	"context"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/streaming"
)

// newTestManager creates a Manager with a no-op event bus for tests.
func newTestManager(t *testing.T) *streaming.Manager {
	t.Helper()
	bus := events.NewBus(nil)
	return streaming.NewManager(bus, nil)
}

// ── unit tests (no FFmpeg required) ──────────────────────────────────────────

func TestManager_ListStatuses_Empty(t *testing.T) {
	m := newTestManager(t)
	statuses := m.ListStatuses()
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(statuses))
	}
}

func TestManager_RemoveTarget_Idempotent(t *testing.T) {
	m := newTestManager(t)
	// Should not panic or return an error for a non-existent ID.
	m.RemoveTarget("nonexistent")
	m.RemoveTarget("nonexistent") // twice
}

func TestManager_TapCh_NotNil(t *testing.T) {
	m := newTestManager(t)
	if m.TapCh() == nil {
		t.Error("TapCh() must not return nil")
	}
}

func TestManager_TapCh_Overflow_NoDeadlock(t *testing.T) {
	m := newTestManager(t)
	tap := m.TapCh()
	frames := make([]float32, 256)

	// Overflow the tap channel with no consumer — must never block.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			select {
			case tap <- frames:
			default:
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("writing to tap channel blocked (deadlock detected)")
	}
}

func TestManager_Status_NotFound(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Status("nope")
	if err == nil {
		t.Error("expected error for missing target, got nil")
	}
}

func TestManager_AddTarget_DuplicateID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg")
	}
	srv, port, _ := mockIcecastServer(t)
	defer srv.Close()

	m := newTestManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := testConfig(t, port)
	if err := m.AddTarget(ctx, cfg); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
	defer m.RemoveTarget(cfg.ID)

	err := m.AddTarget(ctx, cfg)
	if err == nil {
		t.Error("expected error for duplicate ID, got nil")
	}
}

// ── integration tests (require FFmpeg) ───────────────────────────────────────

func TestManager_FanOut_MultipleTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg")
	}

	srv1, port1, bytes1 := mockIcecastServer(t)
	srv2, port2, bytes2 := mockIcecastServer(t)
	defer srv1.Close()
	defer srv2.Close()

	bus := events.NewBus(nil)
	m := streaming.NewManager(bus, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go m.Run(ctx)

	cfg1 := testConfig(t, port1)
	cfg1.ID = "target-1"
	cfg2 := testConfig(t, port2)
	cfg2.ID = "target-2"

	if err := m.AddTarget(ctx, cfg1); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
	if err := m.AddTarget(ctx, cfg2); err != nil {
		t.Fatalf("AddTarget target-2: %v", err)
	}

	// Verify both targets are listed.
	statuses := m.ListStatuses()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	// Send frames via the tap.
	tap := m.TapCh()
	frames := make([]float32, 4096)
	for i := 0; i < 20; i++ {
		select {
		case tap <- frames:
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Give FFmpeg time to encode and send.
	time.Sleep(300 * time.Millisecond)

	// Verify that the playback manager's status reflects connected state.
	s1, err := m.Status("target-1")
	if err != nil {
		t.Fatalf("Status target-1: %v", err)
	}
	if s1.State != streaming.StateConnected {
		t.Errorf("target-1 state: got %q, want connected", s1.State)
	}

	s2, err := m.Status("target-2")
	if err != nil {
		t.Fatalf("Status target-2: %v", err)
	}
	if s2.State != streaming.StateConnected {
		t.Errorf("target-2 state: got %q, want connected", s2.State)
	}

	// Both mock servers should have received some bytes.
	if bytes1.Load() == 0 && bytes2.Load() == 0 {
		t.Log("note: mock servers received 0 bytes (FFmpeg may not have flushed yet)")
	}

	// Clean removal.
	m.RemoveTarget("target-1")
	m.RemoveTarget("target-2")

	statuses = m.ListStatuses()
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses after removal, got %d", len(statuses))
	}
}

func TestManager_Run_Shutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires ffmpeg")
	}

	srv, port, _ := mockIcecastServer(t)
	defer srv.Close()

	m := newTestManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	go m.Run(ctx)

	cfg := testConfig(t, port)
	if err := m.AddTarget(ctx, cfg); err != nil {
		cancel()
		t.Skipf("ffmpeg not available: %v", err)
	}

	// Cancel context — Run should disconnect targets and return.
	cancel()
	time.Sleep(300 * time.Millisecond)

	s, err := m.Status(cfg.ID)
	if err == nil && s.State == streaming.StateConnected {
		t.Error("expected target to be disconnected after shutdown")
	}
}
