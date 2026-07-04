// Package audio defines the internal PCM format used throughout the engine.
// It imports nothing from the project to stay at the base of the dependency graph.
package audio

// AudioFormat describes the PCM format of an audio stream.
// The engine always uses a single internal format (see DefaultFormat).
type AudioFormat struct {
	SampleRate int // samples per second, e.g. 48000
	Channels   int // number of channels, e.g. 2 for stereo
}

// DefaultFormat is the engine's canonical internal PCM format:
// 48 kHz, stereo, float32 interleaved little-endian.
var DefaultFormat = AudioFormat{
	SampleRate: 48000,
	Channels:   2,
}

// SamplesPerFrame returns the number of float32 samples per frame.
// For stereo, each frame contains 2 samples (left + right).
func (f AudioFormat) SamplesPerFrame() int {
	return f.Channels
}

// BytesPerFrame returns the number of bytes per PCM frame.
// Each sample is a float32 (4 bytes).
func (f AudioFormat) BytesPerFrame() int {
	return f.Channels * 4
}

// FramesPerMs returns the number of frames in a given number of milliseconds.
func (f AudioFormat) FramesPerMs(ms int64) int64 {
	return int64(f.SampleRate) * ms / 1000
}

// MsFromFrames converts a frame count back to milliseconds.
func (f AudioFormat) MsFromFrames(frames int64) int64 {
	if f.SampleRate == 0 {
		return 0
	}
	return frames * 1000 / int64(f.SampleRate)
}
