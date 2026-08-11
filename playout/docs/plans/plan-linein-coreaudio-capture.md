# Plano: Captura Line-In via CoreAudio Nativo (macOS)

## Problema

A implementação atual de captura usa FFmpeg como subprocess:

```
[AVFoundation] → [FFmpeg subprocess] → [pipe OS] → [goroutine Go] → [channel] → [mixerOut.Write()] → [CoreAudio Output]
```

Cada camada adiciona jitter independente de sincronização:
- FFmpeg tem bufferização interna própria
- O pipe OS tem bufferização de kernel
- O agendamento do subprocess pelo OS não tem garantias de tempo real
- A entrega via `io.ReadFull()` bloqueia até ter `bufferFrames` samples, criando um perfil de latência irregular

O resultado é áudio "picotado" mesmo com buffers grandes (`bufferFrames = 16384` ≈ 341 ms).

## Solução Proposta

Substituir `FFmpegCapture` por `CoreAudioCapture` no macOS: captura diretamente via `AudioQueueNewInput`, com callback de hardware entregando PCM a cada ~10 ms (512 samples @ 48 kHz) num ring buffer SPSC em C. O Go lê desse ring de forma não bloqueante.

```
[CoreAudio Hardware Input]
   ↓ callback C (hardware-timed, ~10 ms)
[SPSC Ring Buffer C]
   ↓ goroutine Go (polling leve, 1 ms)
[mixerOut.Write()]
   ↓
[CoreAudio Hardware Output]
```

A interface `InputDevice` permanece inalterada. Apenas a implementação muda no macOS com build tag `coreaudio`.

---

## Arquitetura de Arquivos

```
internal/linein/
  coreaudio_input.h            # declarações C (ring buffer + AudioQueue input)
  coreaudio_input.c            # implementação C
  coreaudio_capture_darwin.go  # Go wrapper — build tag: coreaudio
  capture_factory_coreaudio.go # NewCapture() → CoreAudioCapture — build: coreaudio
  capture_factory_ffmpeg.go    # NewCapture() → FFmpegCapture   — build: !coreaudio
```

Os arquivos existentes (`ffmpeg_capture.go`, `ffmpeg_args_*.go`) permanecem como fallback para Linux/Windows.

---

## Fases de Implementação

---

### Fase 1 — Bridge C: Ring Buffer + AudioQueue Input

**Arquivo:** `internal/linein/coreaudio_input.h`

Declarações:

```c
typedef struct {
    float            *data;
    uint32_t          cap;
    _Atomic uint32_t  head; // produtor: callback C
    _Atomic uint32_t  tail; // consumidor: goroutine Go
} CAInputRing;

// Ring buffer
CAInputRing *caInputRingCreate(uint32_t capacitySamples);
void         caInputRingFree(CAInputRing *r);
uint32_t     caInputRingWrite(CAInputRing *r, const float *src, uint32_t n);
uint32_t     caInputRingRead(CAInputRing *r, float *dst, uint32_t n);
uint32_t     caInputRingAvail(CAInputRing *r);

// Contexto da sessão de captura
typedef struct CAInputSession CAInputSession;

CAInputSession *caInputOpen(const char *deviceUID, uint32_t sampleRate,
                            uint32_t channels, uint32_t ringCapSamples,
                            char *errBuf, int errBufLen);
int             caInputStart(CAInputSession *s);
void            caInputStop(CAInputSession *s);
void            caInputClose(CAInputSession *s);
uint32_t        caInputRead(CAInputSession *s, float *dst, uint32_t n);
uint32_t        caInputAvail(CAInputSession *s);
```

**Arquivo:** `internal/linein/coreaudio_input.c`

Implementação:

1. **Ring buffer SPSC** — idêntico ao usado no output (`CARingBuf`), sem lock, seguro entre callback C (produtor) e goroutine Go (consumidor).

2. **`caInputOpen()`**:
   - Cria `CAInputRing` com capacidade de 4 s de áudio (48000 × 2 × 4 = 384 000 samples)
   - `AudioQueueNewInput()` com formato `kAudioFormatLinearPCM | kLinearPCMFormatFlagIsFloat | kLinearPCMFormatFlagIsPacked`, 48 kHz, 2ch, interleaved
   - Se `deviceUID != NULL`, resolve o `AudioDeviceID` via `AudioObjectGetPropertyData(kAudioHardwarePropertyDevices)` e define `kAudioQueueProperty_CurrentDevice`
   - Aloca 3 `AudioQueueBuffer` de 512 samples cada (≈ 10 ms) e os enfileira

3. **Callback de input** (`AudioQueueInputCallback`):
   - Recebe `AudioQueueBufferRef` com frames float32 capturados pelo hardware
   - Escreve via `caInputRingWrite()` no ring buffer (não bloqueia — se ring cheio, descarta, preferível a bloquear o callback de hardware)
   - Re-enfileira o buffer (`AudioQueueEnqueueBuffer`)

4. **`caInputStart()`**: `AudioQueueStart(queue, NULL)`

5. **`caInputStop()`**: `AudioQueueStop(queue, true)` (flush síncrono)

6. **`caInputClose()`**: `AudioQueueDispose()` + `caInputRingFree()`

7. **`caInputRead()`**: lê até `n` samples do ring, retorna quantidade lida (pode ser 0)

8. **`caInputAvail()`**: retorna samples disponíveis no ring

**Formato de buffer:** 3 buffers × 512 samples × 2 ch × 4 bytes = 12 288 bytes total.
A cada ~10 ms o hardware dispara o callback → ≈ 100 callbacks/s.

---

### Fase 2 — Go Wrapper: `CoreAudioCapture`

**Arquivo:** `internal/linein/coreaudio_capture_darwin.go`
**Build tag:** `//go:build coreaudio`

```go
// CoreAudioCapture implementa InputDevice usando AudioQueue nativo.
// O callback C escreve no ring a cada ~10 ms (hardware-timed).
// ReadFrames() lê do ring com polling leve (1 ms sleep quando vazio).
type CoreAudioCapture struct {
    log     *slog.Logger
    session *C.CAInputSession // ponteiro opaco para a sessão C
}

func NewCoreAudioCapture(log *slog.Logger) *CoreAudioCapture

// Open: chama caInputOpen() e caInputStart()
func (c *CoreAudioCapture) Open(ctx context.Context, cfg LineInConfig) error

// ReadFrames: polling no ring até ter n frames ou ctx cancelado
// Loop: caInputRead() → se 0, sleep 1ms → repete
// Retorna io.EOF quando ctx cancelado (sessão encerrada)
func (c *CoreAudioCapture) ReadFrames(ctx context.Context, dst []float32) (int, error)

// Close: caInputStop() + caInputClose()
func (c *CoreAudioCapture) Close() error
```

**`ReadFrames()` em detalhe:**

```
loop:
  n = caInputRead(session, &dst[accumulated], remaining)
  accumulated += n
  if accumulated >= len(dst): return accumulated/channels, nil
  if ctx.Done(): return accumulated/channels, io.EOF
  sleep 1ms   ← headroom mínimo, não gasta CPU
```

Com buffer de 512 samples no hardware e `ReadFrames` pedindo 2048 samples (≈ 42 ms), o Go precisará esperar ~4 callbacks antes de ter dados suficientes — latência previsível, sem jitter de pipe.

**Device ID:**
- `"default"` ou `""` → passa `NULL` para `caInputOpen()`, AudioQueue usa o default input
- Qualquer outro valor → passa o UID como string (mesmo formato de `device_list_darwin.go`)

---

### Fase 3 — Factory de Captura

**Arquivo:** `internal/linein/capture_factory_coreaudio.go` (`//go:build coreaudio`)

```go
func NewCapture(log *slog.Logger) InputDevice {
    return NewCoreAudioCapture(log)
}
```

**Arquivo:** `internal/linein/capture_factory_ffmpeg.go` (`//go:build !coreaudio`)

```go
func NewCapture(log *slog.Logger) InputDevice {
    return NewFFmpegCapture(log)
}
```

**Mudança em `handler.go`:**

```go
// Antes:
capture := NewFFmpegCapture(h.log)

// Depois:
capture := NewCapture(h.log)
```

Um único ponto de troca. Todo o resto permanece inalterado.

---

### Fase 4 — Device Listing via CoreAudio (opcional)

**Arquivo:** `internal/linein/device_list_darwin.go` (substituição)

Atual: usa FFmpeg para listar dispositivos (`ffmpeg -list_devices true -f avfoundation ...`), que é lento e frágil.

Proposta: usar `AudioObjectGetPropertyData` com `kAudioHardwarePropertyDevices` + `kAudioObjectPropertyScopeInput` para listar dispositivos diretamente, retornando `DeviceInfo{ID: uid, Name: name}`.

Vantagens:
- Instantâneo (sem subprocess)
- Retorna UIDs estáveis que CoreAudio aceita diretamente
- Não depende do formato de saída do FFmpeg

---

## Impacto na Qualidade de Áudio

| Fonte de jitter | FFmpeg (atual) | CoreAudio Nativo |
|---|---|---|
| Startup do subprocess | ~200–500 ms | nenhum |
| Bufferização interna FFmpeg | variável | nenhum |
| Pipe OS (kernel buffer) | variável | nenhum |
| Agendamento de processo | variável | nenhum |
| Callback de hardware | — | fixo, hardware-timed |
| Latência total | ~300–700 ms | ~10–40 ms |
| Jitter | alto | mínimo |

---

## O que NÃO muda

- Interface `InputDevice` — inalterada
- `LineInManager`, `Handler`, `mixer/mixer.go` — inalterados
- Linux e Windows continuam com `FFmpegCapture`
- `FFmpegCapture` permanece no repositório como fallback explícito
- Formato de áudio interno (float32, 48 kHz, 2ch) — inalterado

---

## Ordem de Implementação

1. `coreaudio_input.h` + `coreaudio_input.c` (ring buffer + AudioQueue input)
2. `coreaudio_capture_darwin.go` (Go wrapper)
3. `capture_factory_coreaudio.go` + `capture_factory_ffmpeg.go`
4. Atualizar `handler.go` para usar `NewCapture()`
5. Build + teste manual de captura
6. `device_list_darwin.go` via CoreAudio (opcional, Fase 4)

---

## Estimativa de Complexidade

- Fase 1 (C bridge): ~200 linhas C — menor que o bridge de output existente
- Fase 2 (Go wrapper): ~80 linhas Go — análogo a `FFmpegCapture`
- Fase 3 (factory): ~10 linhas — trivial
- Fase 4 (device list): ~60 linhas C + 30 linhas Go
