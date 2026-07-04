// Package output defines the OutputDevice interface and its implementations.
// This package imports audio (format) but must never import api, state, queue,
// or playback — audio must remain decoupled from the control plane.
package output

import (
	"context"
	"math"
)

// OutputDevice is the contract every audio output adapter must satisfy.
// The engine interacts exclusively through this interface, so individual
// drivers (NullOutput, FileOutput, PortAudioOutput) are interchangeable.
type OutputDevice interface {
	// Open initialises the device with the given configuration.
	// Must be called before Start or Write.
	Open(ctx context.Context, cfg OutputConfig) error

	// Start begins device playback (e.g. opens the OS audio stream).
	Start(ctx context.Context) error

	// Write sends interleaved float32 PCM frames to the device.
	// Returns the number of frames written and any error.
	// frames length must be a multiple of cfg.Channels.
	Write(ctx context.Context, frames []float32) (int, error)

	// Stop halts playback without releasing the device.
	Stop(ctx context.Context) error

	// Close releases all resources held by the device.
	// After Close, Open must be called again before use.
	Close() error

	// Info returns static metadata about this device.
	Info() OutputDeviceInfo
}

// OutputConfig carries the parameters used to open an OutputDevice.
type OutputConfig struct {
	DeviceID     string // device name or "default"
	SampleRate   int    // e.g. 48000
	Channels     int    // e.g. 2
	BufferFrames int    // e.g. 2048
}

// OutputDeviceInfo carries read-only metadata about an opened device.
type OutputDeviceInfo struct {
	ID         string
	Name       string
	Driver     string
	SampleRate int
	Channels   int
}

// DBToLinear converts a dB gain value to a linear amplitude multiplier.
// 0 dB → 1.0, -6 dB → ~0.5, -∞ dB → 0.0.
func DBToLinear(db float64) float64 {
	if db <= -144 {
		return 0
	}
	return math.Pow(10, db/20)
}
