package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// ── Stubs ─────────────────────────────────────────────────────────────────────

// stubLineInBus accepts any command and immediately replies with Accepted=true
// (or false when reject=true).
type stubLineInBus struct {
	reject bool
	sent   []commands.Command
}

func (b *stubLineInBus) Send(_ context.Context, cmd commands.Command) error {
	b.sent = append(b.sent, cmd)
	if cmd.Reply != nil {
		accepted := !b.reject
		cmd.Reply <- commands.Result{CommandID: cmd.ID, Accepted: accepted, Reason: ""}
	}
	return nil
}

// stubLineInState implements lineInStateReader via state.Manager for convenience.
type stubLineInState struct {
	snap state.Snapshot
}

func (s *stubLineInState) Snapshot() state.Snapshot { return s.snap }

// stubLineInConfigStore implements LineInConfigStore.
type stubLineInConfigStore struct {
	deviceID string
	saveErr  error
}

func (s *stubLineInConfigStore) GetLineInDefaultDeviceID() string { return s.deviceID }
func (s *stubLineInConfigStore) SetLineInDefaultDeviceID(id string) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.deviceID = id
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func snapWithState(s state.PlayerState) state.Snapshot {
	return state.Snapshot{State: s}
}

func snapLineInActive() state.Snapshot {
	li := state.LineInStatus{
		DeviceID:    "0",
		Label:       "Voz do Brasil",
		StartedAt:   time.Now().UTC(),
		DurationMS:  3_600_000,
		TriggeredBy: "manual",
	}
	return state.Snapshot{State: state.StateLineIn, LineIn: &li}
}

func postBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

func decodeBody(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return m
}

// ── POST /v1/linein/start ─────────────────────────────────────────────────────

func TestLineInStart_Success(t *testing.T) {
	bus := &stubLineInBus{}
	st := &stubLineInState{snap: snapWithState(state.StateIdle)}
	h := handlers.LineInStart(bus, st)

	body := postBody(map[string]any{"device_id": "default", "label": "Test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/linein/start", body)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	if m["ok"] != true {
		t.Errorf("ok = %v, want true", m["ok"])
	}
	if len(bus.sent) != 1 {
		t.Errorf("expected 1 command sent, got %d", len(bus.sent))
	}
	if bus.sent[0].Type != commands.CmdLineInStart {
		t.Errorf("command type = %v, want CmdLineInStart", bus.sent[0].Type)
	}
}

func TestLineInStart_InvalidBody(t *testing.T) {
	bus := &stubLineInBus{}
	st := &stubLineInState{snap: snapWithState(state.StateIdle)}
	h := handlers.LineInStart(bus, st)

	req := httptest.NewRequest(http.MethodPost, "/v1/linein/start", bytes.NewBufferString("{bad json"))
	req.ContentLength = int64(len("{bad json"))
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	if m["error"] != "invalid_body" {
		t.Errorf("error = %v, want invalid_body", m["error"])
	}
}

func TestLineInStart_AlreadyActive(t *testing.T) {
	bus := &stubLineInBus{}
	st := &stubLineInState{snap: snapLineInActive()}
	h := handlers.LineInStart(bus, st)

	req := httptest.NewRequest(http.MethodPost, "/v1/linein/start", nil)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	if m["error"] != "linein_already_active" {
		t.Errorf("error = %v, want linein_already_active", m["error"])
	}
}

func TestLineInStart_EnginePanic(t *testing.T) {
	bus := &stubLineInBus{}
	st := &stubLineInState{snap: snapWithState(state.StatePanic)}
	h := handlers.LineInStart(bus, st)

	req := httptest.NewRequest(http.MethodPost, "/v1/linein/start", nil)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	if m["error"] != "engine_in_panic" {
		t.Errorf("error = %v, want engine_in_panic", m["error"])
	}
}

func TestLineInStart_DefaultDeviceID(t *testing.T) {
	bus := &stubLineInBus{}
	st := &stubLineInState{snap: snapWithState(state.StateIdle)}
	h := handlers.LineInStart(bus, st)

	// No device_id in body — must default to "default"
	req := httptest.NewRequest(http.MethodPost, "/v1/linein/start", postBody(map[string]any{"label": "Test"}))
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if len(bus.sent) == 0 {
		t.Fatal("no command sent")
	}
	p := bus.sent[0].Payload.(commands.LineInStartPayload)
	if p.DeviceID != "default" {
		t.Errorf("DeviceID = %q, want %q", p.DeviceID, "default")
	}
}

// ── POST /v1/linein/stop ──────────────────────────────────────────────────────

func TestLineInStop_Success(t *testing.T) {
	bus := &stubLineInBus{}
	st := &stubLineInState{snap: snapLineInActive()}
	h := handlers.LineInStop(bus, st)

	req := httptest.NewRequest(http.MethodPost, "/v1/linein/stop", postBody(map[string]any{"reason": "end"}))
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	data := m["data"].(map[string]any)
	if data["active"] != false {
		t.Errorf("active = %v, want false", data["active"])
	}
}

func TestLineInStop_NotActive(t *testing.T) {
	bus := &stubLineInBus{}
	st := &stubLineInState{snap: snapWithState(state.StatePlaying)}
	h := handlers.LineInStop(bus, st)

	req := httptest.NewRequest(http.MethodPost, "/v1/linein/stop", nil)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	if m["error"] != "linein_not_active" {
		t.Errorf("error = %v, want linein_not_active", m["error"])
	}
}

// ── GET /v1/linein/status ─────────────────────────────────────────────────────

func TestLineInStatus_Active(t *testing.T) {
	st := &stubLineInState{snap: snapLineInActive()}
	h := handlers.LineInStatus(st)

	req := httptest.NewRequest(http.MethodGet, "/v1/linein/status", nil)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	data := m["data"].(map[string]any)
	if data["active"] != true {
		t.Errorf("active = %v, want true", data["active"])
	}
	if data["label"] != "Voz do Brasil" {
		t.Errorf("label = %v, want Voz do Brasil", data["label"])
	}
}

func TestLineInStatus_Inactive(t *testing.T) {
	st := &stubLineInState{snap: snapWithState(state.StateIdle)}
	h := handlers.LineInStatus(st)

	req := httptest.NewRequest(http.MethodGet, "/v1/linein/status", nil)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	data := m["data"].(map[string]any)
	if data["active"] != false {
		t.Errorf("active = %v, want false", data["active"])
	}
}

// ── GET /v1/linein/devices ────────────────────────────────────────────────────

func TestLineInDevices_List(t *testing.T) {
	list := func() ([]handlers.LineInDevice, error) {
		return []handlers.LineInDevice{
			{ID: "0", Name: "Built-in Microphone", Driver: "avfoundation"},
			{ID: "1", Name: "USB Audio CODEC", Driver: "avfoundation"},
		}, nil
	}
	h := handlers.LineInDevices(list)

	req := httptest.NewRequest(http.MethodGet, "/v1/linein/devices", nil)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	data := m["data"].(map[string]any)
	devs := data["devices"].([]any)
	if len(devs) != 2 {
		t.Errorf("devices count = %d, want 2", len(devs))
	}
}

func TestLineInDevices_Empty(t *testing.T) {
	h := handlers.LineInDevices(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/linein/devices", nil)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	data := m["data"].(map[string]any)
	devs := data["devices"].([]any)
	if len(devs) != 0 {
		t.Errorf("devices count = %d, want 0", len(devs))
	}
}

func TestLineInDevices_EnumerationError(t *testing.T) {
	list := func() ([]handlers.LineInDevice, error) {
		return nil, errors.New("ffmpeg not found")
	}
	h := handlers.LineInDevices(list)

	req := httptest.NewRequest(http.MethodGet, "/v1/linein/devices", nil)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	m := decodeBody(t, rec.Body)
	if m["error"] != "device_enumeration_failed" {
		t.Errorf("error = %v, want device_enumeration_failed", m["error"])
	}
}

// ── PATCH /v1/linein/config ───────────────────────────────────────────────────

func TestLineInPatchConfig_Success(t *testing.T) {
	store := &stubLineInConfigStore{}
	h := handlers.LineInPatchConfig(store)

	body := postBody(map[string]any{"default_device_id": ":1"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/linein/config", body)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if store.deviceID != ":1" {
		t.Errorf("saved deviceID = %q, want %q", store.deviceID, ":1")
	}
}

func TestLineInPatchConfig_MissingField(t *testing.T) {
	store := &stubLineInConfigStore{}
	h := handlers.LineInPatchConfig(store)

	body := postBody(map[string]any{})
	req := httptest.NewRequest(http.MethodPatch, "/v1/linein/config", body)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestLineInPatchConfig_SaveError(t *testing.T) {
	store := &stubLineInConfigStore{saveErr: errors.New("disk full")}
	h := handlers.LineInPatchConfig(store)

	body := postBody(map[string]any{"default_device_id": ":1"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/linein/config", body)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestLineInPatchConfig_StoreNil(t *testing.T) {
	h := handlers.LineInPatchConfig(nil)

	body := postBody(map[string]any{"default_device_id": ":1"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/linein/config", body)
	rec := httptest.NewRecorder()

	h(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}
