# Plano: Hot-reload do arquivo de configuração

## Contexto

O `config.Load()` é chamado **uma única vez** no startup. Cada subsistema recebe
sua fatia da config **por valor** — uma cópia imutável armazenada como campo `cfg`
interno. Não existe nenhum mecanismo de atualização em tempo real.

---

## Análise de complexidade

### Como a config circula hoje

```
config.Load() → *Config (uma instância)
    ↓ por valor, copiada para cada subsistema na construção
playback.Manager.cfg   (struct playback.Config)
health.Monitor.cfg     (struct health.Config)
scheduler.Manager      (campos individuais extraídos no New())
api.Server             (cfg.Security, cfg.Admin, etc.)
```

Mudar qualquer campo após a construção **não produz efeito** sem propagação explícita.

---

### Classificação dos campos por "reloadabilidade"

| Campo | Hot-reloadável? | Dificuldade | Observação |
|---|---|---|---|
| `logging.level` | ✅ Sim | Baixa | `slog.SetLogLoggerLevel` — efeito imediato, sem lock |
| `logging.format` | ❌ Não | Alta | Logger é construído uma vez; trocar handler exige re-wiring de toda a stack |
| `health.silence_threshold_dbfs` | ✅ Sim | Baixa | Lido a cada tick, basta atomic ou mutex no monitor |
| `health.silence_duration_ms` | ✅ Sim | Baixa | Igual ao anterior |
| `health.progress_interval_ms` | ⚠️ Parcial | Média | Exige parar e recriar o `time.Ticker` dentro do loop |
| `health.audio_health_interval_ms` | ⚠️ Parcial | Média | Igual ao anterior |
| `playback.default_crossfade_ms` | ✅ Sim | Baixa | Lido por chamada, não pré-calculado |
| `playback.auto_crossfade_*` | ✅ Sim | Baixa | Lidos por loop de playback; basta mutex |
| `playback.max_consecutive_item_failures` | ✅ Sim | Baixa | Lido pontualmente |
| `panic.bed_path` | ⚠️ Parcial | Média | Pode ser trocado entre panics, mas não durante um panic ativo |
| `panic.auto_on_silence` / thresholds | ✅ Sim | Baixa | Lidos no health monitor a cada tick |
| `scheduler.timezone` | ❌ Não | Alta | Timezone é parseado uma vez e baked no cron runner |
| `scheduler.missed_threshold_ms` | ✅ Sim | Baixa | Lido por disparo |
| `api.host` / `api.port` | ❌ Não | Muito alta | Mudar porta exige fechar e reabrir o listener TCP — equivale a reiniciar |
| `api.cors` | ⚠️ Parcial | Média | Handler CORS lê a config no request; precisa de RWMutex |
| `audio.output.driver` | ❌ Não | Muito alta | Mudar driver exige fechar o stream de áudio, re-inicializar o backend e re-abrir |
| `audio.output.device_id` | ❌ Não | Alta | Mesma razão — requer fechar/reabrir o output device |
| `audio.sample_rate` / `channels` / `buffer_frames` | ❌ Não | Muito alta | Define o formato do pipeline inteiro; mudar em runtime é equivalente a reiniciar o engine |
| `security.allowed_roots` | ✅ Sim | Baixa | Lido por validação de path em cada request |
| `preview.*` | ❌ Não | Alta | Preview player é construído uma vez com o driver selecionado |
| `hora_certa.*` | ✅ Sim | Baixa | Lido no resolve de paths por item |

**Resumo:** ~40% dos campos são reloadáveis sem efeito colateral. Os 60% restantes
exigem reinicialização parcial ou total de subsistemas críticos.

---

### O que seria necessário para implementar hot-reload completo do arquivo YAML

1. **Watcher de arquivo** (`fsnotify` ou polling com `time.Ticker`)
   — dependência nova, goroutine nova, caminho de erro novo.

2. **Propagação atômica para cada subsistema**
   — cada `cfg` copiado por valor hoje precisaria virar `*Config` compartilhado
   com `sync.RWMutex`, ou os subsistemas precisariam expor métodos `ApplyConfig(cfg)`.

3. **Tratamento de campos não-reloadáveis**
   — o loader precisa detectar quais campos mudaram, rejeitar mudanças proibidas
   (porta, driver, sample rate) e logar um aviso claro.

4. **Validação antes de aplicar**
   — a nova config precisa passar pelo mesmo `validate()` antes de ser propagada.
   Um YAML inválido em produção não pode derrubar o estado atual.

5. **Coordenação com o pipeline de áudio**
   — aplicar mudanças de threshold ou crossfade enquanto um item está tocando
   requer que o hot path de áudio (sem locks longos) ainda funcione corretamente.

6. **Testes de concorrência**
   — cada subsistema modificado precisa de testes com `-race` para garantir que
   a atualização concorrente não corrompe o estado.

**Estimativa de esforço:** 3–5 dias de implementação segura + revisão.

---

## Minha opinião sobre hot-reload de arquivo YAML

**Não implementaria.**

O hot-reload resolve um problema real — reiniciar um engine de rádio em produção
interrompe o áudio ao vivo — mas a relação custo/benefício é desfavorável por três razões:

1. Os parâmetros que um operador precisaria ajustar sem reiniciar podem ser expostos
   via `PUT /v1/config/runtime` com menos esforço e zero dependência de filesystem.
2. Os campos de alto impacto (driver de áudio, porta TCP, sample rate) não são
   hot-reloadáveis de forma segura de qualquer jeito.
3. Adiciona concorrência no hot path de áudio onde o projeto precisa de estabilidade.

---

## Alternativa implementada: `PUT /v1/config/runtime`

Endpoint REST que aceita apenas os campos genuinamente hot-reloadáveis, com
validação síncrona e resposta HTTP imediata.

**Vantagens sobre hot-reload de arquivo:**

- Sem dependência de filesystem watcher
- Sem goroutine extra
- Validação síncrona com resposta HTTP imediata (400 se inválido)
- Auditável via logs de acesso da API
- Sem risco de aplicar config parcialmente corrompida
- Fácil de testar (request/response determinístico)
- A UI pode usar o endpoint diretamente sem editar YAML

---

## Implementação de `PUT /v1/config/runtime`

### Campos suportados

```json
PUT /v1/config/runtime
{
  "playback": {
    "default_crossfade_ms": 5000,
    "max_consecutive_item_failures": 5,
    "auto_crossfade_enabled": true,
    "auto_crossfade_energy_threshold_dbfs": -18.0,
    "auto_crossfade_min_before_end_ms": 5000,
    "auto_crossfade_max_before_end_ms": 15000,
    "auto_crossfade_hold_frames": 3
  },
  "health": {
    "silence_threshold_dbfs": -55.0,
    "silence_duration_ms": 3000,
    "auto_panic_silence_duration_ms": 10000
  },
  "panic": {
    "bed_path": "/media/panic/new-bed.mp3"
  },
  "logging": {
    "level": "debug"
  }
}
```

Todos os campos são **opcionais** — apenas os presentes são aplicados (merge parcial).

### Resposta de sucesso — 200

```json
{
  "ok": true,
  "applied": {
    "playback.default_crossfade_ms": 5000,
    "health.silence_threshold_dbfs": -55.0,
    "logging.level": "debug"
  }
}
```

### Resposta de erro — 400

```json
{
  "ok": false,
  "error": "invalid_value",
  "message": "playback.default_crossfade_ms must be >= 0"
}
```

---

### Arquivos a criar/modificar

#### 1. `internal/playback/manager.go` — adicionar `ApplyRuntimeConfig`

Novo método público que atualiza os campos reloadáveis sob o mutex existente:

```go
// RuntimePlaybackConfig holds the subset of Config that can be changed at runtime.
type RuntimePlaybackConfig struct {
    DefaultCrossfadeMS            *int
    MaxConsecutiveFailures        *int
    AutoCrossfadeEnabled          *bool
    AutoCrossfadeEnergyThreshDBFS *float64
    AutoCrossfadeMinBeforeEndMS   *int
    AutoCrossfadeMaxBeforeEndMS   *int
    AutoCrossfadeHoldFrames       *int
    PanicBedPath                  *string
}

// ApplyRuntimeConfig updates the subset of Config fields that are safe to
// change while the engine is running. The update is applied under the existing
// sessionMu lock so the hot path never sees a partial write.
func (m *Manager) ApplyRuntimeConfig(rc RuntimePlaybackConfig) {
    m.sessionMu.Lock()
    defer m.sessionMu.Unlock()
    if rc.DefaultCrossfadeMS != nil {
        m.cfg.DefaultCrossfadeMS = *rc.DefaultCrossfadeMS
    }
    if rc.MaxConsecutiveFailures != nil {
        m.cfg.MaxConsecutiveFailures = *rc.MaxConsecutiveFailures
    }
    if rc.AutoCrossfadeEnabled != nil {
        m.cfg.AutoCrossfadeEnabled = *rc.AutoCrossfadeEnabled
    }
    if rc.AutoCrossfadeEnergyThreshDBFS != nil {
        m.cfg.AutoCrossfadeEnergyThreshDBFS = *rc.AutoCrossfadeEnergyThreshDBFS
    }
    if rc.AutoCrossfadeMinBeforeEndMS != nil {
        m.cfg.AutoCrossfadeMinBeforeEndMS = *rc.AutoCrossfadeMinBeforeEndMS
    }
    if rc.AutoCrossfadeMaxBeforeEndMS != nil {
        m.cfg.AutoCrossfadeMaxBeforeEndMS = *rc.AutoCrossfadeMaxBeforeEndMS
    }
    if rc.AutoCrossfadeHoldFrames != nil {
        m.cfg.AutoCrossfadeHoldFrames = *rc.AutoCrossfadeHoldFrames
    }
    if rc.PanicBedPath != nil {
        m.cfg.PanicBedPath = *rc.PanicBedPath
    }
}
```

#### 2. `internal/health/monitor.go` — adicionar `ApplyRuntimeConfig`

O monitor usa `m.mu` (já existente) para proteger os campos lidos no loop:

```go
// RuntimeHealthConfig holds the subset of health.Config safe to change at runtime.
type RuntimeHealthConfig struct {
    SilenceThresholdDBFS       *float64
    SilenceDurationMS          *int
    AutoPanicSilenceDurationMS *int
}

func (m *Monitor) ApplyRuntimeConfig(rc RuntimeHealthConfig) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if rc.SilenceThresholdDBFS != nil {
        m.cfg.SilenceThresholdDBFS = *rc.SilenceThresholdDBFS
    }
    if rc.SilenceDurationMS != nil {
        m.cfg.SilenceDurationMS = *rc.SilenceDurationMS
    }
    if rc.AutoPanicSilenceDurationMS != nil {
        m.cfg.AutoPanicSilenceDurationMS = *rc.AutoPanicSilenceDurationMS
    }
}
```

#### 3. `internal/api/handlers/runtime_config.go` — novo handler

```go
package handlers

import (
    "encoding/json"
    "log/slog"
    "net/http"

    "github.com/Waelson/radio-playout-engine/internal/health"
    "github.com/Waelson/radio-playout-engine/internal/playback"
)

// RuntimeConfigDeps holds the subsystems that accept runtime config updates.
type RuntimeConfigDeps struct {
    Playback interface{ ApplyRuntimeConfig(playback.RuntimePlaybackConfig) }
    Health   interface{ ApplyRuntimeConfig(health.RuntimeHealthConfig) }
    Logger   *slog.Logger
}

type runtimeConfigRequest struct {
    Playback *runtimePlaybackPatch `json:"playback,omitempty"`
    Health   *runtimeHealthPatch   `json:"health,omitempty"`
    Panic    *runtimePanicPatch    `json:"panic,omitempty"`
    Logging  *runtimeLoggingPatch  `json:"logging,omitempty"`
}

type runtimePlaybackPatch struct {
    DefaultCrossfadeMS            *int     `json:"default_crossfade_ms,omitempty"`
    MaxConsecutiveItemFailures    *int     `json:"max_consecutive_item_failures,omitempty"`
    AutoCrossfadeEnabled          *bool    `json:"auto_crossfade_enabled,omitempty"`
    AutoCrossfadeEnergyThreshDBFS *float64 `json:"auto_crossfade_energy_threshold_dbfs,omitempty"`
    AutoCrossfadeMinBeforeEndMS   *int     `json:"auto_crossfade_min_before_end_ms,omitempty"`
    AutoCrossfadeMaxBeforeEndMS   *int     `json:"auto_crossfade_max_before_end_ms,omitempty"`
    AutoCrossfadeHoldFrames       *int     `json:"auto_crossfade_hold_frames,omitempty"`
}

type runtimeHealthPatch struct {
    SilenceThresholdDBFS       *float64 `json:"silence_threshold_dbfs,omitempty"`
    SilenceDurationMS          *int     `json:"silence_duration_ms,omitempty"`
    AutoPanicSilenceDurationMS *int     `json:"auto_panic_silence_duration_ms,omitempty"`
}

type runtimePanicPatch struct {
    BedPath *string `json:"bed_path,omitempty"`
}

type runtimeLoggingPatch struct {
    Level *string `json:"level,omitempty"` // debug|info|warn|error
}

// UpdateRuntimeConfig returns a handler for PUT /v1/config/runtime.
func UpdateRuntimeConfig(deps RuntimeConfigDeps) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req runtimeConfigRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
            return
        }

        applied := map[string]any{}

        if p := req.Playback; p != nil {
            if p.DefaultCrossfadeMS != nil && *p.DefaultCrossfadeMS < 0 {
                writeError(w, http.StatusBadRequest, "invalid_value",
                    "playback.default_crossfade_ms must be >= 0")
                return
            }
            if p.MaxConsecutiveItemFailures != nil && *p.MaxConsecutiveItemFailures < 1 {
                writeError(w, http.StatusBadRequest, "invalid_value",
                    "playback.max_consecutive_item_failures must be >= 1")
                return
            }
            rc := playback.RuntimePlaybackConfig{
                DefaultCrossfadeMS:            p.DefaultCrossfadeMS,
                MaxConsecutiveFailures:        p.MaxConsecutiveItemFailures,
                AutoCrossfadeEnabled:          p.AutoCrossfadeEnabled,
                AutoCrossfadeEnergyThreshDBFS: p.AutoCrossfadeEnergyThreshDBFS,
                AutoCrossfadeMinBeforeEndMS:   p.AutoCrossfadeMinBeforeEndMS,
                AutoCrossfadeMaxBeforeEndMS:   p.AutoCrossfadeMaxBeforeEndMS,
                AutoCrossfadeHoldFrames:       p.AutoCrossfadeHoldFrames,
            }
            deps.Playback.ApplyRuntimeConfig(rc)
            if p.DefaultCrossfadeMS != nil { applied["playback.default_crossfade_ms"] = *p.DefaultCrossfadeMS }
            // ... demais campos
        }

        if h := req.Health; h != nil {
            rc := health.RuntimeHealthConfig{
                SilenceThresholdDBFS:       h.SilenceThresholdDBFS,
                SilenceDurationMS:          h.SilenceDurationMS,
                AutoPanicSilenceDurationMS: h.AutoPanicSilenceDurationMS,
            }
            deps.Health.ApplyRuntimeConfig(rc)
            if h.SilenceThresholdDBFS != nil { applied["health.silence_threshold_dbfs"] = *h.SilenceThresholdDBFS }
            // ... demais campos
        }

        if pa := req.Panic; pa != nil && pa.BedPath != nil {
            deps.Playback.ApplyRuntimeConfig(playback.RuntimePlaybackConfig{PanicBedPath: pa.BedPath})
            applied["panic.bed_path"] = *pa.BedPath
        }

        if l := req.Logging; l != nil && l.Level != nil {
            lvl, err := parseLogLevel(*l.Level)
            if err != nil {
                writeError(w, http.StatusBadRequest, "invalid_value",
                    "logging.level must be debug|info|warn|error")
                return
            }
            deps.Logger.Handler() // placeholder — aplicar via slog.SetLogLoggerLevel
            _ = lvl
            applied["logging.level"] = *l.Level
        }

        writeJSON(w, http.StatusOK, map[string]any{"ok": true, "applied": applied})
    }
}
```

#### 4. `internal/api/server.go` — registrar o endpoint e injetar deps

```go
// RuntimeConfigDeps a ser adicionado ao Server ou passado diretamente no New():
mux.HandleFunc("PUT /v1/config/runtime", handlers.UpdateRuntimeConfig(handlers.RuntimeConfigDeps{
    Playback: playbackMgr,
    Health:   healthMonitor,
    Logger:   log,
}))
```

#### 5. `internal/api/handlers/runtime_config_test.go` — testes

```
TestUpdateRuntimeConfig_Playback_CrossfadeMS   — campo único, verifica 200 + applied
TestUpdateRuntimeConfig_Health_Silence          — silence_threshold_dbfs e duration
TestUpdateRuntimeConfig_Panic_BedPath           — panic.bed_path atualizado
TestUpdateRuntimeConfig_Logging_Level           — debug/info/warn/error válidos
TestUpdateRuntimeConfig_InvalidJSON             — 400
TestUpdateRuntimeConfig_InvalidCrossfade        — default_crossfade_ms < 0 → 400
TestUpdateRuntimeConfig_InvalidLogLevel         — nível desconhecido → 400
TestUpdateRuntimeConfig_EmptyBody               — {} → 200, applied vazio
TestUpdateRuntimeConfig_PartialPatch            — apenas health presente; playback intocado
```

---

### Resumo dos arquivos

| Arquivo | Ação |
|---|---|
| `internal/playback/manager.go` | Adicionar `RuntimePlaybackConfig` + `ApplyRuntimeConfig` |
| `internal/health/monitor.go` | Adicionar `RuntimeHealthConfig` + `ApplyRuntimeConfig` |
| `internal/api/handlers/runtime_config.go` | CRIAR — handler + DTOs |
| `internal/api/handlers/runtime_config_test.go` | CRIAR — testes de handler |
| `internal/api/server.go` | Registrar `PUT /v1/config/runtime` |
| `docs/specs/03-api-rest.md` | Documentar o novo endpoint |

**Esforço estimado:** 1 dia.
