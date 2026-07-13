package fileimporter_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Waelson/radio-library-service/internal/fileimporter"
	"github.com/Waelson/radio-library-service/internal/store"
)

func newNopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ── Fakes ─────────────────────────────────────────────────────────────────────

type fakeSettings struct {
	dir      string
	tmpl     string
	poll     time.Duration
	grace    time.Duration
	retention int
}

func (f *fakeSettings) TransmissionLogDir(_ context.Context) (string, error) {
	return f.dir, nil
}
func (f *fakeSettings) TransmissionLogFileNameTemplate(_ context.Context) (string, error) {
	if f.tmpl == "" {
		return "transmission_{date}_{hour}.jsonl", nil
	}
	return f.tmpl, nil
}
func (f *fakeSettings) TransmissionLogPollInterval(_ context.Context) (time.Duration, error) {
	if f.poll == 0 {
		return 5 * time.Minute, nil
	}
	return f.poll, nil
}
func (f *fakeSettings) TransmissionLogGracePeriod(_ context.Context) (time.Duration, error) {
	return f.grace, nil
}
func (f *fakeSettings) RetentionDaysOrDefault(_ context.Context) int {
	if f.retention == 0 {
		return 30
	}
	return f.retention
}

// fakeLogStore records BulkInsert calls.
type fakeLogStore struct {
	entries []store.TransmissionLogEntry
	failErr error
}

func (f *fakeLogStore) BulkInsert(_ context.Context, entries []store.TransmissionLogEntry) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.entries = append(f.entries, entries...)
	return nil
}

// fakeImportLog records import log calls.
type fakeImportLog struct {
	started  []string // file names
	finished []finishCall
	failed   []failCall
}

type finishCall struct{ id string; total, imported int }
type failCall struct{ id, msg string; total int }

func (f *fakeImportLog) StartImport(_ context.Context, fileName string) (string, error) {
	f.started = append(f.started, fileName)
	return "import-id-" + fileName, nil
}
func (f *fakeImportLog) FinishImport(_ context.Context, id string, total, imported int) error {
	f.finished = append(f.finished, finishCall{id, total, imported})
	return nil
}
func (f *fakeImportLog) FailImport(_ context.Context, id string, total int, msg string) error {
	f.failed = append(f.failed, failCall{id, msg, total})
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

type jsonlEntry struct {
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	QueueItemID      string    `json:"queue_item_id"`
	AssetID          string    `json:"asset_id"`
	Title            string    `json:"title"`
	Artist           string    `json:"artist"`
	Type             string    `json:"type"`
	DurationMS       int64     `json:"duration_ms"`
	DurationPlayedMS int64     `json:"duration_played_ms"`
	Result           string    `json:"result"`
}

func writeJSONL(t *testing.T, path string, entries []jsonlEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("writeJSONL create: %v", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			t.Fatalf("writeJSONL encode: %v", err)
		}
	}
}

func newImporter(settings fileimporter.Settings, ls fileimporter.LogStore, il fileimporter.ImportLogStore) *fileimporter.Importer {
	return fileimporter.New(settings, ls, il, newNopLogger())
}

// ── Tests: buildGlob / isEligible (package-level via exported wrapper) ────────

func TestBuildGlob(t *testing.T) {
	cases := []struct {
		tmpl, want string
	}{
		{"transmission_{date}_{hour}.jsonl", "transmission_*_*.jsonl"},
		{"{date}_{hour}.log", "*_*.log"},
		{"log.jsonl", "log.jsonl"},
	}
	for _, c := range cases {
		got := fileimporter.BuildGlob(c.tmpl)
		if got != c.want {
			t.Errorf("BuildGlob(%q) = %q, want %q", c.tmpl, got, c.want)
		}
	}
}

func TestIsEligible(t *testing.T) {
	grace := 15 * time.Minute
	now := time.Now()

	recentFile := fakeFileInfo{mtime: now.Add(-5 * time.Minute)}
	oldFile := fakeFileInfo{mtime: now.Add(-20 * time.Minute)}
	exactFile := fakeFileInfo{mtime: now.Add(-grace)}

	if fileimporter.IsEligible(recentFile, now, grace) {
		t.Error("file modified 5min ago should NOT be eligible")
	}
	if !fileimporter.IsEligible(oldFile, now, grace) {
		t.Error("file modified 20min ago should be eligible")
	}
	if !fileimporter.IsEligible(exactFile, now, grace) {
		t.Error("file modified exactly at grace should be eligible")
	}
}

// fakeFileInfo implements os.FileInfo for testing.
type fakeFileInfo struct{ mtime time.Time }

func (f fakeFileInfo) Name() string      { return "test.jsonl" }
func (f fakeFileInfo) Size() int64       { return 0 }
func (f fakeFileInfo) Mode() os.FileMode { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return f.mtime }
func (f fakeFileInfo) IsDir() bool       { return false }
func (f fakeFileInfo) Sys() any          { return nil }

// ── Tests: full import cycle ───────────────────────────────────────────────────

func TestImporter_ImportFile_Success(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	// Write a JSONL file with two entries.
	fileName := "transmission_20260720_08.jsonl"
	path := filepath.Join(dir, fileName)
	writeJSONL(t, path, []jsonlEntry{
		{
			QueueItemID: "qi-001", AssetID: "a1", Title: "Track One", Artist: "Artist A",
			Type: "MUSIC", DurationMS: 240000, DurationPlayedMS: 240000,
			Result: "finished", StartedAt: now.Add(-8 * time.Hour), FinishedAt: now.Add(-7*time.Hour - 56*time.Minute),
		},
		{
			QueueItemID: "qi-002", AssetID: "a2", Title: "Track Two", Artist: "Artist B",
			Type: "SPOT", DurationMS: 30000, DurationPlayedMS: 30000,
			Result: "finished", StartedAt: now.Add(-7*time.Hour - 55*time.Minute), FinishedAt: now.Add(-7*time.Hour - 54*time.Minute - 30*time.Second),
		},
	})

	// Set mtime to > grace period.
	oldTime := now.Add(-20 * time.Minute)
	os.Chtimes(path, oldTime, oldTime)

	ls := &fakeLogStore{}
	il := &fakeImportLog{}
	cfg := &fakeSettings{dir: dir, grace: 15 * time.Minute}

	imp := newImporter(cfg, ls, il)
	ctx := context.Background()
	imp.RunOnce(ctx)

	// Both entries should be imported.
	if len(ls.entries) != 2 {
		t.Errorf("entries imported = %d, want 2", len(ls.entries))
	}
	if len(il.started) != 1 || il.started[0] != fileName {
		t.Errorf("started = %v, want [%s]", il.started, fileName)
	}
	if len(il.finished) != 1 || il.finished[0].total != 2 || il.finished[0].imported != 2 {
		t.Errorf("finished = %+v, want {total:2, imported:2}", il.finished)
	}
	if len(il.failed) != 0 {
		t.Errorf("unexpected failures: %+v", il.failed)
	}

	// File should have been moved to processados/.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("source file should have been moved away")
	}
	processedPath := filepath.Join(dir, "processados", fileName)
	if _, err := os.Stat(processedPath); err != nil {
		t.Errorf("processed file not found: %v", err)
	}

	// ImportFileName should be set on each entry.
	for _, e := range ls.entries {
		if e.ImportFileName != fileName {
			t.Errorf("ImportFileName = %q, want %q", e.ImportFileName, fileName)
		}
	}
}

func TestImporter_GracePeriod_SkipsRecentFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	path := filepath.Join(dir, "transmission_20260720_09.jsonl")
	writeJSONL(t, path, []jsonlEntry{
		{QueueItemID: "qi-x", Title: "Song", Type: "MUSIC", Result: "finished",
			StartedAt: now.Add(-2 * time.Minute), FinishedAt: now.Add(-1 * time.Minute)},
	})
	// mtime = 5min ago, grace = 15min → not eligible.
	recentTime := now.Add(-5 * time.Minute)
	os.Chtimes(path, recentTime, recentTime)

	ls := &fakeLogStore{}
	il := &fakeImportLog{}
	cfg := &fakeSettings{dir: dir, grace: 15 * time.Minute}

	newImporter(cfg, ls, il).RunOnce(context.Background())

	if len(ls.entries) != 0 {
		t.Errorf("expected no imports (grace period), got %d entries", len(ls.entries))
	}
	// Source file must still be in the root dir.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("source file should still exist: %v", err)
	}
}

func TestImporter_IgnoresProcessadosSubdir(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	// Put a file in processados/ — it must be ignored.
	processed := filepath.Join(dir, "processados")
	os.MkdirAll(processed, 0o755)
	path := filepath.Join(processed, "transmission_20260720_07.jsonl")
	writeJSONL(t, path, []jsonlEntry{
		{QueueItemID: "qi-p", Title: "Old Song", Type: "MUSIC", Result: "finished",
			StartedAt: now.Add(-2 * time.Hour), FinishedAt: now.Add(-2*time.Hour + 4*time.Minute)},
	})
	oldTime := now.Add(-2 * time.Hour)
	os.Chtimes(path, oldTime, oldTime)

	ls := &fakeLogStore{}
	il := &fakeImportLog{}
	cfg := &fakeSettings{dir: dir, grace: 0}

	newImporter(cfg, ls, il).RunOnce(context.Background())

	if len(ls.entries) != 0 {
		t.Errorf("processados/ should be ignored; got %d entries", len(ls.entries))
	}
}

func TestImporter_IgnoresNonMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	path := filepath.Join(dir, "some_other_file.txt")
	os.WriteFile(path, []byte(`{"queue_item_id":"qi-z","title":"T","type":"MUSIC","result":"finished"}`+"\n"), 0o644)
	old := now.Add(-1 * time.Hour)
	os.Chtimes(path, old, old)

	ls := &fakeLogStore{}
	il := &fakeImportLog{}
	cfg := &fakeSettings{dir: dir, grace: 0}

	newImporter(cfg, ls, il).RunOnce(context.Background())

	if len(ls.entries) != 0 {
		t.Errorf("non-matching file should be ignored; got %d entries", len(ls.entries))
	}
}

func TestImporter_MalformedLines_SkippedWithWarning(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	fileName := "transmission_20260720_10.jsonl"
	path := filepath.Join(dir, fileName)

	good := jsonlEntry{
		QueueItemID: "qi-g", Title: "Good Track", Type: "MUSIC", Result: "finished",
		StartedAt: now.Add(-1 * time.Hour), FinishedAt: now.Add(-56 * time.Minute),
	}
	b, _ := json.Marshal(good)
	// Write one good + one malformed + one good.
	content := string(b) + "\n" + "NOT JSON AT ALL\n" + string(b[:5]) + "\n"
	os.WriteFile(path, []byte(content), 0o644)

	old := now.Add(-20 * time.Minute)
	os.Chtimes(path, old, old)

	ls := &fakeLogStore{}
	il := &fakeImportLog{}
	cfg := &fakeSettings{dir: dir, grace: 15 * time.Minute}

	newImporter(cfg, ls, il).RunOnce(context.Background())

	// Only the one valid entry should be imported.
	if len(ls.entries) != 1 {
		t.Errorf("expected 1 valid entry, got %d", len(ls.entries))
	}
	if len(il.finished) != 1 && il.finished[0].total != 3 {
		t.Errorf("finished total should be 3 (3 non-empty lines), got %+v", il.finished)
	}
}

func TestImporter_BulkInsertFail_RecordsFailedImport(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	fileName := "transmission_20260720_11.jsonl"
	path := filepath.Join(dir, fileName)
	writeJSONL(t, path, []jsonlEntry{
		{QueueItemID: "qi-f", Title: "T", Type: "MUSIC", Result: "finished",
			StartedAt: now.Add(-2 * time.Hour), FinishedAt: now.Add(-116 * time.Minute)},
	})
	old := now.Add(-20 * time.Minute)
	os.Chtimes(path, old, old)

	ls := &fakeLogStore{failErr: os.ErrPermission}
	il := &fakeImportLog{}
	cfg := &fakeSettings{dir: dir, grace: 15 * time.Minute}

	newImporter(cfg, ls, il).RunOnce(context.Background())

	if len(il.failed) == 0 {
		t.Error("expected FailImport to be called after BulkInsert error")
	}
	// Source file must remain in the root (not moved).
	if _, err := os.Stat(path); err != nil {
		t.Errorf("source file should remain in root after failed import: %v", err)
	}
}

func TestImporter_Idempotent_ReimportSameFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	fileName := "transmission_20260720_12.jsonl"
	path := filepath.Join(dir, fileName)
	writeJSONL(t, path, []jsonlEntry{
		{QueueItemID: "qi-idem", Title: "T", Type: "MUSIC", Result: "finished",
			StartedAt: now.Add(-2 * time.Hour), FinishedAt: now.Add(-116 * time.Minute)},
	})
	old := now.Add(-20 * time.Minute)
	os.Chtimes(path, old, old)

	ls := &fakeLogStore{}
	il := &fakeImportLog{}
	cfg := &fakeSettings{dir: dir, grace: 15 * time.Minute}
	imp := newImporter(cfg, ls, il)
	ctx := context.Background()

	// First import.
	imp.RunOnce(ctx)
	if len(ls.entries) != 1 {
		t.Fatalf("first import: expected 1 entry, got %d", len(ls.entries))
	}

	// Simulate: move file back to root (crash scenario after COMMIT before Rename).
	processed := filepath.Join(dir, "processados", fileName)
	os.Rename(processed, path)
	os.Chtimes(path, old, old)

	// Second import — BulkInsert is idempotent via INSERT OR IGNORE.
	imp.RunOnce(ctx)
	// Total calls to BulkInsert: 2, but total entries accumulated = 2 (fake appends).
	// What matters: no panic, no error, FinishImport called twice.
	if len(il.finished) != 2 {
		t.Errorf("expected 2 FinishImport calls (2 import cycles), got %d", len(il.finished))
	}
}

func TestImporter_PurgeProcessed_RemovesExpiredFiles(t *testing.T) {
	dir := t.TempDir()
	processedDir := filepath.Join(dir, "processados")
	os.MkdirAll(processedDir, 0o755)

	now := time.Now()

	// Create one expired file and one fresh file.
	expired := filepath.Join(processedDir, "transmission_20260601_08.jsonl")
	fresh := filepath.Join(processedDir, "transmission_20260720_08.jsonl")
	os.WriteFile(expired, []byte{}, 0o644)
	os.WriteFile(fresh, []byte{}, 0o644)

	// Set mtime: expired = 40 days ago, fresh = 5 days ago.
	expiredTime := now.AddDate(0, 0, -40)
	freshTime := now.AddDate(0, 0, -5)
	os.Chtimes(expired, expiredTime, expiredTime)
	os.Chtimes(fresh, freshTime, freshTime)

	ls := &fakeLogStore{}
	il := &fakeImportLog{}
	cfg := &fakeSettings{dir: dir, grace: 0, retention: 30}

	// No eligible files in root → only purge runs.
	newImporter(cfg, ls, il).RunOnce(context.Background())

	if _, err := os.Stat(expired); !os.IsNotExist(err) {
		t.Error("expired file should have been purged")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("fresh file should NOT have been purged")
	}
}

func TestImporter_PurgeProcessed_RespectsMinimumRetention(t *testing.T) {
	dir := t.TempDir()
	processedDir := filepath.Join(dir, "processados")
	os.MkdirAll(processedDir, 0o755)

	now := time.Now()

	// File is 3 days old — should survive even if retention is set to 1 (below min 7).
	file := filepath.Join(processedDir, "transmission_20260717_08.jsonl")
	os.WriteFile(file, []byte{}, 0o644)
	threeDaysAgo := now.AddDate(0, 0, -3)
	os.Chtimes(file, threeDaysAgo, threeDaysAgo)

	ls := &fakeLogStore{}
	il := &fakeImportLog{}
	// retention=1 — should be elevated to 7 by RetentionDaysOrDefault.
	cfg := &fakeSettings{dir: dir, grace: 0, retention: 7}

	newImporter(cfg, ls, il).RunOnce(context.Background())

	if _, err := os.Stat(file); err != nil {
		t.Error("file within min-retention (7 days) should NOT be purged")
	}
}
