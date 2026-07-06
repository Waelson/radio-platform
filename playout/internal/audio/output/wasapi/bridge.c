// WASAPI CGo bridge — Windows Audio Session API (WASAPI) output device.
// Compiled only on Windows with -tags wasapi.
//
// Design notes:
//   - Shared-mode rendering with AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM so WASAPI
//     handles sample-rate / channel conversion automatically (Windows 8.1+).
//   - Device identity is the stable GUID from IMMDevice::GetId(), which
//     survives device renames in Sound Settings.
//   - COM is initialised per-thread (MTA) by waCoInit(); callers must call it
//     on every OS thread that invokes a COM-using function.
#define COBJMACROS
#define INITGUID
#define WIN32_LEAN_AND_MEAN
#include "bridge.h"
#include <windows.h>
#include <objbase.h>
#include <mmdeviceapi.h>
#include <audioclient.h>
#include <propsys.h>
#include <string.h>
#include <stdlib.h>

// AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM / _SRC_DEFAULT_QUALITY (Windows 8.1+).
// Defined here for toolchains whose SDK headers may predate Win8.1.
#ifndef AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM
#define AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM 0x80000000
#endif
#ifndef AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY
#define AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY 0x08000000
#endif

// PKEY_Device_FriendlyName — {A45C254E-DF1C-4EFD-8020-67D146A850E0} pid=14
// Defined locally to avoid dependency on functiondiscoverykeys_devpkey.h.
static const PROPERTYKEY kFriendlyName = {
    {0xa45c254e, 0xdf1c, 0x4efd, {0x80, 0x20, 0x67, 0xd1, 0x46, 0xa8, 0x50, 0xe0}},
    14
};

/* -------------------------------------------------------------------------
 * Internal helpers
 * ---------------------------------------------------------------------- */

// Convert a wide-character string to UTF-8, writing at most maxLen bytes.
static void wstrToUtf8(LPCWSTR src, char *dst, int maxLen) {
    if (!src || maxLen <= 0) {
        if (dst && maxLen > 0) dst[0] = '\0';
        return;
    }
    WideCharToMultiByte(CP_UTF8, 0, src, -1, dst, maxLen, NULL, NULL);
}

// Read PKEY_Device_FriendlyName from pDevice's property store.
static void getDeviceName(IMMDevice *pDevice, char *buf, int bufLen) {
    buf[0] = '\0';
    IPropertyStore *pStore = NULL;
    if (FAILED(IMMDevice_OpenPropertyStore(pDevice, STGM_READ, &pStore)) || !pStore)
        return;

    PROPVARIANT var;
    PropVariantInit(&var);
    if (SUCCEEDED(IPropertyStore_GetValue(pStore, &kFriendlyName, &var)) &&
        var.vt == VT_LPWSTR && var.pwszVal) {
        wstrToUtf8(var.pwszVal, buf, bufLen);
    }
    PropVariantClear(&var);
    IPropertyStore_Release(pStore);
}

// Activate an IAudioClient on pDevice and read its mix format channels + rate.
static void getDeviceMixFormat(IMMDevice *pDevice,
                                int *channels, double *sampleRate) {
    *channels   = 0;
    *sampleRate = 0.0;

    IAudioClient *pClient = NULL;
    if (FAILED(IMMDevice_Activate(pDevice, &IID_IAudioClient,
                                   CLSCTX_ALL, NULL, (void **)&pClient)) || !pClient)
        return;

    WAVEFORMATEX *pwfx = NULL;
    if (SUCCEEDED(IAudioClient_GetMixFormat(pClient, &pwfx)) && pwfx) {
        *channels   = (int)pwfx->nChannels;
        *sampleRate = (double)pwfx->nSamplesPerSec;
        CoTaskMemFree(pwfx);
    }
    IAudioClient_Release(pClient);
}

/* -------------------------------------------------------------------------
 * Public API
 * ---------------------------------------------------------------------- */

void waCoInit(void) {
    CoInitializeEx(NULL, COINIT_MULTITHREADED);
}

void waCoUninit(void) {
    CoUninitialize();
}

int waEnumOutputDevices(WADeviceEntry *out, int maxCount) {
    if (!out || maxCount <= 0) return 0;

    CoInitializeEx(NULL, COINIT_MULTITHREADED);

    IMMDeviceEnumerator *pEnum = NULL;
    if (FAILED(CoCreateInstance(&CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL,
                                 &IID_IMMDeviceEnumerator, (void **)&pEnum)) || !pEnum)
        return 0;

    // Resolve default device ID for comparison.
    LPWSTR defaultId = NULL;
    IMMDevice *pDefault = NULL;
    if (SUCCEEDED(IMMDeviceEnumerator_GetDefaultAudioEndpoint(
            pEnum, eRender, eConsole, &pDefault)) && pDefault) {
        IMMDevice_GetId(pDefault, &defaultId);
        IMMDevice_Release(pDefault);
    }

    IMMDeviceCollection *pColl = NULL;
    if (FAILED(IMMDeviceEnumerator_EnumAudioEndpoints(
            pEnum, eRender, DEVICE_STATE_ACTIVE, &pColl)) || !pColl) {
        if (defaultId) CoTaskMemFree(defaultId);
        IMMDeviceEnumerator_Release(pEnum);
        return 0;
    }

    UINT count = 0;
    IMMDeviceCollection_GetCount(pColl, &count);

    int written = 0;
    for (UINT i = 0; i < count && written < maxCount; i++) {
        IMMDevice *pDevice = NULL;
        if (FAILED(IMMDeviceCollection_Item(pColl, i, &pDevice)) || !pDevice)
            continue;

        WADeviceEntry *e = &out[written];
        memset(e, 0, sizeof(*e));

        // Stable GUID identifier.
        LPWSTR pwszId = NULL;
        if (SUCCEEDED(IMMDevice_GetId(pDevice, &pwszId)) && pwszId) {
            wstrToUtf8(pwszId, e->id, sizeof(e->id));
            if (defaultId && wcscmp(pwszId, defaultId) == 0)
                e->isDefault = 1;
            CoTaskMemFree(pwszId);
        }

        getDeviceName(pDevice, e->name, sizeof(e->name));
        getDeviceMixFormat(pDevice, &e->maxOutputChannels, &e->defaultSampleRate);

        IMMDevice_Release(pDevice);
        written++;
    }

    if (defaultId) CoTaskMemFree(defaultId);
    IMMDeviceCollection_Release(pColl);
    IMMDeviceEnumerator_Release(pEnum);
    return written;
}

int waFindDeviceByID(const char *id, void **ppDevice) {
    if (!id || !ppDevice) return -1;
    *ppDevice = NULL;

    CoInitializeEx(NULL, COINIT_MULTITHREADED);

    // Convert UTF-8 id to wide string.
    int wlen = MultiByteToWideChar(CP_UTF8, 0, id, -1, NULL, 0);
    if (wlen <= 0) return -1;
    LPWSTR wid = (LPWSTR)malloc((size_t)wlen * sizeof(WCHAR));
    if (!wid) return -1;
    MultiByteToWideChar(CP_UTF8, 0, id, -1, wid, wlen);

    IMMDeviceEnumerator *pEnum = NULL;
    HRESULT hr = CoCreateInstance(&CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL,
                                   &IID_IMMDeviceEnumerator, (void **)&pEnum);
    if (FAILED(hr) || !pEnum) { free(wid); return -1; }

    IMMDevice *pDevice = NULL;
    hr = IMMDeviceEnumerator_GetDevice(pEnum, wid, &pDevice);
    free(wid);
    IMMDeviceEnumerator_Release(pEnum);

    if (FAILED(hr) || !pDevice) return -1;
    *ppDevice = pDevice;
    return 0;
}

int waFindDeviceByName(const char *name, void **ppDevice) {
    if (!name || !ppDevice) return -1;
    *ppDevice = NULL;

    CoInitializeEx(NULL, COINIT_MULTITHREADED);

    IMMDeviceEnumerator *pEnum = NULL;
    if (FAILED(CoCreateInstance(&CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL,
                                 &IID_IMMDeviceEnumerator, (void **)&pEnum)) || !pEnum)
        return -1;

    IMMDeviceCollection *pColl = NULL;
    if (FAILED(IMMDeviceEnumerator_EnumAudioEndpoints(
            pEnum, eRender, DEVICE_STATE_ACTIVE, &pColl)) || !pColl) {
        IMMDeviceEnumerator_Release(pEnum);
        return -1;
    }

    UINT count = 0;
    IMMDeviceCollection_GetCount(pColl, &count);

    int found = -1;
    char devName[256];
    for (UINT i = 0; i < count; i++) {
        IMMDevice *pDevice = NULL;
        if (FAILED(IMMDeviceCollection_Item(pColl, i, &pDevice)) || !pDevice)
            continue;

        getDeviceName(pDevice, devName, sizeof(devName));
        if (strcmp(devName, name) == 0) {
            *ppDevice = pDevice; // caller owns this reference
            found = 0;
            break;
        }
        IMMDevice_Release(pDevice);
    }

    IMMDeviceCollection_Release(pColl);
    IMMDeviceEnumerator_Release(pEnum);
    return found;
}

int waOpenRenderStream(void *pDevice, int sampleRate, int channels,
                       void **ppClient, void **ppRender,
                       unsigned int *pBufferFrames) {
    if (!ppClient || !ppRender || !pBufferFrames) return -1;
    *ppClient     = NULL;
    *ppRender     = NULL;
    *pBufferFrames = 0;

    CoInitializeEx(NULL, COINIT_MULTITHREADED);

    IMMDevice *dev = (IMMDevice *)pDevice;

    // Resolve default device when none was provided.
    IMMDevice *pDefaultDev = NULL;
    if (!dev) {
        IMMDeviceEnumerator *pEnum = NULL;
        if (FAILED(CoCreateInstance(&CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL,
                                     &IID_IMMDeviceEnumerator, (void **)&pEnum)) || !pEnum)
            return -1;
        HRESULT hr = IMMDeviceEnumerator_GetDefaultAudioEndpoint(
            pEnum, eRender, eConsole, &pDefaultDev);
        IMMDeviceEnumerator_Release(pEnum);
        if (FAILED(hr) || !pDefaultDev) return -1;
        dev = pDefaultDev;
    }

    IAudioClient *pClient = NULL;
    HRESULT hr = IMMDevice_Activate(dev, &IID_IAudioClient,
                                     CLSCTX_ALL, NULL, (void **)&pClient);
    if (pDefaultDev) IMMDevice_Release(pDefaultDev);
    if (FAILED(hr) || !pClient) return (int)hr;

    // Request IEEE float PCM at the engine's native rate and channel count.
    // AUTOCONVERTPCM lets WASAPI resample / remix to the hardware mix format.
    WAVEFORMATEX wfx;
    memset(&wfx, 0, sizeof(wfx));
    wfx.wFormatTag      = WAVE_FORMAT_IEEE_FLOAT;
    wfx.nChannels       = (WORD)channels;
    wfx.nSamplesPerSec  = (DWORD)sampleRate;
    wfx.wBitsPerSample  = 32;
    wfx.nBlockAlign     = (WORD)(channels * (int)sizeof(float));
    wfx.nAvgBytesPerSec = wfx.nSamplesPerSec * (DWORD)wfx.nBlockAlign;
    wfx.cbSize          = 0;

    // 200 ms shared buffer (units: 100-nanosecond intervals).
    REFERENCE_TIME hnsBuffer = 2000000;

    hr = IAudioClient_Initialize(
        pClient,
        AUDCLNT_SHAREMODE_SHARED,
        AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM | AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY,
        hnsBuffer,
        0,
        &wfx,
        NULL
    );
    if (FAILED(hr)) {
        IAudioClient_Release(pClient);
        return (int)hr;
    }

    UINT32 bufFrames = 0;
    hr = IAudioClient_GetBufferSize(pClient, &bufFrames);
    if (FAILED(hr)) {
        IAudioClient_Release(pClient);
        return (int)hr;
    }

    IAudioRenderClient *pRender = NULL;
    hr = IAudioClient_GetService(pClient, &IID_IAudioRenderClient, (void **)&pRender);
    if (FAILED(hr) || !pRender) {
        IAudioClient_Release(pClient);
        return (int)hr;
    }

    *ppClient     = pClient;
    *ppRender     = pRender;
    *pBufferFrames = (unsigned int)bufFrames;
    return 0;
}

int waStartStream(void *pClient) {
    if (!pClient) return -1;
    HRESULT hr = IAudioClient_Start((IAudioClient *)pClient);
    return FAILED(hr) ? (int)hr : 0;
}

int waStopStream(void *pClient) {
    if (!pClient) return -1;
    HRESULT hr = IAudioClient_Stop((IAudioClient *)pClient);
    return FAILED(hr) ? (int)hr : 0;
}

void waReleaseDevice(void *pDevice) {
    if (pDevice) IMMDevice_Release((IMMDevice *)pDevice);
}

void waReleaseClient(void *pClient) {
    if (pClient) IAudioClient_Release((IAudioClient *)pClient);
}

void waReleaseRender(void *pRender) {
    if (pRender) IAudioRenderClient_Release((IAudioRenderClient *)pRender);
}

int waGetAvailableFrames(void *pClient, unsigned int bufferFrames) {
    if (!pClient) return -1;
    UINT32 padding = 0;
    HRESULT hr = IAudioClient_GetCurrentPadding((IAudioClient *)pClient, &padding);
    if (FAILED(hr)) return -1;
    int avail = (int)bufferFrames - (int)padding;
    return avail > 0 ? avail : 0;
}

int waWriteFrames(void *pClient, void *pRender,
                  const float *frames, unsigned int numFrames, int channels) {
    if (!pClient || !pRender || !frames || numFrames == 0) return -1;

    BYTE *pData = NULL;
    HRESULT hr = IAudioRenderClient_GetBuffer(
        (IAudioRenderClient *)pRender, numFrames, &pData);
    if (FAILED(hr) || !pData) return FAILED(hr) ? (int)hr : -1;

    // Copy interleaved float32 samples.
    size_t bytes = (size_t)numFrames * (size_t)channels * sizeof(float);
    memcpy(pData, frames, bytes);

    hr = IAudioRenderClient_ReleaseBuffer(
        (IAudioRenderClient *)pRender, numFrames, 0);
    return FAILED(hr) ? (int)hr : 0;
}
