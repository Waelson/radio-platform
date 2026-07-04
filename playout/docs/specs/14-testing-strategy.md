# 14 — Estratégia de Testes

## Objetivo

Garantir que o Engine seja testável sem depender de dispositivo de áudio real.

## Pirâmide de testes

```text
Unit tests
Integration tests
Audio golden tests
Manual device tests
```

## Unit tests

Cobrir:

- Queue Manager.
- State Machine.
- Command validation.
- Event bus.
- Config parsing.
- Gain conversion.
- Crossfade curve.
- Silence detection.

## NullOutput

Todos os testes automatizados devem poder rodar com `NullOutput`.

`NullOutput` deve:

- aceitar frames;
- contar frames escritos;
- simular erro se configurado;
- não depender de device.

## FileOutput

Usar para testes de áudio.

Fluxo:

```text
input A + input B → crossfade → output.wav
```

Validar:

- tamanho esperado;
- ausência de clipping;
- ganhos no período de fade;
- continuidade sem gap.

## Testes de API

Usar `httptest`.

Cenários:

- `/health` responde OK.
- enqueue item válido.
- rejeita payload inválido.
- play sem fila é rejeitado.
- skip em item obrigatório é rejeitado.
- status reflete estado.

## Testes de WebSocket

Cenários:

- cliente conecta;
- recebe `EngineStarted` ou snapshot;
- recebe `QueueChanged` após enqueue;
- recebe `NowPlayingChanged` após play;
- reconexão não quebra Engine.

## Testes de estado

Cenários:

- IDLE → PLAYING.
- PLAYING → PAUSED → PLAYING.
- PLAYING → PANIC.
- PANIC → IDLE.
- ERROR → RESET → IDLE.

## Testes de resiliência

Cenários:

- arquivo inexistente.
- decoder falha.
- output falha.
- fila vazia.
- múltiplos comandos concorrentes.
- WebSocket lento.

## Testes manuais por plataforma

### macOS

- Listar devices.
- Tocar MP3.
- Tocar WAV.
- Play/stop/skip via curl.
- WebSocket funcionando.
- Crossfade audível.

### Linux

Mesmo conjunto.

### Windows

Mesmo conjunto.

## Comandos de qualidade

```bash
go test ./...
go vet ./...
go test -race ./...
```

## Critérios de aceite MVP

- Engine inicia sem UI.
- `/v1/health` responde.
- Enqueue aceita item válido.
- Play toca áudio.
- Stop para áudio.
- Skip avança.
- Status mostra now playing e posição.
- WebSocket publica eventos.
- Crossfade ocorre faltando 8s.
- Panic toca bed de segurança.
