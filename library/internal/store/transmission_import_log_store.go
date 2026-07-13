package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TransmissionImportLogEntry is one row in transmission_import_log.
type TransmissionImportLogEntry struct {
	ID              string
	FileName        string
	StartedAt       time.Time
	FinishedAt      *time.Time
	Status          string // running|success|failed
	RecordsTotal    int
	RecordsImported int
	ErrorMessage    string
}

// TransmissionImportLogStore manages the transmission_import_log table.
type TransmissionImportLogStore struct {
	db *sql.DB
}

// NewTransmissionImportLogStore creates a TransmissionImportLogStore backed by db.
func NewTransmissionImportLogStore(db *sql.DB) *TransmissionImportLogStore {
	return &TransmissionImportLogStore{db: db}
}

// StartImport inserts a new row with status=running and returns its id.
// Must be called before processing a file.
func (s *TransmissionImportLogStore) StartImport(ctx context.Context, fileName string) (string, error) {
	id := newID()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO transmission_import_log
			(id, file_name, started_at, status)
		VALUES (?, ?, datetime('now'), 'running')
	`, id, fileName)
	if err != nil {
		return "", fmt.Errorf("import_log start %q: %w", fileName, err)
	}
	return id, nil
}

// FinishImport marks an import as successful and records the record counts.
func (s *TransmissionImportLogStore) FinishImport(ctx context.Context, id string, recordsTotal, recordsImported int) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE transmission_import_log
		SET status = 'success',
		    finished_at = datetime('now'),
		    records_total = ?,
		    records_imported = ?,
		    error_message = ''
		WHERE id = ?
	`, recordsTotal, recordsImported, id)
	if err != nil {
		return fmt.Errorf("import_log finish %q: %w", id, err)
	}
	return nil
}

// FailImport marks an import as failed and records the error message.
func (s *TransmissionImportLogStore) FailImport(ctx context.Context, id string, recordsTotal int, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE transmission_import_log
		SET status = 'failed',
		    finished_at = datetime('now'),
		    records_total = ?,
		    error_message = ?
		WHERE id = ?
	`, recordsTotal, errMsg, id)
	if err != nil {
		return fmt.Errorf("import_log fail %q: %w", id, err)
	}
	return nil
}

// List returns import log entries ordered by started_at DESC with pagination.
func (s *TransmissionImportLogStore) List(ctx context.Context, limit, offset int) ([]TransmissionImportLogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, file_name, started_at, finished_at,
		       status, records_total, records_imported, error_message
		FROM transmission_import_log
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("import_log list: %w", err)
	}
	defer rows.Close()

	var out []TransmissionImportLogEntry
	for rows.Next() {
		var e TransmissionImportLogEntry
		var startedAt string
		var finishedAt sql.NullString
		if err := rows.Scan(
			&e.ID, &e.FileName, &startedAt, &finishedAt,
			&e.Status, &e.RecordsTotal, &e.RecordsImported, &e.ErrorMessage,
		); err != nil {
			return nil, fmt.Errorf("import_log list scan: %w", err)
		}
		e.StartedAt, _ = time.Parse("2006-01-02 15:04:05", startedAt)
		if e.StartedAt.IsZero() {
			e.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		}
		if finishedAt.Valid && finishedAt.String != "" {
			t, _ := time.Parse("2006-01-02 15:04:05", finishedAt.String)
			if t.IsZero() {
				t, _ = time.Parse(time.RFC3339, finishedAt.String)
			}
			e.FinishedAt = &t
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
