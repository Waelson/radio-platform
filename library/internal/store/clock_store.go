package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Clock represents a programming template (60-minute format clock).
type Clock struct {
	ID        string
	Name      string
	Slots     []ClockSlot
	CreatedAt time.Time
}

// ClockSlot is an ordered slot within a clock.
type ClockSlot struct {
	ID              string
	ClockID         string
	Position        int
	SlotType        string // CATEGORY|JINGLE|SPOT|VINHETA|HORA_CERTA|FIXED
	CategoryID      string // non-empty when SlotType==CATEGORY
	CategoryName    string // joined from categories table
	FixedTrackID    string // non-empty when SlotType==FIXED
	DurationHintMS  int64
}

// ScheduleCell represents one cell in the 24×7 clock grid.
type ScheduleCell struct {
	Weekday   int
	Hour      int
	ClockID   string // empty when no clock assigned
	ClockName string
}

// ClockStore manages clocks, clock_slots and clock_schedule in SQLite.
type ClockStore struct {
	db *sql.DB
}

// NewClockStore creates a ClockStore backed by db.
func NewClockStore(db *sql.DB) *ClockStore {
	return &ClockStore{db: db}
}

// ── Clocks ────────────────────────────────────────────────────────────────────

// List returns all clocks without their slots.
func (s *ClockStore) List(ctx context.Context) ([]Clock, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, created_at FROM clocks ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("clock list: %w", err)
	}
	defer rows.Close()

	var out []Clock
	for rows.Next() {
		c, err := scanClockRow(rows)
		if err != nil {
			return nil, fmt.Errorf("clock list scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Create inserts a new clock and returns it (with empty Slots).
func (s *ClockStore) Create(ctx context.Context, name string) (Clock, error) {
	if strings.TrimSpace(name) == "" {
		return Clock{}, fmt.Errorf("clock name must not be empty")
	}
	id := newID()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO clocks(id, name) VALUES (?, ?)`, id, name,
	); err != nil {
		return Clock{}, fmt.Errorf("clock create: %w", err)
	}
	return s.Get(ctx, id)
}

// Get returns a clock with its slots, or ErrNotFound.
func (s *ClockStore) Get(ctx context.Context, id string) (Clock, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, created_at FROM clocks WHERE id = ?`, id)
	c, err := scanClockSingleRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Clock{}, ErrNotFound
	}
	if err != nil {
		return Clock{}, err
	}
	slots, err := s.listSlots(ctx, id)
	if err != nil {
		return Clock{}, err
	}
	c.Slots = slots
	return c, nil
}

// Update changes the name of a clock.
func (s *ClockStore) Update(ctx context.Context, id, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("clock name must not be empty")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE clocks SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("clock update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a clock and its slots (CASCADE). Returns error if clock is
// referenced by any clock_schedule cell.
func (s *ClockStore) Delete(ctx context.Context, id string) error {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM clock_schedule WHERE clock_id = ?`, id,
	).Scan(&n); err != nil {
		return fmt.Errorf("clock delete check schedule: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("clock is used in %d schedule cell(s); clear the grid first", n)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM clocks WHERE id = ?`, id); err != nil {
		return fmt.Errorf("clock delete: %w", err)
	}
	return nil
}

// ── Slots ─────────────────────────────────────────────────────────────────────

// AddSlot appends a slot to a clock at the next available position.
func (s *ClockStore) AddSlot(ctx context.Context, clockID string, slot ClockSlot) (ClockSlot, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM clocks WHERE id = ?`, clockID,
	).Scan(&exists); err != nil || exists == 0 {
		return ClockSlot{}, ErrNotFound
	}

	var maxPos sql.NullInt64
	_ = s.db.QueryRowContext(ctx,
		`SELECT MAX(position) FROM clock_slots WHERE clock_id = ?`, clockID,
	).Scan(&maxPos)
	nextPos := int(maxPos.Int64) + 1

	id := newID()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO clock_slots(id, clock_id, position, slot_type, category_id, fixed_track_id, duration_hint_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, clockID, nextPos, slot.SlotType,
		nullableStr(slot.CategoryID), nullableStr(slot.FixedTrackID), slot.DurationHintMS,
	); err != nil {
		return ClockSlot{}, fmt.Errorf("clock add slot: %w", err)
	}
	return s.findSlot(ctx, id)
}

// UpdateSlot updates the fields of an existing slot.
func (s *ClockStore) UpdateSlot(ctx context.Context, slotID string, slot ClockSlot) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE clock_slots
		SET slot_type = ?, category_id = ?, fixed_track_id = ?, duration_hint_ms = ?
		WHERE id = ?`,
		slot.SlotType, nullableStr(slot.CategoryID), nullableStr(slot.FixedTrackID),
		slot.DurationHintMS, slotID,
	)
	if err != nil {
		return fmt.Errorf("clock update slot: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSlot removes a slot and re-compacts positions within the clock.
func (s *ClockStore) DeleteSlot(ctx context.Context, slotID string) error {
	var clockID string
	var pos int
	if err := s.db.QueryRowContext(ctx,
		`SELECT clock_id, position FROM clock_slots WHERE id = ?`, slotID,
	).Scan(&clockID, &pos); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("clock delete slot: find: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("clock delete slot: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM clock_slots WHERE id = ?`, slotID); err != nil {
		return fmt.Errorf("clock delete slot: %w", err)
	}
	// Compact positions after the deleted slot.
	if _, err := tx.ExecContext(ctx, `
		UPDATE clock_slots SET position = position - 1
		WHERE clock_id = ? AND position > ?`, clockID, pos,
	); err != nil {
		return fmt.Errorf("clock delete slot: compact positions: %w", err)
	}
	return tx.Commit()
}

// ReorderSlots reassigns positions according to the supplied ordered list of slot IDs.
// All IDs must belong to clockID.
func (s *ClockStore) ReorderSlots(ctx context.Context, clockID string, orderedSlotIDs []string) error {
	if len(orderedSlotIDs) == 0 {
		return nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM clock_slots WHERE clock_id = ?`, clockID)
	if err != nil {
		return fmt.Errorf("clock reorder slots: fetch: %w", err)
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

	for _, id := range orderedSlotIDs {
		if _, ok := existing[id]; !ok {
			return fmt.Errorf("clock reorder slots: slot %q does not belong to clock", id)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("clock reorder slots: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// First pass: set negative positions to avoid UNIQUE constraint conflicts.
	for i, id := range orderedSlotIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE clock_slots SET position = ? WHERE id = ?`, -(i + 1), id,
		); err != nil {
			return fmt.Errorf("clock reorder slots: temp update %q: %w", id, err)
		}
	}
	// Second pass: set final positive positions.
	for i, id := range orderedSlotIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE clock_slots SET position = ? WHERE id = ?`, i+1, id,
		); err != nil {
			return fmt.Errorf("clock reorder slots: update %q: %w", id, err)
		}
	}
	return tx.Commit()
}

// ── Grid ─────────────────────────────────────────────────────────────────────

// GetGrid returns all populated cells in the 24×7 schedule grid.
func (s *ClockStore) GetGrid(ctx context.Context) ([]ScheduleCell, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cs.weekday, cs.hour, COALESCE(cs.clock_id,''), COALESCE(c.name,'')
		FROM clock_schedule cs
		LEFT JOIN clocks c ON c.id = cs.clock_id
		ORDER BY cs.weekday ASC, cs.hour ASC`)
	if err != nil {
		return nil, fmt.Errorf("clock grid get: %w", err)
	}
	defer rows.Close()

	var out []ScheduleCell
	for rows.Next() {
		var cell ScheduleCell
		if err := rows.Scan(&cell.Weekday, &cell.Hour, &cell.ClockID, &cell.ClockName); err != nil {
			return nil, fmt.Errorf("clock grid scan: %w", err)
		}
		out = append(out, cell)
	}
	return out, rows.Err()
}

// SetGridCells upserts one or more cells in the schedule grid.
// A cell with ClockID == "" clears that cell (sets clock_id to NULL).
func (s *ClockStore) SetGridCells(ctx context.Context, cells []ScheduleCell) error {
	if len(cells) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("clock set grid cells: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, cell := range cells {
		clockID := nullableStr(cell.ClockID)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO clock_schedule(weekday, hour, clock_id) VALUES (?, ?, ?)
			ON CONFLICT(weekday, hour) DO UPDATE SET clock_id = excluded.clock_id`,
			cell.Weekday, cell.Hour, clockID,
		); err != nil {
			return fmt.Errorf("clock set grid cell (%d,%d): %w", cell.Weekday, cell.Hour, err)
		}
	}
	return tx.Commit()
}

// GetClockForHour returns the clock assigned to a specific weekday+hour, or nil if none.
func (s *ClockStore) GetClockForHour(ctx context.Context, weekday, hour int) (*Clock, error) {
	var clockID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT clock_id FROM clock_schedule WHERE weekday = ? AND hour = ?`,
		weekday, hour,
	).Scan(&clockID)
	if errors.Is(err, sql.ErrNoRows) || !clockID.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get clock for hour: %w", err)
	}
	c, err := s.Get(ctx, clockID.String)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (s *ClockStore) listSlots(ctx context.Context, clockID string) ([]ClockSlot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cs.id, cs.clock_id, cs.position, cs.slot_type,
		       COALESCE(cs.category_id,''), COALESCE(cat.name,''),
		       COALESCE(cs.fixed_track_id,''), cs.duration_hint_ms
		FROM clock_slots cs
		LEFT JOIN categories cat ON cat.id = cs.category_id
		WHERE cs.clock_id = ?
		ORDER BY cs.position ASC`, clockID)
	if err != nil {
		return nil, fmt.Errorf("clock list slots: %w", err)
	}
	defer rows.Close()

	var out []ClockSlot
	for rows.Next() {
		slot, err := scanSlotRow(rows)
		if err != nil {
			return nil, fmt.Errorf("clock list slots scan: %w", err)
		}
		out = append(out, slot)
	}
	return out, rows.Err()
}

func (s *ClockStore) findSlot(ctx context.Context, id string) (ClockSlot, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT cs.id, cs.clock_id, cs.position, cs.slot_type,
		       COALESCE(cs.category_id,''), COALESCE(cat.name,''),
		       COALESCE(cs.fixed_track_id,''), cs.duration_hint_ms
		FROM clock_slots cs
		LEFT JOIN categories cat ON cat.id = cs.category_id
		WHERE cs.id = ?`, id)

	var slot ClockSlot
	err := row.Scan(
		&slot.ID, &slot.ClockID, &slot.Position, &slot.SlotType,
		&slot.CategoryID, &slot.CategoryName,
		&slot.FixedTrackID, &slot.DurationHintMS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ClockSlot{}, ErrNotFound
	}
	return slot, err
}

type clockRowScanner interface {
	Scan(dest ...any) error
}

func scanClockRow(rows *sql.Rows) (Clock, error) {
	var c Clock
	var createdAt string
	if err := rows.Scan(&c.ID, &c.Name, &createdAt); err != nil {
		return Clock{}, err
	}
	c.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	return c, nil
}

func scanClockSingleRow(row clockRowScanner) (Clock, error) {
	var c Clock
	var createdAt string
	if err := row.Scan(&c.ID, &c.Name, &createdAt); err != nil {
		return Clock{}, err
	}
	c.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	return c, nil
}

func scanSlotRow(rows *sql.Rows) (ClockSlot, error) {
	var slot ClockSlot
	return slot, rows.Scan(
		&slot.ID, &slot.ClockID, &slot.Position, &slot.SlotType,
		&slot.CategoryID, &slot.CategoryName,
		&slot.FixedTrackID, &slot.DurationHintMS,
	)
}
