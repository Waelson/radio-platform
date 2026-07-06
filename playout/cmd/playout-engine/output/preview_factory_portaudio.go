//go:build portaudio

package outfactory

import (
	"fmt"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	paout "github.com/Waelson/radio-playout-engine/internal/audio/output/portaudio"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

// NewPreviewOutputDevice returns an OutputDevice for the preview (cue) player,
// selected by cfg.Preview.OutputDriver.
// Built with -tags portaudio: "null", "file", and "portaudio" are available.
func NewPreviewOutputDevice(cfg *config.Config) (output.OutputDevice, error) {
	switch cfg.Preview.OutputDriver {
	case "null", "":
		return &output.NullOutput{}, nil
	case "file":
		return &output.FileOutput{}, nil
	case "portaudio":
		out, err := paout.New()
		if err != nil {
			return nil, err
		}
		return out, nil
	case "coreaudio":
		return nil, fmt.Errorf(
			"preview driver %q requires building with -tags coreaudio (macOS only, CGO required)",
			cfg.Preview.OutputDriver,
		)
	default:
		return nil, fmt.Errorf("unknown preview output driver %q", cfg.Preview.OutputDriver)
	}
}
