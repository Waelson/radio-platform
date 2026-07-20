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
	"github.com/Waelson/radio-playout-engine/internal/cart"
	"github.com/Waelson/radio-playout-engine/internal/cue"
	"github.com/Waelson/radio-playout-engine/internal/preview"
	apiws "github.com/Waelson/radio-playout-engine/internal/api/ws"
	"github.com/Waelson/radio-playout-engine/internal/api/handlers"
	"github.com/Waelson/radio-playout-engine/internal/audio/decoder"
	"github.com/Waelson/radio-playout-engine/internal/audio/output"
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
	"github.com/Waelson/radio-playout-engine/internal/prefs"
	"github.com/Waelson/radio-playout-engine/internal/queue"
	"github.com/Waelson/radio-playout-engine/internal/mixbus"
	"github.com/Waelson/radio-playout-engine/internal/streaming"
	"github.com/Waelson/radio-playout-engine/internal/scheduler"
	"github.com/Waelson/radio-playout-engine/internal/state"
	"github.com/Waelson/radio-playout-engine/internal/transmissionlog"
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
	mode := ""
	webviewURL := ""
	webviewTitle := "Playout"
	webviewWidth := 730
	webviewHeight := 430
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
		case len(a) > 7 && a[:7] == "--mode=":
			mode = a[7:]
		case len(a) > 10 && a[:10] == "--webview=":
			webviewURL = a[10:]
		case len(a) > 16 && a[:16] == "--webview-title=":
			webviewTitle = a[16:]
		case len(a) > 16 && a[:16] == "--webview-width=":
			if n, err := fmt.Sscanf(a[16:], "%d", &webviewWidth); n != 1 || err != nil {
				webviewWidth = 730
			}
		case len(a) > 17 && a[:17] == "--webview-height=":
			if n, err := fmt.Sscanf(a[17:], "%d", &webviewHeight); n != 1 || err != nil {
				webviewHeight = 430
			}
		default:
			filteredArgs = append(filteredArgs, a)
		}
	}

	// Subprocess mode: open a native WKWebView window and exit.
	if webviewURL != "" {
		appwebview.RunWebview(webviewURL, webviewTitle, webviewWidth, webviewHeight)
		return
	}

	// Subprocess mode: run the isolated CUE (preview) player and exit.
	if mode == "cue-player" {
		if err := runCuePlayer(filteredArgs); err != nil {
			slog.Error("cue player fatal", "error", err)
			os.Exit(1)
		}
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
	configPath := config.ResolveConfigPath(args)
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
		"audio_driver", outfactory.BuiltinDriverName(),
		"log_level", cfg.Logging.Level,
	)

	// 6. Core infrastructure.
	cmdBus := commands.NewBus()
	evtBus := events.NewBus(log)
	stateMgr := state.NewManager(cfg.Engine.ID)

	// 6b. Load persisted preferences (volume levels) and apply to state.
	prefsPath := prefs.DefaultPath()
	p := prefs.Load(prefsPath)
	stateMgr.SetMainVolume(p.MainVolume)
	stateMgr.SetPreviewVolume(p.PreviewVolume)
	log.Info("preferences loaded", "path", prefsPath, "main_volume", p.MainVolume, "preview_volume", p.PreviewVolume)

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
	// AutoPanicSilenceDurationMS is only set when both panic mode and
	// auto-panic-on-silence are enabled; 0 disables the feature entirely.
	autoPanicDurationMS := 0
	if cfg.Panic.Enabled && cfg.Panic.AutoOnSilence {
		autoPanicDurationMS = cfg.Panic.SilenceDurationMS
		if autoPanicDurationMS <= 0 {
			autoPanicDurationMS = cfg.Health.SilenceDurationMS * 2
		}
	}
	healthCfg := health.Config{
		IntervalMS:                 cfg.Health.AudioHealthIntervalMS,
		SilenceThresholdDBFS:       cfg.Health.SilenceThresholdDBFS,
		SilenceDurationMS:          cfg.Health.SilenceDurationMS,
		SampleRate:                 cfg.Audio.SampleRate,
		Channels:                   cfg.Audio.Channels,
		AutoPanicSilenceDurationMS: autoPanicDurationMS,
		OnAutoPanic: func(reason string) {
			if !cfg.Panic.Enabled || !cfg.Panic.AutoOnSilence {
				return
			}
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
		DefaultStopFadeMS:             cfg.Playback.DefaultStopFadeMS,
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
	disp.Handle(commands.CmdInsertBreakNext, queueMgr.HandleInsertBreakNext)
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
	disp.Handle(commands.CmdSetVolume,        pbMgr.HandleSetVolume)

	// 11b. Streaming Manager — fans PCM audio out to Icecast/SHOUTcast targets.
	// Must be wired before the first play session so the tap is ready.
	// Mix Bus: aggregates main playback + cart into a single fixed-rate
	// PCM stream for the streaming manager, eliminating clock jitter.
	mb := mixbus.New()
	pbMgr.SetStreamingTap(mb.MainIn())
	streamMgr := streaming.NewManager(evtBus, log)
	streamMgr.SetAudioIn(mb.OutCh())
	go mb.Run(ctx)
	go streamMgr.Run(ctx)
	log.Info("streaming manager started")

	// 11c. Preview (CUE) player — optional, runs as an isolated subprocess so
	// its CoreAudio client lives in a separate Mach task. This prevents
	// HAL notifications from the BT/A2DP preview device from disrupting the
	// main engine's AudioQueue during preview start/stop.
	prevProxy := cue.NewProxy(evtBus, stateMgr, args, log)
	disp.Handle(commands.CmdPreviewPlay,      prevProxy.HandlePlay)
	disp.Handle(commands.CmdPreviewPause,     prevProxy.HandlePause)
	disp.Handle(commands.CmdPreviewResume,    prevProxy.HandleResume)
	disp.Handle(commands.CmdPreviewStop,      prevProxy.HandleStop)
	disp.Handle(commands.CmdPreviewSeek,      prevProxy.HandleSeek)
	disp.Handle(commands.CmdPreviewSetVolume, prevProxy.HandleSetVolume)
	previewDeps := api.PreviewDeps{GetStatus: func() any { return prevProxy.GetStatus() }}
	go prevProxy.Run(ctx)
	log.Info("preview player initialized as subprocess", "driver", outfactory.BuiltinDriverName())

	// 11d. Cart player — dedicated audio channel for hotkey-triggered playback.
	// Isolated from both the main pipeline and the preview/CUE channel.
	// Always initialized; device_id empty = driver default.
	cartOut, err := outfactory.NewCartOutputDevice(cfg)
	if err != nil {
		return fmt.Errorf("cart output device: %w", err)
	}
	cartVolAtomic := stateMgr.CartVolAtomicPtr()
	cartAudioCfg := cart.AudioConfig{
		DeviceID:     cfg.HotKeys.Output.DeviceID,
		SampleRate:   cfg.Audio.SampleRate,
		Channels:     cfg.Audio.Channels,
		BufferFrames: cfg.Audio.BufferFrames,
	}
	cartPlayer := cart.New(evtBus, decoder.NewFFmpegDecoder(log), cartOut, cartAudioCfg, cartVolAtomic, log)
	cartPlayer.SetStreamingTap(mb.CartIn())
	disp.Handle(commands.CmdCartPlay, cartPlayer.HandlePlay)
	disp.Handle(commands.CmdCartStop, cartPlayer.HandleStop)
	disp.Handle(commands.CmdCartSetVolume, func(_ context.Context, cmd commands.Command) error {
		payload, ok := cmd.Payload.(commands.CartSetVolumePayload)
		if !ok {
			return fmt.Errorf("cart: SetVolume: unexpected payload %T", cmd.Payload)
		}
		stateMgr.SetCartVolume(payload.Level)
		evtBus.Publish(events.New(events.EvtCartVolumeChanged, events.CartVolumeChangedPayload{Level: payload.Level}))
		return nil
	})
	cartDeps := api.CartDeps{GetStatus: func() any { return cartPlayer.GetStatus() }}
	go cartPlayer.Run(ctx)
	stateMgr.SetCartEnabled(true)
	log.Info("cart player initialized", "device", cfg.HotKeys.Output.DeviceID)

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
		AudioDriver:    outfactory.BuiltinDriverName(),
	}
	devicesDeps := api.DevicesDeps{}
	if lister, ok := out.(output.DeviceLister); ok {
		devicesDeps.List = func() ([]handlers.AudioDevice, error) {
			infos, err := lister.ListDevices()
			if err != nil {
				return nil, err
			}
			devs := make([]handlers.AudioDevice, len(infos))
			for i, d := range infos {
				devs[i] = handlers.AudioDevice{
					ID:                d.ID,
					Name:              d.Name,
					Driver:            d.Driver,
					HostAPI:           d.HostAPI,
					IsDefault:         d.IsDefault,
					MaxOutputChannels: d.MaxOutputChannels,
					DefaultSampleRate: d.DefaultSampleRate,
				}
			}
			return devs, nil
		}
	}

	// 15. Scheduler — timed playback scheduling.
	schedStorePath := cfg.Scheduler.StorePath
	if schedStorePath == "" {
		schedStorePath = scheduler.DefaultStorePath()
	}
	schedMgr, err := scheduler.New(scheduler.Config{
		Timezone:          cfg.Scheduler.Timezone,
		StorePath:         schedStorePath,
		MissedThresholdMS: cfg.Scheduler.MissedThresholdMS,
	}, cmdBus, evtBus, stateMgr, log)
	if err != nil {
		return fmt.Errorf("scheduler: %w", err)
	}

	apiSrv := api.New(apiCfg, stateMgr, cmdBus, queueMgr, wsHub, metricsColl, previewDeps, cartDeps, devicesDeps, api.ScheduleDeps{Mgr: schedMgr}, api.ConfigDeps{Snapshot: cfg, Path: configPath}, api.StreamingDeps{Mgr: streamMgr}, log)

	// Transition from STARTING → IDLE now that core is wired.
	stateMgr.SetState(state.StateIdle)
	stateMgr.SetPanicEnabled(cfg.Panic.Enabled)

	evtBus.Publish(events.New(events.EvtEngineStarted, events.EngineStartedPayload{
		EngineID: cfg.Engine.ID,
		Version:  Version,
	}))

	if cfg.Scheduler.Enabled {
		go schedMgr.Run(ctx)
		log.Info("scheduler started",
			"timezone", cfg.Scheduler.Timezone,
			"store", schedStorePath,
			"missed_threshold_ms", cfg.Scheduler.MissedThresholdMS,
		)
	}

	if cfg.TransmissionLog.Enabled {
		tlCfg := transmissionlog.Config{
			EngineID:         cfg.Engine.ID,
			Dir:              cfg.TransmissionLog.Dir,
			FileNameTemplate: cfg.TransmissionLog.FileNameTemplate,
		}
		if tlCfg.FileNameTemplate == "" {
			tlCfg.FileNameTemplate = "transmission_{date}_{hour}.jsonl"
		}
		if tlCfg.Dir == "" {
			tlCfg.Dir = "./transmission-logs"
		}
		tlWriter := transmissionlog.New(tlCfg, evtBus, log)
		go tlWriter.Run(ctx) //nolint:errcheck
		log.Info("transmission log writer started",
			"dir", tlCfg.Dir,
			"template", tlCfg.FileNameTemplate,
		)
	}

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

// runCuePlayer is the entry point for --mode=cue-player.
// It loads the same config as the main engine (forwarding filteredArgs so
// --config= and other flags are honoured), builds the preview output device,
// and delegates to cue.RunCuePlayer which blocks until stdin closes or
// {"cmd":"quit"} is received.
func runCuePlayer(args []string) error {
	cfg, err := config.Load(args)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	log := logging.New(cfg.Logging.Level, cfg.Logging.Format, os.Stderr)
	log = logging.With(log, "cue")

	out, err := outfactory.NewPreviewOutputDevice(cfg)
	if err != nil {
		return fmt.Errorf("preview output device: %w", err)
	}

	audioCfg := preview.AudioConfig{
		DeviceID:     cfg.Preview.Output.DeviceID,
		SampleRate:   cfg.Audio.SampleRate,
		Channels:     cfg.Audio.Channels,
		BufferFrames: cfg.Audio.BufferFrames,
	}

	p := prefs.Load(prefs.DefaultPath())
	cue.RunCuePlayer(out, audioCfg, p.PreviewVolume, log)
	return nil
}
