# Plano: Suporte a Bloco Comercial e Hora Certa no Scheduler

## Contexto

O scheduler atual (Fases A–D) suporta apenas **itens únicos** — o campo `Entry.Item` é um
`commands.QueueItemInput` que, ao disparar, gera um `CmdInsertNext` simples.

Há dois tipos especiais que precisam de tratamento nativo:

### HORA_CERTA — já funciona (quase)

O tipo `HORA_CERTA` **já funciona** na prática:
- A validação em `handlers/schedule.go` já permite `path` vazio quando `type == "HORA_CERTA"`
- O `fire.go` chama `CmdInsertNext` com o item, que vai para a fila
- O playback manager detecta `item.Type == "HORA_CERTA"` e chama `openHoraCerta()` na hora de tocar

**Gap:** falta apenas documentação clara e testes explícitos que confirmem esse fluxo ponta a ponta.

### Bloco Comercial — não funciona

O `Entry.Item` é um único `QueueItemInput`. O bloco comercial exige `BreakItemInput`
(`Open + Spots + Close`), que é expandido em múltiplos `QueueItem`s com `BreakID` compartilhado.

O único comando existente para blocos é `CmdEnqueueBreak`, que **adiciona ao final da fila**.
Não existe `CmdInsertBreakNext` — por isso o scheduler não consegue inserir um bloco como
"próximo a tocar", que é o comportamento esperado ao disparar com `INTERRUPT`, `AFTER_CURRENT`
ou `CROSSFADE`.

---

## Diagnóstico por tipo de entrada

| Tipo | Situação atual | O que falta |
|---|---|---|
| Item único (`musicas`, `jingles` etc.) | Funciona | — |
| `HORA_CERTA` | Funciona — só falta doc/teste | Fase 9.A |
| Bloco comercial | Não funciona | Fases 9.B + 9.C + 9.D |

---

## Fase 9.A — Documentação e teste do HORA_CERTA no scheduler

**Objetivo:** confirmar o fluxo ponta a ponta e documentar o comportamento.

### 9.A.1 — Adicionar teste em `internal/scheduler/manager_test.go`

Criar caso de teste `TestSchedulerHoraCerta` que:
- Cria entry com `item.Type = "HORA_CERTA"` e `item.Path = ""`
- Confirma que `Add` aceita sem erro
- Confirma que o tick do cron dispara `CmdInsertNext` com o item correto
- Confirma que `ScheduleEntryFired` é emitido

### 9.A.2 — Atualizar `docs/specs/17-scheduler.md`

Adicionar seção "Hora Certa" com:
- Exemplo completo do JSON de criação
- Nota: `path` deve ser vazio — o engine resolve o arquivo na hora do disparo usando `hora_certa.hours_dir` e `hora_certa.minutes_dir`
- Nota: `cron_expr: "0 * * * *"` é a expressão recomendada (todo início de hora)

---

## Fase 9.B — Novo comando `CmdInsertBreakNext`

**Objetivo:** criar um comando que insere um bloco comercial **na frente da fila** (equivalente ao
`CmdInsertNext` mas para breaks), necessário para que o scheduler possa posicionar o bloco como
"próximo a tocar" sem enviá-lo para o final.

### 9.B.1 — MODIFICAR `internal/commands/types.go`

Adicionar nova constante e payload:

```go
CmdInsertBreakNext CommandType = "INSERT_BREAK_NEXT"

// InsertBreakNextPayload carries the payload for CmdInsertBreakNext.
// The break is expanded and inserted at the front of the pending queue,
// exactly like CmdInsertNext but for a full commercial break.
type InsertBreakNextPayload struct {
    Break   BreakItemInput
    BreakID string // optional: pre-computed by caller
}
```

### 9.B.2 — MODIFICAR `internal/queue/manager.go`

Adicionar handler:

```go
// HandleInsertBreakNext handles CmdInsertBreakNext: expands a BreakItemInput
// and inserts the resulting sub-items at the FRONT of the pending queue,
// preserving break metadata (BreakID, BreakSeq, BreakTotal, BreakRole).
func (m *Manager) HandleInsertBreakNext(_ context.Context, cmd commands.Command) error {
    p, ok := cmd.Payload.(commands.InsertBreakNextPayload)
    if !ok {
        return fmt.Errorf("insert-break-next: unexpected payload type %T", cmd.Payload)
    }
    if len(p.Break.Spots) == 0 {
        return fmt.Errorf("insert-break-next: spots list is empty")
    }
    // Expand break into flat items (same logic as HandleEnqueueBreak)
    // then call m.InsertNext for each item in reverse order so the
    // final order in the queue is Open → Spots → Close.
    ...
}
```

**Nota de implementação:** expandir o break usando a mesma lógica de `HandleEnqueueBreak`
(Open → CrossfadeTransition, Spots → Cut, Close → Cut), mas inserir os itens via `InsertNext`
em ordem reversa para preservar a sequência correta na fila.

### 9.B.3 — MODIFICAR `internal/dispatcher/dispatcher.go`

Registrar o novo comando nas tabelas de estados permitidos — mesmas regras do `CmdInsertNext`:
- Permitido em: `IDLE`, `PLAYING`, `PAUSED`, `ASSIST`
- Bloqueado em: `PANIC`, `STOPPING`, `ERROR`

Registrar o handler no dispatcher:

```go
d.Register(commands.CmdInsertBreakNext, queueMgr.HandleInsertBreakNext)
```

---

## Fase 9.C — Integração do bloco comercial no scheduler

### 9.C.1 — MODIFICAR `internal/scheduler/entry.go`

Adicionar campo opcional `Break` em `Entry`:

```go
type Entry struct {
    // ... campos existentes ...
    Item  commands.QueueItemInput  `json:"item"`
    Break *commands.BreakItemInput `json:"break,omitempty"` // mutuamente exclusivo com Item
}
```

**Regra de exclusividade:** ou `Item` é usado (item único / HORA_CERTA) ou `Break` é usado
(bloco comercial). Nunca os dois ao mesmo tempo.

### 9.C.2 — MODIFICAR `internal/scheduler/fire.go`

Substituir a função `insertNext` por um dispatcher interno que escolhe o comando correto:

```go
// dispatchInsert sends the appropriate insert command based on entry type.
func (m *Manager) dispatchInsert(e *Entry) {
    if e.Break != nil {
        m.cmdBus.TrySend(commands.New(commands.CmdInsertBreakNext, commands.InsertBreakNextPayload{
            Break: *e.Break,
        }))
    } else {
        m.cmdBus.TrySend(commands.New(commands.CmdInsertNext, commands.InsertNextPayload{
            Item: e.Item,
        }))
    }
}
```

Substituir todas as chamadas a `m.insertNext(e)` por `m.dispatchInsert(e)`.

Atualizar `publishFired` para incluir informação de break no evento:

```go
func (m *Manager) publishFired(e *Entry) {
    payload := events.ScheduleEntryFiredPayload{
        EntryID:     e.ID,
        EntryName:   e.Name,
        TriggerMode: string(e.TriggerMode),
        OneShot:     e.FireAt != nil,
    }
    if e.Break != nil {
        payload.BreakTitle = e.Break.Title
        payload.SpotCount  = len(e.Break.Spots)
    } else {
        payload.AssetID = e.Item.AssetID
        payload.Title   = e.Item.Title
    }
    m.evtBus.Publish(events.New(events.EvtScheduleEntryFired, payload))
}
```

### 9.C.3 — MODIFICAR `internal/events/types.go`

Adicionar campos no `ScheduleEntryFiredPayload`:

```go
type ScheduleEntryFiredPayload struct {
    EntryID     string `json:"entry_id"`
    EntryName   string `json:"entry_name"`
    TriggerMode string `json:"trigger_mode"`
    AssetID     string `json:"asset_id,omitempty"`   // item único
    Title       string `json:"title,omitempty"`       // item único ou break
    BreakTitle  string `json:"break_title,omitempty"` // bloco comercial
    SpotCount   int    `json:"spot_count,omitempty"`  // bloco comercial
    OneShot     bool   `json:"one_shot"`
}
```

### 9.C.4 — MODIFICAR `internal/api/handlers/schedule.go`

**Request DTO** — adicionar campo `Break`:

```go
type scheduleAddRequest struct {
    Name        string                    `json:"name"`
    Enabled     bool                      `json:"enabled"`
    CronExpr    string                    `json:"cron_expr"`
    FireAt      *time.Time                `json:"fire_at"`
    TriggerMode string                    `json:"trigger_mode"`
    Item        *queueItemInput           `json:"item,omitempty"`  // ponteiro — pode ser nil
    Break       *breakItemInput           `json:"break,omitempty"` // mutuamente exclusivo com Item
}

type breakItemInput struct {
    Title string          `json:"title"`
    Open  *queueItemInput `json:"open,omitempty"`
    Spots []queueItemInput `json:"spots"`
    Close *queueItemInput `json:"close,omitempty"`
}
```

**Validação** — adicionar regras em `validateScheduleRequest`:
- `item` e `break` são mutuamente exclusivos — exatamente um deve ser informado
- Se `break`: `spots` deve ter ≥ 1 elemento; cada spot precisa de `path`
- Se `item`: regras existentes (path obrigatório, exceto HORA_CERTA)

**`toScheduleEntry`** — preencher `entry.Break` quando presente.

**`toScheduleView`** e **`scheduleEntryView`** — incluir `Break` no DTO de resposta.

---

## Fase 9.D — Testes

### `internal/scheduler/manager_test.go`

| Teste | O que verifica |
|---|---|
| `TestSchedulerHoraCerta` | Entry com `type=HORA_CERTA`, path vazio → dispara `CmdInsertNext` correto |
| `TestSchedulerBreak_AfterCurrent` | Entry com Break → dispara `CmdInsertBreakNext` (não `CmdInsertNext`) |
| `TestSchedulerBreak_Interrupt` | Break com INTERRUPT → `CmdInsertBreakNext` + `CmdSkip` |
| `TestSchedulerBreak_Crossfade` | Break com CROSSFADE → `CmdInsertBreakNext` + `CmdSkip{CROSSFADE}` |
| `TestSchedulerBreak_SkipIfBusy_Missed` | Break com SKIP_IF_BUSY quando PLAYING → MISSED |
| `TestSchedulerBreak_Persist` | Entry com Break persiste e restaura corretamente via FileStore |
| `TestSchedulerBreak_ItemAndBreakMutuallyExclusive` | Add com `item` + `break` → erro de validação |

### `internal/queue/manager_test.go`

| Teste | O que verifica |
|---|---|
| `TestInsertBreakNext_Order` | Itens do break aparecem na ordem Open→Spot1→Spot2→Close na frente da fila |
| `TestInsertBreakNext_NoSpots` | Break sem spots retorna erro |
| `TestInsertBreakNext_BreakID` | BreakID compartilhado entre todos os sub-itens |

### `internal/api/handlers/schedule_test.go`

| Teste | O que verifica |
|---|---|
| `TestScheduleAdd_Break_Valid` | POST com break → 201 com entry view correta |
| `TestScheduleAdd_ItemAndBreak` | POST com item + break → 400 |
| `TestScheduleAdd_Break_NoSpots` | POST com break sem spots → 400 |

---

## Fase 9.E — Documentação

### `docs/specs/17-scheduler.md`

- Seção "Hora Certa": exemplo JSON, nota sobre path vazio, expressão cron recomendada
- Seção "Bloco Comercial": estrutura `break` (Open, Spots, Close), regra de exclusividade com `item`, exemplo completo JSON
- Atualizar tabela de tipos de entrada para incluir `break`

### `docs/specs/03-api-rest.md`

- Seção `POST /v1/schedule`: adicionar campo `break` na tabela de campos, com subesquema e exemplos

### `docs/specs/04-events-websocket.md`

- Atualizar payload de `ScheduleEntryFired` para incluir `break_title` e `spot_count`

### `README.md`

- Atualizar Feature 17 para mencionar suporte a blocos comerciais e hora certa

---

## Arquivos modificados — resumo

| Fase | Arquivo | Ação |
|---|---|---|
| 9.A | `internal/scheduler/manager_test.go` | Adicionar teste HORA_CERTA |
| 9.A | `docs/specs/17-scheduler.md` | Seção Hora Certa |
| 9.B | `internal/commands/types.go` | Constante + payload `CmdInsertBreakNext` |
| 9.B | `internal/queue/manager.go` | Handler `HandleInsertBreakNext` |
| 9.B | `internal/dispatcher/dispatcher.go` | Registrar novo comando |
| 9.C | `internal/scheduler/entry.go` | Campo `Break *commands.BreakItemInput` |
| 9.C | `internal/scheduler/fire.go` | `dispatchInsert`, `publishFired` atualizado |
| 9.C | `internal/events/types.go` | `ScheduleEntryFiredPayload` com campos de break |
| 9.C | `internal/api/handlers/schedule.go` | DTO `breakItemInput`, validação, view |
| 9.D | `internal/scheduler/manager_test.go` | Testes de break |
| 9.D | `internal/queue/manager_test.go` | Testes de `InsertBreakNext` |
| 9.D | `internal/api/handlers/schedule_test.go` | Testes de handler com break |
| 9.E | `docs/specs/17-scheduler.md` | Seções completas |
| 9.E | `docs/specs/03-api-rest.md` | Campo `break` no POST /v1/schedule |
| 9.E | `docs/specs/04-events-websocket.md` | Payload `ScheduleEntryFired` atualizado |
| 9.E | `README.md` | Feature 17 atualizada |

---

## Exemplos de uso

### Hora Certa (todo início de hora)

```json
POST /v1/schedule
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

> `path` deve ser vazio — o engine resolve os arquivos de hora e minuto em `hora_certa.hours_dir`
> e `hora_certa.minutes_dir` no momento do disparo.

### Bloco Comercial (todo dia às 10h30)

```json
POST /v1/schedule
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
      "title": "Abertura do bloco",
      "duration_ms": 5000
    },
    "spots": [
      {
        "path": "/library/spots/anunciante-a.mp3",
        "type": "spots",
        "title": "Anunciante A",
        "duration_ms": 30000
      },
      {
        "path": "/library/spots/anunciante-b.mp3",
        "type": "spots",
        "title": "Anunciante B",
        "duration_ms": 30000
      }
    ],
    "close": {
      "path": "/library/jingles/break-close.mp3",
      "type": "jingles",
      "title": "Encerramento do bloco",
      "duration_ms": 5000
    }
  }
}
```

---

## Notas de implementação

- `Item` e `Break` são **mutuamente exclusivos** — a validação deve rejeitar requests com ambos ou nenhum
- `HandleInsertBreakNext` reutiliza a mesma lógica de expansão de `HandleEnqueueBreak` —
  considerar extrair um helper `expandBreak(b BreakItemInput, breakID string) []QueueItem`
  para evitar duplicação
- A Fase 9.A (HORA_CERTA) pode ser entregue independentemente e imediatamente
- As Fases 9.B e 9.C têm dependência sequencial: o comando precisa existir antes de o scheduler
  poder usá-lo
- A Fase 9.D (testes) deve acompanhar cada sub-fase, não ser deixada para o final

---

## Verificação

```bash
# Testes unitários
go test ./internal/scheduler/... ./internal/queue/... ./internal/api/handlers/...

# Race detector
go test -race ./...

# Build
go build -tags coreaudio ./...

# Smoke test — criar bloco comercial agendado para daqui a 1 minuto
curl -s -X POST http://localhost:8080/v1/schedule \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Teste bloco",
    "enabled": true,
    "fire_at": "2026-07-06T17:00:00-03:00",
    "trigger_mode": "AFTER_CURRENT",
    "break": {
      "title": "Bloco teste",
      "spots": [
        {
          "path": "/tmp/spot-a.mp3",
          "type": "spots",
          "title": "Spot A",
          "duration_ms": 15000
        }
      ]
    }
  }'

# Aguardar evento via WebSocket
# esperado: ScheduleEntryFired com break_title e spot_count
```
