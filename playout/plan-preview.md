# Plano: Preview de Áudio (Cue Player)

## Contexto

O preview permite que o locutor ouça um áudio em um dispositivo separado antes de
colocá-lo na fila de reprodução, sem interferir no sinal ao ar. O mecanismo é
completamente isolado do pipeline principal de playback.

---

## Fases

### Fase 1 — Config + Tipos (concluída ✅)

Adicionar suporte de configuração e os tipos de comandos/eventos necessários.

**Arquivos alterados:**
- `internal/config/config.go` — `PreviewConfig` struct adicionada ao `Config`
- `internal/config/loader.go` — defaults e env vars (`RADIOCORE_PREVIEW_*`)
- `playout-engine.yaml` — seção `preview:` com `enabled`, `output_driver`, `output_device`
- `internal/commands/types.go` — `CmdPreviewPlay`, `CmdPreviewPause`, `CmdPreviewResume`, `CmdPreviewStop`, `CmdPreviewSeek` + payloads
- `internal/events/types.go` — `EvtPreviewStarted`, `EvtPreviewPaused`, `EvtPreviewResumed`, `EvtPreviewStopped`, `EvtPreviewProgress`, `EvtPreviewSeeked` + payloads

---

### Fase 2 — Decoder e Source (concluída ✅)

Integração com FFmpeg para decodificação de áudio no preview. O seek é implementado
via `CueInMS` no `decoder.Source`, reaproveitando a infraestrutura existente.

---

### Fase 3 — PreviewPlayer (concluída ✅)

Implementar o pacote `internal/preview` com o player isolado.

**Arquivos a criar:**
- `internal/preview/player.go` — `PreviewPlayer` struct com:
  - State machine: `IDLE` → `PLAYING` → `PAUSED` → `IDLE`
  - Métodos: `HandlePlay`, `HandlePause`, `HandleResume`, `HandleStop`, `HandleSeek`
  - Loop de playback em goroutine dedicada usando `decoder.FFmpegDecoder`
  - Progress events a cada ~100ms
  - Publicação de eventos no `events.Bus`
- `internal/preview/status.go` — `Status` struct com `State`, `Path`, `PositionMS`, `DurationMS`

---

### Fase 4 — Output Device Factory para Preview (concluída ✅)

Criar factory para instanciar o dispositivo de saída do preview a partir da config.

**Arquivos a criar:**
- `cmd/playout-engine/output/preview_factory.go` (ou similar) — `NewPreviewOutputDevice(cfg)`
  - Suporta os mesmos drivers do output principal: `null`, `coreaudio`, `portaudio`, `file`
  - Usa `cfg.Preview.OutputDriver` e `cfg.Preview.OutputDevice`

---

### Fase 5 — Wirear em `main.go` e Dispatcher (concluída ✅)

Integrar o PreviewPlayer no processo principal.

**Arquivos alterados:**
- `cmd/playout-engine/main.go`:
  - Instanciar output de preview via factory
  - Criar `preview.NewPlayer(evtBus, dec, previewOut, cfg.Preview, log)`
  - Registrar handlers no dispatcher:
    - `disp.Handle(commands.CmdPreviewPlay,   previewPlayer.HandlePlay)`
    - `disp.Handle(commands.CmdPreviewPause,  previewPlayer.HandlePause)`
    - `disp.Handle(commands.CmdPreviewResume, previewPlayer.HandleResume)`
    - `disp.Handle(commands.CmdPreviewStop,   previewPlayer.HandleStop)`
    - `disp.Handle(commands.CmdPreviewSeek,   previewPlayer.HandleSeek)`
  - Iniciar `previewPlayer.Run(ctx)` em goroutine

---

### Fase 6 — Endpoints da API REST (concluída ✅)

Expor o preview via HTTP.

**Endpoints:**
| Método | Path | Descrição |
|--------|------|-----------|
| `POST` | `/v1/preview/play` | Inicia preview (body: `path`, `seek_ms`) |
| `POST` | `/v1/preview/pause` | Pausa o preview em andamento |
| `POST` | `/v1/preview/resume` | Retoma o preview pausado |
| `POST` | `/v1/preview/stop` | Para o preview |
| `POST` | `/v1/preview/seek` | Salta para posição (body: `position_ms`) |
| `GET`  | `/v1/preview/status` | Retorna estado atual do preview |

**Arquivos a criar/alterar:**
- `internal/api/handlers/preview.go` — handlers HTTP
- `internal/api/router.go` — registrar rotas (retorna 503 quando `cfg.Preview.Enabled == false`)

---

### Fase 7 — Documentação (concluída ✅)

- Atualizar `README.md` com a seção de preview
- Criar `docs/specs/preview.md` com especificação completa do protocolo de eventos

---

### Fase 8 — Player UI: Botão e Painel de Preview

Adicionar botão `⌄` em cada item da fila no Player (Electron/React) que expande um
painel inline de preview.

**Componentes:**
- Botão de toggle por item da fila
- Painel colapsável com waveform/progress bar do preview

---

### Fase 9 — Controles de Preview no Player

Implementar os controles de playback no painel de preview do Player.

**Controles:**
- Play / Pause / Stop
- Chamadas REST para os endpoints `/v1/preview/*`

---

### Fase 10 — Progress Bar e Seek no Player

- Barra de progresso em tempo real (via eventos WebSocket `PreviewProgress`)
- Seek via clique/drag na progress bar (chama `POST /v1/preview/seek`)

---

## Verificação por fase

```bash
# Fase 3 — compilar sem erros
cd playout
go build -tags coreaudio ./...

# Fase 5 — engine sobe e processa comandos de preview
./radiocore --config playout-engine.yaml

# Fase 6 — testar via curl
curl -X POST http://localhost:8080/v1/preview/play \
  -H "Content-Type: application/json" \
  -d '{"path":"/caminho/para/audio.mp3","seek_ms":0}'

curl http://localhost:8080/v1/preview/status
```
