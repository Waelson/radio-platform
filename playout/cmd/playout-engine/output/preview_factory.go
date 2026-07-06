//go:build !portaudio && !coreaudio

package outfactory

import (
	"fmt"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

// NewPreviewOutputDevice returns an OutputDevice for the preview (cue) player,
// selected by cfg.Preview.OutputDriver.
// Without build tags only "null" and "file" are available.
func NewPreviewOutputDevice(cfg *config.Config) (output.OutputDevice, error) {
	switch cfg.Preview.OutputDriver {
	case "null", "":
		return &output.NullOutput{}, nil
	case "file":
		return &output.FileOutput{}, nil
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
