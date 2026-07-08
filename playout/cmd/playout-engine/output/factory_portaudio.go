//go:build portaudio && !wasapi

package outfactory

import (
	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	paout "github.com/Waelson/radio-playout-engine/internal/audio/output/portaudio"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

func NewOutputDevice(_ *config.Config) (output.OutputDevice, error) {
	return paout.New()
}
