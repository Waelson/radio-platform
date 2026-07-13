package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/api/handlers"
	"github.com/Waelson/radio-library-service/internal/store"
)

// ── Fake ──────────────────────────────────────────────────────────────────────

type fakeSRW struct {
	data map[string]string
}

func newFakeSRW(pairs ...string) *fakeSRW {
	m := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return &fakeSRW{data: m}
}

func (f *fakeSRW) Get(_ context.Context, key string) (string, error) {
	v, ok := f.data[key]
	if !ok {
		return "", store.ErrNotFound
	}
	return v, nil
}

func (f *fakeSRW) Set(_ context.Context, key, value string) error {
	f.data[key] = value
	return nil
}

func (f *fakeSRW) List(_ context.Context) ([]store.SettingRow, error) {
	rows := make([]store.SettingRow, 0, len(f.data))
	for k, v := range f.data {
		rows = append(rows, store.SettingRow{Key: k, Value: v, UpdatedAt: time.Now()})
	}
	return rows, nil
}

// ── ListSettings ──────────────────────────────────────────────────────────────

func TestListSettings_OK(t *testing.T) {
	srw := newFakeSRW(
		"station.name", "Radio Test FM",
		"transmission_log.retention_days", "30",
	)
	w := doRequest(handlers.ListSettings(srw), "GET", "/v1/settings")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeBody(t, w)
	if body["ok"] != true {
		t.Errorf("ok = %v, want true", body["ok"])
	}
	data := body["data"].([]any)
	if len(data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(data))
	}
}

func TestListSettings_Empty(t *testing.T) {
	srw := newFakeSRW()
	w := doRequest(handlers.ListSettings(srw), "GET", "/v1/settings")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeBody(t, w)
	data := body["data"].([]any)
	if len(data) != 0 {
		t.Errorf("expected empty list, got %d entries", len(data))
	}
}

// ── GetSetting ────────────────────────────────────────────────────────────────

func TestGetSetting_Found(t *testing.T) {
	srw := newFakeSRW("station.name", "Radio FM")

	req := makeRequest("GET", "/v1/settings/station.name")
	req.SetPathValue("key", "station.name")
	w := recordRequest(handlers.GetSetting(srw), req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeBody(t, w)
	data := body["data"].(map[string]any)
	if data["value"] != "Radio FM" {
		t.Errorf("value = %v, want Radio FM", data["value"])
	}
	if data["key"] != "station.name" {
		t.Errorf("key = %v, want station.name", data["key"])
	}
}

func TestGetSetting_NotFound(t *testing.T) {
	srw := newFakeSRW()

	req := makeRequest("GET", "/v1/settings/nonexistent.key")
	req.SetPathValue("key", "nonexistent.key")
	w := recordRequest(handlers.GetSetting(srw), req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ── UpdateSetting ─────────────────────────────────────────────────────────────

func TestUpdateSetting_OK(t *testing.T) {
	srw := newFakeSRW("station.name", "Old Name")

	req := makeRequestWithBody("PUT", "/v1/settings/station.name", `{"value":"New Name"}`)
	req.SetPathValue("key", "station.name")
	w := recordRequest(handlers.UpdateSetting(srw), req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["ok"] != true {
		t.Errorf("ok = %v, want true", body["ok"])
	}
	// Verify the store was updated.
	v, _ := srw.Get(context.Background(), "station.name")
	if v != "New Name" {
		t.Errorf("stored value = %q, want New Name", v)
	}
}

func TestUpdateSetting_NotFound(t *testing.T) {
	srw := newFakeSRW()

	req := makeRequestWithBody("PUT", "/v1/settings/missing.key", `{"value":"x"}`)
	req.SetPathValue("key", "missing.key")
	w := recordRequest(handlers.UpdateSetting(srw), req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestUpdateSetting_RetentionDaysBelowMin(t *testing.T) {
	srw := newFakeSRW("transmission_log.retention_days", "30")

	req := makeRequestWithBody("PUT", "/v1/settings/transmission_log.retention_days", `{"value":"3"}`)
	req.SetPathValue("key", "transmission_log.retention_days")
	w := recordRequest(handlers.UpdateSetting(srw), req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (retention < 7)", w.Code)
	}
	body := decodeBody(t, w)
	msg := body["message"].(string)
	if !strings.Contains(msg, "7") {
		t.Errorf("error message should mention minimum 7, got %q", msg)
	}
}

func TestUpdateSetting_RetentionDaysNotANumber(t *testing.T) {
	srw := newFakeSRW("transmission_log.retention_days", "30")

	req := makeRequestWithBody("PUT", "/v1/settings/transmission_log.retention_days", `{"value":"abc"}`)
	req.SetPathValue("key", "transmission_log.retention_days")
	w := recordRequest(handlers.UpdateSetting(srw), req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (non-integer value)", w.Code)
	}
}

func TestUpdateSetting_RetentionDaysExactlyMin(t *testing.T) {
	srw := newFakeSRW("transmission_log.retention_days", "30")

	req := makeRequestWithBody("PUT", "/v1/settings/transmission_log.retention_days", `{"value":"7"}`)
	req.SetPathValue("key", "transmission_log.retention_days")
	w := recordRequest(handlers.UpdateSetting(srw), req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (7 is the allowed minimum)", w.Code)
	}
}

func TestUpdateSetting_BadJSON(t *testing.T) {
	srw := newFakeSRW("station.name", "x")

	req := makeRequestWithBody("PUT", "/v1/settings/station.name", `not json`)
	req.SetPathValue("key", "station.name")
	w := recordRequest(handlers.UpdateSetting(srw), req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ── Local helpers (path values need to be set manually in tests) ──────────────

func makeRequest(method, path string) *http.Request {
	return makeRequestWithBody(method, path, "")
}

func makeRequestWithBody(method, path, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r, _ = http.NewRequest(method, path, nil)
	} else {
		r, _ = http.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func recordRequest(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
