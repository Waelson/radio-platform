package handlers

import "net/http"

// AudioDevice is the API DTO for a single audio output device.
//
// The ID field semantics depend on the active driver:
//   - coreaudio: persistent UID (kAudioDevicePropertyDeviceUID)
//   - portaudio: equal to Name (PortAudio has no stable UID)
//   - null / file: fixed strings "null" / "file"
type AudioDevice struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Driver            string  `json:"driver"`
	IsDefault         bool    `json:"is_default"`
	MaxOutputChannels int     `json:"max_output_channels"`
	DefaultSampleRate float64 `json:"default_sample_rate"`
}

type devicesResponse struct {
	Devices []AudioDevice `json:"devices"`
	Count   int           `json:"count"`
}

// Devices returns a handler for GET /v1/devices.
//
// list is called on every request — no caching — so the response always
// reflects the current state of the system's audio devices.
// If list is nil (driver does not support enumeration), an empty list is returned.
func Devices(list func() ([]AudioDevice, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		if list == nil {
			writeJSON(w, http.StatusOK, devicesResponse{Devices: []AudioDevice{}, Count: 0})
			return
		}

		devs, err := list()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "device_enumeration_failed", err.Error())
			return
		}

		if devs == nil {
			devs = []AudioDevice{}
		}
		writeJSON(w, http.StatusOK, devicesResponse{Devices: devs, Count: len(devs)})
	}
}
