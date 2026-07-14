package handlers_test

// Integration tests for loudness normalization.
// All tests use a real SQLite :memory: database so that migrations, store
// methods, and handlers are exercised together.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
	"github.com/Waelson/radio-library-service/internal/store"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// insertTrack upserts a track and returns its ID.
func insertTrack(t *testing.T, ts *store.TrackStore, tr store.Track) string {
	t.Helper()
	if err := ts.Upsert(context.Background(), tr); err != nil {
		t.Fatalf("upsert track: %v", err)
	}
	got, err := ts.FindByPath(context.Background(), tr.Path)
	if err != nil {
		t.Fatalf("FindByPath: %v", err)
	}
	return got.ID
}

// setLoudness sets loudness values for a track.
func setLoudness(t *testing.T, ts *store.TrackStore, id string, lufs, peak float64) {
	t.Helper()
	if err := ts.UpdateLoudness(context.Background(), id, lufs, peak); err != nil {
		t.Fatalf("UpdateLoudness: %v", err)
	}
}

// getTrackJSON calls GET /v1/tracks/{id} and decodes the JSON response.
func getTrackJSON(t *testing.T, ts *store.TrackStore, ss *store.SettingsStore, id string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/tracks/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	handlers.GetTrack(ts, ss)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/tracks/%s → %d: %s", id, rec.Code, rec.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return m
}

// ─── fakeIntegrationWorker ───────────────────────────────────────────────────

type fakeIntegrationWorker struct {
	enqueued []string
	ts       *store.TrackStore
}

func (w *fakeIntegrationWorker) Enqueue(id string) { w.enqueued = append(w.enqueued, id) }
func (w *fakeIntegrationWorker) IsRunning() bool   { return true }
func (w *fakeIntegrationWorker) DrainQueue()       { w.enqueued = nil }
func (w *fakeIntegrationWorker) Status(ctx context.Context) (map[string]int, error) {
	return w.ts.CountByLoudnessStatus(ctx)
}

// ─── gain_db tests ────────────────────────────────────────────────────────────

func TestIntegration_GainDB_ZeroWhenNoLoudness(t *testing.T) {
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)
	ss := store.NewSettingsStore(db)

	id := insertTrack(t, ts, store.Track{
		Path: "/music/a.mp3", Title: "A", Type: "MUSIC", DurationMS: 180000,
	})

	body := getTrackJSON(t, ts, ss, id)
	if body["gain_db"] != 0.0 {
		t.Errorf("gain_db = %v, want 0.0 (no loudness yet)", body["gain_db"])
	}
	if body["loudness_status"] != "pending" {
		t.Errorf("loudness_status = %v, want pending", body["loudness_status"])
	}
}

func TestIntegration_GainDB_CorrectValue(t *testing.T) {
	// Track measured at -20 LUFS, target -16 LUFS → gain = +4 dB.
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)
	ss := store.NewSettingsStore(db)

	id := insertTrack(t, ts, store.Track{
		Path: "/music/b.mp3", Title: "B", Type: "MUSIC", DurationMS: 200000,
	})
	setLoudness(t, ts, id, -20.0, -1.0)

	body := getTrackJSON(t, ts, ss, id)
	gain, _ := body["gain_db"].(float64)
	if gain != 4.0 {
		t.Errorf("gain_db = %v, want 4.0", gain)
	}
}

func TestIntegration_GainDB_CappedAtMaxGain(t *testing.T) {
	// Track at -40 LUFS, target -16 LUFS → raw gain = 24 dB, capped at 12.
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)
	ss := store.NewSettingsStore(db)

	id := insertTrack(t, ts, store.Track{
		Path: "/music/c.mp3", Title: "C", Type: "MUSIC", DurationMS: 200000,
	})
	setLoudness(t, ts, id, -40.0, -5.0)

	body := getTrackJSON(t, ts, ss, id)
	gain, _ := body["gain_db"].(float64)
	if gain != 12.0 {
		t.Errorf("gain_db = %v, want 12.0 (capped at max_gain_db)", gain)
	}
}

func TestIntegration_GainDB_NormalizationDisabled(t *testing.T) {
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)
	ss := store.NewSettingsStore(db)

	id := insertTrack(t, ts, store.Track{
		Path: "/music/d.mp3", Title: "D", Type: "MUSIC", DurationMS: 200000,
	})
	setLoudness(t, ts, id, -20.0, -1.0)

	// Disable normalization.
	if err := ss.Set(context.Background(), "normalization.enabled", "false"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	body := getTrackJSON(t, ts, ss, id)
	if body["gain_db"] != 0.0 {
		t.Errorf("gain_db = %v, want 0.0 (normalization disabled)", body["gain_db"])
	}
}

func TestIntegration_GainDB_PerTypeTarget(t *testing.T) {
	// With per_type_enabled, SPOT should use target_lufs_spot (-14 by default).
	// SPOT at -20 LUFS → gain = -14 - (-20) = 6 dB.
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)
	ss := store.NewSettingsStore(db)

	if err := ss.Set(context.Background(), "normalization.per_type_enabled", "true"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	id := insertTrack(t, ts, store.Track{
		Path: "/spots/e.mp3", Title: "E", Type: "SPOT", DurationMS: 30000,
	})
	setLoudness(t, ts, id, -20.0, -1.0)

	body := getTrackJSON(t, ts, ss, id)
	gain, _ := body["gain_db"].(float64)
	if gain != 6.0 {
		t.Errorf("gain_db = %v, want 6.0 (SPOT target -14, measured -20)", gain)
	}
}

// ─── break engine-payload ─────────────────────────────────────────────────────

func TestIntegration_BreakEnginePayload_GainDB(t *testing.T) {
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)
	bs := store.NewBreakStore(db)
	ss := store.NewSettingsStore(db)

	// Jingle tracks for open/close.
	openID := insertTrack(t, ts, store.Track{
		Path: "/jingles/open.mp3", Title: "Open", Type: "JINGLE", DurationMS: 5000,
	})
	setLoudness(t, ts, openID, -18.0, -1.0) // gain = -16 - (-18) = +2

	spotID := insertTrack(t, ts, store.Track{
		Path: "/spots/spot1.mp3", Title: "Spot1", Type: "SPOT", DurationMS: 30000,
	})
	setLoudness(t, ts, spotID, -22.0, -1.0) // gain = -16 - (-22) = +6

	brk, err := bs.Create(context.Background(), "Break Comercial", openID, "")
	if err != nil {
		t.Fatalf("create break: %v", err)
	}
	if _, err := bs.AddItem(context.Background(), brk.ID, spotID); err != nil {
		t.Fatalf("add item: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/breaks/"+brk.ID+"?format=engine-payload", nil)
	req.SetPathValue("id", brk.ID)
	rec := httptest.NewRecorder()
	handlers.GetBreak(bs, ss)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}

	open := payload["open"].(map[string]any)
	if openGain, _ := open["gain_db"].(float64); openGain != 2.0 {
		t.Errorf("open gain_db = %v, want 2.0", openGain)
	}

	spots := payload["spots"].([]any)
	spot := spots[0].(map[string]any)
	if spotGain, _ := spot["gain_db"].(float64); spotGain != 6.0 {
		t.Errorf("spot gain_db = %v, want 6.0", spotGain)
	}
}

// ─── loudness filters ─────────────────────────────────────────────────────────

func searchTracks(t *testing.T, ts *store.TrackStore, ss *store.SettingsStore, query string) []any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/tracks?"+query, nil)
	rec := httptest.NewRecorder()
	handlers.SearchTracks(ts, ss)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("SearchTracks → %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body["tracks"].([]any)
}

func TestIntegration_LoudnessStatusFilter(t *testing.T) {
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)
	ss := store.NewSettingsStore(db)

	idA := insertTrack(t, ts, store.Track{Path: "/a.mp3", Title: "A", Type: "MUSIC", DurationMS: 1})
	idB := insertTrack(t, ts, store.Track{Path: "/b.mp3", Title: "B", Type: "MUSIC", DurationMS: 1})
	setLoudness(t, ts, idA, -16.0, -1.0) // status = done

	_ = idB // stays pending

	done := searchTracks(t, ts, ss, "loudness_status=done")
	if len(done) != 1 {
		t.Errorf("loudness_status=done: got %d, want 1", len(done))
	}

	pending := searchTracks(t, ts, ss, "loudness_status=pending")
	if len(pending) != 1 {
		t.Errorf("loudness_status=pending: got %d, want 1", len(pending))
	}
}

func TestIntegration_LoudnessRangeFilter(t *testing.T) {
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)
	ss := store.NewSettingsStore(db)

	idA := insertTrack(t, ts, store.Track{Path: "/a.mp3", Title: "A", Type: "MUSIC", DurationMS: 1})
	idB := insertTrack(t, ts, store.Track{Path: "/b.mp3", Title: "B", Type: "MUSIC", DurationMS: 1})
	idC := insertTrack(t, ts, store.Track{Path: "/c.mp3", Title: "C", Type: "MUSIC", DurationMS: 1})
	setLoudness(t, ts, idA, -14.0, -1.0)
	setLoudness(t, ts, idB, -18.0, -1.0)
	setLoudness(t, ts, idC, -22.0, -1.0)

	// Filter: louder than -20 LUFS (idA and idB).
	tracks := searchTracks(t, ts, ss, "loudness_min=-20")
	if len(tracks) != 2 {
		t.Errorf("loudness_min=-20: got %d, want 2", len(tracks))
	}

	// Filter: quieter than -17 LUFS (idB and idC).
	tracks = searchTracks(t, ts, ss, "loudness_max=-17")
	if len(tracks) != 2 {
		t.Errorf("loudness_max=-17: got %d, want 2", len(tracks))
	}

	// Range: between -20 and -15 (only idB at -18).
	tracks = searchTracks(t, ts, ss, "loudness_min=-20&loudness_max=-15")
	if len(tracks) != 1 {
		t.Errorf("range filter: got %d, want 1", len(tracks))
	}
}

// ─── loudness worker endpoints ────────────────────────────────────────────────

func TestIntegration_LoudnessStatus_Endpoint(t *testing.T) {
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)

	idA := insertTrack(t, ts, store.Track{Path: "/a.mp3", Title: "A", Type: "MUSIC", DurationMS: 1})
	idB := insertTrack(t, ts, store.Track{Path: "/b.mp3", Title: "B", Type: "MUSIC", DurationMS: 1})
	setLoudness(t, ts, idA, -16.0, -1.0) // status → done
	_ = idB                               // stays pending

	lw := &fakeIntegrationWorker{ts: ts}
	req := httptest.NewRequest(http.MethodGet, "/v1/loudness/status", nil)
	rec := httptest.NewRecorder()
	handlers.GetLoudnessStatus(lw)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	counts := body["counts"].(map[string]any)
	if counts["done"].(float64) != 1 {
		t.Errorf("done = %v, want 1", counts["done"])
	}
	if counts["pending"].(float64) != 1 {
		t.Errorf("pending = %v, want 1", counts["pending"])
	}
}

func TestIntegration_ReanalyzeAll_EnqueuesPending(t *testing.T) {
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)

	insertTrack(t, ts, store.Track{Path: "/a.mp3", Title: "A", Type: "MUSIC", DurationMS: 1})
	insertTrack(t, ts, store.Track{Path: "/b.mp3", Title: "B", Type: "MUSIC", DurationMS: 1})

	lw := &fakeIntegrationWorker{ts: ts}
	req := httptest.NewRequest(http.MethodPost, "/v1/loudness/analyze", nil)
	rec := httptest.NewRecorder()
	handlers.ReanalyzeAll(lw, ts)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(lw.enqueued) != 2 {
		t.Errorf("enqueued %d, want 2", len(lw.enqueued))
	}

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["enqueued"].(float64) != 2 {
		t.Errorf("enqueued in response = %v, want 2", body["enqueued"])
	}
}

func TestIntegration_ReanalyzeTrack_KnownID(t *testing.T) {
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)

	id := insertTrack(t, ts, store.Track{Path: "/a.mp3", Title: "A", Type: "MUSIC", DurationMS: 1})

	lw := &fakeIntegrationWorker{ts: ts}
	req := httptest.NewRequest(http.MethodPost, "/v1/loudness/analyze/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	handlers.ReanalyzeTrack(lw, ts)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(lw.enqueued) != 1 || lw.enqueued[0] != id {
		t.Errorf("enqueued = %v, want [%s]", lw.enqueued, id)
	}
}

func TestIntegration_CancelLoudness_DrainAndRespond(t *testing.T) {
	db := openIntegrationDB(t)
	ts := store.NewTrackStore(db)
	lw := &fakeIntegrationWorker{ts: ts, enqueued: []string{"x", "y"}}

	req := httptest.NewRequest(http.MethodDelete, "/v1/loudness/analyze", nil)
	rec := httptest.NewRecorder()
	handlers.CancelLoudness(lw)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(lw.enqueued) != 0 {
		t.Errorf("queue not drained: %v", lw.enqueued)
	}
	if !strings.Contains(rec.Body.String(), `"cancelled":true`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}
