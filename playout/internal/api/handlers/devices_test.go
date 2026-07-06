package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
)

func TestDevices_PopulatedList(t *testing.T) {
	list := func() ([]handlers.AudioDevice, error) {
		return []handlers.AudioDevice{
			{
				ID:                "AppleHDAEngineOutput:0",
				Name:              "MacBook Pro Speakers",
				Driver:            "coreaudio",
				IsDefault:         true,
				MaxOutputChannels: 2,
				DefaultSampleRate: 48000,
			},
			{
				ID:                "BlackHole 2ch",
				Name:              "BlackHole 2ch",
				Driver:            "coreaudio",
				IsDefault:         false,
				MaxOutputChannels: 2,
				DefaultSampleRate: 44100,
			},
		}, nil
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	handlers.Devices(list).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	var body struct {
		Devices []handlers.AudioDevice `json:"devices"`
		Count   int                    `json:"count"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Count != 2 {
		t.Errorf("count = %d, want 2", body.Count)
	}
	if len(body.Devices) != 2 {
		t.Fatalf("len(devices) = %d, want 2", len(body.Devices))
	}

	d := body.Devices[0]
	if d.ID != "AppleHDAEngineOutput:0" {
		t.Errorf("devices[0].id = %q", d.ID)
	}
	if d.Driver != "coreaudio" {
		t.Errorf("devices[0].driver = %q", d.Driver)
	}
	if !d.IsDefault {
		t.Error("devices[0].is_default should be true")
	}
	if d.MaxOutputChannels != 2 {
		t.Errorf("devices[0].max_output_channels = %d, want 2", d.MaxOutputChannels)
	}
	if d.DefaultSampleRate != 48000 {
		t.Errorf("devices[0].default_sample_rate = %v, want 48000", d.DefaultSampleRate)
	}
}

func TestDevices_ListError_Returns500(t *testing.T) {
	list := func() ([]handlers.AudioDevice, error) {
		return nil, errors.New("hardware unavailable")
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	handlers.Devices(list).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}

	var body struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.OK {
		t.Error("ok should be false on error")
	}
	if body.Error != "device_enumeration_failed" {
		t.Errorf("error code = %q, want device_enumeration_failed", body.Error)
	}
	if body.Message == "" {
		t.Error("message should not be empty")
	}
}

func TestDevices_NilList_ReturnsEmpty(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	handlers.Devices(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body struct {
		Devices []handlers.AudioDevice `json:"devices"`
		Count   int                    `json:"count"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Count != 0 {
		t.Errorf("count = %d, want 0", body.Count)
	}
	if len(body.Devices) != 0 {
		t.Errorf("len(devices) = %d, want 0", len(body.Devices))
	}
}
