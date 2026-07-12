package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RotationLogEntry records what was scheduled/played in a rotation slot.
type RotationLogEntry struct {
	ID         string
	TrackID    string
	PlayedAt   time.Time
	ClockID    string
	SlotType   string
	CategoryID string
	Artist     string
	Title      string
	Album      string
}

// RotationLogStore manages the append-only rotation_log table in SQLite.
type RotationLogStore struct {
	db *sql.DB
}

// NewRotationLogStore creates a RotationLogStore backed by db.
func NewRotationLogStore(db *sql.DB) *RotationLogStore {
	return &RotationLogStore{db: db}
}

// Append inserts a new entry into the rotation log.
func (s *RotationLogStore) Append(ctx context.Context, e RotationLogEntry) error {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.PlayedAt.IsZero() {
		e.PlayedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO rotation_log(id, track_id, played_at, clock_id, slot_type, category_id, artist, title, album)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.TrackID, e.PlayedAt.UTC().Format("2006-01-02T15:04:05Z"),
		e.ClockID, e.SlotType, e.CategoryID, e.Artist, e.Title, e.Album,
	)
	if err != nil {
		return fmt.Errorf("rotation log append: %w", err)
	}
	return nil
}

// ListByDate returns all entries logged on a given date (UTC calendar day).
func (s *RotationLogStore) ListByDate(ctx context.Context, date time.Time) ([]RotationLogEntry, error) {
	day := date.UTC().Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, track_id, played_at, clock_id, slot_type, category_id, artist, title, album
		FROM rotation_log
		WHERE date(played_at) = ?
		ORDER BY played_at ASC`, day)
	if err != nil {
		return nil, fmt.Errorf("rotation log list by date: %w", err)
	}
	defer rows.Close()
	return scanRotationRows(rows)
}

// RecentByField returns log entries where a specific field matches value,
// played at or after 'since'. Used by the generator to check separation rules.
// field must be one of: artist, title, album, category.
func (s *RotationLogStore) RecentByField(ctx context.Context, field, value string, since time.Time) ([]RotationLogEntry, error) {
	colMap := map[string]string{
		"artist":   "artist",
		"title":    "title",
		"album":    "album",
		"category": "category_id",
	}
	col, ok := colMap[field]
	if !ok {
		return nil, fmt.Errorf("rotation log: unknown field %q", field)
	}
	sinceStr := since.UTC().Format("2006-01-02T15:04:05Z")
	//nolint:gosec // col is validated above against a fixed whitelist
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`
			SELECT id, track_id, played_at, clock_id, slot_type, category_id, artist, title, album
			FROM rotation_log
			WHERE %s = ? AND played_at >= ?
			ORDER BY played_at DESC`, col),
		value, sinceStr)
	if err != nil {
		return nil, fmt.Errorf("rotation log recent by field: %w", err)
	}
	defer rows.Close()
	return scanRotationRows(rows)
}

// RecentTrackIDs returns a map of track_id → first played_at for tracks played
// since the given time. Used by the generator to build fast lookup sets.
func (s *RotationLogStore) RecentTrackIDs(ctx context.Context, since time.Time) (map[string]time.Time, error) {
	sinceStr := since.UTC().Format("2006-01-02T15:04:05Z")
	rows, err := s.db.QueryContext(ctx,
		`SELECT track_id, played_at FROM rotation_log WHERE played_at >= ? ORDER BY played_at DESC`,
		sinceStr)
	if err != nil {
		return nil, fmt.Errorf("rotation log recent track ids: %w", err)
	}
	defer rows.Close()

	out := map[string]time.Time{}
	for rows.Next() {
		var trackID, playedAtStr string
		if err := rows.Scan(&trackID, &playedAtStr); err != nil {
			return nil, err
		}
		t, _ := time.Parse("2006-01-02T15:04:05Z", playedAtStr)
		if _, exists := out[trackID]; !exists {
			out[trackID] = t // keep most-recent entry per track
		}
	}
	return out, rows.Err()
}

// OldestInCategory returns the track_id that was played longest ago for a
// given category. Returns ("", nil) when there is no history.
func (s *RotationLogStore) OldestInCategory(ctx context.Context, categoryID string) (string, error) {
	var trackID string
	err := s.db.QueryRowContext(ctx, `
		SELECT track_id FROM rotation_log
		WHERE category_id = ?
		ORDER BY played_at ASC
		LIMIT 1`, categoryID,
	).Scan(&trackID)
	if err != nil {
		return "", nil // no history
	}
	return trackID, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func scanRotationRows(rows *sql.Rows) ([]RotationLogEntry, error) {
	var out []RotationLogEntry
	for rows.Next() {
		var e RotationLogEntry
		var playedAt string
		if err := rows.Scan(
			&e.ID, &e.TrackID, &playedAt, &e.ClockID, &e.SlotType, &e.CategoryID,
			&e.Artist, &e.Title, &e.Album,
		); err != nil {
			return nil, fmt.Errorf("rotation log scan: %w", err)
		}
		e.PlayedAt, _ = time.Parse("2006-01-02T15:04:05Z", playedAt)
		out = append(out, e)
	}
	return out, rows.Err()
}
