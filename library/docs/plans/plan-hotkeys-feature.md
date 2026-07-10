# Plano: Feature de Botoneira (Hotkeys)

## Visão geral

Permitir que o operador monte perfis de botoneiras com botões de atalho para
disparar áudios instantaneamente pelo **cart player** — um terceiro canal de
áudio dedicado, completamente isolado do pipeline principal e do canal de preview.

Os dados de perfis e botões são persistidos no **library-service**. A UI é uma
janela Electron separada (`hotkeys.html`) que o operador pode mover para outro
monitor. Múltiplas janelas de botoneira podem ser abertas simultaneamente.

---

## Pipeline de áudio — três canais independentes

```
Main Pipeline:  Queue → Decoder A → Mixer → Output (main device)    ← on air
Preview/CUE:    File  → Decoder B →         Output (preview device)  ← auditória
Cart Player:    File  → Decoder C →         Output (cart device)     ← hotkeys  ← NOVO
```

- 1 cart por vez. Disparar novo cart enquanto um está ativo para o anterior
  e inicia o novo imediatamente ("replace" semântico).
- O cart device é configurado independentemente do main e do preview.

---

## Camadas envolvidas

| Camada | O que muda |
|---|---|
| `playout/` — Cart Player | Novo subsistema: config, package, comandos, eventos, API, volume, config UI |
| `library/` — banco + API | Novas tabelas e endpoints para perfis e botões |
| `player/main.js` | Set de janelas Electron (múltiplas simultâneas) |
| `player/hotkeys.html` | UI da botoneira; chama `/v1/cart/play` no engine |
| `player/player.html` | Botão "Botoneira" + slider volume cart + eventos Cart* |

---

## Fase 1 — Playout Engine: Cart Player

### 1.1 — MODIFICAR `playout/internal/config/config.go`

Adicionar `Cart CartConfig` à struct `Config`:

```go
type CartConfig struct {
    Enabled bool         `yaml:"enabled" json:"enabled"`
    Output  OutputConfig `yaml:"output"  json:"output"`
}
```

Exemplo em YAML:
```yaml
cart:
  enabled: true
  output:
    device_id: "default"
```

> O volume do canal cart começa sempre em 100% e é ajustável em tempo real
> via `PUT /v1/cart/volume` no player. Não há configuração persistida em YAML.

### 1.2 — CRIAR `playout/internal/cart/player.go`

Modelado em `internal/preview/player.go` — mesma estrutura de state machine,
mesmo padrão de goroutine com `cmdCh` / `intCh`. Diferenças:

- Gera `cart_id` (ULID) a cada disparo.
- Se já há cart tocando: para o anterior (`CartStopped` com `reason: "replaced"`),
  inicia o novo imediatamente.
- Publica `CartStarted`, `CartProgress`, `CartStopped` no event bus.

```go
type Player struct {
    evtBus    *events.Bus
    dec       decoder.Decoder
    out       output.OutputDevice
    vol       *atomic.Uint32
    log       *slog.Logger

    mu     sync.RWMutex
    cartID string
    status Status

    cmdCh chan extCmd
    intCh chan intMsg
}

type Status struct {
    CartID     string
    Path       string
    Title      string
    Artist     string
    State      string  // "idle" | "playing" | "stopping"
    PositionMS int64
    DurationMS int64
}
```

### 1.3 — MODIFICAR `playout/internal/events/types.go`

```go
CartStarted      EventType = "CartStarted"
CartProgress     EventType = "CartProgress"
CartStopped      EventType = "CartStopped"
CartVolumeChanged EventType = "CartVolumeChanged"
```

Payloads:
```go
type CartStartedPayload  struct { CartID string; Path, Title, Artist string; DurationMS int64 }
type CartProgressPayload struct { CartID string; PositionMS, DurationMS int64 }
type CartStoppedPayload  struct { CartID string; Reason string } // "finished"|"manual"|"replaced"
```

### 1.4 — MODIFICAR `playout/internal/commands/types.go`

```go
CmdCartPlay      CommandType = "cart_play"
CmdCartStop      CommandType = "cart_stop"
CmdCartSetVolume CommandType = "cart_set_volume"
```

### 1.5 — MODIFICAR `playout/internal/state/manager.go`

Adicionar `CartVolume()` e `SetCartVolume()` — mesmo padrão de `PreviewVolume()`.
Adicionar ao `StateSnapshot`:

```go
CartVolume  float32 `json:"cart_volume"`
CartEnabled bool    `json:"cart_enabled"`
```

### 1.6 — CRIAR `playout/internal/api/handlers/cart.go`

| Handler | Endpoint | Descrição |
|---|---|---|
| `CartPlay` | `POST /v1/cart/play` | Dispara cart |
| `CartStop` | `DELETE /v1/cart` | Para o cart ativo |
| `CartStatus` | `GET /v1/cart` | Status atual |

Se `cart.enabled: false`, retorna `503` (mesmo padrão do preview).

### 1.7 — MODIFICAR `playout/internal/api/handlers/volume.go`

Adicionar `GetCartVolume` / `SetCartVolume` seguindo o padrão exato de
`GetPreviewVolume` / `SetPreviewVolume`:

```go
// GET /v1/cart/volume
func GetCartVolume(stateMgr *state.Manager, enabled bool) http.HandlerFunc { ... }

// PUT /v1/cart/volume  — body: {"level": 0.85}
func SetCartVolume(bus queueBus, stateMgr *state.Manager, enabled bool) http.HandlerFunc { ... }
```

### 1.8 — MODIFICAR `playout/internal/api/server.go`

```go
mux.HandleFunc("POST /v1/cart/play",   handlers.CartPlay(cartPlayer))
mux.HandleFunc("DELETE /v1/cart",      handlers.CartStop(cartPlayer))
mux.HandleFunc("GET /v1/cart",         handlers.CartStatus(cartPlayer))
mux.HandleFunc("GET /v1/cart/volume",  handlers.GetCartVolume(stateMgr, cfg.Cart.Enabled))
mux.HandleFunc("PUT /v1/cart/volume",  handlers.SetCartVolume(bus, stateMgr, cfg.Cart.Enabled))
```

### 1.9 — MODIFICAR `playout/internal/api/handlers/config_html.go`

#### Nav item

Adicionar após `data-s="preview"`, antes de `data-s="scheduler"`:

```html
<div class="nav-item" data-s="cart">Cart</div>
```

#### Painel `#p-cart`

Adicionar após `#p-preview`, antes de `#p-scheduler`, espelhando exatamente a
estrutura do painel Preview:

```html
<div id="p-cart" class="panel">
  <div class="section-title">Cart Player</div>

  <label class="check-row">
    <input id="cart-enabled" type="checkbox" />
    <div>
      <div class="check-lbl">Habilitar cart player</div>
      <div class="check-desc">
        Permite disparar áudios instantaneamente a partir da botoneira, em
        dispositivo de saída dedicado, isolado do sinal ao ar e do canal de preview.
      </div>
    </div>
  </label>

  <div class="field" style="margin-top:16px">
    <label class="lbl">Dispositivo de saída (cart)</label>
    <select id="cart-device"></select>
    <div class="hint">
      Deve ser diferente do dispositivo principal e do dispositivo de preview.
      Vazio = padrão do driver.
    </div>
  </div>

</div>
```

#### JS — variável de estado

Adicionar junto a `audioDeviceID` e `prevDeviceID`:

```javascript
var cartDeviceID = '';
```

#### JS — carregamento (`loadConfig`)

Adicionar ao bloco que carrega `cfg.preview`:

```javascript
var ct = cfg.cart || {};
setCheck('cart-enabled', ct.enabled);
cartDeviceID = (ct.output || {}).device_id || '';
```

#### JS — população de devices (`loadDevices`)

Adicionar à chamada de `populateDeviceSelect`:

```javascript
populateDeviceSelect('cart-device', devs, cartDeviceID);
```

#### JS — salvamento (`saveCfg`)

Adicionar ao objeto enviado no `PUT /v1/config`:

```javascript
cart: {
  enabled: getCheck('cart-enabled'),
  output:  { device_id: getVal('cart-device') || 'default' }
}
```

---

### 1.10 — MODIFICAR `cmd/playout-engine/engine/firstrun.go`

Adicionar a seção `cart:` ao template YAML gerado na primeira execução
(`defaultConfig()`), após a seção `scheduler:`, seguindo o mesmo estilo
de comentários das demais seções:

```yaml
# -----------------------------------------------------------------------------
# Cart Player (botoneira)
# -----------------------------------------------------------------------------
cart:
  # Habilita o cart player. Quando false, os endpoints /v1/cart/*
  # retornam 503 Service Unavailable.
  enabled: false

  # Dispositivo de saída dedicado para o cart player.
  # Deve ser diferente do dispositivo principal e do dispositivo de preview.
  # Vazio = dispositivo padrão do driver.
  # Exemplos: "BlackHole 2ch" (macOS), "hw:1,0" (ALSA/Linux)
  output:
    device_id: ""
```

### 1.11 — MODIFICAR `cmd/playout-engine/main.go`

Instanciar `cart.Player` com seu próprio decoder e output device, passá-lo ao
API server e ao dispatcher — idêntico ao padrão do preview player.

---

## Fase 2 — Library Service

### 2.1 — Migration `003_hotkeys`

```sql
CREATE TABLE IF NOT EXISTS hotkey_profiles (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    columns    INTEGER NOT NULL DEFAULT 4,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS hotkey_buttons (
    id           TEXT PRIMARY KEY,
    profile_id   TEXT NOT NULL REFERENCES hotkey_profiles(id) ON DELETE CASCADE,
    position     INTEGER NOT NULL DEFAULT 0,
    label        TEXT NOT NULL DEFAULT '',
    sub_label    TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    palette      INTEGER NOT NULL DEFAULT 0,
    track_id     TEXT REFERENCES tracks(id) ON DELETE SET NULL,
    track_path   TEXT NOT NULL DEFAULT '',
    track_title  TEXT NOT NULL DEFAULT '',
    track_artist TEXT NOT NULL DEFAULT '',
    track_type   TEXT NOT NULL DEFAULT '',
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_hotkey_buttons_profile
    ON hotkey_buttons(profile_id, position);
```

### 2.2 — CRIAR `internal/store/hotkey_store.go`

Tipos `HotkeyProfile`, `HotkeyButton`, `HotkeyButtonPatch` e métodos:
`ListProfiles`, `CreateProfile`, `FindProfileByID`, `UpdateProfile`,
`DeleteProfile`, `AddButton`, `PatchButton`, `DeleteButton`, `ReorderButtons`.

### 2.3 — CRIAR `internal/api/handlers/hotkeys.go` + `hotkeys_test.go`

| Endpoint | Descrição |
|---|---|
| `GET /v1/hotkeys/profiles` | Lista perfis (sem botões) |
| `POST /v1/hotkeys/profiles` | Cria perfil |
| `GET /v1/hotkeys/profiles/{id}` | Perfil completo com botões |
| `PUT /v1/hotkeys/profiles/{id}` | Atualiza nome/colunas |
| `DELETE /v1/hotkeys/profiles/{id}` | Remove perfil (CASCADE botões) |
| `POST /v1/hotkeys/profiles/{id}/buttons` | Adiciona botão |
| `PUT /v1/hotkeys/profiles/{id}/buttons/reorder` | Reordena botões |
| `PATCH /v1/hotkeys/buttons/{id}` | Edita botão |
| `DELETE /v1/hotkeys/buttons/{id}` | Remove botão |

### 2.4 — MODIFICAR `internal/api/server.go` e `cmd/library-service/main.go`

Registrar rotas e instanciar `HotkeyStore`.

---

## Fase 3 — Player

### 3.1 — MODIFICAR `player/main.js`

```javascript
const hotkeyWindows = new Set();

ipcMain.on('open-hotkeys', () => {
  const win = new BrowserWindow({
    width: 520, height: 680, minWidth: 380, minHeight: 400,
    resizable: true, frame: true,
    title: 'RadioFlow — Botoneira',
    webPreferences: { nodeIntegration: true }
  });
  win.loadFile('hotkeys.html');
  hotkeyWindows.add(win);
  win.on('closed', () => hotkeyWindows.delete(win));
});
```

---

### 3.2 — MODIFICAR `player/player.html`

#### A — Botão para abrir a botoneira

Adicionar em `controls-group-right`, após `☰ Biblioteca` e antes do separador
`controls-sep` que precede os botões de modo:

```html
<!-- ANTES -->
<button class="btn btn-audios"  id="btnAudios"  onclick="advModalOpen()">♪ Catálogo</button>
<button class="btn btn-library" id="btnLibrary" onclick="toggleLibrary()">☰ Biblioteca</button>
<div class="controls-sep"></div>
<button class="btn btn-assist" ...>◎ ASSIST</button>

<!-- DEPOIS -->
<button class="btn btn-audios"   id="btnAudios"   onclick="advModalOpen()">♪ Catálogo</button>
<button class="btn btn-library"  id="btnLibrary"  onclick="toggleLibrary()">☰ Biblioteca</button>
<button class="btn btn-hotkeys"  id="btnHotkeys"  onclick="openHotkeys()">⌨ Botoneira</button>
<div class="controls-sep"></div>
<button class="btn btn-assist" ...>◎ ASSIST</button>
```

Estilo — mesma classe base `btn`, cor similar ao `btn-library` (azul/cinza escuro),
visualmente alinhado no grupo de botões de biblioteca:

```css
.btn-hotkeys {
  background: linear-gradient(180deg, #1e2a3a, #162030);
  border-color: rgba(100,160,255,.3);
  color: #7ab0ff;
}
.btn-hotkeys:hover { background: #1e3050; color: #acd0ff; }
```

Função JS:
```javascript
function openHotkeys() {
  if (typeof require !== 'undefined') {
    require('electron').ipcRenderer.send('open-hotkeys');
  } else {
    window.open('hotkeys.html', '_blank', 'width=520,height=680');
  }
}
```

O botão `#btnHotkeys` é desabilitado automaticamente se `cart_enabled: false`
no `StateSnapshot` (pois sem cart player a botoneira não reproduz áudio):

```javascript
// no handler de StateSnapshot
const cartEnabled = p.cart_enabled ?? true;
document.getElementById('btnHotkeys').disabled = !cartEnabled;
```

---

#### B — Slider de volume do cart

Adicionar na `vol-section` (coluna esquerda), após o `vol-row` do "Player (CUE)"
(`#volPreviewRow`), espelhando exatamente o padrão dos dois sliders existentes:

```html
<!-- Estrutura atual -->
<div class="vol-section">
  <div class="queue-title-label">Volume</div>

  <div class="vol-row"> <!-- Principal — já existe -->
    <div class="vol-label">Principal</div>
    <div class="vol-control">
      <input id="volMain" type="range" class="vol-slider" min="0" max="100" step="1" value="100"
             oninput="onVolInput('main', this)"    onchange="onVolChange('main', this)"
             onmousedown="_draggingMain=true"       onmouseup="_draggingMain=false"
             ontouchstart="_draggingMain=true"      ontouchend="_draggingMain=false">
      <span class="vol-pct" id="volMainPct">100%</span>
    </div>
  </div>

  <div class="vol-row" id="volPreviewRow"> <!-- Player (CUE) — já existe -->
    <div class="vol-label">Player (CUE)</div>
    <div class="vol-control">
      <input id="volPreview" type="range" class="vol-slider" min="0" max="100" step="1" value="100"
             oninput="onVolInput('preview', this)"    onchange="onVolChange('preview', this)"
             onmousedown="_draggingPreview=true"       onmouseup="_draggingPreview=false"
             ontouchstart="_draggingPreview=true"      ontouchend="_draggingPreview=false">
      <span class="vol-pct" id="volPreviewPct">100%</span>
    </div>
  </div>

  <!-- NOVO — Cart Player -->
  <div class="vol-row" id="volCartRow">
    <div class="vol-label">Cart</div>
    <div class="vol-control">
      <input id="volCart" type="range" class="vol-slider" min="0" max="100" step="1" value="100"
             oninput="onVolInput('cart', this)"    onchange="onVolChange('cart', this)"
             onmousedown="_draggingCart=true"       onmouseup="_draggingCart=false"
             ontouchstart="_draggingCart=true"      ontouchend="_draggingCart=false">
      <span class="vol-pct" id="volCartPct">100%</span>
    </div>
  </div>
</div>
```

**JS — variável de drag:**
```javascript
var _draggingCart = false;
```

**JS — endpoint no `onVolInput` / `onVolChange`:**

Estender a função existente `onVolInput(channel, el)` para reconhecer `'cart'`:
```javascript
// channel: 'main' | 'preview' | 'cart'
const endpoint = channel === 'main'    ? '/v1/playback/volume'
               : channel === 'preview' ? '/v1/preview/volume'
               :                         '/v1/cart/volume';
```

**JS — inicialização via `StateSnapshot`:**
```javascript
// já existente (trecho resumido)
const mv = p.MainVolume ?? p.main_volume;
const pv = p.PreviewVolume ?? p.preview_volume;
const cv = p.CartVolume  ?? p.cart_volume;      // NOVO

if (mv != null) setVolDisplay('main',    mv);
if (pv != null) setVolDisplay('preview', pv);
if (cv != null) setVolDisplay('cart',    cv);   // NOVO

// desabilitar slider se cart não habilitado
const cartEnabled = p.cart_enabled ?? true;
document.getElementById('volCartRow').style.opacity = cartEnabled ? '' : '0.35';
document.getElementById('volCart').disabled = !cartEnabled;
```

**JS — fallback `fetchInitialVolumes`:**
```javascript
// já existente
fetch(API_URL + '/v1/playback/volume'),
fetch(API_URL + '/v1/preview/volume'),
fetch(API_URL + '/v1/cart/volume'),    // NOVO — ignora 503 se cart disabled
```

**JS — evento WebSocket `CartVolumeChanged`:**
```javascript
case 'CartVolumeChanged':
  if (!_draggingCart) setVolDisplay('cart', payload.level);
  break;
```

**JS — `setVolDisplay` para 'cart':**

Estender a função existente para aceitar `'cart'`:
```javascript
function setVolDisplay(channel, level) {
  const pct = Math.round(level * 100);
  if (channel === 'main') {
    document.getElementById('volMain').value   = pct;
    document.getElementById('volMainPct').textContent = pct + '%';
  } else if (channel === 'preview') {
    document.getElementById('volPreview').value = pct;
    document.getElementById('volPreviewPct').textContent = pct + '%';
  } else if (channel === 'cart') {                          // NOVO
    document.getElementById('volCart').value    = pct;
    document.getElementById('volCartPct').textContent = pct + '%';
  }
}
```

---

### 3.3 — CRIAR `player/hotkeys.html`

Baseado no prototype `player/docs/hotkeys-preview.html`, integrado às APIs:

**Fontes de dados:**
- Perfis e botões: library-service `GET /v1/hotkeys/profiles`
- Busca de track no modal: library-service `GET /v1/tracks?q=...&type=...`
- Disparar cart: playout engine `POST /v1/cart/play`
- Parar cart: playout engine `DELETE /v1/cart`
- Eventos em tempo real: WebSocket `ws://...8080/v1/events`

**URLs via query string:**
```
hotkeys.html?api=http://...8080&lib=http://...8081
```

**Header:**
```
[ Perfil: Manhã ▾ ] [ 4 col ▾ ] [✎ Editar] [+ Perfil] [↗ Nova janela]
```

**Botão "↗ Nova janela":**
```javascript
function openNewHotkeyWindow() {
  if (typeof require !== 'undefined') {
    require('electron').ipcRenderer.send('open-hotkeys');
  } else {
    window.open('hotkeys.html', '_blank', 'width=520,height=680');
  }
}
```

**Fluxo de disparo:**
1. Clique → `POST /v1/cart/play { path, title, artist }`
2. `CartStarted` → botão entra em animação "playing"
3. `CartProgress` → barra de progresso do botão atualiza
4. `CartStopped` → botão volta ao estado normal

---

## Arquivos modificados — resumo

| Fase | Arquivo | Ação |
|---|---|---|
| 1 | `playout/internal/config/config.go` | Adicionar `CartConfig` |
| 1 | `playout/internal/config/loader.go` | Default + env vars do cart |
| 1 | `playout/cmd/playout-engine/engine/firstrun.go` | Seção `cart:` no YAML gerado no first-run |
| 1 | `playout/internal/cart/player.go` | CRIAR |
| 1 | `playout/internal/events/types.go` | `CartStarted/Progress/Stopped/VolumeChanged` |
| 1 | `playout/internal/commands/types.go` | `CmdCartPlay/Stop/SetVolume` |
| 1 | `playout/internal/state/manager.go` | `CartVolume()`, campos no `StateSnapshot` |
| 1 | `playout/internal/api/handlers/cart.go` | CRIAR |
| 1 | `playout/internal/api/handlers/volume.go` | `GetCartVolume`, `SetCartVolume` |
| 1 | `playout/internal/api/server.go` | Rotas `/v1/cart/...` |
| 1 | `playout/internal/api/handlers/config_html.go` | Nav "Cart" + painel `#p-cart` + JS |
| 1 | `playout/cmd/playout-engine/main.go` | Instanciar `cart.Player` |
| 2 | `library/internal/store/db.go` | Migration `003_hotkeys` |
| 2 | `library/internal/store/hotkey_store.go` | CRIAR |
| 2 | `library/internal/api/handlers/hotkeys.go` | CRIAR |
| 2 | `library/internal/api/handlers/hotkeys_test.go` | CRIAR |
| 2 | `library/internal/api/server.go` | Rotas `/v1/hotkeys/...` |
| 2 | `library/cmd/library-service/main.go` | Instanciar `HotkeyStore` |
| 3 | `player/main.js` | Set de janelas + ipcMain |
| 3 | `player/player.html` | Botão `⌨ Botoneira` + slider `Cart` + eventos `Cart*` |
| 3 | `player/hotkeys.html` | CRIAR |

---

## Ordem de implementação

1. **Criar branch** a partir de `main`:
   ```bash
   git checkout main
   git pull
   git checkout -b feature/hotkeys
   ```
2. `CartConfig` no config.go
3. `cart.Player` + eventos + comandos
4. Handlers de cart + volume + state manager
5. Rotas em server.go
6. Seção Cart na config UI (`config_html.go`)
7. `main.go` do playout — instanciar cart player
8. `go test ./... && go vet ./...` no playout
9. Migration 003 + `HotkeyStore` no library-service
10. Handlers de hotkeys + rotas + instância em main.go
11. `go test ./... && go vet ./...` no library
12. `player/main.js`
13. `player/player.html` — botão Botoneira + slider Cart
14. `player/hotkeys.html`

---

## Verificação

```bash
# Playout
cd playout && go test ./... && go vet ./...
curl -s -X POST http://localhost:8080/v1/cart/play \
  -H 'Content-Type: application/json' \
  -d '{"path":"/audio/abertura.mp3","title":"Abertura","artist":""}' | jq .
curl -s http://localhost:8080/v1/cart | jq .
curl -s http://localhost:8080/v1/cart/volume | jq .
curl -s -X PUT http://localhost:8080/v1/cart/volume -H 'Content-Type: application/json' \
  -d '{"level":0.8}' | jq .
curl -s -X DELETE http://localhost:8080/v1/cart

# Library
cd library && go test ./... && go vet ./...
curl -s -X POST http://localhost:8081/v1/hotkeys/profiles \
  -H 'Content-Type: application/json' -d '{"name":"Manhã","columns":4}' | jq .
curl -s http://localhost:8081/v1/hotkeys/profiles | jq .
```
