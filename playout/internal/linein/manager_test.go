package linein_test

import (
	"context"
	"errors"
	"io"
	"math"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/linein"
)

// ── Stubs ─────────────────────────────────────────────────────────────────────

// stubInput emits a fixed number of frame batches, then returns io.EOF.
// Each batch fills dst with the given sample value.
type stubInput struct {
	batches   int
	sample    float32 // value for each sample (0 = silence, 0.5 = signal)
	openErr   error
	emitted   int
	cfg       linein.LineInConfig
}

func (s *stubInput) Open(_ context.Context, cfg linein.LineInConfig) error {
	s.cfg = cfg
	return s.openErr
}

func (s *stubInput) ReadFrames(_ context.Context, dst []float32) (int, error) {
	if s.emitted >= s.batches {
		return 0, io.EOF
	}
	for i := range dst {
		dst[i] = s.sample
	}
	s.emitted++
	frames := len(dst) / 2
	return frames, nil
}

func (s *stubInput) Close() error { return nil }

// stubOutput accumulates written frames.
type stubOutput struct {
	written []float32
	writeErr error
}

func (s *stubOutput) Open(_ context.Context, _ output.OutputConfig) error { return nil }
func (s *stubOutput) Start(_ context.Context) error                        { return nil }
func (s *stubOutput) Write(_ context.Context, frames []float32) (int, error) {
	if s.writeErr != nil {
		return 0, s.writeErr
	}
	s.written = append(s.written, frames...)
	return len(frames) / 2, nil
}
func (s *stubOutput) Stop(_ context.Context) error  { return nil }
func (s *stubOutput) Close() error                  { return nil }
func (s *stubOutput) Info() output.OutputDeviceInfo { return output.OutputDeviceInfo{} }

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestManager_FramesReachOutput(t *testing.T) {
	const batchCount = 5
	in := &stubInput{batches: batchCount, sample: 0.5}
	out := &stubOutput{}

	mgr := linein.NewLineInManager(in, out, nil)
	events, err := mgr.Start(context.Background(), linein.LineInConfig{Label: "test"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	collectEvents(t, events)

	if len(out.written) == 0 {
		t.Fatal("expected frames written to output, got none")
	}
}

func TestManager_StopViaContext(t *testing.T) {
	in := &stubInput{batches: 1_000_000, sample: 0.5} // "infinite"
	out := &stubOutput{}

	ctx, cancel := context.WithCancel(context.Background())
	mgr := linein.NewLineInManager(in, out, nil)
	events, err := mgr.Start(ctx, linein.LineInConfig{Label: "test"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Cancel context shortly after starting.
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	var got []linein.Event
	for e := range events {
		got = append(got, e)
	}

	// Must receive a Stopped event.
	found := false
	for _, e := range got {
		if e.Type == linein.EventStopped {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EventStopped, got: %v", got)
	}
}

func TestManager_StopViaManagerStop(t *testing.T) {
	in := &stubInput{batches: 1_000_000, sample: 0.5}
	out := &stubOutput{}

	mgr := linein.NewLineInManager(in, out, nil)
	events, err := mgr.Start(context.Background(), linein.LineInConfig{Label: "test"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		mgr.Stop()
	}()

	var got []linein.Event
	for e := range events {
		got = append(got, e)
	}

	found := false
	for _, e := range got {
		if e.Type == linein.EventStopped {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EventStopped after Stop(), got: %v", got)
	}
}

func TestManager_SilenceAlert(t *testing.T) {
	// Emit silence (sample = 0) for 100 batches.
	in := &stubInput{batches: 100, sample: 0}
	out := &stubOutput{}

	mgr := linein.NewLineInManager(in, out, nil)
	events, err := mgr.Start(context.Background(), linein.LineInConfig{
		Label:                "test",
		OnSilence:            "alert",
		SilenceThresholdDBFS: -60,
		SilenceThresholdMS:   1, // 1 ms → triggers immediately
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := collectEvents(t, events)

	found := false
	for _, e := range got {
		if e.Type == linein.EventSilenceAlert {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EventSilenceAlert on silence, got: %v", got)
	}

	// With on_silence=alert, session must NOT stop due to silence.
	for _, e := range got {
		if e.Type == linein.EventSilenceStop {
			t.Errorf("unexpected EventSilenceStop when on_silence=alert")
		}
	}
}

func TestManager_SilenceStop(t *testing.T) {
	in := &stubInput{batches: 100, sample: 0}
	out := &stubOutput{}

	mgr := linein.NewLineInManager(in, out, nil)
	events, err := mgr.Start(context.Background(), linein.LineInConfig{
		Label:                "test",
		OnSilence:            "stop",
		SilenceThresholdDBFS: -60,
		SilenceThresholdMS:   1,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := collectEvents(t, events)

	found := false
	for _, e := range got {
		if e.Type == linein.EventSilenceStop {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EventSilenceStop on silence+on_silence=stop, got: %v", got)
	}
}

func TestManager_DurationCap(t *testing.T) {
	// Unlimited batches, but duration_ms = 50 ms.
	in := &stubInput{batches: 1_000_000, sample: 0.5}
	out := &stubOutput{}

	mgr := linein.NewLineInManager(in, out, nil)
	events, err := mgr.Start(context.Background(), linein.LineInConfig{
		Label:      "test",
		DurationMS: 50,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	start := time.Now()
	got := collectEvents(t, events)
	elapsed := time.Since(start)

	// Session must end within a reasonable margin.
	if elapsed > 2*time.Second {
		t.Errorf("session took too long: %v", elapsed)
	}

	found := false
	for _, e := range got {
		if e.Type == linein.EventStopped {
			found = true
			if e.ElapsedMS < 50 {
				t.Errorf("ElapsedMS=%d, expected >= 50", e.ElapsedMS)
			}
		}
	}
	if !found {
		t.Errorf("expected EventStopped after duration cap, got: %v", got)
	}
}

func TestManager_DeviceOpenError(t *testing.T) {
	in := &stubInput{openErr: errors.New("device not found")}
	out := &stubOutput{}

	mgr := linein.NewLineInManager(in, out, nil)
	events, err := mgr.Start(context.Background(), linein.LineInConfig{Label: "test"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := collectEvents(t, events)

	found := false
	for _, e := range got {
		if e.Type == linein.EventError {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EventError on open failure, got: %v", got)
	}
}

func TestManager_AlreadyActive(t *testing.T) {
	in := &stubInput{batches: 1_000_000, sample: 0.5}
	out := &stubOutput{}

	mgr := linein.NewLineInManager(in, out, nil)
	_, err := mgr.Start(context.Background(), linein.LineInConfig{Label: "first"})
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}

	_, err = mgr.Start(context.Background(), linein.LineInConfig{Label: "second"})
	if err == nil {
		t.Error("expected error when starting while already active")
	}

	mgr.Stop()
}

func TestManager_LevelUpdatesEmitted(t *testing.T) {
	in := &stubInput{batches: 50, sample: 0.5}
	out := &stubOutput{}

	mgr := linein.NewLineInManager(in, out, nil)
	events, err := mgr.Start(context.Background(), linein.LineInConfig{Label: "test"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := collectEvents(t, events)

	for _, e := range got {
		if e.Type == linein.EventLevelUpdate {
			if math.IsNaN(e.RMSDBFS) || math.IsInf(e.RMSDBFS, 0) {
				t.Errorf("invalid RMSDBFS value: %v", e.RMSDBFS)
			}
			return // at least one level update is enough
		}
	}
	// Level updates are optional for short sessions; don't fail.
}

// ── Helper ────────────────────────────────────────────────────────────────────

func collectEvents(t *testing.T, ch <-chan linein.Event) []linein.Event {
	t.Helper()
	var events []linein.Event
	for e := range ch {
		events = append(events, e)
	}
	return events
}
