//go:build coreaudio && !wasapi

package outfactory

import (
	"fmt"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	caout "github.com/Waelson/radio-playout-engine/internal/audio/output/coreaudio"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

// NewPreviewOutputDevice returns an OutputDevice for the preview (cue) player,
// selected by cfg.Preview.OutputDriver.
// Built with -tags coreaudio: "null", "file", and "coreaudio" are available.
func NewPreviewOutputDevice(cfg *config.Config) (output.OutputDevice, error) {
	switch cfg.Preview.OutputDriver {
	case "null", "":
		return &output.NullOutput{}, nil
	case "file":
		return &output.FileOutput{}, nil
	case "coreaudio":
		return caout.New(), nil
	case "portaudio":
		return nil, fmt.Errorf(
			"preview driver %q requires building with -tags portaudio (CGO + PortAudio system library)",
			cfg.Preview.OutputDriver,
		)
	default:
		return nil, fmt.Errorf("unknown preview output driver %q", cfg.Preview.OutputDriver)
	}
}
