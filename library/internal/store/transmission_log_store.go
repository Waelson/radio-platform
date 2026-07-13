package store

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"
)

// TransmissionLogEntry is one row in transmission_log.
type TransmissionLogEntry struct {
	ID               string
	EngineID         string // playout engine instance that produced the entry
	QueueItemID      string
	AssetID          string
	Path             string
	Title            string
	Artist           string
	Type             string // MUSIC|JINGLE|VINHETA|SPOT|CART
	ISRC             string
	Composer         string
	Publisher        string
	DurationMS       int64
	DurationPlayedMS int64
	Result           string // finished|skipped|failed
	Status           string // FINISHED
	StartedAt        time.Time
	FinishedAt       *time.Time
	BreakID          string
	BreakTitle       string
	BreakRole        string // open|spot|close
	BreakPosition    int
	ImportFileName   string // source JSONL file name
}

// TransmissionLogQuery holds filters for listing transmission log entries.
type TransmissionLogQuery struct {
	From   time.Time
	To     time.Time
	Type   string
	Status string
	Search string // matches title or artist
	Limit  int
	Offset int
}

// TransmissionLogSummary aggregates counts for a given day.
type TransmissionLogSummary struct {
	Date          string
	Total         int
	ByType        map[string]int
	TotalPlayedMS int64
}

// TransmissionLogStore manages the transmission_log table.
type TransmissionLogStore struct {
	db *sql.DB
}

// NewTransmissionLogStore creates a TransmissionLogStore backed by db.
func NewTransmissionLogStore(db *sql.DB) *TransmissionLogStore {
	return &TransmissionLogStore{db: db}
}

// BulkInsert inserts entries in a single transaction using INSERT OR IGNORE.
// Duplicate queue_item_id entries are silently skipped (idempotent).
// import_file_name must be set on every entry before calling.
func (s *TransmissionLogStore) BulkInsert(ctx context.Context, entries []TransmissionLogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("transmission_log bulk_insert: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO transmission_log
			(id, engine_id, queue_item_id, asset_id, path, title, artist, type,
			 isrc, composer, publisher,
			 duration_ms, duration_played_ms, result, status,
			 started_at, finished_at,
			 break_id, break_title, break_role, break_position,
			 import_file_name)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`)
	if err != nil {
		return fmt.Errorf("transmission_log bulk_insert: prepare: %w", err)
	}
	defer stmt.Close()

	for _, e := range entries {
		if e.ID == "" {
			e.ID = newID()
		}
		var finishedAt any
		if e.FinishedAt != nil {
			finishedAt = e.FinishedAt.UTC().Format(time.RFC3339)
		}
		status := e.Status
		if status == "" {
			status = "FINISHED"
		}
		if _, err := stmt.ExecContext(ctx,
			e.ID, e.EngineID, e.QueueItemID, e.AssetID, e.Path, e.Title, e.Artist, e.Type,
			e.ISRC, e.Composer, e.Publisher,
			e.DurationMS, e.DurationPlayedMS, e.Result, status,
			e.StartedAt.UTC().Format(time.RFC3339), finishedAt,
			e.BreakID, e.BreakTitle, e.BreakRole, e.BreakPosition,
			e.ImportFileName,
		); err != nil {
			return fmt.Errorf("transmission_log bulk_insert: exec: %w", err)
		}
	}
	return tx.Commit()
}

// List returns entries matching the query, plus the total count (without pagination).
func (s *TransmissionLogStore) List(ctx context.Context, q TransmissionLogQuery) ([]TransmissionLogEntry, int, error) {
	where, args := buildTLWhere(q)

	// Count total.
	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM transmission_log %s`, whereClause(where))
	if err := s.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("transmission_log list count: %w", err)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	listArgs := append(args, limit, q.Offset)
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, engine_id, queue_item_id, asset_id, path, title, artist, type,
		       isrc, composer, publisher,
		       duration_ms, duration_played_ms, result, status,
		       started_at, finished_at,
		       break_id, break_title, break_role, break_position,
		       import_file_name
		FROM transmission_log
		%s
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?`, whereClause(where)), listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("transmission_log list: %w", err)
	}
	defer rows.Close()

	entries, err := scanTLRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// Summary returns aggregate counts for a given UTC day (YYYY-MM-DD).
func (s *TransmissionLogStore) Summary(ctx context.Context, date time.Time) (TransmissionLogSummary, error) {
	day := date.UTC().Format("2006-01-02")
	from := day + "T00:00:00Z"
	to := day + "T23:59:59Z"

	sum := TransmissionLogSummary{
		Date:   day,
		ByType: make(map[string]int),
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT type, COUNT(*), COALESCE(SUM(duration_played_ms), 0)
		FROM transmission_log
		WHERE started_at >= ? AND started_at <= ?
		GROUP BY type
	`, from, to)
	if err != nil {
		return sum, fmt.Errorf("transmission_log summary: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var typ string
		var cnt int
		var ms int64
		if err := rows.Scan(&typ, &cnt, &ms); err != nil {
			return sum, err
		}
		sum.ByType[typ] = cnt
		sum.Total += cnt
		sum.TotalPlayedMS += ms
	}
	return sum, rows.Err()
}

// ExportCSV streams all entries matching the time range as CSV to w.
// Columns: id;started_at;finished_at;duration_played_ms;title;artist;composer;isrc;type;result;status;break_role;path
func (s *TransmissionLogStore) ExportCSV(ctx context.Context, from, to time.Time, w io.Writer) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, started_at, COALESCE(finished_at,''), duration_played_ms,
		       title, artist, composer, isrc, type, result, status, break_role, path
		FROM transmission_log
		WHERE started_at >= ? AND started_at <= ?
		ORDER BY started_at ASC
	`, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("export csv: query: %w", err)
	}
	defer rows.Close()

	cw := csv.NewWriter(w)
	cw.Comma = ';'
	_ = cw.Write([]string{
		"id", "started_at", "finished_at", "duration_played_ms",
		"title", "artist", "composer", "isrc", "type", "result", "status", "break_role", "path",
	})

	for rows.Next() {
		var (
			id, startedAt, finishedAt, title, artist, composer, isrc string
			typ, result, status, breakRole, path                     string
			durationPlayedMS                                         int64
		)
		if err := rows.Scan(
			&id, &startedAt, &finishedAt, &durationPlayedMS,
			&title, &artist, &composer, &isrc, &typ, &result, &status, &breakRole, &path,
		); err != nil {
			return fmt.Errorf("export csv: scan: %w", err)
		}
		_ = cw.Write([]string{
			id, startedAt, finishedAt, fmt.Sprintf("%d", durationPlayedMS),
			title, artist, composer, isrc, typ, result, status, breakRole, path,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("export csv: rows: %w", err)
	}
	cw.Flush()
	return cw.Error()
}

// ExportECAD streams the ECAD declaration CSV for the given period to w.
// Filters automatically: type IN ('MUSIC','JINGLE','VINHETA'), status='FINISHED',
// duration_played_ms > 0. Ordered by started_at ASC.
//
// Format:
//
//	Header line (H): H;NOME;CNPJ;CIDADE;UF;FREQUENCIA;TIPO;PERIODO_INI;PERIODO_FIM
//	Detail lines (D): D;DATA;HORA_INICIO;DURACAO;TITULO;ARTISTA;COMPOSITOR;ISRC;M;R
func (s *TransmissionLogStore) ExportECAD(ctx context.Context, from, to time.Time, station StationInfo, w io.Writer) error {
	// Header line.
	h := fmt.Sprintf("H;%s;%s;%s;%s;%s;%s;%s;%s\n",
		escECAD(station.Name),
		escECAD(station.CNPJ),
		escECAD(station.City),
		escECAD(station.State),
		escECAD(station.Frequency),
		escECAD(station.Type),
		from.UTC().Format("02/01/2006"),
		to.UTC().Format("02/01/2006"),
	)
	if _, err := io.WriteString(w, h); err != nil {
		return err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT started_at, duration_played_ms, title, artist, composer, isrc
		FROM transmission_log
		WHERE type IN ('MUSIC','JINGLE','VINHETA')
		  AND status = 'FINISHED'
		  AND duration_played_ms > 0
		  AND started_at >= ?
		  AND started_at <= ?
		ORDER BY started_at ASC
	`, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("export ecad: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var startedAtStr string
		var durationPlayedMS int64
		var title, artist, composer, isrc string

		if err := rows.Scan(&startedAtStr, &durationPlayedMS, &title, &artist, &composer, &isrc); err != nil {
			return fmt.Errorf("export ecad: scan: %w", err)
		}

		startedAt, _ := time.Parse(time.RFC3339, startedAtStr)

		mins := durationPlayedMS / 1000 / 60
		secs := (durationPlayedMS / 1000) % 60
		durStr := fmt.Sprintf("%02d:%02d", mins, secs)

		line := fmt.Sprintf("D;%s;%s;%s;%s;%s;%s;%s;M;R\n",
			startedAt.UTC().Format("02/01/2006"),
			startedAt.UTC().Format("15:04:05"),
			durStr,
			escECAD(title),
			escECAD(artist),
			escECAD(composer),
			escECAD(isrc),
		)
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return rows.Err()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildTLWhere(q TransmissionLogQuery) ([]string, []any) {
	var where []string
	var args []any

	if !q.From.IsZero() {
		where = append(where, "started_at >= ?")
		args = append(args, q.From.UTC().Format(time.RFC3339))
	}
	if !q.To.IsZero() {
		where = append(where, "started_at <= ?")
		args = append(args, q.To.UTC().Format(time.RFC3339))
	}
	if q.Type != "" {
		where = append(where, "type = ?")
		args = append(args, q.Type)
	}
	if q.Status != "" {
		where = append(where, "status = ?")
		args = append(args, q.Status)
	}
	if q.Search != "" {
		where = append(where, "(title LIKE ? OR artist LIKE ?)")
		like := "%" + q.Search + "%"
		args = append(args, like, like)
	}
	return where, args
}

func whereClause(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(parts, " AND ")
}

func scanTLRows(rows *sql.Rows) ([]TransmissionLogEntry, error) {
	var out []TransmissionLogEntry
	for rows.Next() {
		var e TransmissionLogEntry
		var startedAt, finishedAt sql.NullString
		err := rows.Scan(
			&e.ID, &e.EngineID, &e.QueueItemID, &e.AssetID, &e.Path, &e.Title, &e.Artist, &e.Type,
			&e.ISRC, &e.Composer, &e.Publisher,
			&e.DurationMS, &e.DurationPlayedMS, &e.Result, &e.Status,
			&startedAt, &finishedAt,
			&e.BreakID, &e.BreakTitle, &e.BreakRole, &e.BreakPosition,
			&e.ImportFileName,
		)
		if err != nil {
			return nil, fmt.Errorf("scan transmission_log row: %w", err)
		}
		if startedAt.Valid {
			t, _ := time.Parse(time.RFC3339, startedAt.String)
			e.StartedAt = t
		}
		if finishedAt.Valid && finishedAt.String != "" {
			t, _ := time.Parse(time.RFC3339, finishedAt.String)
			e.FinishedAt = &t
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// escECAD removes semicolons from a string to avoid breaking the CSV format.
func escECAD(s string) string {
	return strings.ReplaceAll(s, ";", ",")
}
