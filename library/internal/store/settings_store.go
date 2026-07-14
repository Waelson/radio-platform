package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// SettingRow is a single row from the settings table.
type SettingRow struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}

// StationInfo holds radio station data used in ECAD export headers.
// Values are populated from settings keys with prefix "station.*".
type StationInfo struct {
	Name      string // station.name
	CNPJ      string // station.cnpj
	Frequency string // station.frequency
	Type      string // station.type — FM | AM | WEB
	City      string // station.city
	State     string // station.state
}

// SettingsStore provides typed access to the settings key→value table.
// All writes use upsert so the table stays in sync regardless of INSERT OR IGNORE
// applied during migrations.
type SettingsStore struct {
	db *sql.DB
}

// NewSettingsStore creates a SettingsStore backed by db.
func NewSettingsStore(db *sql.DB) *SettingsStore { return &SettingsStore{db: db} }

// Get returns the value for key, or ErrNotFound if absent.
func (s *SettingsStore) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM settings WHERE key = ?`, key,
	).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("settings get %q: %w", key, err)
	}
	return v, nil
}

// Set upserts key→value and updates updated_at.
func (s *SettingsStore) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE
		    SET value = excluded.value,
		        updated_at = excluded.updated_at
	`, key, value)
	if err != nil {
		return fmt.Errorf("settings set %q: %w", key, err)
	}
	return nil
}

// List returns all settings rows sorted by key.
func (s *SettingsStore) List(ctx context.Context) ([]SettingRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value, updated_at FROM settings ORDER BY key ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("settings list: %w", err)
	}
	defer rows.Close()

	var out []SettingRow
	for rows.Next() {
		var r SettingRow
		var updAt string
		if err := rows.Scan(&r.Key, &r.Value, &updAt); err != nil {
			return nil, err
		}
		r.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updAt)
		if r.UpdatedAt.IsZero() {
			r.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updAt)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NormalizationSettings holds all loudness-normalization parameters read from
// the settings table (keys injected by migration 010).
type NormalizationSettings struct {
	Enabled        bool
	TargetLUFS     float64
	MaxGainDB      float64
	PerTypeEnabled bool
	TargetMusic    float64
	TargetJingle   float64
	TargetVinheta  float64
	TargetSpot     float64
}

// ── Typed helpers ─────────────────────────────────────────────────────────────

func (s *SettingsStore) getOrDefault(ctx context.Context, key, def string) string {
	v, err := s.Get(ctx, key)
	if err != nil || v == "" {
		return def
	}
	return v
}

// TransmissionLogDir returns the directory where JSONL files are written.
func (s *SettingsStore) TransmissionLogDir(ctx context.Context) (string, error) {
	return s.getOrDefault(ctx, "transmission_log.dir", "/var/radioflow/transmission-logs"), nil
}

// TransmissionLogFileNameTemplate returns the file name template with {date}/{hour} placeholders.
func (s *SettingsStore) TransmissionLogFileNameTemplate(ctx context.Context) (string, error) {
	return s.getOrDefault(ctx, "transmission_log.file_name_template", "transmission_{date}_{hour}.jsonl"), nil
}

// TransmissionLogPollInterval returns the importer poll interval.
// Falls back to 5 minutes on parse error.
func (s *SettingsStore) TransmissionLogPollInterval(ctx context.Context) (time.Duration, error) {
	v := s.getOrDefault(ctx, "transmission_log.poll_interval", "5m")
	d, err := time.ParseDuration(v)
	if err != nil {
		return 5 * time.Minute, nil
	}
	return d, nil
}

// TransmissionLogGracePeriod returns the minimum age before an importer processes a file.
// Falls back to 15 minutes on parse error.
func (s *SettingsStore) TransmissionLogGracePeriod(ctx context.Context) (time.Duration, error) {
	v := s.getOrDefault(ctx, "transmission_log.grace_period", "15m")
	d, err := time.ParseDuration(v)
	if err != nil {
		return 15 * time.Minute, nil
	}
	return d, nil
}

// TransmissionLogRetentionDays returns the number of days to keep processed files.
// Enforces a minimum of 7 days.
func (s *SettingsStore) TransmissionLogRetentionDays(ctx context.Context) (int, error) {
	v := s.getOrDefault(ctx, "transmission_log.retention_days", "30")
	n, err := strconv.Atoi(v)
	if err != nil {
		return 30, nil
	}
	return n, nil
}

// RetentionDaysOrDefault returns retention days, elevating values below 7 to 7.
func (s *SettingsStore) RetentionDaysOrDefault(ctx context.Context) int {
	days, _ := s.TransmissionLogRetentionDays(ctx)
	if days < 7 {
		return 7
	}
	return days
}

// NormalizationSettings reads all normalization.* keys from the settings table.
// Missing or malformed values fall back to the defaults set by migration 010.
func (s *SettingsStore) NormalizationSettings(ctx context.Context) (NormalizationSettings, error) {
	parseBool := func(v, def string) bool {
		if v == "" {
			v = def
		}
		return v == "true" || v == "1"
	}
	parseFloat := func(v, def string) float64 {
		if v == "" {
			v = def
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			f, _ = strconv.ParseFloat(def, 64)
		}
		return f
	}

	g := func(key, def string) string { return s.getOrDefault(ctx, key, def) }

	return NormalizationSettings{
		Enabled:        parseBool(g("normalization.enabled", "true"), "true"),
		TargetLUFS:     parseFloat(g("normalization.target_lufs", "-16.0"), "-16.0"),
		MaxGainDB:      parseFloat(g("normalization.max_gain_db", "12.0"), "12.0"),
		PerTypeEnabled: parseBool(g("normalization.per_type_enabled", "false"), "false"),
		TargetMusic:    parseFloat(g("normalization.target_lufs_music", "-16.0"), "-16.0"),
		TargetJingle:   parseFloat(g("normalization.target_lufs_jingle", "-16.0"), "-16.0"),
		TargetVinheta:  parseFloat(g("normalization.target_lufs_vinheta", "-18.0"), "-18.0"),
		TargetSpot:     parseFloat(g("normalization.target_lufs_spot", "-14.0"), "-14.0"),
	}, nil
}

// StationInfo returns the radio station data from settings.
func (s *SettingsStore) StationInfo(ctx context.Context) StationInfo {
	return StationInfo{
		Name:      s.getOrDefault(ctx, "station.name", ""),
		CNPJ:      s.getOrDefault(ctx, "station.cnpj", ""),
		Frequency: s.getOrDefault(ctx, "station.frequency", ""),
		Type:      s.getOrDefault(ctx, "station.type", "FM"),
		City:      s.getOrDefault(ctx, "station.city", ""),
		State:     s.getOrDefault(ctx, "station.state", ""),
	}
}
