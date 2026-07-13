package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Waelson/radio-library-service/internal/store"
)

func TestMigration005_TransmissionLog(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Verify isrc/composer/publisher columns exist in tracks.
	for _, col := range []string{"isrc", "composer", "publisher"} {
		var v sql.NullString
		err := db.QueryRowContext(ctx,
			"SELECT "+col+" FROM tracks WHERE 1=0",
		).Scan(&v)
		if err != nil && err != sql.ErrNoRows {
			t.Errorf("tracks.%s not found: %v", col, err)
		}
	}

	// Verify transmission_log table exists.
	var n int
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='transmission_log'",
	).Scan(&n)
	if err != nil || n != 1 {
		t.Errorf("transmission_log table not found (n=%d, err=%v)", n, err)
	}
}

func TestMigration006_ImportLog(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var n int
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='transmission_import_log'",
	).Scan(&n)
	if err != nil || n != 1 {
		t.Errorf("transmission_import_log table not found (n=%d, err=%v)", n, err)
	}
}

func TestMigration007_Settings(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Verify settings table and default values.
	keys := []string{
		"transmission_log.dir",
		"transmission_log.file_name_template",
		"transmission_log.poll_interval",
		"transmission_log.grace_period",
		"transmission_log.retention_days",
		"station.name",
		"station.cnpj",
		"station.frequency",
		"station.type",
		"station.city",
		"station.state",
	}
	for _, key := range keys {
		var value string
		err := db.QueryRowContext(ctx,
			"SELECT value FROM settings WHERE key = ?", key,
		).Scan(&value)
		if err != nil {
			t.Errorf("settings key %q not found: %v", key, err)
		}
	}
}
