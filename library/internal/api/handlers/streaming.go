package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// StreamingStore is the store subset required by the streaming handlers.
type StreamingStore interface {
	List(ctx context.Context) ([]store.StreamingTarget, error)
	Get(ctx context.Context, id string) (store.StreamingTarget, error)
	Create(ctx context.Context, in store.StreamingTargetInput) (store.StreamingTarget, error)
	Update(ctx context.Context, id string, in store.StreamingTargetInput) (store.StreamingTarget, error)
	Delete(ctx context.Context, id string) error
}

// ── JSON shapes ───────────────────────────────────────────────────────────────

type reconnectConfigJSON struct {
	Enabled           bool    `json:"enabled"`
	MaxRetries        int     `json:"max_retries"`
	InitialDelaySec   int     `json:"initial_delay_sec"`
	MaxDelaySec       int     `json:"max_delay_sec"`
	BackoffMultiplier float64 `json:"backoff_multiplier"`
}

// streamingTargetJSON is the public representation — password is omitted.
type streamingTargetJSON struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Enabled            bool                `json:"enabled"`
	Type               string              `json:"type"`
	Host               string              `json:"host"`
	Port               int                 `json:"port"`
	Mount              string              `json:"mount"`
	Format             string              `json:"format"`
	BitrateKbps        int                 `json:"bitrate_kbps"`
	SampleRate         int                 `json:"sample_rate"`
	Channels           int                 `json:"channels"`
	SendMetadata       bool                `json:"send_metadata"`
	StationName        string              `json:"station_name"`
	StationDescription string              `json:"station_description"`
	StationGenre       string              `json:"station_genre"`
	StationURL         string              `json:"station_url"`
	Reconnect          reconnectConfigJSON `json:"reconnect"`
	AutoConnect        bool                `json:"auto_connect"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// streamingTargetWithPasswordJSON includes the password — only used internally
// when the Player requests the full config to forward to the Playout Engine.
type streamingTargetWithPasswordJSON struct {
	streamingTargetJSON
	Password string `json:"password"`
}

type streamingInputJSON struct {
	Name                       string   `json:"name"`
	Enabled                    *bool    `json:"enabled"`
	Type                       string   `json:"type"`
	Host                       string   `json:"host"`
	Port                       int      `json:"port"`
	Mount                      string   `json:"mount"`
	Password                   string   `json:"password"`
	Format                     string   `json:"format"`
	BitrateKbps                int      `json:"bitrate_kbps"`
	SampleRate                 int      `json:"sample_rate"`
	Channels                   int      `json:"channels"`
	SendMetadata               *bool    `json:"send_metadata"`
	StationName                string   `json:"station_name"`
	StationDescription         string   `json:"station_description"`
	StationGenre               string   `json:"station_genre"`
	StationURL                 string   `json:"station_url"`
	ReconnectEnabled           *bool    `json:"reconnect_enabled"`
	ReconnectMaxRetries        int      `json:"reconnect_max_retries"`
	ReconnectInitialDelaySec   int      `json:"reconnect_initial_delay_sec"`
	ReconnectMaxDelaySec       int      `json:"reconnect_max_delay_sec"`
	ReconnectBackoffMultiplier float64  `json:"reconnect_backoff_multiplier"`
	AutoConnect                *bool    `json:"auto_connect"`
}

// ── converters ────────────────────────────────────────────────────────────────

func toStreamingJSON(t store.StreamingTarget) streamingTargetJSON {
	return streamingTargetJSON{
		ID:                 t.ID,
		Name:               t.Name,
		Enabled:            t.Enabled,
		Type:               t.Type,
		Host:               t.Host,
		Port:               t.Port,
		Mount:              t.Mount,
		Format:             t.Format,
		BitrateKbps:        t.BitrateKbps,
		SampleRate:         t.SampleRate,
		Channels:           t.Channels,
		SendMetadata:       t.SendMetadata,
		StationName:        t.StationName,
		StationDescription: t.StationDescription,
		StationGenre:       t.StationGenre,
		StationURL:         t.StationURL,
		Reconnect: reconnectConfigJSON{
			Enabled:           t.ReconnectEnabled,
			MaxRetries:        t.ReconnectMaxRetries,
			InitialDelaySec:   t.ReconnectInitialDelaySec,
			MaxDelaySec:       t.ReconnectMaxDelaySec,
			BackoffMultiplier: t.ReconnectBackoffMultiplier,
		},
		AutoConnect: t.AutoConnect,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func toStoreInput(in streamingInputJSON) store.StreamingTargetInput {
	return store.StreamingTargetInput{
		Name:                       in.Name,
		Enabled:                    in.Enabled,
		Type:                       in.Type,
		Host:                       in.Host,
		Port:                       in.Port,
		Mount:                      in.Mount,
		Password:                   in.Password,
		Format:                     in.Format,
		BitrateKbps:                in.BitrateKbps,
		SampleRate:                 in.SampleRate,
		Channels:                   in.Channels,
		SendMetadata:               in.SendMetadata,
		StationName:                in.StationName,
		StationDescription:         in.StationDescription,
		StationGenre:               in.StationGenre,
		StationURL:                 in.StationURL,
		ReconnectEnabled:           in.ReconnectEnabled,
		ReconnectMaxRetries:        in.ReconnectMaxRetries,
		ReconnectInitialDelaySec:   in.ReconnectInitialDelaySec,
		ReconnectMaxDelaySec:       in.ReconnectMaxDelaySec,
		ReconnectBackoffMultiplier: in.ReconnectBackoffMultiplier,
		AutoConnect:                in.AutoConnect,
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

// ListStreamingTargets handles GET /v1/streaming
func ListStreamingTargets(s StreamingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targets, err := s.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		out := make([]streamingTargetJSON, len(targets))
		for i, t := range targets {
			out[i] = toStreamingJSON(t)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": out})
	}
}

// GetStreamingTarget handles GET /v1/streaming/{id}
// Includes password — intended for the Player when forwarding the full config
// to the Playout Engine at connect time.
func GetStreamingTarget(s StreamingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		t, err := s.Get(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "streaming target not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		out := streamingTargetWithPasswordJSON{
			streamingTargetJSON: toStreamingJSON(t),
			Password:            t.Password,
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": out})
	}
}

// CreateStreamingTarget handles POST /v1/streaming
func CreateStreamingTarget(s StreamingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in streamingInputJSON
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
			return
		}
		t, err := s.Create(r.Context(), toStoreInput(in))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": toStreamingJSON(t)})
	}
}

// UpdateStreamingTarget handles PUT /v1/streaming/{id}
func UpdateStreamingTarget(s StreamingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var in streamingInputJSON
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
			return
		}
		t, err := s.Update(r.Context(), id, toStoreInput(in))
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "streaming target not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": toStreamingJSON(t)})
	}
}

// DeleteStreamingTarget handles DELETE /v1/streaming/{id}
func DeleteStreamingTarget(s StreamingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := s.Delete(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
