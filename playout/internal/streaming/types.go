// Package streaming manages Icecast/SHOUTcast streaming targets.
// Configuration is never persisted by the Playout Engine — it is received
// in-memory from the Player at connect time and discarded after disconnect.
package streaming

import "time"

// TargetState describes the lifecycle state of a single streaming target.
type TargetState string

const (
	StateIdle         TargetState = "idle"
	StateConnecting   TargetState = "connecting"
	StateConnected    TargetState = "connected"
	StateReconnecting TargetState = "reconnecting"
	StateError        TargetState = "error"
	StateDisconnected TargetState = "disconnected"
)

// ReconnectConfig controls the exponential back-off reconnection strategy.
type ReconnectConfig struct {
	Enabled           bool
	MaxRetries        int     // 0 = infinite
	InitialDelaySec   int
	MaxDelaySec       int
	BackoffMultiplier float64
}

// TargetConfig is the in-memory configuration received from the Player at
// connect time. It is never written to disk by the Playout Engine.
type TargetConfig struct {
	ID                 string
	Name               string
	Type               string // "icecast" | "shoutcast_v1" | "shoutcast_v2"
	Host               string
	Port               int
	Mount              string
	Password           string // held in memory only during the stream
	Format             string // "mp3" | "ogg_vorbis" | "ogg_opus" | "aac"
	BitrateKbps        int
	SampleRate         int    // encoder output sample rate (e.g. 44100)
	Channels           int
	SendMetadata       bool
	StationName        string
	StationDescription string
	StationGenre       string
	StationURL         string
	Reconnect          ReconnectConfig
}

// TargetStatus is the real-time status of a streaming target (in-memory only).
type TargetStatus struct {
	ID            string      `json:"id"`
	State         TargetState `json:"state"`
	ConnectedAt   *time.Time  `json:"connected_at,omitempty"`
	LastError     string      `json:"last_error,omitempty"`
	RetryCount    int         `json:"retry_count"`
	NextRetryAt   *time.Time  `json:"next_retry_at,omitempty"`
	Listeners     int         `json:"listeners"`
	BytesSent     int64       `json:"bytes_sent"`
	UptimeMS      int64       `json:"uptime_ms"`
	CurrentTitle  string      `json:"current_title"`
	CurrentArtist string      `json:"current_artist"`
}
