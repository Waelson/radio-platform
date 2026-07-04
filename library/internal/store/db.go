// Package store manages the SQLite database for the Library Service.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial.sql
var schema string

// Open opens (or creates) the SQLite database at path, applies required PRAGMAs,
// runs migrations and returns a ready-to-use *sql.DB.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}

	// Single writer to avoid SQLITE_BUSY under concurrent HTTP requests.
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, `
		PRAGMA journal_mode=WAL;
		PRAGMA foreign_keys=ON;
		PRAGMA busy_timeout=5000;
	`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: apply pragmas: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}

	return db, nil
}

// migrate executes the embedded SQL schema (idempotent via IF NOT EXISTS) and
// applies incremental column additions.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	// Add album column to existing databases (idempotent: ignore duplicate column error).
	_, err := db.ExecContext(ctx, `ALTER TABLE tracks ADD COLUMN album TEXT NOT NULL DEFAULT ''`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("add album column: %w", err)
	}
	return nil
}
