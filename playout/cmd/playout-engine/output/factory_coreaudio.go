//go:build coreaudio && !wasapi

package outfactory

import (
	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	caout "github.com/Waelson/radio-playout-engine/internal/audio/output/coreaudio"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

func NewOutputDevice(_ *config.Config) (output.OutputDevice, error) {
	return caout.New(), nil
}
