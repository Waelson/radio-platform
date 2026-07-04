package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Playlist represents a named, ordered list of tracks.
type Playlist struct {
	ID        string
	Name      string
	Category  string
	ItemCount int          // populated by List; zero in FindByID
	Items     []PlaylistItem // populated by FindByID; nil in List
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PlaylistItem is a single track slot in a playlist.
type PlaylistItem struct {
	ID       string
	TrackID  string
	Position int
	Track    Track // always populated when returned from the store
}

// PlaylistPatch holds optional fields for a playlist update.
type PlaylistPatch struct {
	Name     *string
	Category *string
}

// PlaylistStore manages playlist rows in SQLite.
type PlaylistStore struct {
	db *sql.DB
}

// NewPlaylistStore creates a PlaylistStore backed by db.
func NewPlaylistStore(db *sql.DB) *PlaylistStore {
	return &PlaylistStore{db: db}
}

// Create inserts a new playlist and returns it.
func (s *PlaylistStore) Create(ctx context.Context, name, category string) (Playlist, error) {
	if strings.TrimSpace(name) == "" {
		return Playlist{}, fmt.Errorf("playlist name must not be empty")
	}
	id := newID()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO playlists(id, name, category)
		VALUES (?, ?, ?)`,
		id, name, nullableStr(category),
	)
	if err != nil {
		return Playlist{}, fmt.Errorf("playlist create: %w", err)
	}
	return s.FindByID(ctx, id)
}

// FindByID returns a playlist with all its items (including denormalized track
// data), or ErrNotFound.
func (s *PlaylistStore) FindByID(ctx context.Context, id string) (Playlist, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, COALESCE(category,''), created_at, updated_at
		FROM playlists WHERE id = ?`, id)

	var p Playlist
	var createdAt, updatedAt string
	if err := row.Scan(&p.ID, &p.Name, &p.Category, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Playlist{}, ErrNotFound
		}
		return Playlist{}, fmt.Errorf("playlist find: %w", err)
	}
	p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)

	items, err := s.listItems(ctx, id)
	if err != nil {
		return Playlist{}, err
	}
	p.Items = items
	return p, nil
}

// List returns all playlists (without items) with their item counts, ordered
// alphabetically by name.
func (s *PlaylistStore) List(ctx context.Context) ([]Playlist, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, COALESCE(p.category,''),
		       p.created_at, p.updated_at,
		       COUNT(pi.id) AS item_count
		FROM playlists p
		LEFT JOIN playlist_items pi ON pi.playlist_id = p.id
		GROUP BY p.id
		ORDER BY p.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("playlist list: %w", err)
	}
	defer rows.Close()

	var out []Playlist
	for rows.Next() {
		var p Playlist
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Category,
			&createdAt, &updatedAt, &p.ItemCount); err != nil {
			return nil, fmt.Errorf("playlist list scan: %w", err)
		}
		p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
		p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		out = append(out, p)
	}
	return out, rows.Err()
}

// Update applies the non-nil fields from patch to the playlist with id.
// Returns ErrNotFound if the playlist does not exist.
func (s *PlaylistStore) Update(ctx context.Context, id string, patch PlaylistPatch) error {
	var setClauses []string
	var args []any

	if patch.Name != nil {
		if strings.TrimSpace(*patch.Name) == "" {
			return fmt.Errorf("playlist name must not be empty")
		}
		setClauses = append(setClauses, "name = ?")
		args = append(args, *patch.Name)
	}
	if patch.Category != nil {
		setClauses = append(setClauses, "category = ?")
		args = append(args, nullableStr(*patch.Category))
	}
	if len(setClauses) == 0 {
		return nil
	}
	setClauses = append(setClauses, "updated_at = datetime('now')")
	args = append(args, id)

	res, err := s.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE playlists SET %s WHERE id = ?",
			strings.Join(setClauses, ", ")),
		args...,
	)
	if err != nil {
		return fmt.Errorf("playlist update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes the playlist and all its items (CASCADE). Idempotent.
func (s *PlaylistStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM playlists WHERE id = ?`, id)
	return err
}

// AddItem appends a track to the playlist at the next available position.
// Returns ErrNotFound if the playlist or track does not exist.
func (s *PlaylistStore) AddItem(ctx context.Context, playlistID, trackID string) (PlaylistItem, error) {
	// Verify playlist exists.
	var exists int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM playlists WHERE id = ?`, playlistID).Scan(&exists)
	if err != nil || exists == 0 {
		return PlaylistItem{}, ErrNotFound
	}

	// Verify track exists.
	var track Track
	row := s.db.QueryRowContext(ctx, `
		SELECT id, path, title, artist, COALESCE(album,''), type, duration_ms,
		       COALESCE(category,''), indexed_at
		FROM tracks WHERE id = ?`, trackID)
	track, err = scanTrack(row)
	if errors.Is(err, ErrNotFound) {
		return PlaylistItem{}, fmt.Errorf("track %q: %w", trackID, ErrNotFound)
	}
	if err != nil {
		return PlaylistItem{}, fmt.Errorf("add item: check track: %w", err)
	}

	// Determine next position.
	var maxPos sql.NullInt64
	_ = s.db.QueryRowContext(ctx,
		`SELECT MAX(position) FROM playlist_items WHERE playlist_id = ?`, playlistID,
	).Scan(&maxPos)
	nextPos := int(maxPos.Int64) + 1

	itemID := newID()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO playlist_items(id, playlist_id, track_id, position)
		VALUES (?, ?, ?, ?)`,
		itemID, playlistID, trackID, nextPos,
	); err != nil {
		return PlaylistItem{}, fmt.Errorf("add item: insert: %w", err)
	}

	// Update playlist updated_at.
	_, _ = s.db.ExecContext(ctx,
		`UPDATE playlists SET updated_at = datetime('now') WHERE id = ?`, playlistID)

	return PlaylistItem{
		ID:       itemID,
		TrackID:  trackID,
		Position: nextPos,
		Track:    track,
	}, nil
}

// RemoveItem deletes a playlist item by item ID. Idempotent.
func (s *PlaylistStore) RemoveItem(ctx context.Context, itemID string) error {
	// Find playlist_id so we can update its updated_at.
	var playlistID string
	_ = s.db.QueryRowContext(ctx,
		`SELECT playlist_id FROM playlist_items WHERE id = ?`, itemID).Scan(&playlistID)

	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM playlist_items WHERE id = ?`, itemID); err != nil {
		return fmt.Errorf("remove item: %w", err)
	}

	if playlistID != "" {
		_, _ = s.db.ExecContext(ctx,
			`UPDATE playlists SET updated_at = datetime('now') WHERE id = ?`, playlistID)
	}
	return nil
}

// ReorderItems reorders the items of a playlist according to itemIDs (which
// must contain exactly the current item IDs of the playlist in the desired
// order). Returns an error if any ID does not belong to the playlist.
func (s *PlaylistStore) ReorderItems(ctx context.Context, playlistID string, itemIDs []string) error {
	if len(itemIDs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reorder: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Verify all IDs belong to this playlist.
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM playlist_items WHERE playlist_id = ?`, playlistID)
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
			return fmt.Errorf("reorder: item %q does not belong to playlist", id)
		}
	}

	// Assign new positions.
	for i, id := range itemIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE playlist_items SET position = ? WHERE id = ?`, i+1, id); err != nil {
			return fmt.Errorf("reorder: update position: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE playlists SET updated_at = datetime('now') WHERE id = ?`, playlistID); err != nil {
		return fmt.Errorf("reorder: update playlist: %w", err)
	}

	return tx.Commit()
}

// listItems returns all PlaylistItems for a playlist, ordered by position.
func (s *PlaylistStore) listItems(ctx context.Context, playlistID string) ([]PlaylistItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pi.id, pi.track_id, pi.position,
		       t.id, t.path, t.title, t.artist, t.type,
		       t.duration_ms, COALESCE(t.category,''), t.indexed_at
		FROM playlist_items pi
		JOIN tracks t ON t.id = pi.track_id
		WHERE pi.playlist_id = ?
		ORDER BY pi.position ASC`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	var out []PlaylistItem
	for rows.Next() {
		var item PlaylistItem
		var t Track
		var indexedAt string
		if err := rows.Scan(
			&item.ID, &item.TrackID, &item.Position,
			&t.ID, &t.Path, &t.Title, &t.Artist, &t.Type,
			&t.DurationMS, &t.Category, &indexedAt,
		); err != nil {
			return nil, fmt.Errorf("list items scan: %w", err)
		}
		t.IndexedAt, _ = time.Parse("2006-01-02T15:04:05Z", indexedAt)
		item.Track = t
		out = append(out, item)
	}
	return out, rows.Err()
}
