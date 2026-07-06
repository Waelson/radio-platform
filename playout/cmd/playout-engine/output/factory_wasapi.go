//go:build wasapi

package outfactory

import (
	"fmt"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	waout "github.com/Waelson/radio-playout-engine/internal/audio/output/wasapi"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

// NewOutputDevice returns an OutputDevice selected by cfg.Audio.Output.Driver.
// Built with -tags wasapi: "null", "file", and "wasapi" are available.
func NewOutputDevice(cfg *config.Config) (output.OutputDevice, error) {
	switch cfg.Audio.Output.Driver {
	case "null", "":
		return &output.NullOutput{}, nil
	case "file":
		return &output.FileOutput{}, nil
	case "wasapi":
		return waout.New(), nil
	case "portaudio":
		return nil, fmt.Errorf(
			"driver %q requires building with -tags portaudio (CGO + PortAudio system library)",
			cfg.Audio.Output.Driver,
		)
	case "coreaudio":
		return nil, fmt.Errorf(
			"driver %q requires building with -tags coreaudio (macOS only, CGO required)",
			cfg.Audio.Output.Driver,
		)
	default:
		return nil, fmt.Errorf("unknown output driver %q", cfg.Audio.Output.Driver)
	}
}
