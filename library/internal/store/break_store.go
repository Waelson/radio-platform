package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Break represents a commercial break with optional open/close jingles and an
// ordered list of spots.
type Break struct {
	ID           string
	Name         string
	OpenTrack    *Track     // nil when not set
	CloseTrack   *Track     // nil when not set
	ItemCount    int        // populated by List; zero in FindByID
	Items        []BreakItem // populated by FindByID; nil in List
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// BreakItem is a single spot slot inside a break.
type BreakItem struct {
	ID       string
	TrackID  string
	Position int
	Track    Track
}

// BreakPatch holds optional fields for a break update.
// A nil pointer means "do not change this field".
// A pointer to an empty string clears the FK (sets it to NULL).
type BreakPatch struct {
	Name         *string
	OpenTrackID  *string
	CloseTrackID *string
}

// BreakStore manages break rows in SQLite.
type BreakStore struct {
	db *sql.DB
}

// NewBreakStore creates a BreakStore backed by db.
func NewBreakStore(db *sql.DB) *BreakStore {
	return &BreakStore{db: db}
}

// Create inserts a new break and returns it with full detail.
// openTrackID and closeTrackID are optional; pass "" to leave unset.
func (s *BreakStore) Create(ctx context.Context, name, openTrackID, closeTrackID string) (Break, error) {
	if strings.TrimSpace(name) == "" {
		return Break{}, fmt.Errorf("break name must not be empty")
	}
	id := newID()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO breaks(id, name, open_track_id, close_track_id)
		VALUES (?, ?, ?, ?)`,
		id, name, nullableStr(openTrackID), nullableStr(closeTrackID),
	)
	if err != nil {
		return Break{}, fmt.Errorf("break create: %w", err)
	}
	return s.FindByID(ctx, id)
}

// FindByID returns a break with its open/close tracks and all items, or ErrNotFound.
func (s *BreakStore) FindByID(ctx context.Context, id string) (Break, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT b.id, b.name, b.created_at, b.updated_at,
		       ot.id, ot.path, ot.title, ot.artist, ot.type, ot.duration_ms,
		           COALESCE(ot.category,''), ot.indexed_at,
		       ct.id, ct.path, ct.title, ct.artist, ct.type, ct.duration_ms,
		           COALESCE(ct.category,''), ct.indexed_at
		FROM breaks b
		LEFT JOIN tracks ot ON ot.id = b.open_track_id
		LEFT JOIN tracks ct ON ct.id = b.close_track_id
		WHERE b.id = ?`, id)

	brk, err := scanBreakRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Break{}, ErrNotFound
	}
	if err != nil {
		return Break{}, fmt.Errorf("break find: %w", err)
	}

	brk.Items, err = s.listItems(ctx, id)
	return brk, err
}

// List returns all breaks (without items) with item counts, ordered alphabetically.
func (s *BreakStore) List(ctx context.Context) ([]Break, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.id, b.name, b.created_at, b.updated_at,
		       COUNT(bi.id) AS item_count
		FROM breaks b
		LEFT JOIN break_items bi ON bi.break_id = b.id
		GROUP BY b.id
		ORDER BY b.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("break list: %w", err)
	}
	defer rows.Close()

	var out []Break
	for rows.Next() {
		var brk Break
		var createdAt, updatedAt string
		if err := rows.Scan(&brk.ID, &brk.Name,
			&createdAt, &updatedAt, &brk.ItemCount); err != nil {
			return nil, fmt.Errorf("break list scan: %w", err)
		}
		brk.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
		brk.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		out = append(out, brk)
	}
	return out, rows.Err()
}

// Update applies the non-nil fields from patch to the break with id.
// Returns ErrNotFound if the break does not exist.
func (s *BreakStore) Update(ctx context.Context, id string, patch BreakPatch) error {
	var setClauses []string
	var args []any

	if patch.Name != nil {
		if strings.TrimSpace(*patch.Name) == "" {
			return fmt.Errorf("break name must not be empty")
		}
		setClauses = append(setClauses, "name = ?")
		args = append(args, *patch.Name)
	}
	if patch.OpenTrackID != nil {
		setClauses = append(setClauses, "open_track_id = ?")
		args = append(args, nullableStr(*patch.OpenTrackID))
	}
	if patch.CloseTrackID != nil {
		setClauses = append(setClauses, "close_track_id = ?")
		args = append(args, nullableStr(*patch.CloseTrackID))
	}
	if len(setClauses) == 0 {
		return nil
	}
	setClauses = append(setClauses, "updated_at = datetime('now')")
	args = append(args, id)

	res, err := s.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE breaks SET %s WHERE id = ?",
			strings.Join(setClauses, ", ")),
		args...,
	)
	if err != nil {
		return fmt.Errorf("break update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes the break and all its items (CASCADE). Idempotent.
func (s *BreakStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM breaks WHERE id = ?`, id)
	return err
}

// AddItem appends a track (must be SPOT) to the break at the next position.
// Returns ErrNotFound if the break does not exist.
func (s *BreakStore) AddItem(ctx context.Context, breakID, trackID string) (BreakItem, error) {
	var exists int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM breaks WHERE id = ?`, breakID).Scan(&exists)
	if err != nil || exists == 0 {
		return BreakItem{}, ErrNotFound
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT id, path, title, artist, COALESCE(album,''), type, duration_ms,
		       COALESCE(category,''), isrc, composer, publisher, indexed_at
		FROM tracks WHERE id = ?`, trackID)
	track, err := scanTrack(row)
	if errors.Is(err, ErrNotFound) {
		return BreakItem{}, fmt.Errorf("track %q: %w", trackID, ErrNotFound)
	}
	if err != nil {
		return BreakItem{}, fmt.Errorf("add item: check track: %w", err)
	}

	var maxPos sql.NullInt64
	_ = s.db.QueryRowContext(ctx,
		`SELECT MAX(position) FROM break_items WHERE break_id = ?`, breakID).Scan(&maxPos)
	nextPos := int(maxPos.Int64) + 1

	itemID := newID()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO break_items(id, break_id, track_id, position)
		VALUES (?, ?, ?, ?)`,
		itemID, breakID, trackID, nextPos,
	); err != nil {
		return BreakItem{}, fmt.Errorf("add item: insert: %w", err)
	}

	_, _ = s.db.ExecContext(ctx,
		`UPDATE breaks SET updated_at = datetime('now') WHERE id = ?`, breakID)

	return BreakItem{ID: itemID, TrackID: trackID, Position: nextPos, Track: track}, nil
}

// RemoveItem deletes a break item by item ID. Idempotent.
func (s *BreakStore) RemoveItem(ctx context.Context, itemID string) error {
	var breakID string
	_ = s.db.QueryRowContext(ctx,
		`SELECT break_id FROM break_items WHERE id = ?`, itemID).Scan(&breakID)

	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM break_items WHERE id = ?`, itemID); err != nil {
		return fmt.Errorf("remove item: %w", err)
	}
	if breakID != "" {
		_, _ = s.db.ExecContext(ctx,
			`UPDATE breaks SET updated_at = datetime('now') WHERE id = ?`, breakID)
	}
	return nil
}

// ReorderItems reorders the spots of a break according to itemIDs in the
// desired order. All IDs must belong to the break.
func (s *BreakStore) ReorderItems(ctx context.Context, breakID string, itemIDs []string) error {
	if len(itemIDs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reorder: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM break_items WHERE break_id = ?`, breakID)
	if err != nil {
		return fmt.Errorf("reorder: fetch items: %w", err)
	}
	existing := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		existing[id] = struct{}{}
	}
	rows.Close()

	for _, id := range itemIDs {
		if _, ok := existing[id]; !ok {
			return fmt.Errorf("reorder: item %q does not belong to break", id)
		}
	}

	for i, id := range itemIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE break_items SET position = ? WHERE id = ?`, i+1, id); err != nil {
			return fmt.Errorf("reorder: update position: %w", err)
		}
	}

	_, _ = tx.ExecContext(ctx,
		`UPDATE breaks SET updated_at = datetime('now') WHERE id = ?`, breakID)

	return tx.Commit()
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func scanBreakRow(row *sql.Row) (Break, error) {
	var brk Break
	var createdAt, updatedAt string

	// Nullable open-track columns.
	var otID, otPath, otTitle, otArtist, otType, otCategory, otIndexedAt sql.NullString
	var otDuration sql.NullInt64
	// Nullable close-track columns.
	var ctID, ctPath, ctTitle, ctArtist, ctType, ctCategory, ctIndexedAt sql.NullString
	var ctDuration sql.NullInt64

	err := row.Scan(
		&brk.ID, &brk.Name, &createdAt, &updatedAt,
		&otID, &otPath, &otTitle, &otArtist, &otType, &otDuration, &otCategory, &otIndexedAt,
		&ctID, &ctPath, &ctTitle, &ctArtist, &ctType, &ctDuration, &ctCategory, &ctIndexedAt,
	)
	if err != nil {
		return Break{}, err
	}

	brk.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	brk.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)

	if otID.Valid {
		t := &Track{
			ID: otID.String, Path: otPath.String, Title: otTitle.String,
			Artist: otArtist.String, Type: otType.String,
			DurationMS: otDuration.Int64, Category: otCategory.String,
		}
		t.IndexedAt, _ = time.Parse("2006-01-02T15:04:05Z", otIndexedAt.String)
		brk.OpenTrack = t
	}
	if ctID.Valid {
		t := &Track{
			ID: ctID.String, Path: ctPath.String, Title: ctTitle.String,
			Artist: ctArtist.String, Type: ctType.String,
			DurationMS: ctDuration.Int64, Category: ctCategory.String,
		}
		t.IndexedAt, _ = time.Parse("2006-01-02T15:04:05Z", ctIndexedAt.String)
		brk.CloseTrack = t
	}
	return brk, nil
}

func (s *BreakStore) listItems(ctx context.Context, breakID string) ([]BreakItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT bi.id, bi.track_id, bi.position,
		       t.id, t.path, t.title, t.artist, t.type,
		       t.duration_ms, COALESCE(t.category,''), t.indexed_at
		FROM break_items bi
		JOIN tracks t ON t.id = bi.track_id
		WHERE bi.break_id = ?
		ORDER BY bi.position ASC`, breakID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	var out []BreakItem
	for rows.Next() {
		var item BreakItem
		var t Track
		var indexedAt string
		if err := rows.Scan(
			&item.ID, &item.TrackID, &item.Position,
			&t.ID, &t.Path, &t.Title, &t.Artist, &t.Type,
			&t.DurationMS, &t.Category, &indexedAt,
		); err != nil {
			return nil, fmt.Errorf("break items scan: %w", err)
		}
		t.IndexedAt, _ = time.Parse("2006-01-02T15:04:05Z", indexedAt)
		item.Track = t
		out = append(out, item)
	}
	return out, rows.Err()
}
