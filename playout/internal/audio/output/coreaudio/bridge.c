#include "bridge.h"
#include <string.h>
#include <stdlib.h>

static void outputCallback(
    void               *userData,
    AudioQueueRef       queue,
    AudioQueueBufferRef buf)
{
    goBufferReady(userData, queue, buf);
}

OSStatus caNewQueue(double sampleRate, int channels,
                    void *userData, AudioQueueRef *outQueue)
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

    return AudioQueueNewOutput(&fmt, outputCallback, userData,
                               NULL, NULL, 0, outQueue);
}

OSStatus caAllocBuffer(AudioQueueRef queue, int bufferFrames,
                       int channels, AudioQueueBufferRef *outBuf)
{
    UInt32 bytes = (UInt32)(bufferFrames * channels * sizeof(float));
    return AudioQueueAllocateBuffer(queue, bytes, outBuf);
}

OSStatus caEnqueueBuffer(AudioQueueRef queue, AudioQueueBufferRef buf,
                         const float *frames, int nFrames, int channels)
{
    UInt32 bytes = (UInt32)(nFrames * channels * sizeof(float));
    memcpy(buf->mAudioData, frames, bytes);
    buf->mAudioDataByteSize = bytes;
    return AudioQueueEnqueueBuffer(queue, buf, 0, NULL);
}

int caIsRunning(AudioQueueRef queue)
{
    UInt32 isRunning = 0;
    UInt32 size = sizeof(isRunning);
    AudioQueueGetProperty(queue, kAudioQueueProperty_IsRunning, &isRunning, &size);
    return (int)isRunning;
}

OSStatus caFindDeviceByName(const char *name, AudioDeviceID *outID)
{
    AudioObjectPropertyAddress propDevices = {
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        0 /* kAudioObjectPropertyElementMain */
    };

    UInt32 dataSize = 0;
    OSStatus status = AudioObjectGetPropertyDataSize(
        kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize);
    if (status != noErr) return status;

    UInt32 count = dataSize / sizeof(AudioDeviceID);
    AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
    if (!devices) return -108; /* memFullErr */

    status = AudioObjectGetPropertyData(
        kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize, devices);
    if (status != noErr) { free(devices); return status; }

    AudioObjectPropertyAddress propStreams = {
        kAudioDevicePropertyStreams,
        kAudioDevicePropertyScopeOutput,
        0
    };
    AudioObjectPropertyAddress propName = {
        kAudioObjectPropertyName,
        kAudioObjectPropertyScopeGlobal,
        0
    };

    OSStatus result = kAudioHardwareUnknownPropertyError;
    for (UInt32 i = 0; i < count; i++) {
        /* Skip input-only devices */
        UInt32 streamSize = 0;
        AudioObjectGetPropertyDataSize(devices[i], &propStreams, 0, NULL, &streamSize);
        if (streamSize == 0) continue;

        CFStringRef cfName = NULL;
        UInt32 nameSize = sizeof(cfName);
        if (AudioObjectGetPropertyData(devices[i], &propName, 0, NULL, &nameSize, &cfName) != noErr)
            continue;
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
    if (!devices) return -108; /* memFullErr */

    status = AudioObjectGetPropertyData(
        kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize, devices);
    if (status != noErr) { free(devices); return status; }

    AudioObjectPropertyAddress propStreams = {
        kAudioDevicePropertyStreams,
        kAudioDevicePropertyScopeOutput,
        0
    };
    AudioObjectPropertyAddress propUID = {
        kAudioDevicePropertyDeviceUID,
        kAudioObjectPropertyScopeGlobal,
        0
    };

    OSStatus result = kAudioHardwareUnknownPropertyError;
    for (UInt32 i = 0; i < count; i++) {
        /* Skip input-only devices */
        UInt32 streamSize = 0;
        AudioObjectGetPropertyDataSize(devices[i], &propStreams, 0, NULL, &streamSize);
        if (streamSize == 0) continue;

        CFStringRef cfUID = NULL;
        UInt32 uidSize = sizeof(cfUID);
        if (AudioObjectGetPropertyData(devices[i], &propUID, 0, NULL, &uidSize, &cfUID) != noErr)
            continue;
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
    /* kAudioQueueProperty_CurrentDevice expects a CFStringRef UID, not an AudioDeviceID. */
    AudioObjectPropertyAddress propUID = {
        kAudioDevicePropertyDeviceUID,
        kAudioObjectPropertyScopeGlobal,
        0
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
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        0
    };
    UInt32 dataSize = 0;
    if (AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize) != noErr)
        return 0;

    UInt32 count = dataSize / sizeof(AudioDeviceID);
    AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
    if (!devices) return 0;
    if (AudioObjectGetPropertyData(kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize, devices) != noErr) {
        free(devices);
        return 0;
    }

    AudioObjectPropertyAddress propStreams = { kAudioDevicePropertyStreams, kAudioObjectPropertyScopeOutput, 0 };
    AudioObjectPropertyAddress propName   = { kAudioObjectPropertyName, kAudioObjectPropertyScopeGlobal, 0 };

    int written = 0;
    for (UInt32 i = 0; i < count; i++) {
        UInt32 streamSize = 0;
        AudioObjectGetPropertyDataSize(devices[i], &propStreams, 0, NULL, &streamSize);
        if (streamSize == 0) continue;

        CFStringRef cfName = NULL;
        UInt32 nameSize = sizeof(cfName);
        if (AudioObjectGetPropertyData(devices[i], &propName, 0, NULL, &nameSize, &cfName) != noErr) continue;
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

    /* --- collect all AudioDeviceIDs --- */
    AudioObjectPropertyAddress propDevices = {
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        0
    };
    UInt32 dataSize = 0;
    if (AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize) != noErr)
        return 0;

    UInt32 count = dataSize / sizeof(AudioDeviceID);
    AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
    if (!devices) return 0;
    if (AudioObjectGetPropertyData(kAudioObjectSystemObject, &propDevices, 0, NULL, &dataSize, devices) != noErr) {
        free(devices);
        return 0;
    }

    /* --- find the system default output device --- */
    AudioObjectPropertyAddress propDefault = {
        kAudioHardwarePropertyDefaultOutputDevice,
        kAudioObjectPropertyScopeGlobal,
        0
    };
    AudioDeviceID defaultID = kAudioDeviceUnknown;
    UInt32 defaultSize = sizeof(defaultID);
    AudioObjectGetPropertyData(kAudioObjectSystemObject, &propDefault, 0, NULL, &defaultSize, &defaultID);

    /* --- property addresses reused in the loop --- */
    AudioObjectPropertyAddress propStreams = {
        kAudioDevicePropertyStreams,
        kAudioDevicePropertyScopeOutput,
        0
    };
    AudioObjectPropertyAddress propUID = {
        kAudioDevicePropertyDeviceUID,
        kAudioObjectPropertyScopeGlobal,
        0
    };
    AudioObjectPropertyAddress propName = {
        kAudioObjectPropertyName,
        kAudioObjectPropertyScopeGlobal,
        0
    };
    AudioObjectPropertyAddress propRate = {
        kAudioDevicePropertyNominalSampleRate,
        kAudioObjectPropertyScopeGlobal,
        0
    };
    AudioObjectPropertyAddress propStreamCfg = {
        kAudioDevicePropertyStreamConfiguration,
        kAudioDevicePropertyScopeOutput,
        0
    };

    int written = 0;
    for (UInt32 i = 0; i < count && written < maxCount; i++) {
        AudioDeviceID devID = devices[i];

        /* skip input-only devices */
        UInt32 streamSize = 0;
        AudioObjectGetPropertyDataSize(devID, &propStreams, 0, NULL, &streamSize);
        if (streamSize == 0) continue;

        CADeviceEntry *e = &out[written];
        memset(e, 0, sizeof(*e));

        /* UID */
        CFStringRef cfUID = NULL;
        UInt32 uidSize = sizeof(cfUID);
        if (AudioObjectGetPropertyData(devID, &propUID, 0, NULL, &uidSize, &cfUID) == noErr && cfUID) {
            CFStringGetCString(cfUID, e->uid, sizeof(e->uid), kCFStringEncodingUTF8);
            CFRelease(cfUID);
        }

        /* Name */
        CFStringRef cfName = NULL;
        UInt32 nameSize = sizeof(cfName);
        if (AudioObjectGetPropertyData(devID, &propName, 0, NULL, &nameSize, &cfName) == noErr && cfName) {
            CFStringGetCString(cfName, e->name, sizeof(e->name), kCFStringEncodingUTF8);
            CFRelease(cfName);
        }

        /* Max output channels — sum across all output streams */
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

        /* Default sample rate */
        Float64 rate = 0;
        UInt32 rateSize = sizeof(rate);
        if (AudioObjectGetPropertyData(devID, &propRate, 0, NULL, &rateSize, &rate) == noErr)
            e->defaultSampleRate = (double)rate;

        /* Is default? */
        e->isDefault = (devID == defaultID) ? 1 : 0;

        written++;
    }

    free(devices);
    return written;
}
