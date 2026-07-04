//go:build !cli

package apptray

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	getsystray "github.com/getlantern/systray"

	"github.com/Waelson/radio-playout-engine/cmd/playout-engine/engine"
)

const defaultEnginePort = 8080

// RunSystray is the default entry point when no --startup flag is given.
// It blocks until the user chooses Quit from the systray menu.
func RunSystray() {
	getsystray.Run(onSystrayReady, onSystrayExit)
}

func onSystrayReady() {
	getsystray.SetIcon(dotRed)
	getsystray.SetTitle("Playout")
	getsystray.SetTooltip("Playout Engine — Parado")

	mStart := getsystray.AddMenuItem("▶  Iniciar         ", "Inicia a engine de áudio")
	mStop := getsystray.AddMenuItem("■  Parar       ", "Para a engine de áudio")
	mRestart := getsystray.AddMenuItem("↺  Reiniciar        ", "Reinicia a engine de áudio")
	getsystray.AddSeparator()
	mStatus := getsystray.AddMenuItem("◉  Status          ", "Abre status no browser")
	getsystray.AddSeparator()
	mQuit := getsystray.AddMenuItem("✕  Finalizar", "Encerra o systray e a engine")

	mStop.Disable()
	mRestart.Disable()

	eng := engine.NewEngineProc(defaultEnginePort)

	configPath, firstRunErr := engine.EnsureFirstRun()
	var startArgs []string
	if firstRunErr == nil && configPath != "" {
		startArgs = []string{"--config=" + configPath}
	}

	go func() {
		if err := eng.Start(startArgs); err != nil {
			getsystray.SetTooltip("Playout Engine — ERRO ao iniciar: " + err.Error())
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		updateUI := func() {
			if eng.Poll() {
				getsystray.SetIcon(dotGreen)
				getsystray.SetTooltip(fmt.Sprintf("Playout Engine — RODANDO %s (PID %d)", eng.Uptime(), eng.Pid()))
				mStart.Disable()
				mStop.Enable()
				mRestart.Enable()
			} else {
				getsystray.SetIcon(dotRed)
				getsystray.SetTooltip("Playout Engine — Parado")
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
				openBrowser(fmt.Sprintf("http://127.0.0.1:%d/status", defaultEnginePort))

			case <-mQuit.ClickedCh:
				_ = eng.Stop()
				getsystray.Quit()
				return
			}
		}
	}()
}

func onSystrayExit() {}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
