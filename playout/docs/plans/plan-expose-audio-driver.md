# Plano: Expor driver de áudio compilado na página de status

## Contexto

`outfactory.BuiltinDriverName()` já retorna o nome do driver compilado (`"coreaudio"`,
`"portaudio"`, `"wasapi"` ou `"null"`). O dado existe mas não é exposto via API nem
visível na UI. O operador precisa saber qual driver está em uso sem ter que inspecionar
o binário.

O campo é **estático** (não muda em runtime), portanto pertence a `GET /v1/info` —
que expõe identidade imutável do processo (engine_id, version, pid, start_time) —
e não a `GET /v1/status` (que é dinâmico).

---

## O que muda

### 1. `internal/api/server.go`

Adicionar `AudioDriver string` a `api.Config`:

```go
type Config struct {
    Host           string
    Port           int
    AllowedOrigins []string
    EngineID       string
    Version        string
    StartTime      time.Time
    AudioDriver    string  // ← novo: "coreaudio" | "portaudio" | "wasapi" | "null"
}
```

---

### 2. `cmd/playout-engine/main.go`

Preencher o campo ao montar `apiCfg`:

```go
apiCfg := api.Config{
    ...
    AudioDriver: outfactory.BuiltinDriverName(),
}
```

---

### 3. `internal/api/handlers/info.go`

Adicionar `audio_driver` ao response e receber o valor como parâmetro:

```go
type infoResponse struct {
    EngineID    string    `json:"engine_id"`
    PID         int       `json:"pid"`
    Version     string    `json:"version"`
    StartTime   time.Time `json:"start_time"`
    LocalIP     string    `json:"local_ip"`
    OS          string    `json:"os"`
    AudioDriver string    `json:"audio_driver"` // ← novo
}

func Info(engineID, version string, startTime time.Time, audioDriver string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        _ = json.NewEncoder(w).Encode(infoResponse{
            ...
            AudioDriver: audioDriver,
        })
    }
}
```

---

### 4. `internal/api/server.go` — wiring

Passar `cfg.AudioDriver` ao registrar o handler `Info`:

```go
// Antes:
mux.Handle("GET /v1/info", handlers.Info(cfg.EngineID, cfg.Version, cfg.StartTime))

// Depois:
mux.Handle("GET /v1/info", handlers.Info(cfg.EngineID, cfg.Version, cfg.StartTime, cfg.AudioDriver))
```

---

### 5. `internal/api/handlers/status_html.go` — página de status

O SPA já consome `GET /v1/info` para exibir `engine_id` e `version`. Basta:

- Ler `info.audio_driver` na função `loadInfo()` do JavaScript
- Exibir como campo somente leitura na seção de informações do engine:

```html
<!-- Novo item na grade de info -->
<div class="info-item">
  <span class="info-label">Driver de áudio</span>
  <span class="info-value" id="info-driver">—</span>
</div>
```

```js
// Na função que popula os campos de info:
document.getElementById('info-driver').textContent = info.audio_driver || '—';
```

---

## Resposta JSON resultante

```json
GET /v1/info
{
  "engine_id": "studio-a-main",
  "pid": 12345,
  "version": "0.3.1",
  "start_time": "2026-07-07T23:08:16Z",
  "local_ip": "192.168.1.10",
  "os": "darwin/arm64",
  "audio_driver": "coreaudio"
}
```

---

## Arquivos modificados — resumo

| Arquivo | Ação |
|---|---|
| `internal/api/server.go` | Adicionar `AudioDriver string` a `api.Config` |
| `cmd/playout-engine/main.go` | Preencher `AudioDriver: outfactory.BuiltinDriverName()` |
| `internal/api/handlers/info.go` | Adicionar campo ao response e parâmetro ao handler |
| `internal/api/server.go` | Passar `cfg.AudioDriver` ao registrar `Info` |
| `internal/api/handlers/status_html.go` | Exibir `audio_driver` na UI |

Nenhum arquivo de spec, teste ou YAML precisa ser alterado — o campo é aditivo
(`omitempty` não é necessário pois o valor nunca é vazio) e não quebra clientes
existentes.

---

## Riscos

Nenhum. O campo é additive, só leitura, e determinado em compile-time.
