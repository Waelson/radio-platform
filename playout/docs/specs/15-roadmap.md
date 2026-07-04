# 15 — Roadmap Incremental

## Fase 0 — Bootstrap

- Criar repo Go.
- Criar estrutura de pacotes.
- Criar `CLAUDE.md`.
- Criar config loader.
- Criar logger.
- Criar `/v1/health`.

## Fase 1 — Command/Event Bus

- Implementar command envelope.
- Implementar event envelope.
- Implementar dispatcher.
- Implementar state manager.
- Implementar `/v1/status`.

## Fase 2 — Queue

- Implementar queue manager em memória.
- Implementar enqueue.
- Implementar insert-next.
- Implementar clear.
- Eventos `QueueChanged`.

## Fase 3 — Output e Decoder

- Implementar `NullOutput`.
- Implementar `FileOutput` para testes.
- Implementar `FFmpegDecoder`.
- Implementar pipeline mínimo decoder → output.
- Tocar um arquivo.

## Fase 4 — Playback básico

- Play.
- Stop.
- Pause.
- Resume.
- Skip.
- Now playing.
- Progress.

## Fase 5 — WebSocket

- `/v1/events`.
- Publicar eventos de fila, estado, progresso e comandos.
- Reconnect seguro.

## Fase 6 — Crossfade

- Preload do próximo item.
- Crossfade linear.
- Evento `CrossfadeStarted`.
- Config default 8s.

## Fase 7 — Audio Health

- RMS/Peak.
- Silence detection.
- Buffer percentage.
- Underrun count.
- Eventos `AudioHealthChanged`.

## Fase 8 — Panic Mode

- Entrar em panic via API.
- Tocar panic bed.
- Auto panic por silêncio.
- Sair de panic.

## Fase 9 — Hot Buttons + PortAudio

> **Escopo expandido**: PortAudioOutput foi movido da Fase 10 para cá, pois output real
> é pré-requisito para validar ducking e overlay de forma audível.
>
> **Decisões de isolamento**:
> - PortAudio compilado apenas com build tag `portaudio` (`go build -tags portaudio`).
> - `go test ./...` (sem tag) continua usando NullOutput — zero impacto nos testes unitários.
> - Ducking e overlay implementados como mixing inline no PlaybackManager (mesmo padrão do crossfade).
> - Mixer formal fica para Fase 11.

### Hot Buttons
- Trigger overlay (duas streams simultâneas com mixing inline + ducking).
- Trigger interrupt (interrompe item atual, toca hot button, retorna à fila).
- Trigger after-current (insere próximo na fila).
- Ducking: redução de gain atômica no stream principal.
- Eventos `HotButtonTriggered`, `DuckingStarted`, `DuckingEnded`.
- API: `POST /v1/hotbuttons/trigger`.

### PortAudioOutput
- Implementar `internal/audio/output/portaudio/portaudio.go` com `//go:build portaudio`.
- Implementar factory em `main.go` que seleciona driver por `cfg.Audio.Output.Driver`.
- Pré-requisito do sistema: `brew install portaudio` (macOS) / `apt install libportaudio2-dev` (Linux).
- Adicionar `github.com/gordonklaus/portaudio` ao `go.mod` com tag de build.

## Fase 10 — Platform polish

- Testar macOS.
- Testar Linux.
- Testar Windows.
- Scripts de build (com e sem tag `portaudio`).
- Packaging.

## Fase 11 — Hardening

- Race tests.
- Load tests de API.
- Testes de arquivos corrompidos.
- Melhorias de retry.
- Métricas.

## Fase 12 — Recursos futuros

- Persistência opcional da fila.
- Scheduler integrado opcional.
- Better cue points.
- Auto crossfade por análise de energia.
- VU meter avançado.
- WASAPI/CoreAudio/ALSA nativos.
