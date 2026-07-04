// Package decoder defines the Decoder and PCMStream interfaces used by the
// Playback Manager to open audio files and read raw PCM data.
//
// This package imports only audio (format) — never api, state, queue, or output.
// The Playback Manager is responsible for converting a queue.QueueItem into a
// Source before calling Open.
package decoder

import (
	"context"

	"github.com/Waelson/radio-playout-engine/internal/audio"
)

// Source carries the audio source parameters that a Decoder needs.
// It is populated by the Playback Manager from a queue.QueueItem.
type Source struct {
	// Path is the absolute filesystem path to the audio file.
	Path string

	// CueInMS is the start offset in milliseconds. 0 means the beginning.
	CueInMS int64

	// CueOutMS is the end offset in milliseconds. 0 means play to EOF.
	CueOutMS int64
}

// Decoder opens audio sources and returns a PCMStream for reading.
type Decoder interface {
	// Open opens src and returns a PCMStream positioned at CueInMS.
	// The context governs the lifetime of the subprocess or open handle.
	// The caller must call PCMStream.Close when done.
	Open(ctx context.Context, src Source) (PCMStream, error)
}

// PCMStream is a seeked, readable stream of interleaved float32 PCM samples
// in the engine's internal format (see audio.DefaultFormat).
type PCMStream interface {
	// ReadFrames reads audio frames into dst. dst length must be a multiple
	// of Format().Channels. Returns the number of frames read and any error.
	// io.EOF is returned (with n > 0 possible) when the source is exhausted.
	ReadFrames(ctx context.Context, dst []float32) (frames int, err error)

	// Close releases all resources held by this stream.
	Close() error

	// Format returns the PCM format of the samples in this stream.
	Format() audio.AudioFormat
}
