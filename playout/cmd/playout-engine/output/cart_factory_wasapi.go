//go:build wasapi

package outfactory

import (
	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	waout "github.com/Waelson/radio-playout-engine/internal/audio/output/wasapi"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

func NewCartOutputDevice(_ *config.Config) (output.OutputDevice, error) {
	return waout.New(), nil
}
