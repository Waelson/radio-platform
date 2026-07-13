package handlers_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
	"github.com/Waelson/radio-library-service/internal/store"
)

// ── Fakes ─────────────────────────────────────────────────────────────────────

type fakeTLS struct {
	entries []store.TransmissionLogEntry
	total   int
	summary store.TransmissionLogSummary
	csvData string
	ecadData string
}

func (f *fakeTLS) List(_ context.Context, _ store.TransmissionLogQuery) ([]store.TransmissionLogEntry, int, error) {
	return f.entries, f.total, nil
}
func (f *fakeTLS) Summary(_ context.Context, _ time.Time) (store.TransmissionLogSummary, error) {
	return f.summary, nil
}
func (f *fakeTLS) ExportCSV(_ context.Context, _, _ time.Time, w io.Writer) error {
	_, _ = io.WriteString(w, f.csvData)
	return nil
}
func (f *fakeTLS) ExportECAD(_ context.Context, _, _ time.Time, _ store.StationInfo, w io.Writer) error {
	_, _ = io.WriteString(w, f.ecadData)
	return nil
}

type fakeILS struct {
	entries []store.TransmissionImportLogEntry
}

func (f *fakeILS) List(_ context.Context, _, _ int) ([]store.TransmissionImportLogEntry, error) {
	return f.entries, nil
}

type fakeSTG struct {
	info store.StationInfo
}

func (f *fakeSTG) StationInfo(_ context.Context) store.StationInfo { return f.info }

// ── Helpers ───────────────────────────────────────────────────────────────────

func doRequest(h http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// ── ListTransmissionLog ───────────────────────────────────────────────────────

func TestListTransmissionLog_OK(t *testing.T) {
	now := time.Now().UTC()
	finished := now.Add(4 * time.Minute)
	tls := &fakeTLS{
		entries: []store.TransmissionLogEntry{
			{
				ID: "id-1", QueueItemID: "qi-1", Title: "Track A", Artist: "Artist X",
				Type: "MUSIC", DurationMS: 240000, DurationPlayedMS: 240000,
				Result: "finished", Status: "FINISHED",
				StartedAt: now, FinishedAt: &finished,
			},
		},
		total: 1,
	}

	w := doRequest(handlers.ListTransmissionLog(tls), "GET", "/v1/transmission-log")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeBody(t, w)
	if body["ok"] != true {
		t.Errorf("ok = %v, want true", body["ok"])
	}
	data := body["data"].(map[string]any)
	entries := data["entries"].([]any)
	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want 1", len(entries))
	}
	if data["total"].(float64) != 1 {
		t.Errorf("total = %v, want 1", data["total"])
	}
}

func TestListTransmissionLog_InvalidFrom(t *testing.T) {
	tls := &fakeTLS{}
	w := doRequest(handlers.ListTransmissionLog(tls), "GET", "/v1/transmission-log?from=not-a-date")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestListTransmissionLog_InvalidTo(t *testing.T) {
	tls := &fakeTLS{}
	w := doRequest(handlers.ListTransmissionLog(tls), "GET", "/v1/transmission-log?to=bad")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestListTransmissionLog_LimitCappedAt500(t *testing.T) {
	tls := &fakeTLS{entries: nil, total: 0}
	w := doRequest(handlers.ListTransmissionLog(tls), "GET", "/v1/transmission-log?limit=9999")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeBody(t, w)
	data := body["data"].(map[string]any)
	if data["limit"].(float64) > 500 {
		t.Errorf("limit = %v, should be capped at 500", data["limit"])
	}
}

// ── GetTransmissionLogSummary ─────────────────────────────────────────────────

func TestGetTransmissionLogSummary_OK(t *testing.T) {
	tls := &fakeTLS{
		summary: store.TransmissionLogSummary{
			Date:  "2026-07-20",
			Total: 10,
			ByType: map[string]int{
				"MUSIC": 7, "SPOT": 3,
			},
			TotalPlayedMS: 1800000,
		},
	}

	w := doRequest(handlers.GetTransmissionLogSummary(tls), "GET",
		"/v1/transmission-log/summary?date=2026-07-20")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeBody(t, w)
	data := body["data"].(map[string]any)
	if data["total"].(float64) != 10 {
		t.Errorf("total = %v, want 10", data["total"])
	}
	byType := data["by_type"].(map[string]any)
	if byType["MUSIC"].(float64) != 7 {
		t.Errorf("MUSIC = %v, want 7", byType["MUSIC"])
	}
}

func TestGetTransmissionLogSummary_InvalidDate(t *testing.T) {
	tls := &fakeTLS{}
	w := doRequest(handlers.GetTransmissionLogSummary(tls), "GET",
		"/v1/transmission-log/summary?date=20-07-2026")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestGetTransmissionLogSummary_DefaultsToToday(t *testing.T) {
	tls := &fakeTLS{summary: store.TransmissionLogSummary{Date: "today", Total: 0, ByType: map[string]int{}}}
	w := doRequest(handlers.GetTransmissionLogSummary(tls), "GET", "/v1/transmission-log/summary")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when date omitted", w.Code)
	}
}

// ── ExportTransmissionLog (CSV) ───────────────────────────────────────────────

func TestExportTransmissionLog_StreamsCSV(t *testing.T) {
	tls := &fakeTLS{csvData: "started_at;title\n2026-07-20T08:00:00Z;Track A\n"}

	w := doRequest(handlers.ExportTransmissionLog(tls), "GET",
		"/v1/transmission-log/export?from=2026-07-20&to=2026-07-20")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", cd)
	}
	if !strings.Contains(w.Body.String(), "Track A") {
		t.Errorf("body missing Track A: %q", w.Body.String())
	}
}

func TestExportTransmissionLog_InvalidFrom(t *testing.T) {
	tls := &fakeTLS{}
	w := doRequest(handlers.ExportTransmissionLog(tls), "GET",
		"/v1/transmission-log/export?from=bad")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ── ExportECAD ────────────────────────────────────────────────────────────────

func TestExportECAD_StreamsECADFormat(t *testing.T) {
	tls := &fakeTLS{
		ecadData: "H;Radio Test FM;12.345.678/0001-90;São Paulo;SP;98.5 MHz;FM;01/07/2026;31/07/2026\n" +
			"D;20/07/2026;08:00:00;03:58;Track A;Artist X;Composer;ISRC;M;R\n",
	}
	stg := &fakeSTG{info: store.StationInfo{Name: "Radio Test FM", CNPJ: "12.345.678/0001-90"}}

	w := doRequest(handlers.ExportECAD(tls, stg), "GET",
		"/v1/transmission-log/export/ecad?from=2026-07-01&to=2026-07-31")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "ecad_") {
		t.Errorf("Content-Disposition = %q, want ecad_ filename", cd)
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "H;") {
		t.Errorf("body should start with H;, got %q", body[:min2(20, len(body))])
	}
}

func TestExportECAD_InvalidFrom(t *testing.T) {
	tls := &fakeTLS{}
	stg := &fakeSTG{}
	w := doRequest(handlers.ExportECAD(tls, stg), "GET",
		"/v1/transmission-log/export/ecad?from=bad")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ── ListImportLog ─────────────────────────────────────────────────────────────

func TestListImportLog_OK(t *testing.T) {
	now := time.Now().UTC()
	finished := now.Add(2 * time.Second)
	ils := &fakeILS{
		entries: []store.TransmissionImportLogEntry{
			{
				ID: "imp-1", FileName: "transmission_20260720_08.jsonl",
				StartedAt: now, FinishedAt: &finished,
				Status: "success", RecordsTotal: 10, RecordsImported: 10,
			},
		},
	}

	w := doRequest(handlers.ListImportLog(ils), "GET", "/v1/transmission-log/imports")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeBody(t, w)
	if body["ok"] != true {
		t.Errorf("ok = %v, want true", body["ok"])
	}
	data := body["data"].(map[string]any)
	entries := data["entries"].([]any)
	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want 1", len(entries))
	}
	e := entries[0].(map[string]any)
	if e["status"] != "success" {
		t.Errorf("status = %v, want success", e["status"])
	}
	if e["file_name"] != "transmission_20260720_08.jsonl" {
		t.Errorf("file_name = %v", e["file_name"])
	}
}

func TestListImportLog_EmptyList(t *testing.T) {
	ils := &fakeILS{entries: nil}
	w := doRequest(handlers.ListImportLog(ils), "GET", "/v1/transmission-log/imports")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeBody(t, w)
	data := body["data"].(map[string]any)
	// entries must be an array (not null) — but fakeILS returns nil → JSON null
	// The handler uses make([]..., 0) so it should be [].
	entries := data["entries"]
	if entries == nil {
		t.Error("entries should not be null in JSON response")
	}
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
