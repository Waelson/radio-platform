# Plano: Próximo Evento Agendado — Player UI

## Contexto

O Playout Engine expõe uma grade horária via `GET /v1/schedule` e publica eventos
WebSocket quando entradas são adicionadas, removidas, atualizadas ou disparadas.
A UI do player atualmente não exibe nenhuma informação sobre o que está por vir na
grade — o operador precisa acessar outro painel para saber quando o próximo break
comercial, hora certa ou jingle agendado vai entrar no ar.

Este plano descreve como exibir o **próximo evento agendado** como um banner âmbar
que **desce suavemente abaixo dos controles quando faltam menos de 5 minutos** e
**retrai suavemente quando o evento dispara**.

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
3. Monitorar a primeira entrada da lista

### Fonte secundária — WebSocket (atualização reativa)

| Evento WebSocket | Ação |
|---|---|
| `ScheduleEntryFired` | Retrai banner + re-busca próximo evento |
| `ScheduleEntryMissed` | Toast de aviso + re-busca próximo evento |
| `ScheduleEntryAdded` | Re-busca próximo evento |
| `ScheduleEntryRemoved` | Re-busca próximo evento |
| `ScheduleEntryUpdated` | Re-busca próximo evento |

> `ScheduleEntryFired` e `ScheduleEntryMissed` podem ser descartados pelo engine
> sob backpressure. O tick de 1s e o polling de 60s garantem consistência mesmo
> sem esses eventos.

---

## Layout e comportamento

### Estado normal (> 5 minutos para o evento)

Banner **completamente oculto**. Interface fica igual à atual.

```
╔══════════════════════════════════════════════════════╗
║  [ PLAY ] [PAUSE] [ STOP ] [ SKIP ] ...  [ PANIC ]  ║
╚══════════════════════════════════════════════════════╝
      ↑ banner não existe aqui
╔══════════════════════════════════════════════════════╗
║  🎙  MODO ASSIST ATIVADO            (assist-banner) ║
╚══════════════════════════════════════════════════════╝
```

### Estado de alerta (< 5 minutos — banner desce suavemente)

```
╔══════════════════════════════════════════════════════╗
║  [ PLAY ] [PAUSE] [ STOP ] [ SKIP ] ...  [ PANIC ]  ║
╠══════════════════════════════════════════════════════╣  ← borda âmbar
║  🕐  PRÓXIMO EVENTO — EM  00:04:47                  ║  ← fundo âmbar escuro
║      Bloco Comercial 10h30   •   BREAK               ║
╚══════════════════════════════════════════════════════╝
```

Referência visual (mockup):

```
┌──────────────────────────────────────────────────────┐
│ 🕐  PRÓXIMO EVENTO — EM  00:04:47                    │  linha 1: ícone + título + countdown
│      Bloco Comercial 10h30   •   BREAK               │  linha 2: nome do evento + tipo
└──────────────────────────────────────────────────────┘
```

**Paleta âmbar:**
- Fundo: `rgba(245, 166, 35, 0.08)`
- Borda superior: `2px solid rgba(245, 166, 35, 0.55)`
- Countdown: `#f5a623` (monoespaçado)
- Texto principal: `#f0d080`
- Texto secundário: `rgba(240, 208, 128, 0.55)`

### Estado de disparo (evento inicia — banner retrai suavemente)

Ao receber `ScheduleEntryFired` ou quando `delta <= 0`, o banner **desliza para
cima** e some. Mesma transição CSS, sentido inverso.

### Coexistência com o assist-banner

Os dois banners podem aparecer simultaneamente sem conflito. O âmbar fica **acima**
do roxo (assist-banner).

```
╔══════════════════════════════════════════════════════╗
║  [ PLAY ] [PAUSE] [ STOP ] [ SKIP ] ...  [ PANIC ]  ║
╠══════════════════════════════════════════════════════╣
║  🕐  PRÓXIMO EVENTO — EM  00:02:11        [âmbar]   ║
╠══════════════════════════════════════════════════════╣
║  🎙  MODO ASSIST ATIVADO                  [roxo]    ║
╚══════════════════════════════════════════════════════╝
```

---

## Arquitetura do fluxo

```
[inicialização]
      ▼
initSchedule()
  ├── fetchNextScheduledEvent()   → _nextEvent = entry | null
  └── setInterval(tickNextEvent, 1000)
      setInterval(fetchNextScheduledEvent, 60_000)   ← polling de segurança

[tick — a cada 1s]
  delta = next_fire_at - now
  ├── delta <= 0       → fecharBanner() + fetchNextScheduledEvent()
  ├── delta < 5min     → abrirBanner(countdown formatado)
  └── delta >= 5min    → fecharBanner() [já está fechado]

[WebSocket]
  ScheduleEntryFired   → fecharBanner() + fetchNextScheduledEvent()
  ScheduleEntryMissed  → toast("Evento pulado: <nome>") + fetchNextScheduledEvent()
  Entry Added/Removed/Updated → fetchNextScheduledEvent()

[reconexão WebSocket]
  onSnapshot() → initSchedule()
```

---

## Fases de implementação

---

### Fase 1 — Estrutura visual: HTML + CSS

**Objetivo:** banner presente no DOM com aparência correta, animação de slide
funcionando. Sem lógica JS ainda.

**1.1** Localizar `<div class="assist-banner">` em `player.html`.

**1.2** Inserir o banner âmbar **imediatamente antes** do `assist-banner`:

```html
<!-- ─── Banner: Próximo Evento Agendado ─────────────── -->
<div class="next-evt-banner" id="nextEvtBanner">
  <div class="next-evt-banner-icon">🕐</div>
  <div class="next-evt-banner-body">
    <div class="next-evt-banner-title">
      PRÓXIMO EVENTO — EM <span id="nextEvtCountdown">00:00:00</span>
    </div>
    <div class="next-evt-banner-sub">
      <span id="nextEvtName">—</span>
      <span class="next-evt-dot">•</span>
      <span id="nextEvtType">—</span>
    </div>
  </div>
</div>
```

**1.3** Adicionar CSS na seção de estilos:

```css
/* ─── Banner: Próximo Evento ─────────────────────────── */
.next-evt-banner {
  max-height: 0;
  overflow: hidden;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 18px;
  background: rgba(245, 166, 35, 0.08);
  border-top: 2px solid rgba(245, 166, 35, 0.55);
  transition: max-height 0.4s ease, padding 0.4s ease;
}
.next-evt-banner.visible {
  max-height: 80px;
  padding: 10px 18px;
}
.next-evt-banner-icon {
  font-size: 22px;
  flex-shrink: 0;
}
.next-evt-banner-title {
  font-size: 13px;
  font-weight: 900;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #f0d080;
}
#nextEvtCountdown {
  color: #f5a623;
  font-family: 'Courier New', monospace;
}
.next-evt-banner-sub {
  font-size: 12px;
  color: rgba(240, 208, 128, 0.55);
  margin-top: 3px;
  display: flex;
  align-items: center;
  gap: 8px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.next-evt-dot { color: rgba(245, 166, 35, 0.4); }
```

**1.4** Validar visualmente: adicionar `class="next-evt-banner visible"` temporariamente
no HTML para confirmar que layout, cores e animação estão corretos. Remover após validar.

**Entrega:** banner visível com dados estáticos ao adicionar `.visible`; animação
de slide suave ao adicionar/remover a classe no DevTools. Sem JS funcional ainda.

---

### Fase 2 — Busca e estado

**Objetivo:** implementar a lógica de busca, filtragem e seleção do próximo evento.
Sem renderização nem animação ainda — apenas o estado `_nextEvent`.

**2.1** Declarar variáveis de estado no escopo global:

```js
let _nextEvent    = null;   // entrada selecionada ou null
let _nextEvtTimer = null;   // handle do setInterval do countdown
```

**2.2** Implementar `fetchNextScheduledEvent()`:

```js
async function fetchNextScheduledEvent() {
  try {
    const r = await fetch(API_URL + '/v1/schedule');
    if (!r.ok) { _nextEvent = null; return; }
    const d = await r.json();
    const now = Date.now();
    _nextEvent = (d.entries || [])
      .filter(e => e.enabled && e.next_fire_at && new Date(e.next_fire_at).getTime() > now)
      .sort((a, b) => new Date(a.next_fire_at) - new Date(b.next_fire_at))[0] || null;
  } catch (_) {
    _nextEvent = null;
  }
}
```

**2.3** Implementar `_nextEvtTipo()` — helper que infere o tipo a exibir:

```js
function _nextEvtTipo(entry) {
  if (!entry) return '—';
  if (entry.break)                            return 'BREAK';
  if (entry.item?.type === 'HORA_CERTA')      return 'HORA_CERTA';
  return 'ITEM';
}
```

**Entrega:** `fetchNextScheduledEvent()` pode ser chamada no console do DevTools e
`_nextEvent` fica populado com a entrada correta (ou `null`). Nenhuma mudança
visual ainda.

---

### Fase 3 — Countdown e animação (show/hide)

**Objetivo:** implementar as funções de abertura/fechamento do banner e o tick de
1s que controla a lógica de exibição baseada no tempo.

**3.1** Implementar `abrirBannerEvento(countdown)`:

```js
function abrirBannerEvento(countdown) {
  if (!_nextEvent) return;
  document.getElementById('nextEvtName').textContent      = _nextEvent.name || '—';
  document.getElementById('nextEvtType').textContent      = _nextEvtTipo(_nextEvent);
  document.getElementById('nextEvtCountdown').textContent = countdown;
  document.getElementById('nextEvtBanner').classList.add('visible');
}
```

**3.2** Implementar `fecharBannerEvento()`:

```js
function fecharBannerEvento() {
  document.getElementById('nextEvtBanner').classList.remove('visible');
}
```

**3.3** Implementar `tickNextEvent()` — chamado a cada 1s:

```js
function tickNextEvent() {
  if (!_nextEvent) { fecharBannerEvento(); return; }

  const delta = new Date(_nextEvent.next_fire_at).getTime() - Date.now();

  if (delta <= 0) {
    fecharBannerEvento();
    fetchNextScheduledEvent();
    return;
  }

  if (delta < 5 * 60 * 1000) {
    const totalSec = Math.floor(delta / 1000);
    const h = String(Math.floor(totalSec / 3600)).padStart(2, '0');
    const m = String(Math.floor((totalSec % 3600) / 60)).padStart(2, '0');
    const s = String(totalSec % 60).padStart(2, '0');
    abrirBannerEvento(`${h}:${m}:${s}`);
  } else {
    fecharBannerEvento();
  }
}
```

**3.4** Implementar `initSchedule()` e conectar à inicialização da página:

```js
async function initSchedule() {
  await fetchNextScheduledEvent();
  if (_nextEvtTimer) clearInterval(_nextEvtTimer);
  _nextEvtTimer = setInterval(tickNextEvent, 1000);
}
```

Chamar `initSchedule()` após `connect()` na inicialização da página.

**Entrega:** banner desce automaticamente quando o próximo evento está a menos de
5 minutos e retrai ao countdown zerar. Countdown atualiza a cada segundo.

---

### Fase 4 — Reatividade via WebSocket

**Objetivo:** reagir em tempo real às mudanças na grade horária publicadas
pelo engine via WebSocket.

**4.1** Adicionar os novos `case` em `handleEvent(evt)`:

```js
case 'ScheduleEntryFired':
  fecharBannerEvento();
  fetchNextScheduledEvent();
  break;

case 'ScheduleEntryAdded':
case 'ScheduleEntryRemoved':
case 'ScheduleEntryUpdated':
  fetchNextScheduledEvent();
  break;
```

**4.2** Chamar `initSchedule()` dentro de `onSnapshot(p)` para garantir que,
ao reconectar após queda do WebSocket, o monitoramento reinicia com estado fresco.

**Entrega:** banner fecha imediatamente ao receber `ScheduleEntryFired`. Ao
adicionar, remover ou alterar entradas via API, o próximo evento exibido
atualiza sem intervenção do operador.

---

### Fase 5 — Robustez: polling, reconexão e eventos perdidos

**Objetivo:** garantir consistência mesmo sob backpressure e perda de eventos
WebSocket; notificar o operador quando um evento é pulado.

**5.1** Adicionar o case `ScheduleEntryMissed` em `handleEvent(evt)`:

```js
case 'ScheduleEntryMissed':
  showToast(false, 'Evento pulado: ' + (evt.payload.entry_name || ''));
  fetchNextScheduledEvent();
  break;
```

**5.2** Adicionar o polling de segurança dentro de `initSchedule()`:

```js
async function initSchedule() {
  await fetchNextScheduledEvent();
  if (_nextEvtTimer) clearInterval(_nextEvtTimer);
  _nextEvtTimer = setInterval(tickNextEvent, 1000);
  setInterval(fetchNextScheduledEvent, 60_000);   // ← polling de segurança
}
```

**5.3** Verificar que `tickNextEvent()` com `delta <= 0` fecha o banner e re-busca
a lista mesmo sem receber `ScheduleEntryFired` — garante que o banner nunca fique
preso com countdown negativo.

**Entrega:** sistema robusto sob condições adversas — events perdidos, reconexões,
eventos pulados por `SKIP_IF_BUSY` ou `PANIC`. Operador vê toast quando evento
não dispara.

---

## Resumo de arquivos modificados

| Arquivo | Fase | Mudanças |
|---|---|---|
| `player/player.html` | 1 | HTML: `#nextEvtBanner`; CSS: `.next-evt-banner` + transição |
| `player/player.html` | 2 | JS: `_nextEvent`, `_nextEvtTimer`, `fetchNextScheduledEvent()`, `_nextEvtTipo()` |
| `player/player.html` | 3 | JS: `abrirBannerEvento()`, `fecharBannerEvento()`, `tickNextEvent()`, `initSchedule()` |
| `player/player.html` | 4 | JS: cases `ScheduleEntryFired/Added/Removed/Updated` em `handleEvent()`; `initSchedule()` em `onSnapshot()` |
| `player/player.html` | 5 | JS: case `ScheduleEntryMissed` + toast; polling 60s em `initSchedule()` |

Nenhum arquivo novo. Nenhuma dependência nova. Nenhuma mudança no Playout Engine.

---

## Riscos e mitigações

### 1. Scheduler não configurado ou desabilitado

**Risco:** `GET /v1/schedule` retorna `404` ou lista vazia.

**Mitigação:** `fetchNextScheduledEvent()` seta `_nextEvent = null` em qualquer
erro ou lista vazia. `tickNextEvent()` chama `fecharBannerEvento()` — banner
permanece oculto silenciosamente.

---

### 2. Eventos WebSocket descartados sob backpressure

**Risco:** `ScheduleEntryFired` é de baixa prioridade e pode ser descartado.
Banner ficaria exibindo countdown após o evento disparar.

**Mitigação:** `tickNextEvent()` detecta `delta <= 0` e fecha o banner + re-busca.
Polling de 60s garante consistência adicional. No pior caso, o banner fecha quando
o countdown natural zera.

---

### 3. Diferença de relógio entre cliente e servidor

**Risco:** relógio do operador adiantado/atrasado → banner abre cedo/tarde demais.

**Mitigação:** aceitar como limitação nesta fase. Sistemas sincronizados via NTP
em produção. Mitigação futura: usar `server_time` do `StateSnapshot` para calcular
offset de clock.

---

### 4. Entradas `fire_at` já disparadas

**Risco:** cliente busca a lista com delay e tenta exibir evento já ocorrido.

**Mitigação:** filtro `new Date(e.next_fire_at).getTime() > now` descarta entradas
no passado independentemente do campo `enabled`. `tickNextEvent()` com `delta <= 0`
fecha imediatamente.

---

### 5. `SKIP_IF_BUSY` — evento não dispara mas countdown zera

**Risco:** operador vê banner fechar sem o evento ter acontecido de fato.

**Mitigação:** `ScheduleEntryMissed` via WebSocket dispara toast `"Evento pulado: <nome>"`.
O banner fecha normalmente — o evento passou, pulado ou não.

---

### 6. Animação CSS travada (transição não funciona)

**Risco:** `max-height: auto` não pode ser interpolado por CSS — transição não anima.

**Mitigação:** o plano usa `max-height: 80px` (valor fixo e suficiente), garantindo
interpolação correta. O conteúdo real do banner é menor que 80px, sem risco de corte.

---

## Checklist de validação

### Fase 1
- [ ] Banner oculto por padrão (sem `.visible`)
- [ ] Adicionar `.visible` no DevTools faz o banner descer com animação suave
- [ ] Remover `.visible` no DevTools faz o banner subir com animação suave
- [ ] Cores e layout correspondem ao mockup (fundo âmbar escuro, countdown laranja)

### Fase 2
- [ ] `fetchNextScheduledEvent()` no console popula `_nextEvent` com a entrada mais próxima
- [ ] Lista vazia ou erro HTTP resulta em `_nextEvent = null`
- [ ] `_nextEvtTipo()` retorna `BREAK`, `HORA_CERTA` ou `ITEM` corretamente

### Fase 3
- [ ] Banner desce automaticamente quando `delta < 5min`
- [ ] Countdown exibe `HH:MM:SS` e decrementa a cada segundo
- [ ] Banner retrai quando countdown atinge zero
- [ ] `initSchedule()` inicializa corretamente após `connect()`

### Fase 4
- [ ] `ScheduleEntryFired` recebido via WebSocket fecha o banner imediatamente
- [ ] Adicionar nova entrada via API atualiza o próximo evento exibido
- [ ] Reconexão WebSocket reinicia o monitoramento via `onSnapshot()`

### Fase 5
- [ ] Toast `"Evento pulado: <nome>"` aparece ao receber `ScheduleEntryMissed`
- [ ] Polling de 60s mantém consistência mesmo com todos os eventos WebSocket perdidos
- [ ] Banner nunca exibe countdown negativo
- [ ] Dois banners coexistem corretamente: âmbar (próximo evento) acima de roxo (assist)
