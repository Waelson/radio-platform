//go:build !portaudio && !coreaudio && !wasapi

package outfactory

import (
	"github.com/Waelson/radio-playout-engine/internal/audio/output"
	"github.com/Waelson/radio-playout-engine/internal/config"
)

// NewOutputDevice returns the NullOutput for binaries built without a driver tag.
// The driver is determined at compile-time via build tag (-tags coreaudio, -tags portaudio, etc.).
func NewOutputDevice(_ *config.Config) (output.OutputDevice, error) {
	return &output.NullOutput{}, nil
}
