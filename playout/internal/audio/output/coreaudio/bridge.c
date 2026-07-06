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
