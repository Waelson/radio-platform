package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SeparationRule defines how long (in minutes) to wait before repeating
// the same artist, title, category or album.
type SeparationRule struct {
	ID            string
	Field         string // artist | title | category | album
	MinSepMinutes int
}

// SeparationRuleStore manages separation_rules in SQLite.
type SeparationRuleStore struct {
	db *sql.DB
}

// NewSeparationRuleStore creates a SeparationRuleStore backed by db.
func NewSeparationRuleStore(db *sql.DB) *SeparationRuleStore {
	return &SeparationRuleStore{db: db}
}

// List returns all separation rules.
func (s *SeparationRuleStore) List(ctx context.Context) ([]SeparationRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, field, min_sep_minutes FROM separation_rules ORDER BY field ASC`)
	if err != nil {
		return nil, fmt.Errorf("separation rules list: %w", err)
	}
	defer rows.Close()

	var out []SeparationRule
	for rows.Next() {
		var r SeparationRule
		if err := rows.Scan(&r.ID, &r.Field, &r.MinSepMinutes); err != nil {
			return nil, fmt.Errorf("separation rules list scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Create inserts a new separation rule.
func (s *SeparationRuleStore) Create(ctx context.Context, field string, minSepMinutes int) (SeparationRule, error) {
	validFields := map[string]bool{"artist": true, "title": true, "category": true, "album": true}
	if !validFields[field] {
		return SeparationRule{}, fmt.Errorf("invalid field %q; must be one of: artist, title, category, album", field)
	}
	if minSepMinutes <= 0 {
		return SeparationRule{}, fmt.Errorf("min_sep_minutes must be positive")
	}
	id := newID()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO separation_rules(id, field, min_sep_minutes) VALUES (?, ?, ?)`,
		id, field, minSepMinutes,
	); err != nil {
		return SeparationRule{}, fmt.Errorf("separation rule create: %w", err)
	}
	return SeparationRule{ID: id, Field: field, MinSepMinutes: minSepMinutes}, nil
}

// Update replaces the field and min_sep_minutes of a rule.
func (s *SeparationRuleStore) Update(ctx context.Context, id, field string, minSepMinutes int) error {
	validFields := map[string]bool{"artist": true, "title": true, "category": true, "album": true}
	if !validFields[field] {
		return fmt.Errorf("invalid field %q", field)
	}
	if minSepMinutes <= 0 {
		return fmt.Errorf("min_sep_minutes must be positive")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE separation_rules SET field = ?, min_sep_minutes = ? WHERE id = ?`,
		field, minSepMinutes, id)
	if err != nil {
		return fmt.Errorf("separation rule update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a separation rule.
func (s *SeparationRuleStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM separation_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("separation rule delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Get returns a single rule by ID.
func (s *SeparationRuleStore) Get(ctx context.Context, id string) (SeparationRule, error) {
	var r SeparationRule
	err := s.db.QueryRowContext(ctx,
		`SELECT id, field, min_sep_minutes FROM separation_rules WHERE id = ?`, id,
	).Scan(&r.ID, &r.Field, &r.MinSepMinutes)
	if errors.Is(err, sql.ErrNoRows) {
		return SeparationRule{}, ErrNotFound
	}
	return r, err
}
