// Package fileimporter periodically imports JSONL transmission-log files written
// by the Playout Engine into the SQLite transmission_log table.
//
// Import protocol (zero data loss):
//  1. StartImport → marks the attempt as running in transmission_import_log.
//  2. Parse JSONL line-by-line; skip malformed lines with a warning.
//  3. BulkInsert inside a single SQLite transaction (INSERT OR IGNORE — idempotent).
//  4. os.Rename the file to processados/ — only after COMMIT succeeds.
//  5. FinishImport or FailImport to close the import log entry.
//  6. purgeProcessed — remove processed files older than retention_days.
package fileimporter

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Waelson/radio-library-service/internal/store"
)

// ── Interfaces ────────────────────────────────────────────────────────────────

// LogStore is the subset of TransmissionLogStore required by the importer.
type LogStore interface {
	BulkInsert(ctx context.Context, entries []store.TransmissionLogEntry) error
}

// ImportLogStore is the subset of TransmissionImportLogStore required by the importer.
type ImportLogStore interface {
	StartImport(ctx context.Context, fileName string) (id string, err error)
	FinishImport(ctx context.Context, id string, recordsTotal, recordsImported int) error
	FailImport(ctx context.Context, id string, recordsTotal int, errMsg string) error
}

// Settings is the subset of SettingsStore required by the importer.
type Settings interface {
	TransmissionLogDir(ctx context.Context) (string, error)
	TransmissionLogFileNameTemplate(ctx context.Context) (string, error)
	TransmissionLogPollInterval(ctx context.Context) (time.Duration, error)
	TransmissionLogGracePeriod(ctx context.Context) (time.Duration, error)
	RetentionDaysOrDefault(ctx context.Context) int
}

// ── logEntry mirrors the JSONL record written by the Playout Engine. ──────────

// logEntry is the JSONL record written by the Playout Engine's transmissionlog.Writer.
// It is defined here so the Library Service never imports the playout package.
type logEntry struct {
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	EngineID         string    `json:"engine_id"`
	QueueItemID      string    `json:"queue_item_id"`
	AssetID          string    `json:"asset_id"`
	Title            string    `json:"title"`
	Artist           string    `json:"artist"`
	Type             string    `json:"type"`
	DurationMS       int64     `json:"duration_ms"`
	DurationPlayedMS int64     `json:"duration_played_ms"`
	Result           string    `json:"result"`
	ISRC             string    `json:"isrc,omitempty"`
	Composer         string    `json:"composer,omitempty"`
	Publisher        string    `json:"publisher,omitempty"`
	BreakID          string    `json:"break_id,omitempty"`
	BreakTitle       string    `json:"break_title,omitempty"`
	BreakRole        string    `json:"break_role,omitempty"`
	BreakPosition    int       `json:"break_position,omitempty"`
}

// ── Importer ──────────────────────────────────────────────────────────────────

// Importer is the goroutine that periodically imports JSONL files.
type Importer struct {
	settings  Settings
	logStore  LogStore
	importLog ImportLogStore
	log       *slog.Logger
}

// New creates an Importer. Call Run(ctx) in a goroutine.
func New(settings Settings, logStore LogStore, importLog ImportLogStore, log *slog.Logger) *Importer {
	return &Importer{
		settings:  settings,
		logStore:  logStore,
		importLog: importLog,
		log:       log,
	}
}

// Run blocks until ctx is cancelled, polling at the interval configured in settings.
func (imp *Importer) Run(ctx context.Context) error {
	// First cycle runs immediately.
	imp.RunOnce(ctx)

	for {
		interval, err := imp.settings.TransmissionLogPollInterval(ctx)
		if err != nil || interval <= 0 {
			interval = 5 * time.Minute
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
			imp.RunOnce(ctx)
		}
	}
}

// RunOnce executes a single import cycle. Exported for testing.
func (imp *Importer) RunOnce(ctx context.Context) {
	imp.runCycle(ctx)
}

// runCycle executes one import cycle: scan → import eligible files → purge.
func (imp *Importer) runCycle(ctx context.Context) {
	dir, err := imp.settings.TransmissionLogDir(ctx)
	if err != nil || dir == "" {
		imp.log.Warn("fileimporter: transmission_log.dir not configured")
		return
	}

	tmpl, err := imp.settings.TransmissionLogFileNameTemplate(ctx)
	if err != nil || tmpl == "" {
		tmpl = "transmission_{date}_{hour}.jsonl"
	}

	grace, err := imp.settings.TransmissionLogGracePeriod(ctx)
	if err != nil || grace <= 0 {
		grace = 15 * time.Minute
	}

	glob := BuildGlob(tmpl)
	now := time.Now()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			imp.log.Warn("fileimporter: read dir failed", "dir", dir, "err", err)
		}
		return
	}

	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		matched, _ := filepath.Match(glob, name)
		if !matched {
			continue
		}
		fi, err := de.Info()
		if err != nil {
			continue
		}
		if !IsEligible(fi, now, grace) {
			imp.log.Debug("fileimporter: file not yet eligible (grace period)",
				"file", name, "mtime", fi.ModTime())
			continue
		}
		imp.importFile(ctx, dir, name)
	}

	imp.purgeProcessed(ctx, dir, now)
}

// importFile processes a single JSONL file.
func (imp *Importer) importFile(ctx context.Context, dir, name string) {
	path := filepath.Join(dir, name)

	importID, err := imp.importLog.StartImport(ctx, name)
	if err != nil {
		imp.log.Error("fileimporter: StartImport failed", "file", name, "err", err)
		return
	}

	entries, total, skipped := imp.parseJSONL(path, name)
	if skipped > 0 {
		imp.log.Warn("fileimporter: skipped malformed lines",
			"file", name, "skipped", skipped, "parsed", len(entries))
	}

	if err := imp.logStore.BulkInsert(ctx, entries); err != nil {
		imp.log.Error("fileimporter: BulkInsert failed", "file", name, "err", err)
		imp.failImport(ctx, importID, total, err.Error())
		return
	}
	imported := len(entries)

	// Move file to processados/ — only after successful COMMIT.
	processedDir := filepath.Join(dir, "processados")
	if err := os.MkdirAll(processedDir, 0o755); err != nil {
		imp.log.Error("fileimporter: mkdir processados failed", "err", err)
		imp.failImport(ctx, importID, total, err.Error())
		return
	}
	dest := filepath.Join(processedDir, name)
	if err := os.Rename(path, dest); err != nil {
		imp.log.Error("fileimporter: rename failed", "src", path, "dest", dest, "err", err)
		imp.failImport(ctx, importID, total, err.Error())
		return
	}

	if err := imp.importLog.FinishImport(ctx, importID, total, imported); err != nil {
		imp.log.Warn("fileimporter: FinishImport failed", "file", name, "err", err)
	}
	imp.log.Info("fileimporter: imported file",
		"file", name, "total", total, "imported", imported)
}

// parseJSONL reads the JSONL file and returns valid entries, total line count and skipped count.
func (imp *Importer) parseJSONL(path, fileName string) ([]store.TransmissionLogEntry, int, int) {
	f, err := os.Open(path)
	if err != nil {
		imp.log.Error("fileimporter: open file failed", "path", path, "err", err)
		return nil, 0, 0
	}
	defer f.Close()

	var (
		entries []store.TransmissionLogEntry
		total   int
		skipped int
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		total++

		var le logEntry
		if err := json.Unmarshal([]byte(line), &le); err != nil {
			imp.log.Warn("fileimporter: malformed line", "file", fileName,
				"line", total, "err", err)
			skipped++
			continue
		}
		if le.QueueItemID == "" || le.Title == "" {
			imp.log.Warn("fileimporter: incomplete line (missing queue_item_id or title)",
				"file", fileName, "line", total)
			skipped++
			continue
		}

		finishedAt := le.FinishedAt
		e := store.TransmissionLogEntry{
			EngineID:         le.EngineID,
			QueueItemID:      le.QueueItemID,
			AssetID:          le.AssetID,
			Title:            le.Title,
			Artist:           le.Artist,
			Type:             le.Type,
			ISRC:             le.ISRC,
			Composer:         le.Composer,
			Publisher:        le.Publisher,
			DurationMS:       le.DurationMS,
			DurationPlayedMS: le.DurationPlayedMS,
			Result:           le.Result,
			StartedAt:        le.StartedAt,
			FinishedAt:       &finishedAt,
			BreakID:          le.BreakID,
			BreakTitle:       le.BreakTitle,
			BreakRole:        le.BreakRole,
			BreakPosition:    le.BreakPosition,
			ImportFileName:   fileName,
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		imp.log.Warn("fileimporter: scanner error", "path", path, "err", err)
	}
	return entries, total, skipped
}

// purgeProcessed removes processed files whose mtime is older than retention_days.
func (imp *Importer) purgeProcessed(ctx context.Context, dir string, now time.Time) {
	retentionDays := imp.settings.RetentionDaysOrDefault(ctx)
	cutoff := now.AddDate(0, 0, -retentionDays)

	processedDir := filepath.Join(dir, "processados")
	entries, err := os.ReadDir(processedDir)
	if err != nil {
		return // directory may not exist yet; silent
	}

	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		fi, err := de.Info()
		if err != nil {
			continue
		}
		if fi.ModTime().Before(cutoff) {
			path := filepath.Join(processedDir, de.Name())
			if err := os.Remove(path); err != nil {
				imp.log.Warn("fileimporter: purge failed", "file", de.Name(), "err", err)
			} else {
				imp.log.Info("fileimporter: purged processed file", "file", de.Name())
			}
		}
	}
}

// failImport is a convenience helper that logs on error.
func (imp *Importer) failImport(ctx context.Context, importID string, total int, msg string) {
	if err := imp.importLog.FailImport(ctx, importID, total, msg); err != nil {
		imp.log.Warn("fileimporter: FailImport failed", "err", err)
	}
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// BuildGlob converts a file-name template into a filepath.Match glob by replacing
// {date} and {hour} placeholders with *.
//
//	"transmission_{date}_{hour}.jsonl" → "transmission_*_*.jsonl"
func BuildGlob(template string) string {
	s := strings.ReplaceAll(template, "{date}", "*")
	s = strings.ReplaceAll(s, "{hour}", "*")
	return s
}

// IsEligible returns true when the file is old enough to be safely imported.
// A file is eligible when its mtime is at least grace ago.
func IsEligible(fi os.FileInfo, now time.Time, grace time.Duration) bool {
	return now.Sub(fi.ModTime()) >= grace
}
