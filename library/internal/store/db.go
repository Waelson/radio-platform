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
var migration001 string

//go:embed migrations/003_hotkeys.sql
var migration003 string

//go:embed migrations/004_clock_rotation.sql
var migration004 string

//go:embed migrations/005_transmission_log.sql
var migration005 string

//go:embed migrations/006_transmission_import_log.sql
var migration006 string

//go:embed migrations/007_settings.sql
var migration007 string

//go:embed migrations/008_transmission_log_engine_id.sql
var migration008 string

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

	// 003_hotkeys: hotkey_profiles and hotkey_buttons tables.
	if !migrationDone(ctx, db, "003_hotkeys") {
		if _, err := db.ExecContext(ctx, migration003); err != nil {
			return fmt.Errorf("003_hotkeys: %w", err)
		}
		if err := markMigration(ctx, db, "003_hotkeys"); err != nil {
			return err
		}
	}

	// 004_clock_rotation: categories, clocks, clock_slots, clock_schedule,
	// separation_rules, rotation_log tables.
	if !migrationDone(ctx, db, "004_clock_rotation") {
		if _, err := db.ExecContext(ctx, migration004); err != nil {
			return fmt.Errorf("004_clock_rotation: %w", err)
		}
		if err := markMigration(ctx, db, "004_clock_rotation"); err != nil {
			return err
		}
	}

	// 005_transmission_log: isrc/composer/publisher em tracks + tabela transmission_log.
	if !migrationDone(ctx, db, "005_transmission_log") {
		if err := applyMigration005(ctx, db); err != nil {
			return fmt.Errorf("005_transmission_log: %w", err)
		}
		if err := markMigration(ctx, db, "005_transmission_log"); err != nil {
			return err
		}
	}

	// 006_transmission_import_log: registro de tentativas de importação.
	if !migrationDone(ctx, db, "006_transmission_import_log") {
		if _, err := db.ExecContext(ctx, migration006); err != nil {
			return fmt.Errorf("006_transmission_import_log: %w", err)
		}
		if err := markMigration(ctx, db, "006_transmission_import_log"); err != nil {
			return err
		}
	}

	// 007_settings: tabela key→value com valores padrão de configuração.
	if !migrationDone(ctx, db, "007_settings") {
		if _, err := db.ExecContext(ctx, migration007); err != nil {
			return fmt.Errorf("007_settings: %w", err)
		}
		if err := markMigration(ctx, db, "007_settings"); err != nil {
			return err
		}
	}

	// 008_transmission_log_engine_id: adiciona engine_id à tabela transmission_log.
	if !migrationDone(ctx, db, "008_transmission_log_engine_id") {
		if _, err := db.ExecContext(ctx, migration008); err != nil {
			if !isDuplicateColumn(err) {
				return fmt.Errorf("008_transmission_log_engine_id: %w", err)
			}
		}
		if err := markMigration(ctx, db, "008_transmission_log_engine_id"); err != nil {
			return err
		}
	}

	return nil
}

// applyMigration005 adds isrc, composer, publisher to tracks and creates the
// transmission_log table. Each ALTER TABLE is applied individually so that a
// "duplicate column" error (partial previous run) is safely ignored.
func applyMigration005(ctx context.Context, db *sql.DB) error {
	// Apply each ALTER TABLE separately — SQLite has no ADD COLUMN IF NOT EXISTS.
	for _, col := range []string{"isrc", "composer", "publisher"} {
		stmt := fmt.Sprintf("ALTER TABLE tracks ADD COLUMN %s TEXT NOT NULL DEFAULT ''", col)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			// Ignore "duplicate column name" — column already exists from a
			// partial previous run.
			if !isDuplicateColumn(err) {
				return fmt.Errorf("add column %s: %w", col, err)
			}
		}
	}

	// Create transmission_log table and indexes (all idempotent via IF NOT EXISTS).
	createStmts := []string{
		`CREATE TABLE IF NOT EXISTS transmission_log (
			id                 TEXT     PRIMARY KEY,
			queue_item_id      TEXT     NOT NULL DEFAULT '' UNIQUE,
			asset_id           TEXT     NOT NULL DEFAULT '',
			path               TEXT     NOT NULL DEFAULT '',
			title              TEXT     NOT NULL DEFAULT '',
			artist             TEXT     NOT NULL DEFAULT '',
			type               TEXT     NOT NULL DEFAULT '',
			isrc               TEXT     NOT NULL DEFAULT '',
			composer           TEXT     NOT NULL DEFAULT '',
			publisher          TEXT     NOT NULL DEFAULT '',
			duration_ms        INTEGER  NOT NULL DEFAULT 0,
			duration_played_ms INTEGER  NOT NULL DEFAULT 0,
			result             TEXT     NOT NULL DEFAULT '',
			status             TEXT     NOT NULL DEFAULT 'FINISHED',
			started_at         DATETIME NOT NULL,
			finished_at        DATETIME,
			break_id           TEXT     NOT NULL DEFAULT '',
			break_title        TEXT     NOT NULL DEFAULT '',
			break_role         TEXT     NOT NULL DEFAULT '',
			break_position     INTEGER  NOT NULL DEFAULT 0,
			import_file_name   TEXT     NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transmission_log_started_at ON transmission_log(started_at)`,
		`CREATE INDEX IF NOT EXISTS idx_transmission_log_type       ON transmission_log(type)`,
		`CREATE INDEX IF NOT EXISTS idx_transmission_log_status     ON transmission_log(status)`,
		`CREATE INDEX IF NOT EXISTS idx_transmission_log_asset_id   ON transmission_log(asset_id)`,
	}
	for _, stmt := range createStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("transmission_log schema: %w", err)
		}
	}
	return nil
}

// isDuplicateColumn reports whether err is a SQLite "duplicate column name" error.
func isDuplicateColumn(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
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
