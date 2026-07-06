# CLAUDE.md — Radio Playout Engine

## Papel do Claude Code

Você está implementando um **Playout Engine em Go** para automação de rádio local FM/AM.

O projeto exige foco em:

- resiliência;
- baixo acoplamento;
- comunicação simples com UI;
- execução como processo separado;
- pipeline de áudio testável;
- suporte inicial a macOS, Linux e Windows.

## Regras fundamentais

1. **Não implemente UI neste repositório.**
2. **A UI nunca toca áudio.**
3. **A API nunca acessa mixer/output diretamente.**
4. **Tudo passa por Command Bus.**
5. **Todo estado visível deve vir do State Manager.**
6. **Eventos devem ser publicados no Event Bus.**
7. **Não use banco de dados no Engine nesta fase.**
8. **A fila inicial é em memória.**
9. **O Engine recebe paths e metadados já resolvidos.**
10. **Não implemente upload/cadastro de áudio neste projeto.**
11. **Hot path de áudio deve evitar alocações e locks longos.**
12. **Nunca bloqueie o áudio por causa de WebSocket, logs ou API.**

## Arquitetura obrigatória

Use esta arquitetura:

```text
API Server
  ↓
Command Bus
  ↓
Dispatcher
  ↓
Playback / Queue / Mode Managers
  ↓
Audio Pipeline
  ↓
Output Device
```

Eventos retornam pelo Event Bus:

```text
Managers / Audio Pipeline
  ↓
Event Bus
  ↓
WebSocket / Logs / State
```

## Estrutura de pastas recomendada

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

## Estilo de código

- Use Go idiomático.
- Prefira interfaces pequenas.
- Use `context.Context` em operações com I/O, decoder, API e shutdown.
- Evite singletons globais.
- Evite package cycles.
- Erros devem ser explícitos e contextualizados com `%w`.
- Use nomes claros.
- Não adicione dependências sem necessidade.
- Escreva testes para lógica de fila, estado, comandos e áudio offline.

## Versão do Go

Usar Go 1.24+ ou a versão estável configurada no projeto.

## Primeira milestone

Implementar apenas:

- config loader básico;
- logger;
- API server;
- `/v1/health`;
- `/v1/status`;
- command bus;
- event bus;
- state manager;
- queue manager em memória;
- endpoints de enqueue/play/stop/skip;
- NullOutput;
- FileOutput opcional;
- FFmpegDecoder;
- playback básico de um arquivo;
- WebSocket `/v1/events`.

Depois implementar crossfade.

## Contratos públicos

Siga os arquivos:

- `03-api-rest.md`
- `04-events-websocket.md`
- `05-state-machine.md`

Não quebre contratos sem atualizar a especificação.

## Estados

Estados principais:

```text
STARTING
IDLE
PLAYING
PAUSED
ASSIST
PANIC
STOPPING
ERROR
```

Modos:

```text
AUTO
ASSIST
PANIC
```

## Regras de comando

- `PANIC` tem prioridade máxima.
- `STOP` deve ser seguro e idempotente.
- `SKIP` pode ser rejeitado se o item atual for obrigatório.
- `PLAY` sem fila deve ser rejeitado com motivo claro.
- Todo comando deve gerar `command_id`.
- Todo comando aceito/rejeitado deve gerar evento.

## Audio pipeline

Formato interno recomendado:

```text
sample_rate: 48000
channels: 2
sample_format: float32
layout: interleaved stereo
```

Decoder inicial:

- FFmpeg via subprocesso.
- Output em PCM float32 little-endian.

Comando conceitual:

```bash
ffmpeg -hide_banner -loglevel error -i input.mp3 -f f32le -acodec pcm_f32le -ac 2 -ar 48000 pipe:1
```

## Output

Implemente primeiro:

- `NullOutput` para testes.
- Um adapter real isolado por interface.

Não acople playback diretamente a PortAudio, Oto, CoreAudio, ALSA ou WASAPI.

## WebSocket

- Não bloquear pipeline se cliente for lento.
- Eventos de progresso podem ser descartados se necessário.
- Eventos críticos não devem ser descartados silenciosamente.

## Testes obrigatórios

Antes de considerar tarefa concluída:

```bash
go test ./...
go vet ./...
```

Quando possível:

```bash
go test -race ./...
```

## Definição de pronto

Uma entrega está pronta quando:

- compila;
- possui testes relevantes;
- não cria package cycles;
- segue a arquitetura especificada;
- expõe eventos/estado quando altera comportamento observável;
- não implementa responsabilidades fora do Engine;
- não adiciona dependências desnecessárias.

## O que NÃO fazer

- Não criar UI.
- Não criar banco de dados.
- Não implementar cadastro/upload de áudio.
- Não fazer a API chamar mixer diretamente.
- Não fazer WebSocket bloquear áudio.
- Não usar `time.Sleep` como relógio principal do pipeline de áudio em produção.
- Não assumir separador `/` em paths.
- Não depender de estado global mutável.
- Não iniciar goroutines sem caminho claro de shutdown.

## Ordem recomendada para Claude Code

1. Criar estrutura de diretórios.
2. Criar tipos base: commands, events, state, queue item.
3. Criar config e logger.
4. Criar command bus e event bus.
5. Criar state manager.
6. Criar API server com health/status.
7. Criar queue manager.
8. Criar endpoints de queue.
9. Criar playback manager com NullOutput.
10. Criar FFmpegDecoder.
11. Criar output real atrás de interface.
12. Criar WebSocket events.
13. Criar progress tracking.
14. Criar crossfade.
15. Criar audio health.
16. Criar panic mode.
17. Criar hot buttons/ducking.

## Documentação viva

Sempre que alterar comportamento público:

- atualizar especificação correspondente;
- atualizar exemplos JSON;
- atualizar README se necessário.

## Planos de implementação

Todo plano ou proposta de implementação gerado pelo Claude Code deve ser armazenado em:

```
docs/plans/
```

Nunca criar arquivos de plano na raiz do projeto nem fora do repositório. Nomeie os arquivos de forma descritiva, por exemplo: `plan-feature-name.md` ou `proposal-feature-name.md`.
