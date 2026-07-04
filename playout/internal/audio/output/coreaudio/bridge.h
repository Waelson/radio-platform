#ifndef COREAUDIO_BRIDGE_H
#define COREAUDIO_BRIDGE_H

#include <AudioToolbox/AudioToolbox.h>

// Called by the AudioQueue callback; implemented in coreaudio.go via CGO export.
extern void goBufferReady(void *userData, AudioQueueRef queue, AudioQueueBufferRef buf);

// Creates and configures an output AudioQueue.
// Returns 0 on success, OSStatus on failure.
OSStatus caNewQueue(
    double         sampleRate,
    int            channels,
    void          *userData,
    AudioQueueRef *outQueue
);

// Allocates a buffer in the queue.
OSStatus caAllocBuffer(
    AudioQueueRef        queue,
    int                  bufferFrames,
    int                  channels,
    AudioQueueBufferRef *outBuf
);

// Fills a buffer with float32 PCM and enqueues it.
OSStatus caEnqueueBuffer(
    AudioQueueRef       queue,
    AudioQueueBufferRef buf,
    const float        *frames,
    int                 nFrames,
    int                 channels
);

// Returns 1 if the AudioQueue is currently running, 0 otherwise.
int caIsRunning(AudioQueueRef queue);

#endif
