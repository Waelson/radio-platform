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

	// Cue point fields (populated by migration 011). All nil = use defaults.
	CueInMS  *int64 // playback seek start (ms) — silence-trimmed head
	IntroMS  *int64 // vocal intro end (ms) — announcer window countdown
	OutroMS  *int64 // outro start (ms) — crossfade trigger
	CueOutMS *int64 // playback stop (ms) — silence-trimmed tail
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

// CuePoints holds the four cue point markers for a track.
// A nil pointer means "clear this marker" (store NULL).
type CuePoints struct {
	CueInMS  *int64 `json:"cue_in_ms"`
	IntroMS  *int64 `json:"intro_ms"`
	OutroMS  *int64 `json:"outro_ms"`
	CueOutMS *int64 `json:"cue_out_ms"`
}

// Validate checks that the cue point values are internally consistent:
//   - all non-nil values must be >= 0
//   - when multiple markers are set they must not contradict each other:
//     cue_in <= intro <= outro <= cue_out, with cue_in strictly < cue_out.
//
// Equal values between adjacent markers are allowed (e.g. cue_in == intro == 0
// means vocals start at the very beginning of the playable region).
func (cp CuePoints) Validate() error {
	check := func(name string, v *int64) error {
		if v != nil && *v < 0 {
			return fmt.Errorf("%s must be >= 0, got %d", name, *v)
		}
		return nil
	}
	if err := check("cue_in_ms", cp.CueInMS); err != nil {
		return err
	}
	if err := check("intro_ms", cp.IntroMS); err != nil {
		return err
	}
	if err := check("outro_ms", cp.OutroMS); err != nil {
		return err
	}
	if err := check("cue_out_ms", cp.CueOutMS); err != nil {
		return err
	}
	// Ordering: cue_in <= intro
	if cp.CueInMS != nil && cp.IntroMS != nil && *cp.CueInMS > *cp.IntroMS {
		return fmt.Errorf("cue_in_ms (%d) must be <= intro_ms (%d)", *cp.CueInMS, *cp.IntroMS)
	}
	// intro <= outro
	if cp.IntroMS != nil && cp.OutroMS != nil && *cp.IntroMS > *cp.OutroMS {
		return fmt.Errorf("intro_ms (%d) must be <= outro_ms (%d)", *cp.IntroMS, *cp.OutroMS)
	}
	// outro <= cue_out
	if cp.OutroMS != nil && cp.CueOutMS != nil && *cp.OutroMS > *cp.CueOutMS {
		return fmt.Errorf("outro_ms (%d) must be <= cue_out_ms (%d)", *cp.OutroMS, *cp.CueOutMS)
	}
	// cue_in strictly < cue_out (defines a non-zero playable region)
	if cp.CueInMS != nil && cp.CueOutMS != nil && *cp.CueInMS >= *cp.CueOutMS {
		return fmt.Errorf("cue_in_ms (%d) must be less than cue_out_ms (%d)", *cp.CueInMS, *cp.CueOutMS)
	}
	return nil
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
		       loudness_analyzed_at,
		       cue_in_ms, intro_ms, outro_ms, cue_out_ms
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
		       loudness_analyzed_at,
		       cue_in_ms, intro_ms, outro_ms, cue_out_ms
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
		       loudness_analyzed_at,
		       cue_in_ms, intro_ms, outro_ms, cue_out_ms
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

// --- cue point methods -------------------------------------------------------

// SaveCuePoints updates the four cue point markers for the track with id.
// Nil fields are stored as NULL (marker removed). Returns ErrNotFound when
// no track with that id exists.
func (s *TrackStore) SaveCuePoints(ctx context.Context, id string, cp CuePoints) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE tracks
		SET cue_in_ms  = ?,
		    intro_ms   = ?,
		    outro_ms   = ?,
		    cue_out_ms = ?
		WHERE id = ?`,
		cp.CueInMS, cp.IntroMS, cp.OutroMS, cp.CueOutMS, id,
	)
	if err != nil {
		return fmt.Errorf("save cue points: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetCueIn sets cue_in_ms for the track with id, but only when the column is
// currently NULL. If it is already set (even to 0), this is a no-op — the
// operator's manual value is never overwritten by auto-detection.
// Returns nil (not ErrNotFound) when the track exists but already has a value.
func (s *TrackStore) SetCueIn(ctx context.Context, id string, ms int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE tracks SET cue_in_ms = ?
		WHERE id = ? AND cue_in_ms IS NULL`,
		ms, id,
	)
	if err != nil {
		return fmt.Errorf("set cue_in: %w", err)
	}
	return nil
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

// ListNullCueIn returns IDs of tracks where cue_in_ms is NULL (not yet detected).
func (s *TrackStore) ListNullCueIn(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM tracks
		WHERE cue_in_ms IS NULL
		ORDER BY rowid ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list null cue_in: %w", err)
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

// CountCueInStatus returns counts of tracks with and without cue_in_ms set.
// The returned map has keys "pending" (NULL) and "done" (NOT NULL).
func (s *TrackStore) CountCueInStatus(ctx context.Context) (map[string]int, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN cue_in_ms IS NULL     THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN cue_in_ms IS NOT NULL THEN 1 ELSE 0 END), 0)
		FROM tracks`)
	var pending, done int
	if err := row.Scan(&pending, &done); err != nil {
		return nil, fmt.Errorf("count cue_in status: %w", err)
	}
	return map[string]int{"pending": pending, "done": done}, nil
}

// --- helpers -----------------------------------------------------------------

func scanTrack(row *sql.Row) (Track, error) {
	var t Track
	var indexedAt string
	var lufs, truePeak sql.NullFloat64
	var loudnessAnalyzedAt sql.NullString
	var cueInMS, introMS, outroMS, cueOutMS sql.NullInt64
	err := row.Scan(
		&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.Type,
		&t.DurationMS, &t.Category, &t.ISRC, &t.Composer, &t.Publisher, &indexedAt,
		&lufs, &truePeak, &t.LoudnessStatus, &t.LoudnessError, &loudnessAnalyzedAt,
		&cueInMS, &introMS, &outroMS, &cueOutMS,
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
	if cueInMS.Valid {
		t.CueInMS = &cueInMS.Int64
	}
	if introMS.Valid {
		t.IntroMS = &introMS.Int64
	}
	if outroMS.Valid {
		t.OutroMS = &outroMS.Int64
	}
	if cueOutMS.Valid {
		t.CueOutMS = &cueOutMS.Int64
	}
	return t, nil
}

func scanTrackRow(rows *sql.Rows) (Track, error) {
	var t Track
	var indexedAt string
	var lufs, truePeak sql.NullFloat64
	var loudnessAnalyzedAt sql.NullString
	var cueInMS, introMS, outroMS, cueOutMS sql.NullInt64
	err := rows.Scan(
		&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.Type,
		&t.DurationMS, &t.Category, &t.ISRC, &t.Composer, &t.Publisher, &indexedAt,
		&lufs, &truePeak, &t.LoudnessStatus, &t.LoudnessError, &loudnessAnalyzedAt,
		&cueInMS, &introMS, &outroMS, &cueOutMS,
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
	if cueInMS.Valid {
		t.CueInMS = &cueInMS.Int64
	}
	if introMS.Valid {
		t.IntroMS = &introMS.Int64
	}
	if outroMS.Valid {
		t.OutroMS = &outroMS.Int64
	}
	if cueOutMS.Valid {
		t.CueOutMS = &cueOutMS.Int64
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
