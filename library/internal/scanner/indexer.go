package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/Waelson/radio-library-service/internal/config"
	"github.com/Waelson/radio-library-service/internal/store"
)

// TrackWriter is the store method subset the Indexer needs.
type TrackWriter interface {
	Upsert(ctx context.Context, t store.Track) error
	DeleteByPath(ctx context.Context, path string) error
}

// Indexer walks the library directories, probes each audio file and upserts
// the resulting metadata into the track store.
type Indexer struct {
	cfg   config.ScannerConfig
	store TrackWriter
	log   *slog.Logger
}

// NewIndexer creates an Indexer.
func NewIndexer(cfg config.ScannerConfig, ts TrackWriter, log *slog.Logger) *Indexer {
	return &Indexer{cfg: cfg, store: ts, log: log}
}

// ScanResult contains statistics from a full library scan.
type ScanResult struct {
	Indexed  int
	Skipped  int
	ErrCount int
}

// Scan walks all configured directories, indexes every supported audio file
// and returns summary statistics.
func (ix *Indexer) Scan(ctx context.Context) (ScanResult, error) {
	var res ScanResult

	for subdir, assetType := range ix.cfg.Directories {
		dir := filepath.Join(ix.cfg.LibraryRoot, subdir)
		ix.log.Info("scanning directory", "dir", dir, "type", assetType)

		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				ix.log.Warn("walk error", "path", path, "error", err)
				res.ErrCount++
				return nil // keep walking
			}
			if d.IsDir() {
				return nil
			}
			if !ix.isSupportedExt(path) {
				res.Skipped++
				return nil
			}

			if indexErr := ix.IndexFile(ctx, path, assetType); indexErr != nil {
				ix.log.Error("index file failed", "path", path, "error", indexErr)
				res.ErrCount++
			} else {
				res.Indexed++
			}

			// Respect cancellation without aborting the whole walk prematurely.
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			return nil
		})
		if err != nil {
			return res, fmt.Errorf("scan %q: %w", dir, err)
		}
	}

	ix.log.Info("scan complete",
		"indexed", res.Indexed,
		"skipped", res.Skipped,
		"errors", res.ErrCount,
	)
	return res, nil
}

// IndexFile probes a single audio file and upserts it into the track store.
// assetType must be one of MUSIC, VINHETA, JINGLE, SPOT, EFEITOS.
// Metadata strategy is controlled by cfg.MetadataSource ("filename" or "tags").
func (ix *Indexer) IndexFile(ctx context.Context, path, assetType string) error {
	// Always probe for duration; tags are only used when strategy is "tags".
	meta, err := Probe(ctx, ix.cfg.FFprobePath, path)
	if err != nil {
		return fmt.Errorf("probe %q: %w", path, err)
	}

	baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	var title, artist, album, category string
	switch ix.cfg.MetadataSource {
	case "tags":
		title = meta.Title
		artist = meta.Artist
		album = meta.Album
		category = meta.Genre
	default: // "filename"
		parsed := Parse(baseName)
		title = parsed.Title
		artist = parsed.Artist
		album = parsed.Album
		category = parsed.Category
	}

	t := store.Track{
		Path:       path,
		Title:      title,
		Artist:     artist,
		Album:      album,
		Type:       assetType,
		DurationMS: meta.DurationMS,
		Category:   category,
	}

	if err := ix.store.Upsert(ctx, t); err != nil {
		return fmt.Errorf("upsert %q: %w", path, err)
	}

	ix.log.Debug("indexed", "path", path, "title", title, "artist", artist,
		"album", album, "category", category, "type", assetType)
	return nil
}

func (ix *Indexer) isSupportedExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range ix.cfg.Extensions {
		if strings.ToLower(e) == ext {
			return true
		}
	}
	return false
}
