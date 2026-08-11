#ifndef CAINPUT_H
#define CAINPUT_H

#include <stdint.h>
#include <stdatomic.h>

// ── SPSC lock-free ring buffer ─────────────────────────────────────────────
// Producer: CoreAudio input callback (C, audio thread).
// Consumer: goroutine Go (ReadFrames polling loop).
//
// cap MUST be a power of 2. head and tail are monotonically increasing
// uint32 counters; modular arithmetic handles wrap-around.
typedef struct {
    float            *data;
    uint32_t          cap;
    _Atomic uint32_t  head; // write cursor — advanced by callback
    _Atomic uint32_t  tail; // read  cursor — advanced by Go
} CAIRing;

// Writes up to n samples from src. Returns actual count written.
// Never blocks — drops samples silently when full to protect the audio thread.
uint32_t caIRingWrite(CAIRing *r, const float *src, uint32_t n);

// Reads up to n samples into dst. Returns actual count read (may be 0).
uint32_t caIRingRead(CAIRing *r, float *dst, uint32_t n);

// Returns the number of samples available for reading.
uint32_t caIRingAvail(const CAIRing *r);

// ── Capture session ────────────────────────────────────────────────────────
typedef struct {
    void   *queue; // AudioQueueRef — opaque to header consumers
    CAIRing ring;
} CAInputSession;

// Opens a new AudioQueue input session.
//
// deviceID: avfoundation index ("0", "1", ...) or "" / "default" for system default.
// sampleRate: target sample rate (e.g. 48000).
// channels:   number of channels (e.g. 2 for stereo).
// ringCap:    ring buffer capacity in float32 samples — MUST be a power of 2.
// errBuf:     receives a null-terminated error message on failure.
//
// Returns NULL on error; caller must invoke caInputClose() on success.
CAInputSession *caInputOpen(const char *deviceID,
                            uint32_t sampleRate, uint32_t channels,
                            uint32_t ringCap,
                            char *errBuf, int errBufLen);

// Starts the AudioQueue. Returns 0 on success, non-zero OSStatus on error.
int caInputStart(CAInputSession *s);

// Stops the AudioQueue (synchronous flush — waits for in-flight callbacks).
void caInputStop(CAInputSession *s);

// Disposes the AudioQueue and frees all resources. Safe to call after Stop.
void caInputClose(CAInputSession *s);

// Reads up to n samples from the ring. Returns actual count (may be 0).
uint32_t caInputRead(CAInputSession *s, float *dst, uint32_t n);

// Returns the number of samples currently available in the ring.
uint32_t caInputAvail(const CAInputSession *s);

#endif // CAINPUT_H
