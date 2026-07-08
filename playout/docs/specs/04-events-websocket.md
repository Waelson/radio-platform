# 04 — Eventos via WebSocket

## Objetivo

A UI deve receber informações em tempo real sem polling agressivo.

O Engine expõe:

```text
GET /v1/events
```

Protocolo: WebSocket.

## Envelope de evento

Todos os eventos seguem o mesmo envelope:

```json
{
  "event_id": "evt_01HX...",
  "type": "PlayerStateChanged",
  "version": 1,
  "timestamp": "2026-01-25T23:20:12.123Z",
  "payload": {}
}
```

## Frequência

- Eventos de estado: sob demanda.
- Progresso: 250ms a 1000ms configurável.
- Audio level: 100ms a 500ms configurável.
- Buffer: 250ms a 1000ms configurável.

MVP recomendado:

- `ProgressChanged`: 500ms.
- `AudioHealthChanged`: 500ms.

## Eventos obrigatórios

### EngineStarted

```json
{
  "type": "EngineStarted",
  "payload": {
    "engine_id": "studio-a-main",
    "version": "0.1.0"
  }
}
```

### EngineStopping

```json
{
  "type": "EngineStopping",
  "payload": {
    "reason": "signal"
  }
}
```

### PlayerStateChanged

```json
{
  "type": "PlayerStateChanged",
  "payload": {
    "from": "IDLE",
    "to": "PLAYING",
    "mode": "AUTO"
  }
}
```

### NowPlayingChanged

```json
{
  "type": "NowPlayingChanged",
  "payload": {
    "queue_item_id": "qi_001",
    "asset_id": "asset_123",
    "path": "/library/music/musicA.mp3",
    "title": "Faixa A",
    "artist": "Artista A",
    "type": "musicas",
    "duration_ms": 240000
  }
}
```

### ProgressChanged

```json
{
  "type": "ProgressChanged",
  "payload": {
    "queue_item_id": "qi_001",
    "position_ms": 123000,
    "duration_ms": 240000,
    "percent": 51.25,
    "remaining_ms": 117000
  }
}
```

### AudioHealthChanged

```json
{
  "type": "AudioHealthChanged",
  "payload": {
    "level_dbfs": -14.2,
    "peak_dbfs": -3.1,
    "silence": false,
    "silence_duration_ms": 0,
    "buffer_pct": 82,
    "underrun_count": 0
  }
}
```

### QueueChanged

```json
{
  "type": "QueueChanged",
  "payload": {
    "size": 7,
    "reason": "enqueue",
    "items": [
      {
        "queue_item_id": "qi_002",
        "asset_id": "asset_456",
        "title": "Faixa B",
        "type": "musicas",
        "duration_ms": 220000
      }
    ]
  }
}
```

### CommandAccepted

```json
{
  "type": "CommandAccepted",
  "payload": {
    "command_id": "cmd_001",
    "command": "SKIP",
    "reason": "operator"
  }
}
```

### CommandRejected

```json
{
  "type": "CommandRejected",
  "payload": {
    "command_id": "cmd_001",
    "command": "SKIP",
    "reason": "cannot skip mandatory item"
  }
}
```

### ItemStarted

```json
{
  "type": "ItemStarted",
  "payload": {
    "queue_item_id": "qi_001",
    "asset_id": "asset_123"
  }
}
```

### ItemFinished

```json
{
  "type": "ItemFinished",
  "payload": {
    "queue_item_id": "qi_001",
    "asset_id": "asset_123",
    "result": "PLAYED",
    "duration_played_ms": 240000
  }
}
```

### CrossfadeStarted

```json
{
  "type": "CrossfadeStarted",
  "payload": {
    "from_queue_item_id": "qi_001",
    "to_queue_item_id": "qi_002",
    "duration_ms": 8000
  }
}
```

### PanicEntered

```json
{
  "type": "PanicEntered",
  "payload": {
    "reason": "operator triggered",
    "bed_asset_id": "panic_bed"
  }
}
```

### PanicExited

```json
{
  "type": "PanicExited",
  "payload": {
    "reason": "operator reset"
  }
}

```

### AlertRaised

```json
{
  "type": "AlertRaised",
  "payload": {
    "alert_id": "alert_001",
    "severity": "WARNING",
    "source": "audio_health",
    "message": "silence detected for 2000ms"
  }
}
```

### AlertCleared

```json
{
  "type": "AlertCleared",
  "payload": {
    "alert_id": "alert_001"
  }
}
```

## Eventos de Volume

Publicados imediatamente após cada mudança de volume via `PUT /v1/playback/volume` ou `PUT /v1/preview/volume`.

### VolumeChanged

```json
{
  "type": "VolumeChanged",
  "payload": {
    "level": 0.8
  }
}
```

| Campo | Tipo | Descrição |
|---|---|---|
| `level` | float | Novo nível de volume da fila principal (`0.0–1.0`) |

**Prioridade:** alta — nunca descartado sob backpressure.

### PreviewVolumeChanged

```json
{
  "type": "PreviewVolumeChanged",
  "payload": {
    "level": 0.6
  }
}
```

| Campo | Tipo | Descrição |
|---|---|---|
| `level` | float | Novo nível de volume do player de preview CUE (`0.0–1.0`) |

**Prioridade:** alta — nunca descartado sob backpressure.

---

## Eventos de Scheduler

Publicados pelo módulo `internal/scheduler` quando entradas da grade horária são avaliadas.

### ScheduleEntryFired

Publicado quando uma entrada dispara com sucesso e os comandos de playback foram enviados ao Command Bus.

Exemplo — item único:

```json
{
  "type": "ScheduleEntryFired",
  "payload": {
    "entry_id": "sched_01JZ...",
    "entry_name": "Noticiário das 10h",
    "trigger_mode": "CROSSFADE",
    "asset_id": "asset_noticiao_10h",
    "title": "Noticiário das 10h",
    "one_shot": false
  }
}
```

Exemplo — bloco comercial:

```json
{
  "type": "ScheduleEntryFired",
  "payload": {
    "entry_id": "sched_01JZ...",
    "entry_name": "Bloco Comercial 10h30",
    "trigger_mode": "AFTER_CURRENT",
    "break_title": "Bloco das 10h30",
    "spot_count": 2,
    "one_shot": false
  }
}
```

| Campo | Tipo | Descrição |
|---|---|---|
| `entry_id` | string | ID da entrada do scheduler |
| `entry_name` | string | Nome legível da entrada |
| `trigger_mode` | string | `INTERRUPT` \| `AFTER_CURRENT` \| `CROSSFADE` \| `SKIP_IF_BUSY` |
| `asset_id` | string | ID do ativo agendado — presente apenas em entradas de **item único** |
| `title` | string | Título do item — presente apenas em entradas de **item único** |
| `break_title` | string | Título do bloco — presente apenas em entradas de **bloco comercial** |
| `spot_count` | int | Número de spots no bloco — presente apenas em entradas de **bloco comercial** |
| `one_shot` | bool | `true` se a entrada é one-shot (`fire_at`) e foi auto-desabilitada após disparar |

### ScheduleEntryMissed

Publicado quando uma entrada deveria ter disparado mas o estado do engine impediu a execução.

**Causas possíveis:**
- Engine em estado `PANIC` (qualquer modo de disparo)
- `SKIP_IF_BUSY`: engine em estado `PLAYING` ou `PAUSED`
- Entrada `FireAt` avaliada com atraso superior a `missed_threshold_ms`

```json
{
  "type": "ScheduleEntryMissed",
  "payload": {
    "entry_id": "sched_01JZ...",
    "entry_name": "Jingle das 9h",
    "trigger_mode": "SKIP_IF_BUSY",
    "reason": "engine is busy (state=PLAYING)"
  }
}
```

| Campo | Tipo | Descrição |
|---|---|---|
| `entry_id` | string | ID da entrada |
| `entry_name` | string | Nome legível |
| `trigger_mode` | string | Modo de disparo da entrada |
| `reason` | string | Motivo textual do miss |

### ScheduleEntryAdded

Publicado quando uma nova entrada é registrada via `POST /v1/schedule`.

```json
{
  "type": "ScheduleEntryAdded",
  "payload": {
    "entry_id": "sched_01JZ...",
    "name": "Noticiário das 10h",
    "cron_expr": "0 10 * * *",
    "one_shot": false
  }
}
```

### ScheduleEntryRemoved

Publicado quando uma entrada é removida via `DELETE /v1/schedule/{id}`.

```json
{
  "type": "ScheduleEntryRemoved",
  "payload": {
    "entry_id": "sched_01JZ..."
  }
}
```

### ScheduleEntryUpdated

Publicado quando uma entrada é habilitada (`POST /v1/schedule/{id}/enable`), desabilitada (`POST /v1/schedule/{id}/disable`) ou atualizada (`PUT /v1/schedule/{id}`).

```json
{
  "type": "ScheduleEntryUpdated",
  "payload": {
    "entry_id": "sched_01JZ...",
    "enabled": false
  }
}
```

**Prioridade de backpressure:** `ScheduleEntryFired` e `ScheduleEntryMissed` são eventos de baixa prioridade (podem ser descartados sob carga). `ScheduleEntryAdded`, `ScheduleEntryRemoved` e `ScheduleEntryUpdated` são discretos e raramente publicados, portanto não geram pressão significativa.

---

## StateSnapshot — snapshot inicial

Ao conectar, o hub envia imediatamente um evento do tipo `StateSnapshot` com o estado completo do engine, incluindo:

- estado do player, modo, now_playing, fila, audio health
- `main_volume` — volume atual da fila principal (`0.0–1.0`)
- `preview_volume` — volume atual do player de preview CUE (`0.0–1.0`)

A UI usa esse snapshot para inicializar os sliders de volume sem uma chamada REST adicional. Eventos subsequentes `VolumeChanged` e `PreviewVolumeChanged` mantêm os sliders sincronizados em tempo real.

## Reconnect

A UI deve reconectar automaticamente se o WebSocket cair.

Ao reconectar:

1. Abrir WebSocket.
2. Receber `StateSnapshot` — o hub envia automaticamente ao conectar.
3. Reconstruir tela com snapshot (incluindo volumes).
4. Continuar recebendo eventos.

## Backpressure

Se a UI não consumir eventos rapidamente, o Engine deve:

- descartar eventos de alta frequência antigos (`ProgressChanged`, `AudioHealthChanged`);
- nunca bloquear o pipeline de áudio;
- manter eventos críticos (`PanicEntered`, `CommandRejected`, `AlertRaised`).
