//go:build !portaudio && !coreaudio && !wasapi

package outfactory

import (
	"fmt"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

// NewOutputDevice returns an OutputDevice selected by cfg.Audio.Output.Driver.
// Without build tags only "null" and "file" are available.
func NewOutputDevice(cfg *config.Config) (output.OutputDevice, error) {
	switch cfg.Audio.Output.Driver {
	case "null", "":
		return &output.NullOutput{}, nil
	case "file":
		return &output.FileOutput{}, nil
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
	case "wasapi":
		return nil, fmt.Errorf(
			"driver %q requires building with -tags wasapi (Windows only, CGO required)",
			cfg.Audio.Output.Driver,
		)
	default:
		return nil, fmt.Errorf("unknown output driver %q", cfg.Audio.Output.Driver)
	}
}
