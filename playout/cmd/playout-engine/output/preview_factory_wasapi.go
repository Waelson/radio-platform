//go:build wasapi

package outfactory

import (
	"fmt"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	waout "github.com/Waelson/radio-playout-engine/internal/audio/output/wasapi"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

// NewPreviewOutputDevice returns an OutputDevice for the preview (cue) player,
// selected by cfg.Preview.OutputDriver.
// Built with -tags wasapi: "null", "file", and "wasapi" are available.
func NewPreviewOutputDevice(cfg *config.Config) (output.OutputDevice, error) {
	switch cfg.Preview.OutputDriver {
	case "null", "":
		return &output.NullOutput{}, nil
	case "file":
		return &output.FileOutput{}, nil
	case "wasapi":
		return waout.New(), nil
	case "portaudio":
		return nil, fmt.Errorf(
			"preview driver %q requires building with -tags portaudio (CGO + PortAudio system library)",
			cfg.Preview.OutputDriver,
		)
	case "coreaudio":
		return nil, fmt.Errorf(
			"preview driver %q requires building with -tags coreaudio (macOS only, CGO required)",
			cfg.Preview.OutputDriver,
		)
	default:
		return nil, fmt.Errorf("unknown preview output driver %q", cfg.Preview.OutputDriver)
	}
}
