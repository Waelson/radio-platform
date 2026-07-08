# Plano: CUE Player como Subprocesso Isolado

## Contexto e motivação

Todas as tentativas de eliminar o break no áudio principal ao iniciar um preview (CUE)
dentro do mesmo processo falharam. A causa raiz é estrutural:

- CoreAudio envia notificações HAL para **todos os AudioQueues ativos no mesmo processo Mach**
  quando qualquer um deles chama `AudioQueueStart()` ou sofre uma transição de dispositivo.
- No caso de dispositivo Bluetooth (A2DP), a sequência `AudioQueueStart()` no CUE
  dispara uma storm de notificações HAL que causa underrun breve no AudioQueue principal.
- Esse comportamento não é configurável — é uma propriedade do HAL do CoreAudio.

**A única solução definitiva é separação de processo.** Um processo separado = task Mach
separada = cliente HAL independente = notificações não se propagam entre eles.

---

## Arquitetura proposta

```
┌─────────────────────────────────────────────────────┐
│  playout-engine (processo principal)                │
│                                                     │
│  ┌──────────────┐    ┌──────────────────────────┐  │
│  │  Playback    │    │  CueProxy                │  │
│  │  (fila)      │    │  - spawn subprocess      │  │
│  │  CoreAudio   │    │  - stdin: comandos JSON  │  │
│  │  (BuiltIn)   │    │  - stdout: eventos JSON  │  │
│  └──────────────┘    └──────────┬───────────────┘  │
│                                 │ pipe stdin/stdout  │
└─────────────────────────────────┼───────────────────┘
                                  │ OS pipe (não HAL)
┌─────────────────────────────────┼───────────────────┐
│  playout-engine --mode=cue-player                   │
│                       │                             │
│              ┌────────┴──────────┐                 │
│              │  preview.Player   │                 │
│              │  CoreAudio        │                 │
│              │  (Bluetooth/BT)   │                 │
│              └───────────────────┘                 │
└─────────────────────────────────────────────────────┘
```

### Reutilização do binário existente

O mesmo binário `playout-engine` é usado, com um novo flag de modo:

```
playout-engine --mode=cue-player [--cue-device=<id>] [--cue-samplerate=48000] ...
```

Mesmo padrão já usado para webview:

| Flag               | Modo               | O que faz                             |
|--------------------|--------------------|---------------------------------------|
| *(nenhum)*         | ui (systray)       | Tray + spawna engine filho            |
| `--startup=cli`    | engine principal   | Engine completo                       |
| `--webview=<url>`  | webview subprocess | Janela WKWebView isolada              |
| `--mode=cue-player`| **CUE subprocess** | Player de preview isolado (novo)      |

---

## Protocolo de comunicação (stdin / stdout)

Comunicação via pipes POSIX — sem porta HTTP adicional, sem conflito de porta,
portável para macOS, Linux e Windows.

### Comandos (main engine → CUE, via stdin)

Cada linha é um objeto JSON seguido de `\n`:

```json
{"cmd": "play",   "path": "/musicas/track.mp3", "seek_ms": 0}
{"cmd": "pause"}
{"cmd": "resume"}
{"cmd": "stop"}
{"cmd": "seek",   "position_ms": 30000}
{"cmd": "quit"}
```

### Eventos (CUE → main engine, via stdout)

```json
{"event": "ready"}
{"event": "started",  "path": "/musicas/track.mp3", "duration_ms": 185000, "seek_ms": 0}
{"event": "progress", "position_ms": 1500,  "duration_ms": 185000}
{"event": "paused",   "position_ms": 15000, "duration_ms": 185000}
{"event": "resumed",  "position_ms": 15000, "duration_ms": 185000}
{"event": "stopped",  "reason": "end",   "position_ms": 185000}
{"event": "stopped",  "reason": "stop",  "position_ms": 15000}
{"event": "stopped",  "reason": "error", "position_ms": 0}
{"event": "seeked",   "position_ms": 30000, "duration_ms": 185000}
{"event": "error",    "message": "decoder open: exit status 1"}
```

O evento `ready` é publicado pelo CUE imediatamente após abrir o dispositivo de áudio
com sucesso — o main engine aguarda esse evento antes de enviar o primeiro `play`.

---

## Lifecycle e prevenção de processos órfãos

### Início

O CUE subprocess é iniciado na primeira vez que um comando `CmdPreviewPlay` é recebido
(lazy start), ou opcionalmente no startup do engine se `preview.preopen = true` na config.

### Parada normal

1. Main engine envia `{"cmd": "quit"}` via stdin.
2. CUE finaliza playback, fecha dispositivo, encerra processo com `os.Exit(0)`.
3. Main engine chama `cmd.Wait()` para colher o exit status.

### Parada por crash do main engine

Dois mecanismos em camadas:

1. **stdin EOF** (universal): quando o main engine morre, o pipe stdin do CUE é fechado
   pelo OS. O CUE detecta EOF no loop de leitura e encerra imediatamente.
   Implementado via `bufio.Scanner` — `scanner.Scan()` retorna `false` em EOF.

2. **`Pdeathsig: SIGTERM`** (Linux): via `SysProcAttr.Pdeathsig = syscall.SIGTERM`.
   O kernel envia SIGTERM ao filho quando o pai morre, independente de pipes.

3. **SIGKILL de fallback**: se CUE não encerrar em 5 segundos após `quit` ou SIGTERM,
   o main engine (ou seu defer) chama `cmd.Process.Kill()`.

### O que NÃO pode acontecer

- CUE **nunca** pode rodar sem o main engine estar vivo (garantido pelo stdin EOF).
- CUE **nunca** escuta porta HTTP própria (sem risk de conflito ou acesso externo).
- CUE **não tem** lock file — apenas o main engine tem lock de instância.

---

## Fases de implementação

---

### Fase 1 — Skeleton do subprocesso (plumbing)

**Objetivo:** Subprocesso reconhece `--mode=cue-player`, lê comandos de stdin,
escreve eventos em stdout. Sem áudio — apenas NullOutput.

**Arquivos:**

#### `cmd/playout-engine/main.go`

Adicionar parsing do flag `--mode=`:

```go
case len(a) > 7 && a[:7] == "--mode=":
    mode = a[7:]
```

E no corpo do `main()`:

```go
if mode == "cue-player" {
    cue.RunCuePlayer(filteredArgs)
    return
}
```

#### `internal/cue/runner.go` (novo pacote)

```go
package cue

// RunCuePlayer é o entry point do subprocesso --mode=cue-player.
// Lê comandos JSON de stdin, escreve eventos JSON em stdout.
// Encerra quando stdin fecha (parent morreu) ou recebe {"cmd":"quit"}.
func RunCuePlayer(args []string) { ... }
```

#### `internal/cue/proxy.go`

```go
package cue

// Proxy gerencia o subprocesso CUE a partir do main engine.
// Implementa as mesmas interfaces que preview.Player, substituindo-o.
type Proxy struct { ... }

func (p *Proxy) HandlePlay(_ context.Context, cmd commands.Command) error { ... }
func (p *Proxy) HandlePause(_ context.Context, _ commands.Command) error  { ... }
func (p *Proxy) HandleResume(_ context.Context, _ commands.Command) error { ... }
func (p *Proxy) HandleStop(_ context.Context, _ commands.Command) error   { ... }
func (p *Proxy) HandleSeek(_ context.Context, cmd commands.Command) error { ... }
func (p *Proxy) GetStatus() preview.Status                                { ... }
func (p *Proxy) Run(ctx context.Context)                                  { ... }
```

**Risco:** Baixo. Nenhum áudio ainda. Testa apenas o canal de comunicação.

---

### Fase 2 — Áudio no subprocesso

**Objetivo:** `RunCuePlayer` instancia um `preview.Player` real (com CoreAudio),
traduz comandos stdin para chamadas do Player, publica eventos Player para stdout.

**Arquivos:**

#### `internal/cue/runner.go`

```go
func RunCuePlayer(args []string) {
    cfg, _ := config.Load(args)
    log := slog.Default()

    // Output device: mesmo que preview.OutputDevice da config.
    out, _ := outfactory.NewPreviewOutputDevice(cfg)
    dec := decoder.NewFFmpegDecoder(log)
    evtBus := events.NewBus()
    player := preview.New(evtBus, dec, out, preview.AudioConfig{
        DeviceID:     cfg.Preview.OutputDevice,
        SampleRate:   cfg.Audio.SampleRate,
        Channels:     cfg.Audio.Channels,
        BufferFrames: cfg.Audio.BufferFrames,
    }, log)

    // Repassa eventos do EventBus para stdout como JSON.
    go forwardEventsToStdout(evtBus)

    // Loop de leitura de comandos do stdin.
    go player.Run(ctx)
    readCommandsFromStdin(player)
}
```

**Arquivos impactados:**
- `internal/cue/runner.go` — implementação completa
- `internal/cue/proxy.go` — spawn, write pipe, read pipe, publish EventBus

**Risco:** Médio. O CoreAudio no subprocess funciona de forma idêntica ao processo
principal, mas isolado. Possível falha na herança do PATH para `ffprobe`/`ffmpeg`
dentro do subprocess — mitigado usando `engine.ExpandedEnv()` ao spawnar.

---

### Fase 3 — Integração no main engine (substituição do preview.Player)

**Objetivo:** `cmd/playout-engine/main.go` usa `cue.Proxy` em vez de `preview.Player`.
O `Proxy` spawna o subprocesso, repassa comandos e eventos.

**Arquivos:**

#### `cmd/playout-engine/main.go`

```go
// Antes:
prevPlayer := preview.New(evtBus, dec, previewOut, preview.AudioConfig{...}, log)

// Depois:
prevPlayer := cue.NewProxy(evtBus, cue.Config{
    DeviceID:     cfg.Preview.OutputDevice,
    SampleRate:   cfg.Audio.SampleRate,
    Channels:     cfg.Audio.Channels,
    BufferFrames: cfg.Audio.BufferFrames,
}, log)
```

O `cue.Proxy.Run(ctx)` é chamado no mesmo goroutine slot que antes era `preview.Player.Run(ctx)`.

#### `internal/cue/proxy.go`

```go
type Config struct {
    DeviceID     string
    SampleRate   int
    Channels     int
    BufferFrames int
}

type Proxy struct {
    evtBus *events.Bus
    cfg    Config
    log    *slog.Logger

    mu      sync.RWMutex
    status  preview.Status
    subpid  int      // PID do subprocess, 0 se não rodando
    alive   bool

    stdin  io.WriteCloser  // pipe de escrita para o subprocess
    cancel context.CancelFunc
}
```

**Como o Proxy spawna o subprocess:**

```go
func (p *Proxy) spawn(ctx context.Context) error {
    self, _ := os.Executable()
    cmd := exec.CommandContext(ctx, self,
        "--mode=cue-player",
        "--startup=cli",
        // config flags transparentemente repassados
        fmt.Sprintf("--preview-device=%s", p.cfg.DeviceID),
        fmt.Sprintf("--audio-samplerate=%d", p.cfg.SampleRate),
        ...
    )
    cmd.Env = engine.ExpandedEnv()
    // stdin/stdout pipes para IPC
    p.stdin, _ = cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    cmd.Stderr = os.Stderr // logs do subprocess aparecem no mesmo log do engine

    setProcAttr(cmd)  // Setpgid + Pdeathsig (Linux)
    cmd.Start()
    p.subpid = cmd.Process.Pid
    p.alive = true

    go p.readEvents(stdout)
    go func() {
        cmd.Wait()
        p.mu.Lock()
        p.alive = false
        p.subpid = 0
        p.mu.Unlock()
    }()
    return nil
}
```

**Risco:** Médio. Se o subprocess falhar ao iniciar (ex: `ffmpeg` não encontrado),
o Proxy deve degradar para `NullOutput` internamente — sem pânico no engine principal.

---

### Fase 4 — Visibilidade do subprocesso

**Objetivo:** Expor o estado do CUE subprocess via API REST e no Status da UI.

#### `GET /v1/preview/status`

Novo endpoint (ou extensão de `GET /v1/status`):

```json
{
  "state":          "playing",
  "path":           "/musicas/track.mp3",
  "position_ms":    15000,
  "duration_ms":    185000,
  "subprocess": {
    "pid":   12345,
    "alive": true,
    "mode":  "subprocess"
  }
}
```

Campo `mode` pode ser `"subprocess"` (nova implementação) ou `"embedded"` (fallback
para `preview.Player` direto, se modo subprocess estiver desabilitado na config).

#### Config: novo campo

```yaml
preview:
  output_device: "EC-46-54-10-A5-4C:output"
  isolated: true   # true = subprocess; false = embedded (legado)
```

Permite voltar ao modo embedded via config, sem rebuild, para diagnóstico.

#### UI — Página de Status

Adicionar seção "CUE Player" com:
- Estado atual (idle/playing/paused)
- PID do subprocess e status (alive/dead)
- Dispositivo de saída configurado

**Risco:** Baixo. Endpoint novo, campo novo — backwards compatible.

---

### Fase 5 — Resiliência e reconexão automática

**Objetivo:** Se o subprocess CUE morrer inesperadamente (crash, OOM, dispositivo
BT desconectado), o Proxy detecta e oferece recuperação.

**Estratégias:**

1. **Restart automático**: se subprocess morrer e não havia `quit` explícito, Proxy
   aguarda 2 segundos e spawna novamente. Máximo de 3 tentativas consecutivas em
   janela de 30 segundos — depois, publica `CueSubprocessDown` e aguarda comando manual.

2. **Evento de erro no EventBus**: `EvtPreviewError` (novo tipo) com campo
   `reason: "subprocess_crash"` — UI pode exibir alerta.

3. **Reconexão de dispositivo BT**: ao spawnar, o subprocess tenta abrir o dispositivo
   configurado. Se falhar, publica `{"event":"error","message":"device unavailable"}` e
   Proxy fallback para NullOutput no subprocess (preview ainda funciona, mas sem áudio BT).

**Risco:** Baixo. Mecanismo defensivo, não altera fluxo normal.

---

## Riscos e mitigações

### R1 — Subprocess não herda PATH correto (macOS .app bundle)

**Descrição:** Quando rodando como `.app`, o PATH do processo pai é o PATH mínimo do
LaunchServices, sem `/opt/homebrew/bin`. O subprocess `--mode=cue-player` herdaria
esse PATH e não encontraria `ffmpeg`/`ffprobe`.

**Mitigação:** Usar `engine.ExpandedEnv()` ao spawnar o CUE subprocess — já existe
e é usado para webview e engine filho. Sem mudança nova necessária.

---

### R2 — Subprocess deixa dispositivo BT aberto indefinidamente

**Descrição:** Com o CUE subprocess vivo enquanto o engine roda, o dispositivo BT
fica em A2DP ativo 100% do tempo, mesmo sem preview tocando.

**Consequências:**
- Bateria do fone/caixa BT drena mais rápido.
- Dispositivo aparece "ocupado" para outros apps de áudio.

**Mitigação:**
- Documentar o comportamento no README e nas specs.
- Considerar parar o subprocess após N minutos de inatividade (configurável via
  `preview.idle_timeout_min: 10`). Subprocess é reiniciado automaticamente no próximo play.
- Alternativa: manter `PauseAudio()` no subprocess entre sessions, como já feito no
  modo embedded — dispositivo fica em "warm standby" com consumo mínimo.

---

### R3 — Latência de startup do subprocess no primeiro play

**Descrição:** Na primeira vez que CUE subprocess é iniciado (lazy start), há latência
de 50–300 ms para o processo iniciar, abrir o dispositivo BT e enviar `{"event":"ready"}`.
O usuário percebe um delay no primeiro preview.

**Mitigação:**
- Opção `preview.preopen: true` na config: subprocess é iniciado junto com o engine,
  sem aguardar o primeiro play. Custo: dispositivo BT sempre ocupado (ver R2).
- Alternativa: "warm start" — spawnar o subprocess no startup, mas fechar o dispositivo
  de áudio imediatamente; reabrir no primeiro play. Reduz delay de ~200ms para ~50ms.

---

### R4 — Subprocess acumula se spawnar falhar e matar silenciosamente

**Descrição:** Se `cmd.Start()` falhar, o Proxy retorna erro mas não fica em loop
tentando spawnar. Porém, se o subprocess crashar silenciosamente após `ready`, o Proxy
pode tentar spawnar infinitamente.

**Mitigação:**
- Circuit breaker: máximo de 3 restarts em 30 segundos (ver Fase 5).
- Log explícito em cada restart com contagem.
- Depois do limite, evento `EvtPreviewError` no EventBus + estado `CueDown`.

---

### R5 — Conflito de configuração: embedded vs subprocess

**Descrição:** A configuração `preview.isolated: true/false` permite escolher o modo.
Se um operador alternar entre modos sem restart, pode haver estado inconsistente.

**Mitigação:**
- Modo é lido apenas no startup do engine — mudança exige restart.
- Config UI mostra claramente "restart necessário" ao alterar esse campo.
- Padrão: `isolated: true` em macOS (onde BT glitch é crítico); `isolated: false`
  em Linux/Windows onde o problema não foi observado.

---

### R6 — Subprocesso CUE não aparece no lock file / instância única

**Descrição:** O lock file protege contra múltiplas instâncias do engine principal.
O subprocess CUE não possui lock próprio. Se o main engine for reiniciado sem cleanup
adequado, pode haver dois subprocessos CUE.

**Mitigação:**
- Proxy always envia `{"cmd":"quit"}` ao subprocess antes de criar novo.
- O stdin pipe fecha automaticamente quando Proxy é garbage collected.
- Subprocess usa stdin EOF como sinal de morte — não sobrevive ao Proxy.

---

## Impacto nas UIs

### Página de Status (803×430)

**Alterações necessárias:**
- Adicionar card "CUE Player" com:
  - Estado: IDLE / PLAYING / PAUSED / STOPPED / OFFLINE
  - Arquivo em reprodução (se playing/paused)
  - Barra de progresso
  - PID do subprocess (modo debug, opcional)
- Já consome `GET /v1/preview/status` — basta adicionar campo `subprocess.alive`

**Nenhuma mudança visual obrigatória** para que a funcionalidade core funcione.
O subprocess é transparente para a UI — os WebSocket events continuam sendo os mesmos.

### Página de Configuração (1095×741)

**Alterações necessárias:**
- Nova seção "Preview / CUE Player":
  - Campo `isolated`: toggle "Subprocess isolado / Embutido" com nota de restart
  - Campo `output_device`: dropdown de dispositivos (já existe)
  - Campo `idle_timeout_min`: tempo de inatividade antes de encerrar subprocess (opcional)
- Indicador de estado do subprocess: "CUE subprocess: ativo (PID 12345)" ou "inativo"

### WebSocket (sem quebra de contrato)

Os eventos existentes continuam sem mudança de envelope:
- `PreviewStarted`, `PreviewProgress`, `PreviewPaused`, `PreviewResumed`,
  `PreviewStopped`, `PreviewSeeked`

Novo evento opcional (pode ser adicionado sem quebrar clientes existentes):
```json
{
  "type": "PreviewSubprocessChanged",
  "payload": {
    "pid":   12345,
    "alive": true,
    "reason": "started"
  }
}
```

---

## Documentação a atualizar

| Documento | O que atualizar |
|---|---|
| `README.md` | Diagrama de processos; seção "Preview/CUE"; build tags |
| `docs/specs/02-process-model.md` | Novo diagrama com CUE subprocess; lifecycle |
| `docs/specs/03-api-rest.md` | `GET /v1/preview/status` com campo `subprocess` |
| `docs/specs/04-events-websocket.md` | Evento `PreviewSubprocessChanged` |
| `docs/specs/12-configuration.md` | Campos `preview.isolated`, `preview.idle_timeout_min` |
| `docs/specs/13-platform-support.md` | Nota sobre BT glitch em macOS e solução subprocess |
| `docs/plans/plan-preview-preopen.md` | Nota: Fases 1-3 implementadas mas insuficientes; substituídas por este plano |

---

## Diagrama de processo atualizado (pós-implementação)

```text
┌──────────────────────────────────────────┐
│  playout-engine (UI/systray)             │
│  - System tray                           │
│  - Webview windows                       │
└──────────────┬───────────────────────────┘
               │ spawn --startup=cli
               ▼
┌──────────────────────────────────────────┐
│  playout-engine --startup=cli            │
│  (engine principal)                      │
│                                          │
│  ┌──────────────┐  ┌──────────────────┐  │
│  │  Playback    │  │  CueProxy        │  │
│  │  Manager     │  │  (stdin/stdout)  │  │
│  └──────────────┘  └────────┬─────────┘  │
│   CoreAudio BuiltIn          │ OS pipe    │
└──────────────────────────────┼───────────┘
                               │ spawn --mode=cue-player
                               ▼
┌──────────────────────────────────────────┐
│  playout-engine --mode=cue-player        │
│  (CUE subprocess)                        │
│                                          │
│  ┌──────────────────────────────────┐    │
│  │  preview.Player                  │    │
│  │  CoreAudio BT/A2DP               │    │
│  └──────────────────────────────────┘    │
└──────────────────────────────────────────┘
```

---

## Resumo das fases

| Fase | O que entrega | Risco | Impacto no áudio principal |
|---|---|---|---|
| 1 | Skeleton: `--mode=cue-player`, IPC stdin/stdout, NullOutput | Baixo | Nenhum |
| 2 | Áudio real no subprocess (CoreAudio/PortAudio isolado) | Médio | Elimina break (objetivo) |
| 3 | Integração no main engine: `cue.Proxy` substitui `preview.Player` | Médio | Transparente para API |
| 4 | Visibilidade: `GET /v1/preview/status` + UI status | Baixo | Nenhum |
| 5 | Resiliência: restart automático, circuit breaker, idle timeout | Baixo | Nenhum |

As fases 1–3 são o núcleo e devem ser implementadas em sequência.
As fases 4 e 5 são melhorias incrementais que podem ser entregues depois.

---

## Decisões de arquitetura registradas

| Decisão | Alternativa descartada | Motivo |
|---|---|---|
| stdin/stdout JSON como IPC | HTTP local no subprocess | Sem conflito de porta, portável, sem risco de acesso externo |
| stdin/stdout JSON como IPC | Pipe nomeado / Unix socket | Mais simples, mesmo nível de isolamento |
| stdin EOF como sinal de morte | PPID watchdog (polling) | Reativo vs proativo; EOF é imediato, polling tem latência |
| Reutilizar binário existente | Binário separado `playout-cue` | Sem duplicate de código, sem sincronização de versão |
| Lazy start (primeiro play) | Eager start (startup do engine) | Não ocupa dispositivo BT sem necessidade |
| `preview.isolated: true/false` | Hard-coded sempre subprocess | Permite fallback para diagnóstico sem rebuild |
