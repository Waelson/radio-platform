# Plano: Estabilidade de device ID no Linux e Windows

## Contexto

Na Fase 6 foi implementada a resolução em cascata (UID → nome → default) para o driver CoreAudio no macOS. O PortAudio — usado em Linux e Windows — ainda identifica dispositivos exclusivamente pelo nome legível, o que significa que:

- **Windows**: o usuário pode renomear o dispositivo em *Sound Settings* e o engine perde a referência
- **Linux**: depende do host API; ALSA é razoavelmente estável, PulseAudio/PipeWire varia

O objetivo é: adicionar IDs estáveis onde for possível, e onde não for, expor metadados que ajudem o operador a entender o comportamento de estabilidade do dispositivo configurado.

---

## Diagnóstico por plataforma

| Plataforma | Host API (PortAudio) | ID estável disponível? | Mecanismo |
|---|---|---|---|
| Linux | ALSA | Parcial — `hw:X,X` via ALSA direta | Nome PortAudio já é razoavelmente estável |
| Linux | PulseAudio / PipeWire | Parcial — sink names internos | PortAudio expõe display name, não o sink name |
| Linux | JACK | Sim — port names são estáveis | Estável por natureza |
| Windows | WASAPI | **Sim** — GUID via `IMMDeviceEnumerator::GetId()` | Requer CGo + COM |
| Windows | DirectSound | Não | Somente nome |
| Windows | MME | Não | Somente nome |

---

## Fase 7 — Linux: metadados de host API + documentação

**Objetivo:** Expor qual host API está por trás de cada device (ALSA, PulseAudio, JACK etc.), permitindo que o operador entenda a estabilidade do ID sem mudança de comportamento de resolução.

### 7.1 — MODIFICAR `internal/audio/output/devices.go`

Adicionar campo `HostAPI` à struct `DeviceInfo`:

```go
type DeviceInfo struct {
    ID                string
    Name              string
    Driver            string
    HostAPI           string  // "ALSA" | "PulseAudio" | "JACK" | "WASAPI" | "CoreAudio" | ""
    IsDefault         bool
    MaxOutputChannels int
    DefaultSampleRate float64
}
```

### 7.2 — MODIFICAR `internal/audio/output/portaudio/portaudio.go`

Em `ListDevices()`, preencher `HostAPI` usando `pa.HostApiInfo(d.HostApi).Name`:

```go
hostApiInfo, _ := pa.HostApiInfo(d.HostApi)
hostAPIName := ""
if hostApiInfo != nil {
    hostAPIName = hostApiInfo.Name // ex: "ALSA", "PulseAudio", "JACK"
}
result = append(result, output.DeviceInfo{
    ...
    HostAPI: hostAPIName,
})
```

### 7.3 — MODIFICAR `internal/api/handlers/devices.go`

Adicionar `HostAPI` ao DTO `AudioDevice`:

```go
type AudioDevice struct {
    ...
    HostAPI           string  `json:"host_api,omitempty"`
}
```

### 7.4 — ATUALIZAR `coreaudio.go`, `null.go`, `file.go`

- CoreAudio: `HostAPI: "CoreAudio"`
- NullOutput / FileOutput: `HostAPI: ""`

### 7.5 — Documentação

- `docs/specs/03-api-rest.md`: adicionar `host_api` na tabela de campos de `GET /v1/devices`
- `docs/specs/09-device-abstraction.md`: tabela de estabilidade por host API no Linux
- `README.md`: nota sobre comportamento no Linux por host API

**Resposta JSON resultante (Linux):**
```json
{
  "devices": [
    {
      "id":                   "Built-in Audio Analog Stereo",
      "name":                 "Built-in Audio Analog Stereo",
      "driver":               "portaudio",
      "host_api":             "ALSA",
      "is_default":           true,
      "max_output_channels":  2,
      "default_sample_rate":  48000.0
    }
  ]
}
```

**Impacto:** aditivo — campo novo com `omitempty`, nenhum cliente quebra.

---

## Fase 8 — Windows: driver WASAPI com IDs estáveis via COM

**Objetivo:** Implementar um driver nativo WASAPI que expõe o GUID persistente de cada dispositivo como `id`, e usa resolução em cascata (GUID → nome → default) no `Open()`.

### 8.1 — CRIAR `internal/audio/output/wasapi/`

Novo pacote com build tag `wasapi` (Windows + CGo):

```
internal/audio/output/wasapi/
  bridge.h          — declarações COM (IMMDeviceEnumerator, IMMDevice)
  bridge.c          — implementação: enumerate, getID, findByID, findByName, setDevice
  wasapi.go         — Output struct: Open/Start/Write/Stop/Close/Info/ListDevices
```

**Funções C principais:**
- `waEnumOutputDevices(WADeviceEntry *out, int maxCount)` — enumera com GUID, nome, canais, default
- `waFindDeviceByID(const char *guid, IMMDevice **out)` — busca por GUID estável
- `waFindDeviceByName(const char *name, IMMDevice **out)` — fallback por nome

**Struct C:**
```c
typedef struct {
    char   id[256];    // GUID — ex: "{0.0.0.00000000}.{1a2b3c...}"
    char   name[256];  // nome legível
    int    maxOutputChannels;
    double defaultSampleRate;
    int    isDefault;
} WADeviceEntry;
```

### 8.2 — Resolução em cascata no `Open()` do WASAPI

Mesmo padrão da Fase 6 (CoreAudio):
```
1. Tenta resolver cfg.DeviceID como GUID via waFindDeviceByID
2. Se não encontrar, tenta como nome via waFindDeviceByName
3. Se não encontrar, usa default + log de aviso
```

### 8.3 — CRIAR `cmd/playout-engine/output/factory_wasapi.go`

```go
//go:build wasapi

func NewOutputDevice(cfg *config.Config) (output.OutputDevice, error) {
    switch cfg.Audio.Output.Driver {
    case "wasapi":
        return wasapiout.New()
    case "null", "":
        return &output.NullOutput{}, nil
    case "file":
        return &output.FileOutput{}, nil
    default:
        return nil, fmt.Errorf("unknown driver %q", cfg.Audio.Output.Driver)
    }
}
```

### 8.4 — Documentação

- `docs/specs/09-device-abstraction.md`: seção WASAPI com GUID como ID estável + tabela de drivers por plataforma recomendada
- `docs/specs/03-api-rest.md`: exemplo de resposta Windows
- `README.md`: adicionar `wasapi` na tabela de drivers, com build tag e dependência

**Resposta JSON resultante (Windows + WASAPI):**
```json
{
  "devices": [
    {
      "id":                   "{0.0.0.00000000}.{1a2b3c4d-...}",
      "name":                 "Speakers (Realtek Audio)",
      "driver":               "wasapi",
      "host_api":             "WASAPI",
      "is_default":           true,
      "max_output_channels":  2,
      "default_sample_rate":  48000.0
    }
  ]
}
```

---

## Tabela de drivers recomendados por plataforma (pós-implementação)

| Plataforma | Driver recomendado | Build tag | ID estável? |
|---|---|---|---|
| macOS | `coreaudio` | `coreaudio` | Sim (UID) |
| Linux | `portaudio` | `portaudio` | Parcialmente (ALSA) |
| Windows | `wasapi` | `wasapi` | Sim (GUID) |
| Testes / CI | `null` | — | Sempre |

---

## Arquivos modificados — resumo

| Fase | Arquivo | Ação |
|---|---|---|
| 7 | `internal/audio/output/devices.go` | Adicionar campo `HostAPI` |
| 7 | `internal/audio/output/portaudio/portaudio.go` | Preencher `HostAPI` via `pa.HostApiInfo` |
| 7 | `internal/audio/output/coreaudio/coreaudio.go` | Preencher `HostAPI: "CoreAudio"` |
| 7 | `internal/audio/output/null.go` + `file.go` | `HostAPI: ""` (sem mudança comportamental) |
| 7 | `internal/api/handlers/devices.go` | Adicionar `host_api` ao DTO |
| 7 | `docs/specs/03-api-rest.md` | Novo campo na tabela |
| 7 | `docs/specs/09-device-abstraction.md` | Tabela de estabilidade Linux |
| 7 | `README.md` | Nota Linux por host API |
| 8 | `internal/audio/output/wasapi/bridge.h` | CRIAR |
| 8 | `internal/audio/output/wasapi/bridge.c` | CRIAR |
| 8 | `internal/audio/output/wasapi/wasapi.go` | CRIAR |
| 8 | `cmd/playout-engine/output/factory_wasapi.go` | CRIAR |
| 8 | `docs/specs/09-device-abstraction.md` | Seção WASAPI |
| 8 | `docs/specs/03-api-rest.md` | Exemplo Windows |
| 8 | `README.md` | Driver WASAPI na tabela |

---

## Verificação

### Fase 7
```bash
# Linux — verificar campo host_api na resposta
PLAYOUT_AUDIO_OUTPUT_DRIVER=portaudio ./playout-engine --startup=cli &
curl -s http://localhost:8080/v1/devices | jq '.[].host_api'
```

### Fase 8
```bash
# Windows — build com WASAPI
go build -tags wasapi ./...

# Verificar IDs são GUIDs
curl -s http://localhost:8080/v1/devices | jq '.[].id'
# esperado: "{0.0.0.00000000}.{...}"

# Testar cascata: configurar device_id com GUID → renomear device no SO → engine ainda encontra
```

---

## Notas de implementação

- **Fase 7** pode ser implementada e testada agora (macOS + Linux disponíveis)
- **Fase 8** requer ambiente Windows para compilar e testar; o CGo + COM tem complexidade similar ao CoreAudio — bridge C de ~150 linhas
- `HostAPI` com `omitempty` garante backward compat: clientes que não conhecem o campo simplesmente o ignoram
- A Fase 8 não afeta macOS nem Linux — build tag `wasapi` é exclusivo de Windows
