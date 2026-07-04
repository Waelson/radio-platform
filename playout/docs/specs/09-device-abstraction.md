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

## Seleção de dispositivo

Endpoint futuro:

```text
GET /v1/audio/devices
```

Resposta:

```json
{
  "devices": [
    {
      "id": "default",
      "name": "Default Output",
      "driver": "coreaudio",
      "channels": 2,
      "sample_rate": 48000
    }
  ]
}
```

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
