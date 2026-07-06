//go:build wasapi && windows

// Package wasapi provides a WASAPI-backed OutputDevice for Windows.
// Build with: go build -tags wasapi ./...
//
// Requires CGO and the Windows SDK headers (available with MinGW-w64).
// No external Go or C libraries are needed beyond the standard Windows system
// libraries (ole32, oleaut32, uuid) that ship with every Windows installation.
//
// Device IDs are the stable GUIDs returned by IMMDevice::GetId(), which
// persist even when the device is renamed in Windows Sound Settings.
// Resolution in Open() follows the same cascade as the CoreAudio driver:
//
//	1. Try cfg.DeviceID as a GUID via IMMDeviceEnumerator::GetDevice.
//	2. Fall back to a friendly-name search across active render devices.
//	3. If neither matches, use the system default and log a warning.
package wasapi

/*
#cgo windows LDFLAGS: -lole32 -loleaut32 -luuid
#include "bridge.h"
#include <stdlib.h>
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

// Output implements output.OutputDevice using WASAPI shared-mode rendering.
//
// WASAPI uses a poll-based write model: Write() accumulates interleaved
// float32 frames in an internal buffer, then polls IAudioClient::GetCurrentPadding
// until the hardware has consumed enough data to accept the next chunk.
// Each poll cycle sleeps 1 ms, keeping CPU usage negligible while maintaining
// latency comparable to the engine's BufferFrames setting (typically ~42 ms
// at 48 kHz / 2048 frames).
type Output struct {
	mu  sync.Mutex
	cfg output.OutputConfig

	device  unsafe.Pointer // IMMDevice*        — nil when using system default
	client  unsafe.Pointer // IAudioClient*
	render  unsafe.Pointer // IAudioRenderClient*
	bufSize uint32         // WASAPI total buffer in frames

	accum  []float32
	accumN int

	opened  bool
	started bool
}

// New creates a new WASAPI Output and initialises COM (MTA) on the calling
// OS thread. No system audio resources are allocated until Open() is called.
func New() *Output {
	C.waCoInit()
	return &Output{}
}

// Open resolves the output device and opens a WASAPI shared-mode render stream.
// Resolution order: GUID → friendly name → system default.
func (o *Output) Open(_ context.Context, cfg output.OutputConfig) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.opened {
		return fmt.Errorf("wasapi: already open")
	}

	C.waCoInit() // ensure COM is initialised on this OS thread

	o.cfg = cfg

	var device unsafe.Pointer // nil → use default

	if cfg.DeviceID != "" && cfg.DeviceID != "default" {
		resolved := false

		// 1. Try as stable GUID (IMMDeviceEnumerator::GetDevice).
		cID := C.CString(cfg.DeviceID)
		if C.waFindDeviceByID(cID, &device) == 0 {
			resolved = true
		}
		C.free(unsafe.Pointer(cID))

		// 2. Fall back to friendly-name search.
		if !resolved {
			cName := C.CString(cfg.DeviceID)
			if C.waFindDeviceByName(cName, &device) == 0 {
				resolved = true
			}
			C.free(unsafe.Pointer(cName))
		}

		// 3. Neither matched — use system default and warn.
		if !resolved {
			fmt.Printf("wasapi: device %q not found by ID or name; using system default.\n",
				cfg.DeviceID)
		}
	}

	var client, render unsafe.Pointer
	var bufSize C.uint

	rc := C.waOpenRenderStream(
		device,
		C.int(cfg.SampleRate),
		C.int(cfg.Channels),
		&client,
		&render,
		&bufSize,
	)
	if rc != 0 {
		if device != nil {
			C.waReleaseDevice(device)
		}
		return fmt.Errorf("wasapi: OpenRenderStream: HRESULT 0x%08X", uint32(rc))
	}

	o.device = device
	o.client = client
	o.render = render
	o.bufSize = uint32(bufSize)
	o.accum = make([]float32, cfg.BufferFrames*cfg.Channels)
	o.accumN = 0
	o.opened = true
	return nil
}

// Start begins WASAPI playback (IAudioClient::Start).
func (o *Output) Start(_ context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened {
		return fmt.Errorf("wasapi: not open")
	}
	if o.started {
		return nil
	}
	if rc := C.waStartStream(o.client); rc != 0 {
		return fmt.Errorf("wasapi: Start: HRESULT 0x%08X", uint32(rc))
	}
	o.started = true
	return nil
}

// Write accumulates interleaved float32 frames and flushes a full buffer to
// WASAPI whenever cfg.BufferFrames samples have been collected.
//
// The method releases o.mu before polling for available WASAPI buffer space,
// allowing Stop/Close to proceed concurrently. It re-acquires o.mu before
// each write and verifies that the device is still open.
func (o *Output) Write(ctx context.Context, frames []float32) (int, error) {
	o.mu.Lock()
	if !o.opened {
		o.mu.Unlock()
		return 0, fmt.Errorf("wasapi: not open")
	}

	C.waCoInit() // ensure COM is initialised on this OS thread

	fullSize := o.cfg.BufferFrames * o.cfg.Channels
	src := frames

	for len(src) > 0 {
		// Accumulate until we have a full buffer.
		space := fullSize - o.accumN
		n := len(src)
		if n > space {
			n = space
		}
		copy(o.accum[o.accumN:o.accumN+n], src[:n])
		o.accumN += n
		src = src[n:]

		if o.accumN < fullSize {
			continue
		}

		framesToWrite := uint32(o.cfg.BufferFrames)
		client := o.client
		render := o.render
		bufSize := o.bufSize
		channels := o.cfg.Channels
		// Take a local reference so the slice survives lock release.
		accum := o.accum[:fullSize]

		// Release the lock before blocking so Stop/Close can proceed.
		o.mu.Unlock()

		// Poll until WASAPI has room for one full buffer.
		for {
			if ctx.Err() != nil {
				o.mu.Lock()
				o.accumN = 0
				o.mu.Unlock()
				return len(frames) / channels, nil
			}
			avail := int(C.waGetAvailableFrames(client, C.uint(bufSize)))
			if avail < 0 {
				o.mu.Lock()
				return 0, fmt.Errorf("wasapi: GetAvailableFrames failed")
			}
			if uint32(avail) >= framesToWrite {
				break
			}
			time.Sleep(time.Millisecond)
		}

		rc := C.waWriteFrames(
			client, render,
			(*C.float)(unsafe.Pointer(&accum[0])),
			C.uint(framesToWrite),
			C.int(channels),
		)

		o.mu.Lock()
		if !o.opened {
			// Device was closed while we were polling — return cleanly.
			o.mu.Unlock()
			return len(frames) / channels, nil
		}
		if rc != 0 {
			o.mu.Unlock()
			return 0, fmt.Errorf("wasapi: WriteFrames: HRESULT 0x%08X", uint32(rc))
		}
		o.accumN = 0
	}

	o.mu.Unlock()
	return len(frames) / o.cfg.Channels, nil
}

// Stop halts playback (IAudioClient::Stop) without releasing the device.
func (o *Output) Stop(_ context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || !o.started {
		return nil
	}
	if rc := C.waStopStream(o.client); rc != 0 {
		return fmt.Errorf("wasapi: Stop: HRESULT 0x%08X", uint32(rc))
	}
	o.started = false
	return nil
}

// Close stops playback and releases all WASAPI and COM resources.
func (o *Output) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened {
		return nil
	}
	if o.started {
		C.waStopStream(o.client)
		o.started = false
	}
	C.waReleaseRender(o.render)
	C.waReleaseClient(o.client)
	if o.device != nil {
		C.waReleaseDevice(o.device)
	}
	o.render = nil
	o.client = nil
	o.device = nil
	o.opened = false
	o.accumN = 0
	return nil
}

// Info returns static metadata about this output device.
func (o *Output) Info() output.OutputDeviceInfo {
	o.mu.Lock()
	defer o.mu.Unlock()
	return output.OutputDeviceInfo{
		ID:         o.cfg.DeviceID,
		Name:       o.cfg.DeviceID,
		Driver:     "wasapi",
		SampleRate: o.cfg.SampleRate,
		Channels:   o.cfg.Channels,
	}
}

// ListDevices enumerates all active WASAPI render devices.
// DeviceInfo.ID is the stable GUID from IMMDevice::GetId(), which survives
// device renames in Windows Sound Settings.
func (o *Output) ListDevices() ([]output.DeviceInfo, error) {
	C.waCoInit()
	const maxDevices = 64
	var cEntries [maxDevices]C.WADeviceEntry
	n := int(C.waEnumOutputDevices(&cEntries[0], C.int(maxDevices)))
	if n == 0 {
		return []output.DeviceInfo{}, nil
	}
	devs := make([]output.DeviceInfo, n)
	for i := 0; i < n; i++ {
		e := &cEntries[i]
		devs[i] = output.DeviceInfo{
			ID:                C.GoString(&e.id[0]),
			Name:              C.GoString(&e.name[0]),
			Driver:            "wasapi",
			HostAPI:           "WASAPI",
			IsDefault:         e.isDefault != 0,
			MaxOutputChannels: int(e.maxOutputChannels),
			DefaultSampleRate: float64(e.defaultSampleRate),
		}
	}
	return devs, nil
}
