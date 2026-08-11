# Plano: Line-In através do Mixer (VU Meter + Controle de Ganho)

## Diagnóstico

### Arquitetura atual (incorreta)

```
[FFmpegCapture] → [LineInManager.run()] → [OutputDevice (CoreAudio)]
                         ↓
                  EventLevelUpdate (computeRMS simples)
                         ↓
                  EvtLineInLevel (canal separado, ignorado pelo VU meter da UI)
```

### Arquitetura do playback (correta — referência)

```
[Decoder (FFmpeg)] → [runPlayLoop]
                           ↓
                     applyGain (duck/overlay/volume)
                           ↓
                     m.out.Write()
                           ↓
                     healthMon.Push()  →  EvtVUMeter  →  VU Meter UI
```

### Problema raiz

O `LineInManager` roteia o áudio **diretamente** de `InputDevice.ReadFrames()` para `OutputDevice.Write()`, **sem passar pelo pipeline de áudio do playback**. Consequências:

| Funcionalidade | Playback (arquivo) | Line-In (atual) |
|---|---|---|
| VU Meter (EvtVUMeter) | Sim | **Não** |
| Volume principal (MainVolume) | Sim | **Não** |
| Duck gain (hot button) | Sim | **Não** |
| Streaming tap (Icecast) | Sim | **Não** |
| Silence watchdog | Sim | Sim (paralelo, duplicado) |

---

## Solução proposta: 3 fases incrementais

### Fase 1 — Conectar ao health.Monitor (VU Meter imediato)

**Objetivo:** Line-in alimenta o VU meter da UI sem mudar a arquitetura estrutural.

**O que muda:**

1. `LineInManager` recebe um `*health.Monitor` via campo opcional `healthMon`.
2. Em `manager.go`, no loop consumer, após `m.out.Write(ctx, frames)`:
   ```go
   if m.healthMon != nil {
       m.healthMon.Push(frames)
   }
   ```
3. `Handler.HandleStart` injeta o monitor no `LineInManager` via `mgr.SetHealthMonitor(mon)`.
4. O monitor já existente (o mesmo do playback) é reutilizado — **não criar instância separada**.

**Impacto:** VU Meter passa a exibir o nível do line-in imediatamente. Nenhuma mudança na UI é necessária.

**Arquivos alterados:**
- `internal/linein/manager.go` — adicionar campo `healthMon`, método `SetHealthMonitor`, chamada a `Push` no loop.
- `internal/linein/handler.go` — injetar o monitor via `SetHealthMonitor`.
- `cmd/playout-engine/main.go` — passar `healthMon` ao `Handler`.

**Remover:** `computeRMS`, `linearToDBFS`, `dbfsToLinear` de `manager.go` e o `EventLevelUpdate` (substituído pelo `EvtVUMeter` do monitor).

---

### Fase 2 — Aplicar ganho e streaming tap no Line-In

**Objetivo:** Volume principal, duck gain e streaming tap (Icecast) funcionam para line-in.

**O que muda:**

No loop consumer do `LineInManager`, antes de `m.out.Write`:

```go
// 1. Aplicar volume principal
applyGain(frames, m.stateMgr.MainVolume())

// 2. Após o write, enviar ao streaming tap (não-bloqueante)
if tap := m.streamingTap; tap != nil {
    cp := make([]float32, len(frames))
    copy(cp, frames)
    select {
    case tap <- cp:
    default:
    }
}
```

**Dependências novas no LineInManager:**
- `*state.Manager` — para `MainVolume()` e `duckGain`.
- `chan<- []float32` — streaming tap (opcional).

**Arquivos alterados:**
- `internal/linein/manager.go` — adicionar campos `stateMgr`, `streamingTap`, função `applyGain`.
- `internal/linein/handler.go` — injetar dependências.

---

### Fase 3 — Mixer unificado (arquitetura correta de longo prazo)

**Objetivo:** Toda fonte de áudio passa por um único ponto de mix antes do `OutputDevice`. Permite ducking do line-in quando um hot button toca, crossfade entre line-in e fila, e garantia de que qualquer nova funcionalidade de áudio afeta todas as fontes automaticamente.

**Nova abstração:**

```go
// internal/audio/mixer/mixer.go
type Source interface {
    // Pull lê até len(dst) frames interleaved float32 no dst.
    // Retorna n frames lidos e erro (io.EOF = fonte encerrada).
    Pull(ctx context.Context, dst []float32) (int, error)
}

type Mixer struct {
    out      output.OutputDevice
    healthMon *health.Monitor
    stateMgr  *state.Manager
    streamTap chan<- []float32
    // ...gains, duck, overlay
}

func (m *Mixer) AddSource(src Source, gain float32) SourceHandle
func (m *Mixer) RemoveSource(h SourceHandle)
func (m *Mixer) Run(ctx context.Context) // loop principal: Pull de todas as fontes → mix → Write → Push
```

**Como as fontes se encaixam:**

```
[decoder.PCMStream]  → Source adapter → Mixer.Run() → out.Write()
[FFmpegCapture]      → Source adapter →              → healthMon.Push()
[hot button overlay] → Source adapter →              → streamTap
```

**Impacto:**
- `playback.Manager` deixa de chamar `m.out.Write` diretamente; em vez disso, registra um `Source` no `Mixer`.
- `LineInManager` registra um `Source` no `Mixer` ao iniciar e remove ao parar.
- Duck gain, volume, overlay passam a ser responsabilidade do `Mixer`, removendo lógica duplicada do `playback.Manager`.

**Arquivos novos:**
- `internal/audio/mixer/mixer.go`
- `internal/audio/mixer/source.go`
- `internal/audio/mixer/mixer_test.go`

**Arquivos alterados (refator profundo):**
- `internal/playback/manager.go` — remover `runPlayLoop` write direto; usar Mixer.
- `internal/linein/manager.go` — remover write direto; registrar Source no Mixer.
- `internal/linein/handler.go` — receber Mixer em vez de OutputDevice.
- `cmd/playout-engine/main.go` — construir Mixer e injetar nos managers.

---

## Ordem de execução recomendada

```
Fase 1 (1-2h) → testar VU meter no line-in → merge
Fase 2 (1-2h) → testar volume/streaming → merge
Fase 3 (1 dia) → refatoração do Mixer → testes → merge
```

As fases 1 e 2 são incrementais e não quebram nada. A fase 3 é o refator estrutural que cumpre a arquitetura correta.

---

## Critério de pronto (por fase)

- Fase 1: VU meter da UI exibe nível durante line-in; `go test ./...` passa.
- Fase 2: Slider de volume afeta line-in; áudio do line-in aparece no stream Icecast; `go test ./...` passa.
- Fase 3: `playback.Manager` e `LineInManager` não chamam `OutputDevice.Write` diretamente; toda a lógica de ganho/ducking/overlay está no `Mixer`; `go test ./...` com `-race` passa.
