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
	"runtime/cgo"
	"sync"
	"unsafe"

	"github.com/Waelson/radio-playout-engine/internal/audio/output"
)

const numBuffers = 3

// Output implements output.OutputDevice using CoreAudio AudioQueue.
// AudioQueue is push-based: Go calls Write() which accumulates frames and
// enqueues them; CoreAudio calls the C callback when a buffer is consumed,
// which returns it to the freeBufs pool via goBufferReady.
//
// Pause/resume are handled via AudioQueuePause / AudioQueueStart so that
// buffered data is preserved across pauses of arbitrary length — the queue
// never drains and never auto-stops.
// PauseAudio() closes pauseSig to unblock any Write() that is waiting for a
// free buffer, then pauses the AudioQueue. ResumeAudio() restarts the queue
// before the Go playback loop is unblocked, ensuring seamless audio.
type Output struct {
	mu  sync.Mutex
	cfg output.OutputConfig

	queue    C.AudioQueueRef
	cBufs    [numBuffers]C.AudioQueueBufferRef
	freeBufs chan C.AudioQueueBufferRef // buffers returned by CoreAudio callback

	handle cgo.Handle // safe opaque reference passed to C as userData

	accum  []float32 // accumulates partial Write() calls
	accumN int

	opened  bool
	started bool

	// pauseSig is closed by PauseAudio() to immediately unblock any Write()
	// that is waiting on freeBufs. It is replaced with a fresh channel each
	// time PauseAudio() is called so subsequent Write() calls are not
	// affected. Access is protected by mu.
	pauseSig chan struct{}
}

// New creates a new CoreAudio Output. No system resources are allocated
// until Open() is called.
func New() *Output {
	return &Output{}
}

// goBufferReady is called from the C AudioQueue callback when CoreAudio has
// finished consuming a buffer. It recovers the Output via cgo.Handle and
// returns the buffer to the freeBufs pool so Write() can reuse it.
//
//export goBufferReady
func goBufferReady(userData unsafe.Pointer, _ C.AudioQueueRef, buf C.AudioQueueBufferRef) {
	o := cgo.Handle(uintptr(userData)).Value().(*Output)
	select {
	case o.freeBufs <- buf:
	default:
		// Channel full: engine stopped or not consuming — drop silently.
	}
}

// Open initialises the AudioQueue and allocates the buffer pool.
func (o *Output) Open(_ context.Context, cfg output.OutputConfig) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.opened {
		return fmt.Errorf("coreaudio: already open")
	}

	o.cfg = cfg
	o.freeBufs = make(chan C.AudioQueueBufferRef, numBuffers)
	o.pauseSig = make(chan struct{})

	// cgo.Handle stores the Go pointer in a table keyed by an integer.
	// Passing the integer (uintptr) to C is safe: no Go pointer crosses the boundary.
	o.handle = cgo.NewHandle(o)
	userData := unsafe.Pointer(uintptr(o.handle))

	// Create the AudioQueue output stream.
	status := C.caNewQueue(
		C.double(cfg.SampleRate),
		C.int(cfg.Channels),
		userData,
		&o.queue,
	)
	if status != 0 {
		o.handle.Delete()
		return fmt.Errorf("coreaudio: AudioQueueNewOutput: OSStatus %d", int(status))
	}

	// Route to a specific device when DeviceID is set and not "default".
	// Resolution order: UID → name → system default.
	// Using UID is more robust: it survives device renames. Using name is
	// backward-compatible with existing configs.
	if cfg.DeviceID != "" && cfg.DeviceID != "default" {
		var devID C.AudioDeviceID
		resolved := false

		// 1. Try UID (kAudioDevicePropertyDeviceUID) — stable across renames.
		cUID := C.CString(cfg.DeviceID)
		if C.caFindDeviceByUID(cUID, &devID) == 0 {
			resolved = true
		}
		C.free(unsafe.Pointer(cUID))

		// 2. Fall back to name lookup.
		if !resolved {
			cName := C.CString(cfg.DeviceID)
			if C.caFindDeviceByName(cName, &devID) == 0 {
				resolved = true
			}
			C.free(unsafe.Pointer(cName))
		}

		// 3. Neither UID nor name matched — use system default and warn.
		if !resolved {
			var listBuf [4096]C.char
			C.caListOutputDevices(&listBuf[0], 4096)
			fmt.Printf("coreaudio: device %q not found by UID or name; using system default.\nAvailable output devices:\n%s",
				cfg.DeviceID, C.GoString(&listBuf[0]))
		} else if st := C.caSetQueueDevice(o.queue, devID); st != 0 {
			fmt.Printf("coreaudio: set device %q failed (OSStatus %d); using system default.\n", cfg.DeviceID, int(st))
		}
	}

	// Allocate the buffer pool and pre-fill freeBufs.
	for i := 0; i < numBuffers; i++ {
		status = C.caAllocBuffer(o.queue,
			C.int(cfg.BufferFrames), C.int(cfg.Channels), &o.cBufs[i])
		if status != 0 {
			_ = C.AudioQueueDispose(o.queue, C.Boolean(1))
			o.handle.Delete()
			return fmt.Errorf("coreaudio: AllocBuffer[%d]: OSStatus %d", i, int(status))
		}
		o.freeBufs <- o.cBufs[i]
	}

	o.accum = make([]float32, cfg.BufferFrames*cfg.Channels)
	o.accumN = 0
	o.opened = true
	return nil
}

// Start begins AudioQueue playback.
func (o *Output) Start(_ context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened {
		return fmt.Errorf("coreaudio: not open")
	}
	if o.started {
		return nil
	}
	if status := C.AudioQueueStart(o.queue, nil); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueueStart: OSStatus %d", int(status))
	}
	o.started = true
	return nil
}

// Write accumulates interleaved float32 frames and enqueues full buffers to
// the AudioQueue. Blocks until a free buffer is available, ctx is cancelled,
// or PauseAudio() signals via pauseSig.
//
// When pauseSig fires (pause requested), Write() returns (0, nil) immediately
// so the playback loop can reach the pause-wait point without deadlock.
//
// Important: o.mu is released while blocking on freeBufs so that goBufferReady
// (called from C on a CoreAudio thread) can send to the channel without deadlock.
func (o *Output) Write(ctx context.Context, frames []float32) (int, error) {
	o.mu.Lock()
	if !o.opened {
		o.mu.Unlock()
		return 0, fmt.Errorf("coreaudio: not open")
	}

	fullSize := o.cfg.BufferFrames * o.cfg.Channels
	src := frames

	for len(src) > 0 {
		space := fullSize - o.accumN
		n := len(src)
		if n > space {
			n = space
		}
		copy(o.accum[o.accumN:o.accumN+n], src[:n])
		o.accumN += n
		src = src[n:]

		if o.accumN == fullSize {
			// Snapshot pauseSig under the lock before releasing it.
			// PauseAudio() may replace pauseSig (under the same lock) while we
			// are in the select below — we want to wake on the OLD channel.
			pauseSig := o.pauseSig

			// Release lock before blocking; goBufferReady must be able to send.
			o.mu.Unlock()
			var buf C.AudioQueueBufferRef
			select {
			case <-ctx.Done():
				return len(frames) / o.cfg.Channels, nil
			case buf = <-o.freeBufs:
			case <-pauseSig:
				// PauseAudio() was called; return immediately so the playback
				// loop can reach its pause-wait point. The partial accumulation
				// buffer was already reset by PauseAudio().
				return 0, nil
			}
			o.mu.Lock()

			status := C.caEnqueueBuffer(
				o.queue, buf,
				(*C.float)(unsafe.Pointer(&o.accum[0])),
				C.int(o.cfg.BufferFrames),
				C.int(o.cfg.Channels),
			)
			if status != 0 {
				o.mu.Unlock()
				return 0, fmt.Errorf("coreaudio: EnqueueBuffer: OSStatus %d", int(status))
			}
			o.accumN = 0
		}
	}

	o.mu.Unlock()
	return len(frames) / o.cfg.Channels, nil
}

// PauseAudio suspends the AudioQueue without draining its buffered data, then
// signals Write() to return immediately so the playback loop is not blocked
// inside the output device during a pause.
//
// Called by the playback manager via interface type-assertion:
//
//	if p, ok := m.out.(interface{ PauseAudio() error }); ok { p.PauseAudio() }
func (o *Output) PauseAudio() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || !o.started {
		return nil
	}
	// Unblock any Write() waiting on freeBufs.
	close(o.pauseSig)
	o.pauseSig = make(chan struct{}) // fresh channel for the next pause cycle
	o.accumN = 0                    // discard any partial accumulation buffer

	if status := C.AudioQueuePause(o.queue); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueuePause: OSStatus %d", int(status))
	}
	return nil
}

// ResumeAudio restarts the AudioQueue from exactly where it was paused.
// The buffered data that was preserved by AudioQueuePause is played first,
// followed by new data from Write() as the playback loop feeds it.
//
// Called by the playback manager via interface type-assertion before
// unblocking the Go playback loop:
//
//	if r, ok := m.out.(interface{ ResumeAudio() error }); ok { r.ResumeAudio() }
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

// RestartAudio explicitly stops the AudioQueue then restarts it fresh.
// Use this after an item finishes and the queue has auto-drained (ASSIST mode
// wait), NOT after a user-initiated pause. Unlike ResumeAudio, which calls
// AudioQueueStart on a paused queue (preserving buffered data), RestartAudio
// first calls AudioQueueStop(immediate=true) to transition the queue from its
// auto-stopped / "hungry" state to a clean stopped state before restarting.
// Without this explicit stop, the restarted queue may consume newly-enqueued
// buffers faster than real-time (firing callbacks immediately) because it is
// in an "idle-running" state that is distinct from a properly stopped one.
//
// Called by the playback manager via interface type-assertion:
//
//	if r, ok := m.out.(interface{ RestartAudio() error }); ok { r.RestartAudio() }
func (o *Output) RestartAudio() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || !o.started {
		return nil
	}
	// Explicit stop: transitions queue from auto-stopped / "hungry" state to
	// a clean stopped state. Idempotent — safe to call even if already stopped.
	C.AudioQueueStop(o.queue, C.Boolean(1)) //nolint:errcheck
	o.accumN = 0                            // discard any partial accumulation
	if status := C.AudioQueueStart(o.queue, nil); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueueStart (restart): OSStatus %d", int(status))
	}
	return nil
}

// Stop halts the AudioQueue immediately (does not drain remaining buffers).
// Resets the accumulation buffer so no partial frames leak into the next session.
func (o *Output) Stop(_ context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened || !o.started {
		return nil
	}
	// immediate = true: stop without waiting for queued buffers to finish.
	if status := C.AudioQueueStop(o.queue, C.Boolean(1)); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueueStop: OSStatus %d", int(status))
	}
	o.started = false
	o.accumN = 0 // discard any partial accumulation buffer to prevent pop/click on next Start
	return nil
}

// Close disposes the AudioQueue and releases all resources.
func (o *Output) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.opened {
		return nil
	}
	// Dispose stops the queue and frees its C buffers.
	if status := C.AudioQueueDispose(o.queue, C.Boolean(1)); status != 0 {
		return fmt.Errorf("coreaudio: AudioQueueDispose: OSStatus %d", int(status))
	}
	// Release the cgo.Handle so the GC can collect the Output.
	o.handle.Delete()
	o.opened = false
	o.started = false
	o.accumN = 0
	// Drain the free buffer channel.
	for len(o.freeBufs) > 0 {
		<-o.freeBufs
	}
	return nil
}

// ListDevices enumerates all audio output devices available on the system.
// It does not require the Output to be open — it queries CoreAudio directly.
// The returned DeviceInfo.ID is the persistent UID from CoreAudio
// (kAudioDevicePropertyDeviceUID), which survives device renames.
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
