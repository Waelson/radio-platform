# 09 — Abstração de Dispositivo de Áudio

## Objetivo

Permitir que o Engine rode em macOS, Linux e Windows sem acoplar o núcleo do playback a uma API específica.

## Interface

```go
type OutputDevice interface {
    Open(ctx context.Context, cfg OutputConfig) error
    Start(ctx context.Context) error
    Write(ctx context.Context, frames []float32) (int, error)
    Stop(ctx context.Context) error
    Close() error
    Info() OutputDeviceInfo
}
```

## OutputConfig

```go
type OutputConfig struct {
    DeviceID string
    SampleRate int
    Channels int
    BufferFrames int
}
```

## OutputDeviceInfo

```go
type OutputDeviceInfo struct {
    ID string
    Name string
    Driver string
    SampleRate int
    Channels int
}
```

## Implementações

### NullOutput

Obrigatório para testes.

- Consome frames.
- Não toca áudio.
- Permite medir fluxo.

### FileOutput

Opcional para testes.

- Escreve PCM/WAV em arquivo.
- Útil para validar crossfade automaticamente.

### PortAudioOutput

Implementação inicial recomendada para execução local cross-platform.

Requisitos:

- macOS: CoreAudio via PortAudio.
- Linux: ALSA/Pulse/JACK via PortAudio.
- Windows: WASAPI/DirectSound via PortAudio.

### NativeOutput futuro

Implementações futuras podem usar diretamente:

- CoreAudio no macOS.
- ALSA/JACK/PulseAudio no Linux.
- WASAPI no Windows.

## DeviceLister

Interface opcional implementada por drivers que suportam enumeração de dispositivos:

```go
type DeviceLister interface {
    ListDevices() ([]DeviceInfo, error)
}
```

Todos os drivers implementam `DeviceLister`: `NullOutput`, `FileOutput`, `coreaudio.Output` e `portaudio.Output`.

### DeviceInfo

```go
type DeviceInfo struct {
    ID                string  // identificador único (semântica varia por driver)
    Name              string  // nome legível (ex: "MacBook Pro Speakers")
    Driver            string  // "coreaudio" | "portaudio" | "null" | "file"
    HostAPI           string  // "ALSA" | "PulseAudio" | "JACK" | "CoreAudio" | "WASAPI" | ""
    IsDefault         bool    // true se for o output padrão do sistema
    MaxOutputChannels int     // número máximo de canais de saída suportados
    DefaultSampleRate float64 // taxa de amostragem padrão reportada pelo SO
}
```

### Semântica do campo `ID` por driver

| Driver | Valor de `ID` | Estabilidade |
|---|---|---|
| `coreaudio` | `kAudioDevicePropertyDeviceUID` — string opaca, ex: `"AppleHDAEngineOutput:0,1"` | Persiste mesmo se o nome do dispositivo for alterado no SO |
| `wasapi` | GUID de `IMMDevice::GetId()` — ex: `"{0.0.0.00000000}.{1a2b3c4d-...}"` | Persiste mesmo se o dispositivo for renomeado em Sound Settings |
| `portaudio` | Igual ao `Name` — PortAudio não expõe UID interno | Muda se o dispositivo for renomeado no SO |
| `null` / `file` | `"null"` / `"file"` (fixo) | Sempre estável |

### Campo `HostAPI` — estabilidade no Linux por host API

O campo `host_api` (exposto na API REST como `host_api`, omitido se vazio) indica qual host API está por trás do dispositivo. No Linux, isso é relevante para avaliar a estabilidade do `id`:

| Host API | Estabilidade do ID | Observação |
|---|---|---|
| `ALSA` | Razoavelmente estável | Nome PortAudio = nome ALSA; hardware cards costumam manter o nome |
| `PulseAudio` | Parcialmente estável | Display name pode mudar; sink name interno não é exposto via PortAudio |
| `PipeWire` | Parcialmente estável | Semelhante ao PulseAudio — display name exposto |
| `JACK` | Estável | Port names são estáveis por natureza do protocolo |
| `CoreAudio` | Estável | Usa UID interno (`kAudioDevicePropertyDeviceUID`) |
| `""` | N/A | Drivers `null` e `file` (pseudo-dispositivos) |

### Driver WASAPI (Windows)

O pacote `internal/audio/output/wasapi` implementa `OutputDevice` e `DeviceLister` usando WASAPI shared-mode via CGo + COM.

**Características:**
- `DeviceInfo.ID` = GUID de `IMMDevice::GetId()` — persiste mesmo após renomear o dispositivo
- `HostAPI` = `"WASAPI"` em todos os dispositivos listados
- Renderização float32 com `AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM` — WASAPI realiza conversão de formato automaticamente (Windows 8.1+)
- Buffer compartilhado de 200 ms; polling de 1 ms para disponibilidade de frames
- Resolução em cascata em `Open()`: GUID → nome amigável → default do sistema

**Build:** `go build -tags wasapi ./...` (apenas Windows; requer MinGW-w64)

**Dependências C:** `ole32`, `oleaut32`, `uuid` (incluídas em toda instalação Windows)

### Tabela de drivers recomendados por plataforma

| Plataforma | Driver recomendado | Build tag | ID estável? |
|---|---|---|---|
| macOS | `coreaudio` | `coreaudio` | Sim — UID (`kAudioDevicePropertyDeviceUID`) |
| Linux | `portaudio` | `portaudio` | Parcial — depende do host API (ALSA > PulseAudio > PipeWire) |
| Windows | `wasapi` | `wasapi` | Sim — GUID (`IMMDevice::GetId()`) |
| Testes / CI | `null` | — | Sempre |

### Endpoint REST

```
GET /v1/devices
```

Retorna a lista atualizada a cada request — sem cache (`Cache-Control: no-store`). Ver `docs/specs/03-api-rest.md` para contrato completo.

### Wiring

O `OutputDevice` criado em `main.go` é type-assertado para `DeviceLister`. Se o driver implementar a interface, a função de listagem é injetada no servidor HTTP via `api.DevicesDeps`:

```go
if lister, ok := out.(output.DeviceLister); ok {
    devicesDeps.List = func() ([]handlers.AudioDevice, error) { ... }
}
```

## Seleção de dispositivo

## Regras

- O core do Engine nunca deve importar diretamente bibliotecas específicas de output.
- O output adapter deve ficar isolado em pacote próprio.
- Testes unitários devem usar `NullOutput`.
- Testes de integração podem usar `FileOutput`.

## Buffer

Configurações iniciais:

```yaml
audio:
  sample_rate: 48000
  channels: 2
  buffer_frames: 2048
```

## Erros

Se device falhar ao abrir:

- publicar `OutputOpenFailed`;
- Engine pode iniciar em modo degradado se `allow_null_output=true`;
- caso contrário, entrar em `ERROR`.

Se device falhar durante write:

- publicar `OutputWriteFailed`;
- tentar reabrir se configurado;
- se falhar, entrar em `PANIC` ou `ERROR`.
