#ifndef COREAUDIO_BRIDGE_H
#define COREAUDIO_BRIDGE_H

#include <AudioToolbox/AudioToolbox.h>
#include <stdatomic.h>
#include <stdint.h>

// ── SPSC lock-free ring buffer ────────────────────────────────────────────────
// Single-producer (Go goroutine via Write) /
// single-consumer (CoreAudio callback thread).
//
// The ring buffer decouples the Go writer rate (FFmpeg / playback loop) from
// the CoreAudio consumption rate. When the producer is ahead (burst delivery),
// the ring absorbs the surplus. When the producer stalls, the callback serves
// silence instead of letting the AudioQueue auto-stop.
typedef struct {
    float            *data;
    uint32_t          cap;   // capacity in samples — always a power of 2
    _Atomic uint32_t  head;  // write cursor, advanced by producer (Go)
    _Atomic uint32_t  tail;  // read cursor,  advanced by consumer (C callback)
} CARingBuf;

// Allocates a ring buffer. capacitySamples is rounded up to the next power-of-2.
CARingBuf *caRingCreate(uint32_t capacitySamples);

// Frees a ring buffer created by caRingCreate.
void caRingFree(CARingBuf *r);

// Discards all buffered data (moves tail to head).
void caRingDrain(CARingBuf *r);

// Writes up to n samples from src into the ring buffer.
// Returns the number of samples actually written (may be < n if buffer is full).
uint32_t caRingWrite(CARingBuf *r, const float *src, uint32_t n);

// Reads exactly n samples from the ring buffer into dst.
// Fills any remainder with 0.0f (silence) if fewer than n samples are available.
void caRingRead(CARingBuf *r, float *dst, uint32_t n);

// Returns the number of samples available for reading.
uint32_t caRingAvail(CARingBuf *r);

// Returns the number of samples of free space available for writing.
uint32_t caRingSpace(CARingBuf *r);

// ── AudioQueue — pull model ────────────────────────────────────────────────────
// The pull-model callback reads from the ring buffer on every invocation and
// always re-enqueues the buffer. The AudioQueue NEVER auto-stops: when the ring
// is empty the callback serves silence, but playback continues uninterrupted.
//
// Usage:
//   1. caRingCreate()      — create ring buffer
//   2. caNewQueuePull()    — create AudioQueue using pull callback
//   3. caAllocBuffer() ×N  — allocate N AudioQueue buffers
//   4. caEnqueueSilent() ×N — kick-start the circular buffer loop
//   5. AudioQueueStart()   — begin playback (silence until ring has data)
//   6. caRingWrite()       — feed PCM data from Go; callback picks it up
//   7. AudioQueueStop()    — stop playback
//   8. caRingFree()        — release ring buffer

// Creates an output AudioQueue in pull mode. ring must remain valid for the
// lifetime of the queue.
OSStatus caNewQueuePull(double sampleRate, int channels,
                        CARingBuf *ring, AudioQueueRef *outQueue);

// Allocates one AudioQueue buffer of bufferFrames × channels × sizeof(float) bytes.
OSStatus caAllocBuffer(AudioQueueRef queue, int bufferFrames,
                       int channels, AudioQueueBufferRef *outBuf);

// Fills buf with silence and enqueues it. Call once per buffer at startup to
// kick-start the pull-model circular loop.
OSStatus caEnqueueSilent(AudioQueueRef queue, AudioQueueBufferRef buf);

// ── Device enumeration ────────────────────────────────────────────────────────

// Finds an output device by human-readable name. Sets *outID on success.
OSStatus caFindDeviceByName(const char *name, AudioDeviceID *outID);

// Finds an output device by persistent UID (kAudioDevicePropertyDeviceUID).
OSStatus caFindDeviceByUID(const char *uid, AudioDeviceID *outID);

// Routes an AudioQueue to a specific output device.
OSStatus caSetQueueDevice(AudioQueueRef queue, AudioDeviceID deviceID);

// Writes newline-separated device names into buf. Returns bytes written.
int caListOutputDevices(char *buf, int bufSize);

// Holds metadata for one output device returned by caEnumOutputDevices.
typedef struct {
    char   uid[256];
    char   name[256];
    int    maxOutputChannels;
    double defaultSampleRate;
    int    isDefault;
} CADeviceEntry;

// Enumerates output devices into out[0..maxCount-1]. Returns count written.
int caEnumOutputDevices(CADeviceEntry *out, int maxCount);

#endif /* COREAUDIO_BRIDGE_H */
