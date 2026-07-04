//go:build portaudio

package outfactory

import (
	"fmt"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	paout "github.com/Waelson/radio-playout-engine/internal/audio/output/portaudio"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

func NewOutputDevice(cfg *config.Config) (output.OutputDevice, error) {
	switch cfg.Audio.Output.Driver {
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
	default:
		return nil, fmt.Errorf("unknown output driver %q", cfg.Audio.Output.Driver)
	}
}
