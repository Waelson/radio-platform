#include "bridge.h"
#include <string.h>

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
