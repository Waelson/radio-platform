//go:build !cli

package apptray

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	getsystray "github.com/getlantern/systray"

	"github.com/Waelson/radio-playout-engine/cmd/playout-engine/engine"
	"github.com/Waelson/radio-playout-engine/cmd/playout-engine/webview"
	appcfg "github.com/Waelson/radio-playout-engine/internal/config"
)

// eng is package-level so onSystrayExit can access it.
var eng *engine.EngineProc

const defaultEnginePort = 8080

// RunSystray is the default entry point when no --startup flag is given.
// It blocks until the user chooses Quit from the systray menu.
func RunSystray() {
	getsystray.Run(onSystrayReady, onSystrayExit)
}

func onSystrayReady() {
	getsystray.SetIcon(iconFailure)
	getsystray.SetTitle("")
	getsystray.SetTooltip("RadioCore — Parado")

	mStart := getsystray.AddMenuItem("▶  Iniciar         ", "Inicia a engine de áudio")
	mStop := getsystray.AddMenuItem("■  Parar       ", "Para a engine de áudio")
	mRestart := getsystray.AddMenuItem("↺  Reiniciar        ", "Reinicia a engine de áudio")
	getsystray.AddSeparator()
	mStatus := getsystray.AddMenuItem("◉  Status          ", "Abre status no browser")
	mConfig := getsystray.AddMenuItem("⚙  Configuração    ", "Abre configurações no browser")
	getsystray.AddSeparator()
	mQuit := getsystray.AddMenuItem("✕  Finalizar", "Encerra o systray e a engine")

	mStop.Disable()
	mRestart.Disable()

	eng = engine.NewEngineProc(defaultEnginePort)

	configPath, firstRunErr := engine.EnsureFirstRun()
	var startArgs []string
	if firstRunErr == nil && configPath != "" {
		startArgs = []string{"--config=" + configPath}
		if cfg, err := appcfg.Load(startArgs); err == nil {
			eng.SetLogDir(cfg.Logging.Dir)
		}
	}

	// Stop engine and webviews on SIGTERM or SIGINT (e.g. Activity Monitor "Quit", kill command).
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		webview.KillAll()
		_ = eng.Stop()
		getsystray.Quit()
	}()

	go func() {
		if err := eng.Start(startArgs); err != nil {
			getsystray.SetTooltip("RadioCore — ERRO ao iniciar: " + err.Error())
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		updateUI := func() {
			if eng.Poll() {
				getsystray.SetIcon(iconSuccess)
				getsystray.SetTooltip(fmt.Sprintf("RadioCore — RODANDO %s (PID %d)", eng.Uptime(), eng.Pid()))
				mStart.Disable()
				mStop.Enable()
				mRestart.Enable()
			} else {
				getsystray.SetIcon(iconFailure)
				getsystray.SetTooltip("RadioCore — Parado")
				mStart.Enable()
				mStop.Disable()
				mRestart.Disable()
			}
		}

		time.Sleep(2 * time.Second)
		updateUI()

		for {
			select {
			case <-ticker.C:
				updateUI()

			case <-mStart.ClickedCh:
				_ = eng.Start(startArgs)
				time.Sleep(2 * time.Second)
				updateUI()

			case <-mStop.ClickedCh:
				_ = eng.Stop()
				updateUI()

			case <-mRestart.ClickedCh:
				_ = eng.Restart(startArgs)
				time.Sleep(2 * time.Second)
				updateUI()

			case <-mStatus.ClickedCh:
				webview.OpenPlayerWindow(fmt.Sprintf("http://127.0.0.1:%d/status", defaultEnginePort), "RadioCore — Status", 803, 430)

			case <-mConfig.ClickedCh:
				webview.OpenPlayerWindow(fmt.Sprintf("http://127.0.0.1:%d/config", defaultEnginePort), "RadioCore — Configuração", 1095, 741)

			case <-mQuit.ClickedCh:
				webview.KillAll()
				_ = eng.Stop()
				getsystray.Quit()
				return
			}
		}
	}()
}

// onSystrayExit is called by the systray library when the process is exiting
// for any reason. It ensures the engine child process is always stopped.
func onSystrayExit() {
	webview.KillAll()
	if eng != nil {
		_ = eng.Stop()
	}
}
