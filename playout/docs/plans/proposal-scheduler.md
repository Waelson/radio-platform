# Proposta: Scheduler — Programação Horária

## 1. Visão geral

O Scheduler é um módulo interno do Playout Engine responsável por disparar itens de áudio em horários pré-determinados, independentemente da fila de reprodução corrente. Ele resolve o principal problema operacional de emissoras locais: garantir que spots comerciais, jingles, noticiários e vinhetas de identificação toquem exatamente no horário contratado ou definido pelo programador musical — sem intervenção humana.

---

## 2. Perspectiva operacional

### Como o locutor e o operador enxergam o scheduler

```
┌─────────────────────────────────────────────────────────────┐
│  Player / UI                                                │
│                                                             │
│  [Grade Horária]                                            │
│  ├─ 10:00  Noticiário         CROSSFADE    ativo  ✓         │
│  ├─ 10:30  Spot Banco X       AFTER_CURRENT ativo ✓         │
│  ├─ 12:00  Vinheta Meio-dia   INTERRUPT    ativo  ✓         │
│  └─ 20:00  Transmissão Ao Vivo INTERRUPT   one-shot ✓       │
└─────────────────────────────────────────────────────────────┘
```

**Cenários típicos de operação:**

| Cenário | Cron / Horário | Modo de disparo |
|---|---|---|
| Noticiário diário às 10h | `0 10 * * *` | `CROSSFADE` — encavalha suavemente na música atual |
| Jingle a cada 30 min | `0,30 * * * *` | `AFTER_CURRENT` — aguarda a música terminar |
| Spot comercial (seg-sex, 9h/12h/18h) | `0 9,12,18 * * 1-5` | `AFTER_CURRENT` |
| Vinheta de ID a cada hora cheia | `0 * * * *` | `INTERRUPT` — interrompe imediatamente |
| Evento único — live amanhã às 20h | fire_at: `2026-07-07T20:00:00` | `INTERRUPT` |

**O que acontece quando um entry dispara:**

- **`INTERRUPT`**: O item agendado é inserido imediatamente à frente da fila e um skip (com transição configurada) é enviado. A música atual para ou faz fade; o agendado toca; ao final, a fila normal retoma.
- **`AFTER_CURRENT`**: O item é inserido como próximo. A música atual termina normalmente; o agendado toca em seguida; a fila continua.
- **`CROSSFADE`**: Igual ao `INTERRUPT`, mas o transition type do item é forçado como `CROSSFADE`. A música atual faz crossfade para o item agendado.
- **`SKIP_IF_BUSY`**: Só dispara se o engine estiver `IDLE`. Se estiver tocando, registra `MISSED` e não interfere.

**O que o locutor vê no ar:**
- Nos modos `INTERRUPT` e `CROSSFADE`, a troca é imperceptível ou suave.
- Nos modos `AFTER_CURRENT`, a música atual termina naturalmente — o ouvinte não percebe nada.
- Em todos os casos, ao final do item agendado, a fila principal retoma de onde estava.

---

## 3. Modelo de dados

```go
// Entry representa um agendamento. Pode ser recorrente (CronExpr) ou único (FireAt).
type Entry struct {
    ID          string      // ULID gerado no ADD
    Name        string      // "Noticiário das 10h"
    Enabled     bool        // false = pausado sem remover
    CronExpr    string      // expressão cron 5 campos (minuto precision)
                            // "" quando FireAt for usado
    FireAt      *time.Time  // horário único de disparo (one-shot)
    TriggerMode TriggerMode // INTERRUPT | AFTER_CURRENT | CROSSFADE | SKIP_IF_BUSY
    Item        ScheduledItem
    CreatedAt   time.Time
    LastFiredAt *time.Time  // preenchido após cada disparo
    NextFireAt  *time.Time  // calculado pelo scheduler (read-only na API)
}

// ScheduledItem é o áudio a ser reproduzido quando a entrada disparar.
// Tem os mesmos campos de um QueueItem — o scheduler converte para QueueItem antes de enviar.
type ScheduledItem struct {
    AssetID    string
    Path       string
    Type       string            // SPOT | JINGLE | BED | VOICE | MUSIC | etc.
    Title      string
    Artist     string
    DurationMS int64
    CueInMS    int64
    CueOutMS   int64
    Transition TransitionSpec    // usado em INTERRUPT e CROSSFADE
    Mandatory  bool
    Metadata   map[string]string
}

type TriggerMode string

const (
    TriggerInterrupt    TriggerMode = "INTERRUPT"
    TriggerAfterCurrent TriggerMode = "AFTER_CURRENT"
    TriggerCrossfade    TriggerMode = "CROSSFADE"
    TriggerSkipIfBusy   TriggerMode = "SKIP_IF_BUSY"
)
```

---

## 4. Arquitetura

### 4.1 Posição no sistema

```
┌──────────────────────────────────────────────────────────────────┐
│  API Server  (GET/POST /v1/schedule/*)                           │
└─────────────────────┬────────────────────────────────────────────┘
                      │ CmdScheduleAdd / CmdScheduleRemove / ...
                      ▼
              ┌───────────────┐
              │  Command Bus  │
              └───────┬───────┘
                      │
                      ▼
              ┌───────────────┐         ┌──────────────────────┐
              │  Dispatcher   │────────▶│  Scheduler Manager   │
              └───────────────┘         │  (internal/scheduler)│
                                        └──────────┬───────────┘
                                                   │  tick goroutine
                                                   │  (a cada 1s)
                                                   │
                               quando entry dispara:│
                                                   ▼
                                        ┌───────────────────┐
                                        │   Command Bus     │
                                        │ CmdInsertNext     │
                                        │ CmdSkip (se INTERRUPT/CROSSFADE)
                                        │ CmdPlay (se IDLE) │
                                        └────────┬──────────┘
                                                 │
                                                 ▼
                                        ┌───────────────────┐
                                        │ Playback Manager  │
                                        └───────────────────┘

Eventos (Event Bus → WebSocket):
  ScheduleEntryFired
  ScheduleEntryMissed
  ScheduleEntryAdded
  ScheduleEntryRemoved
  ScheduleEntryUpdated
```

### 4.2 Pacote `internal/scheduler`

```
internal/scheduler/
  entry.go        — structs Entry, ScheduledItem, TriggerMode
  manager.go      — SchedulerManager: Run, Add, Remove, Enable, Disable, List
  store.go        — FileStore: Load, Save (JSON em disco)
  cron.go         — wrapper sobre robfig/cron/v3 para calcular NextFireAt
  fire.go         — lógica de disparo: converte Entry → QueueItem → comandos
```

### 4.3 Interface do Manager

```go
type Manager interface {
    Run(ctx context.Context)
    Add(e Entry) (Entry, error)
    Update(id string, e Entry) (Entry, error)
    Remove(id string) error
    Enable(id string) error
    Disable(id string) error
    List() []Entry
    Get(id string) (Entry, error)
}
```

### 4.4 Dependências do Scheduler Manager

| Dependência | Por quê |
|---|---|
| `commands.Bus` | Enviar CmdInsertNext, CmdSkip, CmdPlay quando entry dispara |
| `state.Manager` | Ler estado atual do engine (IDLE, PLAYING, PANIC) para decidir o comportamento |
| `events.Bus` | Publicar ScheduleEntryFired, ScheduleEntryMissed etc. |
| `robfig/cron/v3` | Avaliar expressões cron e calcular NextFireAt |
| `FileStore` | Persistir schedule em `~/RadioFlow/schedule.json` |

O Scheduler Manager **não importa** `playback`, `audio` ou `output` — respeita a regra de que todo controle passa pelo Command Bus.

### 4.5 Lógica de disparo (fire.go)

```
quando entry dispara:
  1. Ler estado atual via stateMgr.Snapshot()

  2. Se estado == PANIC:
       publicar ScheduleEntryMissed
       retornar

  3. Converter Entry.Item → QueueItem (gerar novo queue_item_id)

  4. Enviar CmdInsertNext com o QueueItem

  5. Se TriggerMode == INTERRUPT ou CROSSFADE:
       Se estado == PLAYING ou PAUSED:
           Enviar CmdSkip
       Se estado == IDLE:
           Enviar CmdPlay

  6. Se TriggerMode == AFTER_CURRENT:
       Se estado == IDLE:
           Enviar CmdPlay
       (se PLAYING: item já está na posição correta — queue avança naturalmente)

  7. Se TriggerMode == SKIP_IF_BUSY:
       Se estado != IDLE:
           publicar ScheduleEntryMissed
           retornar
       Caso contrário: Enviar CmdPlay

  8. Atualizar entry.LastFiredAt
  9. Se one-shot (FireAt != nil): desativar entry automaticamente
  10. Publicar ScheduleEntryFired
```

---

## 5. API REST

### `POST /v1/schedule`

Adiciona uma entrada de agendamento.

**Request:**
```json
{
  "name": "Noticiário das 10h",
  "enabled": true,
  "cron_expr": "0 10 * * *",
  "trigger_mode": "CROSSFADE",
  "item": {
    "asset_id": "asset_noticiao_10h",
    "path": "/media/spots/noticiao.mp3",
    "type": "SPOT",
    "title": "Noticiário das 10h",
    "duration_ms": 120000,
    "cue_in_ms": 0,
    "cue_out_ms": 120000,
    "transition": { "type": "CROSSFADE", "duration_ms": 3000 }
  }
}
```

**Response:**
```json
{
  "ok": true,
  "entry": {
    "id": "01JZ...",
    "name": "Noticiário das 10h",
    "enabled": true,
    "cron_expr": "0 10 * * *",
    "trigger_mode": "CROSSFADE",
    "next_fire_at": "2026-07-07T10:00:00-03:00",
    "last_fired_at": null,
    "created_at": "2026-07-06T13:00:00-03:00",
    "item": { "..." }
  }
}
```

---

### `GET /v1/schedule`

Lista todas as entradas com `next_fire_at` calculado.

**Response:**
```json
{
  "entries": [
    {
      "id": "01JZ...",
      "name": "Noticiário das 10h",
      "enabled": true,
      "cron_expr": "0 10 * * *",
      "trigger_mode": "CROSSFADE",
      "next_fire_at": "2026-07-07T10:00:00-03:00",
      "last_fired_at": "2026-07-06T10:00:01-03:00"
    }
  ],
  "count": 1
}
```

---

### `GET /v1/schedule/{id}`

Retorna uma entrada específica com todos os campos.

---

### `PUT /v1/schedule/{id}`

Atualiza uma entrada existente (mesma estrutura do POST).

---

### `DELETE /v1/schedule/{id}`

Remove uma entrada.

---

### `POST /v1/schedule/{id}/enable`

Habilita uma entrada pausada.

---

### `POST /v1/schedule/{id}/disable`

Desabilita uma entrada sem removê-la.

---

## 6. Eventos WebSocket

### `ScheduleEntryFired`

```json
{
  "type": "ScheduleEntryFired",
  "payload": {
    "entry_id": "01JZ...",
    "name": "Noticiário das 10h",
    "trigger_mode": "CROSSFADE",
    "fired_at": "2026-07-07T10:00:00-03:00",
    "next_fire_at": "2026-07-08T10:00:00-03:00"
  }
}
```

### `ScheduleEntryMissed`

```json
{
  "type": "ScheduleEntryMissed",
  "payload": {
    "entry_id": "01JZ...",
    "name": "Noticiário das 10h",
    "reason": "engine_in_panic",
    "missed_at": "2026-07-07T10:00:00-03:00"
  }
}
```

### `ScheduleEntryAdded` / `ScheduleEntryRemoved` / `ScheduleEntryUpdated`

```json
{
  "type": "ScheduleEntryAdded",
  "payload": {
    "entry_id": "01JZ...",
    "name": "Noticiário das 10h"
  }
}
```

---

## 7. Persistência

```
~/RadioFlow/
  schedule.json     — lista serializada de Entry
  queue.json        — (existente) fila de reprodução
  playout-engine.yaml
```

`schedule.json` é carregado na inicialização do engine e salvo atomicamente a cada modificação (write to `.tmp` + rename). Formato:

```json
{
  "version": 1,
  "entries": [ ... ]
}
```

---

## 8. Configuração (playout-engine.yaml)

```yaml
scheduler:
  enabled: true

  # Timezone para avaliação das expressões cron.
  # Padrão: timezone do sistema operacional.
  # Exemplos: "America/Sao_Paulo", "America/Manaus", "UTC"
  timezone: "America/Sao_Paulo"

  # Caminho do arquivo de persistência do schedule.
  # Vazio = ~/RadioFlow/schedule.json
  store_path: ""

  # Tolerância de atraso: se um entry deveria ter disparado há mais que
  # este tempo (ex: engine foi reiniciado), ele é marcado como MISSED
  # em vez de disparar com atraso.
  missed_threshold_ms: 5000
```

---

## 9. Expressões cron — referência

O scheduler usa expressões cron de 5 campos (precisão de minuto) via `robfig/cron/v3`:

```
┌──────── minuto     (0-59)
│ ┌────── hora       (0-23)
│ │ ┌──── dia do mês (1-31)
│ │ │ ┌── mês        (1-12)
│ │ │ │ ┌ dia da sem. (0-6, 0=Dom)
│ │ │ │ │
* * * * *
```

| Expressão | Significado |
|---|---|
| `0 10 * * *` | Todo dia às 10:00 |
| `0,30 * * * *` | A cada 30 minutos (:00 e :30) |
| `0 9,12,18 * * 1-5` | Seg-Sex às 9h, 12h e 18h |
| `0 * * * *` | A cada hora cheia |
| `0 8 * * 1` | Toda segunda-feira às 8h |

Strings especiais suportadas: `@hourly`, `@daily`, `@weekly`, `@midnight`.

---

## 10. Relação com Hora Certa

A Hora Certa existente é um mecanismo especializado ativado por um tipo específico de `QueueItem` (`HORA_CERTA`). Ela **permanece inalterada** — o scheduler não a substitui.

No futuro, a Hora Certa poderia ser reimplementada como um entry recorrente de scheduler (`0 * * * *`, trigger `INTERRUPT`, item do tipo `HORA_CERTA`), mas por ora os dois sistemas coexistem e são independentes.

---

## 11. Dependência externa

| Biblioteca | Versão | Propósito |
|---|---|---|
| `github.com/robfig/cron/v3` | v3.0.1 | Avaliação de expressões cron e cálculo de NextFireAt |

Licença: MIT. Zero dependências transitivas.

---

## 12. Arquivos a criar/modificar

| Arquivo | Ação |
|---|---|
| `internal/scheduler/entry.go` | CRIAR — structs Entry, ScheduledItem, TriggerMode |
| `internal/scheduler/manager.go` | CRIAR — SchedulerManager, Run, Add, Remove, Enable, Disable, List |
| `internal/scheduler/store.go` | CRIAR — FileStore JSON (load/save atômico) |
| `internal/scheduler/fire.go` | CRIAR — lógica de disparo: Entry → comandos |
| `internal/scheduler/manager_test.go` | CRIAR — testes de agendamento, disparo, missed |
| `internal/commands/commands.go` | MODIFICAR — adicionar CmdScheduleAdd, CmdScheduleRemove, CmdScheduleEnable, CmdScheduleDisable |
| `internal/events/events.go` | MODIFICAR — adicionar ScheduleEntryFired, ScheduleEntryMissed, ScheduleEntryAdded, ScheduleEntryRemoved, ScheduleEntryUpdated |
| `internal/api/handlers/schedule.go` | CRIAR — handlers REST do scheduler |
| `internal/api/handlers/schedule_test.go` | CRIAR — testes dos handlers |
| `internal/api/server.go` | MODIFICAR — ScheduleDeps + rotas /v1/schedule/* |
| `internal/config/config.go` | MODIFICAR — seção `scheduler` (enabled, timezone, store_path, missed_threshold_ms) |
| `cmd/playout-engine/engine/firstrun.go` | MODIFICAR — adicionar seção `scheduler` no YAML padrão |
| `cmd/playout-engine/main.go` | MODIFICAR — instanciar SchedulerManager, injetar no Dispatcher e no API server |
| `docs/specs/03-api-rest.md` | MODIFICAR — endpoints /v1/schedule/* |
| `docs/specs/04-events-websocket.md` | MODIFICAR — eventos de scheduler |
| `README.md` | MODIFICAR — Feature 18: Scheduler |

---

## 13. Fases de implementação

### Fase A — Core do Scheduler (sem persistência, sem API)
- `entry.go`, `manager.go`, `fire.go`
- Tick goroutine com timer de 1s
- Suporte a `FireAt` (one-shot) e `CronExpr` via `robfig/cron/v3`
- Integração em `main.go`
- Testes unitários com clock mockado

### Fase B — Persistência
- `store.go` — FileStore JSON
- Load on startup, save atômico a cada mudança
- Seção `scheduler` na config + `firstrun.go`

### Fase C — API REST
- Handlers: Add, List, Get, Update, Delete, Enable, Disable
- `ScheduleDeps` injetado no API server
- Testes dos handlers

### Fase D — Eventos e documentação
- Eventos WebSocket: Fired, Missed, Added, Removed, Updated
- Atualizar `docs/specs/`, `README.md`

---

## 14. Verificação

```bash
# Adicionar entry via API
curl -s -X POST http://localhost:8080/v1/schedule \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Jingle test",
    "enabled": true,
    "cron_expr": "* * * * *",
    "trigger_mode": "AFTER_CURRENT",
    "item": {
      "path": "/tmp/jingle.mp3",
      "type": "JINGLE",
      "title": "Jingle Teste"
    }
  }' | jq .

# Verificar próximo disparo
curl -s http://localhost:8080/v1/schedule | jq '.entries[].next_fire_at'

# Escutar eventos de disparo no WebSocket
wscat -c ws://localhost:8080/v1/events | grep Schedule
```
