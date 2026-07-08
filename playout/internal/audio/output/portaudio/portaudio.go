//go:build portaudio

// Package portaudio provides a PortAudio-backed OutputDevice.
// This package requires CGO and the PortAudio system library:
//
//	macOS:  brew install portaudio
//	Linux:  apt install libportaudio2-dev
//
// Compile with: go build -tags portaudio ./...
// Tests without the tag continue to use NullOutput and do not require CGO.
package portaudio

import (
	"context"
	"fmt"
	"sync"

	pa "github.com/gordonklaus/portaudio"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
)

// Output is an OutputDevice backed by PortAudio.
// gordonklaus/portaudio uses a non-callback (blocking) mode: a slice is bound
// to the stream at OpenStream time, and stream.Write() flushes that slice to
// the device, blocking until the hardware is ready for the next buffer.
//
// Because the playback pipeline may send partial buffers (fewer than
// BufferFrames frames), we maintain an accumulator that collects samples
// until a full buffer is available, then flushes to PortAudio.
type Output struct {
	mu     sync.Mutex
	cfg    output.OutputConfig
	stream *pa.Stream

	// paBuf is the slice bound to the stream at Open time.
	// stream.Write() reads from paBuf; we must fill it before each call.
	paBuf []float32

	// accum accumulates partial Write() calls until a full buffer is ready.
	accum  []float32
	accumN int // number of samples currently in accum

	opened bool
}

// New creates a new PortAudio-backed OutputDevice and initialises the
// PortAudio library. Callers must call Shutdown() when done (e.g. via defer)
// to release the PA library resources. Open/Close only manage the stream.
func New() (*Output, error) {
	if err := pa.Initialize(); err != nil {
		return nil, fmt.Errorf("portaudio: initialize: %w", err)
	}
	return &Output{}, nil
}

// Shutdown releases the PortAudio library. Call once when the process exits.
func (o *Output) Shutdown() error {
	if err := pa.Terminate(); err != nil {
		return fmt.Errorf("portaudio: terminate: %w", err)
	}
	return nil
}

// Open opens the output stream. pa.Initialize() has already been called by New().
func (o *Output) Open(_ context.Context, cfg output.OutputConfig) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.opened {
		return fmt.Errorf("portaudio: already open")
	}

	devInfo, err := o.resolveDevice(cfg.DeviceID)
	if err != nil {
		return err
	}

	params := pa.HighLatencyParameters(nil, devInfo)
	params.Output.Channels = cfg.Channels
	params.SampleRate = float64(cfg.SampleRate)
	params.FramesPerBuffer = cfg.BufferFrames

	fullSize := cfg.BufferFrames * cfg.Channels
	paBuf := make([]float32, fullSize)

	stream, err := pa.OpenStream(params, paBuf)
	if err != nil {
		return fmt.Errorf("portaudio: open stream: %w", err)
	}

	o.cfg = cfg
	o.stream = stream
	o.paBuf = paBuf
	o.accum = make([]float32, fullSize)
	o.accumN = 0
	o.opened = true
	return nil
}

// Start begins audio playback.
func (o *Output) Start(_ context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened {
		return fmt.Errorf("portaudio: not open")
	}
	if err := o.stream.Start(); err != nil {
		return fmt.Errorf("portaudio: start stream: %w", err)
	}
	return nil
}

// Write accumulates PCM samples and flushes to PortAudio whenever a full
// buffer (BufferFrames * Channels samples) has been collected.
// Partial trailing data is held in the accumulator for the next call.
func (o *Output) Write(ctx context.Context, frames []float32) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.opened {
		return 0, fmt.Errorf("portaudio: not open")
	}

	fullSize := len(o.paBuf)
	totalFramesWritten := 0
	src := frames

	for len(src) > 0 {
		// How many samples fit in the remainder of the accumulator?
		space := fullSize - o.accumN
		n := len(src)
		if n > space {
			n = space
		}
		copy(o.accum[o.accumN:o.accumN+n], src[:n])
		o.accumN += n
		src = src[n:]

		if o.accumN == fullSize {
			copy(o.paBuf, o.accum)
			if err := o.stream.Write(); err != nil {
				if ctx.Err() != nil {
					return totalFramesWritten, nil
				}
				// OutputUnderflowed means the hardware ran dry (e.g. after a
				// pause). It is recoverable — the stream is still usable.
				if err == pa.OutputUnderflowed {
					totalFramesWritten += fullSize / o.cfg.Channels
					o.accumN = 0
					continue
				}
				return totalFramesWritten, fmt.Errorf("portaudio: write: %w", err)
			}
			totalFramesWritten += fullSize / o.cfg.Channels
			o.accumN = 0
		}
	}

	return len(frames) / o.cfg.Channels, nil
}

// Stop halts audio output without releasing the device.
func (o *Output) Stop(_ context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || o.stream == nil {
		return nil
	}
	if err := o.stream.Stop(); err != nil {
		return fmt.Errorf("portaudio: stop stream: %w", err)
	}
	return nil
}

// PauseAudio suspends the stream without closing it.
// Implements the optional PauseAudio/ResumeAudio contract used by preview.Player
// to keep the device warm between cue sessions.
func (o *Output) PauseAudio() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || o.stream == nil {
		return nil
	}
	if err := o.stream.Stop(); err != nil {
		return fmt.Errorf("portaudio: pause (stop): %w", err)
	}
	return nil
}

// ResumeAudio restarts a stream previously suspended by PauseAudio.
func (o *Output) ResumeAudio() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || o.stream == nil {
		return fmt.Errorf("portaudio: resume: stream not open")
	}
	if err := o.stream.Start(); err != nil {
		return fmt.Errorf("portaudio: resume (start): %w", err)
	}
	return nil
}

// Close closes the output stream. The PortAudio library stays initialised;
// call Shutdown() to release it. After Close, Open may be called again.
func (o *Output) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened {
		return nil
	}
	var firstErr error
	if o.stream != nil {
		if err := o.stream.Close(); err != nil {
			firstErr = fmt.Errorf("portaudio: close stream: %w", err)
		}
		o.stream = nil
	}
	o.opened = false
	o.accumN = 0
	return firstErr
}

// Info returns static metadata about this output device.
func (o *Output) Info() output.OutputDeviceInfo {
	o.mu.Lock()
	defer o.mu.Unlock()
	return output.OutputDeviceInfo{
		ID:         o.cfg.DeviceID,
		Name:       o.cfg.DeviceID,
		Driver:     "portaudio",
		SampleRate: o.cfg.SampleRate,
		Channels:   o.cfg.Channels,
	}
}

// ListDevices enumerates all audio output devices available via PortAudio.
// pa.Initialize() must already have been called (done by New()).
//
// Because PortAudio does not expose a persistent device UID, DeviceInfo.ID is
// set equal to DeviceInfo.Name. If the device is renamed in the OS, the ID
// changes accordingly — consistent with how resolveDevice selects devices.
func (o *Output) ListDevices() ([]output.DeviceInfo, error) {
	defaultDev, _ := pa.DefaultOutputDevice()

	devs, err := pa.Devices()
	if err != nil {
		return nil, fmt.Errorf("portaudio: list devices: %w", err)
	}

	result := make([]output.DeviceInfo, 0, len(devs))
	for _, d := range devs {
		if d.MaxOutputChannels <= 0 {
			continue
		}
		isDefault := defaultDev != nil && d.Name == defaultDev.Name
		hostAPI := ""
		if d.HostApi != nil {
			hostAPI = d.HostApi.Name
		}
		result = append(result, output.DeviceInfo{
			ID:                d.Name,
			Name:              d.Name,
			Driver:            "portaudio",
			HostAPI:           hostAPI,
			IsDefault:         isDefault,
			MaxOutputChannels: d.MaxOutputChannels,
			DefaultSampleRate: d.DefaultSampleRate,
		})
	}
	return result, nil
}

// resolveDevice returns the PortAudio DeviceInfo for the requested device ID.
// When matching by name, only devices with MaxOutputChannels > 0 are considered
// to avoid returning the input-only variant of a device (e.g. AirPods microphone)
// when an output stream is needed.
func (o *Output) resolveDevice(deviceID string) (*pa.DeviceInfo, error) {
	if deviceID == "" || deviceID == "default" {
		dev, err := pa.DefaultOutputDevice()
		if err != nil {
			return nil, fmt.Errorf("portaudio: default output device: %w", err)
		}
		return dev, nil
	}
	devs, err := pa.Devices()
	if err != nil {
		return nil, fmt.Errorf("portaudio: list devices: %w", err)
	}
	for _, d := range devs {
		if d.Name == deviceID && d.MaxOutputChannels > 0 {
			return d, nil
		}
	}
	return nil, fmt.Errorf("portaudio: output device %q not found", deviceID)
}
