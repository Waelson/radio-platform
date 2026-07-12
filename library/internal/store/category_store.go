package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Category represents a rotation category (e.g. "MPB Clássica", "Rock Nacional").
type Category struct {
	ID          string
	Name        string
	Description string
	Color       string
	TrackCount  int
	CreatedAt   time.Time
}

// CategoryStore manages categories and track_categories in SQLite.
type CategoryStore struct {
	db *sql.DB
}

// NewCategoryStore creates a CategoryStore backed by db.
func NewCategoryStore(db *sql.DB) *CategoryStore {
	return &CategoryStore{db: db}
}

// List returns all categories with their track counts, ordered by name.
func (s *CategoryStore) List(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.description, c.color, c.created_at,
		       COUNT(tc.track_id) AS track_count
		FROM categories c
		LEFT JOIN track_categories tc ON tc.category_id = c.id
		GROUP BY c.id
		ORDER BY c.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("category list: %w", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("category list scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Create inserts a new category and returns it.
func (s *CategoryStore) Create(ctx context.Context, name, description, color string) (Category, error) {
	if strings.TrimSpace(name) == "" {
		return Category{}, fmt.Errorf("category name must not be empty")
	}
	id := newID()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO categories(id, name, description, color) VALUES (?, ?, ?, ?)`,
		id, name, description, color,
	); err != nil {
		return Category{}, fmt.Errorf("category create: %w", err)
	}
	return s.Get(ctx, id)
}

// Get returns a category by ID (with track count), or ErrNotFound.
func (s *CategoryStore) Get(ctx context.Context, id string) (Category, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT c.id, c.name, c.description, c.color, c.created_at,
		       COUNT(tc.track_id) AS track_count
		FROM categories c
		LEFT JOIN track_categories tc ON tc.category_id = c.id
		WHERE c.id = ?
		GROUP BY c.id`, id)

	c, err := scanCategoryRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	return c, err
}

// Update updates name, description and color of a category.
func (s *CategoryStore) Update(ctx context.Context, id, name, description, color string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("category name must not be empty")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE categories SET name = ?, description = ?, color = ? WHERE id = ?`,
		name, description, color, id)
	if err != nil {
		return fmt.Errorf("category update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a category. Returns an error if the category is referenced by any clock slot.
func (s *CategoryStore) Delete(ctx context.Context, id string) error {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM clock_slots WHERE category_id = ?`, id,
	).Scan(&n); err != nil {
		return fmt.Errorf("category delete check slots: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("category is referenced by %d clock slot(s); remove them first", n)
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, id); err != nil {
		return fmt.Errorf("category delete: %w", err)
	}
	return nil
}

// ListTracks returns tracks associated with categoryID, paginated.
func (s *CategoryStore) ListTracks(ctx context.Context, categoryID string, limit, offset int) ([]Track, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.path, t.title, t.artist, t.album, t.type, t.duration_ms, COALESCE(t.category,''), t.indexed_at
		FROM tracks t
		JOIN track_categories tc ON tc.track_id = t.id
		WHERE tc.category_id = ?
		ORDER BY t.artist ASC, t.title ASC
		LIMIT ? OFFSET ?`, categoryID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("category list tracks: %w", err)
	}
	defer rows.Close()

	var out []Track
	for rows.Next() {
		t, err := scanTrackRow(rows)
		if err != nil {
			return nil, fmt.Errorf("category list tracks scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AddTracks adds one or more tracks to a category. Silently ignores duplicates.
func (s *CategoryStore) AddTracks(ctx context.Context, categoryID string, trackIDs []string) error {
	if len(trackIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("category add tracks: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, tid := range trackIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO track_categories(track_id, category_id) VALUES (?, ?)`,
			tid, categoryID,
		); err != nil {
			return fmt.Errorf("category add track %q: %w", tid, err)
		}
	}
	return tx.Commit()
}

// RemoveTrack removes a single track from a category. Idempotent.
func (s *CategoryStore) RemoveTrack(ctx context.Context, categoryID, trackID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM track_categories WHERE category_id = ? AND track_id = ?`,
		categoryID, trackID)
	return err
}

// SetTrackCategories replaces all categories of a track with the given list.
// Pass an empty slice to remove all category associations for the track.
func (s *CategoryStore) SetTrackCategories(ctx context.Context, trackID string, categoryIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("set track categories: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM track_categories WHERE track_id = ?`, trackID,
	); err != nil {
		return fmt.Errorf("set track categories: delete old: %w", err)
	}
	for _, cid := range categoryIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO track_categories(track_id, category_id) VALUES (?, ?)`,
			trackID, cid,
		); err != nil {
			return fmt.Errorf("set track categories: insert %q: %w", cid, err)
		}
	}
	return tx.Commit()
}

// ListByTrack returns all categories a given track belongs to.
func (s *CategoryStore) ListByTrack(ctx context.Context, trackID string) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.description, c.color, c.created_at, 0 AS track_count
		FROM categories c
		JOIN track_categories tc ON tc.category_id = c.id
		WHERE tc.track_id = ?
		ORDER BY c.name ASC`, trackID)
	if err != nil {
		return nil, fmt.Errorf("list categories by track: %w", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("list categories by track scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ── internal helpers ──────────────────────────────────────────────────────────

type categoryRowScanner interface {
	Scan(dest ...any) error
}

func scanCategory(rows *sql.Rows) (Category, error) {
	var c Category
	var createdAt string
	if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Color, &createdAt, &c.TrackCount); err != nil {
		return Category{}, err
	}
	c.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	return c, nil
}

func scanCategoryRow(row categoryRowScanner) (Category, error) {
	var c Category
	var createdAt string
	if err := row.Scan(&c.ID, &c.Name, &c.Description, &c.Color, &createdAt, &c.TrackCount); err != nil {
		return Category{}, err
	}
	c.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	return c, nil
}

