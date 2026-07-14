package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// ErrNotFound is returned when a requested resource does not exist in the store.
var ErrNotFound = errors.New("not found")

// Track represents an indexed audio file.
type Track struct {
	ID         string
	Path       string
	Title      string
	Artist     string
	Album      string // optional
	Type       string // MUSIC | VINHETA | JINGLE | SPOT
	ISRC       string // optional — from TSRC/ISRC tag
	Composer   string // optional — from TCOM/COMPOSER tag
	Publisher  string // optional — from TPUB/ORGANIZATION tag
	DurationMS int64
	Category   string
	IndexedAt  time.Time

	// Loudness analysis fields (populated by migration 009).
	// LoudnessLUFS and TruePeakDBTP are nil when not yet analyzed.
	LoudnessLUFS       *float64   // integrated loudness in LUFS (EBU R128)
	TruePeakDBTP       *float64   // true peak in dBTP
	LoudnessStatus     string     // pending | analyzing | done | error
	LoudnessError      string     // error message when LoudnessStatus = "error"
	LoudnessAnalyzedAt *time.Time // timestamp of last successful analysis
}

// SearchQuery holds optional filters for track searches.
type SearchQuery struct {
	Q        string // full-text search on title and artist
	Type     string
	Artist   string
	Album    string
	Category string
	Limit    int // default 50, max 200
	Offset   int

	// Loudness filters (all optional).
	LoudnessStatus string   // filter by loudness_status (pending|analyzing|done|error)
	LoudnessMin    *float64 // tracks with loudness_lufs >= value
	LoudnessMax    *float64 // tracks with loudness_lufs <= value
}

// TrackPatch carries the fields that may be updated via PATCH.
// A nil pointer means "do not change this field".
type TrackPatch struct {
	Title     *string
	Artist    *string
	Category  *string
	Type      *string
	ISRC      *string
	Composer  *string
	Publisher *string
}

// TrackStore manages track rows in SQLite.
type TrackStore struct {
	db *sql.DB
}

// NewTrackStore creates a TrackStore backed by db.
func NewTrackStore(db *sql.DB) *TrackStore {
	return &TrackStore{db: db}
}

// Upsert inserts a new track or updates an existing one identified by path.
// When updating, the original id is preserved so that foreign keys in
// playlist_items and break_items remain valid.
func (s *TrackStore) Upsert(ctx context.Context, t Track) error {
	var existingID string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM tracks WHERE path = ?`, t.Path,
	).Scan(&existingID)

	switch {
	case err == nil:
		// Row exists — update metadata only, keep the original id.
		_, err = s.db.ExecContext(ctx, `
			UPDATE tracks
			SET title = ?, artist = ?, album = ?, type = ?, duration_ms = ?,
			    category = ?, isrc = ?, composer = ?, publisher = ?,
			    indexed_at = datetime('now')
			WHERE id = ?`,
			t.Title, t.Artist, t.Album, t.Type, t.DurationMS,
			nullableStr(t.Category), t.ISRC, t.Composer, t.Publisher, existingID,
		)
		return err

	case errors.Is(err, sql.ErrNoRows):
		// New row — generate an id if not provided.
		if t.ID == "" {
			t.ID = newID()
		}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO tracks(id, path, title, artist, album, type, duration_ms, category, isrc, composer, publisher)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.Path, t.Title, t.Artist, t.Album, t.Type, t.DurationMS,
			nullableStr(t.Category), t.ISRC, t.Composer, t.Publisher,
		)
		return err

	default:
		return fmt.Errorf("track upsert: check existing: %w", err)
	}
}

// FindByID returns the track with the given id or ErrNotFound.
func (s *TrackStore) FindByID(ctx context.Context, id string) (Track, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, path, title, artist, COALESCE(album,''), type, duration_ms,
		       COALESCE(category,''), isrc, composer, publisher, indexed_at,
		       loudness_lufs, true_peak_dbtp,
		       COALESCE(loudness_status,'pending'), COALESCE(loudness_error,''),
		       loudness_analyzed_at
		FROM tracks WHERE id = ?`, id)
	return scanTrack(row)
}

// FindByPath returns the track with the given path or ErrNotFound.
func (s *TrackStore) FindByPath(ctx context.Context, path string) (Track, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, path, title, artist, COALESCE(album,''), type, duration_ms,
		       COALESCE(category,''), isrc, composer, publisher, indexed_at,
		       loudness_lufs, true_peak_dbtp,
		       COALESCE(loudness_status,'pending'), COALESCE(loudness_error,''),
		       loudness_analyzed_at
		FROM tracks WHERE path = ?`, path)
	return scanTrack(row)
}

// Search returns tracks matching the query filters, ordered by title.
func (s *TrackStore) Search(ctx context.Context, q SearchQuery) ([]Track, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var where []string
	var args []any

	if q.Q != "" {
		where = append(where, "(title LIKE ? OR artist LIKE ?)")
		like := "%" + q.Q + "%"
		args = append(args, like, like)
	}
	if q.Type != "" {
		where = append(where, "type = ?")
		args = append(args, q.Type)
	}
	if q.Artist != "" {
		where = append(where, "artist LIKE ?")
		args = append(args, "%"+q.Artist+"%")
	}
	if q.Album != "" {
		where = append(where, "album LIKE ?")
		args = append(args, "%"+q.Album+"%")
	}
	if q.Category != "" {
		where = append(where, "category = ?")
		args = append(args, q.Category)
	}
	if q.LoudnessStatus != "" {
		where = append(where, "COALESCE(loudness_status,'pending') = ?")
		args = append(args, q.LoudnessStatus)
	}
	if q.LoudnessMin != nil {
		where = append(where, "loudness_lufs >= ?")
		args = append(args, *q.LoudnessMin)
	}
	if q.LoudnessMax != nil {
		where = append(where, "loudness_lufs <= ?")
		args = append(args, *q.LoudnessMax)
	}

	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, path, title, artist, COALESCE(album,''), type, duration_ms,
		       COALESCE(category,''), isrc, composer, publisher, indexed_at,
		       loudness_lufs, true_peak_dbtp,
		       COALESCE(loudness_status,'pending'), COALESCE(loudness_error,''),
		       loudness_analyzed_at
		FROM tracks %s
		ORDER BY title ASC
		LIMIT ? OFFSET ?`, clause)

	args = append(args, limit, q.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("track search: %w", err)
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		t, err := scanTrackRow(rows)
		if err != nil {
			return nil, fmt.Errorf("track search: scan: %w", err)
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// Count returns the total number of indexed tracks.
func (s *TrackStore) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tracks`).Scan(&n)
	return n, err
}

// CountFiltered returns the total number of tracks matching the query filters,
// using the same WHERE clauses as Search but without LIMIT/OFFSET.
func (s *TrackStore) CountFiltered(ctx context.Context, q SearchQuery) (int, error) {
	var where []string
	var args []any

	if q.Q != "" {
		where = append(where, "(title LIKE ? OR artist LIKE ?)")
		like := "%" + q.Q + "%"
		args = append(args, like, like)
	}
	if q.Type != "" {
		where = append(where, "type = ?")
		args = append(args, q.Type)
	}
	if q.Artist != "" {
		where = append(where, "artist LIKE ?")
		args = append(args, "%"+q.Artist+"%")
	}
	if q.Album != "" {
		where = append(where, "album LIKE ?")
		args = append(args, "%"+q.Album+"%")
	}
	if q.Category != "" {
		where = append(where, "category = ?")
		args = append(args, q.Category)
	}
	if q.LoudnessStatus != "" {
		where = append(where, "COALESCE(loudness_status,'pending') = ?")
		args = append(args, q.LoudnessStatus)
	}
	if q.LoudnessMin != nil {
		where = append(where, "loudness_lufs >= ?")
		args = append(args, *q.LoudnessMin)
	}
	if q.LoudnessMax != nil {
		where = append(where, "loudness_lufs <= ?")
		args = append(args, *q.LoudnessMax)
	}

	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM tracks %s`, clause)

	var n int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count filtered: %w", err)
	}
	return n, nil
}

// ListArtists returns distinct artist names, optionally filtered by track type,
// sorted alphabetically. Empty artist strings are excluded.
func (s *TrackStore) ListArtists(ctx context.Context, trackType string) ([]string, error) {
	query := `SELECT DISTINCT artist FROM tracks WHERE artist != '' `
	var args []any
	if trackType != "" {
		query += `AND type = ? `
		args = append(args, trackType)
	}
	query += `ORDER BY artist ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list artists: %w", err)
	}
	defer rows.Close()

	var artists []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

// UpdateMeta applies the non-nil fields from patch to the track with id.
// Returns ErrNotFound if no such track exists.
func (s *TrackStore) UpdateMeta(ctx context.Context, id string, patch TrackPatch) error {
	var setClauses []string
	var args []any

	if patch.Title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *patch.Title)
	}
	if patch.Artist != nil {
		setClauses = append(setClauses, "artist = ?")
		args = append(args, *patch.Artist)
	}
	if patch.Category != nil {
		setClauses = append(setClauses, "category = ?")
		args = append(args, nullableStr(*patch.Category))
	}
	if patch.Type != nil {
		setClauses = append(setClauses, "type = ?")
		args = append(args, *patch.Type)
	}
	if patch.ISRC != nil {
		setClauses = append(setClauses, "isrc = ?")
		args = append(args, *patch.ISRC)
	}
	if patch.Composer != nil {
		setClauses = append(setClauses, "composer = ?")
		args = append(args, *patch.Composer)
	}
	if patch.Publisher != nil {
		setClauses = append(setClauses, "publisher = ?")
		args = append(args, *patch.Publisher)
	}
	if len(setClauses) == 0 {
		return nil // nothing to update
	}

	args = append(args, id)
	result, err := s.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE tracks SET %s WHERE id = ?",
			strings.Join(setClauses, ", ")),
		args...,
	)
	if err != nil {
		return fmt.Errorf("update track: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByPath removes the track with the given path. It is a no-op if the
// track does not exist (idempotent).
func (s *TrackStore) DeleteByPath(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tracks WHERE path = ?`, path)
	return err
}

// --- loudness methods --------------------------------------------------------

// UpdateLoudness sets the measured loudness values and marks the track as done.
func (s *TrackStore) UpdateLoudness(ctx context.Context, id string, lufs float64, truePeak float64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE tracks
		SET loudness_lufs        = ?,
		    true_peak_dbtp       = ?,
		    loudness_status      = 'done',
		    loudness_error       = '',
		    loudness_analyzed_at = datetime('now')
		WHERE id = ?`, lufs, truePeak, id)
	if err != nil {
		return fmt.Errorf("update loudness: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLoudnessStatus sets the loudness_status for a track and, optionally,
// an error message (pass empty string when not in error state).
func (s *TrackStore) UpdateLoudnessStatus(ctx context.Context, id string, status string, errMsg string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE tracks SET loudness_status = ?, loudness_error = ? WHERE id = ?`,
		status, errMsg, id)
	if err != nil {
		return fmt.Errorf("update loudness status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountByLoudnessStatus returns a map of loudness_status → count for all tracks.
func (s *TrackStore) CountByLoudnessStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT COALESCE(loudness_status,'pending'), COUNT(*) FROM tracks GROUP BY loudness_status`)
	if err != nil {
		return nil, fmt.Errorf("count by loudness status: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}
	return result, rows.Err()
}

// ListPendingLoudness returns IDs of tracks with loudness_status IN ('pending', 'error'),
// up to limit entries, ordered by rowid (insertion order).
func (s *TrackStore) ListPendingLoudness(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM tracks
		WHERE loudness_status IN ('pending', 'error')
		ORDER BY rowid ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending loudness: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// --- helpers -----------------------------------------------------------------

func scanTrack(row *sql.Row) (Track, error) {
	var t Track
	var indexedAt string
	var lufs, truePeak sql.NullFloat64
	var loudnessAnalyzedAt sql.NullString
	err := row.Scan(
		&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.Type,
		&t.DurationMS, &t.Category, &t.ISRC, &t.Composer, &t.Publisher, &indexedAt,
		&lufs, &truePeak, &t.LoudnessStatus, &t.LoudnessError, &loudnessAnalyzedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Track{}, ErrNotFound
	}
	if err != nil {
		return Track{}, err
	}
	t.IndexedAt, _ = time.Parse("2006-01-02T15:04:05Z", indexedAt)
	if t.IndexedAt.IsZero() {
		t.IndexedAt, _ = time.Parse("2006-01-02 15:04:05", indexedAt)
	}
	if lufs.Valid {
		t.LoudnessLUFS = &lufs.Float64
	}
	if truePeak.Valid {
		t.TruePeakDBTP = &truePeak.Float64
	}
	if loudnessAnalyzedAt.Valid && loudnessAnalyzedAt.String != "" {
		ts := parseSQLiteTime(loudnessAnalyzedAt.String)
		if !ts.IsZero() {
			t.LoudnessAnalyzedAt = &ts
		}
	}
	return t, nil
}

func scanTrackRow(rows *sql.Rows) (Track, error) {
	var t Track
	var indexedAt string
	var lufs, truePeak sql.NullFloat64
	var loudnessAnalyzedAt sql.NullString
	err := rows.Scan(
		&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.Type,
		&t.DurationMS, &t.Category, &t.ISRC, &t.Composer, &t.Publisher, &indexedAt,
		&lufs, &truePeak, &t.LoudnessStatus, &t.LoudnessError, &loudnessAnalyzedAt,
	)
	if err != nil {
		return Track{}, err
	}
	t.IndexedAt, _ = time.Parse("2006-01-02T15:04:05Z", indexedAt)
	if t.IndexedAt.IsZero() {
		t.IndexedAt, _ = time.Parse("2006-01-02 15:04:05", indexedAt)
	}
	if lufs.Valid {
		t.LoudnessLUFS = &lufs.Float64
	}
	if truePeak.Valid {
		t.TruePeakDBTP = &truePeak.Float64
	}
	if loudnessAnalyzedAt.Valid && loudnessAnalyzedAt.String != "" {
		ts := parseSQLiteTime(loudnessAnalyzedAt.String)
		if !ts.IsZero() {
			t.LoudnessAnalyzedAt = &ts
		}
	}
	return t, nil
}

func parseSQLiteTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05Z", s)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	return t
}

// nullableStr returns nil for empty strings so SQLite stores NULL instead of "".
func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// newID generates a new ULID string.
func newID() string {
	return ulid.Make().String()
}
