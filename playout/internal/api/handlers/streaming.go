package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/streaming"
)

// StreamingManager is the interface streaming handlers require from streaming.Manager.
// Using an interface keeps handlers independently testable without FFmpeg.
type StreamingManager interface {
	AddTarget(ctx context.Context, cfg streaming.TargetConfig) error
	RemoveTarget(id string)
	ListStatuses() []streaming.TargetStatus
	Status(id string) (streaming.TargetStatus, error)
}

// --- request / response DTOs -------------------------------------------------

type streamingConnectRequest struct {
	Name               string                   `json:"name"`
	Type               string                   `json:"type"`
	Host               string                   `json:"host"`
	Port               int                      `json:"port"`
	Mount              string                   `json:"mount,omitempty"`
	Password           string                   `json:"password"`
	Format             string                   `json:"format"`
	BitrateKbps        int                      `json:"bitrate_kbps,omitempty"`
	SampleRate         int                      `json:"sample_rate,omitempty"`
	Channels           int                      `json:"channels,omitempty"`
	SendMetadata       bool                     `json:"send_metadata"`
	StationName        string                   `json:"station_name,omitempty"`
	StationDescription string                   `json:"station_description,omitempty"`
	StationGenre       string                   `json:"station_genre,omitempty"`
	StationURL         string                   `json:"station_url,omitempty"`
	Reconnect          streamingReconnectConfig `json:"reconnect"`
}

type streamingReconnectConfig struct {
	Enabled           bool    `json:"enabled"`
	MaxRetries        int     `json:"max_retries,omitempty"`
	InitialDelaySec   int     `json:"initial_delay_sec,omitempty"`
	MaxDelaySec       int     `json:"max_delay_sec,omitempty"`
	BackoffMultiplier float64 `json:"backoff_multiplier,omitempty"`
}

type streamingTargetView struct {
	ID          string     `json:"id"`
	State       string     `json:"state"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	RetryCount  int        `json:"retry_count"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
	BytesSent   int64      `json:"bytes_sent"`
	UptimeMS    int64      `json:"uptime_ms"`
}

type streamingListResponse struct {
	Targets []streamingTargetView `json:"targets"`
}

type streamingTestRequest struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type streamingTestResponse struct {
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// --- handlers ----------------------------------------------------------------

// StreamingConnect handles POST /v1/streaming/{id}/connect.
// The full target config is received in the request body; the ID comes from
// the URL path. No config is stored on disk by the engine.
func StreamingConnect(mgr StreamingManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "missing target id in path")
			return
		}

		var req streamingConnectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON: "+err.Error())
			return
		}

		if req.Host == "" || req.Port <= 0 {
			writeError(w, http.StatusBadRequest, "bad_request", "host and port are required")
			return
		}

		if err := streaming.ValidateFormat(req.Format); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_format", err.Error())
			return
		}

		cfg := streaming.TargetConfig{
			ID:                 id,
			Name:               req.Name,
			Type:               req.Type,
			Host:               req.Host,
			Port:               req.Port,
			Mount:              req.Mount,
			Password:           req.Password,
			Format:             req.Format,
			BitrateKbps:        req.BitrateKbps,
			SampleRate:         req.SampleRate,
			Channels:           req.Channels,
			SendMetadata:       req.SendMetadata,
			StationName:        req.StationName,
			StationDescription: req.StationDescription,
			StationGenre:       req.StationGenre,
			StationURL:         req.StationURL,
			Reconnect: streaming.ReconnectConfig{
				Enabled:           req.Reconnect.Enabled,
				MaxRetries:        req.Reconnect.MaxRetries,
				InitialDelaySec:   req.Reconnect.InitialDelaySec,
				MaxDelaySec:       req.Reconnect.MaxDelaySec,
				BackoffMultiplier: req.Reconnect.BackoffMultiplier,
			},
		}

		if err := mgr.AddTarget(r.Context(), cfg); err != nil {
			writeError(w, http.StatusConflict, "connect_failed", err.Error())
			return
		}

		status, _ := mgr.Status(id)
		writeJSON(w, http.StatusCreated, targetToView(status))
	}
}

// StreamingDisconnect handles POST /v1/streaming/{id}/disconnect.
func StreamingDisconnect(mgr StreamingManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "missing target id in path")
			return
		}
		mgr.RemoveTarget(id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// StreamingList handles GET /v1/streaming — lists all registered targets.
func StreamingList(mgr StreamingManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statuses := mgr.ListStatuses()
		views := make([]streamingTargetView, len(statuses))
		for i, s := range statuses {
			views[i] = targetToView(s)
		}
		writeJSON(w, http.StatusOK, streamingListResponse{Targets: views})
	}
}

// StreamingTest handles POST /v1/streaming/{id}/test — TCP reachability probe.
// It does not require a connected target; the host/port come from the request body.
func StreamingTest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req streamingTestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
			return
		}
		if req.Host == "" || req.Port <= 0 {
			writeError(w, http.StatusBadRequest, "bad_request", "host and port are required")
			return
		}

		addr := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
		start := time.Now()
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		elapsed := time.Since(start).Milliseconds()

		if err != nil {
			writeJSON(w, http.StatusOK, streamingTestResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
		_ = conn.Close()
		writeJSON(w, http.StatusOK, streamingTestResponse{
			OK:        true,
			LatencyMS: elapsed,
		})
	}
}

// --- helpers -----------------------------------------------------------------

func targetToView(s streaming.TargetStatus) streamingTargetView {
	return streamingTargetView{
		ID:          s.ID,
		State:       string(s.State),
		ConnectedAt: s.ConnectedAt,
		LastError:   s.LastError,
		RetryCount:  s.RetryCount,
		NextRetryAt: s.NextRetryAt,
		BytesSent:   s.BytesSent,
		UptimeMS:    s.UptimeMS,
	}
}
