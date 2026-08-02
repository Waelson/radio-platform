//go:build coreaudio && darwin

// Package coreaudio provides a CoreAudio AudioQueue-backed OutputDevice.
// Build with: go build -tags coreaudio ./...
// Requires macOS; no external Go dependencies beyond the system frameworks.
package coreaudio

/*
#cgo LDFLAGS: -framework AudioToolbox -framework CoreFoundation -framework CoreAudio
#include "bridge.h"
*/
import "C"
import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
)

// numBuffers is the number of AudioQueue buffers that circulate between the
// queue and the pull callback. More buffers = smoother playback at the cost of
// higher latency. 3 × 2048 frames @ 48 kHz ≈ 128 ms total queue latency.
const numBuffers = 3

// ringBufSamples is the ring buffer capacity in float32 samples.
// 48000 Hz × 2 channels × 2 s = 192000 → rounded up to 2^18 = 262144 by caRingCreate.
// This gives ~2.7 s of stereo audio headroom at 48 kHz.
const ringBufSamples = 48000 * 2 * 2

// Output implements output.OutputDevice using CoreAudio AudioQueue in pull mode.
//
// Architecture:
//
//	Go (Write) → ring buffer → C callback (pullCallback) → AudioQueue hardware
//
// The C callback is invoked by CoreAudio whenever it finishes playing a buffer.
// It reads from the ring buffer (or serves silence when empty) and ALWAYS
// re-enqueues the buffer. The AudioQueue therefore NEVER auto-stops — gaps in
// the PCM supply produce silence, not a hung or stopped queue.
//
// Pause/resume still use AudioQueuePause/AudioQueueStart. Write() returns
// immediately when PauseAudio() is called so the playback loop does not block
// inside the output device.
type Output struct {
	mu  sync.Mutex
	cfg output.OutputConfig

	queue C.AudioQueueRef
	cBufs [numBuffers]C.AudioQueueBufferRef
	ring  unsafe.Pointer // *C.CARingBuf — allocated in C, freed in Close()

	opened  bool
	started bool

	// pauseSig is closed by PauseAudio() to immediately unblock Write() that
	// is polling for ring buffer space while the queue is paused.
	// Replaced with a fresh channel after each pause.
	pauseSig chan struct{}
}

// New creates a new CoreAudio Output. No system resources are allocated
// until Open() is called.
func New() *Output {
	return &Output{}
}

// Open initialises the ring buffer, AudioQueue, and buffer pool.
func (o *Output) Open(_ context.Context, cfg output.OutputConfig) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.opened {
		return fmt.Errorf("coreaudio: already open")
	}

	o.cfg = cfg
	o.pauseSig = make(chan struct{})

	// Allocate the SPSC ring buffer in C.
	o.ring = unsafe.Pointer(C.caRingCreate(C.uint32_t(ringBufSamples)))
	if o.ring == nil {
		return fmt.Errorf("coreaudio: failed to allocate ring buffer")
	}

	// Create the AudioQueue in pull mode. The C callback reads from the ring
	// buffer and always re-enqueues — the queue never auto-stops.
	status := C.caNewQueuePull(
		C.double(cfg.SampleRate),
		C.int(cfg.Channels),
		(*C.CARingBuf)(o.ring),
		&o.queue,
	)
	if status != 0 {
		C.caRingFree((*C.CARingBuf)(o.ring))
		o.ring = nil
		return fmt.Errorf("coreaudio: AudioQueueNewOutput: OSStatus %d", int(status))
	}

	// Route to a specific device when DeviceID is set and not "default".
	if cfg.DeviceID != "" && cfg.DeviceID != "default" {
		var devID C.AudioDeviceID
		resolved := false

		cUID := C.CString(cfg.DeviceID)
		if C.caFindDeviceByUID(cUID, &devID) == 0 {
			resolved = true
		}
		C.free(unsafe.Pointer(cUID))

		if !resolved {
			cName := C.CString(cfg.DeviceID)
			if C.caFindDeviceByName(cName, &devID) == 0 {
				resolved = true
			}
			C.free(unsafe.Pointer(cName))
		}

		if !resolved {
			var listBuf [4096]C.char
			C.caListOutputDevices(&listBuf[0], 4096)
			fmt.Printf("coreaudio: device %q not found; using system default.\nAvailable:\n%s",
				cfg.DeviceID, C.GoString(&listBuf[0]))
		} else if st := C.caSetQueueDevice(o.queue, devID); st != 0 {
			fmt.Printf("coreaudio: set device %q failed (OSStatus %d); using system default.\n",
				cfg.DeviceID, int(st))
		}
	}

	// Allocate the AudioQueue buffer pool.
	bufFrames := cfg.BufferFrames
	if bufFrames == 0 {
		bufFrames = 2048
	}
	for i := 0; i < numBuffers; i++ {
		status = C.caAllocBuffer(o.queue, C.int(bufFrames), C.int(cfg.Channels), &o.cBufs[i])
		if status != 0 {
			C.AudioQueueDispose(o.queue, C.Boolean(1))
			C.caRingFree((*C.CARingBuf)(o.ring))
			o.ring = nil
			return fmt.Errorf("coreaudio: AllocBuffer[%d]: OSStatus %d", i, int(status))
		}
	}

	o.opened = true
	return nil
}

// Start kicks off the pull-model circular buffer loop and begins playback.
// All AudioQueue buffers are pre-enqueued with silence; the C callback takes
// over from there, re-enqueuing each buffer after filling it from the ring.
func (o *Output) Start(_ context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened {
		return fmt.Errorf("coreaudio: not open")
	}
	if o.started {
		return nil
	}

	// Enqueue all buffers filled with silence to start the callback loop.
	for i := 0; i < numBuffers; i++ {
		if status := C.caEnqueueSilent(o.queue, o.cBufs[i]); status != 0 {
			return fmt.Errorf("coreaudio: caEnqueueSilent[%d]: OSStatus %d", i, int(status))
		}
	}

	if status := C.AudioQueueStart(o.queue, nil); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueueStart: OSStatus %d", int(status))
	}
	o.started = true
	return nil
}

// Write copies interleaved float32 PCM frames into the ring buffer.
//
// If the ring buffer is full (queue paused for too long), Write blocks briefly
// (polling every millisecond) until space is available. It returns immediately
// when ctx is cancelled or PauseAudio() is called.
//
// Because the C callback continuously reads from the ring, Write never needs to
// interact with AudioQueue buffers directly — no idle-running, no auto-stop.
func (o *Output) Write(ctx context.Context, frames []float32) (int, error) {
	o.mu.Lock()
	if !o.opened {
		o.mu.Unlock()
		return 0, fmt.Errorf("coreaudio: not open")
	}
	pauseSig := o.pauseSig
	ring := o.ring
	o.mu.Unlock()

	src := frames
	for len(src) > 0 {
		n := int(C.caRingWrite(
			(*C.CARingBuf)(ring),
			(*C.float)(unsafe.Pointer(&src[0])),
			C.uint32_t(len(src)),
		))
		src = src[n:]
		if len(src) == 0 {
			break
		}

		// Ring buffer is full — wait briefly for the callback to drain it.
		// This path is hit only when the queue is paused for an extended period.
		select {
		case <-ctx.Done():
			return (len(frames) - len(src)) / o.cfg.Channels, nil
		case <-pauseSig:
			// PauseAudio() was called; return so the playback loop can reach
			// its pause-wait point.
			return 0, nil
		case <-time.After(time.Millisecond):
			// Retry after a short wait.
		}
	}

	return len(frames) / o.cfg.Channels, nil
}

// RingOccupancy returns the number of float32 samples currently buffered in the
// hardware ring — written by Go but not yet consumed by the CoreAudio callback.
// Dividing by (sampleRate × channels) converts the value to seconds of lag.
// Returns 0 when the device is not open.
func (o *Output) RingOccupancy() int64 {
	o.mu.Lock()
	ring := o.ring
	o.mu.Unlock()
	if ring == nil {
		return 0
	}
	return int64(C.caRingAvail((*C.CARingBuf)(ring)))
}

// FlushAudio drains the ring buffer immediately, silencing any audio that was
// written ahead of real-time but not yet played by the hardware callback.
// The AudioQueue keeps running — subsequent Write calls refill the ring normally.
// Call this after a playback session stops to eliminate the tail of pre-buffered
// audio that would otherwise continue playing for up to ~2.7 s.
func (o *Output) FlushAudio() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened {
		return nil
	}
	C.caRingDrain((*C.CARingBuf)(o.ring))
	return nil
}

// PauseAudio suspends the AudioQueue hardware output without draining the ring
// buffer, then signals any blocked Write() to return immediately.
func (o *Output) PauseAudio() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || !o.started {
		return nil
	}
	// Unblock any Write() waiting for ring buffer space.
	close(o.pauseSig)
	o.pauseSig = make(chan struct{})

	if status := C.AudioQueuePause(o.queue); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueuePause: OSStatus %d", int(status))
	}
	return nil
}

// ResumeAudio restarts the AudioQueue from where it was paused.
// The ring buffer retains any data accumulated during the pause.
func (o *Output) ResumeAudio() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || !o.started {
		return nil
	}
	if status := C.AudioQueueStart(o.queue, nil); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueueStart (resume): OSStatus %d", int(status))
	}
	return nil
}

// Stop halts AudioQueue playback and drains the ring buffer.
func (o *Output) Stop(_ context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || !o.started {
		return nil
	}
	// Discard buffered audio so the next session starts clean.
	C.caRingDrain((*C.CARingBuf)(o.ring))
	if status := C.AudioQueueStop(o.queue, C.Boolean(1)); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueueStop: OSStatus %d", int(status))
	}
	o.started = false
	return nil
}

// Close disposes the AudioQueue and frees the ring buffer.
func (o *Output) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened {
		return nil
	}
	if status := C.AudioQueueDispose(o.queue, C.Boolean(1)); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueueDispose: OSStatus %d", int(status))
	}
	C.caRingFree((*C.CARingBuf)(o.ring))
	o.ring = nil
	o.opened = false
	o.started = false
	return nil
}

// ListDevices enumerates all audio output devices available on the system.
func (o *Output) ListDevices() ([]output.DeviceInfo, error) {
	const maxDevices = 64
	var cEntries [maxDevices]C.CADeviceEntry
	n := int(C.caEnumOutputDevices(&cEntries[0], C.int(maxDevices)))
	if n == 0 {
		return []output.DeviceInfo{}, nil
	}
	devs := make([]output.DeviceInfo, n)
	for i := 0; i < n; i++ {
		e := &cEntries[i]
		devs[i] = output.DeviceInfo{
			ID:                C.GoString(&e.uid[0]),
			Name:              C.GoString(&e.name[0]),
			Driver:            "coreaudio",
			HostAPI:           "CoreAudio",
			IsDefault:         e.isDefault != 0,
			MaxOutputChannels: int(e.maxOutputChannels),
			DefaultSampleRate: float64(e.defaultSampleRate),
		}
	}
	return devs, nil
}

// Info returns static metadata about this device.
func (o *Output) Info() output.OutputDeviceInfo {
	o.mu.Lock()
	defer o.mu.Unlock()
	return output.OutputDeviceInfo{
		ID:         o.cfg.DeviceID,
		Name:       o.cfg.DeviceID,
		Driver:     "coreaudio",
		SampleRate: o.cfg.SampleRate,
		Channels:   o.cfg.Channels,
	}
}
