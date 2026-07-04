package playback_test

import (
	"context"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/testutil"
)

// enqueueBreak enqueues a commercial break with the given number of spots
// (480 frames each) via HandleEnqueueBreak directly on the queue manager.
func (f *fixture) enqueueBreak(t *testing.T, title string, numSpots int) string {
	t.Helper()
	spots := make([]commands.QueueItemInput, numSpots)
	for i := range spots {
		spots[i] = commands.QueueItemInput{
			Path:       "/fake/spot.mp3",
			Type:       "spots",
			Title:      "Spot",
			DurationMS: 10,
		}
	}
	breakID := "brk_TEST_" + title
	err := f.queueMgr.HandleEnqueueBreak(context.Background(), commands.New(
		commands.CmdEnqueueBreak,
		commands.EnqueueBreakPayload{
			BreakID: breakID,
			Break: commands.BreakItemInput{
				Title: title,
				Spots: spots,
			},
		},
	))
	if err != nil {
		t.Fatalf("HandleEnqueueBreak: %v", err)
	}
	return breakID
}

// collectBreakEvents returns all events of the given types from the bus.
func collectBreakEventsOfTypes(evtBus *events.Bus, types ...events.EventType) []events.Event {
	want := make(map[events.EventType]bool, len(types))
	for _, t := range types {
		want[t] = true
	}
	var out []events.Event
	for _, e := range evtBus.Recent(500) {
		if want[e.Type] {
			out = append(out, e)
		}
	}
	return out
}

// --- Tests -------------------------------------------------------------------

func TestBreak_BreakStartedAndEndedPublished(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueueBreak(t, "Bloco 14h", 2)
	f.play(t)
	waitSessionEnd(t, f.evtBus, 5*time.Second)

	evts := collectBreakEventsOfTypes(f.evtBus,
		events.EvtBreakStarted, events.EvtBreakEnded)

	started := countByType(evts, events.EvtBreakStarted)
	ended := countByType(evts, events.EvtBreakEnded)

	if started != 1 {
		t.Errorf("BreakStarted count = %d, want 1", started)
	}
	if ended != 1 {
		t.Errorf("BreakEnded count = %d, want 1", ended)
	}

	// Verify BreakID in payload.
	for _, e := range evts {
		if e.Type == events.EvtBreakStarted {
			p := e.Payload.(events.BreakStartedPayload)
			if p.BreakID != "brk_TEST_Bloco 14h" {
				t.Errorf("BreakStarted.BreakID = %q, want brk_TEST_Bloco 14h", p.BreakID)
			}
			if p.BreakTitle != "Bloco 14h" {
				t.Errorf("BreakStarted.BreakTitle = %q, want Bloco 14h", p.BreakTitle)
			}
			if p.BreakTotal != 2 {
				t.Errorf("BreakStarted.BreakTotal = %d, want 2", p.BreakTotal)
			}
		}
	}
}

func TestBreak_SpotStartedPublishedForEachSpot(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueueBreak(t, "Bloco", 3)
	f.play(t)
	waitSessionEnd(t, f.evtBus, 5*time.Second)

	evts := collectBreakEventsOfTypes(f.evtBus, events.EvtSpotStarted)
	if len(evts) != 3 {
		t.Errorf("SpotStarted count = %d, want 3", len(evts))
	}

	// Verify sequence ordering.
	for i, e := range evts {
		p := e.Payload.(events.SpotStartedPayload)
		if p.BreakSeq != i+1 {
			t.Errorf("SpotStarted[%d].BreakSeq = %d, want %d", i, p.BreakSeq, i+1)
		}
		if p.BreakTotal != 3 {
			t.Errorf("SpotStarted[%d].BreakTotal = %d, want 3", i, p.BreakTotal)
		}
		if p.BreakRole != "spot" {
			t.Errorf("SpotStarted[%d].BreakRole = %q, want spot", i, p.BreakRole)
		}
	}
}

func TestBreak_SpotEndedPublishedForEachSpot(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueueBreak(t, "Bloco", 3)
	f.play(t)
	waitSessionEnd(t, f.evtBus, 5*time.Second)

	evts := collectBreakEventsOfTypes(f.evtBus, events.EvtSpotEnded)
	if len(evts) != 3 {
		t.Errorf("SpotEnded count = %d, want 3", len(evts))
	}
}

func TestBreak_BreakEndedOnlyOnceForSingleBreak(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueueBreak(t, "Bloco", 4)
	f.play(t)
	waitSessionEnd(t, f.evtBus, 5*time.Second)

	evts := collectBreakEventsOfTypes(f.evtBus, events.EvtBreakEnded)
	if len(evts) != 1 {
		t.Errorf("BreakEnded count = %d, want 1 (not once per spot)", len(evts))
	}
}

func TestBreak_TwoBreaks_IndependentEvents(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueueBreak(t, "Bloco1", 2)
	f.enqueueBreak(t, "Bloco2", 2)
	f.play(t)
	waitSessionEnd(t, f.evtBus, 8*time.Second)

	started := collectBreakEventsOfTypes(f.evtBus, events.EvtBreakStarted)
	ended := collectBreakEventsOfTypes(f.evtBus, events.EvtBreakEnded)
	spotStarted := collectBreakEventsOfTypes(f.evtBus, events.EvtSpotStarted)
	spotEnded := collectBreakEventsOfTypes(f.evtBus, events.EvtSpotEnded)

	if len(started) != 2 {
		t.Errorf("BreakStarted count = %d, want 2", len(started))
	}
	if len(ended) != 2 {
		t.Errorf("BreakEnded count = %d, want 2", len(ended))
	}
	if len(spotStarted) != 4 {
		t.Errorf("SpotStarted count = %d, want 4 (2 per break)", len(spotStarted))
	}
	if len(spotEnded) != 4 {
		t.Errorf("SpotEnded count = %d, want 4 (2 per break)", len(spotEnded))
	}

	// Verify both breaks have distinct IDs.
	ids := map[string]bool{}
	for _, e := range started {
		p := e.Payload.(events.BreakStartedPayload)
		ids[p.BreakID] = true
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 distinct break IDs, got %d: %v", len(ids), ids)
	}
}

func TestBreak_MusicThenBreak_BreakStartedAfterMusic(t *testing.T) {
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	// Enqueue music first, then break.
	f.enqueue("musicas", 48)
	f.enqueueBreak(t, "Bloco", 2)
	f.play(t)
	waitSessionEnd(t, f.evtBus, 8*time.Second)

	allEvts := f.evtBus.Recent(500)

	// BreakStarted must come AFTER ItemStarted for the music item.
	musicStartIdx := -1
	breakStartIdx := -1
	for i, e := range allEvts {
		if e.Type == events.EvtItemStarted {
			p := e.Payload.(events.ItemStartedPayload)
			if p.AssetID == "asset-musicas" && musicStartIdx == -1 {
				musicStartIdx = i
			}
		}
		if e.Type == events.EvtBreakStarted && breakStartIdx == -1 {
			breakStartIdx = i
		}
	}
	if musicStartIdx < 0 {
		t.Fatal("ItemStarted for music not found")
	}
	if breakStartIdx < 0 {
		t.Fatal("BreakStarted not found")
	}
	if breakStartIdx <= musicStartIdx {
		t.Errorf("BreakStarted (idx=%d) should come after ItemStarted for music (idx=%d)",
			breakStartIdx, musicStartIdx)
	}
}

func TestBreak_NowPlayingChangedContainsBreakFields(t *testing.T) {
	// FakeDecoder with small frame count runs instantly (non-realtime).
	// We verify via the NowPlayingChanged event published by startItem,
	// which is reliably in the ring buffer after the session ends.
	dec := &testutil.FakeDecoder{Frames: 48}
	f := newFixture(t, dec, false, 0)

	f.enqueueBreak(t, "Bloco NP", 1)
	f.play(t)
	waitSessionEnd(t, f.evtBus, 5*time.Second)

	// Find the NowPlayingChanged event that carries break fields.
	var found *events.NowPlayingChangedPayload
	for _, e := range f.evtBus.Recent(200) {
		if e.Type != events.EvtNowPlayingChanged {
			continue
		}
		p := e.Payload.(events.NowPlayingChangedPayload)
		if p.BreakID != "" {
			found = &p
			break
		}
	}
	if found == nil {
		t.Fatal("no NowPlayingChanged event with BreakID found")
	}
	if found.BreakTitle != "Bloco NP" {
		t.Errorf("BreakTitle = %q, want Bloco NP", found.BreakTitle)
	}
	if found.BreakPosition != 1 {
		t.Errorf("BreakPosition = %d, want 1", found.BreakPosition)
	}
	if found.BreakTotal != 1 {
		t.Errorf("BreakTotal = %d, want 1", found.BreakTotal)
	}
	if found.BreakRole != "spot" {
		t.Errorf("BreakRole = %q, want spot", found.BreakRole)
	}
}

// --- helper ------------------------------------------------------------------

func countByType(evts []events.Event, t events.EventType) int {
	n := 0
	for _, e := range evts {
		if e.Type == t {
			n++
		}
	}
	return n
}
