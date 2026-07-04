# 12 — Configuração

## Objetivo

Permitir configuração simples por arquivo e variáveis de ambiente.

## Ordem de precedência

1. Flags CLI.
2. Variáveis de ambiente.
3. Arquivo de configuração.
4. Defaults internos.

## Arquivo padrão

Nome sugerido:

```text
playout-engine.yaml
```

## Exemplo

```yaml
engine:
  id: "studio-a-main"
  instance_lock: true

api:
  host: "127.0.0.1"
  port: 8080
  cors:
    enabled: true
    allowed_origins:
      - "http://localhost:3000"
      - "http://localhost:5173"

audio:
  sample_rate: 48000
  channels: 2
  buffer_frames: 2048
  output:
    driver: "portaudio"
    device_id: "default"
    allow_null_output: false

playback:
  default_crossfade_ms: 8000
  default_stop_fade_ms: 300
  preload_next_ms: 3000
  max_consecutive_item_failures: 3

health:
  progress_interval_ms: 500
  audio_health_interval_ms: 500
  silence_threshold_dbfs: -60
  silence_duration_ms: 2000

panic:
  enabled: true
  bed_path: "./library/beds/panic-bed.mp3"
  auto_on_silence: true

logging:
  level: "info"
  format: "json"
```

## Variáveis de ambiente

Prefixo:

```text
PLAYOUT_
```

Exemplos:

```text
PLAYOUT_API_PORT=8080
PLAYOUT_AUDIO_OUTPUT_DRIVER=portaudio
PLAYOUT_LOG_LEVEL=debug
```

## Flags CLI

Exemplos:

```bash
playout-engine --config ./playout-engine.yaml
playout-engine --api-port 8080
playout-engine --log-level debug
```

## Validação

Na inicialização, validar:

- porta livre;
- output driver conhecido;
- sample rate válido;
- channels válido;
- panic bed existente se panic habilitado;
- diretórios necessários.

## Defaults recomendados

| Campo | Default |
|---|---|
| api.host | 127.0.0.1 |
| api.port | 8080 |
| audio.sample_rate | 48000 |
| audio.channels | 2 |
| audio.buffer_frames | 2048 |
| playback.default_crossfade_ms | 8000 |
| health.silence_threshold_dbfs | -60 |
| health.silence_duration_ms | 2000 |

## Hot reload

Não implementar hot reload na primeira versão.

Futuro:

- permitir reload seguro de configurações não críticas;
- nunca alterar output device enquanto tocando sem comando explícito.
