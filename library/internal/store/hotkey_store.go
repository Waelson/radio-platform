package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// HotkeyProfile represents a named set of hotkey buttons.
type HotkeyProfile struct {
	ID        string
	Name      string
	Columns   int
	Buttons   []HotkeyButton // populated by FindProfileByID; nil in ListProfiles
	CreatedAt time.Time
	UpdatedAt time.Time
}

// HotkeyButton represents a single button within a hotkey profile.
// track_id may be NULL (ON DELETE SET NULL); track_path/title/artist are cached
// copies so the button remains usable even if the track is deleted from the library.
type HotkeyButton struct {
	ID           string
	ProfileID    string
	Position     int
	Label        string
	SubLabel     string
	Icon         string
	Palette      int
	TrackID      string // empty when track was deleted
	TrackPath    string
	TrackTitle   string
	TrackArtist  string
	TrackType    string
	DurationMS   int64
	CueInMS      *int64
	IntroMS      *int64
	OutroMS      *int64
	CueOutMS     *int64
	CreatedAt    time.Time
	LoudnessLUFS *float64 // nil when track not analyzed or button has no track
}

// HotkeyButtonPatch holds optional fields for a button update.
// A nil pointer means "do not change this field".
type HotkeyButtonPatch struct {
	Label       *string
	SubLabel    *string
	Icon        *string
	Palette     *int
	TrackID     *string
	TrackPath   *string
	TrackTitle  *string
	TrackArtist *string
	TrackType   *string
	DurationMS  *int64
	Position    *int
}

// HotkeyStore manages hotkey_profiles and hotkey_buttons in SQLite.
type HotkeyStore struct {
	db *sql.DB
}

// NewHotkeyStore creates a HotkeyStore backed by db.
func NewHotkeyStore(db *sql.DB) *HotkeyStore {
	return &HotkeyStore{db: db}
}

// ── Profiles ──────────────────────────────────────────────────────────────────

// ListProfiles returns all profiles (without buttons), ordered by name.
func (s *HotkeyStore) ListProfiles(ctx context.Context) ([]HotkeyProfile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, columns, created_at, updated_at
		FROM hotkey_profiles
		ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("hotkey list profiles: %w", err)
	}
	defer rows.Close()

	var out []HotkeyProfile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("hotkey list profiles scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateProfile inserts a new profile and returns it.
func (s *HotkeyStore) CreateProfile(ctx context.Context, name string, columns int) (HotkeyProfile, error) {
	if strings.TrimSpace(name) == "" {
		return HotkeyProfile{}, fmt.Errorf("profile name must not be empty")
	}
	if columns <= 0 {
		columns = 4
	}
	id := newID()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO hotkey_profiles(id, name, columns) VALUES (?, ?, ?)`,
		id, name, columns,
	); err != nil {
		return HotkeyProfile{}, fmt.Errorf("hotkey create profile: %w", err)
	}
	return s.FindProfileByID(ctx, id)
}

// FindProfileByID returns a profile with its buttons, or ErrNotFound.
func (s *HotkeyStore) FindProfileByID(ctx context.Context, id string) (HotkeyProfile, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, columns, created_at, updated_at
		FROM hotkey_profiles WHERE id = ?`, id)

	var p HotkeyProfile
	var createdAt, updatedAt string
	if err := row.Scan(&p.ID, &p.Name, &p.Columns, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HotkeyProfile{}, ErrNotFound
		}
		return HotkeyProfile{}, fmt.Errorf("hotkey find profile: %w", err)
	}
	p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)

	buttons, err := s.listButtons(ctx, id)
	if err != nil {
		return HotkeyProfile{}, err
	}
	p.Buttons = buttons
	return p, nil
}

// UpdateProfile updates name and/or columns of a profile.
// Returns ErrNotFound if it does not exist.
func (s *HotkeyStore) UpdateProfile(ctx context.Context, id, name string, columns int) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("profile name must not be empty")
	}
	if columns <= 0 {
		columns = 4
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE hotkey_profiles
		SET name = ?, columns = ?, updated_at = datetime('now')
		WHERE id = ?`, name, columns, id)
	if err != nil {
		return fmt.Errorf("hotkey update profile: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteProfile removes a profile and all its buttons (CASCADE). Idempotent.
func (s *HotkeyStore) DeleteProfile(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM hotkey_profiles WHERE id = ?`, id)
	return err
}

// ── Buttons ───────────────────────────────────────────────────────────────────

// AddButton appends a button to a profile at the next available position.
// Returns ErrNotFound if the profile does not exist.
func (s *HotkeyStore) AddButton(ctx context.Context, profileID string, b HotkeyButton) (HotkeyButton, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM hotkey_profiles WHERE id = ?`, profileID,
	).Scan(&exists); err != nil || exists == 0 {
		return HotkeyButton{}, ErrNotFound
	}

	var maxPos sql.NullInt64
	_ = s.db.QueryRowContext(ctx,
		`SELECT MAX(position) FROM hotkey_buttons WHERE profile_id = ?`, profileID,
	).Scan(&maxPos)
	nextPos := int(maxPos.Int64) + 1

	id := newID()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO hotkey_buttons(
			id, profile_id, position, label, sub_label, icon, palette,
			track_id, track_path, track_title, track_artist, track_type, duration_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, profileID, nextPos,
		b.Label, b.SubLabel, b.Icon, b.Palette,
		nullableStr(b.TrackID), b.TrackPath, b.TrackTitle, b.TrackArtist, b.TrackType, b.DurationMS,
	); err != nil {
		return HotkeyButton{}, fmt.Errorf("hotkey add button: %w", err)
	}

	_, _ = s.db.ExecContext(ctx,
		`UPDATE hotkey_profiles SET updated_at = datetime('now') WHERE id = ?`, profileID)

	return s.findButton(ctx, id)
}

// PatchButton updates the non-nil fields of a button.
// Returns ErrNotFound if the button does not exist.
func (s *HotkeyStore) PatchButton(ctx context.Context, buttonID string, patch HotkeyButtonPatch) (HotkeyButton, error) {
	var setClauses []string
	var args []any

	if patch.Label != nil {
		setClauses = append(setClauses, "label = ?")
		args = append(args, *patch.Label)
	}
	if patch.SubLabel != nil {
		setClauses = append(setClauses, "sub_label = ?")
		args = append(args, *patch.SubLabel)
	}
	if patch.Icon != nil {
		setClauses = append(setClauses, "icon = ?")
		args = append(args, *patch.Icon)
	}
	if patch.Palette != nil {
		setClauses = append(setClauses, "palette = ?")
		args = append(args, *patch.Palette)
	}
	if patch.TrackID != nil {
		setClauses = append(setClauses, "track_id = ?")
		args = append(args, nullableStr(*patch.TrackID))
	}
	if patch.TrackPath != nil {
		setClauses = append(setClauses, "track_path = ?")
		args = append(args, *patch.TrackPath)
	}
	if patch.TrackTitle != nil {
		setClauses = append(setClauses, "track_title = ?")
		args = append(args, *patch.TrackTitle)
	}
	if patch.TrackArtist != nil {
		setClauses = append(setClauses, "track_artist = ?")
		args = append(args, *patch.TrackArtist)
	}
	if patch.TrackType != nil {
		setClauses = append(setClauses, "track_type = ?")
		args = append(args, *patch.TrackType)
	}
	if patch.DurationMS != nil {
		setClauses = append(setClauses, "duration_ms = ?")
		args = append(args, *patch.DurationMS)
	}
	if patch.Position != nil {
		setClauses = append(setClauses, "position = ?")
		args = append(args, *patch.Position)
	}
	if len(setClauses) == 0 {
		return s.findButton(ctx, buttonID)
	}
	args = append(args, buttonID)

	res, err := s.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE hotkey_buttons SET %s WHERE id = ?",
			strings.Join(setClauses, ", ")),
		args...,
	)
	if err != nil {
		return HotkeyButton{}, fmt.Errorf("hotkey patch button: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return HotkeyButton{}, ErrNotFound
	}

	btn, err := s.findButton(ctx, buttonID)
	if err != nil {
		return HotkeyButton{}, err
	}
	_, _ = s.db.ExecContext(ctx,
		`UPDATE hotkey_profiles SET updated_at = datetime('now') WHERE id = ?`, btn.ProfileID)
	return btn, nil
}

// DeleteButton removes a button by ID. Idempotent.
func (s *HotkeyStore) DeleteButton(ctx context.Context, buttonID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM hotkey_buttons WHERE id = ?`, buttonID)
	return err
}

// ReorderButtons sets the position of each button in the given order.
// All IDs must belong to profileID.
func (s *HotkeyStore) ReorderButtons(ctx context.Context, profileID string, buttonIDs []string) error {
	if len(buttonIDs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("hotkey reorder: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM hotkey_buttons WHERE profile_id = ?`, profileID)
	if err != nil {
		return fmt.Errorf("hotkey reorder: fetch buttons: %w", err)
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

	for _, id := range buttonIDs {
		if _, ok := existing[id]; !ok {
			return fmt.Errorf("hotkey reorder: button %q does not belong to profile", id)
		}
	}
	for i, id := range buttonIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE hotkey_buttons SET position = ? WHERE id = ?`, i+1, id); err != nil {
			return fmt.Errorf("hotkey reorder: update position: %w", err)
		}
	}
	_, _ = tx.ExecContext(ctx,
		`UPDATE hotkey_profiles SET updated_at = datetime('now') WHERE id = ?`, profileID)

	return tx.Commit()
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (s *HotkeyStore) listButtons(ctx context.Context, profileID string) ([]HotkeyButton, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT hb.id, hb.profile_id, hb.position, hb.label, hb.sub_label, hb.icon, hb.palette,
		       COALESCE(hb.track_id,''), hb.track_path, hb.track_title, hb.track_artist, hb.track_type,
		       hb.duration_ms, hb.created_at,
		       t.loudness_lufs,
		       t.cue_in_ms, t.intro_ms, t.outro_ms, t.cue_out_ms
		FROM hotkey_buttons hb
		LEFT JOIN tracks t ON t.id = hb.track_id
		WHERE hb.profile_id = ?
		ORDER BY hb.position ASC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("hotkey list buttons: %w", err)
	}
	defer rows.Close()

	var out []HotkeyButton
	for rows.Next() {
		b, err := scanButtonWithLoudness(rows)
		if err != nil {
			return nil, fmt.Errorf("hotkey list buttons scan: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *HotkeyStore) findButton(ctx context.Context, id string) (HotkeyButton, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, profile_id, position, label, sub_label, icon, palette,
		       COALESCE(track_id,''), track_path, track_title, track_artist, track_type,
		       duration_ms, created_at
		FROM hotkey_buttons WHERE id = ?`, id)

	var b HotkeyButton
	var createdAt string
	err := row.Scan(
		&b.ID, &b.ProfileID, &b.Position, &b.Label, &b.SubLabel, &b.Icon, &b.Palette,
		&b.TrackID, &b.TrackPath, &b.TrackTitle, &b.TrackArtist, &b.TrackType,
		&b.DurationMS, &createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return HotkeyButton{}, ErrNotFound
	}
	if err != nil {
		return HotkeyButton{}, fmt.Errorf("hotkey find button: %w", err)
	}
	b.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	return b, nil
}

type buttonScanner interface {
	Scan(dest ...any) error
}

func scanProfile(rows *sql.Rows) (HotkeyProfile, error) {
	var p HotkeyProfile
	var createdAt, updatedAt string
	if err := rows.Scan(&p.ID, &p.Name, &p.Columns, &createdAt, &updatedAt); err != nil {
		return HotkeyProfile{}, err
	}
	p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
	return p, nil
}

func scanButton(sc buttonScanner) (HotkeyButton, error) {
	var b HotkeyButton
	var createdAt string
	if err := sc.Scan(
		&b.ID, &b.ProfileID, &b.Position, &b.Label, &b.SubLabel, &b.Icon, &b.Palette,
		&b.TrackID, &b.TrackPath, &b.TrackTitle, &b.TrackArtist, &b.TrackType,
		&b.DurationMS, &createdAt,
	); err != nil {
		return HotkeyButton{}, err
	}
	b.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	return b, nil
}

func scanButtonWithLoudness(sc buttonScanner) (HotkeyButton, error) {
	var b HotkeyButton
	var createdAt string
	var lufs sql.NullFloat64
	var cueIn, intro, outro, cueOut sql.NullInt64
	if err := sc.Scan(
		&b.ID, &b.ProfileID, &b.Position, &b.Label, &b.SubLabel, &b.Icon, &b.Palette,
		&b.TrackID, &b.TrackPath, &b.TrackTitle, &b.TrackArtist, &b.TrackType,
		&b.DurationMS, &createdAt, &lufs,
		&cueIn, &intro, &outro, &cueOut,
	); err != nil {
		return HotkeyButton{}, err
	}
	b.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	if lufs.Valid {
		b.LoudnessLUFS = &lufs.Float64
	}
	if cueIn.Valid {
		b.CueInMS = &cueIn.Int64
	}
	if intro.Valid {
		b.IntroMS = &intro.Int64
	}
	if outro.Valid {
		b.OutroMS = &outro.Int64
	}
	if cueOut.Valid {
		b.CueOutMS = &cueOut.Int64
	}
	return b, nil
}
