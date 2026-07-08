# Plano: Controle de volume individual para fila e preview

## Contexto

O engine não expõe controle de volume em runtime. O operador não consegue ajustar o nível
da fila principal nem do preview (CUE) sem reiniciar o processo. As preferências de volume
precisam sobreviver a reinicializações sem tocar no YAML de configuração estrutural.

---

## Estratégia

**Ganho de software** — multiplicação de cada sample float32 pelo escalar `v ∈ [0.0, 1.0]`
imediatamente antes de cada `output.Write()`. Não depende de driver de áudio nem de SO.

**Persistência** — arquivo `~/.radiocore/preferences.json`, separado do YAML de configuração.
O YAML nunca é modificado em runtime.

**IPC do subprocess CUE** — protocolo stdin já usa JSON newline-delimited. Um novo comando
`set_volume` é adicionado ao protocolo existente.

**Eventos WebSocket** — `EvtVolumeChanged` e `EvtPreviewVolumeChanged` notificam todos os
clientes conectados imediatamente após cada mudança. `EvtStateSnapshot` inclui os volumes
atuais para clientes que conectam após a inicialização.

---

## Novos endpoints

| Método | Path | Body | Descrição |
|---|---|---|---|
| `PUT /v1/playback/volume` | `{"level": 0.80}` | Volume da fila (0.0 – 1.0) |
| `PUT /v1/preview/volume`  | `{"level": 0.60}` | Volume do preview/CUE |
| `GET /v1/status`          | — | Retorna `main_volume` e `preview_volume` |

---

## Fase 1 — Fundação: preferências e state

Estabelece a base que todas as fases seguintes dependem. Sem side effects de áudio ou API.

### 1.1 — CRIAR `internal/prefs/prefs.go`

Novo pacote sem dependências internas:

```go
package prefs

type Preferences struct {
    MainVolume    float32 `json:"main_volume"`
    PreviewVolume float32 `json:"preview_volume"`
}

// DefaultPath → ~/.radiocore/preferences.json
func DefaultPath() string { ... }

// Load: retorna defaults (1.0, 1.0) se o arquivo não existir ou falhar.
func Load(path string) Preferences { ... }

// Save: escrita atômica via WriteFile(tmp) + Rename.
func Save(path string, p Preferences) error { ... }
```

### 1.2 — MODIFICAR `internal/events/types.go`

```go
EvtVolumeChanged        EventType = "VolumeChanged"
EvtPreviewVolumeChanged EventType = "PreviewVolumeChanged"

type VolumeChangedPayload struct {
    Level float32 `json:"level"`
}

type PreviewVolumeChangedPayload struct {
    Level float32 `json:"level"`
}
```

Ambos os eventos são **PriorityHigh** — não descartados sob backpressure.

### 1.3 — MODIFICAR `internal/commands/types.go`

```go
CmdSetVolume        CommandType = "SET_VOLUME"
CmdPreviewSetVolume CommandType = "PREVIEW_SET_VOLUME"

type SetVolumePayload struct {
    Level float32
}

type PreviewSetVolumePayload struct {
    Level float32
}
```

### 1.4 — MODIFICAR `internal/state/manager.go`

Campos atômicos — sem lock no hot path de áudio:

```go
type Manager struct {
    // ...existing...
    mainVol    atomic.Uint32 // bits de float32
    previewVol atomic.Uint32
}

// Inicializar com 1.0 em NewManager.
// Métodos: SetMainVolume, MainVolume, SetPreviewVolume, PreviewVolume.
```

`Snapshot` ganha dois campos:

```go
type Snapshot struct {
    // ...existing...
    MainVolume    float32 `json:"main_volume"`
    PreviewVolume float32 `json:"preview_volume"`
}
```

### 1.5 — MODIFICAR `cmd/playout-engine/main.go`

Carregar preferências na inicialização e propagar ao state:

```go
prefsPath := prefs.DefaultPath()
p := prefs.Load(prefsPath)
stateMgr.SetMainVolume(p.MainVolume)
stateMgr.SetPreviewVolume(p.PreviewVolume)
// passar prefsPath ao dispatcher e ao cue.Proxy
```

---

## Fase 2 — Pipeline de áudio: aplicação do ganho

Aplica o volume nos pontos de escrita de áudio. Depende da Fase 1 (state com volume).

### 2.1 — MODIFICAR `internal/playback/manager.go`

Helper sem alocação, retorno imediato em gain == 1.0:

```go
func applyGain(buf []float32, gain float32) {
    if gain == 1.0 {
        return
    }
    for i := range buf {
        buf[i] *= gain
    }
}
```

Aplicar nos 3 pontos de `m.out.Write()` (linhas 672, 903, 1477):

```go
applyGain(buf[:n*spf], m.stateMgr.MainVolume())
if _, werr := m.out.Write(ctx, buf[:n*spf]); werr != nil { ... }
```

### 2.2 — MODIFICAR `internal/cue/msg.go`

```go
type subCmd struct {
    // ...existing...
    Volume float32 `json:"volume,omitempty"` // usado em set_volume
}
```

### 2.3 — MODIFICAR `internal/cue/runner.go`

Receber `initialVolume float32`, usar `atomic.Uint32` para atualizações em runtime:

```go
func RunCuePlayer(out output.OutputDevice, audioCfg preview.AudioConfig, initialVolume float32, log *slog.Logger) {
    var vol atomic.Uint32
    vol.Store(math.Float32bits(clamp01(initialVolume)))
    // ...loop stdin...
    case "set_volume":
        vol.Store(math.Float32bits(clamp01(msg.Volume)))
}
```

### 2.4 — MODIFICAR `internal/preview/player.go`

Receber `*atomic.Uint32` e aplicar ganho antes de cada write:

```go
type Player struct {
    // ...existing...
    vol *atomic.Uint32
}

// no loop:
gain := math.Float32frombits(p.vol.Load())
applyGain(buf[:n*p.channels], gain)
p.out.Write(ctx, buf[:n*p.channels])
```

---

## Fase 3 — Comandos e dispatcher

Liga a API ao pipeline de áudio. Depende das Fases 1 e 2.

### 3.1 — MODIFICAR `internal/playback/dispatcher.go`

Novo handler `HandleSetVolume`:

```go
func (d *Dispatcher) HandleSetVolume(ctx context.Context, cmd commands.Command) commands.Result {
    pl := cmd.Payload.(commands.SetVolumePayload)
    d.stateMgr.SetMainVolume(pl.Level)
    d.evtBus.Publish(events.New(events.EvtVolumeChanged, events.VolumeChangedPayload{Level: pl.Level}))
    _ = prefs.Save(d.prefsPath, prefs.Preferences{
        MainVolume:    pl.Level,
        PreviewVolume: d.stateMgr.PreviewVolume(),
    })
    return commands.Result{CommandID: cmd.ID, Accepted: true}
}
```

### 3.2 — MODIFICAR `internal/cue/proxy.go`

Novo handler `HandlePreviewSetVolume`:

```go
func (p *Proxy) HandlePreviewSetVolume(_ context.Context, cmd commands.Command) commands.Result {
    pl := cmd.Payload.(commands.PreviewSetVolumePayload)
    p.stateMgr.SetPreviewVolume(pl.Level)
    p.send(subCmd{Cmd: "set_volume", Volume: pl.Level})
    p.evtBus.Publish(events.New(events.EvtPreviewVolumeChanged, events.PreviewVolumeChangedPayload{Level: pl.Level}))
    _ = prefs.Save(p.prefsPath, prefs.Preferences{
        MainVolume:    p.stateMgr.MainVolume(),
        PreviewVolume: pl.Level,
    })
    return commands.Result{CommandID: cmd.ID, Accepted: true}
}
```

---

## Fase 4 — API HTTP

Expõe os endpoints REST. Depende da Fase 3.

### 4.1 — CRIAR `internal/api/handlers/volume.go`

```go
type volumeRequest struct {
    Level float32 `json:"level"`
}

func SetMainVolume(bus *commands.Bus) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req volumeRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Level < 0 || req.Level > 1 {
            http.Error(w, `{"ok":false,"error":"invalid_level"}`, http.StatusBadRequest)
            return
        }
        bus.Dispatch(commands.New(commands.CmdSetVolume, commands.SetVolumePayload{Level: req.Level}))
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{"ok":true}`))
    }
}

func SetPreviewVolume(bus *commands.Bus) http.HandlerFunc { ... } // mesmo padrão
```

### 4.2 — MODIFICAR `internal/api/server.go`

```go
mux.HandleFunc("PUT /v1/playback/volume", handlers.SetMainVolume(s.cmdBus))
mux.HandleFunc("PUT /v1/preview/volume",  handlers.SetPreviewVolume(s.cmdBus))
```

### 4.3 — MODIFICAR `internal/api/handlers/status.go`

`Snapshot` já terá `MainVolume` e `PreviewVolume` — incluir no response JSON de
`GET /v1/status`.

---

## Fase 5 — WebSocket e UI

Notificação em tempo real e exibição na página de status. Depende das Fases 3 e 4.

### 5.1 — MODIFICAR `internal/api/ws/hub.go`

Incluir `main_volume` e `preview_volume` no `EvtStateSnapshot` enviado a novos clientes:

```go
// no snapshot inicial enviado ao conectar:
snap := stateMgr.Snapshot()
// snap.MainVolume e snap.PreviewVolume já estão disponíveis
```

A UI inicializa os sliders a partir desse snapshot — sem chamada REST adicional.

### 5.2 — MODIFICAR `internal/api/handlers/status_html.go`

```html
<!-- no card Processo -->
<div class="row">
  <div class="k">Volume fila</div>
  <div id="mainVol" class="v">—</div>
</div>
<div class="row">
  <div class="k">Volume preview</div>
  <div id="previewVol" class="v">—</div>
</div>
```

```js
// em updateUI(status)
$('mainVol').textContent    = status.main_volume    != null ? Math.round(status.main_volume    * 100) + '%' : '—';
$('previewVol').textContent = status.preview_volume != null ? Math.round(status.preview_volume * 100) + '%' : '—';
```

---

## Fase 6 — Documentação

Atualiza toda a documentação pública afetada pela feature. Deve ser feita após as fases
anteriores estarem implementadas e testadas.

### 6.1 — `README.md`

- Adicionar seção ou linha na tabela de endpoints mencionando `PUT /v1/playback/volume`
  e `PUT /v1/preview/volume`
- Mencionar o arquivo `~/.radiocore/preferences.json` na seção de configuração

### 6.2 — `docs/specs/03-api-rest.md`

Adicionar os dois novos endpoints na especificação REST:

```
PUT /v1/playback/volume
  Body: {"level": 0.0–1.0}
  Response 200: {"ok": true}
  Response 400: {"ok": false, "error": "invalid_level"}

PUT /v1/preview/volume
  Body: {"level": 0.0–1.0}
  Response 200: {"ok": true}
  Response 400: {"ok": false, "error": "invalid_level"}
```

Adicionar `main_volume` e `preview_volume` na tabela de campos de `GET /v1/status`.

### 6.3 — `docs/specs/04-events-websocket.md`

Documentar os dois novos eventos:

| Evento | Prioridade | Payload |
|---|---|---|
| `VolumeChanged` | High | `{"level": 0.80}` |
| `PreviewVolumeChanged` | High | `{"level": 0.60}` |

Documentar que `EvtStateSnapshot` inclui `main_volume` e `preview_volume` no snapshot
inicial enviado a novos clientes WebSocket.

### 6.4 — `docs/specs/06-audio-pipeline.md`

Adicionar seção descrevendo o ganho de software:
- Ponto de aplicação: antes de cada `output.Write()`
- Escalar `v ∈ [0.0, 1.0]`; retorno imediato se `v == 1.0`
- Independente de driver de áudio e SO

### 6.5 — `docs/specs/12-configuration.md`

Documentar o arquivo de preferências:
- Localização: `~/.radiocore/preferences.json`
- Campos: `main_volume`, `preview_volume`
- Comportamento em primeira execução (defaults 1.0)
- Escrita atômica; falha não interrompe o engine

---

## Resumo por fase

| Fase | O que entrega | Arquivos |
|---|---|---|
| **1 — Fundação** | Preferências, eventos, comandos, state com volume | `prefs/prefs.go`, `events/types.go`, `commands/types.go`, `state/manager.go`, `main.go` |
| **2 — Áudio** | Ganho aplicado antes de cada Write; IPC CUE | `playback/manager.go`, `cue/msg.go`, `cue/runner.go`, `preview/player.go` |
| **3 — Comandos** | Dispatcher e proxy tratam os novos comandos, publicam eventos, salvam prefs | `playback/dispatcher.go`, `cue/proxy.go` |
| **4 — API** | Endpoints REST para ajuste de volume | `handlers/volume.go`, `api/server.go`, `handlers/status.go` |
| **5 — UI** | WebSocket snapshot + exibição na página de status | `ws/hub.go`, `handlers/status_html.go` |
| **6 — Docs** | README, specs REST, eventos, pipeline e configuração | `README.md`, `03-api-rest.md`, `04-events-websocket.md`, `06-audio-pipeline.md`, `12-configuration.md` |

---

## Arquivo de preferências — detalhes

| Aspecto | Decisão |
|---|---|
| Localização padrão | `~/.radiocore/preferences.json` |
| Configurável via | `engine.preferences_path` no YAML (opcional) |
| Escrita | Atômica: `WriteFile(tmp)` + `Rename(tmp, path)` |
| Leitura | Na inicialização, antes de criar `state.Manager` |
| Falha na escrita | Logada como `warn`; não interrompe o engine |
| Falha na leitura | Defaults (1.0, 1.0) aplicados silenciosamente |
| Primeira execução | Arquivo não existe → defaults aplicados. Sem erro |

---

## Riscos

- **Hot path**: `applyGain` percorre o buffer frame a frame. Para buffers típicos de 512–2048
  frames estéreos (1024–4096 float32), o custo é desprezível (<1 μs). Com `gain == 1.0` a
  função retorna imediatamente.
- **Clipping**: volume máximo = 1.0 sobre fonte já normalizada → sem clipping.
- **Subprocess IPC**: `set_volume` tem prioridade igual aos demais comandos no canal stdin —
  sem preempção necessária.
- **Múltiplos clientes WebSocket**: todos recebem `EvtVolumeChanged` simultaneamente —
  sliders sincronizados sem polling.
