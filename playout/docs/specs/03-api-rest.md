# 03 — API REST

## Princípios

- REST é usado para comandos e consultas pontuais.
- WebSocket é usado para eventos em tempo real.
- A API deve ser local por padrão: `127.0.0.1`.
- Todos os endpoints devem ser versionados com `/v1`.
- Toda resposta de comando deve retornar ACK ou REJECT de forma explícita.

## Content-Type

Todas as requisições com body usam:

```http
Content-Type: application/json
```

Todas as respostas JSON usam:

```http
Content-Type: application/json
```

## Envelope de resposta

### Sucesso

```json
{
  "ok": true,
  "command_id": "cmd_01HX...",
  "accepted": true,
  "message": "command accepted"
}
```

### Rejeição operacional

```json
{
  "ok": true,
  "command_id": "cmd_01HX...",
  "accepted": false,
  "reason": "cannot skip mandatory item"
}
```

### Erro HTTP

```json
{
  "ok": false,
  "error": "invalid_payload",
  "message": "field path is required"
}
```

## Endpoints básicos

### GET /v1/health

Verifica se o processo está vivo.

Resposta:

```json
{
  "status": "ok",
  "engine": "running",
  "audio_output": "ready"
}
```

### GET /v1/status

Retorna snapshot atual.

Resposta:

```json
{
  "engine_id": "studio-a-main",
  "state": "PLAYING",
  "mode": "AUTO",
  "panic": false,
  "now_playing": {
    "queue_item_id": "qi_001",
    "asset_id": "asset_123",
    "path": "/library/music/musicA.mp3",
    "title": "Faixa A",
    "artist": "Artista A",
    "type": "musicas",
    "duration_ms": 240000,
    "position_ms": 123000,
    "percent": 51.25,
    "transition": {
      "type": "CROSSFADE",
      "duration_ms": 8000
    }
  },
  "queue": {
    "size": 7,
    "next_item_id": "qi_002"
  },
  "audio_health": {
    "level_dbfs": -14.2,
    "peak_dbfs": -3.1,
    "silence": false,
    "buffer_pct": 82,
    "underrun_count": 0
  },
  "last_command": {
    "command": "SKIP",
    "status": "ACCEPTED",
    "at": "2026-01-25T23:20:12Z"
  }
}
```

## Queue

### POST /v1/queue/enqueue

Adiciona itens ao final da fila.

Request:

```json
{
  "items": [
    {
      "asset_id": "asset_123",
      "path": "/library/music/musicA.mp3",
      "type": "musicas",
      "title": "Faixa A",
      "artist": "Artista A",
      "duration_ms": 240000,
      "cue_in_ms": 0,
      "cue_out_ms": 240000,
      "transition": {
        "type": "CROSSFADE",
        "duration_ms": 8000
      },
      "mandatory": false,
      "metadata": {
        "album": "Demo Album"
      }
    }
  ]
}
```

Response:

```json
{
  "ok": true,
  "accepted": true,
  "queue_size": 1
}
```

### POST /v1/queue/insert-next

Insere item após o item atual.

Request:

```json
{
  "item": {
    "asset_id": "asset_vinheta_01",
    "path": "/library/jingles/vinheta.mp3",
    "type": "jingles",
    "title": "Vinheta Padrão",
    "duration_ms": 12000,
    "transition": {
      "type": "CUT"
    }
  }
}
```

### POST /v1/queue/insert-after

Insere item após um item específico.

Request:

```json
{
  "after_queue_item_id": "qi_002",
  "item": {
    "asset_id": "asset_999",
    "path": "/library/music/musicC.mp3",
    "type": "musicas",
    "title": "Faixa C",
    "duration_ms": 230000
  }
}
```

### POST /v1/queue/clear

Limpa fila pendente. Não remove item em execução.

Request:

```json
{
  "preserve_current": true
}
```

### GET /v1/queue

Retorna fila atual.

Resposta:

```json
{
  "items": [
    {
      "queue_item_id": "qi_001",
      "asset_id": "asset_123",
      "title": "Faixa A",
      "type": "musicas",
      "status": "PLAYING"
    },
    {
      "queue_item_id": "qi_002",
      "asset_id": "asset_456",
      "title": "Faixa B",
      "type": "musicas",
      "status": "QUEUED"
    }
  ]
}
```

## Playback commands

### POST /v1/playback/play

Inicia reprodução da fila.

Request:

```json
{
  "reason": "operator"
}
```

### POST /v1/playback/pause

Pausa execução.

Request:

```json
{
  "reason": "operator"
}
```

### POST /v1/playback/resume

Retoma execução.

Request:

```json
{
  "reason": "operator"
}
```

### POST /v1/playback/stop

Para execução.

Request:

```json
{
  "clear_queue": false,
  "fade_ms": 300,
  "reason": "operator"
}
```

### POST /v1/playback/skip

Pula item atual.

Request:

```json
{
  "reason": "operator",
  "transition": {
    "type": "FADE_OUT",
    "duration_ms": 500
  }
}
```

Regras:

- Se o item atual for `mandatory=true`, o Engine pode rejeitar o comando.
- A rejeição deve retornar motivo explícito.

## Mode commands

### POST /v1/mode/auto

Retorna ao modo automático.

### POST /v1/mode/assist

Entra em Assist Mode.

Request:

```json
{
  "reason": "live program"
}
```

### POST /v1/mode/panic

Entra em Panic Mode.

Request:

```json
{
  "reason": "operator triggered",
  "bed": {
    "asset_id": "panic_bed",
    "path": "/library/beds/panic-bed.mp3"
  }
}
```

## Hot buttons

### POST /v1/hot-buttons/trigger

Dispara áudio instantâneo.

Request:

```json
{
  "button_id": "hb_001",
  "asset": {
    "asset_id": "vinheta_001",
    "path": "/library/jingles/vinheta.mp3",
    "type": "jingles",
    "title": "Vinheta"
  },
  "play_mode": "OVERLAY",
  "duck_main": true,
  "duck_gain_db": -8,
  "reason": "operator"
}
```

Modos:

- `OVERLAY`: toca sobre o programa.
- `INTERRUPT`: corta o programa.
- `AFTER_CURRENT`: insere após item atual.

## Admin

### GET /v1/build

Retorna informações de build.

```json
{
  "version": "0.1.0",
  "commit": "abc123",
  "go_version": "go1.24",
  "os": "darwin",
  "arch": "arm64"
}
```
