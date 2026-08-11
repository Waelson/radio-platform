#include "bridge.h"
#include <string.h>
#include <stdlib.h>

// ── SPSC ring buffer ──────────────────────────────────────────────────────────

CARingBuf *caRingCreate(uint32_t cap) {
    // Round up to next power of 2.
    uint32_t p = 1;
    while (p < cap) p <<= 1;

    CARingBuf *r = (CARingBuf *)calloc(1, sizeof(CARingBuf));
    if (!r) return NULL;
    r->data = (float *)calloc(p, sizeof(float));
    if (!r->data) { free(r); return NULL; }
    r->cap = p;
    atomic_store_explicit(&r->head, 0u, memory_order_relaxed);
    atomic_store_explicit(&r->tail, 0u, memory_order_relaxed);
    return r;
}

void caRingFree(CARingBuf *r) {
    if (r) {
        free(r->data);
        free(r);
    }
}

void caRingDrain(CARingBuf *r) {
    uint32_t head = atomic_load_explicit(&r->head, memory_order_acquire);
    atomic_store_explicit(&r->tail, head, memory_order_release);
}

uint32_t caRingAvail(CARingBuf *r) {
    uint32_t head = atomic_load_explicit(&r->head, memory_order_acquire);
    uint32_t tail = atomic_load_explicit(&r->tail, memory_order_relaxed);
    return head - tail; // wraps correctly with unsigned arithmetic
}

uint32_t caRingSpace(CARingBuf *r) {
    uint32_t head = atomic_load_explicit(&r->head, memory_order_relaxed);
    uint32_t tail = atomic_load_explicit(&r->tail, memory_order_acquire);
    return r->cap - (head - tail);
}

uint32_t caRingWrite(CARingBuf *r, const float *src, uint32_t n) {
    uint32_t head = atomic_load_explicit(&r->head, memory_order_relaxed);
    uint32_t tail = atomic_load_explicit(&r->tail, memory_order_acquire);
    uint32_t space = r->cap - (head - tail);
    if (n > space) n = space;
    if (n == 0) return 0;

    uint32_t mask  = r->cap - 1;
    uint32_t h     = head & mask;
    uint32_t first = r->cap - h; // samples until end of array

    if (n <= first) {
        memcpy(r->data + h, src, n * sizeof(float));
    } else {
        memcpy(r->data + h,       src,         first       * sizeof(float));
        memcpy(r->data,           src + first, (n - first) * sizeof(float));
    }
    atomic_store_explicit(&r->head, head + n, memory_order_release);
    return n;
}

void caRingRead(CARingBuf *r, float *dst, uint32_t n) {
    uint32_t tail  = atomic_load_explicit(&r->tail, memory_order_relaxed);
    uint32_t head  = atomic_load_explicit(&r->head, memory_order_acquire);
    uint32_t avail = head - tail;
    uint32_t got   = (avail < n) ? avail : n;

    if (got > 0) {
        uint32_t mask  = r->cap - 1;
        uint32_t t     = tail & mask;
        uint32_t first = r->cap - t;

        if (got <= first) {
            memcpy(dst, r->data + t, got * sizeof(float));
        } else {
            memcpy(dst,       r->data + t, first       * sizeof(float));
            memcpy(dst + first, r->data,   (got - first) * sizeof(float));
        }
        atomic_store_explicit(&r->tail, tail + got, memory_order_release);
    }

    // Fill remainder with silence if the ring had fewer samples than requested.
    if (got < n) {
        memset(dst + got, 0, (n - got) * sizeof(float));
    }
}

// ── Pull-model AudioQueue callback ────────────────────────────────────────────
// Called by CoreAudio when it is done with a buffer and wants more audio.
// We read from the ring buffer (silence when empty) and ALWAYS re-enqueue the
// buffer. This makes the AudioQueue self-sustaining — it never auto-stops.

static void pullCallback(
    void               *userData,
    AudioQueueRef       queue,
    AudioQueueBufferRef buf)
{
    CARingBuf *ring    = (CARingBuf *)userData;
    uint32_t   nSamples = (uint32_t)(buf->mAudioDataBytesCapacity / sizeof(float));
    float     *dst      = (float *)buf->mAudioData;

    caRingRead(ring, dst, nSamples); // fills with zeros if ring is empty

    buf->mAudioDataByteSize = buf->mAudioDataBytesCapacity;
    AudioQueueEnqueueBuffer(queue, buf, 0, NULL); // always re-enqueue
}

OSStatus caNewQueuePull(double sampleRate, int channels,
                        CARingBuf *ring, AudioQueueRef *outQueue)
{
    AudioStreamBasicDescription fmt = {0};
    fmt.mSampleRate       = sampleRate;
    fmt.mFormatID         = kAudioFormatLinearPCM;
    fmt.mFormatFlags      = kAudioFormatFlagIsFloat
                          | kAudioFormatFlagIsPacked;
    fmt.mBitsPerChannel   = 32;
    fmt.mChannelsPerFrame = (UInt32)channels;
    fmt.mFramesPerPacket  = 1;
    fmt.mBytesPerFrame    = sizeof(float) * (UInt32)channels;
    fmt.mBytesPerPacket   = fmt.mBytesPerFrame;

    return AudioQueueNewOutput(&fmt, pullCallback, ring,
                               NULL, NULL, 0, outQueue);
}

OSStatus caAllocBuffer(AudioQueueRef queue, int bufferFrames,
                       int channels, AudioQueueBufferRef *outBuf)
{
    UInt32 bytes = (UInt32)(bufferFrames * channels * sizeof(float));
    return AudioQueueAllocateBuffer(queue, bytes, outBuf);
}

OSStatus caEnqueueSilent(AudioQueueRef queue, AudioQueueBufferRef buf)
{
    memset(buf->mAudioData, 0, buf->mAudioDataBytesCapacity);
    buf->mAudioDataByteSize = buf->mAudioDataBytesCapacity;
    return AudioQueueEnqueueBuffer(queue, buf, 0, NULL);
}

// ── Device lookup helpers ─────────────────────────────────────────────────────

OSStatus caFindDeviceByName(const char *name, AudioDeviceID *outID)
{
    AudioObjectPropertyAddress propDevices = {
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        0
    };

    UInt32 dataSize = 0;
    OSStatus status = AudioObjectGetPropertyDataSize(
        kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize);
    if (status != noErr) return status;

    UInt32 count = dataSize / sizeof(AudioDeviceID);
    AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
    if (!devices) return -108;

    status = AudioObjectGetPropertyData(
        kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize, devices);
    if (status != noErr) { free(devices); return status; }

    AudioObjectPropertyAddress propStreams = {
        kAudioDevicePropertyStreams, kAudioDevicePropertyScopeOutput, 0
    };
    AudioObjectPropertyAddress propName = {
        kAudioObjectPropertyName, kAudioObjectPropertyScopeGlobal, 0
    };

    OSStatus result = kAudioHardwareUnknownPropertyError;
    for (UInt32 i = 0; i < count; i++) {
        UInt32 streamSize = 0;
        AudioObjectGetPropertyDataSize(devices[i], &propStreams, 0, NULL, &streamSize);
        if (streamSize == 0) continue;

        CFStringRef cfName = NULL;
        UInt32 nameSize = sizeof(cfName);
        if (AudioObjectGetPropertyData(devices[i], &propName, 0, NULL,
                                       &nameSize, &cfName) != noErr) continue;
        if (!cfName) continue;

        char buf[256] = {0};
        Boolean ok = CFStringGetCString(cfName, buf, sizeof(buf), kCFStringEncodingUTF8);
        CFRelease(cfName);

        if (ok && strcmp(buf, name) == 0) {
            *outID = devices[i];
            result = noErr;
            break;
        }
    }

    free(devices);
    return result;
}

OSStatus caFindDeviceByUID(const char *uid, AudioDeviceID *outID)
{
    AudioObjectPropertyAddress propDevices = {
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        0
    };

    UInt32 dataSize = 0;
    OSStatus status = AudioObjectGetPropertyDataSize(
        kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize);
    if (status != noErr) return status;

    UInt32 count = dataSize / sizeof(AudioDeviceID);
    AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
    if (!devices) return -108;

    status = AudioObjectGetPropertyData(
        kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize, devices);
    if (status != noErr) { free(devices); return status; }

    AudioObjectPropertyAddress propStreams = {
        kAudioDevicePropertyStreams, kAudioDevicePropertyScopeOutput, 0
    };
    AudioObjectPropertyAddress propUID = {
        kAudioDevicePropertyDeviceUID, kAudioObjectPropertyScopeGlobal, 0
    };

    OSStatus result = kAudioHardwareUnknownPropertyError;
    for (UInt32 i = 0; i < count; i++) {
        UInt32 streamSize = 0;
        AudioObjectGetPropertyDataSize(devices[i], &propStreams, 0, NULL, &streamSize);
        if (streamSize == 0) continue;

        CFStringRef cfUID = NULL;
        UInt32 uidSize = sizeof(cfUID);
        if (AudioObjectGetPropertyData(devices[i], &propUID, 0, NULL,
                                       &uidSize, &cfUID) != noErr) continue;
        if (!cfUID) continue;

        char buf[256] = {0};
        Boolean ok = CFStringGetCString(cfUID, buf, sizeof(buf), kCFStringEncodingUTF8);
        CFRelease(cfUID);

        if (ok && strcmp(buf, uid) == 0) {
            *outID = devices[i];
            result = noErr;
            break;
        }
    }

    free(devices);
    return result;
}

OSStatus caSetQueueDevice(AudioQueueRef queue, AudioDeviceID deviceID)
{
    AudioObjectPropertyAddress propUID = {
        kAudioDevicePropertyDeviceUID, kAudioObjectPropertyScopeGlobal, 0
    };
    CFStringRef uid = NULL;
    UInt32 uidSize = sizeof(uid);
    OSStatus st = AudioObjectGetPropertyData(deviceID, &propUID, 0, NULL, &uidSize, &uid);
    if (st != noErr) return st;
    st = AudioQueueSetProperty(queue, kAudioQueueProperty_CurrentDevice, &uid, sizeof(uid));
    if (uid) CFRelease(uid);
    return st;
}

int caListOutputDevices(char *buf, int bufSize)
{
    AudioObjectPropertyAddress propDevices = {
        kAudioHardwarePropertyDevices, kAudioObjectPropertyScopeGlobal, 0
    };
    UInt32 dataSize = 0;
    if (AudioObjectGetPropertyDataSize(kAudioObjectSystemObject,
                                       &propDevices, 0, NULL, &dataSize) != noErr)
        return 0;

    UInt32 count = dataSize / sizeof(AudioDeviceID);
    AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
    if (!devices) return 0;
    if (AudioObjectGetPropertyData(kAudioObjectSystemObject,
                                   &propDevices, 0, NULL, &dataSize, devices) != noErr) {
        free(devices); return 0;
    }

    AudioObjectPropertyAddress propStreams = {
        kAudioDevicePropertyStreams, kAudioObjectPropertyScopeOutput, 0
    };
    AudioObjectPropertyAddress propName = {
        kAudioObjectPropertyName, kAudioObjectPropertyScopeGlobal, 0
    };

    int written = 0;
    for (UInt32 i = 0; i < count; i++) {
        UInt32 streamSize = 0;
        AudioObjectGetPropertyDataSize(devices[i], &propStreams, 0, NULL, &streamSize);
        if (streamSize == 0) continue;

        CFStringRef cfName = NULL;
        UInt32 nameSize = sizeof(cfName);
        if (AudioObjectGetPropertyData(devices[i], &propName, 0, NULL,
                                       &nameSize, &cfName) != noErr) continue;
        if (!cfName) continue;

        char tmp[256] = {0};
        if (CFStringGetCString(cfName, tmp, sizeof(tmp), kCFStringEncodingUTF8)) {
            int n = snprintf(buf + written, bufSize - written, "%s\n", tmp);
            if (n > 0) written += n;
        }
        CFRelease(cfName);
    }
    free(devices);
    return written;
}

int caEnumOutputDevices(CADeviceEntry *out, int maxCount)
{
    if (!out || maxCount <= 0) return 0;

    AudioObjectPropertyAddress propDevices = {
        kAudioHardwarePropertyDevices, kAudioObjectPropertyScopeGlobal, 0
    };
    UInt32 dataSize = 0;
    if (AudioObjectGetPropertyDataSize(kAudioObjectSystemObject,
                                       &propDevices, 0, NULL, &dataSize) != noErr)
        return 0;

    UInt32 count = dataSize / sizeof(AudioDeviceID);
    AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
    if (!devices) return 0;
    if (AudioObjectGetPropertyData(kAudioObjectSystemObject,
                                   &propDevices, 0, NULL, &dataSize, devices) != noErr) {
        free(devices); return 0;
    }

    AudioObjectPropertyAddress propDefault = {
        kAudioHardwarePropertyDefaultOutputDevice, kAudioObjectPropertyScopeGlobal, 0
    };
    AudioDeviceID defaultID = kAudioDeviceUnknown;
    UInt32 defaultSize = sizeof(defaultID);
    AudioObjectGetPropertyData(kAudioObjectSystemObject,
                               &propDefault, 0, NULL, &defaultSize, &defaultID);

    AudioObjectPropertyAddress propStreams = {
        kAudioDevicePropertyStreams, kAudioDevicePropertyScopeOutput, 0
    };
    AudioObjectPropertyAddress propUID = {
        kAudioDevicePropertyDeviceUID, kAudioObjectPropertyScopeGlobal, 0
    };
    AudioObjectPropertyAddress propName = {
        kAudioObjectPropertyName, kAudioObjectPropertyScopeGlobal, 0
    };
    AudioObjectPropertyAddress propRate = {
        kAudioDevicePropertyNominalSampleRate, kAudioObjectPropertyScopeGlobal, 0
    };
    AudioObjectPropertyAddress propStreamCfg = {
        kAudioDevicePropertyStreamConfiguration, kAudioDevicePropertyScopeOutput, 0
    };

    int written = 0;
    for (UInt32 i = 0; i < count && written < maxCount; i++) {
        AudioDeviceID devID = devices[i];

        UInt32 streamSize = 0;
        AudioObjectGetPropertyDataSize(devID, &propStreams, 0, NULL, &streamSize);
        if (streamSize == 0) continue;

        CADeviceEntry *e = &out[written];
        memset(e, 0, sizeof(*e));

        CFStringRef cfUID = NULL;
        UInt32 uidSize = sizeof(cfUID);
        if (AudioObjectGetPropertyData(devID, &propUID, 0, NULL, &uidSize, &cfUID) == noErr && cfUID) {
            CFStringGetCString(cfUID, e->uid, sizeof(e->uid), kCFStringEncodingUTF8);
            CFRelease(cfUID);
        }

        CFStringRef cfName = NULL;
        UInt32 nameSize = sizeof(cfName);
        if (AudioObjectGetPropertyData(devID, &propName, 0, NULL, &nameSize, &cfName) == noErr && cfName) {
            CFStringGetCString(cfName, e->name, sizeof(e->name), kCFStringEncodingUTF8);
            CFRelease(cfName);
        }

        UInt32 cfgSize = 0;
        if (AudioObjectGetPropertyDataSize(devID, &propStreamCfg, 0, NULL, &cfgSize) == noErr && cfgSize > 0) {
            AudioBufferList *bufList = (AudioBufferList *)malloc(cfgSize);
            if (bufList) {
                if (AudioObjectGetPropertyData(devID, &propStreamCfg, 0, NULL, &cfgSize, bufList) == noErr) {
                    for (UInt32 b = 0; b < bufList->mNumberBuffers; b++)
                        e->maxOutputChannels += (int)bufList->mBuffers[b].mNumberChannels;
                }
                free(bufList);
            }
        }

        Float64 rate = 0;
        UInt32 rateSize = sizeof(rate);
        if (AudioObjectGetPropertyData(devID, &propRate, 0, NULL, &rateSize, &rate) == noErr)
            e->defaultSampleRate = (double)rate;

        e->isDefault = (devID == defaultID) ? 1 : 0;
        written++;
    }

    free(devices);
    return written;
}
