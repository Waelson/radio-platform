package transmissionlog_test

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/transmissionlog"
)

func newBus(t *testing.T) *events.Bus {
	t.Helper()
	return events.NewBus(slog.Default())
}

func runWriter(ctx context.Context, t *testing.T, bus *events.Bus, dir string) {
	t.Helper()
	cfg := transmissionlog.Config{
		Dir:              dir,
		FileNameTemplate: "transmission_{date}_{hour}.jsonl",
	}
	w := transmissionlog.New(cfg, bus, slog.Default())
	go w.Run(ctx) //nolint:errcheck
	// Give the goroutine a moment to subscribe before tests publish events.
	time.Sleep(20 * time.Millisecond)
}

// publish sends an event on the bus and sleeps briefly so the goroutine processes it.
func publish(bus *events.Bus, typ events.EventType, payload any) {
	bus.Publish(events.New(typ, payload))
	time.Sleep(30 * time.Millisecond)
}

func readEntries(t *testing.T, dir string) []transmissionlog.LogEntry {
	t.Helper()
	var entries []transmissionlog.LogEntry
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		path := filepath.Join(dir, f.Name())
		fh, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		sc := bufio.NewScanner(fh)
		for sc.Scan() {
			var e transmissionlog.LogEntry
			if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
				t.Fatalf("unmarshal line in %s: %v", path, err)
			}
			entries = append(entries, e)
		}
		fh.Close()
	}
	return entries
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestWriter_NowPlayingThenItemFinished_WritesEntry(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(t)
	runWriter(ctx, t, bus, dir)

	qid := "qi_abc123"
	now := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)

	// Simulate NowPlayingChanged.
	np := events.NowPlayingChangedPayload{
		QueueItemID: qid,
		AssetID:     "asset-1",
		Title:       "Como Nossos Pais",
		Artist:      "Elis Regina",
		Type:        "MUSIC",
		DurationMS:  238000,
		ISRC:        "BR-UM7-12-00123",
		Composer:    "Milton Nascimento",
	}
	evt := events.New(events.EvtNowPlayingChanged, np)
	evt.Timestamp = now
	bus.Publish(evt)
	time.Sleep(30 * time.Millisecond)

	// Simulate ItemFinished.
	fin := events.New(events.EvtItemFinished, events.ItemFinishedPayload{
		QueueItemID:      qid,
		AssetID:          "asset-1",
		Result:           "finished",
		DurationPlayedMS: 235000,
	})
	fin.Timestamp = now.Add(3*time.Minute + 55*time.Second)
	bus.Publish(fin)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(30 * time.Millisecond)

	entries := readEntries(t, dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Title != "Como Nossos Pais" {
		t.Errorf("Title: got %q, want %q", e.Title, "Como Nossos Pais")
	}
	if e.ISRC != "BR-UM7-12-00123" {
		t.Errorf("ISRC: got %q, want %q", e.ISRC, "BR-UM7-12-00123")
	}
	if e.Composer != "Milton Nascimento" {
		t.Errorf("Composer: got %q, want %q", e.Composer, "Milton Nascimento")
	}
	if e.Result != "finished" {
		t.Errorf("Result: got %q, want %q", e.Result, "finished")
	}
	if e.DurationPlayedMS != 235000 {
		t.Errorf("DurationPlayedMS: got %d, want 235000", e.DurationPlayedMS)
	}
}

func TestWriter_ItemFinishedWithoutNowPlaying_Ignored(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(t)
	runWriter(ctx, t, bus, dir)

	// ItemFinished with no prior NowPlayingChanged — should be silently ignored.
	publish(bus, events.EvtItemFinished, events.ItemFinishedPayload{
		QueueItemID:      "qi_orphan",
		AssetID:          "asset-x",
		Result:           "finished",
		DurationPlayedMS: 100000,
	})

	cancel()
	time.Sleep(30 * time.Millisecond)

	entries := readEntries(t, dir)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestWriter_CartStartedThenStopped_WritesEntry(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(t)
	runWriter(ctx, t, bus, dir)

	cartID := "cart_jingle_01"
	cartStart := events.New(events.EvtCartStarted, events.CartStartedPayload{
		CartID:     cartID,
		Title:      "Jingle Entrada",
		Artist:     "Radio XYZ",
		DurationMS: 15000,
	})
	cartStart.Timestamp = time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)
	bus.Publish(cartStart)
	time.Sleep(30 * time.Millisecond)

	cartStop := events.New(events.EvtCartStopped, events.CartStoppedPayload{
		CartID: cartID,
		Reason: "finished",
	})
	cartStop.Timestamp = cartStart.Timestamp.Add(15 * time.Second)
	bus.Publish(cartStop)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(30 * time.Millisecond)

	entries := readEntries(t, dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Type != "CART" {
		t.Errorf("Type: got %q, want CART", e.Type)
	}
	if e.Title != "Jingle Entrada" {
		t.Errorf("Title: got %q, want %q", e.Title, "Jingle Entrada")
	}
	if e.Result != "finished" {
		t.Errorf("Result: got %q, want finished", e.Result)
	}
}

func TestWriter_CartStopped_Manual_ResultSkipped(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(t)
	runWriter(ctx, t, bus, dir)

	cartID := "cart_skipped"
	cs := events.New(events.EvtCartStarted, events.CartStartedPayload{
		CartID: cartID, Title: "Spot X", DurationMS: 30000,
	})
	cs.Timestamp = time.Now().UTC()
	bus.Publish(cs)
	time.Sleep(30 * time.Millisecond)

	ce := events.New(events.EvtCartStopped, events.CartStoppedPayload{
		CartID: cartID, Reason: "manual",
	})
	ce.Timestamp = cs.Timestamp.Add(5 * time.Second)
	bus.Publish(ce)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(30 * time.Millisecond)

	entries := readEntries(t, dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Result != "skipped" {
		t.Errorf("Result: got %q, want skipped", entries[0].Result)
	}
}

func TestWriter_HourRotation_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(t)
	runWriter(ctx, t, bus, dir)

	qid1 := "qi_hour_08"
	qid2 := "qi_hour_09"

	base := time.Date(2026, 7, 20, 8, 55, 0, 0, time.UTC)

	np1 := events.New(events.EvtNowPlayingChanged, events.NowPlayingChangedPayload{
		QueueItemID: qid1, AssetID: "a1", Title: "Track A", Type: "MUSIC", DurationMS: 300000,
	})
	np1.Timestamp = base
	bus.Publish(np1)
	time.Sleep(20 * time.Millisecond)

	// Finished at 09:02 → goes into the 09 file.
	fin1 := events.New(events.EvtItemFinished, events.ItemFinishedPayload{
		QueueItemID: qid1, AssetID: "a1", Result: "finished", DurationPlayedMS: 300000,
	})
	fin1.Timestamp = base.Add(7 * time.Minute) // 09:02
	bus.Publish(fin1)
	time.Sleep(50 * time.Millisecond)

	np2 := events.New(events.EvtNowPlayingChanged, events.NowPlayingChangedPayload{
		QueueItemID: qid2, AssetID: "a2", Title: "Track B", Type: "MUSIC", DurationMS: 240000,
	})
	np2.Timestamp = fin1.Timestamp
	bus.Publish(np2)
	time.Sleep(20 * time.Millisecond)

	fin2 := events.New(events.EvtItemFinished, events.ItemFinishedPayload{
		QueueItemID: qid2, AssetID: "a2", Result: "finished", DurationPlayedMS: 240000,
	})
	fin2.Timestamp = fin1.Timestamp.Add(4 * time.Minute) // 09:06
	bus.Publish(fin2)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(30 * time.Millisecond)

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(files) != 1 {
		// Both FinishedAt timestamps are in hour 09 → same file.
		// fin1.Timestamp = 09:02, fin2.Timestamp = 09:06 → both in 09 file.
		t.Logf("files in dir:")
		for _, f := range files {
			t.Logf("  %s", f.Name())
		}
		t.Fatalf("expected 1 file (both in hour 09), got %d", len(files))
	}
}

func TestWriter_TwoHours_TwoFiles(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := newBus(t)
	runWriter(ctx, t, bus, dir)

	// Track finishing at 08:59
	qid1 := "qi_h08"
	np1 := events.New(events.EvtNowPlayingChanged, events.NowPlayingChangedPayload{
		QueueItemID: qid1, AssetID: "a1", Title: "Track 08", Type: "MUSIC", DurationMS: 180000,
	})
	np1.Timestamp = time.Date(2026, 7, 20, 8, 56, 0, 0, time.UTC)
	bus.Publish(np1)
	time.Sleep(20 * time.Millisecond)

	fin1 := events.New(events.EvtItemFinished, events.ItemFinishedPayload{
		QueueItemID: qid1, AssetID: "a1", Result: "finished", DurationPlayedMS: 180000,
	})
	fin1.Timestamp = time.Date(2026, 7, 20, 8, 59, 0, 0, time.UTC)
	bus.Publish(fin1)
	time.Sleep(50 * time.Millisecond)

	// Track finishing at 10:05 (different hour)
	qid2 := "qi_h10"
	np2 := events.New(events.EvtNowPlayingChanged, events.NowPlayingChangedPayload{
		QueueItemID: qid2, AssetID: "a2", Title: "Track 10", Type: "MUSIC", DurationMS: 300000,
	})
	np2.Timestamp = time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	bus.Publish(np2)
	time.Sleep(20 * time.Millisecond)

	fin2 := events.New(events.EvtItemFinished, events.ItemFinishedPayload{
		QueueItemID: qid2, AssetID: "a2", Result: "finished", DurationPlayedMS: 300000,
	})
	fin2.Timestamp = time.Date(2026, 7, 20, 10, 5, 0, 0, time.UTC)
	bus.Publish(fin2)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(30 * time.Millisecond)

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(files) != 2 {
		t.Logf("files:")
		for _, f := range files {
			t.Logf("  %s", f.Name())
		}
		t.Fatalf("expected 2 files (hour 08 and hour 10), got %d", len(files))
	}
}

func TestWriter_Shutdown_ClosesFileCleanly(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())

	bus := newBus(t)
	runWriter(ctx, t, bus, dir)

	qid := "qi_shutdown"
	np := events.New(events.EvtNowPlayingChanged, events.NowPlayingChangedPayload{
		QueueItemID: qid, AssetID: "a1", Title: "Shutdown Track", Type: "MUSIC", DurationMS: 60000,
	})
	np.Timestamp = time.Now().UTC()
	bus.Publish(np)
	time.Sleep(20 * time.Millisecond)

	fin := events.New(events.EvtItemFinished, events.ItemFinishedPayload{
		QueueItemID: qid, AssetID: "a1", Result: "finished", DurationPlayedMS: 60000,
	})
	fin.Timestamp = np.Timestamp.Add(time.Minute)
	bus.Publish(fin)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	// After shutdown, file must exist and be readable.
	entries := readEntries(t, dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after shutdown, got %d", len(entries))
	}
}

// ── buildFileName unit tests ───────────────────────────────────────────────────

func TestBuildFileName(t *testing.T) {
	cases := []struct {
		template string
		date     string
		hour     int
		want     string
	}{
		{"transmission_{date}_{hour}.jsonl", "20260720", 8, "transmission_20260720_08.jsonl"},
		{"transmission_{date}_{hour}.jsonl", "20260720", 23, "transmission_20260720_23.jsonl"},
		{"log_{date}T{hour}.jsonl", "20260101", 0, "log_20260101T00.jsonl"},
		{"no_placeholders.jsonl", "20260720", 12, "no_placeholders.jsonl"},
	}
	for _, tc := range cases {
		got := transmissionlog.BuildFileName(tc.template, tc.date, tc.hour)
		if got != tc.want {
			t.Errorf("buildFileName(%q, %q, %d) = %q, want %q",
				tc.template, tc.date, tc.hour, got, tc.want)
		}
	}
}
