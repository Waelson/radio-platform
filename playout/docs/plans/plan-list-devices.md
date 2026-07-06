# Plano: GET /v1/devices — Lista de dispositivos de áudio

## Contexto

O playout engine suporta múltiplos drivers de áudio (coreaudio, portaudio, null). Atualmente não há como consultar via API quais devices de áudio estão disponíveis no sistema. O endpoint `GET /v1/devices` expõe essa lista de forma sempre atualizada (chamada ao SO a cada request, sem cache), permitindo que o player/UI mostre ao operador quais saídas estão disponíveis e qual é o padrão do sistema.

---

## Considerações por driver

| Campo | CoreAudio | PortAudio | Null / File |
|-------|-----------|-----------|-------------|
| `id` | UID único do sistema (`kAudioDevicePropertyDeviceUID`), persiste mesmo que o nome mude | Igual ao `name` — PortAudio não expõe UID interno | `"null"` / `"file"` (fixo) |
| `name` | Nome legível do dispositivo | Nome legível do dispositivo | `"Null Output"` / `"File Output"` |
| `is_default` | Via `kAudioHardwarePropertyDefaultOutputDevice` | Via `pa.DefaultOutputDevice()` | Sempre `true` |
| `max_output_channels` | Via `kAudioDevicePropertyStreamConfiguration` | `pa.DeviceInfo.MaxOutputChannels` | `2` (fixo) |
| `default_sample_rate` | Via `kAudioDevicePropertyNominalSampleRate` | `pa.DeviceInfo.DefaultSampleRate` | `48000` (fixo) |

**Nota PortAudio:** Como `id == name`, se o usuário renomear um device no SO o ID muda junto. Ao usar o ID como referência para selecionar o device em `OutputConfig.DeviceID`, esse comportamento é idêntico ao que o `resolveDevice` existente já pratica (busca por nome).

---

## Resposta JSON esperada

```
GET /v1/devices → 200 OK
{
  "devices": [
    {
      "id":                   "AppleHDAEngineOutput:0",
      "name":                 "MacBook Pro Speakers",
      "driver":               "coreaudio",
      "is_default":           true,
      "max_output_channels":  2,
      "default_sample_rate":  48000.0
    },
    {
      "id":                   "BlackHole 2ch",
      "name":                 "BlackHole 2ch",
      "driver":               "coreaudio",
      "is_default":           false,
      "max_output_channels":  2,
      "default_sample_rate":  44100.0
    }
  ],
  "count": 2
}
```

---

## Restrições arquiteturais respeitadas

- API **não importa** pacotes de áudio (`output`, `coreaudio`, `portaudio`)
- Nenhum cache — SO consultado a cada request; `Cache-Control: no-store` na resposta
- Injeção via função, mesmo padrão de `previewStatus func() any` / `PreviewDeps`
- O `OutputDevice` já criado em `main.go` é type-assertado para `DeviceLister` — nenhuma instância extra

---

## Fase 1 — Camada de áudio: interface + implementações

**Objetivo:** Adicionar a capacidade de listar devices em cada driver, sem tocar na API.

### 1.1 — CRIAR `internal/audio/output/devices.go`

```go
// DeviceInfo descreve um dispositivo de saída de áudio disponível no sistema.
type DeviceInfo struct {
    ID                string  // UID único no CoreAudio; igual ao Name no PortAudio
    Name              string  // Nome legível (ex: "MacBook Pro Speakers")
    Driver            string  // "coreaudio" | "portaudio" | "null" | "file"
    IsDefault         bool    // true se for o output padrão do sistema
    MaxOutputChannels int     // número máximo de canais de saída
    DefaultSampleRate float64 // taxa de amostragem padrão reportada pelo SO
}

// DeviceLister é implementado por qualquer OutputDevice capaz de listar
// os dispositivos disponíveis no sistema sem precisar de um stream aberto.
type DeviceLister interface {
    ListDevices() ([]DeviceInfo, error)
}
```

### 1.2 — MODIFICAR `internal/audio/output/coreaudio/bridge.h`

Adicionar struct C e declaração:

```c
typedef struct {
    char   uid[256];   // kAudioDevicePropertyDeviceUID — persiste entre reinicializações
    char   name[256];  // kAudioObjectPropertyName — nome legível
    int    maxOutputChannels;
    double defaultSampleRate;
    int    isDefault;
} CADeviceEntry;

// Preenche `out` com até `maxCount` devices de saída.
// Retorna o número de devices encontrados.
int caEnumOutputDevices(CADeviceEntry *out, int maxCount);
```

### 1.3 — MODIFICAR `internal/audio/output/coreaudio/bridge.c`

Implementar `caEnumOutputDevices`:
- `kAudioHardwarePropertyDevices` → lista de `AudioDeviceID`
- `kAudioHardwarePropertyDefaultOutputDevice` → identifica o default
- Para cada device com output streams (`kAudioDevicePropertyStreams` scope output):
  - `kAudioDevicePropertyDeviceUID` → `uid`
  - `kAudioObjectPropertyName` → `name`
  - `kAudioDevicePropertyStreamConfiguration` (output scope) → soma de canais
  - `kAudioDevicePropertyNominalSampleRate` → `defaultSampleRate`
  - Compara `AudioDeviceID` com o default → `isDefault`

### 1.4 — MODIFICAR `internal/audio/output/coreaudio/coreaudio.go`

Adicionar `ListDevices() ([]output.DeviceInfo, error)` ao `Output`:
- Não requer device aberto
- Aloca array C de até 64 `CADeviceEntry`
- Chama `caEnumOutputDevices`
- Popula `DeviceInfo.ID` com `uid` (UID único persistente)

### 1.5 — MODIFICAR `internal/audio/output/portaudio/portaudio.go`

Adicionar `ListDevices() ([]output.DeviceInfo, error)` ao `Output`:
- `pa.DefaultOutputDevice()` → identifica o default
- `pa.Devices()` → itera todos, filtra `d.MaxOutputChannels > 0`
- `DeviceInfo.ID = d.Name` (PortAudio não expõe UID — usa nome como identificador, consistente com `resolveDevice`)
- `pa.Initialize()` já foi chamado em `New()` — nenhuma inicialização extra

### 1.6 — MODIFICAR `internal/audio/output/null.go` e `file.go`

Adicionar `ListDevices()` a `NullOutput` e `FileOutput`, retornando um pseudo-device fixo:
```go
// NullOutput
[]DeviceInfo{{ID: "null", Name: "Null Output", Driver: "null",
    IsDefault: true, MaxOutputChannels: 2, DefaultSampleRate: 48000}}

// FileOutput
[]DeviceInfo{{ID: "file", Name: "File Output", Driver: "file",
    IsDefault: true, MaxOutputChannels: 2, DefaultSampleRate: 48000}}
```

---

## Fase 2 — Camada de API: handler e rota

**Objetivo:** Expor `GET /v1/devices` sem acoplar a API ao pacote de áudio.

### 2.1 — CRIAR `internal/api/handlers/devices.go`

```go
type AudioDevice struct {
    ID                string  `json:"id"`
    Name              string  `json:"name"`
    Driver            string  `json:"driver"`
    IsDefault         bool    `json:"is_default"`
    MaxOutputChannels int     `json:"max_output_channels"`
    DefaultSampleRate float64 `json:"default_sample_rate"`
}

// Devices retorna handler para GET /v1/devices.
// `list` é chamada a cada request — sem cache.
// Se `list` for nil, retorna lista vazia com status 200.
func Devices(list func() ([]AudioDevice, error)) http.HandlerFunc
```

Comportamento:
- `list == nil` → `{"devices":[],"count":0}` com 200
- erro em `list()` → 500 com envelope padrão (via `respond.go`)
- sucesso → `{"devices":[...],"count":N}` + `Cache-Control: no-store`

### 2.2 — MODIFICAR `internal/api/server.go`

Adicionar `DevicesDeps` (mesmo padrão de `PreviewDeps`):

```go
type DevicesDeps struct {
    List func() ([]handlers.AudioDevice, error) // nil → lista vazia
}
```

Adicionar campo ao `Server`:
```go
listDevices func() ([]handlers.AudioDevice, error)
```

Atualizar `New()` para receber `DevicesDeps`.

Registrar rota (sempre, independente de `listDevices` ser nil):
```go
mux.HandleFunc("GET /v1/devices", handlers.Devices(s.listDevices))
```

---

## Fase 3 — Wiring: main.go

**Objetivo:** Conectar a implementação real ao servidor HTTP.

### 3.1 — MODIFICAR `cmd/playout-engine/main.go`

Após criar `out` via `outfactory.NewOutputDevice(cfg)`, type-assertar para `DeviceLister`:

```go
devicesDeps := api.DevicesDeps{}
if lister, ok := out.(output.DeviceLister); ok {
    devicesDeps.List = func() ([]handlers.AudioDevice, error) {
        infos, err := lister.ListDevices()
        if err != nil {
            return nil, err
        }
        devs := make([]handlers.AudioDevice, len(infos))
        for i, d := range infos {
            devs[i] = handlers.AudioDevice{
                ID:                d.ID,
                Name:              d.Name,
                Driver:            d.Driver,
                IsDefault:         d.IsDefault,
                MaxOutputChannels: d.MaxOutputChannels,
                DefaultSampleRate: d.DefaultSampleRate,
            }
        }
        return devs, nil
    }
}
```

Passar `devicesDeps` para `api.New(...)`.

---

## Fase 4 — Testes

**Objetivo:** Garantir cobertura mínima antes de considerar pronto.

### 4.1 — CRIAR `internal/api/handlers/devices_test.go`

Casos:
- `list` retorna lista populada → verifica JSON completo, `count` correto, campos
- `list` retorna erro → verifica status 500 e envelope de erro
- `list == nil` → verifica `{"devices":[],"count":0}` e status 200

### 4.2 — MODIFICAR `internal/audio/output/null_test.go`

Adicionar:
- `TestNullOutput_ListDevices`: verifica que retorna exatamente 1 device, `IsDefault=true`, `Driver="null"`

---

## Fase 5 — Documentação

**Objetivo:** Manter specs e README sincronizados com o novo contrato, incluindo as considerações por driver.

### 5.1 — MODIFICAR `docs/specs/03-api-rest.md`

Adicionar seção `GET /v1/devices` com:
- Descrição do endpoint
- Exemplo de resposta JSON completo
- Nota sobre `Cache-Control: no-store` e leitura ao vivo do SO
- Nota sobre a semântica do campo `id` por driver (UID persistente no CoreAudio; igual ao name no PortAudio)

### 5.2 — MODIFICAR `docs/specs/09-device-abstraction.md`

Adicionar:
- Interface `DeviceLister` com `ListDevices() ([]DeviceInfo, error)`
- Struct `DeviceInfo` com todos os campos e seus comentários
- Tabela com comportamento do campo `id` por driver (CoreAudio vs PortAudio vs Null/File)
- Nota de que todas as implementações satisfazem a interface

### 5.3 — MODIFICAR `README.md` (playout)

Na seção **Features**, adicionar item sobre o endpoint `GET /v1/devices` com:
- Descrição funcional: "lista os dispositivos de áudio disponíveis no sistema em tempo real"
- Tabela de considerações por driver:

  | Driver | Campo `id` | Estabilidade |
  |--------|-----------|--------------|
  | CoreAudio | UID do sistema (`kAudioDevicePropertyDeviceUID`) | Persiste mesmo se o nome mudar |
  | PortAudio | Igual ao `name` | Muda se o dispositivo for renomeado no SO |
  | Null / File | `"null"` / `"file"` (fixo) | Sempre estável |

---

## Verificação final

```bash
# Rodar testes e vet
go test ./...
go vet ./...

# Build com CoreAudio (macOS)
make build-playout

# Smoke test
./playout-engine --startup=cli &
curl -s http://localhost:8080/v1/devices | jq .
```

---

## Arquivos modificados — resumo

| Fase | Arquivo | Ação |
|------|---------|------|
| 1 | `internal/audio/output/devices.go` | CRIAR |
| 1 | `internal/audio/output/coreaudio/bridge.h` | MODIFICAR |
| 1 | `internal/audio/output/coreaudio/bridge.c` | MODIFICAR |
| 1 | `internal/audio/output/coreaudio/coreaudio.go` | MODIFICAR |
| 1 | `internal/audio/output/portaudio/portaudio.go` | MODIFICAR |
| 1 | `internal/audio/output/null.go` | MODIFICAR |
| 1 | `internal/audio/output/file.go` | MODIFICAR |
| 2 | `internal/api/handlers/devices.go` | CRIAR |
| 2 | `internal/api/server.go` | MODIFICAR |
| 3 | `cmd/playout-engine/main.go` | MODIFICAR |
| 4 | `internal/api/handlers/devices_test.go` | CRIAR |
| 4 | `internal/audio/output/null_test.go` | MODIFICAR |
| 5 | `docs/specs/03-api-rest.md` | MODIFICAR |
| 5 | `docs/specs/09-device-abstraction.md` | MODIFICAR |
| 5 | `README.md` | MODIFICAR |
