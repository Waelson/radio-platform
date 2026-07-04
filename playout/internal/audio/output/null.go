package output

import (
	"context"
	"sync/atomic"
	"time"
)

// NullOutput is an OutputDevice that discards every audio frame without
// sending anything to a hardware device. It is used in all automated tests.
//
// When Realtime is true, Write sleeps to simulate the wall-clock duration of
// each buffer (frames/sampleRate seconds), making it suitable for integration
// tests that track real-time progress.
//
// ForceWriteErr, if non-nil, is returned by Write to test error-handling paths.
type NullOutput struct {
	Realtime      bool
	ForceWriteErr error

	cfg     OutputConfig
	opened  atomic.Bool
	started atomic.Bool
	written atomic.Int64 // frames written since last Reset
}

var _ OutputDevice = (*NullOutput)(nil)

func (n *NullOutput) Open(_ context.Context, cfg OutputConfig) error {
	n.cfg = cfg
	n.opened.Store(true)
	return nil
}

func (n *NullOutput) Start(_ context.Context) error {
	n.started.Store(true)
	return nil
}

// Write discards frames, optionally sleeping to simulate real-time output.
func (n *NullOutput) Write(ctx context.Context, frames []float32) (int, error) {
	if n.ForceWriteErr != nil {
		return 0, n.ForceWriteErr
	}

	channels := n.cfg.Channels
	if channels == 0 {
		channels = 2
	}
	sampleRate := n.cfg.SampleRate
	if sampleRate == 0 {
		sampleRate = 48000
	}

	nFrames := len(frames) / channels
	n.written.Add(int64(nFrames))

	if n.Realtime && nFrames > 0 {
		dur := time.Duration(float64(nFrames) / float64(sampleRate) * float64(time.Second))
		select {
		case <-time.After(dur):
		case <-ctx.Done():
			return nFrames, ctx.Err()
		}
	}

	return nFrames, nil
}

func (n *NullOutput) Stop(_ context.Context) error {
	n.started.Store(false)
	return nil
}

func (n *NullOutput) Close() error {
	n.opened.Store(false)
	n.started.Store(false)
	return nil
}

func (n *NullOutput) Info() OutputDeviceInfo {
	return OutputDeviceInfo{
		ID:         "null",
		Name:       "NullOutput",
		Driver:     "null",
		SampleRate: n.cfg.SampleRate,
		Channels:   n.cfg.Channels,
	}
}

// FramesWritten returns the cumulative number of frames written since the last
// Reset (or creation). Safe to call from any goroutine.
func (n *NullOutput) FramesWritten() int64 {
	return n.written.Load()
}

// Reset zeroes the frame counter. Useful between test assertions.
func (n *NullOutput) Reset() {
	n.written.Store(0)
}
