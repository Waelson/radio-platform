# Plano: Redesign do NowPlaying (PlayNow)

## Objetivo

Substituir o layout atual do "Tocando Agora" (np5-card, coluna lateral de metadados + waveform VU + coluna de restante) pelo novo design da imagem de referência:

- Top bar com badge de status, nome do player e modo
- Título e artista em destaque
- Linha de metadados inline (Início, Cue, Outro, Gain, ISRC)
- Waveform estático com marcadores de cue-in/outro e playhead
- Barra de progresso
- Linha de tempo: elapsed | countdown central grande | total
- Controles redesenhados: 5 botões de largura uniforme

---

## Análise do Layout Atual vs Novo

### Atual (np5-card — 3 colunas)
```
[meta sidebar 260px] | [waveCanvas VU + title/artist] | [remaining col]
[progress bar: pos / total / fill / percent]
---
[Play] [Pause] [Stop] [Skip] | [Catálogo] [Lib] [Botoneira] [Assist] [Panic]
```
- Waveform: oscilloscope ao vivo (RMS em tempo real via `wavePush`)
- Controles utilitários espalhados: catálogo, biblioteca, botoneira, assist, panic

### Novo (playnow-card — layout vertical)
```
[● TOCANDO AGORA]   Player A · Saída principal   [LIVE ASSIST]
MÚSICA · B - RECURRENT
Adventure of a Lifetime
Coldplay
Início 16:19:08  Cue 00:00.320  Outro 00:12.000  Gain -1.3 dB  ISRC FR-UM7-11-00321
[===========waveform canvas (barras ciano + markers amarelos + playhead branco)===========]
[=============================== progress bar ==========================================]
00:40                          -03:43                                              04:23
---
[Voltar]   [Pausar]   [▶ NO AR]   [Próximo]   [⚠ Emergência]
```
- Waveform: estático (peaks pré-analisados do arquivo), com playhead atualizado via progresso
- Controles: 5 botões de largura igual, sem ícones separados de utilitários no meio

---

## Campos Disponíveis vs Faltantes

### Payload atual `NowPlayingChangedPayload` (engine)
| Campo         | Status | Uso no novo layout |
|---------------|--------|--------------------|
| `path`        | ✓ Presente | Carregar waveform estático |
| `title`       | ✓ Presente | Título grande |
| `artist`      | ✓ Presente | Artista |
| `type`        | ✓ Presente | Parte do label de categoria |
| `duration_ms` | ✓ Presente | Tempo total |
| `isrc`        | ✓ Presente | Metadado inline |
| `cue_in_ms`   | ✗ Ausente | Marcador no waveform + "Cue" inline |
| `outro_ms`    | ✗ Ausente | Marcador no waveform + "Outro" inline |
| `gain_db`     | ✗ Ausente | Metadado "Gain" inline |
| `intro_ms`    | ✗ Ausente | (opcional para futuro) |
| `category`    | ✗ Ausente | Label "B - RECURRENT" (virá do sistema de rotação) |

### Payload `ProgressChangedPayload` — sem mudanças necessárias
Já tem `position_ms`, `remaining_ms`, `percent` — suficiente para countdown e progress fill.

---

## Etapas de Implementação

### Fase 1 — Backend: Enriquecer `NowPlayingChangedPayload`

**Arquivo:** `playout/internal/events/types.go`

Adicionar campos ao struct `NowPlayingChangedPayload`:
```go
CueInMS  int64   `json:"cue_in_ms,omitempty"`
IntroMS  int64   `json:"intro_ms,omitempty"`
OutroMS  int64   `json:"outro_ms,omitempty"`
GainDB   float64 `json:"gain_db,omitempty"`
```

**Arquivo:** onde o evento `NowPlayingChanged` é publicado (provavelmente `playout/internal/playback/manager.go`)
- Mapear os campos `cue_in_ms`, `intro_ms`, `outro_ms`, `gain_db` do `QueueItem` para o payload ao emitir o evento.

> **Nota:** O campo `category` (ex: "B - RECURRENT") depende do sistema de rotação e não está disponível ainda na pipeline do engine. Será exibido como `—` por enquanto, ou inferido do `type`.

---

### Fase 2 — HTML: Substituir `.nowplaying-bar` e `.controls-bar`

**Arquivo:** `player/player.html`

**2a. Remover:**
- Todo o bloco `.np5-card` (do `<!-- Now Playing -->` até `</div><!-- end np5-card -->`)
- Todo o bloco `.controls-bar`

**2b. Inserir `.playnow-wrap`:**

```html
<!-- Now Playing (novo layout) -->
<div class="playnow-wrap" id="playnowWrap">

  <!-- Top bar -->
  <div class="pn-topbar">
    <span class="pn-live-badge" id="pnLiveBadge">
      <span class="pn-dot"></span>TOCANDO AGORA
    </span>
    <span class="pn-source" id="pnSource">Player A · Saída principal</span>
    <span class="pn-mode-badge" id="pnModeBadge" style="display:none">LIVE ASSIST</span>
  </div>

  <!-- Metadados e título -->
  <div class="pn-content">
    <div class="pn-category" id="pnCategory">—</div>
    <div class="pn-title"    id="npTitle">—</div>
    <div class="pn-artist"   id="npArtist"></div>
    <div class="pn-meta-row">
      <span class="pn-meta-item"><span class="pn-meta-label">Início</span> <span id="pnInicio">—</span></span>
      <span class="pn-meta-sep">·</span>
      <span class="pn-meta-item"><span class="pn-meta-label">Cue</span> <span id="pnCue">—</span></span>
      <span class="pn-meta-sep">·</span>
      <span class="pn-meta-item"><span class="pn-meta-label">Outro</span> <span id="pnOutro">—</span></span>
      <span class="pn-meta-sep">·</span>
      <span class="pn-meta-item"><span class="pn-meta-label">Gain</span> <span id="pnGain">—</span></span>
      <span class="pn-meta-sep">·</span>
      <span class="pn-meta-item"><span class="pn-meta-label">ISRC</span> <span id="pnISRC">—</span></span>
    </div>
  </div>

  <!-- Waveform estático -->
  <div class="pn-wave-wrap">
    <canvas class="pn-wave-canvas" id="pnWaveCanvas"></canvas>
  </div>

  <!-- Progress bar -->
  <div class="pn-progress-wrap">
    <div class="pn-progress-track">
      <div class="pn-progress-fill" id="pnProgressFill" style="width:0%"></div>
    </div>
  </div>

  <!-- Linha de tempo -->
  <div class="pn-times">
    <span class="pn-time-side" id="pnElapsed">00:00</span>
    <span class="pn-time-countdown" id="pnCountdown">-00:00</span>
    <span class="pn-time-side" id="npTotal">00:00</span>
  </div>

</div>

<!-- Controles (novo layout) -->
<div class="pn-controls">
  <button class="pn-btn"              id="btnBack"        onclick="sendCmd('back')">↩ Voltar</button>
  <button class="pn-btn"              id="btnPauseToggle" onclick="sendCmd('pause-toggle')">⏸ Pausar</button>
  <button class="pn-btn pn-btn-primary" id="btnPlay"     onclick="sendCmd('play')">▶ NO AR</button>
  <button class="pn-btn"              id="btnSkip"        onclick="sendCmd('skip')">Próximo ↪</button>
  <button class="pn-btn pn-btn-danger"  id="btnPanicToggle" onclick="sendCmd('panic-toggle')">⚠ Emergência</button>
</div>
```

> Os botões utilitários (catálogo, biblioteca, botoneira, assist, panic) serão reposicionados para a barra de topo global ou permanecem acessíveis via atalhos/drawer — a definir em plano futuro.

---

### Fase 3 — CSS: Novo bloco `.playnow-*` / `.pn-*`

**Remover:** Todo o bloco CSS `/* ─── Now Playing v5 ─────── */` (`.np5-*`)

**Adicionar:**

```css
/* ─── PlayNow Card ────────────────────────────────────── */
.playnow-wrap {
  background: #090f14;
  border-bottom: 1px solid rgba(43,216,222,0.15);
  padding: 14px 16px 0;
  flex-shrink: 0;
}

/* Top bar */
.pn-topbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}
.pn-live-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.08em;
  color: #ff4455;
  border: 1px solid rgba(255,68,85,0.5);
  border-radius: 5px;
  padding: 3px 8px;
}
.pn-dot {
  width: 7px; height: 7px;
  border-radius: 50%;
  background: #ff4455;
  animation: blink 1s step-start infinite;
}
.pn-live-badge.idle { color: var(--text-dim); border-color: var(--border); }
.pn-live-badge.idle .pn-dot { background: var(--text-dim); animation: none; }
.pn-source {
  font-size: 12px;
  color: var(--text-dim);
  flex: 1;
}
.pn-mode-badge {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: #27ae60;
  border: 1px solid rgba(39,174,96,0.5);
  border-radius: 5px;
  padding: 3px 8px;
}

/* Título / artista / categoria */
.pn-content { padding-bottom: 10px; }
.pn-category {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.1em;
  color: var(--cyan);
  text-transform: uppercase;
  margin-bottom: 4px;
}
.pn-title {
  font-size: 28px;
  font-weight: 800;
  color: #fff;
  line-height: 1.1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.pn-artist {
  font-size: 16px;
  color: rgba(255,255,255,0.75);
  margin-top: 2px;
  margin-bottom: 8px;
}
.pn-meta-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px 10px;
  font-size: 11px;
}
.pn-meta-label { color: var(--text-dim); }
.pn-meta-item  { color: #fff; }
.pn-meta-sep   { color: var(--border); }

/* Waveform */
.pn-wave-wrap {
  position: relative;
  height: 80px;
  border-radius: 4px;
  overflow: hidden;
  background: #0a1520;
  margin: 0 -16px;
}
.pn-wave-canvas {
  display: block;
  width: 100%;
  height: 100%;
}

/* Progress */
.pn-progress-wrap { padding: 0; margin: 0 -16px; }
.pn-progress-track {
  height: 3px;
  background: rgba(43,216,222,0.15);
}
.pn-progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #4fc3f7, #20e6ff);
  transition: width 0.25s linear;
}

/* Linha de tempo */
.pn-times {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 0 10px;
}
.pn-time-side { font-size: 11px; color: var(--text-dim); }
.pn-time-countdown {
  font-size: 38px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
  line-height: 1;
}

/* Controles */
.pn-controls {
  display: flex;
  gap: 8px;
  padding: 10px 16px 12px;
  flex-shrink: 0;
}
.pn-btn {
  flex: 1;
  height: 48px;
  border-radius: 10px;
  border: 1px solid rgba(255,255,255,0.12);
  background: #0f1e28;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s;
}
.pn-btn:hover { background: #1a2e3a; }
.pn-btn-primary {
  background: linear-gradient(135deg, #1ab2cc, #20e6ff);
  border-color: transparent;
  color: #000;
  font-weight: 800;
  font-size: 14px;
  flex: 1.4;
}
.pn-btn-primary:hover { background: linear-gradient(135deg, #20c6e0, #40f0ff); }
.pn-btn-danger {
  border-color: rgba(220,50,50,0.5);
  color: var(--red);
}
.pn-btn-danger:hover { background: rgba(220,50,50,0.1); }
```

---

### Fase 4 — JS: Waveform Estático com Playhead e Marcadores

**Remover:** Loop `waveRenderLoop` (oscilloscope ao vivo), `wavePush`, `waveReset`, `waveBuffer`.

**Adicionar:** Sistema de waveform estático para o `pnWaveCanvas`:

```js
// Estado do waveform do nowplaying
let _pnWavePeaks   = [];
let _pnWaveDurMS   = 0;
let _pnWaveCueInMS = null;
let _pnWaveOutroMS = null;
let _pnWavePos     = 0;  // 0..1, atualizado por onProgress

async function pnLoadWaveform(filePath, durationMS, cueInMS, outroMS) {
  _pnWavePeaks   = [];
  _pnWaveDurMS   = durationMS;
  _pnWaveCueInMS = cueInMS;
  _pnWaveOutroMS = outroMS;
  _pnWavePos     = 0;
  pnDrawWaveform();
  // Carregar peaks via Web Audio API (mesmo algoritmo do cue editor)
  // ... (reutilizar _loadCueWaveform como base)
}

function pnDrawWaveform() {
  const canvas = document.getElementById('pnWaveCanvas');
  // ... desenhar barras ciano, linha de playhead branca, marcadores amarelos
}
```

**A função `pnDrawWaveform` deve:**
1. Fundo: `#0a1520`
2. Barras: `rgba(32,230,255,0.7)` (ciano), centradas verticalmente
3. Linha branca vertical na posição `_pnWavePos * canvasWidth`
4. Linha amarela `#ffb300` na posição `cueInMS / durationMS * W`
5. Linha amarela `#ffb300` na posição `outroMS / durationMS * W`
6. Barras antes do cue-in: levemente dimmed (`rgba(32,230,255,0.3)`)

---

### Fase 5 — JS: Atualizar `onNowPlaying` e `onProgress`

**`onNowPlaying(p)` — novos campos a popular:**
```js
// Categoria
document.getElementById('pnCategory').textContent =
  [typeLabel(p.type), p.category].filter(Boolean).join(' · ') || '—';

// Título e artista (mantidos nos mesmos IDs: npTitle, npArtist)

// Metadados inline
document.getElementById('pnCue').textContent    = p.cue_in_ms != null ? msToMMSSmmm(p.cue_in_ms) : '—';
document.getElementById('pnOutro').textContent  = p.outro_ms  != null ? msToMMSSmmm(p.outro_ms)  : '—';
document.getElementById('pnGain').textContent   = p.gain_db   != null ? p.gain_db.toFixed(1) + ' dB' : '—';
document.getElementById('pnISRC').textContent   = p.isrc || '—';

// Hora de início (calculada: agora - position_ms)
// Atualizar pnInicio ao receber o primeiro progress

// Carregar waveform estático
if (p.path) pnLoadWaveform(p.path, p.duration_ms, p.cue_in_ms, p.outro_ms);
```

**`onProgress(p)` — atualizar countdown e waveform playhead:**
```js
// Countdown negativo centralizado
const remS = Math.ceil(p.remaining_ms / 1000);
document.getElementById('pnCountdown').textContent = '-' + msToTime2(remS * 1000);

// Elapsed (esquerda)
document.getElementById('pnElapsed').textContent = msToTime2(p.position_ms);

// Progress fill
document.getElementById('pnProgressFill').style.width = (p.percent ?? 0) + '%';

// Playhead no waveform
_pnWavePos = (p.percent ?? 0) / 100;
pnDrawWaveform();

// Hora de início (somente na primeira atualização após NowPlaying)
if (!_pnInicioSet) {
  const now = new Date(Date.now() - p.position_ms);
  document.getElementById('pnInicio').textContent = now.toLocaleTimeString('pt-BR', {hour:'2-digit', minute:'2-digit', second:'2-digit'});
  _pnInicioSet = true;
}
```

---

### Fase 6 — JS: Top Bar (badge TOCANDO AGORA e modo)

**`setStateBadge(state, mode)`:**
```js
const badge = document.getElementById('pnLiveBadge');
const modeBadge = document.getElementById('pnModeBadge');
const isPlaying = state === 'PLAYING' || state === 'ASSIST';
badge.className = 'pn-live-badge' + (isPlaying ? '' : ' idle');

// Modo badge (LIVE ASSIST / AUTO / PANIC)
if (mode === 'ASSIST') {
  modeBadge.textContent = 'LIVE ASSIST';
  modeBadge.style.display = '';
} else if (state === 'PANIC') {
  modeBadge.textContent = 'PANIC';
  modeBadge.style.color = 'var(--red)';
  modeBadge.style.borderColor = 'rgba(220,50,50,0.5)';
  modeBadge.style.display = '';
} else {
  modeBadge.style.display = 'none';
}
```

**`syncPauseToggle(state)`** — adaptar para pn-btn:
```js
const btn = document.getElementById('btnPauseToggle');
const paused = state === 'PAUSED';
btn.textContent = paused ? '▶ Retomar' : '⏸ Pausar';
```

---

### Fase 7 — Botões Utilitários (reposicionamento)

Os botões atualmente na `.controls-bar` direita (Catálogo, Biblioteca, Botoneira, Assist, Return Auto) precisam ser reposicionados. Opções:

- **Opção A (recomendada):** Mover para a barra de topo global (`col-left` ou header fixo)
- **Opção B:** Adicionar uma segunda linha de controles abaixo dos 5 botões principais (compacta, com ícones)

Esta decisão é separada e pode ser tratada em plano próprio.

---

## Ordem de Execução

| # | Tarefa | Arquivo(s) |
|---|--------|------------|
| 1 | Adicionar `CueInMS`, `IntroMS`, `OutroMS`, `GainDB` ao `NowPlayingChangedPayload` | `playout/internal/events/types.go` |
| 2 | Popular campos novos ao publicar `NowPlayingChanged` | `playout/internal/playback/manager.go` |
| 3 | Substituir HTML do NowPlaying (`.np5-card` → `.playnow-wrap`) | `player/player.html` |
| 4 | Substituir HTML dos controles (`.controls-bar` → `.pn-controls`) | `player/player.html` |
| 5 | Remover CSS `.np5-*` e adicionar CSS `.pn-*` / `.playnow-*` | `player/player.html` |
| 6 | Remover waveform VU ao vivo, adicionar waveform estático `pnLoadWaveform` / `pnDrawWaveform` | `player/player.html` |
| 7 | Atualizar `onNowPlaying`, `onProgress`, `setStateBadge`, `syncPauseToggle` | `player/player.html` |
| 8 | Decidir e implementar reposicionamento dos botões utilitários | `player/player.html` |

---

## Dependências e Riscos

- **IDs preservados:** `npTitle`, `npArtist`, `npTotal` permanecem os mesmos para não quebrar referências em outros pontos do JS.
- **`waveCanvas` (oscilloscope):** O loop VU é usado para feedback visual de áudio ao vivo. Com a remoção, o feedback visual de volume passa a ser apenas pelo medidor de VU (se houver). Avaliar se é necessário manter o VU em outro lugar.
- **Comando `back`:** O botão "Voltar" envia `sendCmd('back')` — verificar se o engine já suporta este comando ou se precisa ser implementado.
- **Backup:** Cópia do `player.html` atual salva em `player/player.html.bak-playnow`.
