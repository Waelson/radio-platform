package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// StreamingTarget holds the configuration of a streaming destination.
// The Password field is always populated when returned from the store —
// callers must redact it before exposing it to API responses.
type StreamingTarget struct {
	ID                         string
	Name                       string
	Enabled                    bool
	Type                       string // "icecast" | "shoutcast_v1" | "shoutcast_v2"
	Host                       string
	Port                       int
	Mount                      string
	Password                   string
	Format                     string // "mp3" | "ogg_vorbis" | "ogg_opus" | "aac"
	BitrateKbps                int
	SampleRate                 int
	Channels                   int
	SendMetadata               bool
	StationName                string
	StationDescription         string
	StationGenre               string
	StationURL                 string
	ReconnectEnabled           bool
	ReconnectMaxRetries        int
	ReconnectInitialDelaySec   int
	ReconnectMaxDelaySec       int
	ReconnectBackoffMultiplier float64
	AutoConnect                bool
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// StreamingTargetInput carries the fields accepted on create and update.
type StreamingTargetInput struct {
	Name                       string
	Enabled                    *bool
	Type                       string
	Host                       string
	Port                       int
	Mount                      string
	Password                   string
	Format                     string
	BitrateKbps                int
	SampleRate                 int
	Channels                   int
	SendMetadata               *bool
	StationName                string
	StationDescription         string
	StationGenre               string
	StationURL                 string
	ReconnectEnabled           *bool
	ReconnectMaxRetries        int
	ReconnectInitialDelaySec   int
	ReconnectMaxDelaySec       int
	ReconnectBackoffMultiplier float64
	AutoConnect                *bool
}

// StreamingStore manages streaming_targets in SQLite.
type StreamingStore struct {
	db *sql.DB
}

// NewStreamingStore creates a StreamingStore backed by db.
func NewStreamingStore(db *sql.DB) *StreamingStore {
	return &StreamingStore{db: db}
}

// List returns all streaming targets ordered by name.
func (s *StreamingStore) List(ctx context.Context) ([]StreamingTarget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, enabled, type, host, port, mount, password,
		       format, bitrate_kbps, sample_rate, channels, send_metadata,
		       station_name, station_description, station_genre, station_url,
		       reconnect_enabled, reconnect_max_retries, reconnect_initial_delay_sec,
		       reconnect_max_delay_sec, reconnect_backoff_multiplier,
		       auto_connect, created_at, updated_at
		FROM streaming_targets
		ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("streaming list: %w", err)
	}
	defer rows.Close()

	var out []StreamingTarget
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, fmt.Errorf("streaming list scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns a single target by ID or ErrNotFound.
func (s *StreamingStore) Get(ctx context.Context, id string) (StreamingTarget, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, enabled, type, host, port, mount, password,
		       format, bitrate_kbps, sample_rate, channels, send_metadata,
		       station_name, station_description, station_genre, station_url,
		       reconnect_enabled, reconnect_max_retries, reconnect_initial_delay_sec,
		       reconnect_max_delay_sec, reconnect_backoff_multiplier,
		       auto_connect, created_at, updated_at
		FROM streaming_targets WHERE id = ?`, id)

	t, err := scanTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return StreamingTarget{}, ErrNotFound
	}
	if err != nil {
		return StreamingTarget{}, fmt.Errorf("streaming get: %w", err)
	}
	return t, nil
}

// Create inserts a new streaming target and returns it with generated ID.
func (s *StreamingStore) Create(ctx context.Context, in StreamingTargetInput) (StreamingTarget, error) {
	if err := validateInput(in); err != nil {
		return StreamingTarget{}, err
	}
	id := newID()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO streaming_targets (
			id, name, enabled, type, host, port, mount, password,
			format, bitrate_kbps, sample_rate, channels, send_metadata,
			station_name, station_description, station_genre, station_url,
			reconnect_enabled, reconnect_max_retries, reconnect_initial_delay_sec,
			reconnect_max_delay_sec, reconnect_backoff_multiplier, auto_connect
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		in.Name,
		boolToInt(derefBool(in.Enabled, true)),
		in.Type,
		in.Host,
		in.Port,
		in.Mount,
		in.Password,
		formatOrDefault(in.Format),
		bitrateOrDefault(in.BitrateKbps),
		sampleRateOrDefault(in.SampleRate),
		channelsOrDefault(in.Channels),
		boolToInt(derefBool(in.SendMetadata, true)),
		in.StationName,
		in.StationDescription,
		in.StationGenre,
		in.StationURL,
		boolToInt(derefBool(in.ReconnectEnabled, true)),
		in.ReconnectMaxRetries,
		reconnectDelayOrDefault(in.ReconnectInitialDelaySec, 2),
		reconnectDelayOrDefault(in.ReconnectMaxDelaySec, 60),
		backoffOrDefault(in.ReconnectBackoffMultiplier),
		boolToInt(derefBool(in.AutoConnect, false)),
	)
	if err != nil {
		return StreamingTarget{}, fmt.Errorf("streaming create: %w", err)
	}
	return s.Get(ctx, id)
}

// Update replaces all mutable fields of a target. Returns ErrNotFound when the
// target does not exist.
func (s *StreamingStore) Update(ctx context.Context, id string, in StreamingTargetInput) (StreamingTarget, error) {
	if err := validateInput(in); err != nil {
		return StreamingTarget{}, err
	}

	// Keep existing password when caller sends an empty string.
	setClauses := []string{
		"name = ?", "enabled = ?", "type = ?", "host = ?", "port = ?",
		"mount = ?", "format = ?", "bitrate_kbps = ?", "sample_rate = ?",
		"channels = ?", "send_metadata = ?",
		"station_name = ?", "station_description = ?", "station_genre = ?", "station_url = ?",
		"reconnect_enabled = ?", "reconnect_max_retries = ?",
		"reconnect_initial_delay_sec = ?", "reconnect_max_delay_sec = ?",
		"reconnect_backoff_multiplier = ?", "auto_connect = ?",
		"updated_at = datetime('now')",
	}
	args := []any{
		in.Name,
		boolToInt(derefBool(in.Enabled, true)),
		in.Type,
		in.Host,
		in.Port,
		in.Mount,
		formatOrDefault(in.Format),
		bitrateOrDefault(in.BitrateKbps),
		sampleRateOrDefault(in.SampleRate),
		channelsOrDefault(in.Channels),
		boolToInt(derefBool(in.SendMetadata, true)),
		in.StationName,
		in.StationDescription,
		in.StationGenre,
		in.StationURL,
		boolToInt(derefBool(in.ReconnectEnabled, true)),
		in.ReconnectMaxRetries,
		reconnectDelayOrDefault(in.ReconnectInitialDelaySec, 2),
		reconnectDelayOrDefault(in.ReconnectMaxDelaySec, 60),
		backoffOrDefault(in.ReconnectBackoffMultiplier),
		boolToInt(derefBool(in.AutoConnect, false)),
	}

	if in.Password != "" {
		setClauses = append([]string{"password = ?"}, setClauses...)
		args = append([]any{in.Password}, args...)
	}

	args = append(args, id)
	res, err := s.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE streaming_targets SET %s WHERE id = ?",
			strings.Join(setClauses, ", ")),
		args...,
	)
	if err != nil {
		return StreamingTarget{}, fmt.Errorf("streaming update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return StreamingTarget{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Delete removes a target by ID. Idempotent — no error if not found.
func (s *StreamingStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM streaming_targets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("streaming delete: %w", err)
	}
	return nil
}

// ── scanner ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTarget(r rowScanner) (StreamingTarget, error) {
	var t StreamingTarget
	var enabled, sendMeta, reconnEnabled, autoConnect int
	var createdAt, updatedAt string

	if err := r.Scan(
		&t.ID, &t.Name, &enabled, &t.Type, &t.Host, &t.Port, &t.Mount, &t.Password,
		&t.Format, &t.BitrateKbps, &t.SampleRate, &t.Channels, &sendMeta,
		&t.StationName, &t.StationDescription, &t.StationGenre, &t.StationURL,
		&reconnEnabled, &t.ReconnectMaxRetries, &t.ReconnectInitialDelaySec,
		&t.ReconnectMaxDelaySec, &t.ReconnectBackoffMultiplier,
		&autoConnect, &createdAt, &updatedAt,
	); err != nil {
		return StreamingTarget{}, err
	}
	t.Enabled = enabled != 0
	t.SendMetadata = sendMeta != 0
	t.ReconnectEnabled = reconnEnabled != 0
	t.AutoConnect = autoConnect != 0
	t.CreatedAt = parseSQLiteTime(createdAt)
	t.UpdatedAt = parseSQLiteTime(updatedAt)
	return t, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func validateInput(in StreamingTargetInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("name must not be empty")
	}
	switch in.Type {
	case "icecast", "shoutcast_v1", "shoutcast_v2":
	default:
		return fmt.Errorf("type must be icecast, shoutcast_v1 or shoutcast_v2")
	}
	if strings.TrimSpace(in.Host) == "" {
		return fmt.Errorf("host must not be empty")
	}
	if in.Port <= 0 || in.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func derefBool(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func formatOrDefault(f string) string {
	switch f {
	case "mp3", "ogg_vorbis", "ogg_opus", "aac":
		return f
	}
	return "mp3"
}

func bitrateOrDefault(b int) int {
	if b <= 0 {
		return 128
	}
	return b
}

func sampleRateOrDefault(r int) int {
	if r <= 0 {
		return 44100
	}
	return r
}

func channelsOrDefault(c int) int {
	if c <= 0 {
		return 2
	}
	return c
}

func reconnectDelayOrDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func backoffOrDefault(v float64) float64 {
	if v <= 0 {
		return 2.0
	}
	return v
}
