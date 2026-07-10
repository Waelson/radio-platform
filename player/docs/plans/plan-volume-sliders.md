# Plano: Sliders de Volume no Painel Lateral ✅ IMPLEMENTADO

## Contexto

O Playout Engine (v0.2+) já expõe controle de volume individual para a fila principal e para
o player de preview (CUE) via REST e WebSocket. A UI (`player.html`) ainda não consome esses
dados — não há sliders, não há leitura do volume atual e não há sincronização entre clientes.

Este plano descreve como implementar os sliders de volume no painel lateral esquerdo
(`col-meters`), de forma consistente com o design visual atual e segura diante de múltiplos
clientes conectados simultaneamente.

---

## Layout completo da UI

O layout atual possui uma barra superior (`topbar`) e três colunas principais. A seção
**VOLUME** é inserida na coluna esquerda (`col-meters`), entre "Underruns" e "VU Meters".

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│ TOPBAR                                                                                    │
│  ● ENGINE: online    ESTADO: PLAYING    MODO: AUTO    PAYOUT: studio-a-main   🕐 14:25:03 │
└──────────────────────────────────────────────────────────────────────────────────────────┘

┌───────────────────────────┬──────────────────────────────────┬───────────────────────────┐
│  col-meters               │  col-player                      │  col-library              │
│                           │                                  │                           │
│  ┌─────────────────────┐  │  ┌──────────────────────────┐   │  ┌─────────────────────┐  │
│  │ LOUDNESS (EBU R128) │  │  │ NOW PLAYING              │   │  │ BIBLIOTECA          │  │
│  │  LUFS momentâneo    │  │  │  ♪ Faixa A – Artista A   │   │  │  🔍 Buscar...       │  │
│  │  ─────── -14.2 LUFS │  │  │  Álbum • 2024 • musicas  │   │  │                     │  │
│  │  LUFS integrado     │  │  │                          │   │  │  Todas as faixas    │  │
│  │  ─────── -16.0 LUFS │  │  │  [░░░░░░░░░░░░░░░░░░░░]  │   │  │  ┌────────────────┐ │  │
│  └─────────────────────┘  │  │  waveform canvas         │   │  │  │ Faixa A    3:58│ │  │
│                           │  │  [░░░░░░░░░░░░░░░░░░░░]  │   │  │  │ Faixa B    4:12│ │  │
│  ┌─────────────────────┐  │  │  2:03  ──────●────  3:58 │   │  │  │ Faixa C    2:44│ │  │
│  │ CANAL L   CANAL R   │  │  └──────────────────────────┘   │  │  │ Faixa D    5:01│ │  │
│  │ -14.2dBFS -14.2dBFS │  │                                  │  │  └────────────────┘ │  │
│  └─────────────────────┘  │  ┌──────────────────────────┐   │  └─────────────────────┘  │
│                           │  │ CONTROLES                │   │                           │
│  ┌─────────────────────┐  │  │ [▶ PLAY] [⏸ PAUSE]      │   │  ┌─────────────────────┐  │
│  │ SAÚDE DO ÁUDIO      │  │  │ [⏹ STOP] [⏭ SKIP]       │   │  │ FILA                │  │
│  │  Buffer saída   0%  │  │  │                          │   │  │  1. Faixa A → atual │  │
│  │  Nível RMS  -60dBFS │  │  │ [BOTÕES] [BIB] [ASSIST]  │   │  │  2. Faixa B         │  │
│  │  Silêncio    NÃO    │  │  │          [PANIC]         │   │  │  3. Faixa C         │  │
│  │  Underruns     0    │  │  └──────────────────────────┘   │  └─────────────────────┘  │
│  └─────────────────────┘  │                                  │                           │
│                           │  ┌──────────────────────────┐   │                           │
│  ┌─────────────────────┐  │  │ BOTÕES QUENTES           │   │                           │
│  │ VOLUME          ←new│  │  │  [JNG 1] [JNG 2] [JNG 3] │   │                           │
│  │                     │  │  │  [VNH 1] [VNH 2] [VNH 3] │   │                           │
│  │  PRINCIPAL          │  │  └──────────────────────────┘   │                           │
│  │  ●──────────○  80%  │  │                                  │                           │
│  │                     │  │                                  │                           │
│  │  PLAYER (CUE)       │  │                                  │                           │
│  │  ●────────────○100% │  │                                  │                           │
│  │  [desab. se CUE off]│  │                                  │                           │
│  └─────────────────────┘  │                                  │                           │
│                           │                                  │                           │
│  ┌─────────────────────┐  │                                  │                           │
│  │ VU METERS           │  │                                  │                           │
│  │   L          R      │  │                                  │                           │
│  │  ┃███┃      ┃███┃   │  │                                  │                           │
│  └─────────────────────┘  │                                  │                           │
└───────────────────────────┴──────────────────────────────────┴───────────────────────────┘
```

---

## Posicionamento na UI

**Coluna esquerda (`col-meters`), entre "Underruns" e "VU Meters".**

Essa coluna já é a "coluna de controle técnico" — o operador mantém o olhar ali durante a
transmissão. Os sliders ficam acessíveis sem interromper o fluxo de operação da fila e dos
botões de playback.

```
┌─────────────────────────────┐
│ LOUDNESS (EBU R128)         │
│  LUFS momentâneo  ───────   │
│  LUFS integrado   ───────   │
├─────────────────────────────┤
│ CANAL L       CANAL R       │
│ -60.0 dBFS    -60.0 dBFS    │
├─────────────────────────────┤
│ SAÚDE DO ÁUDIO              │
│  Buffer saída    0%         │
│  Nível RMS      -60 dBFS    │
│  Silêncio       NÃO         │
│  Underruns      0           │
├═════════════════════════════┤  ← nova seção entra aqui
│ VOLUME                      │
│                             │
│  PRINCIPAL                  │
│  ●────────────────○  80%    │
│                             │
│  PLAYER (CUE)               │
│  ●──────────────────○  100% │
│                             │
│  [desabilitado se CUE off]  │
├─────────────────────────────┤
│ VU METERS                   │
│   L          R              │
│  ┃███┃      ┃███┃           │
└─────────────────────────────┘
```

---

## Arquitetura do fluxo

```
Engine (Go)
    │
    ├── WebSocket /v1/events ──► handleEvent()
    │       ├── StateSnapshot        → inicializa sliders (volume inicial)
    │       ├── VolumeChanged        → atualiza slider Principal
    │       └── PreviewVolumeChanged → atualiza slider Player (CUE)
    │
    └── REST API
            ├── PUT /v1/playback/volume  ← slider Principal (ao soltar)
            └── PUT /v1/preview/volume   ← slider Player CUE (ao soltar)
```

**Fluxo de uma alteração pelo operador:**

```
[Operador arrasta slider]
        │
        ▼
 Atualização visual otimista (imediata, sem esperar API)
        │
        ▼
 PUT enviado ao soltar (evento change)
        │
        ├── 200 OK → confirma; engine publica VolumeChanged no WebSocket
        │                       → outros clientes abertos sincronizam slider
        │
        └── Erro (rede/500) → rollback: restaura valor anterior no slider
                             → exibe toast de erro
```

---

## Eventos WebSocket consumidos

### `StateSnapshot`

Enviado automaticamente pelo hub ao conectar ou reconectar. Contém o estado completo do
engine, incluindo os campos `main_volume` e `preview_volume`.

```json
{
  "type": "StateSnapshot",
  "payload": {
    "state": "PLAYING",
    "mode": "AUTO",
    "main_volume": 0.8,
    "preview_volume": 1.0,
    ...
  }
}
```

**Ação na UI:** inicializar ambos os sliders com os valores recebidos. Isso garante que ao
abrir a UI ou reconectar após queda de rede, os sliders sempre refletem o volume real do
engine sem precisar de uma chamada REST adicional.

**Onde no código:** dentro de `onSnapshot(p)` já existente em `player.html`.

---

### `VolumeChanged`

Publicado pelo engine imediatamente após processar um `PUT /v1/playback/volume`. Prioridade
alta — nunca descartado sob backpressure.

```json
{
  "type": "VolumeChanged",
  "payload": { "level": 0.8 }
}
```

**Ação na UI:** atualizar o slider Principal e o rótulo de percentual, **exceto** se o
usuário estiver ativamente arrastando o slider neste momento (flag `_draggingMain`). Essa
exclusão evita o efeito de "slider pulando" durante o arrasto quando o evento WebSocket chega
em lag.

**Onde no código:** novo `case 'VolumeChanged':` em `handleEvent()`.

---

### `PreviewVolumeChanged`

Publicado pelo engine imediatamente após processar um `PUT /v1/preview/volume`.

```json
{
  "type": "PreviewVolumeChanged",
  "payload": { "level": 0.6 }
}
```

**Ação na UI:** atualizar o slider Player (CUE), exceto se `_draggingPreview` estiver ativo.

**Onde no código:** novo `case 'PreviewVolumeChanged':` em `handleEvent()`.

---

## Endpoints REST chamados

| Método | Endpoint | Body | Quando |
|---|---|---|---|
| `PUT` | `/v1/playback/volume` | `{"level": 0.8}` | Ao soltar slider Principal (`change`) |
| `PUT` | `/v1/preview/volume` | `{"level": 0.6}` | Ao soltar slider Player CUE (`change`) |
| `GET` | `/v1/playback/volume` | — | Fallback se `StateSnapshot` não tiver `main_volume` |
| `GET` | `/v1/preview/volume` | — | Fallback se `StateSnapshot` não tiver `preview_volume` |

**Nota sobre o evento `input` vs `change`:**
- `input` — dispara a cada pixel de movimento. Usar apenas para atualizar o rótulo visual.
- `change` — dispara ao soltar o mouse/touch. Usar para enviar o `PUT`. Isso evita flood de
  requisições e é o comportamento correto para operações de controle.

---

## Estrutura HTML a adicionar

Inserir imediatamente **após** o bloco `health-section` (que termina no `</div>` após
"Underruns") e **antes** de `<!-- VU Meters -->`:

```html
<!-- ─── Volume ───────────────────────────── -->
<div class="vol-section">
  <div class="section-title">Volume</div>

  <div class="vol-row">
    <div class="vol-label">Principal</div>
    <div class="vol-control">
      <input
        id="volMain"
        type="range"
        class="vol-slider"
        min="0" max="100" step="1" value="100"
        oninput="onVolInput('main', this)"
        onchange="onVolChange('main', this)"
        onmousedown="_draggingMain=true"
        onmouseup="_draggingMain=false"
        ontouchstart="_draggingMain=true"
        ontouchend="_draggingMain=false"
      >
      <span class="vol-pct" id="volMainPct">100%</span>
    </div>
  </div>

  <div class="vol-row" id="volPreviewRow">
    <div class="vol-label">Player (CUE)</div>
    <div class="vol-control">
      <input
        id="volPreview"
        type="range"
        class="vol-slider"
        min="0" max="100" step="1" value="100"
        oninput="onVolInput('preview', this)"
        onchange="onVolChange('preview', this)"
        onmousedown="_draggingPreview=true"
        onmouseup="_draggingPreview=false"
        ontouchstart="_draggingPreview=true"
        ontouchend="_draggingPreview=false"
      >
      <span class="vol-pct" id="volPreviewPct">100%</span>
    </div>
  </div>
</div>
```

---

## CSS a adicionar

Inserir na seção de estilos, consistente com o design escuro existente (`--bg2`, `--border`,
`--cyan`, `--text-dim`):

```css
/* ─── Volume ─────────────────────────────── */
.vol-section {
  padding: 10px 12px 6px;
  border-top: 1px solid var(--border);
}

.vol-row {
  margin-bottom: 10px;
}

.vol-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1px;
  color: var(--text-dim);
  margin-bottom: 4px;
}

.vol-control {
  display: flex;
  align-items: center;
  gap: 8px;
}

.vol-pct {
  font-size: 12px;
  font-weight: 700;
  color: var(--cyan);
  min-width: 36px;
  text-align: right;
  font-family: 'Courier New', monospace;
}

input[type=range].vol-slider {
  flex: 1;
  accent-color: var(--cyan);
  cursor: pointer;
  height: 4px;
}

/* Slider desabilitado (preview off) */
input[type=range].vol-slider:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
```

---

## JavaScript a adicionar

### Estado local

```js
let _draggingMain    = false;
let _draggingPreview = false;
let _volMainPrev     = 1.0;  // para rollback em caso de erro
let _volPreviewPrev  = 1.0;
```

### Atualização visual (sem API)

```js
function onVolInput(channel, el) {
  const pct = el.value + '%';
  if (channel === 'main')    document.getElementById('volMainPct').textContent    = pct;
  if (channel === 'preview') document.getElementById('volPreviewPct').textContent = pct;
}
```

### Envio à API (ao soltar o slider)

```js
async function onVolChange(channel, el) {
  const level   = parseFloat((parseInt(el.value, 10) / 100).toFixed(2));
  const prevVal = channel === 'main' ? _volMainPrev : _volPreviewPrev;
  const endpoint = channel === 'main'
    ? '/v1/playback/volume'
    : '/v1/preview/volume';

  // Salva valor anterior para rollback
  if (channel === 'main')    _volMainPrev    = level;
  if (channel === 'preview') _volPreviewPrev = level;

  try {
    const r = await fetch(API_URL + endpoint, {
      method:  'PUT',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify({ level }),
    });
    if (!r.ok) throw new Error('HTTP ' + r.status);
  } catch (e) {
    // Rollback visual
    const pctId  = channel === 'main' ? 'volMainPct' : 'volPreviewPct';
    const slider = document.getElementById(channel === 'main' ? 'volMain' : 'volPreview');
    slider.value = Math.round(prevVal * 100);
    document.getElementById(pctId).textContent = slider.value + '%';
    if (channel === 'main')    _volMainPrev    = prevVal;
    if (channel === 'preview') _volPreviewPrev = prevVal;
    showToast(false, 'Erro ao ajustar volume');
  }
}
```

### Atualização pelos eventos WebSocket

Adicionar em `handleEvent()`:

```js
case 'VolumeChanged':
  if (!_draggingMain) setVolDisplay('main', payload.level);
  break;

case 'PreviewVolumeChanged':
  if (!_draggingPreview) setVolDisplay('preview', payload.level);
  break;
```

Função auxiliar:

```js
function setVolDisplay(channel, level) {
  const sliderId = channel === 'main' ? 'volMain'    : 'volPreview';
  const pctId    = channel === 'main' ? 'volMainPct' : 'volPreviewPct';
  const pct      = Math.round(level * 100);
  document.getElementById(sliderId).value          = pct;
  document.getElementById(pctId).textContent = pct + '%';
  if (channel === 'main')    _volMainPrev    = level;
  if (channel === 'preview') _volPreviewPrev = level;
}
```

### Inicialização via StateSnapshot

Adicionar em `onSnapshot(p)`:

```js
if (p.main_volume    != null) setVolDisplay('main',    p.main_volume);
if (p.preview_volume != null) setVolDisplay('preview', p.preview_volume);
```

### Fallback de inicialização (se StateSnapshot chegar sem volume)

Chamar na inicialização da página, após `connect()`:

```js
async function fetchInitialVolumes() {
  try {
    const [rv, rp] = await Promise.all([
      fetch(API_URL + '/v1/playback/volume'),
      fetch(API_URL + '/v1/preview/volume'),
    ]);
    if (rv.ok) { const d = await rv.json(); setVolDisplay('main',    d.level); }
    if (rp.ok) { const d = await rp.json(); setVolDisplay('preview', d.level); }
  } catch (e) {
    console.warn('[player] volume fetch fallback error', e);
  }
}
```

### Desabilitar slider de Preview quando CUE está offline

O endpoint `/v1/preview/volume` retorna `503` quando `preview.enabled: false`. O slider deve
ser desabilitado e acinzentado nesses casos:

```js
function setPreviewAvailable(available) {
  const el = document.getElementById('volPreview');
  if (el) el.disabled = !available;
}
```

Chamar em resposta ao status do engine (verificar se `preview_enabled` está no payload do
`StateSnapshot` ou inferir pelo `503` no fallback).

---

## Riscos e mitigações

### 1. Slider "pulando" durante arrasto (race condition WebSocket × usuário)

**Risco:** enquanto o operador está arrastando o slider, chega um evento `VolumeChanged` de
outro cliente (ou o eco do próprio `PUT`). O slider salta para o valor do evento, interrompendo
o arrasto.

**Mitigação:** flags `_draggingMain` e `_draggingPreview` definidas em `onmousedown`/
`ontouchstart`. Enquanto a flag estiver ativa, eventos WebSocket não atualizam o slider.
Flags limpas em `onmouseup`/`ontouchend`. Risco residual: se o usuário arrastar fora do
slider (perda do `mouseup`), a flag fica presa. Solução: adicionar listener de
`mouseup` no `document` como backup.

---

### 2. Dois operadores alterando volume simultaneamente

**Risco:** operador A muda para 60%, operador B muda para 40% quase ao mesmo tempo. O engine
processa sequencialmente, mas os dois recebem o evento do outro e seus sliders saltam após
soltar.

**Mitigação:** comportamento esperado e correto — o WebSocket sincroniza todos os clientes
para o estado real do engine. O "salto" ocorre apenas após o operador soltar o slider
(evento `change`), não durante o arrasto. Isso é aceitável em ambiente de rádio onde há
apenas um operador por cabine.

---

### 3. Latência de rede / echo loop

**Risco:** o evento `VolumeChanged` chega de volta ao cliente que originou a mudança,
causando uma segunda atualização visual desnecessária.

**Mitigação:** como a flag `_draggingMain` já está limpa quando o `PUT` é enviado (evento
`change`), o echo WebSocket não causa problemas visuais — simplesmente reconfirma o valor
que o slider já exibe. Não é necessário suprimir o echo.

---

### 4. Preview desabilitado (`503`)

**Risco:** o slider de Preview envia `PUT /v1/preview/volume` quando o engine foi iniciado
com `preview.enabled: false`. A requisição retorna `503`.

**Mitigação:** o fallback `fetchInitialVolumes()` detecta `503` no endpoint de preview e
chama `setPreviewAvailable(false)`, que desabilita o slider com `disabled` e aplica opacidade
reduzida via CSS. O rollback é trivial (slider já estava desabilitado).

---

### 5. Perda de conexão WebSocket durante arrasto

**Risco:** a conexão cai enquanto o operador arrasta. O `PUT` é enviado mas o
`VolumeChanged` nunca chega. Ao reconectar, o `StateSnapshot` traz o estado real.

**Mitigação:** a reconexão automática já existe (`setTimeout(connect, 2000)` no `onclose`).
O `StateSnapshot` inicial atualizará os sliders via `onSnapshot()`. Nenhuma ação adicional
necessária.

---

### 6. Valor inválido no slider (NaN, < 0, > 1)

**Risco:** bug no código de conversão `parseInt / 100` gera valor inválido. O engine retorna
`400 invalid_level`.

**Mitigação:** o endpoint do engine já valida e rejeita valores fora de `[0.0, 1.0]`. Na UI,
`Math.min(100, Math.max(0, parseInt(el.value)))` antes de converter garante que nunca sai
do intervalo. O rollback via `showToast` informa o operador.

---

### 7. Toque em dispositivos touch (tablet de cabine)

**Risco:** eventos `ontouchstart`/`ontouchend` não disparam `onchange` em alguns navegadores
embedded do Electron.

**Mitigação:** usar também o evento `change` nativo do `<input type="range">`, que é
disparado consistentemente no Electron/Chromium ao soltar o toque. Monitorar em testes com
touch. Alternativa: usar `pointerdown`/`pointerup` (API unificada mouse + touch).

---

## Fases de implementação

### Fase 1 — Estrutura HTML e CSS

**Objetivo:** sliders visíveis na UI com aparência correta, sem funcionalidade ainda.

**1.1** Localizar o ponto de inserção em `player.html`: o `</div>` que fecha o bloco
`health-section` (contém "Underruns"), imediatamente antes do comentário `<!-- VU Meters -->`.

**1.2** Inserir o bloco HTML `.vol-section` com dois `<div class="vol-row">`:
- `#volMain` + `#volMainPct` para o canal Principal
- `#volPreview` + `#volPreviewPct` para o canal Player (CUE)
- Atributos `oninput`, `onchange`, `onmousedown`, `onmouseup`, `ontouchstart`, `ontouchend`
  podem ficar presentes no HTML mesmo que as funções JS ainda não existam (chamadas serão
  silenciosas até a Fase 3).

**1.3** Inserir o bloco CSS: `.vol-section`, `.vol-row`, `.vol-label`, `.vol-control`,
`.vol-pct`, `input[type=range].vol-slider`, `input[type=range].vol-slider:disabled`.

**1.4** Verificar visualmente no browser/Electron: seção "Volume" deve aparecer entre
"Saúde do Áudio" e "VU Meters", com dois sliders e rótulos "100%".

**Entrega:** sliders visíveis mas estáticos (valor fixo 100%, sem interação).

---

### Fase 2 — Inicialização de estado

**Objetivo:** sliders inicializados com o volume real do engine ao abrir ou reconectar.

**2.1** Declarar as 4 variáveis de estado no escopo global do script:
```js
let _draggingMain = false, _draggingPreview = false;
let _volMainPrev  = 1.0,   _volPreviewPrev  = 1.0;
```

**2.2** Implementar a função `setVolDisplay(channel, level)`:
- Calcula `pct = Math.round(level * 100)`
- Atualiza `.value` do slider e `.textContent` do rótulo
- Atualiza `_volMainPrev` ou `_volPreviewPrev` para referência de rollback

**2.3** Adicionar ao corpo de `onSnapshot(p)` (já existe na UI):
```js
if (p.main_volume    != null) setVolDisplay('main',    p.main_volume);
if (p.preview_volume != null) setVolDisplay('preview', p.preview_volume);
```
Isso garante inicialização automática ao conectar ou reconectar via WebSocket.

**2.4** Implementar `fetchInitialVolumes()` como fallback para cenários onde
`StateSnapshot` não inclui os campos de volume (engine antigo ou bug):
- `Promise.all` com `GET /v1/playback/volume` e `GET /v1/preview/volume`
- Chamar `setVolDisplay` apenas se a resposta for `ok`
- Se preview retornar `503`, chamar `setPreviewAvailable(false)` (implementada na Fase 5)
- Erros de rede: `console.warn` apenas, sem interromper inicialização

**2.5** Chamar `fetchInitialVolumes()` logo após `connect()` na inicialização da página,
como segunda linha de defesa.

**Entrega:** sliders inicializados corretamente ao abrir a UI pela primeira vez e após
qualquer reconexão.

---

### Fase 3 — Interação (PUT ao soltar o slider)

**Objetivo:** operador pode ajustar volume em tempo real; erros mostram rollback + toast.

**3.1** Implementar `onVolInput(channel, el)`:
- Apenas atualiza o rótulo `#volMainPct` ou `#volPreviewPct` com `el.value + '%'`
- Sem chamada de rede — disparado a cada frame do arrasto para feedback visual imediato

**3.2** Implementar `onVolChange(channel, el)`:
- Converte `el.value` (0–100) para `level` (0.0–1.0): `parseFloat((parseInt(el.value)/100).toFixed(2))`
- Guarda `prevVal = _volMainPrev` (ou preview) antes de qualquer alteração
- Atualiza `_volMainPrev` (ou preview) com o novo valor
- Executa `fetch(API_URL + endpoint, { method: 'PUT', ... })`
- Em caso de erro HTTP ou rede: restaura `slider.value`, `_volXxxPrev` e chama `showToast(false, 'Erro ao ajustar volume')`

**3.3** Adicionar listener de safety net no `document` para evitar flag presa:
```js
document.addEventListener('mouseup',  () => { _draggingMain = false; _draggingPreview = false; });
document.addEventListener('touchend', () => { _draggingMain = false; _draggingPreview = false; });
```
Inserir durante a inicialização da página (junto com `connect()` / `fetchInitialVolumes()`).

**3.4** Validar que `onmousedown`/`onmouseup`/`ontouchstart`/`ontouchend` inline no HTML
(adicionados na Fase 1) agora chamam as flags corretamente.

**Entrega:** sliders funcionais; ajuste de volume opera em tempo real; erros de rede fazem
rollback limpo; flag de drag nunca fica presa.

---

### Fase 4 — Sincronização via WebSocket

**Objetivo:** múltiplos clientes abertos sincronizam sliders em tempo real.

**4.1** Adicionar `case 'VolumeChanged':` em `handleEvent(evt)`:
```js
case 'VolumeChanged':
  if (!_draggingMain) setVolDisplay('main', evt.payload.level);
  break;
```
A guard `!_draggingMain` é essencial: sem ela, o echo do próprio `PUT` salta o slider
enquanto o operador ainda arrasta em outro dispositivo.

**4.2** Adicionar `case 'PreviewVolumeChanged':` em `handleEvent(evt)`:
```js
case 'PreviewVolumeChanged':
  if (!_draggingPreview) setVolDisplay('preview', evt.payload.level);
  break;
```

**4.3** Verificar que o eco do `PUT` (enviado pelo próprio cliente) não causa salto visual:
- Após soltar o slider, `_draggingMain` já é `false`
- O eco chega via WebSocket e `setVolDisplay` reconfirma o valor que o slider já exibe
- Comportamento correto: sem salto visível, sem efeito colateral

**4.4** Teste manual: abrir a UI em duas abas/janelas. Mover slider na janela A → slider na
janela B deve atualizar automaticamente sem interação.

**Entrega:** todos os clientes conectados exibem sempre o volume real do engine.

---

### Fase 5 — Preview desabilitado

**Objetivo:** UI resiliente quando `preview.enabled: false` no engine.

**5.1** Implementar `setPreviewAvailable(available)`:
```js
function setPreviewAvailable(available) {
  const el = document.getElementById('volPreview');
  if (el) el.disabled = !available;
}
```
O CSS `input[type=range].vol-slider:disabled { opacity: 0.35; cursor: not-allowed; }`
(adicionado na Fase 1) cuidará da aparência visual automaticamente.

**5.2** Chamar `setPreviewAvailable(false)` em `fetchInitialVolumes()` quando o
`GET /v1/preview/volume` retornar `503`.

**5.3** Verificar comportamento: slider desabilitado não dispara `onchange`, portanto não
envia `PUT`. Não é necessário guardar lógica adicional — `disabled` bloqueia o elemento
nativamente no browser/Electron.

**5.4** Teste: iniciar engine com `preview.enabled: false` → abrir UI → slider "Player (CUE)"
deve aparecer acinzentado e não-interativo.

**Entrega:** UI resiliente; slider de Preview desabilitado quando CUE está offline.

---

## Resumo de arquivos modificados

| Arquivo | Mudanças |
|---|---|
| `player/player.html` | HTML: bloco `.vol-section`; CSS: estilos do slider; JS: variáveis de estado, `setVolDisplay()`, `onVolInput()`, `onVolChange()`, `fetchInitialVolumes()`, `setPreviewAvailable()`; `onSnapshot()` atualizado; `handleEvent()` com 2 novos cases |

Nenhum arquivo novo. Nenhuma dependência nova. Nenhuma mudança no Playout Engine.

---

## Checklist de validação

- [ ] Slider Principal reflete o volume correto ao abrir a UI pela primeira vez
- [ ] Slider Principal reflete o volume correto ao reconectar após queda de WebSocket
- [ ] Arrastar slider Principal atualiza o rótulo visual em tempo real (sem lag)
- [ ] Soltar slider Principal envia `PUT /v1/playback/volume` e engine aplica ganho
- [ ] Segunda janela aberta sincroniza o slider ao soltar na primeira
- [ ] Erro de rede no `PUT` faz rollback do slider e exibe toast
- [ ] Slider Player (CUE) funciona de forma idêntica ao Principal
- [ ] Quando `preview.enabled: false`, slider Player (CUE) aparece desabilitado
- [ ] Arrastar fora do slider (`mouseup` no document) não deixa flag `_dragging` presa
- [ ] Volume persiste após reiniciar o engine (preferências salvas em `preferences.json`)
