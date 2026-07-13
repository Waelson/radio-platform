package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Waelson/radio-library-service/internal/api"
	"github.com/Waelson/radio-library-service/internal/config"
	"github.com/Waelson/radio-library-service/internal/fileimporter"
	"github.com/Waelson/radio-library-service/internal/indexsvc"
	"github.com/Waelson/radio-library-service/internal/logging"
	"github.com/Waelson/radio-library-service/internal/scanner"
	"github.com/Waelson/radio-library-service/internal/scheduler"
	"github.com/Waelson/radio-library-service/internal/store"
)

// Version is injected at build time:
//
//	go build -ldflags "-X main.Version=0.1.0" ./cmd/library-service
var Version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// 1. Load configuration.
	cfg, err := config.Load(args)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	cfg.Service.Version = Version

	// 2. Initialise structured logger.
	log := logging.New(cfg.Logging.Level, cfg.Logging.Format, os.Stderr)
	log = logging.With(log, "library")

	// 3. Signal-aware context for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("library service starting",
		"version", Version,
		"service_id", cfg.Service.ID,
		"api_addr", fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port),
		"db_path", cfg.DB.Path,
		"library_root", cfg.Scanner.LibraryRoot,
	)

	// 4. Open SQLite database and apply migrations.
	db, err := store.Open(ctx, cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("database close error", "error", err)
		}
	}()

	log.Info("database ready", "path", cfg.DB.Path)

	// 5. Build stores and scanner.
	settingsStore := store.NewSettingsStore(db)
	tlStore := store.NewTransmissionLogStore(db)
	ilStore := store.NewTransmissionImportLogStore(db)
	trackStore := store.NewTrackStore(db)
	indexer := scanner.NewIndexer(cfg.Scanner, trackStore, logging.With(log, "scanner"))
	idxSvc := indexsvc.New(indexer, trackStore, logging.With(log, "indexsvc"))

	// 6. Initial library scan (non-blocking; state tracked via idxSvc).
	idxSvc.RunInitialScan(ctx)

	// 7. Start filesystem watcher (if enabled).
	if cfg.Scanner.WatchEnabled {
		watcher := scanner.NewWatcher(
			cfg.Scanner, indexer, trackStore,
			logging.With(log, "watcher"),
		)
		go func() {
			if err := watcher.Run(ctx); err != nil {
				slog.Error("watcher stopped with error", "error", err)
			}
		}()
		log.Info("filesystem watcher started")
	}

	// 8. Start file importer (reads config from DB each cycle).
	imp := fileimporter.New(settingsStore, tlStore, ilStore, logging.With(log, "fileimporter"))
	go func() {
		if err := imp.Run(ctx); err != nil {
			slog.Error("fileimporter stopped with error", "error", err)
		}
	}()
	log.Info("transmission log importer started")

	// 10. Start HTTP API server.
	playlistStore := store.NewPlaylistStore(db)
	breakStore := store.NewBreakStore(db)
	hotkeyStore := store.NewHotkeyStore(db)
	categoryStore := store.NewCategoryStore(db)
	clockStore := store.NewClockStore(db)
	separationStore := store.NewSeparationRuleStore(db)
	rotationLogStore := store.NewRotationLogStore(db)

	gen := scheduler.New(
		&scheduler.ClockStoreAdapter{S: clockStore},
		&scheduler.TrackStoreAdapter{S: trackStore, C: categoryStore},
		&scheduler.SeparationRuleStoreAdapter{S: separationStore},
		&scheduler.RotationLogStoreAdapter{S: rotationLogStore},
	)

	srv := api.New(cfg.API, trackStore, playlistStore, breakStore, hotkeyStore, idxSvc,
		categoryStore, clockStore, separationStore, rotationLogStore, gen,
		tlStore, ilStore, settingsStore, settingsStore,
		logging.With(log, "api"))
	go func() {
		if err := srv.Start(ctx); err != nil {
			slog.Error("API server error", "error", err)
		}
	}()

	log.Info("library service ready")

	// 11. Block until shutdown signal.
	<-ctx.Done()
	log.Info("shutdown signal received", "reason", ctx.Err().Error())

	stop() // release OS signal resources

	// 12. Graceful shutdown with timeout.
	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	log.Info("library service stopped")
	return nil
}
