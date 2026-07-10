# Plano: Próximo Evento Agendado — Player UI

## Contexto

O Playout Engine expõe uma grade horária via `GET /v1/schedule` e publica eventos
WebSocket quando entradas são adicionadas, removidas, atualizadas ou disparadas.
A UI do player atualmente não exibe nenhuma informação sobre o que está por vir na
grade — o operador precisa acessar outro painel para saber quando o próximo break
comercial, hora certa ou jingle agendado vai entrar no ar.

Este plano descreve como exibir o **próximo evento agendado** diretamente na tela
do player, com contagem regressiva em tempo real.

---

## De onde vem a informação

### Fonte primária — REST

```
GET /v1/schedule
```

Retorna todas as entradas da grade. Cada entrada habilitada com `next_fire_at`
representa um evento futuro.

```json
{
  "entries": [
    {
      "id": "sched_01JZ...",
      "name": "Bloco Comercial 10h30",
      "enabled": true,
      "cron_expr": "30 10 * * *",
      "trigger_mode": "AFTER_CURRENT",
      "break": { "title": "Bloco das 10h30", "spot_count": 2 },
      "next_fire_at": "2026-07-10T10:30:00-03:00"
    },
    {
      "id": "sched_01JZ...",
      "name": "Hora Certa",
      "enabled": true,
      "cron_expr": "0 * * * *",
      "trigger_mode": "INTERRUPT",
      "item": { "type": "HORA_CERTA", "title": "Hora Certa" },
      "next_fire_at": "2026-07-10T11:00:00-03:00"
    }
  ],
  "count": 2
}
```

**Lógica de seleção do próximo evento:**

1. Filtrar entradas: `enabled === true` e `next_fire_at` no futuro
2. Ordenar por `next_fire_at` ascendente
3. Exibir a primeira entrada da lista

### Fonte secundária — WebSocket (atualização reativa)

A lista deve ser re-buscada via REST quando um dos seguintes eventos chegar:

| Evento WebSocket | Por que re-buscar |
|---|---|
| `ScheduleEntryFired` | Entrada disparou; `next_fire_at` avançou para o próximo ciclo |
| `ScheduleEntryAdded` | Nova entrada pode ser o próximo evento |
| `ScheduleEntryRemoved` | Próximo evento pode ter sido removido |
| `ScheduleEntryUpdated` | Entrada pode ter sido habilitada/desabilitada ou ter mudado de horário |

> `ScheduleEntryFired` e `ScheduleEntryMissed` podem ser descartados pelo engine
> sob backpressure. Por isso o polling de segurança (ver Fase 2) é necessário.

---

## Layout — onde a informação será exibida

O widget é inserido na **topbar**, à direita dos cards de status (Engine, Estado,
Modo, Playout, Library) e à esquerda do relógio. Segue o mesmo padrão visual dos
cards existentes (`.topbar-item`).

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ [LOGO]  [RadioFlow]      ENGINE │ ESTADO │ MODO │ PLAYOUT │ LIBRARY │  PRÓXIMO EVENTO  │  [RELÓGIO]  │
│                           —    │  —     │  —   │ online  │ online  │  10h30 em 00:47  │   14:25:03  │
└──────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Detalhe do card "PRÓXIMO EVENTO"

```
┌────────────────────────────────────┐
│  PRÓXIMO EVENTO                    │
│  Bloco Comercial 10h30   00:47:23  │
│  BREAK • AFTER_CURRENT             │
└────────────────────────────────────┘
```

- **Linha 1 (label):** `PRÓXIMO EVENTO` — padrão `.topbar-label`
- **Linha 2:** nome do evento (`entry.name`) + contagem regressiva `HH:MM:SS`
  em destaque com a cor âmbar `#f5a623`
- **Linha 3:** tipo do evento (`BREAK` / `HORA_CERTA` / `ITEM`) + `trigger_mode`
  em texto diminuto e esmaecido

**Estados do widget:**

| Situação | Exibição |
|---|---|
| Existe próximo evento | Nome + countdown |
| Menos de 5 minutos para o evento | Countdown pisca em vermelho |
| Nenhum evento agendado habilitado | Widget oculto (`display: none`) |
| Scheduler desabilitado / erro na API | Widget oculto silenciosamente |

---

## Arquitetura do fluxo

```
[inicialização da página]
        │
        ▼
fetchNextScheduledEvent()
  GET /v1/schedule
        │
        ├── filtra enabled=true + next_fire_at > agora
        ├── ordena por next_fire_at asc
        └── guarda _nextEvent = entries[0] ou null
                │
                ▼
        renderNextEvent()  — atualiza DOM
                │
                ▼
        setInterval(tickNextEvent, 1000) — countdown a cada 1s

[WebSocket: ScheduleEntryFired | Added | Removed | Updated]
        │
        └── fetchNextScheduledEvent()  — re-busca e re-renderiza

[polling de segurança a cada 60s]
        └── fetchNextScheduledEvent()  — garante consistência mesmo com eventos perdidos
```

---

## Fases de implementação

### Fase 1 — HTML e CSS do widget na topbar

**1.1** Localizar o `<div class="topbar-sep">` e os cards de status em `player.html`.

**1.2** Inserir o novo card imediatamente antes do `<div class="topbar-clock">`:

```html
<!-- ─── Próximo Evento Agendado ─────────────────────────── -->
<div class="topbar-status-div" id="nextEvtDiv" style="display:none"></div>
<div class="topbar-item" id="nextEvtCard" style="display:none">
  <span class="topbar-label">Próximo Evento</span>
  <span class="next-evt-name" id="nextEvtName">—</span>
  <span class="next-evt-meta" id="nextEvtMeta"></span>
</div>
```

**1.3** Adicionar CSS:

```css
/* ─── Próximo Evento ──────────────────────────────────── */
.next-evt-name {
  font-size: 12px; font-weight: 700; color: var(--text-hi);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  max-width: 200px;
  display: flex; align-items: center; gap: 8px;
}
.next-evt-countdown {
  font-size: 13px; font-weight: 900; color: #f5a623;
  font-family: 'Courier New', monospace; letter-spacing: 0.04em;
  flex-shrink: 0;
}
.next-evt-countdown.urgent { color: #ff4444; animation: blink 1s step-end infinite; }
.next-evt-meta {
  font-size: 10px; color: var(--text-dim); letter-spacing: 0.06em;
  text-transform: uppercase; margin-top: 2px;
}
```

**Entrega:** widget visível na topbar com dados estáticos de placeholder.

---

### Fase 2 — Busca e seleção do próximo evento

**2.1** Declarar estado global:

```js
let _nextEvent    = null;   // entrada selecionada ou null
let _nextEvtTimer = null;   // handle do setInterval do countdown
```

**2.2** Implementar `fetchNextScheduledEvent()`:

```js
async function fetchNextScheduledEvent() {
  try {
    const r = await fetch(API_URL + '/v1/schedule');
    if (!r.ok) { hideNextEvent(); return; }
    const d = await r.json();
    const now = Date.now();
    const next = (d.entries || [])
      .filter(e => e.enabled && e.next_fire_at && new Date(e.next_fire_at).getTime() > now)
      .sort((a, b) => new Date(a.next_fire_at) - new Date(b.next_fire_at))[0] || null;
    _nextEvent = next;
    renderNextEvent();
  } catch (_) {
    hideNextEvent();
  }
}
```

**2.3** Implementar `renderNextEvent()`:

- Se `_nextEvent === null`: chama `hideNextEvent()` e encerra
- Caso contrário: popula `#nextEvtName`, `#nextEvtMeta`, inicia o countdown
- Tipo inferido: se `_nextEvent.break` → `BREAK`; se `_nextEvent.item?.type === 'HORA_CERTA'` → `HORA_CERTA`; caso contrário → `ITEM`

**2.4** Implementar `tickNextEvent()` — chamado a cada 1s pelo `setInterval`:

- Calcula `delta = new Date(_nextEvent.next_fire_at) - Date.now()`
- Se `delta <= 0`: chama `fetchNextScheduledEvent()` (evento disparou)
- Formata `HH:MM:SS` e atualiza o span de countdown
- Aplica classe `.urgent` se `delta < 5 * 60 * 1000` (menos de 5 minutos)

**2.5** Polling de segurança:

```js
setInterval(fetchNextScheduledEvent, 60_000);
```

Garante consistência mesmo se eventos WebSocket forem descartados.

**Entrega:** widget exibe o próximo evento real com countdown funcional.

---

### Fase 3 — Atualização reativa via WebSocket

**3.1** Em `handleEvent(evt)`, adicionar os novos cases:

```js
case 'ScheduleEntryFired':
case 'ScheduleEntryMissed':
case 'ScheduleEntryAdded':
case 'ScheduleEntryRemoved':
case 'ScheduleEntryUpdated':
  fetchNextScheduledEvent();
  break;
```

**3.2** Chamar `fetchNextScheduledEvent()` dentro de `onSnapshot(p)` — ao conectar
ou reconectar, o snapshot não inclui dados de schedule, mas a chamada REST garante
inicialização imediata.

**Entrega:** widget atualiza automaticamente ao entrar/sair/alterar entradas na grade.

---

## Resumo de arquivos modificados

| Arquivo | Mudanças |
|---|---|
| `player/player.html` | HTML: card `#nextEvtCard` na topbar; CSS: `.next-evt-*`; JS: `fetchNextScheduledEvent()`, `renderNextEvent()`, `tickNextEvent()`, `hideNextEvent()`, cases no `handleEvent()`, chamada no `onSnapshot()` |

Nenhum arquivo novo. Nenhuma dependência nova. Nenhuma mudança no Playout Engine.

---

## Riscos e mitigações

### 1. Scheduler não configurado ou desabilitado

**Risco:** `GET /v1/schedule` retorna `404` ou lista vazia quando o scheduler está
desabilitado (`scheduler.enabled: false` no YAML do engine).

**Mitigação:** qualquer erro HTTP ou lista vazia após filtragem chama `hideNextEvent()`,
que oculta o card silenciosamente. O operador não vê erro — o widget simplesmente
não aparece.

---

### 2. Eventos WebSocket descartados sob backpressure

**Risco:** `ScheduleEntryFired` é evento de baixa prioridade e pode ser descartado
pelo engine sob carga. Se o evento de disparo não chegar, o countdown pode continuar
rodando após o horário do evento, exibindo tempo negativo ou dado desatualizado.

**Mitigação:**
- `tickNextEvent()` detecta `delta <= 0` e re-busca imediatamente
- Polling de 60s garante que, no pior caso, o widget desatualiza por até 1 minuto

---

### 3. Diferença de relógio entre cliente e servidor

**Risco:** se o relógio do computador do operador estiver adiantado ou atrasado em
relação ao servidor do engine, o countdown exibirá tempo incorreto. Exemplo:
cliente com 3 minutos adiantados verá "00:00:00" 3 minutos antes do evento realmente
disparar.

**Mitigação:** aceitar como limitação conhecida nesta fase. O impacto é baixo — o
evento acontece no horário correto; apenas o countdown visual fica impreciso.
Mitigação futura: incluir timestamp do servidor no `StateSnapshot` para calcular
offset de clock.

---

### 4. Entradas `fire_at` sem `next_fire_at` após disparo

**Risco:** entradas do tipo `fire_at` são auto-desabilitadas após disparar
(`enabled: false`). Se o widget buscar a lista antes do engine processar a
desabilitação, pode tentar exibir um evento já disparado.

**Mitigação:** o filtro `new Date(e.next_fire_at).getTime() > now` descarta
automaticamente entradas com `next_fire_at` no passado, independente do campo
`enabled`. O `tickNextEvent()` com `delta <= 0` re-busca a lista.

---

### 5. Schedule com muitas entradas

**Risco:** grades com dezenas ou centenas de entradas podem tornar a resposta de
`GET /v1/schedule` lenta ou pesada para o cliente.

**Mitigação:** na prática, grades de rádio raramente excedem 20–30 entradas.
Se necessário no futuro, o endpoint pode ser filtrado com `?enabled=true&limit=1&sort=next_fire_at`.
Essa mudança é no engine e fora do escopo deste plano.

---

### 6. Countdown zera mas evento não disparou (SKIP_IF_BUSY / PANIC)

**Risco:** o countdown chega a zero, o evento era `SKIP_IF_BUSY` e o engine estava
ocupado. O evento não disparou, mas o widget re-busca a lista e vai mostrar o
próximo ciclo do cron (se houver) — correto para cron, mas confuso para o operador
que esperava ver a mensagem de "pulado".

**Mitigação:** ao receber `ScheduleEntryMissed` via WebSocket, exibir brevemente
um toast de aviso: `"Evento pulado: <nome>"` por 4 segundos antes de re-renderizar
o próximo evento. Isso dá ao operador feedback visual do miss.

---

## Checklist de validação

- [ ] Widget aparece na topbar quando há pelo menos um evento habilitado com `next_fire_at` futuro
- [ ] Widget fica oculto quando não há eventos agendados ou scheduler está desabilitado
- [ ] Countdown atualiza a cada segundo corretamente no formato `HH:MM:SS`
- [ ] Countdown fica vermelho e piscando quando faltam menos de 5 minutos
- [ ] Ao disparar um evento, o widget atualiza para o próximo da lista automaticamente
- [ ] Ao adicionar/remover/alterar uma entrada via API, o widget re-renderiza
- [ ] Toast de aviso aparece quando `ScheduleEntryMissed` é recebido
- [ ] Ao reconectar o WebSocket, o widget reflete o estado atual
- [ ] Polling de 60s mantém o widget consistente mesmo sem eventos WebSocket
