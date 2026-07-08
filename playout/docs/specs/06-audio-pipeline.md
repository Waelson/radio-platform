# 06 — Pipeline de Áudio

## Objetivo

Definir como áudio é decodificado, processado, mixado e enviado ao dispositivo.

## Pipeline conceitual

```text
Queue Item
   │
   ▼
Decoder
   │ PCM interno
   ▼
Track Channel
   │ gain / fade
   ▼
Mixer
   │ mixed PCM
   ▼
Output Buffer
   │
   ▼
Device Adapter
   │
   ▼
Audio Device
```

## Formato interno recomendado

Usar formato interno único para o mixer:

```text
sample_rate: 48000 Hz
channels: 2
sample_format: float32
layout: interleaved stereo
```

Motivo:

- float32 facilita mixagem, ganho e ducking.
- reduz risco de clipping durante soma.
- conversão para int16/s16le pode ocorrer apenas no output adapter se necessário.

## Decoder

Interface:

```go
type Decoder interface {
    Open(ctx context.Context, item QueueItem) (PCMStream, error)
}

type PCMStream interface {
    ReadFrames(ctx context.Context, dst []float32) (frames int, err error)
    Close() error
    Format() AudioFormat
}
```

Primeira implementação:

- `FFmpegDecoder`.
- Executa `ffmpeg` como subprocesso.
- Converte qualquer input suportado para PCM no formato interno.

Comando conceitual:

```bash
ffmpeg -hide_banner -loglevel error \
  -i input.mp3 \
  -f f32le \
  -acodec pcm_f32le \
  -ac 2 \
  -ar 48000 \
  pipe:1
```

## Mixer

O mixer recebe múltiplos canais.

Canais mínimos:

- `main`: música/fila principal.
- `next`: usado durante crossfade.
- `hot`: botoneira.
- `panic`: áudio de emergência.

Cada canal possui:

```go
type MixerChannel struct {
    ID string
    GainDB float64
    Muted bool
    Priority int
}
```

## Gain

Gain deve ser aplicado em escala linear internamente.

Conversão:

```text
linear = pow(10, db / 20)
```

## Crossfade

Crossfade é uma automação de gain entre dois canais:

```text
main:  1.0 → 0.0
next:  0.0 → 1.0
```

Durante o crossfade:

```text
out = main_sample * gain_main + next_sample * gain_next
```

## Ducking

Ducking reduz o volume do canal principal quando um canal prioritário toca.

Exemplo:

- Hot button toca vinheta.
- Main reduz para -8 dB.
- Vinheta toca a 0 dB.
- Ao terminar, main volta a 0 dB.

## Output Buffer

O Engine deve usar buffer entre mixer e dispositivo.

Objetivos:

- absorver jitter;
- evitar underrun;
- permitir medição de buffer.

## Audio Health

O Audio Health deve ser calculado preferencialmente no sinal final do mixer, antes do output adapter.

### Nível

Calcular RMS e Peak por janela.

```text
window: 50ms
update: 250ms ou 500ms
```

### Silêncio

Silêncio se RMS ficar abaixo do threshold por tempo configurado.

Exemplo:

```text
threshold_dbfs: -60
min_duration_ms: 2000
```

### Buffer

Percentual:

```text
buffer_pct = used_frames / capacity_frames * 100
```

## Controle de volume em software

O engine aplica um ganho de software independente de driver imediatamente antes de cada `output.Write()`.

### Mecanismo

```text
sample_out = sample_pcm * volume_level
```

- `volume_level ∈ [0.0, 1.0]`: `1.0` = sem atenuação; `0.0` = mudo.
- Se `volume_level == 1.0`, a função retorna imediatamente sem percorrer o buffer.
- O nível é armazenado em `atomic.Uint32` (bits de `float32`) — leitura lock-free no hot path de áudio.
- Alterações são efetivas no próximo buffer (~`buffer_frames / sample_rate` segundos de latência).

### Canais independentes

| Canal | Endpoint de controle | Evento publicado |
|---|---|---|
| Fila principal | `PUT /v1/playback/volume` | `VolumeChanged` |
| Preview (CUE) | `PUT /v1/preview/volume` | `PreviewVolumeChanged` |

O volume do canal CUE é encaminhado ao subprocess via IPC JSON (`{"cmd":"set_volume","volume":0.6}`).

### Persistência

O nível de cada canal é salvo em `~/.radiocore/preferences.json` após cada mudança e restaurado na inicialização — sem alterar o YAML de configuração estrutural.

## Hot path

No hot path de áudio:

- evitar alocação;
- evitar log por sample/frame;
- evitar chamadas HTTP;
- evitar acesso a banco;
- evitar locks longos;
- preferir buffers pré-alocados.

## Erros

Se decoder falhar:

- publicar `DecoderError`;
- marcar item como falho;
- tentar próximo item;
- se não houver próximo, entrar em `PANIC` ou `IDLE` conforme config.

Se output falhar:

- publicar `OutputError`;
- tentar reabrir device se permitido;
- se falhar, entrar em `ERROR` ou `PANIC`.
