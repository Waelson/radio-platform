#ifndef WASAPI_BRIDGE_H
#define WASAPI_BRIDGE_H

// WADeviceEntry holds structured metadata for one WASAPI render device.
typedef struct {
    char   id[256];               // IMMDevice::GetId() — stable GUID, persists across renames
    char   name[256];             // PKEY_Device_FriendlyName — human-readable label
    int    maxOutputChannels;     // mix format channel count
    double defaultSampleRate;     // mix format sample rate (Hz)
    int    isDefault;             // 1 if this is the system default render device
} WADeviceEntry;

// Fills out[0..maxCount-1] with metadata for each active render device.
// Returns the number of entries written (always <= maxCount).
int waEnumOutputDevices(WADeviceEntry *out, int maxCount);

// Find a device by its stable ID (GUID from IMMDevice::GetId()).
// On success returns 0 and sets *ppDevice to an AddRef'd IMMDevice*.
// Caller must pass the pointer to waReleaseDevice() when done.
int waFindDeviceByID(const char *id, void **ppDevice);

// Find a device by its friendly name (PKEY_Device_FriendlyName).
// On success returns 0 and sets *ppDevice to an AddRef'd IMMDevice*.
// Caller must pass the pointer to waReleaseDevice() when done.
int waFindDeviceByName(const char *name, void **ppDevice);

// Open a WASAPI shared-mode render stream.
// pDevice may be NULL to use the system default render device.
// sampleRate: desired sample rate (e.g. 48000); channels: 1 or 2.
// AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM lets WASAPI handle format conversion.
// On success returns 0 and sets *ppClient, *ppRender, *pBufferFrames.
// Caller must release *ppClient and *ppRender via waReleaseClient/waReleaseRender.
int waOpenRenderStream(void *pDevice, int sampleRate, int channels,
                       void **ppClient, void **ppRender,
                       unsigned int *pBufferFrames);

// Start playback on the audio client (IAudioClient::Start).
// Returns 0 on success, non-zero HRESULT on failure.
int waStartStream(void *pClient);

// Stop playback on the audio client (IAudioClient::Stop).
// Returns 0 on success, non-zero HRESULT on failure.
int waStopStream(void *pClient);

// Release an IMMDevice pointer returned by waFindDeviceByID/Name.
void waReleaseDevice(void *pDevice);

// Release an IAudioClient pointer returned by waOpenRenderStream.
void waReleaseClient(void *pClient);

// Release an IAudioRenderClient pointer returned by waOpenRenderStream.
void waReleaseRender(void *pRender);

// Returns the number of frames that can be written without blocking.
// Returns -1 on error.
int waGetAvailableFrames(void *pClient, unsigned int bufferFrames);

// Write numFrames of interleaved float32 PCM to the render client.
// Returns 0 on success, non-zero HRESULT on failure.
int waWriteFrames(void *pClient, void *pRender,
                  const float *frames, unsigned int numFrames, int channels);

// Initialise COM (COINIT_MULTITHREADED) on the calling thread.
// Safe to call multiple times per thread — reference-counted by the OS.
void waCoInit(void);

// Release the COM reference acquired by waCoInit on the calling thread.
void waCoUninit(void);

#endif /* WASAPI_BRIDGE_H */
