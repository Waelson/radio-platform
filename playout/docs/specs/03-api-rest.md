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
  },
  "main_volume": 0.8,
  "preview_volume": 1.0
}
```

### GET /v1/devices

Lista todos os dispositivos de saída de áudio disponíveis no sistema. A resposta reflete o estado atual do hardware — nenhum cache é aplicado (`Cache-Control: no-store`).

Resposta:

```json
{
  "devices": [
    {
      "id":                   "AppleHDAEngineOutput:0,1",
      "name":                 "MacBook Pro Speakers",
      "driver":               "coreaudio",
      "host_api":             "CoreAudio",
      "is_default":           true,
      "max_output_channels":  2,
      "default_sample_rate":  48000.0
    },
    {
      "id":                   "BlackHole 2ch",
      "name":                 "BlackHole 2ch",
      "driver":               "coreaudio",
      "host_api":             "CoreAudio",
      "is_default":           false,
      "max_output_channels":  2,
      "default_sample_rate":  44100.0
    }
  ],
  "count": 2
}
```

Exemplo (Windows / WASAPI):

```json
{
  "devices": [
    {
      "id":                   "{0.0.0.00000000}.{1a2b3c4d-5e6f-7890-abcd-ef1234567890}",
      "name":                 "Speakers (Realtek Audio)",
      "driver":               "wasapi",
      "host_api":             "WASAPI",
      "is_default":           true,
      "max_output_channels":  2,
      "default_sample_rate":  48000.0
    }
  ],
  "count": 1
}
```

Exemplo (Linux / PortAudio com ALSA):

```json
{
  "devices": [
    {
      "id":                   "Built-in Audio Analog Stereo",
      "name":                 "Built-in Audio Analog Stereo",
      "driver":               "portaudio",
      "host_api":             "ALSA",
      "is_default":           true,
      "max_output_channels":  2,
      "default_sample_rate":  48000.0
    }
  ],
  "count": 1
}
```

**Campos:**

| Campo | Tipo | Descrição |
|---|---|---|
| `id` | string | Identificador único (semântica varia por driver — ver abaixo) |
| `name` | string | Nome legível do dispositivo |
| `driver` | string | Driver em uso: `coreaudio`, `portaudio`, `null` ou `file` |
| `host_api` | string | Host API subjacente: `"CoreAudio"`, `"ALSA"`, `"PulseAudio"`, `"JACK"` etc. Omitido se vazio (`null`/`file`). |
| `is_default` | bool | `true` se for o output padrão do sistema |
| `max_output_channels` | int | Número máximo de canais de saída suportados |
| `default_sample_rate` | float | Taxa de amostragem padrão reportada pelo SO |

**Semântica do campo `id` por driver:**

| Driver | Valor de `id` | Estabilidade |
|---|---|---|
| `coreaudio` | UID do sistema (`kAudioDevicePropertyDeviceUID`) | Persiste mesmo se o nome do dispositivo mudar |
| `portaudio` | Igual ao `name` — PortAudio não expõe UID interno | Muda se o dispositivo for renomeado no SO |
| `null` / `file` | `"null"` / `"file"` (fixo) | Sempre estável |

Erro (driver não consegue enumerar dispositivos):

```json
{
  "ok": false,
  "error": "device_enumeration_failed",
  "message": "..."
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

## Volume

### GET /v1/playback/volume

Retorna o volume atual da fila principal.

Resposta:

```json
{ "level": 0.8 }
```

### PUT /v1/playback/volume

Ajusta o volume da fila principal em runtime. A alteração é instantânea (software gain aplicado antes de cada `output.Write()`). O novo valor é persistido em `~/.radiocore/preferences.json` e sobrevive a reinicializações.

Request:

```json
{ "level": 0.8 }
```

- `level`: `float`, intervalo `[0.0, 1.0]`. `1.0` = sem atenuação; `0.0` = mudo.

Resposta (aceito):

```json
{
  "ok": true,
  "command_id": "cmd_01HX...",
  "accepted": true
}
```

Resposta (nível inválido):

```json
{
  "ok": false,
  "error": "invalid_level",
  "message": "level must be between 0.0 and 1.0"
}
```

Publica o evento `VolumeChanged` no Event Bus após aplicar a mudança.

### GET /v1/preview/volume

Retorna o volume atual do player de preview (CUE).

Resposta:

```json
{ "level": 1.0 }
```

### PUT /v1/preview/volume

Ajusta o volume do player de preview (CUE) em runtime. O valor é encaminhado ao subprocess CUE via IPC (`{"cmd":"set_volume","volume":0.6}`) e persistido em `~/.radiocore/preferences.json`.

Request:

```json
{ "level": 0.6 }
```

Resposta (aceito):

```json
{
  "ok": true,
  "command_id": "cmd_01HX...",
  "accepted": true
}
```

Publica o evento `PreviewVolumeChanged` no Event Bus após aplicar a mudança.

> **Nota:** os endpoints de volume do preview retornam `503 Service Unavailable` quando `preview.enabled: false` no YAML.

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

## Schedule

Gerenciamento da grade horária. Requer que o scheduler esteja habilitado (`scheduler.enabled: true` no YAML).

### POST /v1/schedule

Cria uma nova entrada na grade.

Request (item único — cron recorrente):

```json
{
  "name": "Noticiário das 10h",
  "enabled": true,
  "cron_expr": "0 10 * * *",
  "trigger_mode": "CROSSFADE",
  "item": {
    "asset_id": "asset_noticiao_10h",
    "path": "/library/spots/noticiao-10h.mp3",
    "type": "spot",
    "title": "Noticiário das 10h",
    "duration_ms": 180000
  }
}
```

Request (item único — one-shot via `fire_at`):

```json
{
  "name": "Jingle especial",
  "enabled": true,
  "fire_at": "2026-07-06T14:30:00Z",
  "trigger_mode": "AFTER_CURRENT",
  "item": {
    "path": "/library/jingles/especial.mp3",
    "type": "jingles",
    "title": "Jingle Especial",
    "duration_ms": 30000
  }
}
```

Request (bloco comercial — cron recorrente):

```json
{
  "name": "Bloco Comercial 10h30",
  "enabled": true,
  "cron_expr": "30 10 * * *",
  "trigger_mode": "AFTER_CURRENT",
  "break": {
    "title": "Bloco das 10h30",
    "open": {
      "path": "/library/jingles/break-open.mp3",
      "type": "jingles",
      "title": "Abertura"
    },
    "spots": [
      { "path": "/library/spots/anunciante-a.mp3", "type": "spots", "title": "Anunciante A", "duration_ms": 30000 },
      { "path": "/library/spots/anunciante-b.mp3", "type": "spots", "title": "Anunciante B", "duration_ms": 30000 }
    ],
    "close": {
      "path": "/library/jingles/break-close.mp3",
      "type": "jingles",
      "title": "Encerramento"
    }
  }
}
```

Request (Hora Certa — todo início de hora):

```json
{
  "name": "Hora Certa",
  "enabled": true,
  "cron_expr": "0 * * * *",
  "trigger_mode": "INTERRUPT",
  "item": {
    "type": "HORA_CERTA",
    "title": "Hora Certa"
  }
}
```

**Campos do request:**

| Campo | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `name` | string | não | Nome legível da entrada |
| `enabled` | bool | não | Se `false`, a entrada é criada desabilitada (padrão: `false`) |
| `cron_expr` | string | exclusivo¹ | Expressão cron de 5 campos (minuto hora dia mês dia-semana) |
| `fire_at` | string (RFC3339) | exclusivo¹ | Data/hora para disparo único em formato RFC 3339 — ver detalhes abaixo |
| `trigger_mode` | string | não | `INTERRUPT` \| `AFTER_CURRENT` \| `CROSSFADE` \| `SKIP_IF_BUSY` (padrão: `AFTER_CURRENT`) |
| `item` | object | exclusivo² | Item de playback a ser inserido na frente da fila |
| `item.path` | string | sim³ | Path do arquivo de áudio |
| `item.type` | string | não | Tipo do item: `musicas`, `jingles`, `spots`, `HORA_CERTA`, `COMMERCIAL`, `BED`, `EFFECT`, `VOICE` |
| `break` | object | exclusivo² | Bloco comercial a ser inserido na frente da fila |
| `break.title` | string | não | Nome do bloco (aparece em logs e eventos) |
| `break.open` | object | não | Item de abertura (mesmo schema que `item`) |
| `break.spots` | array | **sim** (≥ 1) | Lista de spots; cada spot requer `path` |
| `break.close` | object | não | Item de encerramento (mesmo schema que `item`) |

¹ `cron_expr` e `fire_at` são mutuamente exclusivos — exatamente um deve ser informado.
² `item` e `break` são mutuamente exclusivos — exatamente um deve ser informado.
³ Obrigatório a menos que `item.type == "HORA_CERTA"`.

**`cron_expr` — expressão cron (5 campos):**

```
┌─── minuto      (0–59)
│ ┌─── hora       (0–23)
│ │ ┌─── dia do mês (1–31)
│ │ │ ┌─── mês       (1–12)
│ │ │ │ ┌─── dia da semana (0–7, domingo = 0 ou 7)
│ │ │ │ │
* * * * *
```

| Expressão | Significado |
|---|---|
| `0 10 * * *` | Todo dia às 10h00 |
| `30 7 * * 1-5` | Segunda a sexta às 07h30 |
| `0 */2 * * *` | A cada 2 horas |
| `0 6,12,18 * * *` | Às 06h, 12h e 18h todos os dias |

As expressões são avaliadas no timezone configurado em `scheduler.timezone` (padrão: timezone do sistema). Entradas cron **nunca são auto-desabilitadas** — disparam indefinidamente enquanto `enabled: true`.

**`fire_at` — disparo único (one-shot):**

O valor deve ser uma string **RFC 3339**. Após o disparo, a entrada é automaticamente desabilitada (`enabled: false`) e não dispara novamente.

| Formato | Exemplo | Observação |
|---|---|---|
| Com offset explícito | `2026-07-06T16:31:00-03:00` | Recomendado — sem ambiguidade de fuso |
| UTC (sufixo Z) | `2026-07-06T19:31:00Z` | Equivalente ao exemplo acima |
| Sem timezone | `2026-07-06T16:31:00` | Interpretado no `scheduler.timezone` configurado |

Anatomia do valor:

```
2026-07-06T16:31:00-03:00
│          │        │
│          │        └─ offset: -03:00 = Brasília (BRT)
│          └─ horário local: 16h31min00s
└─ data: 6 de julho de 2026
```

Se o engine estava parado quando o horário passou e reiniciar com atraso:
- Atraso < `missed_threshold_ms` → dispara normalmente
- Atraso ≥ `missed_threshold_ms` → marca como `MISSED`, não dispara (evita disparos obsoletos após queda)

**Modos de disparo (`trigger_mode`):**

| Modo | Comportamento |
|---|---|
| `INTERRUPT` | Interrompe o item atual e inicia imediatamente |
| `AFTER_CURRENT` | Insere como próximo da fila; aguarda o item atual terminar |
| `CROSSFADE` | Inicia com crossfade sobre o final do item atual |
| `SKIP_IF_BUSY` | Dispara apenas se o engine estiver IDLE; caso contrário, marca como MISSED |

Response (`201 Created`) — item único:

```json
{
  "ok": true,
  "entry": {
    "id": "sched_01JZ...",
    "name": "Noticiário das 10h",
    "enabled": true,
    "cron_expr": "0 10 * * *",
    "trigger_mode": "CROSSFADE",
    "item": {
      "path": "/library/spots/noticiao-10h.mp3",
      "type": "spot",
      "title": "Noticiário das 10h",
      "duration_ms": 180000
    },
    "created_at": "2026-07-06T12:00:00Z",
    "next_fire_at": "2026-07-07T10:00:00Z"
  }
}
```

Response (`201 Created`) — bloco comercial:

```json
{
  "ok": true,
  "entry": {
    "id": "sched_01JZ...",
    "name": "Bloco Comercial 10h30",
    "enabled": true,
    "cron_expr": "30 10 * * *",
    "trigger_mode": "AFTER_CURRENT",
    "break": {
      "title": "Bloco das 10h30",
      "open":  { "path": "/library/jingles/break-open.mp3", "type": "jingles", "title": "Abertura" },
      "spots": [
        { "path": "/library/spots/anunciante-a.mp3", "type": "spots", "title": "Anunciante A", "duration_ms": 30000 },
        { "path": "/library/spots/anunciante-b.mp3", "type": "spots", "title": "Anunciante B", "duration_ms": 30000 }
      ],
      "close": { "path": "/library/jingles/break-close.mp3", "type": "jingles", "title": "Encerramento" }
    },
    "created_at": "2026-07-06T12:00:00Z",
    "next_fire_at": "2026-07-07T10:30:00Z"
  }
}
```

---

### GET /v1/schedule

Lista todas as entradas da grade.

Response:

```json
{
  "entries": [
    {
      "id": "sched_01JZ...",
      "name": "Noticiário das 10h",
      "enabled": true,
      "cron_expr": "0 10 * * *",
      "trigger_mode": "CROSSFADE",
      "item": { "path": "/library/spots/noticiao-10h.mp3", "title": "Noticiário das 10h" },
      "created_at": "2026-07-06T12:00:00Z",
      "last_fired_at": "2026-07-06T10:00:00Z",
      "next_fire_at": "2026-07-07T10:00:00Z"
    }
  ],
  "count": 1
}
```

---

### GET /v1/schedule/{id}

Retorna uma entrada pelo ID.

Response: igual ao objeto `entry` do `POST /v1/schedule`.

Erro (não encontrado):

```json
{ "ok": false, "error": "not_found", "message": "schedule entry not found" }
```

---

### PUT /v1/schedule/{id}

Atualiza uma entrada existente. O body segue o mesmo schema do `POST /v1/schedule`.

Response: igual ao `POST` com o objeto `entry` atualizado.

---

### DELETE /v1/schedule/{id}

Remove uma entrada da grade.

Response:

```json
{ "ok": true }
```

---

### POST /v1/schedule/{id}/enable

Habilita uma entrada desabilitada.

Response:

```json
{ "ok": true, "entry_id": "sched_01JZ...", "enabled": true }
```

---

### POST /v1/schedule/{id}/disable

Desabilita uma entrada sem removê-la.

Response:

```json
{ "ok": true, "entry_id": "sched_01JZ...", "enabled": false }
```

---

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
