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

## Reconnect

A UI deve reconectar automaticamente se o WebSocket cair.

Ao reconectar:

1. Abrir WebSocket.
2. Chamar `GET /v1/status`.
3. Reconstruir tela com snapshot.
4. Continuar recebendo eventos.

## Backpressure

Se a UI não consumir eventos rapidamente, o Engine deve:

- descartar eventos de alta frequência antigos (`ProgressChanged`, `AudioHealthChanged`);
- nunca bloquear o pipeline de áudio;
- manter eventos críticos (`PanicEntered`, `CommandRejected`, `AlertRaised`).
