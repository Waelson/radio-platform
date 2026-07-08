// Package store manages the SQLite database for the Library Service.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial.sql
var migration001 string

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

// migrate applies all migrations in order, tracking each one in _migrations.
func migrate(ctx context.Context, db *sql.DB) error {
	// Apply base schema (idempotent via IF NOT EXISTS).
	if _, err := db.ExecContext(ctx, migration001); err != nil {
		return fmt.Errorf("apply schema 001: %w", err)
	}

	// Migration tracking table — created once, never dropped.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			name       TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}

	// 001_album_column: add album to existing databases (idempotent).
	if !migrationDone(ctx, db, "001_album_column") {
		if _, err := db.ExecContext(ctx,
			`ALTER TABLE tracks ADD COLUMN album TEXT NOT NULL DEFAULT ''`,
		); err != nil {
			// Ignore "duplicate column" — DB already has the column.
			return fmt.Errorf("001_album_column: %w", err)
		}
		if err := markMigration(ctx, db, "001_album_column"); err != nil {
			return err
		}
	}

	// 002_advanced_search: add EFEITOS type + album/title indexes.
	if !migrationDone(ctx, db, "002_advanced_search") {
		if err := applyMigration002(ctx, db); err != nil {
			return fmt.Errorf("002_advanced_search: %w", err)
		}
		if err := markMigration(ctx, db, "002_advanced_search"); err != nil {
			return err
		}
	}

	return nil
}

// applyMigration002 recreates the tracks table to add EFEITOS to the CHECK
// constraint and adds album/title indexes for the advanced-search feature.
// Foreign keys are disabled for the duration of the table recreation and
// re-enabled afterwards; data integrity is preserved because all track IDs
// are copied verbatim before the old table is dropped.
func applyMigration002(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		return fmt.Errorf("disable FK: %w", err)
	}
	defer db.ExecContext(ctx, `PRAGMA foreign_keys=ON`) //nolint:errcheck

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	steps := []string{
		// New table with expanded CHECK constraint and album column always present.
		`CREATE TABLE tracks_new (
			id          TEXT PRIMARY KEY,
			path        TEXT NOT NULL UNIQUE,
			title       TEXT NOT NULL DEFAULT '',
			artist      TEXT NOT NULL DEFAULT '',
			album       TEXT NOT NULL DEFAULT '',
			type        TEXT NOT NULL CHECK(type IN ('MUSIC','VINHETA','JINGLE','SPOT','EFEITOS')),
			duration_ms INTEGER NOT NULL DEFAULT 0,
			category    TEXT,
			indexed_at  DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		// Copy all rows; COALESCE handles DBs where album column may be NULL.
		`INSERT INTO tracks_new(id, path, title, artist, album, type, duration_ms, category, indexed_at)
		 SELECT id, path, title, artist, COALESCE(album,''), type, duration_ms, category, indexed_at
		 FROM tracks`,
		`DROP TABLE tracks`,
		`ALTER TABLE tracks_new RENAME TO tracks`,
		// Restore existing indexes.
		`CREATE INDEX IF NOT EXISTS idx_tracks_type     ON tracks(type)`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_artist   ON tracks(artist)`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_category ON tracks(category)`,
		// New indexes for advanced search.
		`CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album)`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_title ON tracks(title)`,
	}

	for _, stmt := range steps {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("step %q: %w", stmt[:min(50, len(stmt))], err)
		}
	}

	return tx.Commit()
}

// migrationDone reports whether a named migration has already been applied.
func migrationDone(ctx context.Context, db *sql.DB, name string) bool {
	var n int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM _migrations WHERE name = ?`, name,
	).Scan(&n)
	return n > 0
}

// markMigration records that a named migration has been applied.
func markMigration(ctx context.Context, db *sql.DB, name string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO _migrations(name) VALUES (?)`, name,
	)
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
