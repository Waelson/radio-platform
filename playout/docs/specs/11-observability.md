# 11 — Observabilidade

## Objetivo

Permitir diagnóstico rápido do Engine em operação local.

## Logs estruturados

Usar logs JSON ou formato estruturado equivalente.

Campos comuns:

```json
{
  "ts": "2026-01-25T23:20:12.123Z",
  "level": "INFO",
  "component": "playback",
  "event": "item_started",
  "queue_item_id": "qi_001",
  "asset_id": "asset_123"
}
```

## Níveis

- `DEBUG`: diagnóstico detalhado.
- `INFO`: eventos normais.
- `WARN`: condição anômala recuperável.
- `ERROR`: falha operacional.
- `FATAL`: falha irrecuperável no startup.

## Health endpoints

### GET /v1/health

Verifica vida do processo.

### GET /v1/ready

Verifica se pode aceitar comandos.

### GET /v1/status

Snapshot operacional.

## Métricas mínimas

Mesmo sem Prometheus inicialmente, expor em JSON:

```text
GET /v1/metrics
```

Resposta:

```json
{
  "uptime_seconds": 3600,
  "items_played_total": 120,
  "items_failed_total": 1,
  "commands_total": 88,
  "commands_rejected_total": 2,
  "underrun_total": 0,
  "panic_total": 0,
  "decoder_errors_total": 1,
  "output_errors_total": 0
}
```

## Build info

```text
GET /v1/build
```

Campos:

- version
- commit
- build_time
- go_version
- os
- arch

## Diagnóstico de áudio

Endpoint:

```text
GET /v1/audio/diagnostics
```

Resposta:

```json
{
  "output": {
    "device_id": "default",
    "sample_rate": 48000,
    "channels": 2,
    "buffer_frames": 2048
  },
  "health": {
    "level_dbfs": -14.2,
    "peak_dbfs": -3.1,
    "silence": false,
    "buffer_pct": 82,
    "underrun_count": 0
  },
  "current_pipeline": {
    "main_channel": "qi_001",
    "next_channel": null,
    "hot_channels": []
  }
}
```

## Event log

O Engine deve manter um ring buffer em memória dos últimos eventos.

Endpoint:

```text
GET /v1/events/recent?limit=100
```

Útil para UI e debug.

## Correlation ID

Todo comando recebido deve gerar `command_id`.

Esse ID deve aparecer em:

- resposta HTTP;
- evento `CommandAccepted` ou `CommandRejected`;
- logs internos.

## Não bloquear áudio

Observabilidade nunca pode bloquear o audio pipeline.

Se WebSocket estiver lento:

- descartar eventos não críticos.
- manter eventos críticos.
- nunca bloquear mixer/output.
