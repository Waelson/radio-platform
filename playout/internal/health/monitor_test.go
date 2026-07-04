package health_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/health"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// windowSamples returns the number of PCM samples in a 50 ms window.
func windowSamples(sampleRate, channels int) int {
	return sampleRate * 50 / 1000 * channels
}

// makeSamples creates a slice of identical PCM samples with the given amplitude.
func makeSamples(amplitude float32, count int) []float32 {
	s := make([]float32, count)
	for i := range s {
		s[i] = amplitude
	}
	return s
}

// TestMonitor_LevelDBFS verifies that pushing a known constant amplitude
// produces the expected dBFS level in the AudioHealthChanged event.
func TestMonitor_LevelDBFS(t *testing.T) {
	// amplitude 0.1 → RMS = 0.1 → 20*log10(0.1) = -20 dBFS
	const amplitude = float32(0.1)
	wantDBFS := 20 * math.Log10(0.1) // -20.0

	cfg := health.Config{
		IntervalMS:           50,
		SilenceThresholdDBFS: -60,
		SilenceDurationMS:    2000,
		SampleRate:           48000,
		Channels:             2,
	}
	evtBus := events.NewBus(nil)
	mon := health.NewMonitor(cfg, evtBus, nil, nil)

	// Subscribe before starting Run so we don't miss the first tick.
	ch, unsub := evtBus.Subscribe(16)
	defer unsub()

	ws := windowSamples(cfg.SampleRate, cfg.Channels)
	mon.Push(makeSamples(amplitude, ws*2)) // push 2 windows

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	go mon.Run(ctx)

	deadline := time.NewTimer(250 * time.Millisecond)
	defer deadline.Stop()
	for {
		select {
		case evt := <-ch:
			if evt.Type != events.EvtAudioHealthChanged {
				continue
			}
			p, ok := evt.Payload.(events.AudioHealthChangedPayload)
			if !ok {
				t.Fatalf("unexpected payload type %T", evt.Payload)
			}
			// Allow ±1 dBFS tolerance.
			if math.Abs(p.LevelDBFS-wantDBFS) > 1.0 {
				t.Errorf("LevelDBFS: want ~%.1f dBFS, got %.1f dBFS", wantDBFS, p.LevelDBFS)
			}
			return
		case <-deadline.C:
			t.Fatal("no AudioHealthChanged event received within timeout")
		}
	}
}

// TestMonitor_StateUpdated verifies that Run writes health metrics into the
// state manager snapshot.
func TestMonitor_StateUpdated(t *testing.T) {
	cfg := health.Config{
		IntervalMS:           50,
		SilenceThresholdDBFS: -60,
		SilenceDurationMS:    2000,
		SampleRate:           48000,
		Channels:             2,
	}
	evtBus := events.NewBus(nil)
	stateMgr := state.NewManager("test")
	mon := health.NewMonitor(cfg, evtBus, stateMgr, nil)

	ch, unsub := evtBus.Subscribe(16)
	defer unsub()

	ws := windowSamples(cfg.SampleRate, cfg.Channels)
	mon.Push(makeSamples(0.5, ws*2))

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	go mon.Run(ctx)

	deadline := time.NewTimer(250 * time.Millisecond)
	defer deadline.Stop()
	for {
		select {
		case evt := <-ch:
			if evt.Type != events.EvtAudioHealthChanged {
				continue
			}
			snap := stateMgr.Snapshot()
			if snap.AudioHealth.LevelDBFS >= 0 || snap.AudioHealth.LevelDBFS < -120 {
				t.Errorf("unexpected LevelDBFS in state: %v", snap.AudioHealth.LevelDBFS)
			}
			return
		case <-deadline.C:
			t.Fatal("no AudioHealthChanged event received within timeout")
		}
	}
}

// TestMonitor_SilenceAlert verifies that AlertRaised fires when silence
// persists beyond SilenceDurationMS.
func TestMonitor_SilenceAlert(t *testing.T) {
	cfg := health.Config{
		IntervalMS:           30,
		SilenceThresholdDBFS: -60,
		SilenceDurationMS:    60, // short for test speed
		SampleRate:           48000,
		Channels:             2,
	}
	evtBus := events.NewBus(nil)
	mon := health.NewMonitor(cfg, evtBus, nil, nil)

	ch, unsub := evtBus.Subscribe(32)
	defer unsub()

	ws := windowSamples(cfg.SampleRate, cfg.Channels)
	mon.Push(makeSamples(0, ws*3)) // push 3 windows of silence

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go mon.Run(ctx)

	deadline := time.NewTimer(450 * time.Millisecond)
	defer deadline.Stop()
	for {
		select {
		case evt := <-ch:
			if evt.Type != events.EvtAlertRaised {
				continue
			}
			p, ok := evt.Payload.(events.AlertRaisedPayload)
			if !ok {
				t.Fatalf("unexpected payload type %T", evt.Payload)
			}
			if p.Source != "audio_health" {
				t.Errorf("AlertRaised.Source: want audio_health, got %s", p.Source)
			}
			if p.Severity != "WARNING" {
				t.Errorf("AlertRaised.Severity: want WARNING, got %s", p.Severity)
			}
			if p.AlertID == "" {
				t.Error("AlertRaised.AlertID must not be empty")
			}
			return // pass
		case <-deadline.C:
			t.Fatal("AlertRaised did not fire within timeout")
		}
	}
}

// TestMonitor_SilenceCleared verifies that AlertCleared fires after audio
// resumes above the silence threshold.
func TestMonitor_SilenceCleared(t *testing.T) {
	cfg := health.Config{
		IntervalMS:           30,
		SilenceThresholdDBFS: -60,
		SilenceDurationMS:    60,
		SampleRate:           48000,
		Channels:             2,
	}
	evtBus := events.NewBus(nil)
	mon := health.NewMonitor(cfg, evtBus, nil, nil)

	ch, unsub := evtBus.Subscribe(64)
	defer unsub()

	ws := windowSamples(cfg.SampleRate, cfg.Channels)
	mon.Push(makeSamples(0, ws*3)) // push silence to prime latest

	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	go mon.Run(ctx)

	var gotRaised, gotCleared bool
	deadline := time.NewTimer(650 * time.Millisecond)
	defer deadline.Stop()

	for !gotRaised || !gotCleared {
		select {
		case evt := <-ch:
			switch evt.Type {
			case events.EvtAlertRaised:
				if !gotRaised {
					gotRaised = true
					// Respond to raised alert by pushing audio above threshold.
					mon.Push(makeSamples(0.5, ws*2))
				}
			case events.EvtAlertCleared:
				gotCleared = true
			}
		case <-deadline.C:
			t.Fatalf("raised=%v cleared=%v: did not get both AlertRaised and AlertCleared", gotRaised, gotCleared)
		}
	}
}

// TestMonitor_NoAlertBelowDuration verifies that AlertRaised does NOT fire
// when the silence duration is shorter than SilenceDurationMS.
func TestMonitor_NoAlertBelowDuration(t *testing.T) {
	cfg := health.Config{
		IntervalMS:           30,
		SilenceThresholdDBFS: -60,
		SilenceDurationMS:    500, // long — silence won't last this long
		SampleRate:           48000,
		Channels:             2,
	}
	evtBus := events.NewBus(nil)
	mon := health.NewMonitor(cfg, evtBus, nil, nil)

	ch, unsub := evtBus.Subscribe(32)
	defer unsub()

	ws := windowSamples(cfg.SampleRate, cfg.Channels)
	mon.Push(makeSamples(0, ws*2)) // push silence but not long enough

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go mon.Run(ctx)

	// Run for 150 ms — well below the 500 ms silence threshold.
	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case evt := <-ch:
			if evt.Type == events.EvtAlertRaised {
				t.Fatal("AlertRaised should not fire for short silence duration")
			}
		case <-timer.C:
			return // pass
		}
	}
}

// TestMonitor_AutoPanic verifies that OnAutoPanic is called once when silence
// exceeds AutoPanicSilenceDurationMS, and is reset after audio resumes.
func TestMonitor_AutoPanic(t *testing.T) {
	called := make(chan string, 4)

	cfg := health.Config{
		IntervalMS:                 30,
		SilenceThresholdDBFS:       -60,
		SilenceDurationMS:          30,  // alert at 30 ms
		AutoPanicSilenceDurationMS: 60,  // auto-panic at 60 ms
		SampleRate:                 48000,
		Channels:                   2,
		OnAutoPanic: func(reason string) {
			select {
			case called <- reason:
			default:
			}
		},
	}
	evtBus := events.NewBus(nil)
	mon := health.NewMonitor(cfg, evtBus, nil, nil)

	ws := windowSamples(cfg.SampleRate, cfg.Channels)
	mon.Push(makeSamples(0, ws*4)) // push silence

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	go mon.Run(ctx)

	// OnAutoPanic must fire exactly once.
	select {
	case reason := <-called:
		if reason == "" {
			t.Error("OnAutoPanic reason must not be empty")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("OnAutoPanic was not called within timeout")
	}

	// Must not fire a second time.
	select {
	case <-called:
		t.Error("OnAutoPanic fired more than once during the same silence event")
	case <-time.After(80 * time.Millisecond):
		// expected: no second call
	}
}

// TestMonitor_AutoPanic_ResetsAfterAudio verifies that OnAutoPanic can fire
// again after audio resumes and then silence returns.
func TestMonitor_AutoPanic_ResetsAfterAudio(t *testing.T) {
	callCount := make(chan struct{}, 8)

	cfg := health.Config{
		IntervalMS:                 30,
		SilenceThresholdDBFS:       -60,
		SilenceDurationMS:          30,
		AutoPanicSilenceDurationMS: 60,
		SampleRate:                 48000,
		Channels:                   2,
		OnAutoPanic: func(_ string) {
			select {
			case callCount <- struct{}{}:
			default:
			}
		},
	}
	evtBus := events.NewBus(nil)
	mon := health.NewMonitor(cfg, evtBus, nil, nil)

	ch, unsub := evtBus.Subscribe(32)
	defer unsub()

	ws := windowSamples(cfg.SampleRate, cfg.Channels)
	mon.Push(makeSamples(0, ws*4)) // first silence burst

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	go mon.Run(ctx)

	// Wait for first auto-panic.
	select {
	case <-callCount:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first OnAutoPanic not called")
	}

	// Wait for AlertRaised so we know the monitor saw the silence.
	waitForAlert := time.NewTimer(300 * time.Millisecond)
	defer waitForAlert.Stop()
	for {
		select {
		case evt := <-ch:
			if evt.Type == events.EvtAlertRaised {
				goto audioResume
			}
		case <-waitForAlert.C:
			goto audioResume
		}
	}
audioResume:
	// Resume audio — this should reset autoPanicFired and silenceAlertID.
	mon.Push(makeSamples(0.5, ws*4))

	// Wait for AlertCleared to confirm monitor reset its state.
	cleared := time.NewTimer(300 * time.Millisecond)
	defer cleared.Stop()
	for {
		select {
		case evt := <-ch:
			if evt.Type == events.EvtAlertCleared {
				goto secondSilence
			}
		case <-cleared.C:
			goto secondSilence
		}
	}
secondSilence:
	// Second silence burst — auto-panic should fire again.
	mon.Push(makeSamples(0, ws*4))

	select {
	case <-callCount:
		// second auto-panic fired as expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second OnAutoPanic not called after audio resumed")
	}
}

// TestMonitor_IncUnderrun verifies that underrun counts appear in the payload.
func TestMonitor_IncUnderrun(t *testing.T) {
	cfg := health.Config{
		IntervalMS:           50,
		SilenceThresholdDBFS: -60,
		SilenceDurationMS:    2000,
		SampleRate:           48000,
		Channels:             2,
	}
	evtBus := events.NewBus(nil)
	mon := health.NewMonitor(cfg, evtBus, nil, nil)

	ch, unsub := evtBus.Subscribe(16)
	defer unsub()

	mon.IncUnderrun()
	mon.IncUnderrun()

	ws := windowSamples(cfg.SampleRate, cfg.Channels)
	mon.Push(makeSamples(0.1, ws))

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	go mon.Run(ctx)

	deadline := time.NewTimer(250 * time.Millisecond)
	defer deadline.Stop()
	for {
		select {
		case evt := <-ch:
			if evt.Type != events.EvtAudioHealthChanged {
				continue
			}
			p := evt.Payload.(events.AudioHealthChangedPayload)
			if p.UnderrunCount != 2 {
				t.Errorf("UnderrunCount: want 2, got %d", p.UnderrunCount)
			}
			return
		case <-deadline.C:
			t.Fatal("no AudioHealthChanged event received")
		}
	}
}
