# Plano — Tocar qualquer item da fila imediatamente

**Branch:** `feature/play-any-queue-item`
**Data:** 2026-07-21

## Objetivo

Permitir que o operador clique em "Tocar agora" (▶) em qualquer item da fila
que não esteja sendo reproduzido no momento, fazendo o engine interromper o
item atual (skip) e começar a tocar o item selecionado imediatamente.

---

## Análise do estado atual

### Frontend (`player/player.html`)
- O botão "Tocar agora" já existe na UI (`q-ha-btn play`), linha 3865.
- Seu `onclick` atual é `event.stopPropagation()` — sem ação real.
- O botão aparece ao passar o mouse sobre qualquer item com status `QUEUED`.

### Backend (playout engine)
- Não existe endpoint `POST /v1/queue/play-now`.
- Existe `POST /v1/queue/reorder-item` — move item para qualquer posição na fila.
- Existe `POST /v1/playback/skip` — pula o item atual.
- A sequência "mover para frente + skip" precisaria de duas chamadas HTTP,
  criando uma janela de inconsistência (outro item pode entrar entre as duas).
- Solução correta: implementar um único endpoint atômico no engine.

---

## Arquitetura da solução

```
Frontend
  └─ queuePlayNow(itemId)
       └─ POST /v1/queue/play-now  { queue_item_id }
            └─ CmdPlayNow (Command Bus)
                 └─ Dispatcher → queue.Manager.HandlePlayNow
                      ├─ Move o item para o topo da fila (pending[0])
                      └─ Envia sinal de skip para o playback manager
```

O `HandlePlayNow` no `queue.Manager` move o item para `pending[0]`.
O skip é disparado via `Command Bus → CmdSkip` de forma síncrona no mesmo handler,
garantindo atomicidade do ponto de vista do operador.

---

## Fases

### Fase 1 — Backend: novo comando `CmdPlayNow`

**Arquivos modificados:**
- `playout/internal/commands/types.go` — adicionar `CmdPlayNow` e `PlayNowPayload`
- `playout/internal/queue/manager.go` — implementar `HandlePlayNow`
- `playout/internal/dispatcher/dispatcher.go` — registrar `CmdPlayNow` nas matrizes de estado
- `playout/internal/api/handlers/queue.go` — implementar handler `PlayNow`
- `playout/internal/api/server.go` — registrar `POST /v1/queue/play-now`

**Detalhes de implementação:**

```go
// commands/types.go
CmdPlayNow CommandType = "PLAY_NOW"

type PlayNowPayload struct {
    QueueItemID string `json:"queue_item_id"`
}
```

```go
// queue/manager.go — HandlePlayNow
// 1. Localizar o item em m.pending pelo QueueItemID
// 2. Mover para pending[0] (topo da fila)
// 3. Publicar evento de atualização da fila
// 4. Enviar CmdSkip no command bus para interromper o item atual
//    (somente se engine não estiver em IDLE)
```

**Regras:**
- Se `queue_item_id` não for encontrado em `pending` → retornar erro `not_found`.
- Se o item já estiver em `pending[0]` → apenas emitir `CmdSkip` (skip sem reordenação).
- Se o engine estiver `IDLE` (sem nada tocando) → apenas mover para o topo; o
  playback manager iniciará o item naturalmente.
- O comando é permitido nos estados: `PLAYING`, `PAUSED`, `IDLE`, `ASSIST`.
- O comando é rejeitado nos estados: `STOPPING`, `ERROR`, `PANIC`, `STARTING`.

**Dispatcher — estados permitidos:**
```go
commands.CmdPlayNow: true,  // em PLAYING, PAUSED, IDLE, ASSIST
```

**Endpoint:**
```
POST /v1/queue/play-now
Body: { "queue_item_id": "<id>" }
Response 200: { "ok": true, "command_id": "...", "accepted": true }
Response 400: { "ok": false, "error": "bad_request", "message": "queue_item_id is required" }
Response 422: { "ok": false, "error": "not_found",   "message": "item not found in queue" }
```

**Testes (Go):**
- `queue_item_id` vazio → erro de validação.
- Item existente em posição > 0 → move para topo + skip.
- Item já em posição 0 → apenas skip.
- Item inexistente → erro `not_found`.
- Engine em `IDLE` → move para topo, sem skip (playback inicia naturalmente).

---

### Fase 2 — Frontend: wiring do botão "Tocar agora"

**Arquivo modificado:** `player/player.html`

**Objetivo:** substituir o `onclick` no-op pelo handler real.

**Implementação:**

```javascript
// Substituir:
onclick="event.stopPropagation()"
// Por:
onclick="event.stopPropagation(); queuePlayNow('${escHtml(itemId)}')"
```

```javascript
async function queuePlayNow(itemId) {
  if (!itemId) return
  try {
    const res = await fetch(API_URL + '/v1/queue/play-now', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ queue_item_id: itemId }),
    })
    const j = await res.json()
    if (!j.ok) showToast(false, j.reason || j.message || 'Erro ao tocar item')
    // fetchQueue() será chamado automaticamente pelo polling ou WS
  } catch {
    showToast(false, 'Erro de rede ao tocar item')
  }
}
```

**Regras de visibilidade do botão:**
- O botão "Tocar agora" só aparece em itens com status `QUEUED` (já funciona
  assim — o código atual só renderiza os botões `.q-ha-btn` para esses itens).
- Itens `PLAYING` ou `PAUSED` já mostram a barra de progresso; não têm o botão.
- Itens em estado de crossfade (`pending` mas não `is-draggable`) não têm o botão.

---

### Fase 3 — Feedback visual e casos de borda

**Feedback ao usuário:**
- Toast de erro quando `j.ok === false` (item não encontrado, engine em estado inválido).
- A fila atualiza automaticamente via polling (1 s) após o skip — sem ação extra.
- Não é necessário spinner no botão: o polling já reflete o novo estado em ~1 s.

**Casos de borda cobertos pela Fase 1:**
| Cenário | Comportamento |
|---|---|
| Item já é o próximo (pending[0]) | Skip imediato sem reordenar |
| Engine em IDLE | Move para topo; playback inicia naturalmente |
| Engine em PANIC | Comando rejeitado pelo dispatcher; toast de erro |
| Engine em STOPPING | Comando rejeitado; toast de erro |
| item_id inválido | 400/422 com mensagem de erro; toast |
| Dois cliques rápidos | Segundo skip é no-op (nada tocando entre os dois) |

---

## Ordem de execução

1. [ ] Fase 1a — `commands/types.go`: adicionar `CmdPlayNow` e `PlayNowPayload`
2. [ ] Fase 1b — `dispatcher/dispatcher.go`: registrar `CmdPlayNow`
3. [ ] Fase 1c — `queue/manager.go`: implementar `HandlePlayNow`
4. [ ] Fase 1d — `api/handlers/queue.go`: implementar handler HTTP `PlayNow`
5. [ ] Fase 1e — `api/server.go`: registrar rota `POST /v1/queue/play-now`
6. [ ] Fase 1f — `go test ./...` para validar
7. [ ] Fase 2 — `player/player.html`: wiring do botão + função `queuePlayNow`
8. [ ] Fase 3 — validação manual dos casos de borda

---

## Critérios de aceite

- Clicar em "Tocar agora" em qualquer item `QUEUED` faz o engine pular o item
  atual e começar a tocar o item selecionado em até ~2 s.
- A fila é atualizada visualmente sem ação do operador.
- Clicar em um item já inexistente (removido entre render e clique) exibe toast
  de erro claro.
- `go test ./...` passa sem erros.
- Nenhuma regressão nos endpoints existentes.
