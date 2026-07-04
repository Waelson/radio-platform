package events_test

import (
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
)

func TestNew_SetsFields(t *testing.T) {
	before := time.Now().UTC().Add(-time.Millisecond)
	evt := events.New(events.EvtEngineStarted, nil)
	after := time.Now().UTC().Add(time.Millisecond)

	if evt.Type != events.EvtEngineStarted {
		t.Errorf("Type = %s, want %s", evt.Type, events.EvtEngineStarted)
	}
	if evt.Version != 1 {
		t.Errorf("Version = %d, want 1", evt.Version)
	}
	if len(evt.EventID) < 5 || evt.EventID[:4] != "evt_" {
		t.Errorf("EventID = %q, want evt_<ulid>", evt.EventID)
	}
	if evt.Timestamp.Before(before) || evt.Timestamp.After(after) {
		t.Errorf("Timestamp %v outside expected range", evt.Timestamp)
	}
}

func TestBus_PublishSubscribeReceive(t *testing.T) {
	bus := events.NewBus(nil)

	ch, cancel := bus.Subscribe(8)
	defer cancel()

	evt := events.New(events.EvtEngineStarted, nil)
	bus.Publish(evt)

	select {
	case received := <-ch:
		if received.EventID != evt.EventID {
			t.Errorf("received EventID %q, want %q", received.EventID, evt.EventID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for event")
	}
}

func TestBus_FanOut(t *testing.T) {
	bus := events.NewBus(nil)

	ch1, cancel1 := bus.Subscribe(8)
	defer cancel1()
	ch2, cancel2 := bus.Subscribe(8)
	defer cancel2()

	evt := events.New(events.EvtNowPlayingChanged, nil)
	bus.Publish(evt)

	timeout := time.After(100 * time.Millisecond)
	for _, ch := range []<-chan events.Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.EventID != evt.EventID {
				t.Errorf("subscriber got wrong event ID %q", got.EventID)
			}
		case <-timeout:
			t.Fatal("timed out waiting for fan-out delivery")
		}
	}
}

func TestBus_UnsubscribeStopsDelivery(t *testing.T) {
	bus := events.NewBus(nil)

	ch, cancel := bus.Subscribe(8)
	cancel() // unsubscribe immediately

	bus.Publish(events.New(events.EvtQueueChanged, nil))

	select {
	case <-ch:
		t.Fatal("received event after cancel")
	case <-time.After(20 * time.Millisecond):
		// expected: nothing delivered after cancel
	}
}

func TestBus_Recent_ReturnsPublishedEvents(t *testing.T) {
	bus := events.NewBus(nil)

	for i := 0; i < 5; i++ {
		bus.Publish(events.New(events.EvtProgressChanged, nil))
	}

	got := bus.Recent(3)
	if len(got) != 3 {
		t.Fatalf("Recent(3) = %d events, want 3", len(got))
	}
}

func TestBus_Recent_EmptyBus(t *testing.T) {
	bus := events.NewBus(nil)
	got := bus.Recent(5)
	if len(got) != 0 {
		t.Fatalf("Recent on empty bus = %d, want 0", len(got))
	}
}

func TestBus_Recent_LargerThanStored(t *testing.T) {
	bus := events.NewBus(nil)
	bus.Publish(events.New(events.EvtProgressChanged, nil))
	bus.Publish(events.New(events.EvtProgressChanged, nil))

	got := bus.Recent(100)
	if len(got) != 2 {
		t.Fatalf("Recent(100) on 2-event bus = %d events, want 2", len(got))
	}
}

func TestBus_SlowSubscriberDoesNotBlockPublish(t *testing.T) {
	bus := events.NewBus(nil)

	// Subscribe with a tiny buffer and never read.
	_, cancel := bus.Subscribe(1)
	defer cancel()

	// Publish many events — should never block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 300; i++ {
			bus.Publish(events.New(events.EvtProgressChanged, nil))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on slow subscriber")
	}
}

func TestIsCritical(t *testing.T) {
	critical := []events.EventType{
		events.EvtPanicEntered,
		events.EvtPanicExited,
		events.EvtCommandRejected,
		events.EvtAlertRaised,
		events.EvtEngineStarted,
		events.EvtEngineStopping,
		events.EvtPlaybackError,
		events.EvtDecoderError,
		events.EvtOutputOpenFailed,
		events.EvtOutputWriteFailed,
	}
	for _, et := range critical {
		if !events.IsCritical(et) {
			t.Errorf("IsCritical(%s) = false, want true", et)
		}
	}

	nonCritical := []events.EventType{
		events.EvtProgressChanged,
		events.EvtAudioHealthChanged,
		events.EvtQueueChanged,
		events.EvtNowPlayingChanged,
	}
	for _, et := range nonCritical {
		if events.IsCritical(et) {
			t.Errorf("IsCritical(%s) = true, want false", et)
		}
	}
}
