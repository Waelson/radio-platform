# Plano: Remover `output_driver` da configuração (Opção A)

## Contexto

O campo `output_driver` no YAML induz o operador a acreditar que pode trocar o
driver de áudio em runtime. Na prática, o driver é determinado em compile-time
via build tag (`-tags coreaudio`, `-tags portaudio`, `-tags wasapi`). Configurar
um driver incompatível com o binário em uso causa falha silenciosa — especialmente
crítico no CUE subprocess, onde o processo filho morre antes de emitir qualquer
evento de erro visível.

A solução é remover o campo e tornar cada binário auto-declarativo: quem compila
decide o driver, e o operador configura apenas o **device ID** (qual dispositivo
físico usar dentro do driver).

---

## O que muda conceitualmente

| Antes | Depois |
|---|---|
| `output_driver: coreaudio` no YAML + binary coreaudio | binary coreaudio (driver implícito) |
| `output_driver: portaudio` com binary coreaudio → falha | Impossível de configurar errado |
| Operador configura driver e device | Operador configura apenas device |
| Factory lê `cfg.Audio.Output.Driver` | Factory ignora config; retorna driver compilado |

**O que permanece:** `output.device_id` e `preview.output_device` continuam como
configuração, pois dependem do ambiente (qual dispositivo físico está disponível).
`allow_null_output` também permanece — controla fallback gracioso se o device falhar.

---

## Arquivos impactados

### 1. `internal/config/config.go`

**Remover** `Driver string` de `OutputConfig`:

```go
// Antes:
type OutputConfig struct {
    Driver          string `yaml:"driver"            json:"driver"`
    DeviceID        string `yaml:"device_id"         json:"device_id"`
    AllowNullOutput bool   `yaml:"allow_null_output" json:"allow_null_output"`
}

// Depois:
type OutputConfig struct {
    DeviceID        string `yaml:"device_id"         json:"device_id"`
    AllowNullOutput bool   `yaml:"allow_null_output" json:"allow_null_output"`
}
```

**Remover** `OutputDriver string` de `PreviewConfig`:

```go
// Antes:
type PreviewConfig struct {
    Enabled      bool   `yaml:"enabled"       json:"enabled"`
    OutputDriver string `yaml:"output_driver" json:"output_driver"`
    OutputDevice string `yaml:"output_device" json:"output_device"`
}

// Depois:
type PreviewConfig struct {
    Enabled      bool   `yaml:"enabled"       json:"enabled"`
    OutputDevice string `yaml:"output_device" json:"output_device"`
}
```

---

### 2. `internal/config/loader.go`

**Remover** de `defaults()`:
```go
// Remover:
Output: OutputConfig{
    Driver: "null",  // ← remover apenas esta linha
    ...
}
// Remover:
Preview: PreviewConfig{
    OutputDriver: "null",  // ← remover apenas esta linha
    ...
}
```

**Remover** env vars:
```go
// Remover:
if v := os.Getenv("PLAYOUT_AUDIO_OUTPUT_DRIVER"); v != "" {
    cfg.Audio.Output.Driver = v
}
if v := os.Getenv("RADIOCORE_PREVIEW_OUTPUT_DRIVER"); v != "" {
    cfg.Preview.OutputDriver = v
}
```

**Remover** validação em `Validate()`:
```go
// Remover completamente:
validDrivers := map[string]bool{"null": true, "portaudio": true, "file": true, "coreaudio": true}
if !validDrivers[strings.ToLower(cfg.Audio.Output.Driver)] {
    return fmt.Errorf("audio.output.driver %q is not supported ...", cfg.Audio.Output.Driver)
}
```

---

### 3. `cmd/playout-engine/output/factory.go` (sem build tags — NullOutput)

**Antes:** switch em `cfg.Audio.Output.Driver`.
**Depois:** retorna `NullOutput` diretamente, sem ler config de driver.

```go
func NewOutputDevice(_ *config.Config) (output.OutputDevice, error) {
    return &output.NullOutput{}, nil
}
```

> O binário sem build tags não tem driver real — sempre NullOutput.
> Mantém a assinatura `*config.Config` por consistência (ainda usa DeviceID no futuro).

---

### 4. `cmd/playout-engine/output/factory_coreaudio.go`

```go
func NewOutputDevice(_ *config.Config) (output.OutputDevice, error) {
    return caout.New(), nil
}
```

Sem switch. Sem referência a `cfg.Audio.Output.Driver`.

---

### 5. `cmd/playout-engine/output/factory_portaudio.go`

```go
func NewOutputDevice(_ *config.Config) (output.OutputDevice, error) {
    return paout.New()
}
```

---

### 6. `cmd/playout-engine/output/factory_wasapi.go`

```go
func NewOutputDevice(_ *config.Config) (output.OutputDevice, error) {
    return waout.New(), nil
}
```

---

### 7–10. `cmd/playout-engine/output/preview_factory*.go` (4 arquivos)

Mesma simplificação nos quatro arquivos (sem tags, coreaudio, portaudio, wasapi).

```go
// preview_factory_coreaudio.go
func NewPreviewOutputDevice(_ *config.Config) (output.OutputDevice, error) {
    return caout.New(), nil
}

// preview_factory.go (sem tags)
func NewPreviewOutputDevice(_ *config.Config) (output.OutputDevice, error) {
    return &output.NullOutput{}, nil
}
```

---

### 11. `cmd/playout-engine/output/` — novo arquivo `driver_name.go`

Para que o engine saiba logar e expor qual driver está em uso, adicionar uma
função por build tag que retorna o nome do driver compilado:

```go
// driver_name.go (sem build tags)
func BuiltinDriverName() string { return "null" }

// driver_name_coreaudio.go
//go:build coreaudio && !wasapi
func BuiltinDriverName() string { return "coreaudio" }

// driver_name_portaudio.go
//go:build portaudio && !wasapi
func BuiltinDriverName() string { return "portaudio" }

// driver_name_wasapi.go
//go:build wasapi
func BuiltinDriverName() string { return "wasapi" }
```

---

### 12. `cmd/playout-engine/main.go`

**Remover** referência ao driver da config no log de startup:

```go
// Antes:
log.Info("engine starting",
    ...
    "audio_driver", cfg.Audio.Output.Driver,
    ...
)

// Depois:
log.Info("engine starting",
    ...
    "audio_driver", outfactory.BuiltinDriverName(),
    ...
)
```

**Remover** referência no log do preview:
```go
// Antes:
log.Info("preview player enabled as subprocess", "driver", cfg.Preview.OutputDriver)

// Depois:
log.Info("preview player enabled as subprocess", "driver", outfactory.BuiltinDriverName())
```

**Remover** o `runCuePlayer` que usa `cfg.Preview.OutputDriver` — a factory não
precisa mais do driver, então nenhuma outra mudança necessária lá.

---

### 13. `internal/api/handlers/config_html.go`

**Remover** o radio group "Driver de saída do preview" da UI:

```html
<!-- Remover completamente: -->
<label class="lbl">Driver de saída do preview</label>
<div class="radio-group" data-name="prev-driver">
  <label class="radio-pill">...</label>
  ...
</div>
<div class="hint">* coreaudio exibido apenas em macOS...</div>
```

**Remover** do JavaScript de leitura:
```js
// Remover:
setRadio('prev-driver', pv.output_driver);
```

**Remover** do JavaScript de escrita:
```js
// Remover esta linha do objeto preview:
output_driver: getRadio('prev-driver') || 'null',
```

**Adicionar** (opcional mas recomendado): exibir o driver compilado como campo
somente leitura:
```html
<label class="lbl">Driver de áudio</label>
<span class="value-readonly">coreaudio</span>
<div class="hint">Determinado em compile-time. Para mudar, recompile com a build tag correspondente.</div>
```

O valor pode vir de um novo campo `audio_driver` em `GET /v1/status` ou
`GET /v1/config` (já expõe o config completo).

---

### 14. `playout-engine.yaml`

**Remover** linha `output_driver: "coreaudio"` da seção `preview`:

```yaml
# Antes:
preview:
  enabled: true
  output_driver: "coreaudio"
  output_device: "EC-46-54-10-A5-4C:output"

# Depois:
preview:
  enabled: true
  output_device: "EC-46-54-10-A5-4C:output"
```

**Remover** linha `driver: "coreaudio"` da seção `audio.output` se existir:

```yaml
# Antes:
audio:
  output:
    driver: coreaudio
    device_id: "BuiltInSpeakerDevice"

# Depois:
audio:
  output:
    device_id: "BuiltInSpeakerDevice"
```

---

### 15. `docs/specs/12-configuration.md`

Remover `audio.output.driver` e `preview.output_driver` das tabelas de campos.
Adicionar nota: "o driver de áudio é determinado pela build tag usada na compilação."

---

### 16. `README.md`

Atualizar a frase:
```
# Antes:
O driver é selecionável em runtime via `audio.output.driver` no YAML, sem recompilar.

# Depois:
O driver de áudio é fixo por binário, determinado pela build tag usada na compilação.
Para trocar o driver, compile com a tag correspondente (ex: -tags coreaudio).
O operador configura apenas o device_id — qual dispositivo físico usar dentro do driver.
```

---

### 17. `internal/api/handlers/config_test.go`

Os testes que fazem `cfg.Audio.Output.Driver = "null"` precisam ser ajustados:
- Remover a atribuição ao campo removido
- O campo `Driver` não existe mais em `OutputConfig`

---

## Riscos

### R1 — Breaking change no YAML

Operadores com `output_driver` ou `driver` no YAML existente vão ter o campo
**ignorado silenciosamente** pelo YAML parser (campos desconhecidos são ignorados
por padrão em `gopkg.in/yaml.v3`). Não causa erro, mas o campo fica como lixo
no arquivo de configuração.

**Mitigação:** Adicionar aviso no startup se o campo antigo for detectado. Alternativa:
documentar no CHANGELOG que o campo foi removido.

### R2 — `AllowNullOutput` fica sem `Driver: "null"` para ativar fallback

Se o device principal falhar e `AllowNullOutput: true`, o engine hoje pode não
ter mecanismo explícito de fallback (depende de como a factory ou o playback
manager usa esse campo).

**Mitigação:** Verificar se `AllowNullOutput` está realmente sendo usado no caminho
de inicialização. Se não estiver, é questão separada — não impacta este plano.

### R3 — `GET /v1/config` expõe o config salvo

A API de configuração salva e retorna o YAML como struct. Após a remoção do campo,
a resposta JSON não terá mais `driver` em `audio.output`. Clientes que leem esse
campo terão `undefined` em vez de um valor.

**Mitigação:** Backwards compat: adicionar `driver` como campo computed/readonly
no JSON de resposta, preenchido com `outfactory.BuiltinDriverName()`, sem persistir no YAML.

---

## Resumo das fases

| Fase | O que entrega | Risco |
|---|---|---|
| 1 | Remover `Driver`/`OutputDriver` de config.go, loader.go e validação | Baixo |
| 2 | Simplificar os 8 factory files (remover switches) | Baixo |
| 3 | Adicionar `BuiltinDriverName()` e usar em main.go | Baixo |
| 4 | Remover UI do driver na config_html.go | Baixo |
| 5 | Atualizar playout-engine.yaml, README e docs/specs | Baixo |
| 6 | Ajustar testes em config_test.go | Baixo |

Todas as fases são independentes e podem ser feitas em sequência contínua.
Nenhuma fase tem risco médio ou alto.
