# Plano: Captura de Entrada de Linha (Line-In / Fonte de Áudio Externa)

**Status:** proposta
**Módulos impactados:** `playout/`, `player/`
**Branch sugerida:** `feature/linein-external-source`

---

## 1. O Problema

### 1.1 Contexto regulatório brasileiro

O Decreto-Lei nº 236/1967 e o Código Brasileiro de Telecomunicações tornam
**obrigatória** a retransmissão do programa **A Voz do Brasil** em emissoras
de rádio AM e FM. O programa é transmitido diariamente (de segunda a sexta)
pela EBC — Rádio Gov e precisa entrar ao vivo no ar às 19h00, com duração de
até 60 minutos.

O modelo operacional vigente na maioria das emissoras é:

1. Um rádio AM/FM é sintonizado na frequência da emissora retransmissora.
2. A saída de áudio desse rádio é conectada via cabo P2 ou RCA na **entrada de
   linha (line-in)** da placa de som do servidor de automação.
3. O software de automação precisa **detectar o horário, parar ou suspender o
   que está tocando, capturar o áudio da entrada de linha e roteá-lo para a
   saída principal** enquanto o programa estiver no ar.
4. Ao final, o sistema retoma a programação normal de forma automática.

### 1.2 O que o sistema atual não resolve

O playout engine atual:

- Só conhece arquivos de áudio locais (paths resolvidos pela biblioteca).
- Não tem conceito de "fonte de áudio externa" — toda reprodução parte de um
  subprocesso FFmpeg lendo um arquivo em disco.
- Não tem estado `LINE_IN` na máquina de estados.
- Não expõe API para iniciar/parar captura de entrada.
- Não permite ao scheduler disparar esse tipo de evento.
- Não há controles ou indicadores na UI do player para esta situação.

Sem essa feature, o operador precisa usar um mixer externo ou software
adicional paralelo ao playout para cumprir a obrigação legal, o que aumenta
custo, complexidade operacional e risco de falha humana.

### 1.3 Outros cenários onde a feature é necessária

Além da Voz do Brasil, a captura de linha-in é usada em:

| Cenário | Descrição |
|---|---|
| Retransmissão de rede | Emissoras afiliadas que retransmitem programas da cabeça-de-rede (ex.: Jovem Pan, CBN) em horários fixos |
| Programa ao vivo com apresentador | O áudio do estúdio chega via linha-in enquanto o apresentador está no ar |
| Entradas externas eventuais | Shows ao vivo, cobertura de eventos, notícias urgentes |
| Backup de emergência | Se a biblioteca falhar, o operador pode ligar um rádio na linha-in como plano B |
| Retransmissão de streaming | Com um cabo virtual (ex.: VB-Cable), é possível capturar streams IP como se fossem linha-in |

---

## 2. Referências do Mercado

### 2.1 Soluções nacionais

**Playlist Digital (Playlist Solutions — Brasil)**
Software de automação muito usado em rádios brasileiras.
Implementa linha-in como um "evento" especial na programação:
- Possui um campo de configuração de dispositivo de entrada nas Opções → Áudio.
- Insere um item do tipo "Line-In" na playlist com duração configurável.
- Ao chegar no item, muda automaticamente a fonte de áudio da saída principal
  para o dispositivo de linha configurado.
- Suporta uso de VB-Cable para capturar streams IP como se fossem linha-in.
- Referência: tutorial oficial "Configurando o Playlist Digital para exibir a
  Voz do Brasil automaticamente no horário".

**ZaraRadio (distribuição no Brasil via Taaqui)**
Software free muito popular em rádios comunitárias.
- Implementa "entrada de linha" como um item especial do tipo `LINE_IN` na playlist.
- Dispara no horário configurado via scheduler interno.
- A retomada da programação é manual (o operador clica em "Continuar").
- Não tem monitoração de nível do sinal externo.

**Mídiaplay Produtora**
- Oferece solução específica para automação da Voz do Brasil, baseada em
  configuração de entrada de áudio na placa de som + agendamento por software.

### 2.2 Soluções internacionais

**RadioBOSS (DJSoft — Russia/Internacional)**
Um dos softwares mais completos do segmento.
- Implementa linha-in via menu Playlist → Add Line Input.
- Permite definir duração do bloco de linha-in.
- Usa o dispositivo de entrada configurado em Settings → Microphone/Line.In.
- Suporta "Virtual Sound Cards" para capturar fontes de áudio de outros softwares.
- O Manual RadioBOSS documenta: "Line input is useful for retransmitting a
  signal received via the sound card's input (e.g., a satellite feed)."

**mAirList (Alemanha)**
Software profissional amplamente adotado em rádios europeias.
- Implementa linha-in como um item de playlist do tipo `"Live Feed"`.
- A fonte de áudio é configurada em Configuration → Audio Routing → Recording Tab.
- Suporta múltiplos dispositivos de entrada com roteamento granular.
- Permite misturar o sinal de linha com o áudio automático (ducking/crossfade).
- Referência: mAirList User Manual — Audio Routing e Encoder Audio Routing.

**Rivendell (Linux — Open Source)**
Sistema profissional de código aberto, muito usado em emissoras públicas e universitárias.
- Tem um módulo específico chamado **RDCatch** (automatic recorder and task scheduler)
  para captura de áudio externo.
- Usa placas AudioScience HPI para captura com alta confiabilidade.
- O playout principal (RDAirPlay) pode ser configurado para aceitar um "macro event"
  que muda a fonte de saída para linha-in em um horário específico.
- A captura e o playout são modulares e comunicam via banco de dados MySQL.

**Axia Livewire / AES67 (Telos Alliance — EUA)**
Padrão de rádio IP profissional de alto nível.
- Usa multicast UDP para rotear áudio de "qualquer fonte para qualquer destino" na LAN.
- A linha-in em uma workstation é transformada em um stream Livewire e pode ser
  roteada para o playout via software driver.
- Usado em grandes emissoras como CNN, BBC afiliadas, NPR.

**OmniPlayer (M&I Broadcast Services — Países Baixos)**
- Permite captura simultânea de múltiplas fontes (CDs, fitas, links ao vivo, chamadas telefônicas).
- Integra captura de linha-in diretamente no workflow de produção.

### 2.3 Padrão identificado no mercado

Todos os sistemas acima convergem para o **mesmo modelo conceitual**:

1. **Configuração prévia** do dispositivo de entrada (uma vez, nas configurações).
2. **Item especial de linha-in** na playlist/fila (tipo diferente de uma música).
3. **Goroutine/thread dedicada** que captura o áudio da entrada e o envia direto para a saída.
4. **Duração** configurável (ou parada manual).
5. **Retomada automática** da fila de automação ao término.
6. **Monitoração de nível** do sinal de entrada (silêncio detectado = alerta).

---

## 3. Proposta de Solução

### 3.1 Visão geral

Adicionar ao playout engine um novo mecanismo de **captura e passagem de áudio**
(audio passthrough) usando FFmpeg como capturador — exatamente o mesmo modelo já
usado pelo `FFmpegDecoder` para arquivos, mas com uma fonte de dispositivo de
hardware em vez de um arquivo em disco.

O FFmpeg já suporta captura de dispositivos de áudio em todas as plataformas
relevantes:

| Plataforma | Formato FFmpeg | Exemplo |
|---|---|---|
| macOS | `avfoundation` | `-f avfoundation -i ":0"` |
| Linux | `alsa` | `-f alsa -i hw:0,0` |
| Windows | `dshow` | `-f dshow -i audio="Line In"` |

O áudio capturado é convertido para o formato interno do engine (PCM float32 LE,
48 kHz, stereo) e escrito diretamente no `OutputDevice` ativo, sem passar pela
fila de reprodução.

### 3.2 Princípios de design

- A captura de linha-in **nunca aloca no hot path** de áudio.
- A fila de reprodução permanece intacta durante o line-in (pausada, não descartada).
- O `PANIC` mode tem prioridade absoluta e interrompe o line-in se ativado.
- Toda mudança de estado gera evento no Event Bus (observabilidade completa via WebSocket).
- A feature é **opt-in** — emissoras que não usam linha-in não são afetadas.
- O dispositivo de entrada é configurável via API e persiste em config/prefs.

---

## 4. Arquitetura em Alto Nível

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                          PLAYOUT ENGINE                                      ║
║                                                                              ║
║  ┌────────────┐   comando    ┌─────────────┐   despacha   ┌──────────────┐  ║
║  │  REST API  │ ──────────► │ Command Bus │ ────────────► │  Dispatcher  │  ║
║  │ /v1/linein │             └─────────────┘               └──────┬───────┘  ║
║  │ /start     │                                                   │          ║
║  │ /stop      │                                          ┌────────▼────────┐ ║
║  │ /status    │          ┌──────────────┐                │ LineInManager   │ ║
║  │ /devices   │          │ State Manager│◄── setState ── │                 │ ║
║  └────────────┘          │  LINE_IN     │                │ .Start(cfg)     │ ║
║                          │  IDLE        │                │ .Stop()         │ ║
║  ┌────────────┐          │  PLAYING     │                │ .Status()       │ ║
║  │ Scheduler  │          └──────────────┘                └────────┬────────┘ ║
║  │ cron 19h00 │                                                   │          ║
║  │ cron 20h00 │          ┌──────────────┐   eventos   ┌──────────▼────────┐ ║
║  └─────┬──────┘          │  Event Bus   │◄──────────  │ FFmpegCapture     │ ║
║        │                 │              │             │                   │ ║
║        │ CmdLineInStart  │ linein.start │             │ ffmpeg subprocess  │ ║
║        └────────────────►│ linein.stop  │             │ (avfoundation /   │ ║
║                          │ linein.error │             │  alsa / dshow)    │ ║
║                          └──────┬───────┘             └────────┬──────────┘ ║
║                                 │                              │ PCM f32    ║
║                          ┌──────▼───────┐                      │ frames     ║
║                          │  WebSocket   │             ┌────────▼──────────┐ ║
║                          │  /v1/events  │             │  OutputDevice     │ ║
║                          └──────────────┘             │  (CoreAudio/      │ ║
║                                                       │   WASAPI/ALSA)    │ ║
╚══════════════════════════════════════════════════════ └───────────────────┘ ╝
          ▲ WebSocket events                                    ▲ áudio físico
          │                                                     │
╔═════════╧═══════════════════╗              ╔═════════════════╧══════════════╗
║  player/player.html (UI)    ║              ║  Placa de Som (Hardware)       ║
║                             ║              ║                                ║
║  • Badge "LINE-IN ATIVO"    ║              ║  OUTPUT ──► Transmissor FM/AM  ║
║  • Botão Iniciar/Parar      ║              ║  INPUT  ◄── Rádio (Voz Brasil) ║
║  • Seletor de dispositivo   ║              ╚════════════════════════════════╝
║  • Monitor de nível VU      ║
╚═════════════════════════════╝
```

### 4.1 Fluxo de dados — captura ativa

```
Placa de Som (line-in)
  │
  │  amostras analógicas → digitais
  ▼
FFmpeg subprocess
  │  -f avfoundation -i ":0" -f f32le -acodec pcm_f32le -ac 2 -ar 48000 pipe:1
  ▼
stdout pipe  (PCM float32 LE, 48 kHz, stereo)
  │
  ▼
FFmpegCapture.Read()    ← goroutine dedicada dentro de LineInManager
  │
  │  []float32  (buffer de 2048 frames = ~42ms a 48kHz)
  ▼
OutputDevice.Write()    ← mesmo device usado pelo playback normal
  │
  ▼
Placa de Som (output) → Transmissor
```

### 4.2 Fluxo de controle — linha do tempo

```
18:59:59  Scheduler (cron "0 19 * * 1-6") dispara
          ↓
19:00:00  CmdLineInStart → Dispatcher
          ↓
          LineInManager.Start()
            → SetState(LINE_IN)
            → Pausa o playback da fila (CmdPause ou suspensão interna)
            → Inicia subprocesso FFmpeg capturando device_id
            → Goroutine: loop Read→Write até ctx cancelado
            → Publica EvtLineInStarted
          ↓
19:00:01  UI recebe "linein.started" via WebSocket
          → Badge "LINE-IN ATIVO" aparece
          → Fila de reprodução mostra estado suspenso
          ↓
...  (60 minutos de transmissão da Voz do Brasil)
          ↓
20:00:00  Scheduler (cron "0 20 * * 1-6") dispara
          → CmdLineInStop → Dispatcher
          ↓
          LineInManager.Stop()
            → Cancela context do FFmpeg → subprocess termina
            → SetState(PLAYING ou IDLE conforme fila)
            → Publica EvtLineInStopped
          ↓
20:00:01  UI recebe "linein.stopped"
          → Badge desaparece
          → Programação automática retoma
```

---

## 5. Novos Componentes — Detalhamento Técnico

### 5.1 `internal/linein/` — pacote novo

#### `device.go`

```go
package linein

import "context"

// InputDevice é o contrato que todo adaptador de captura deve satisfazer.
// Segue o mesmo padrão de output.OutputDevice para consistência.
type InputDevice interface {
    Open(ctx context.Context, cfg InputConfig) error
    Start(ctx context.Context) error
    // Read preenche buf com frames PCM float32 LE interleaved.
    // Bloqueia até ter dados ou ctx ser cancelado.
    Read(ctx context.Context, buf []float32) (int, error)
    Stop(ctx context.Context) error
    Close() error
    Info() InputDeviceInfo
}

type InputConfig struct {
    DeviceID     string // "default" ou nome do dispositivo do SO
    SampleRate   int    // 48000
    Channels     int    // 2
    BufferFrames int    // 2048
}

type InputDeviceInfo struct {
    ID         string
    Name       string
    Driver     string // "avfoundation" | "alsa" | "dshow"
    SampleRate int
    Channels   int
}
```

#### `ffmpeg_capture.go` (com arquivos de build tag por plataforma)

```
linein/
  ffmpeg_capture.go         ← lógica comum (spawn, pipe, Read)
  ffmpeg_args_darwin.go     ← func inputArgs() → -f avfoundation -i ":N"
  ffmpeg_args_linux.go      ← func inputArgs() → -f alsa -i hw:N,0
  ffmpeg_args_windows.go    ← func inputArgs() → -f dshow -i audio="NAME"
  device_list_darwin.go     ← lista dispositivos via avfoundation -list_devices
  device_list_linux.go      ← lista via arecord -l ou ALSA
  device_list_windows.go    ← lista via dshow -list_devices
  manager.go
  manager_test.go
```

Comando FFmpeg gerado (macOS, exemplo):
```bash
ffmpeg -hide_banner -loglevel error \
  -f avfoundation -i ":0" \
  -f f32le -acodec pcm_f32le -ac 2 -ar 48000 \
  pipe:1
```

#### `manager.go`

```go
type LineInConfig struct {
    DeviceID   string        // dispositivo de entrada
    Label      string        // rótulo exibido na UI ("A Voz do Brasil")
    DurationMS int64         // 0 = indefinido; >0 = para automaticamente
}

type Status struct {
    Active     bool
    DeviceID   string
    DeviceName string
    Label      string
    StartedAt  time.Time
    DurationMS int64
}

type Manager struct { /* ... */ }

func (m *Manager) Start(cfg LineInConfig) error
func (m *Manager) Stop() error
func (m *Manager) Status() Status
```

Internamente, `Start()` cria um context cancelável e lança uma goroutine:

```go
go func() {
    buf := make([]float32, cfg.BufferFrames * cfg.Channels)
    for {
        n, err := m.input.Read(ctx, buf)
        if err != nil { break }
        m.output.Write(ctx, buf[:n*cfg.Channels])
    }
    // ao sair: publicar evento + transição de estado
}()
```

---

## 6. Contratos de API (REST)

Todos os endpoints seguem o envelope padrão do engine:
```json
{ "ok": true,  "data": { ... } }
{ "ok": false, "error": "codigo_snake", "message": "descrição" }
```

### `POST /v1/linein/start`

**Inicia captura de linha-in.**

Request body:
```json
{
  "device_id": "default",
  "label": "A Voz do Brasil",
  "duration_ms": 3600000,
  "on_silence": "alert"
}
```

| Campo | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `device_id` | string | não | `"default"` ou ID retornado por `/v1/linein/devices`. Padrão: `"default"` |
| `label` | string | não | Rótulo exibido na UI durante a transmissão |
| `duration_ms` | int64 | não | `0` = sem limite; valor positivo encerra automaticamente após N ms |
| `on_silence` | string | não | `"alert"` (padrão) = emite alerta mas mantém ativo; `"stop"` = para o line-in e retoma a programação |

Resposta `200 OK`:
```json
{
  "ok": true,
  "data": {
    "active": true,
    "device_id": "default",
    "device_name": "Built-in Line Input",
    "label": "A Voz do Brasil",
    "started_at": "2026-07-29T19:00:00Z",
    "duration_ms": 3600000
  }
}
```

Erros possíveis:
| Código HTTP | `error` | Situação |
|---|---|---|
| 409 | `linein_already_active` | Já existe uma captura em andamento |
| 503 | `engine_in_panic` | Engine em modo PANIC — linha-in recusado |
| 400 | `invalid_device` | `device_id` não encontrado |
| 500 | `ffmpeg_not_found` | FFmpeg não disponível no PATH |
| 500 | `capture_start_failed` | FFmpeg não conseguiu abrir o dispositivo |

---

### `POST /v1/linein/stop`

**Para a captura ativa.**

Request body (opcional):
```json
{
  "reason": "fim do programa"
}
```

Resposta `200 OK`:
```json
{
  "ok": true,
  "data": {
    "active": false,
    "stopped_at": "2026-07-29T20:00:00Z"
  }
}
```

Erros:
| Código HTTP | `error` | Situação |
|---|---|---|
| 409 | `linein_not_active` | Nenhuma captura em andamento |

---

### `GET /v1/linein/status`

**Retorna o estado atual do line-in.**

Resposta (line-in ativo):
```json
{
  "ok": true,
  "data": {
    "active": true,
    "device_id": "default",
    "device_name": "Built-in Line Input",
    "label": "A Voz do Brasil",
    "started_at": "2026-07-29T19:00:00Z",
    "duration_ms": 3600000,
    "elapsed_ms": 1234567,
    "signal_level_dbfs": -18.4,
    "silence_detected": false
  }
}
```

Resposta (line-in inativo):
```json
{
  "ok": true,
  "data": {
    "active": false
  }
}
```

---

### `GET /v1/linein/devices`

**Lista os dispositivos de entrada de áudio disponíveis no sistema.**

Resposta:
```json
{
  "ok": true,
  "data": [
    { "id": "default",     "name": "Entrada Padrão do Sistema",   "driver": "avfoundation" },
    { "id": ":0",          "name": "Built-in Microphone",          "driver": "avfoundation" },
    { "id": ":1",          "name": "USB Audio CODEC",              "driver": "avfoundation" }
  ]
}
```

---

### `PATCH /v1/linein/config`

**Salva a configuração padrão de line-in (dispositivo de entrada).** Persiste via `prefs`.

Request body:
```json
{
  "default_device_id": ":1"
}
```

Resposta `200 OK`:
```json
{ "ok": true, "data": { "saved": true } }
```

---

## 7. Eventos WebSocket

Novos eventos emitidos no Event Bus e entregues ao cliente via `/v1/events`:

### `linein.started`
```json
{
  "type": "linein.started",
  "ts": "2026-07-29T19:00:00.123Z",
  "payload": {
    "device_id": "default",
    "device_name": "Built-in Line Input",
    "label": "A Voz do Brasil",
    "duration_ms": 3600000
  }
}
```

### `linein.stopped`
```json
{
  "type": "linein.stopped",
  "ts": "2026-07-29T20:00:01.456Z",
  "payload": {
    "reason": "fim do programa",
    "elapsed_ms": 3601333
  }
}
```

### `linein.error`
```json
{
  "type": "linein.error",
  "ts": "2026-07-29T19:05:00.789Z",
  "payload": {
    "error": "silence_detected",
    "message": "Nenhum sinal detectado por 30 segundos",
    "silence_duration_ms": 30000
  }
}
```

### `linein.level`  *(opcional — Fase 6)*
```json
{
  "type": "linein.level",
  "ts": "2026-07-29T19:01:00.000Z",
  "payload": {
    "level_dbfs": -18.4,
    "peak_dbfs": -12.1
  }
}
```
*Emitido a cada ~250ms durante captura ativa. O cliente pode usar para exibir VU meter.*

---

## 8. Novos Comandos (`internal/commands/`)

```go
const (
    CmdLineInStart CommandType = "linein.start"
    CmdLineInStop  CommandType = "linein.stop"
)

type LineInStartPayload struct {
    DeviceID   string `json:"device_id"`
    Label      string `json:"label"`
    DurationMS int64  `json:"duration_ms,omitempty"`
    // OnSilence define o comportamento quando silêncio é detectado.
    // "alert" (padrão): emite linein.error mas mantém a captura ativa.
    // "stop": para o line-in e retoma a programação automática.
    OnSilence  string `json:"on_silence,omitempty"` // "alert" | "stop"
}

type LineInStopPayload struct {
    Reason string `json:"reason,omitempty"`
}
```

---

## 9. Máquina de Estados — Mudanças

### 9.1 Novo estado `LINE_IN`

```
Estados atuais:
  STARTING → IDLE → PLAYING ⇄ PAUSED
                 ↘ ASSIST
                 ↘ PANIC (prioridade máxima)
                 ↘ ERROR

Com a feature:
  PLAYING ──► LINE_IN ──► PLAYING  (se fila tinha itens)
  IDLE    ──► LINE_IN ──► IDLE     (se fila estava vazia)
  PAUSED  ──► LINE_IN ──► PAUSED   (retoma no estado anterior)
  ASSIST  ──► LINE_IN ──► ASSIST

  PANIC ─✗─► LINE_IN   (recusado; PANIC tem prioridade)
  LINE_IN ──► PANIC     (PANIC interrompe o line-in)
```

### 9.2 Snapshot — campos adicionados

```go
type Snapshot struct {
    // ... campos existentes ...

    // LineIn descreve o estado atual da captura de linha (nil se inativa).
    LineIn *LineInStatus `json:"line_in,omitempty"`
}

type LineInStatus struct {
    Active           bool      `json:"active"`
    DeviceID         string    `json:"device_id"`
    DeviceName       string    `json:"device_name"`
    Label            string    `json:"label"`
    StartedAt        time.Time `json:"started_at"`
    DurationMS       int64     `json:"duration_ms"`
    ElapsedMS        int64     `json:"elapsed_ms"`
    SignalLevelDBFS  float64   `json:"signal_level_dbfs"`
    SilenceDetected  bool      `json:"silence_detected"`
}
```

---

## 10. Integração com o Scheduler

### 10.1 Valores disponíveis para `trigger_mode`

O campo `trigger_mode` controla como o agendamento se comporta em relação ao
conteúdo que estiver tocando no momento do disparo:

| Valor | Comportamento | Uso recomendado |
|---|---|---|
| `INTERRUPT` | Corta imediatamente o que está tocando e inicia o line-in | A Voz do Brasil (entrada obrigatória em horário fixo) |
| `AFTER_CURRENT` | Aguarda o item atual terminar naturalmente e então inicia | Programas com flexibilidade de alguns minutos no início |
| `CROSSFADE` | Inicia um crossfade no item atual e entra com o line-in | Transições musicais suaves sem corte brusco |
| `SKIP_IF_BUSY` | Inicia apenas se o engine estiver idle; caso contrário, marca como `MISSED` | Conteúdo opcional que não deve interromper nada |

Para o caso da Voz do Brasil, `INTERRUPT` é o valor correto — a emissora
precisa entrar no ar exatamente às 19h00 independentemente do que estiver
tocando.

---

### 10.2 Campos adicionados a `Entry`

```go
type Entry struct {
    // ... campos existentes ...

    // LineIn é preenchido quando o evento agendado inicia uma captura de linha.
    // Mutuamente exclusivo com Item, Break e LineInStop.
    LineIn *commands.LineInStartPayload `json:"line_in,omitempty"`

    // LineInStop, quando true, encerra uma captura ativa ao disparar.
    // Mutuamente exclusivo com Item, Break e LineIn.
    LineInStop bool `json:"line_in_stop,omitempty"`
}
```

### 10.2 Abordagens de configuração para A Voz do Brasil

O mecanismo principal de encerramento é o campo `duration_ms` dentro do próprio
`line_in`. Com ele, **uma única entrada no scheduler é suficiente** para o caso
mais comum:

#### Abordagem 1 — Entrada única com duração fixa (recomendada)

Via `POST /v1/schedule` (uma entrada):

```json
{
  "name": "Voz do Brasil",
  "enabled": true,
  "cron_expr": "0 19 * * 1-5",
  "trigger_mode": "INTERRUPT",
  "line_in": {
    "device_id": "default",
    "label": "A Voz do Brasil",
    "duration_ms": 3600000,
    "on_silence": "alert"
  }
}
```

O `LineInManager` encerra automaticamente após 60 minutos sem necessidade de
uma segunda entrada. Use esta abordagem quando a duração do programa é fixa e
conhecida.

---

#### Abordagem 2 — Entrada única sem duração + parada manual

Omitir `duration_ms` (ou defini-lo como `0`) quando a duração é variável
(ex.: a Voz do Brasil pode terminar antes das 20h dependendo da sessão do
Congresso). O operador para manualmente via UI ou via `POST /v1/linein/stop`.

```json
{
  "name": "Voz do Brasil",
  "enabled": true,
  "cron_expr": "0 19 * * 1-5",
  "trigger_mode": "INTERRUPT",
  "line_in": {
    "device_id": "default",
    "label": "A Voz do Brasil",
    "on_silence": "alert"
  }
}
```

---

#### Abordagem 3 — Entrada de início + entrada de hard-stop (máxima segurança)

Quando se deseja duração automática E um hard-stop de segurança em horário
fixo, independentemente de o programa ter terminado antes:

```json
[
  {
    "name": "Voz do Brasil — Início",
    "enabled": true,
    "cron_expr": "0 19 * * 1-5",
    "trigger_mode": "INTERRUPT",
    "line_in": {
      "device_id": "default",
      "label": "A Voz do Brasil",
      "duration_ms": 3600000,
      "on_silence": "alert"
    }
  },
  {
    "name": "Voz do Brasil — Hard-stop 20h",
    "enabled": true,
    "cron_expr": "0 20 * * 1-5",
    "line_in_stop": true
  }
]
```

O line-in para pelo que ocorrer primeiro: o `duration_ms` expirar ou a entrada
de `line_in_stop` disparar. Se o `duration_ms` já encerrou antes das 20h, a
entrada de hard-stop é um no-op silencioso.

---

| Abordagem | `duration_ms` | `line_in_stop` | Caso de uso |
|---|---|---|---|
| 1 — Entrada única | definido | não | Duração fixa e conhecida |
| 2 — Sem duração | omitido | não | Duração variável + parada manual |
| 3 — Dupla entrada | definido | sim | Máxima segurança operacional |

---

## 11. Impacto em Cada Módulo

### 11.1 `playout/` — impacto principal

| Área | Mudanças |
|---|---|
| `internal/linein/` | **Pacote novo** (6-8 arquivos) |
| `internal/commands/` | 2 novos tipos de comando + payloads |
| `internal/events/` | 4 novos tipos de evento + payloads |
| `internal/state/` | Novo estado `LINE_IN` + campo `LineIn` no `Snapshot` |
| `internal/dispatcher/` | 2 novos `case` para despachar ao `LineInManager` |
| `internal/scheduler/entry.go` | Campos `LineIn` e `LineInStop` |
| `internal/scheduler/fire.go` | Lógica de despacho para os novos campos |
| `internal/api/handlers/linein.go` | **Arquivo novo** com 5 endpoints |
| `internal/prefs/prefs.go` | Campo `LineInDefaultDeviceID` |
| `cmd/playout-engine/` | Wiring do `LineInManager` na inicialização |

**Não há mudanças no pipeline de áudio existente** — o `LineInManager` escreve
diretamente no `OutputDevice` injetado, sem alterar o `PlaybackManager` atual.

### 11.2 `library/` — impacto: nenhum

O Library Service gerencia metadados de arquivos. Captura de linha-in é
inteiramente responsabilidade do playout engine. Nenhum arquivo do
`library/` precisa ser alterado.

### 11.3 `player/` — impacto na UI

| Área | Mudanças |
|---|---|
| Badge de estado | Exibir "LINE-IN ATIVO" quando `state == "LINE_IN"` |
| Painel de controles | Botão "Entrada de Linha" com sub-ações Iniciar / Parar |
| Modal de configuração | Seletor de dispositivo (populated via `/v1/linein/devices`), campo de label, duração |
| Monitor de nível | VU meter simples para o sinal de entrada (opcional, Fase 6) |
| WebSocket | Reagir a `linein.started`, `linein.stopped`, `linein.error` |
| Fila de reprodução | Exibir item especial "LINE-IN" na posição "tocando agora" durante captura |

---

## 12. Pontos Adicionais (não solicitados, mas relevantes)

### 12.1 Detecção de silêncio (watchdog)

Um problema comum em linha-in é o sinal ausente (rádio desligado, cabo
desconectado, sinal fraco). O sistema deve monitorar o nível de RMS dos
frames capturados e:

- Se nível < -60 dBFS por mais de 30 segundos: emitir `linein.error` com
  `error: "silence_detected"`.
- A UI exibe alerta visual.
- Comportamento controlado pelo campo `on_silence` do payload de `/v1/linein/start`:
  - `"alert"` (padrão): emite `linein.error` e alerta na UI, mas mantém o line-in ativo — comportamento conservador que não interrompe a transmissão por soluços de sinal.
  - `"stop"`: emite `linein.error`, para o line-in imediatamente e retoma a programação automática da fila.

### 12.2 Controle de ganho de entrada

Adicionar um parâmetro `gain_db` (padrão: 0.0) no `LineInStartPayload` que
aplica um multiplicador de ganho nos frames antes de enviá-los ao output.
Útil quando o sinal do rádio chega fraco ou forte demais.

### 12.3 Gravação simultânea (logging de transmissão)

Muitas emissoras precisam gravar tudo que foi ao ar (obrigação legal no
Brasil — Resolução ANATEL). O `LineInManager` pode opcionalmente escrever
os frames capturados em um arquivo WAV/MP3 simultaneamente à transmissão,
via `FileOutput` (já existe no engine).

### 12.4 Ducking da fila durante transição

Em vez de cortar abruptamente ao entrar/sair do line-in, aplicar um
fade-out no item em reprodução (ex.: 3s) antes de iniciar o line-in, e um
fade-in ao retomar. Isso garante transições profissionais.

### 12.5 Suporte a streaming HTTP como fonte virtual

Além do dispositivo de hardware, suportar uma URL de stream Icecast/HLS como
"dispositivo virtual":
```json
{ "device_id": "http://radiogov.ebc.com.br/stream", "label": "Voz do Brasil (stream)" }
```
FFmpeg já suporta isso nativamente. Para emissoras sem rádio AM/FM, é
possível capturar o áudio da Voz do Brasil diretamente do stream oficial
da EBC.

### 12.6 Múltiplos dispositivos de entrada

Estrutura preparada para que, no futuro, seja possível ter dois line-ins
simultâneos (ex.: entrada A para rede nacional, entrada B para rede
regional), com seleção de qual vai para o output.

### 12.7 Histórico de transmissões de linha-in

Registrar em memória (e opcionalmente em arquivo JSON) o histórico de quando
o line-in foi ativado/desativado, por quanto tempo, e se houve alertas de
silêncio. Útil para auditoria e prova de cumprimento da obrigação legal.

---

## 13. Fases de Implementação

### Fase 1 — Infraestrutura de captura (`internal/linein/`)

**Objetivo:** ter captura funcional testável sem integração com o engine.

**Entregáveis:**
- Interface `InputDevice` e tipos (`device.go`)
- `FFmpegCapture` com build tags por plataforma
- Listagem de dispositivos por plataforma (`device_list_*.go`)
- `LineInManager` com goroutine de passagem (`manager.go`)
- Testes com stub de `InputDevice` e stub de `OutputDevice`

**Critério de aceite:** `go test ./internal/linein/...` passa; captura real
funciona em desenvolvimento (verificável via `go run` com linha-in aberta).

---

### Fase 2 — Novos comandos e eventos

**Objetivo:** conectar o `LineInManager` ao Command Bus e ao Event Bus.

**Entregáveis:**
- `CmdLineInStart`, `CmdLineInStop` em `internal/commands/`
- `EvtLineInStarted`, `EvtLineInStopped`, `EvtLineInError` em `internal/events/`
- Cases no Dispatcher para os novos comandos
- Estado `LINE_IN` em `internal/state/`
- Campo `LineIn *LineInStatus` no `Snapshot`
- Testes de transição de estado

---

### Fase 3 — API REST

**Objetivo:** expor controle completo via HTTP.

**Entregáveis:**
- `internal/api/handlers/linein.go` (5 endpoints)
- Wiring no router principal
- Testes com `httptest.NewRecorder`
- Documentação dos contratos (este arquivo, seção 6)

---

### Fase 4 — Integração com o Scheduler

**Objetivo:** permitir que o line-in inicie e pare automaticamente por cron.

**Entregáveis:**
- Campos `LineIn` e `LineInStop` em `Entry`
- Lógica de despacho em `fire.go`
- Testes: stub do CommandBus verifica que `CmdLineInStart` é enviado
- Exemplo de configuração documentado (seção 10.2)

---

### Fase 5 — Configuração persistente via Prefs

**Objetivo:** lembrar o dispositivo padrão entre reinicializações.

**Entregáveis:**
- Campo `LineInDefaultDeviceID` em `internal/prefs/`
- Endpoint `PATCH /v1/linein/config`
- O endpoint `POST /v1/linein/start` usa o device padrão quando `device_id` é omitido

---

### Fase 6 — Monitoração de sinal e VU meter

**Objetivo:** alertar silêncio e expor nível de sinal para a UI.

**Entregáveis:**
- Watchdog de silêncio integrado ao `LineInManager`
- Campo `signal_level_dbfs` e `silence_detected` no `Status`
- Evento `linein.error` com `"silence_detected"`
- Evento `linein.level` emitido a cada 250ms (throttled)
- Configurável: `silence_threshold_db` e `silence_timeout_ms`

---

### Fase 7 — UI (`player/player.html`)

**Objetivo:** operação completa via interface gráfica.

**Entregáveis:**
- Badge "LINE-IN ATIVO" na barra de status (ciano `#00bcd4`, animado)
- Item especial na fila de reprodução representando o line-in ativo
- Botão "Entrada de Linha" no painel de controles
- Modal de configuração:
  - Dropdown de dispositivos (via `/v1/linein/devices`)
  - Campo "Rótulo" (ex.: "A Voz do Brasil")
  - Campo "Duração" (HH:MM ou "Indefinida")
  - Seletor `on_silence`: "Apenas alertar" / "Parar e retomar programação"
- VU meter simples (barra horizontal) reagindo a `linein.level`
- Alerta visual quando `silence_detected == true`
- Consumo dos eventos WebSocket: `linein.started`, `linein.stopped`, `linein.error`
- Seção "Entrada de Linha" na tela de Configurações → Dispositivos

#### 7.0 Tela de Configurações — seção "Dispositivos"

O dispositivo de entrada padrão é configurado na tela de Configurações do
playout, na seção **Dispositivos**, abaixo dos dispositivos de saída existentes.
O valor salvo aqui é usado automaticamente quando um agendamento ou captura
manual não especifica `device_id`.

```
┌─────────────────┬────────────────────────────────────────────────────────────┐
│ Engine          │  DISPOSITIVOS                                              │
│ API             │  ─────────────────────────────────────────────────────── │
│ Dispositivos ◄  │                                                            │
│ Áudio           │  SAÍDA PRINCIPAL                                           │
│ Reprodução      │  ┌──────────────────────────────────────────────────── ▼ ┐ │
│ Saúde           │  │ Alto-falantes (MacBook Pro) (padrão)                  │ │
│ Panic           │  └───────────────────────────────────────────────────────┘ │
│ Logging         │                                                            │
│ Fila            │  PREVIEW (CUE)                                             │
│ Hora Certa      │  ┌──────────────────────────────────────────────────── ▼ ┐ │
│ Scheduler       │  │ — selecione um dispositivo —                          │ │
│ Log Transmissão │  └───────────────────────────────────────────────────────┘ │
│                 │                                                            │
│                 │  HOT KEYS                                                  │
│                 │  ┌──────────────────────────────────────────────────── ▼ ┐ │
│                 │  │ Alto-falantes (MacBook Pro) (padrão)                  │ │
│                 │  └───────────────────────────────────────────────────────┘ │
│                 │                                                            │
│                 │  ───────────────────────────────────────────────────────── │
│                 │                                                            │
│                 │  ENTRADA DE LINHA (LINE-IN)                                │
│                 │                                                            │
│                 │  DISPOSITIVO DE ENTRADA                                    │
│                 │  ┌──────────────────────────────────────────────────── ▼ ┐ │
│                 │  │ — selecione um dispositivo —                          │ │
│                 │  └───────────────────────────────────────────────────────┘ │
│                 │                                                            │
│                 │                            [ Salvar configurações ]       │
└─────────────────┴────────────────────────────────────────────────────────────┘
```

| Campo | Persiste em | Usado quando |
|---|---|---|
| Dispositivo de Entrada | `prefs.LineInDefaultDeviceID` | `device_id` ausente no agendamento ou no `POST /v1/linein/start` |

> A configuração de comportamento em caso de silêncio (`on_silence`) fica na
> seção **Panic** das Configurações, por ser uma condição de falha operacional.

**Hierarquia de precedência para `device_id`:**

```
device_id no agendamento (line_in.device_id)   ← maior prioridade
  ↓ (se ausente)
device_id no POST /v1/linein/start
  ↓ (se ausente)
prefs.LineInDefaultDeviceID                    ← configurado aqui
  ↓ (se ausente)
"default" (alias do dispositivo padrão do SO)  ← menor prioridade
```

#### 7.1 Seção "Agendamentos"

Entradas de line-in aparecem na lista de agendamentos com ícone e cor próprios,
diferenciando-se visualmente dos breaks comerciais:

```
┌──────────────────────────────────────────────────────────────┐
│  AGENDAMENTOS                               [Atualizar]      │
│                                                              │
│  [📻]  A Voz do Brasil                        19:00         │
│        Entrada de linha                                      │
│                                                              │
│  [⏹]  Voz do Brasil — Hard-stop               20:00         │
│        Encerramento de linha-in                              │
└──────────────────────────────────────────────────────────────┘
```

Quando o dispositivo de entrada não está configurado, o card exibe um ícone
de aviso `⚠` ao lado do horário. Um tooltip ao passar o mouse orienta o
operador:

```
┌──────────────────────────────────────────────────────────────┐
│  AGENDAMENTOS                               [Atualizar]      │
│                                                              │
│  [📻]  A Voz do Brasil          ⚠ sem dispositivo    19:00  │
│        Entrada de linha                                      │
│                    ┌─────────────────────────────────────┐   │
│                    │ Nenhum dispositivo de entrada       │   │
│                    │ configurado. Acesse Configurações   │   │
│                    │ → Dispositivos.                     │   │
│                    └─────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

O campo `device_warning: true` no retorno de `GET /v1/schedule` sinaliza ao
player que a entry não tem dispositivo resolvido, permitindo renderizar o `⚠`
sem chamadas adicionais.

| Elemento | Bloco Comercial | Line-In (início) | Line-In (sem dispositivo) | Line-In (hard-stop) |
|---|---|---|---|---|
| Ícone | grid `⊞` | antena `📻` | antena `📻` | stop `⏹` |
| Cor do ícone | laranja | ciano `#00bcd4` | amarelo `#f59e0b` | ciano `#00bcd4` (opaco) |
| Título | nome do break | valor de `label` | valor de `label` | nome da entrada |
| Subtítulo | "Break comercial" | "Entrada de linha" | "Entrada de linha" | "Encerramento de linha-in" |
| Aviso | — | — | `⚠ sem dispositivo` + tooltip | — |
| Horário | `HH:MM` | `HH:MM` | `HH:MM` | `HH:MM` |

A entrada de hard-stop (`line_in_stop: true`) é exibida com o ícone de parada
e subtítulo distinto para deixar claro ao operador que ela encerra uma captura,
não inicia uma.

#### 7.1.1 Alerta de erro pós-disparo (toast)

Quando o agendamento dispara mas o dispositivo não pode ser aberto, o engine
emite `linein.error` via WebSocket. O player exibe um **toast de notificação**
no canto superior direito, com duração de 10 segundos e opção de fechar
manualmente:

```
┌─────────────────────────────────────────────────┐
│ ⚠  Entrada de Linha — Erro                  ✕  │
│    Nenhum dispositivo de entrada configurado.   │
│    O agendamento "Voz do Brasil" foi ignorado.  │
└─────────────────────────────────────────────────┘
```

O toast é exibido para qualquer valor de `linein.error`, não apenas para
`no_device_configured` — cobre também `silence_detected` e falhas de FFmpeg.
A mensagem exibida vem do campo `message` do payload do evento.

#### 7.2 Fila de Reprodução — line-in ativo

Quando o line-in está ativo, ele ocupa a posição "tocando agora" na fila de
reprodução, seguindo o mesmo layout dos itens normais mas com identidade visual
ciana e informações específicas de captura ao vivo:

```
┌─────────────────────────────────────────────────────────────┐
│  FILA DE REPRODUÇÃO                                         │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Buscar na fila...                              ↺    │    │
│  └─────────────────────────────────────────────────────┘    │
│  1 ITEM  |  ⏱ 00:36:13                          ⊘   🕐     │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │▌ 📻          │ A Voz do Brasil                      │    │
│  │▌  19:00:00   │ Entrada de linha                     │    │
│  │▌  00:23:47   │                    [AO VIVO ●]       │    │
│  └─────────────────────────────────────────────────────┘    │
│  ────────────────── FILA SUSPENSA ──────────────────────    │
│  ┌─────────────────────────────────────────────────────┐    │
│ ::📻  22:29:31   │ Tu És Adorado Aqui (Ao Vivo)        │    │
│       6:50       │ Davi Fernandes, Cultura do Cé...    │    │
│                  │                          [Música]   │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

| Elemento | Item normal (tocando) | Line-In (ativo) |
|---|---|---|
| Gradiente | vermelho | ciano `#00bcd4` |
| Barra lateral | vermelha | ciana |
| Ícone | VU meter animado | antena `📻` |
| Linha 1 — esquerda | horário estimado de fim | horário de início (`19:00:00`) |
| Linha 2 — esquerda | duração total | tempo decorrido (`00:23:47`) |
| Título | título da faixa | valor de `label` |
| Subtítulo | artista | "Entrada de linha" |
| Badge | tipo (ex.: "Música") | `AO VIVO ●` pulsante em ciano |
| Separador abaixo | "A SEGUIR" | "FILA SUSPENSA" |

O separador muda de **"A SEGUIR"** para **"FILA SUSPENSA"** para comunicar ao
operador que os itens abaixo não serão tocados até o line-in encerrar. Ao
encerrar, o separador volta para "A SEGUIR" e a reprodução retoma normalmente.

#### 7.3 Widget "Próximo Agendamento"

Quando o próximo agendamento for um line-in, o widget exibe:

```
┌──────────────────────────────────────────────────────────────┐
│  [📻]  A Voz do Brasil 19:00                   47:21        │
│        Entrada de linha                                      │
└──────────────────────────────────────────────────────────────┘
```

| Elemento | Bloco Comercial | Line-In |
|---|---|---|
| Ícone | grid `⊞` (fundo laranja escuro) | antena `📻` (fundo ciano escuro) |
| Título | "Nome do Break HH:MM" | "`label` HH:MM" |
| Subtítulo | "Break agendado" | "Entrada de linha" |
| Countdown | `MM:SS` em laranja | `MM:SS` em ciano `#00bcd4` |

A cor ciano diferencia o line-in do break (laranja) e do item tocando (vermelho),
criando uma linguagem visual consistente para "fonte externa".

---

### Fase 8 — Gravação simultânea (opcional)

**Objetivo:** gravar o áudio capturado em arquivo para fins de auditoria e
cumprimento de obrigações legais (Resolução ANATEL nº 344/2004).

#### Mecanismo

A goroutine do `LineInManager` já percorre o loop:

```
InputDevice.Read() → OutputDevice.Write()   ← transmissão ao vivo
```

Com gravação ativada, o mesmo buffer de frames é escrito em um segundo destino
em paralelo, sem custo de processamento adicional:

```
InputDevice.Read() → OutputDevice.Write()   ← transmissão ao vivo
                   → FileOutput.Write()      ← gravação em disco
```

O `FileOutput` já existe no engine (`internal/audio/output/file.go`). Não há
nova lógica de áudio — apenas uma segunda chamada de escrita no mesmo loop.

#### Ciclo de vida

1. `POST /v1/linein/start` com `"record": true`
2. `LineInManager.Start()` abre o `FileOutput` em paralelo com o `OutputDevice`
3. Goroutine escreve os frames PCM nos dois destinos a cada ciclo
4. `LineInManager.Stop()` fecha o `FileOutput` e finaliza o arquivo em disco
5. Arquivo disponível via `GET /v1/linein/recordings`

#### Nomenclatura automática

Quando `record_path` é omitido, o nome é gerado a partir do label e da data/hora de início:

```
linein_2026-07-29_19h00_voz-do-brasil.wav
```

#### Considerações de tamanho

| Formato | Tamanho/hora | Qualidade |
|---|---|---|
| PCM float32 (padrão interno) | ~660 MB | máxima |
| WAV 16-bit (recomendado) | ~330 MB | broadcast |
| MP3 128 kbps (conversão pós-stop) | ~56 MB | arquivo |

A conversão para MP3 é feita via FFmpeg ao fechar o arquivo (`LineInManager.Stop()`),
substituindo o WAV temporário. Configurável via campo `record_format` no payload.

#### Onde configurar a gravação

Os campos de gravação fazem parte do mesmo payload `LineInStartPayload` — o que
muda é apenas **quem dispara** a captura: o scheduler ou o operador via UI.

**Caso 1 — Evento recorrente e previsível (via scheduler):**

A configuração fica centralizada na entrada do scheduler. Toda execução do cron
grava automaticamente, sem intervenção humana:

```json
{
  "name": "Voz do Brasil",
  "cron_expr": "0 19 * * 1-5",
  "line_in": {
    "device_id": "default",
    "label": "A Voz do Brasil",
    "duration_ms": 3600000,
    "record": true,
    "record_format": "mp3"
  }
}
```

**Caso 2 — Evento ad hoc iniciado manualmente (via UI / `POST /v1/linein/start`):**

Quando a captura não é previsível (cobertura inesperada, link externo de última
hora, teste de sinal), o operador abre o modal no player, marca "Gravar" e inicia.
O mesmo campo `record` é enviado no body da requisição:

```json
{
  "device_id": "default",
  "label": "Link externo — Cobertura especial",
  "record": true,
  "record_format": "wav"
}
```

| Situação | Mecanismo | Intervenção humana |
|---|---|---|
| Evento recorrente (Voz do Brasil) | Campo `record` no `line_in` do scheduler | Nenhuma |
| Evento ad hoc (breaking news, teste) | Campo `record` no body do `POST /v1/linein/start` | Operador inicia manualmente |

#### Campos adicionados ao `LineInStartPayload`

```go
type LineInStartPayload struct {
    // ... campos existentes ...

    // Record ativa a gravação simultânea em arquivo.
    Record       bool   `json:"record,omitempty"`
    // RecordPath define o caminho do arquivo de saída.
    // Se vazio, é gerado automaticamente com base em label e data/hora.
    RecordPath   string `json:"record_path,omitempty"`
    // RecordFormat define o formato do arquivo gravado.
    // "wav" (padrão) ou "mp3" (requer conversão pós-stop via FFmpeg).
    RecordFormat string `json:"record_format,omitempty"` // "wav" | "mp3"
}
```

#### Endpoint `GET /v1/linein/recordings`

Lista todas as gravações anteriores, independentemente de terem sido iniciadas
pelo scheduler ou manualmente pelo operador:

```json
{
  "ok": true,
  "data": [
    {
      "label": "A Voz do Brasil",
      "started_at": "2026-07-29T19:00:00Z",
      "stopped_at": "2026-07-29T20:00:01Z",
      "duration_ms": 3600123,
      "path": "/recordings/linein_2026-07-29_19h00_voz-do-brasil.mp3",
      "format": "mp3",
      "size_bytes": 56700000,
      "triggered_by": "scheduler"
    },
    {
      "label": "Link externo — Cobertura especial",
      "started_at": "2026-07-28T14:30:00Z",
      "stopped_at": "2026-07-28T15:10:00Z",
      "duration_ms": 2400000,
      "path": "/recordings/linein_2026-07-28_14h30_link-externo.wav",
      "format": "wav",
      "size_bytes": 792000000,
      "triggered_by": "manual"
    }
  ]
}
```

O campo `triggered_by` (`"scheduler"` ou `"manual"`) permite ao operador
distinguir gravações automáticas de capturas ad hoc na listagem.

**Entregáveis:**
- Campos `record`, `record_path` e `record_format` no `LineInStartPayload`
- Abertura do `FileOutput` em paralelo no `LineInManager.Start()`
- Escrita dupla (output + file) no loop de captura
- Conversão pós-stop para MP3 quando `record_format == "mp3"` (via FFmpeg)
- Campo `triggered_by` preenchido pelo Dispatcher (scheduler vs. handler HTTP)
- Endpoint `GET /v1/linein/recordings` com listagem unificada
- Testes: verificar que frames chegam ao `FileOutput`; verificar nome automático gerado; verificar `triggered_by` correto em cada origem

---

## 14. Testes Planejados

| Pacote | Cenário | Abordagem |
|---|---|---|
| `linein` | Frames chegam ao output | Stub InputDevice → Stub OutputDevice; verificar bytes escritos |
| `linein` | Silêncio detectado após threshold | Stub retorna frames zero; verificar evento emitido |
| `linein` | Stop durante captura | Cancelar ctx; verificar goroutine termina sem vazamento |
| `linein` | FFmpeg não disponível | Mock exec.LookPath; verificar erro descritivo |
| `state` | Transição PLAYING→LINE_IN→PLAYING | Tabela de estados; verificar campo `LineIn` no Snapshot |
| `state` | PANIC rejeita LINE_IN | Dispatcher verifica estado antes de despachar |
| `commands` | Serialize/deserialize payload | Round-trip JSON dos novos payloads |
| `events` | Publish/subscribe novos eventos | Bus in-memory; subscriber recebe evento correto |
| `handlers` | POST /v1/linein/start — sucesso | httptest; body válido; status 200 |
| `handlers` | POST /v1/linein/start — já ativo | httptest; espera 409 |
| `handlers` | POST /v1/linein/start — PANIC | httptest; espera 503 |
| `handlers` | GET /v1/linein/devices | httptest; resposta com lista não vazia |
| `scheduler` | Entry com LineIn dispara CmdLineInStart | Stub CommandBus; verifica tipo de cmd |
| `scheduler` | Entry com LineInStop dispara CmdLineInStop | Idem |

---

## 15. Riscos e Mitigações

| Risco | Probabilidade | Mitigação |
|---|---|---|
| Nome do dispositivo difere entre SO e hardware | Alta | Endpoint `/devices` + alias `"default"` + documentação |
| Latência inicial do FFmpeg (200–500ms) | Média | Aceitável para programas de 1h; pré-aquecimento não necessário |
| Sinal ausente (rádio desligado) | Alta | Watchdog de silêncio + alerta na UI |
| FFmpeg não instalado no servidor | Média | Verificação no startup; erro descritivo com instrução de instalação |
| Buffer underrun no output durante captura | Baixa | Buffer de 2048 frames (~42ms); goroutine dedicada sem contenção |
| Conflito de dispositivo (captura e output na mesma placa) | Baixa | Documentar que input e output devem ser em dispositivos separados ou multi-channel |
| Programa termina antes do horário agendado | Média | Watchdog de silêncio + parada manual via UI + duração automática como fallback |

---

## 16. Estrutura de Arquivos (resultado final esperado)

```
playout/
  internal/
    linein/
      device.go                  ← interfaces e tipos
      ffmpeg_capture.go          ← FFmpegCapture (lógica comum)
      ffmpeg_args_darwin.go      ← build tag: avfoundation
      ffmpeg_args_linux.go       ← build tag: alsa
      ffmpeg_args_windows.go     ← build tag: dshow
      device_list_darwin.go      ← listar dispositivos macOS
      device_list_linux.go       ← listar dispositivos Linux
      device_list_windows.go     ← listar dispositivos Windows
      manager.go                 ← LineInManager
      manager_test.go
    commands/
      commands.go                ← + CmdLineInStart, CmdLineInStop
    events/
      events.go                  ← + EvtLineInStarted/Stopped/Error/Level
    state/
      manager.go                 ← + StateLineIn, LineInStatus no Snapshot
    api/
      handlers/
        linein.go                ← novo: 5 endpoints
    scheduler/
      entry.go                   ← + campos LineIn, LineInStop
      fire.go                    ← + despacho dos novos casos
    prefs/
      prefs.go                   ← + LineInDefaultDeviceID

player/
  player.html                    ← + badge, botão, modal, VU meter
```

---

## Fontes

- [Configurando o Playlist Digital para exibir a Voz do Brasil automaticamente](https://playlistsolutions.com/configurando-o-playlist-digital-para-exibir-a-voz-do-brasil-automaticamente-no-horario/)
- [Como Transmitir A Voz do Brasil no Playlist Digital](https://playlistsolutions.com/transmitir-voz-do-brasil-playlist-digital/)
- [A Voz do Brasil na Programação — ZaraRádio (Taaqui)](https://ajuda.taaqui.org/artigo/a-voz-do-brasil-na-programacao-da-sua-radio-utilizando-o-zararadio)
- [Automação A Voz do Brasil — Mídiaplay](https://midiaplayprodutora.com.br/automacao-a-voz-do-brasil/)
- [RadioBOSS — Line Inputs Manual](https://manual.djsoft.net/radioboss/en/microphone___line-in.htm)
- [RadioBOSS — Working with Linear Input](https://manual.djsoft.net/radioboss/en/working_with_linear_input.htm)
- [mAirList — Audio Routing](https://mairlist.docs.mairlist.com/configuration/audiorouting/)
- [mAirList — Encoder Audio Routing](https://mairlist.docs.mairlist.com/features/encoder/audiorouting/)
- [Rivendell Radio Automation — Operations Guide](https://opsguide.rivendellaudio.org/)
- [Rivendell GitHub](https://github.com/ElvishArtisan/rivendell)
- [Telos Alliance — Livewire+ AES67](https://www.telosalliance.com/livewire-aes67-aoip-networking)
- [OmniPlayer — M&I Broadcast Services](https://www.mibroadcastservices.nl/products/omniplayer/)
- [Jutel — How does radio automation handle live broadcasts](https://jutel.fi/how-does-radio-automation-software-handle-live-broadcasts/)
