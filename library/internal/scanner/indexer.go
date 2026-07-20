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

// TrackCategoryLister returns all tracks eligible for category sync.
type TrackCategoryLister interface {
	ListForCategorySync(ctx context.Context) ([]store.Track, error)
}

// TrackWriter is the store method subset the Indexer needs.
type TrackWriter interface {
	Upsert(ctx context.Context, t store.Track) error
	DeleteByPath(ctx context.Context, path string) error
	FindByPath(ctx context.Context, path string) (store.Track, error)
	// SetCueIn sets cue_in_ms only when it is currently NULL (never overwrites).
	SetCueIn(ctx context.Context, id string, ms int64) error
}

// CategoryLinker syncs a track with its category entity after indexing.
type CategoryLinker interface {
	FindOrCreateByName(ctx context.Context, name string) (store.Category, error)
	LinkTrack(ctx context.Context, categoryID, trackID string) error
}

// LoudnessEnqueuer is an optional hook called after each successful file index.
// Implementations should be non-blocking (e.g. drop to a buffered channel).
type LoudnessEnqueuer interface {
	Enqueue(id string)
}

// Indexer walks the library directories, probes each audio file and upserts
// the resulting metadata into the track store.
type Indexer struct {
	cfg      config.ScannerConfig
	store    TrackWriter
	log      *slog.Logger
	loudness LoudnessEnqueuer // optional; nil = no loudness analysis
	catLink  CategoryLinker   // optional; nil = no category sync
}

// NewIndexer creates an Indexer.
func NewIndexer(cfg config.ScannerConfig, ts TrackWriter, log *slog.Logger) *Indexer {
	return &Indexer{cfg: cfg, store: ts, log: log}
}

// SetLoudnessEnqueuer attaches a LoudnessEnqueuer that is called after each
// successful file index. Safe to call before Start/Scan.
func (ix *Indexer) SetLoudnessEnqueuer(e LoudnessEnqueuer) {
	ix.loudness = e
}

// SetCategoryLinker attaches a CategoryLinker that automatically creates and
// links category entities for each indexed track. Safe to call before Scan.
func (ix *Indexer) SetCategoryLinker(cl CategoryLinker) {
	ix.catLink = cl
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

	ix.log.Info("scan started",
		"library_root", ix.cfg.LibraryRoot,
		"directories", len(ix.cfg.Directories),
	)

	for subdir, assetType := range ix.cfg.Directories {
		dir := filepath.Join(ix.cfg.LibraryRoot, subdir)
		ix.log.Info("scanning directory", "dir", dir, "type", assetType)

		dirIndexed := 0
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				ix.log.Warn("walk error", "path", path, "error", err)
				res.ErrCount++
				return nil // keep walking
			}
			if d.IsDir() {
				if path != dir {
					ix.log.Info("entering subdirectory", "path", path)
				}
				return nil
			}
			if !ix.isSupportedExt(path) {
				ix.log.Info("skipping unsupported file", "path", path, "ext", filepath.Ext(path))
				res.Skipped++
				return nil
			}

			if indexErr := ix.IndexFile(ctx, path, assetType); indexErr != nil {
				ix.log.Error("index file failed", "path", path, "error", indexErr)
				res.ErrCount++
			} else {
				res.Indexed++
				dirIndexed++
				if dirIndexed%50 == 0 {
					ix.log.Info("scan progress",
						"dir", dir,
						"indexed_in_dir", dirIndexed,
						"total_indexed", res.Indexed,
						"total_errors", res.ErrCount,
					)
				}
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

		ix.log.Info("directory scan complete",
			"dir", dir,
			"type", assetType,
			"indexed", dirIndexed,
		)
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
		// ECAD fields — always extracted from tags regardless of MetadataSource,
		// since ISRC/composer/publisher are never encoded in the filename.
		ISRC:      meta.ISRC,
		Composer:  meta.Composer,
		Publisher: meta.Publisher,
	}

	ix.log.Info("upserting track",
		"path", path,
		"title", title,
		"artist", artist,
		"album", album,
		"category", category,
		"type", assetType,
		"duration_ms", t.DurationMS,
	)

	if err := ix.store.Upsert(ctx, t); err != nil {
		return fmt.Errorf("upsert %q: %w", path, err)
	}

	// After upsert, fetch the stored track (needed for ID and current cue_in_ms).
	tr, err := ix.store.FindByPath(ctx, path)
	if err == nil {
		ix.log.Info("track upserted", "id", tr.ID, "path", path)

		// Auto-detect cue_in_ms when enabled and not already set.
		if ix.cfg.AutoDetectCueIn && tr.CueInMS == nil {
			ix.log.Info("detecting cue_in", "path", path)
			if ms := DetectCueIn(ix.cfg.FFmpegPath, path); ms > 0 {
				if setErr := ix.store.SetCueIn(ctx, tr.ID, ms); setErr != nil {
					ix.log.Warn("set cue_in failed", "path", path, "error", setErr)
				} else {
					ix.log.Info("cue_in detected", "path", path, "cue_in_ms", ms)
				}
			} else {
				ix.log.Info("cue_in not detected (no leading silence)", "path", path)
			}
		}

		// Sync category entity and link track.
		if ix.catLink != nil && category != "" {
			ix.log.Info("syncing category", "category", category, "track_id", tr.ID)
			if cat, catErr := ix.catLink.FindOrCreateByName(ctx, category); catErr != nil {
				ix.log.Warn("category sync failed", "category", category, "error", catErr)
			} else {
				ix.log.Info("category resolved", "category", category, "category_id", cat.ID)
				if linkErr := ix.catLink.LinkTrack(ctx, cat.ID, tr.ID); linkErr != nil {
					ix.log.Warn("category link failed", "track_id", tr.ID, "category_id", cat.ID, "error", linkErr)
				} else {
					ix.log.Info("track linked to category", "track_id", tr.ID, "category_id", cat.ID, "category", category)
				}
			}
		}

		// Enqueue for loudness analysis.
		if ix.loudness != nil {
			ix.loudness.Enqueue(tr.ID)
			ix.log.Info("enqueued for loudness analysis", "track_id", tr.ID, "path", path)
		}
	} else {
		ix.log.Warn("could not fetch track after upsert", "path", path, "error", err)
	}

	return nil
}

// SyncCategoryResult holds statistics from a category sync operation.
type SyncCategoryResult struct {
	Linked  int
	Created int
	Errors  int
}

// SyncCategories iterates all tracks with a non-empty category string and
// ensures each one is linked to a category entity via track_categories.
// Missing categories are created automatically using NormalizeCategory.
// This is safe to call concurrently with ongoing indexing.
func (ix *Indexer) SyncCategories(ctx context.Context, lister TrackCategoryLister) (SyncCategoryResult, error) {
	if ix.catLink == nil {
		return SyncCategoryResult{}, fmt.Errorf("no CategoryLinker configured")
	}
	tracks, err := lister.ListForCategorySync(ctx)
	if err != nil {
		return SyncCategoryResult{}, fmt.Errorf("sync categories: list tracks: %w", err)
	}

	// Cache category lookups to avoid hitting the DB for every track.
	catCache := make(map[string]string) // normalizedName → category ID
	var res SyncCategoryResult

	for _, t := range tracks {
		if ctx.Err() != nil {
			break
		}
		cat, catErr := ix.catLink.FindOrCreateByName(ctx, t.Category)
		if catErr != nil {
			ix.log.Warn("sync: category lookup/create failed", "category", t.Category, "error", catErr)
			res.Errors++
			continue
		}
		_, exists := catCache[t.Category]
		if !exists {
			catCache[t.Category] = cat.ID
			res.Created++ // approximate: counts first time we see each category
		}
		if linkErr := ix.catLink.LinkTrack(ctx, cat.ID, t.ID); linkErr != nil {
			ix.log.Warn("sync: link track failed", "track_id", t.ID, "error", linkErr)
			res.Errors++
			continue
		}
		res.Linked++
	}
	ix.log.Info("category sync complete", "linked", res.Linked, "categories", len(catCache), "errors", res.Errors)
	return res, ctx.Err()
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
