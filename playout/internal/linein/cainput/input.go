//go:build coreaudio

// Package cainput wraps the CoreAudio AudioQueue input capture C bridge.
// It provides hardware-timed PCM capture via an SPSC ring buffer, eliminating
// the jitter introduced by FFmpeg subprocesses and OS pipe buffering.
//
// Lifecycle:
//
//	s, err := cainput.Open(deviceID, 48000, 2)
//	if err != nil { ... }
//	defer s.Close()
//	s.Start()
//	n := s.Read(buf)
package cainput

// #cgo LDFLAGS: -framework AudioToolbox -framework CoreAudio
// #include "input.h"
// #include <stdlib.h>
// #include <string.h>
import "C"
import (
	"fmt"
	"unsafe"
)

// ringCapSamples is the SPSC ring buffer size in float32 samples.
// Must be a power of 2. 524288 = 2^19 ≈ 5.46 s of stereo 48 kHz audio.
const ringCapSamples = 1 << 19 // 524 288

// Session holds an active CoreAudio input capture session.
type Session struct {
	s *C.CAInputSession
}

// Open creates and initialises a new capture session.
//
//   - deviceID: avfoundation index ("0", "1", ...) or "" / "default" for
//     the system default input device.
//   - sampleRate: 48000
//   - channels: 2
func Open(deviceID string, sampleRate, channels uint32) (*Session, error) {
	cDevice := C.CString(deviceID)
	defer C.free(unsafe.Pointer(cDevice))

	const errLen = 256
	errBuf := (*C.char)(C.malloc(errLen))
	defer C.free(unsafe.Pointer(errBuf))
	C.memset(unsafe.Pointer(errBuf), 0, errLen)

	sess := C.caInputOpen(
		cDevice,
		C.uint32_t(sampleRate),
		C.uint32_t(channels),
		C.uint32_t(ringCapSamples),
		errBuf, C.int(errLen),
	)
	if sess == nil {
		return nil, fmt.Errorf("cainput: open: %s", C.GoString(errBuf))
	}
	return &Session{s: sess}, nil
}

// Start begins audio capture. Must be called after Open.
func (s *Session) Start() error {
	if rc := C.caInputStart(s.s); rc != 0 {
		return fmt.Errorf("cainput: start: OSStatus %d", rc)
	}
	return nil
}

// Read copies up to len(dst) float32 samples from the ring buffer into dst.
// Returns the number of samples actually read (may be 0 if the ring is empty).
// Non-blocking — the caller is responsible for polling until data is available.
func (s *Session) Read(dst []float32) int {
	if len(dst) == 0 {
		return 0
	}
	n := C.caInputRead(s.s,
		(*C.float)(unsafe.Pointer(&dst[0])),
		C.uint32_t(len(dst)),
	)
	return int(n)
}

// Avail returns the number of float32 samples currently available in the ring.
func (s *Session) Avail() int {
	return int(C.caInputAvail(s.s))
}

// Stop halts the AudioQueue synchronously (waits for the in-flight callback).
func (s *Session) Stop() {
	C.caInputStop(s.s)
}

// Close stops (if not already stopped) and releases all resources.
func (s *Session) Close() {
	if s.s == nil {
		return
	}
	C.caInputClose(s.s)
	s.s = nil
}
