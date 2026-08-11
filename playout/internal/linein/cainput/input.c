#include "input.h"

#include <AudioToolbox/AudioToolbox.h>
#include <CoreAudio/CoreAudio.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// CA_ELEMENT_MAIN == 0 in all SDK versions.
// Use the literal to avoid deprecation warnings from the pre-macOS-12 alias.
#define CA_ELEMENT_MAIN 0

// ── SPSC ring buffer ───────────────────────────────────────────────────────

uint32_t caIRingWrite(CAIRing *r, const float *src, uint32_t n) {
    uint32_t head  = atomic_load_explicit(&r->head, memory_order_relaxed);
    uint32_t tail  = atomic_load_explicit(&r->tail, memory_order_acquire);
    uint32_t space = r->cap - (head - tail);
    if (n > space) n = space;
    uint32_t mask = r->cap - 1;
    for (uint32_t i = 0; i < n; i++) {
        r->data[(head + i) & mask] = src[i];
    }
    atomic_store_explicit(&r->head, head + n, memory_order_release);
    return n;
}

uint32_t caIRingRead(CAIRing *r, float *dst, uint32_t n) {
    uint32_t tail  = atomic_load_explicit(&r->tail, memory_order_relaxed);
    uint32_t head  = atomic_load_explicit(&r->head, memory_order_acquire);
    uint32_t avail = head - tail;
    if (n > avail) n = avail;
    uint32_t mask = r->cap - 1;
    for (uint32_t i = 0; i < n; i++) {
        dst[i] = r->data[(tail + i) & mask];
    }
    atomic_store_explicit(&r->tail, tail + n, memory_order_release);
    return n;
}

uint32_t caIRingAvail(const CAIRing *r) {
    uint32_t head = atomic_load_explicit(&r->head, memory_order_acquire);
    uint32_t tail = atomic_load_explicit(&r->tail, memory_order_relaxed);
    return head - tail;
}

// ── AudioQueue input callback ──────────────────────────────────────────────

static void inputCallback(
    void *userData,
    AudioQueueRef queue,
    AudioQueueBufferRef buffer,
    const AudioTimeStamp *startTime,
    UInt32 numPackets,
    const AudioStreamPacketDescription *packetDescs)
{
    CAInputSession *s = (CAInputSession *)userData;
    uint32_t samples = buffer->mAudioDataByteSize / sizeof(float);
    // Non-blocking write — drops on overflow to never stall the audio thread.
    caIRingWrite(&s->ring, (const float *)buffer->mAudioData, samples);
    // Always re-enqueue so the AudioQueue never stops.
    AudioQueueEnqueueBuffer(queue, buffer, 0, NULL);
}

// ── Device resolution ──────────────────────────────────────────────────────

// Returns the AudioDeviceID of the n-th CoreAudio input device (0-based).
// Input devices are those with at least one input stream.
// Returns kAudioObjectUnknown if index is out of range.
static AudioDeviceID deviceByIndex(int idx) {
    AudioObjectPropertyAddress addr = {
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        CA_ELEMENT_MAIN
    };

    UInt32 dataSize = 0;
    if (AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &addr,
                                      0, NULL, &dataSize) != noErr)
        return kAudioObjectUnknown;

    int total = (int)(dataSize / sizeof(AudioDeviceID));
    AudioDeviceID *devs = malloc(dataSize);
    if (!devs) return kAudioObjectUnknown;

    if (AudioObjectGetPropertyData(kAudioObjectSystemObject, &addr,
                                   0, NULL, &dataSize, devs) != noErr) {
        free(devs);
        return kAudioObjectUnknown;
    }

    int inputIdx = 0;
    AudioDeviceID result = kAudioObjectUnknown;

    for (int i = 0; i < total && result == kAudioObjectUnknown; i++) {
        AudioObjectPropertyAddress streamAddr = {
            kAudioDevicePropertyStreams,
            kAudioObjectPropertyScopeInput,
            CA_ELEMENT_MAIN
        };
        UInt32 streamSize = 0;
        if (AudioObjectGetPropertyDataSize(devs[i], &streamAddr,
                                          0, NULL, &streamSize) != noErr)
            continue;
        if (streamSize == 0)
            continue; // no input streams → output-only device, skip

        if (inputIdx == idx)
            result = devs[i];
        inputIdx++;
    }

    free(devs);
    return result;
}

// ── Session lifecycle ──────────────────────────────────────────────────────

CAInputSession *caInputOpen(const char *deviceID,
                            uint32_t sampleRate, uint32_t channels,
                            uint32_t ringCap,
                            char *errBuf, int errBufLen) {
    CAInputSession *s = calloc(1, sizeof(CAInputSession));
    if (!s) {
        snprintf(errBuf, errBufLen, "caInputOpen: out of memory");
        return NULL;
    }

    // Initialise ring buffer (ringCap must be power-of-2, enforced by Go caller).
    s->ring.data = calloc(ringCap, sizeof(float));
    if (!s->ring.data) {
        snprintf(errBuf, errBufLen, "caInputOpen: ring buffer alloc failed");
        free(s);
        return NULL;
    }
    s->ring.cap = ringCap;
    atomic_store(&s->ring.head, 0);
    atomic_store(&s->ring.tail, 0);

    // PCM float32 interleaved format — same as the rest of the engine.
    AudioStreamBasicDescription fmt = {
        .mSampleRate       = (Float64)sampleRate,
        .mFormatID         = kAudioFormatLinearPCM,
        .mFormatFlags      = kLinearPCMFormatFlagIsFloat |
                             kLinearPCMFormatFlagIsPacked,
        .mBytesPerPacket   = channels * (uint32_t)sizeof(float),
        .mFramesPerPacket  = 1,
        .mBytesPerFrame    = channels * (uint32_t)sizeof(float),
        .mChannelsPerFrame = channels,
        .mBitsPerChannel   = 32,
    };

    OSStatus err = AudioQueueNewInput(&fmt, inputCallback, s,
                                     NULL,  // run loop (NULL = internal)
                                     NULL,  // run loop mode
                                     0,     // flags (reserved)
                                     (AudioQueueRef *)&s->queue);
    if (err != noErr) {
        snprintf(errBuf, errBufLen, "AudioQueueNewInput: OSStatus %d", (int)err);
        free(s->ring.data);
        free(s);
        return NULL;
    }

    // Select device if an explicit index was provided.
    if (deviceID && deviceID[0] &&
        strcmp(deviceID, "default") != 0 &&
        strcmp(deviceID, "0") != 0) // "0" == first input == system default
    {
        char *endPtr;
        long idx = strtol(deviceID, &endPtr, 10);
        if (*endPtr == '\0' && idx > 0) {
            AudioDeviceID dev = deviceByIndex((int)idx);
            if (dev != kAudioObjectUnknown) {
                AudioQueueSetProperty((AudioQueueRef)s->queue,
                                      kAudioQueueProperty_CurrentDevice,
                                      &dev, sizeof(dev));
            }
        }
        // Non-numeric or unknown UIDs: silently fall through to default device.
    }

    // Allocate 3 small AudioQueue buffers: 512 frames × channels × float32.
    // At 48 kHz, 512 frames ≈ 10.67 ms — tight hardware-timed callbacks.
    const uint32_t bufFrames = 512;
    const uint32_t bufBytes  = bufFrames * channels * (uint32_t)sizeof(float);
    for (int i = 0; i < 3; i++) {
        AudioQueueBufferRef buf;
        err = AudioQueueAllocateBuffer((AudioQueueRef)s->queue, bufBytes, &buf);
        if (err != noErr) {
            snprintf(errBuf, errBufLen,
                     "AudioQueueAllocateBuffer[%d]: OSStatus %d", i, (int)err);
            AudioQueueDispose((AudioQueueRef)s->queue, true);
            free(s->ring.data);
            free(s);
            return NULL;
        }
        buf->mAudioDataByteSize = bufBytes;
        memset(buf->mAudioData, 0, bufBytes);
        AudioQueueEnqueueBuffer((AudioQueueRef)s->queue, buf, 0, NULL);
    }

    return s;
}

int caInputStart(CAInputSession *s) {
    if (!s || !s->queue) return -1;
    OSStatus err = AudioQueueStart((AudioQueueRef)s->queue, NULL);
    return (int)err;
}

void caInputStop(CAInputSession *s) {
    if (!s || !s->queue) return;
    // true = synchronous: waits for in-flight callback to finish before returning.
    AudioQueueStop((AudioQueueRef)s->queue, true);
}

void caInputClose(CAInputSession *s) {
    if (!s) return;
    if (s->queue) {
        AudioQueueDispose((AudioQueueRef)s->queue, true);
        s->queue = NULL;
    }
    free(s->ring.data);
    s->ring.data = NULL;
    free(s);
}

uint32_t caInputRead(CAInputSession *s, float *dst, uint32_t n) {
    if (!s) return 0;
    return caIRingRead(&s->ring, dst, n);
}

uint32_t caInputAvail(const CAInputSession *s) {
    if (!s) return 0;
    return caIRingAvail(&s->ring);
}
