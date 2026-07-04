# 01 — Arquitetura

## Arquitetura de alto nível

```text
┌──────────────────────────────┐
│              UI              │
│ React / Angular / Desktop    │
└───────────────┬──────────────┘
                │ REST + WebSocket
                ▼
┌──────────────────────────────┐
│        Playout Engine         │
│ Go process                    │
│                              │
│ ┌──────────────────────────┐ │
│ │ API Server               │ │
│ └───────────┬──────────────┘ │
│             ▼                │
│ ┌──────────────────────────┐ │
│ │ Command Bus              │ │
│ └───────────┬──────────────┘ │
│             ▼                │
│ ┌──────────────────────────┐ │
│ │ Playback Manager         │ │
│ └───────────┬──────────────┘ │
│             ▼                │
│ ┌──────────────────────────┐ │
│ │ Audio Pipeline           │ │
│ │ Decoder → Mixer → Output │ │
│ └───────────┬──────────────┘ │
│             ▼                │
│ ┌──────────────────────────┐ │
│ │ Device Adapter           │ │
│ └──────────────────────────┘ │
└───────────────┬──────────────┘
                │
                ▼
        Audio Device / OS
```

## Componentes internos

### API Server

Responsável por:

- Expor REST local.
- Expor WebSocket de eventos.
- Validar payloads.
- Criar comandos internos.
- Retornar ACK/REJECT.

Não deve tocar áudio nem acessar diretamente o mixer.

### Command Bus

Canal interno de comandos. Todas as ações passam por ele.

Exemplos:

- `ENQUEUE`
- `PLAY`
- `STOP`
- `SKIP`
- `ENTER_PANIC`
- `TRIGGER_HOT_BUTTON`

### Event Bus

Canal interno de eventos. Eventos são publicados para:

- WebSocket.
- Logs.
- Métricas.
- Debug.

Exemplos:

- `PlayerStateChanged`
- `QueueChanged`
- `NowPlayingChanged`
- `ProgressChanged`
- `AudioLevelChanged`
- `CommandAccepted`
- `CommandRejected`
- `PanicEntered`

### State Manager

Mantém snapshot consistente do estado do Engine.

Deve permitir leitura segura para `/status` e WebSocket.

### Queue Manager

Mantém fila em memória.

Responsável por:

- Inserir itens.
- Remover itens.
- Avançar para próximo.
- Proteger itens em execução.
- Emitir eventos de mudança.

### Playback Manager

Responsável por:

- Orquestrar execução da fila.
- Iniciar decoder.
- Iniciar crossfade.
- Tratar fim de item.
- Reagir a skip/stop/panic.

### Decoder Manager

Responsável por abrir arquivos e gerar PCM no formato interno do Engine.

Primeira implementação recomendada:

- `FFmpegDecoder` via processo externo.

Interface deve permitir substituir por decoder nativo depois.

### Mixer

Responsável por:

- Somar canais.
- Aplicar gain.
- Crossfade.
- Ducking.
- Hot buttons.
- Controle de volume.

### Output Device Adapter

Responsável por escrever PCM no dispositivo de áudio.

Deve ser implementado atrás de interface.

### Audio Health Monitor

Responsável por calcular:

- nível RMS/Peak;
- silêncio;
- nível do buffer;
- underrun;
- falhas do output.

## Modelo de concorrência

O Engine deve ser orientado a mensagens:

```text
API → Command Bus → Dispatcher → Managers → Event Bus → UI
```

Evitar chamadas diretas entre UI/API e componentes de áudio.

## Pacotes sugeridos

```text
/cmd/playout-engine
/internal/api
/internal/commands
/internal/events
/internal/state
/internal/queue
/internal/playback
/internal/audio
/internal/audio/decoder
/internal/audio/mixer
/internal/audio/output
/internal/health
/internal/config
/internal/logging
/internal/platform
/internal/testutil
```

## Regra contra acoplamento

O pacote `api` não pode importar `audio/mixer` ou `audio/output` diretamente.

Fluxo permitido:

```text
api → commands → dispatcher → playback → audio
```

## Dependências externas

Preferir poucas dependências. Toda dependência deve ser justificada.

Categorias aceitáveis:

- HTTP/router leve ou stdlib.
- WebSocket.
- Logging estruturado.
- Audio output adapter.
- Test assertions.

## Regras de evolução

- Não quebrar contratos públicos sem versionar API.
- Manter `/v1` nos endpoints.
- Novos recursos devem entrar por comandos e eventos.
- Evitar estado global.
- Tudo que for hot path de áudio deve evitar alocação desnecessária.
