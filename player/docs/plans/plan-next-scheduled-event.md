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

A lista deve ser re-buscada via REST quando um dos seguintes eventos chegar:

| Evento WebSocket | Por que re-buscar |
|---|---|
| `ScheduleEntryFired` | Entrada disparou; re-calcula próximo evento e retrai o banner |
| `ScheduleEntryAdded` | Nova entrada pode ser o próximo evento |
| `ScheduleEntryRemoved` | Próximo evento pode ter sido removido |
| `ScheduleEntryUpdated` | Entrada pode ter sido habilitada/desabilitada ou alterada |

> `ScheduleEntryFired` e `ScheduleEntryMissed` podem ser descartados pelo engine
> sob backpressure. Por isso o polling de segurança (ver Fase 2) é necessário.

---

## Layout — comportamento do banner

### Estado normal (mais de 5 minutos para o evento)

O banner está **completamente oculto** (`max-height: 0`, invisível). A interface
fica igual à atual — sem nenhuma adição visual.

```
╔══════════════════════════════════════════════════════╗
║  [ PLAY ] [PAUSE] [ STOP ] [ SKIP ] ...  [ PANIC ]  ║
╚══════════════════════════════════════════════════════╝
                         ↑
              (banner não existe aqui)
╔══════════════════════════════════════════════════════╗
║  🎙  MODO ASSIST ATIVADO  (assist-banner existente) ║
╚══════════════════════════════════════════════════════╝
```

---

### Estado de alerta (menos de 5 minutos — banner desce)

O banner **desliza para baixo** com animação CSS suave (transition em `max-height`),
aparecendo logo abaixo da controls-bar. Fundo âmbar escuro, borda superior âmbar.

```
╔══════════════════════════════════════════════════════╗
║  [ PLAY ] [PAUSE] [ STOP ] [ SKIP ] ...  [ PANIC ]  ║
╠══════════════════════════════════════════════════════╣  ← borda âmbar
║  🕐  PRÓXIMO EVENTO — EM  00:04:47                  ║  ← fundo âmbar escuro
║      Bloco Comercial 10h30   •   BREAK               ║
╚══════════════════════════════════════════════════════╝
```

Referência visual do banner conforme mockup:

```
┌──────────────────────────────────────────────────────┐
│ 🕐  PRÓXIMO EVENTO — EM  00:47:23                    │  ← linha 1: ícone + título + countdown
│      Bloco Comercial 10h30   •   BREAK               │  ← linha 2: nome + tipo
└──────────────────────────────────────────────────────┘
```

**Paleta âmbar:**
- Fundo: `rgba(245, 166, 35, 0.08)`
- Borda superior: `2px solid rgba(245, 166, 35, 0.55)`
- Ícone e countdown: `#f5a623`
- Texto principal: `#f0d080`
- Texto secundário: `rgba(240, 208, 128, 0.55)`

---

### Estado de disparo (evento inicia — banner retrai)

Ao receber `ScheduleEntryFired` (ou quando `delta <= 0` no tick), o banner
**desliza para cima** e some. A mesma transição CSS em `max-height` é usada,
mas no sentido inverso — de `max-height: 80px` para `max-height: 0`.

---

### Coexistência com o assist-banner

O `assist-banner` (roxo) já existe abaixo da controls-bar. O banner âmbar fica
**acima** do assist-banner — entre a controls-bar e o assist-banner. Os dois podem
aparecer simultaneamente sem conflito.

```
╔══════════════════════════════════════════════════════╗
║  [ PLAY ] [PAUSE] [ STOP ] [ SKIP ] ...  [ PANIC ]  ║
╠══════════════════════════════════════════════════════╣
║  🕐  PRÓXIMO EVENTO — EM  00:02:11   [âmbar]        ║
║      Hora Certa   •   HORA_CERTA                     ║
╠══════════════════════════════════════════════════════╣
║  🎙  MODO ASSIST ATIVADO             [roxo]          ║
╚══════════════════════════════════════════════════════╝
```

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
        inicia setInterval(tickNextEvent, 1000)

[a cada 1 segundo — tickNextEvent()]
        │
        ├── calcula delta = next_fire_at - Date.now()
        │
        ├── delta <= 0  → fetchNextScheduledEvent() + fecharBanner()
        │
        ├── delta < 5min → abrirBanner() + atualiza countdown
        │
        └── delta >= 5min → (banner já fechado, não faz nada)

[WebSocket: ScheduleEntryFired]
        └── fecharBanner() → fetchNextScheduledEvent()

[WebSocket: ScheduleEntryAdded | Removed | Updated]
        └── fetchNextScheduledEvent()

[polling de segurança a cada 60s]
        └── fetchNextScheduledEvent()
```

---

## Fases de implementação

### Fase 1 — HTML e CSS do banner

**1.1** Localizar o bloco `<div class="assist-banner">` em `player.html`.

**1.2** Inserir o banner âmbar **imediatamente antes** do `assist-banner`:

```html
<!-- ─── Banner: Próximo Evento ──────────────────────── -->
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

**1.3** Adicionar CSS — a animação usa `max-height` + `overflow: hidden` para o
efeito de slide suave sem precisar conhecer a altura exata do banner:

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

.next-evt-dot {
  color: rgba(245, 166, 35, 0.4);
}
```

**Entrega:** banner renderizado no DOM mas oculto (`max-height: 0`). Adicionar
temporariamente a classe `.visible` no HTML confirma visualmente o layout.

---

### Fase 2 — Busca e seleção do próximo evento

**2.1** Declarar estado global:

```js
let _nextEvent    = null;
let _nextEvtTimer = null;
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

**2.3** Implementar `abrirBannerEvento(countdown)`:

```js
function abrirBannerEvento(countdown) {
  const banner = document.getElementById('nextEvtBanner');
  if (!banner) return;
  // Inferir tipo
  let tipo = 'ITEM';
  if (_nextEvent.break)                            tipo = 'BREAK';
  else if (_nextEvent.item?.type === 'HORA_CERTA') tipo = 'HORA_CERTA';
  document.getElementById('nextEvtName').textContent     = _nextEvent.name || '—';
  document.getElementById('nextEvtType').textContent     = tipo;
  document.getElementById('nextEvtCountdown').textContent = countdown;
  banner.classList.add('visible');
}
```

**2.4** Implementar `fecharBannerEvento()`:

```js
function fecharBannerEvento() {
  const banner = document.getElementById('nextEvtBanner');
  if (banner) banner.classList.remove('visible');
}
```

**Entrega:** funções de controle do banner operacionais.

---

### Fase 3 — Countdown e lógica de abertura/fechamento

**3.1** Implementar `tickNextEvent()` — chamado pelo `setInterval` a cada 1s:

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
    // Formata HH:MM:SS
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

**3.2** Iniciar o timer após o primeiro fetch:

```js
async function initSchedule() {
  await fetchNextScheduledEvent();
  if (_nextEvtTimer) clearInterval(_nextEvtTimer);
  _nextEvtTimer = setInterval(tickNextEvent, 1000);
  // Polling de segurança
  setInterval(fetchNextScheduledEvent, 60_000);
}
```

Chamar `initSchedule()` na inicialização da página, após `connect()`.

**Entrega:** banner desce 5 minutos antes do evento e retrai ao atingir zero.

---

### Fase 4 — Atualização reativa via WebSocket

**4.1** Em `handleEvent(evt)`, adicionar os novos cases:

```js
case 'ScheduleEntryFired':
  fecharBannerEvento();
  await fetchNextScheduledEvent();
  break;

case 'ScheduleEntryMissed':
  showToast(false, 'Evento pulado: ' + (evt.payload.entry_name || ''));
  await fetchNextScheduledEvent();
  break;

case 'ScheduleEntryAdded':
case 'ScheduleEntryRemoved':
case 'ScheduleEntryUpdated':
  await fetchNextScheduledEvent();
  break;
```

**4.2** Chamar `initSchedule()` dentro de `onSnapshot(p)` (ao reconectar,
reinicia o monitoramento com estado fresco).

**Entrega:** banner reage em tempo real a qualquer mudança na grade ou disparo.

---

## Resumo de arquivos modificados

| Arquivo | Mudanças |
|---|---|
| `player/player.html` | HTML: `#nextEvtBanner` antes do `assist-banner`; CSS: `.next-evt-banner`, transição `max-height`; JS: `fetchNextScheduledEvent()`, `abrirBannerEvento()`, `fecharBannerEvento()`, `tickNextEvent()`, `initSchedule()`, cases em `handleEvent()`, chamada em `onSnapshot()` |

Nenhum arquivo novo. Nenhuma dependência nova. Nenhuma mudança no Playout Engine.

---

## Riscos e mitigações

### 1. Scheduler não configurado ou desabilitado

**Risco:** `GET /v1/schedule` retorna `404` ou lista vazia quando o scheduler está
desabilitado (`scheduler.enabled: false` no YAML do engine).

**Mitigação:** qualquer erro HTTP ou `_nextEvent = null` mantém o banner oculto
silenciosamente. O operador não vê erro.

---

### 2. Eventos WebSocket descartados sob backpressure

**Risco:** `ScheduleEntryFired` é evento de baixa prioridade e pode ser descartado
pelo engine sob carga. Se o evento não chegar, o banner continuaria exibindo o
countdown mesmo após o evento disparar.

**Mitigação:** `tickNextEvent()` detecta `delta <= 0` e retrai o banner + re-busca
a lista. No pior caso o banner fecha sozinho quando o countdown zera, sem depender
do WebSocket.

---

### 3. Diferença de relógio entre cliente e servidor

**Risco:** se o relógio do computador do operador estiver adiantado ou atrasado em
relação ao servidor do engine, o banner pode abrir cedo ou tarde demais, e o
countdown pode exibir valor impreciso.

**Mitigação:** aceitar como limitação nesta fase. Em geral, os dois sistemas
estarão sincronizados via NTP. Mitigação futura: incluir `server_time` no
`StateSnapshot` para calcular o offset de clock.

---

### 4. Entradas `fire_at` sem `next_fire_at` após disparo

**Risco:** entradas do tipo `fire_at` são auto-desabilitadas após disparar. Se o
cliente buscar a lista com delay, pode tentar exibir um evento já ocorrido.

**Mitigação:** o filtro `new Date(e.next_fire_at).getTime() > now` descarta
automaticamente entradas no passado. O `tickNextEvent()` com `delta <= 0` fecha
o banner imediatamente.

---

### 5. `SKIP_IF_BUSY` — countdown zera mas evento não disparou

**Risco:** o countdown chega a zero, o evento era `SKIP_IF_BUSY` e o engine estava
ocupado. O evento não disparou, mas o banner fecha como se tivesse disparado.

**Mitigação:** ao receber `ScheduleEntryMissed` via WebSocket, exibir toast de
aviso: `"Evento pulado: <nome>"`. O banner fecha normalmente (comportamento
esperado — o evento passou, pulado ou não).

---

### 6. Animação travada (banner não fecha)

**Risco:** a transição CSS em `max-height` pode não funcionar se o valor final
for `auto` — CSS não sabe interpolar de um valor fixo para `auto`.

**Mitigação:** o plano usa `max-height: 80px` (valor fixo) no estado `.visible`,
garantindo que a transição funcione corretamente. O banner tem altura real menor
que 80px, então nunca é cortado.

---

## Checklist de validação

- [ ] Banner permanece oculto quando o próximo evento está a mais de 5 minutos
- [ ] Banner desce suavemente quando faltam exatamente 5 minutos
- [ ] Countdown exibe formato `HH:MM:SS` e atualiza a cada segundo
- [ ] Banner retrai suavemente ao receber `ScheduleEntryFired`
- [ ] Banner retrai quando `delta <= 0` mesmo sem evento WebSocket
- [ ] Toast de aviso aparece quando `ScheduleEntryMissed` é recebido
- [ ] Dois banners podem coexistir: âmbar (próximo evento) + roxo (assist-banner)
- [ ] Ao reconectar o WebSocket, o banner reflete o estado atual
- [ ] Polling de 60s mantém consistência mesmo com eventos WebSocket perdidos
- [ ] Scheduler desabilitado/ausente não causa erro visual — banner simplesmente não aparece
