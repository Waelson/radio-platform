// Package testutil provides test helpers shared across packages.
// It must not be imported by non-test production code.
package testutil

import (
	"context"
	"errors"
	"io"

	"github.com/Waelson/radio-playout-engine/internal/audio"
	"github.com/Waelson/radio-playout-engine/internal/audio/decoder"
)

// FakeDecoder is a Decoder implementation for unit tests.
// It returns silence frames without spawning any subprocess.
type FakeDecoder struct {
	// Frames is the total number of PCM frames the stream will return before EOF.
	// If zero the stream returns EOF immediately.
	Frames int
	// FailAfter makes the stream return an error after this many frames.
	// Zero means never fail.
	FailAfter int
	// OpenErr, if non-nil, is returned by Open() immediately.
	OpenErr error
}

// Open implements decoder.Decoder.
func (f *FakeDecoder) Open(_ context.Context, _ decoder.Source) (decoder.PCMStream, error) {
	if f.OpenErr != nil {
		return nil, f.OpenErr
	}
	return &fakeStream{
		remaining: f.Frames,
		failAfter: f.FailAfter,
	}, nil
}

// fakeStream implements decoder.PCMStream.
type fakeStream struct {
	remaining int
	failAfter int
	delivered int
}

// ReadFrames fills dst with silence (zeros) up to the remaining frame count.
func (s *fakeStream) ReadFrames(_ context.Context, dst []float32) (int, error) {
	if s.remaining == 0 {
		return 0, io.EOF
	}
	samplesPerFrame := audio.DefaultFormat.SamplesPerFrame()
	maxFrames := len(dst) / samplesPerFrame
	if maxFrames == 0 {
		return 0, nil
	}
	frames := maxFrames
	if frames > s.remaining {
		frames = s.remaining
	}

	// Check FailAfter threshold.
	if s.failAfter > 0 && s.delivered+frames > s.failAfter {
		frames = s.failAfter - s.delivered
		if frames <= 0 {
			return 0, errors.New("fake decoder: forced error after FailAfter frames")
		}
	}

	// Silence: dst is already zeroed by Go runtime.
	for i := 0; i < frames*samplesPerFrame; i++ {
		dst[i] = 0
	}
	s.remaining -= frames
	s.delivered += frames

	if s.failAfter > 0 && s.delivered >= s.failAfter {
		return frames, errors.New("fake decoder: forced error after FailAfter frames")
	}
	if s.remaining == 0 {
		return frames, io.EOF
	}
	return frames, nil
}

func (s *fakeStream) Close() error { return nil }

func (s *fakeStream) Format() audio.AudioFormat { return audio.DefaultFormat }
