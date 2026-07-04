//go:build coreaudio

package outfactory

import (
	"fmt"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	caout "github.com/Waelson/radio-playout-engine/internal/audio/output/coreaudio"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

func NewOutputDevice(cfg *config.Config) (output.OutputDevice, error) {
	switch cfg.Audio.Output.Driver {
	case "null", "":
		return &output.NullOutput{}, nil
	case "file":
		return &output.FileOutput{}, nil
	case "coreaudio":
		return caout.New(), nil
	case "portaudio":
		return nil, fmt.Errorf(
			"driver %q requires building with -tags portaudio (CGO + PortAudio system library)",
			cfg.Audio.Output.Driver,
		)
	default:
		return nil, fmt.Errorf("unknown output driver %q", cfg.Audio.Output.Driver)
	}
}
