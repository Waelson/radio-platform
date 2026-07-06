package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	outfactory "github.com/Waelson/radio-playout-engine/cmd/playout-engine/output"
	apptray "github.com/Waelson/radio-playout-engine/cmd/playout-engine/systray"
	appwebview "github.com/Waelson/radio-playout-engine/cmd/playout-engine/webview"
	"github.com/Waelson/radio-playout-engine/internal/api"
	"github.com/Waelson/radio-playout-engine/internal/preview"
	apiws "github.com/Waelson/radio-playout-engine/internal/api/ws"
	"github.com/Waelson/radio-playout-engine/internal/audio/decoder"
	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/config"
	"github.com/Waelson/radio-playout-engine/internal/dispatcher"
	"github.com/Waelson/radio-playout-engine/internal/events"
	"github.com/Waelson/radio-playout-engine/internal/health"
	"github.com/Waelson/radio-playout-engine/internal/horacerta"
	"github.com/Waelson/radio-playout-engine/internal/logging"
	"github.com/Waelson/radio-playout-engine/internal/metrics"
	"github.com/Waelson/radio-playout-engine/internal/platform"
	"github.com/Waelson/radio-playout-engine/internal/playback"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/state"
)

// Version is injected at build time:
//
//	go build -ldflags "-X main.Version=0.1.0" ./cmd/playout-engine
var Version = "dev"

func main() {
	// Parse --startup / -startup before handing off to run() or RunSystray().
	// Supports both -startup=cli and -startup cli forms.
	// The flag is stripped from args so config.Load() never sees it.
	startup := "ui"
	webviewURL := ""
	webviewTitle := "Playout"
	var filteredArgs []string
	raw := os.Args[1:]
	for i := 0; i < len(raw); i++ {
		a := raw[i]
		switch {
		case a == "--startup=cli" || a == "-startup=cli":
			startup = "cli"
		case a == "--startup=ui" || a == "-startup=ui":
			startup = "ui"
		case (a == "--startup" || a == "-startup") && i+1 < len(raw):
			startup = raw[i+1]
			i++ // skip value
		case len(a) > 10 && a[:10] == "--webview=":
			webviewURL = a[10:]
		case len(a) > 16 && a[:16] == "--webview-title=":
			webviewTitle = a[16:]
		default:
			filteredArgs = append(filteredArgs, a)
		}
	}

	// Subprocess mode: open a native WKWebView window and exit.
	if webviewURL != "" {
		appwebview.RunWebview(webviewURL, webviewTitle)
		return
	}

	if startup == "cli" {
		if err := run(filteredArgs); err != nil {
			slog.Error("fatal", "error", err)
			os.Exit(1)
		}
		return
	}

	apptray.RunSystray()
}

func run(args []string) error {
	// 1. Load configuration (flags > env > yaml > defaults).
	cfg, err := config.Load(args)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Validate that ffmpeg is available before doing anything else.
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: install ffmpeg and ensure it is accessible")
	}

	// 3. Initialise structured logger.
	log := logging.New(cfg.Logging.Level, cfg.Logging.Format, os.Stderr)
	log = logging.With(log, "engine")

	// 4. Acquire instance lock (prevents duplicate engine processes for the same ID).
	if cfg.Engine.InstanceLock {
		lock := platform.NewInstanceLock(platform.LockPath(cfg.Engine.ID))
		if err := lock.Acquire(); err != nil {
			return fmt.Errorf("instance lock: %w", err)
		}
		defer func() { _ = lock.Release() }()
	}

	// 5. Set up signal-aware context for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Parent watchdog: if the systray process dies (even via SIGKILL), this
	// goroutine detects the PPID change and triggers a graceful shutdown so
	// ffmpeg subprocesses are also cleaned up.
	go func() {
		ppid := os.Getppid()
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if os.Getppid() != ppid {
					stop()
					return
				}
			}
		}
	}()

	log.Info("engine starting",
		"version", Version,
		"engine_id", cfg.Engine.ID,
		"api_addr", fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port),
		"audio_driver", cfg.Audio.Output.Driver,
		"log_level", cfg.Logging.Level,
	)

	// 6. Core infrastructure.
	cmdBus := commands.NewBus()
	evtBus := events.NewBus(log)
	stateMgr := state.NewManager(cfg.Engine.ID)

	// 7. Queue Manager — in-memory playback queue (optionally persistent).
	queueMgr := queue.NewManager(evtBus, stateMgr, log)

	var queueStore *queue.FileStore
	if cfg.Queue.Persistence.Enabled {
		storePath := cfg.Queue.Persistence.Path
		if storePath == "" {
			storePath = queue.DefaultStorePath(cfg.Engine.ID)
		}
		queueStore = queue.NewFileStore(storePath)
		queueMgr.WithStore(queueStore)
		log.Info("queue persistence enabled", "path", storePath)

		if cfg.Queue.Persistence.RestoreOnStart {
			snap, err := queueStore.Load()
			if err != nil {
				log.Warn("queue persistence: failed to load snapshot, starting with empty queue",
					"error", err)
			} else if len(snap.Items) > 0 {
				queueMgr.RestoreFrom(snap)
				log.Info("queue restored", "items", len(snap.Items))
			}
		}
	}

	// 8. Decoder and output device (driver selected by cfg.Audio.Output.Driver).
	dec := decoder.NewFFmpegDecoder(log)
	out, err := outfactory.NewOutputDevice(cfg)
	if err != nil {
		return fmt.Errorf("output device: %w", err)
	}
	if sh, ok := out.(interface{ Shutdown() error }); ok {
		defer func() {
			if err := sh.Shutdown(); err != nil {
				log.Error("output shutdown", "error", err)
			}
		}()
	}

	// 9. Audio Health Monitor — computes RMS/peak, detects silence.
	// AutoPanicSilenceDurationMS: set to 2× SilenceDurationMS so auto-panic
	// triggers after the silence alert has already fired.
	healthCfg := health.Config{
		IntervalMS:                 cfg.Health.AudioHealthIntervalMS,
		SilenceThresholdDBFS:       cfg.Health.SilenceThresholdDBFS,
		SilenceDurationMS:          cfg.Health.SilenceDurationMS,
		SampleRate:                 cfg.Audio.SampleRate,
		Channels:                   cfg.Audio.Channels,
		AutoPanicSilenceDurationMS: cfg.Health.SilenceDurationMS * 2,
		OnAutoPanic: func(reason string) {
			if stateMgr.Snapshot().State == state.StateAssist {
				return
			}
			_ = cmdBus.TrySend(commands.New(commands.CmdEnterPanic, commands.EnterPanicPayload{
				Reason: reason,
			}))
		},
		VUMeterEnabled:    cfg.Health.VUMeterEnabled,
		VUMeterIntervalMS: cfg.Health.VUMeterIntervalMS,
		PeakHoldMS:        cfg.Health.PeakHoldMS,
	}
	healthMon := health.NewMonitor(healthCfg, evtBus, stateMgr, log)

	// 10. Playback Manager — drives the audio session loop.
	pbCfg := playback.Config{
		DeviceID:                      cfg.Audio.Output.DeviceID,
		BufferFrames:                  cfg.Audio.BufferFrames,
		ProgressIntervalMS:            cfg.Health.ProgressIntervalMS,
		MaxConsecutiveFailures:        cfg.Playback.MaxConsecutiveItemFailures,
		DefaultCrossfadeMS:            cfg.Playback.DefaultCrossfadeMS,
		PanicBedPath:                  cfg.Panic.BedPath,
		AutoCrossfadeEnabled:          cfg.Playback.AutoCrossfadeEnabled,
		AutoCrossfadeEnergyThreshDBFS: cfg.Playback.AutoCrossfadeEnergyThreshDBFS,
		AutoCrossfadeMinBeforeEndMS:   cfg.Playback.AutoCrossfadeMinBeforeEndMS,
		AutoCrossfadeMaxBeforeEndMS:   cfg.Playback.AutoCrossfadeMaxBeforeEndMS,
		AutoCrossfadeHoldFrames:       cfg.Playback.AutoCrossfadeHoldFrames,
	}
	pbMgr := playback.NewManager(evtBus, stateMgr, queueMgr, dec, out, pbCfg, healthMon, log)

	// Hora Certa — optional feature; enabled when hours_dir is configured.
	hc := cfg.HoraCerta
	if hc.HoursDir != "" && hc.MinutesDir != "" {
		hcResolver := horacerta.NewResolver(horacerta.Config{
			HoursDir:      hc.HoursDir,
			MinutesDir:    hc.MinutesDir,
			HourPattern:   hc.HourPattern,
			MinutePattern: hc.MinutePattern,
			GainDB:        hc.GainDB,
		})
		pbMgr.WithHoraCerta(hcResolver)
		log.Info("hora certa enabled",
			"hours_dir", hc.HoursDir,
			"minutes_dir", hc.MinutesDir,
			"hour_pattern", hc.HourPattern,
			"minute_pattern", hc.MinutePattern,
			"gain_db", hc.GainDB,
		)
	}

	// 11. Dispatcher wires commands to business-logic handlers.
	disp := dispatcher.New(cmdBus, evtBus, stateMgr, log)
	disp.Handle(commands.CmdEnqueue, queueMgr.HandleEnqueue)
	disp.Handle(commands.CmdEnqueueBreak, queueMgr.HandleEnqueueBreak)
	disp.Handle(commands.CmdInsertNext, queueMgr.HandleInsertNext)
	disp.Handle(commands.CmdInsertAfter, queueMgr.HandleInsertAfter)
	disp.Handle(commands.CmdClearQueue, queueMgr.HandleClear)
	disp.Handle(commands.CmdRemoveItem, queueMgr.HandleRemoveItem)
	disp.Handle(commands.CmdMoveItem, queueMgr.HandleMoveItem)
	disp.Handle(commands.CmdReorderItem, queueMgr.HandleReorderItem)
	disp.Handle(commands.CmdPlay, pbMgr.HandlePlay)
	disp.Handle(commands.CmdPause, pbMgr.HandlePause)
	disp.Handle(commands.CmdResume, pbMgr.HandleResume)
	disp.Handle(commands.CmdStop, pbMgr.HandleStop)
	disp.Handle(commands.CmdSkip, pbMgr.HandleSkip)
	disp.Handle(commands.CmdEnterAssist, pbMgr.HandleEnterAssist)
	disp.Handle(commands.CmdReturnAuto, pbMgr.HandleReturnAuto)
	disp.Handle(commands.CmdEnterPanic, pbMgr.HandleEnterPanic)
	disp.Handle(commands.CmdExitPanic, pbMgr.HandleExitPanic)
	disp.Handle(commands.CmdTriggerHotButton, pbMgr.HandleTriggerHotButton)

	// 11b. Preview player — optional, isolated from the main playback pipeline.
	previewDeps := api.PreviewDeps{Enabled: cfg.Preview.Enabled}
	if cfg.Preview.Enabled {
		previewOut, err := outfactory.NewPreviewOutputDevice(cfg)
		if err != nil {
			return fmt.Errorf("preview output: %w", err)
		}
		prevPlayer := preview.New(evtBus, dec, previewOut, preview.AudioConfig{
			SampleRate:   cfg.Audio.SampleRate,
			Channels:     cfg.Audio.Channels,
			BufferFrames: cfg.Audio.BufferFrames,
		}, log)
		disp.Handle(commands.CmdPreviewPlay,   prevPlayer.HandlePlay)
		disp.Handle(commands.CmdPreviewPause,  prevPlayer.HandlePause)
		disp.Handle(commands.CmdPreviewResume, prevPlayer.HandleResume)
		disp.Handle(commands.CmdPreviewStop,   prevPlayer.HandleStop)
		disp.Handle(commands.CmdPreviewSeek,   prevPlayer.HandleSeek)
		previewDeps.GetStatus = func() any { return prevPlayer.GetStatus() }
		go prevPlayer.Run(ctx)
		log.Info("preview player enabled", "driver", cfg.Preview.OutputDriver)
	}

	// 12. WebSocket Hub — fans out events to connected clients.
	wsHub := apiws.NewHub(evtBus, stateMgr, log)

	// 13. Metrics collector — counts events from the bus.
	metricsColl := metrics.New()

	// 14. API server (reads state, sends commands).
	apiCfg := api.Config{
		Host:           cfg.API.Host,
		Port:           cfg.API.Port,
		AllowedOrigins: cfg.API.CORS.AllowedOrigins,
		EngineID:       cfg.Engine.ID,
		Version:        Version,
		StartTime:      time.Now(),
	}
	apiSrv := api.New(apiCfg, stateMgr, cmdBus, queueMgr, wsHub, metricsColl, previewDeps, log)

	// Transition from STARTING → IDLE now that core is wired.
	stateMgr.SetState(state.StateIdle)

	evtBus.Publish(events.New(events.EvtEngineStarted, events.EngineStartedPayload{
		EngineID: cfg.Engine.ID,
		Version:  Version,
	}))

	// 15. Start goroutines.
	go healthMon.Run(ctx)
	go wsHub.Run(ctx)
	go metricsColl.Run(ctx, evtBus)

	dispErrCh := make(chan struct{})
	go func() {
		defer close(dispErrCh)
		disp.Run(ctx)
	}()

	apiErrCh := make(chan error, 1)
	go func() {
		apiErrCh <- apiSrv.Start()
	}()

	log.Info("engine ready")

	// 16. Block until shutdown signal or API error.
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received", "reason", ctx.Err().Error())
	case err := <-apiErrCh:
		if err != nil {
			return fmt.Errorf("api server: %w", err)
		}
	}

	// 17. Ordered shutdown.
	stop()

	evtBus.Publish(events.New(events.EvtEngineStopping, events.EngineStoppingPayload{
		Reason: "shutdown",
	}))
	stateMgr.SetState(state.StateStopping)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := apiSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("api server shutdown error", "error", err)
	}

	select {
	case <-dispErrCh:
	case <-shutdownCtx.Done():
		log.Warn("dispatcher did not stop in time")
	}

	if queueStore != nil && cfg.Queue.Persistence.ClearOnStop {
		if err := queueStore.Clear(); err != nil {
			log.Warn("queue persistence: failed to clear snapshot on shutdown", "error", err)
		}
	}

	log.Info("engine stopped")
	return nil
}

