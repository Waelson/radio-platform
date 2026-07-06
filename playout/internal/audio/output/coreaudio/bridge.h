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

// Finds an audio output device by its name.
// Returns noErr (0) and sets *outID on success.
// Returns kAudioHardwareUnknownPropertyError if not found.
OSStatus caFindDeviceByName(const char *name, AudioDeviceID *outID);

// Finds an audio output device by its persistent UID (kAudioDevicePropertyDeviceUID).
// Returns noErr (0) and sets *outID on success.
// Returns kAudioHardwareUnknownPropertyError if not found.
OSStatus caFindDeviceByUID(const char *uid, AudioDeviceID *outID);

// Routes an AudioQueue to a specific output device.
OSStatus caSetQueueDevice(AudioQueueRef queue, AudioDeviceID deviceID);

// Writes the names of all output devices into buf (newline-separated).
// bufSize is the total buffer capacity; returns the number of bytes written.
int caListOutputDevices(char *buf, int bufSize);

// CADeviceEntry holds structured metadata for one output device.
typedef struct {
    char   uid[256];            // kAudioDevicePropertyDeviceUID — persists across renames
    char   name[256];           // kAudioObjectPropertyName — human-readable label
    int    maxOutputChannels;   // total output channels across all output streams
    double defaultSampleRate;   // kAudioDevicePropertyNominalSampleRate
    int    isDefault;           // 1 if this is the system default output device
} CADeviceEntry;

// Fills out[0..maxCount-1] with metadata for each available output device.
// Returns the number of entries written (always <= maxCount).
int caEnumOutputDevices(CADeviceEntry *out, int maxCount);

#endif
