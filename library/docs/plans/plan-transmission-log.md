# Plano — Log de Transmissão

## Visão geral

Implementar o **log de transmissão** do RadioFlow: registro automático e persistente de
tudo que vai ao ar — cada faixa tocada, horário de início, horário de término, duração
real reproduzida, tipo de áudio e contexto (bloco comercial, cart da botoneira ou fila
principal).

O log é a base legal e operacional da emissora:
- **Declaração ao ECAD** — obrigação legal de reportar mensalmente todas as músicas
  executadas com título, artista e horário.
- **Prova de veiculação** — comprovante para anunciantes de que o comercial foi ao ar
  no horário contratado.
- **Auditoria interna** — verificação de aderência à grade programada.

**Branch:** `feature/transmission-log`
**Base:** `main`

---

## Contexto atual

### Playout Engine
O Engine já publica eventos no WebSocket (`ws://host:8080/v1/events`) a cada transição
de faixa. Os eventos relevantes e seus payloads são:

| Evento | Payload relevante |
|--------|-------------------|
| `NowPlayingChanged` | `queue_item_id`, `asset_id`, `path`, `title`, `artist`, `type`, `duration_ms`, `break_id`, `break_title`, `break_position`, `break_total`, `break_role` |
| `ItemFinished` | `queue_item_id`, `asset_id`, `result`, `duration_played_ms` |
| `CartStarted` | `cart_id`, `path`, `title`, `artist`, `duration_ms` |
| `CartStopped` | `cart_id`, `reason` |
| `SpotStarted` | `break_id`, `break_title`, `queue_item_id`, `title`, `break_seq`, `break_total`, `break_role` |
| `SpotEnded` | `break_id`, `queue_item_id`, `break_seq` |

O Engine **não persiste nada** — apenas emite eventos. A persistência é responsabilidade
do Library Service, conforme a regra de arquitetura do projeto.

### Library Service
Já possui:
- Padrão de migração em `internal/store/migrations/*.sql` (próxima: `005`)
- Padrão de store com `database/sql` puro
- Padrão de handler retornando `http.HandlerFunc`
- Config estruturada em `internal/config/config.go`
- `main.go` com injeção de dependências e goroutines com shutdown via context

### Config
A URL do Playout Engine **não existe** ainda em `Config`. Será adicionada.

---

## Estratégia de coleta

O Library Service iniciará um **log consumer** — goroutine com cliente WebSocket que
subscreve ao stream de eventos do Engine. A correlação entre início e fim de cada faixa
é feita por `queue_item_id` (fila principal) ou `cart_id` (botoneira):

```
NowPlayingChanged  →  INSERT entry (status=PLAYING, started_at=now)
ItemFinished       →  UPDATE entry WHERE queue_item_id (status=FINISHED/SKIPPED/FAILED)
CartStarted        →  INSERT entry (type=CART, status=PLAYING)
CartStopped        →  UPDATE entry WHERE cart_id (status=FINISHED/STOPPED)
```

Entradas sem `ItemFinished` correspondente (Engine reiniciado, crash) ficam com
`status=INTERRUPTED` após restart do consumer, detectado por `finished_at IS NULL`
com `started_at` mais antiga que o tempo de reconexão.

---

## Modelo de dados

### Migration 005 — transmission_log

Arquivo: `internal/store/migrations/005_transmission_log.sql`

```sql
CREATE TABLE IF NOT EXISTS transmission_log (
    id                 TEXT     PRIMARY KEY,
    queue_item_id      TEXT     NOT NULL DEFAULT '',
    asset_id           TEXT     NOT NULL DEFAULT '',
    path               TEXT     NOT NULL DEFAULT '',
    title              TEXT     NOT NULL DEFAULT '',
    artist             TEXT     NOT NULL DEFAULT '',
    type               TEXT     NOT NULL DEFAULT '',   -- MUSIC|JINGLE|VINHETA|SPOT|CART|HORA_CERTA
    duration_ms        INTEGER  NOT NULL DEFAULT 0,
    duration_played_ms INTEGER  NOT NULL DEFAULT 0,
    result             TEXT     NOT NULL DEFAULT '',   -- finished|skipped|failed|interrupted
    status             TEXT     NOT NULL DEFAULT 'PLAYING', -- PLAYING|FINISHED|SKIPPED|FAILED|INTERRUPTED
    started_at         DATETIME NOT NULL,
    finished_at        DATETIME,
    break_id           TEXT     NOT NULL DEFAULT '',
    break_title        TEXT     NOT NULL DEFAULT '',
    break_role         TEXT     NOT NULL DEFAULT '',   -- open|spot|close (vazio se não é break)
    break_position     INTEGER  NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_transmission_log_started_at  ON transmission_log(started_at);
CREATE INDEX IF NOT EXISTS idx_transmission_log_type        ON transmission_log(type);
CREATE INDEX IF NOT EXISTS idx_transmission_log_status      ON transmission_log(status);
CREATE INDEX IF NOT EXISTS idx_transmission_log_asset_id    ON transmission_log(asset_id);
```

---

## Estrutura de pacotes novos

```
library/
  internal/
    config/
      config.go                      ← adicionar PlayoutConfig { URL string }
    store/
      migrations/
        005_transmission_log.sql     ← nova migração
      transmission_log_store.go      ← TransmissionLogStore
    logconsumer/
      consumer.go                    ← goroutine WebSocket client
      consumer_test.go
    api/
      handlers/
        transmission_log.go          ← handlers GET /v1/transmission-log
```

---

## Config — nova seção

Adicionar em `internal/config/config.go`:

```go
// PlayoutConfig holds connection settings for the Playout Engine.
type PlayoutConfig struct {
    URL            string        `yaml:"url"`              // ws://host:8080
    ReconnectDelay time.Duration `yaml:"reconnect_delay"`  // default: 5s
}
```

Integrar em `Config`:

```go
type Config struct {
    Service ServiceConfig `yaml:"service"`
    API     APIConfig     `yaml:"api"`
    DB      DBConfig      `yaml:"database"`
    Scanner ScannerConfig `yaml:"scanner"`
    Logging LoggingConfig `yaml:"logging"`
    Playout PlayoutConfig `yaml:"playout"`       // ← novo
}
```

Default:
```go
Playout: PlayoutConfig{
    URL:            "ws://127.0.0.1:8080",
    ReconnectDelay: 5 * time.Second,
},
```

---

## Store — TransmissionLogStore

Arquivo: `internal/store/transmission_log_store.go`

### Tipos

```go
type TransmissionLogEntry struct {
    ID               string
    QueueItemID      string
    AssetID          string
    Path             string
    Title            string
    Artist           string
    Type             string
    DurationMS       int64
    DurationPlayedMS int64
    Result           string
    Status           string
    StartedAt        time.Time
    FinishedAt       *time.Time
    BreakID          string
    BreakTitle       string
    BreakRole        string
    BreakPosition    int
}

type TransmissionLogQuery struct {
    From   time.Time
    To     time.Time
    Type   string
    Status string
    Search string // busca por title ou artist
    Limit  int
    Offset int
}
```

### Métodos

```go
type TransmissionLogStore struct { db *sql.DB }

func NewTransmissionLogStore(db *sql.DB) *TransmissionLogStore

// OpenEntry insere entrada com status=PLAYING.
func (s *TransmissionLogStore) OpenEntry(ctx context.Context, e TransmissionLogEntry) error

// CloseEntry finaliza uma entrada em aberto pelo queue_item_id.
func (s *TransmissionLogStore) CloseEntry(ctx context.Context, queueItemID, result string, durationPlayedMS int64, finishedAt time.Time) error

// CloseCartEntry finaliza uma entrada de cart pelo asset_id (cart_id).
func (s *TransmissionLogStore) CloseCartEntry(ctx context.Context, cartID, result string, finishedAt time.Time) error

// MarkInterrupted marca como INTERRUPTED entradas com status=PLAYING
// anteriores a beforeTime (usado no reconnect do consumer).
func (s *TransmissionLogStore) MarkInterrupted(ctx context.Context, beforeTime time.Time) error

// List retorna entradas paginadas com filtros.
func (s *TransmissionLogStore) List(ctx context.Context, q TransmissionLogQuery) ([]TransmissionLogEntry, int, error)

// ExportCSV escreve as entradas do período em formato CSV no io.Writer.
func (s *TransmissionLogStore) ExportCSV(ctx context.Context, from, to time.Time, w io.Writer) error
```

---

## Log Consumer

Arquivo: `internal/logconsumer/consumer.go`

### Responsabilidades

- Conectar via WebSocket à URL do Playout Engine (`/v1/events`)
- Reconectar automaticamente com backoff em caso de desconexão
- Receber eventos JSON e despachar apenas os relevantes para o store
- Ao reconectar: chamar `MarkInterrupted` com `time.Now()` para fechar entradas orfãs
- Encerrar limpo ao cancelar o context

### Interface pública

```go
package logconsumer

type Consumer struct {
    cfg   config.PlayoutConfig
    store TransmissionStore
    log   *slog.Logger
}

// TransmissionStore é a interface mínima necessária (evita import do store concreto).
type TransmissionStore interface {
    OpenEntry(ctx context.Context, e store.TransmissionLogEntry) error
    CloseEntry(ctx context.Context, queueItemID, result string, durationPlayedMS int64, finishedAt time.Time) error
    CloseCartEntry(ctx context.Context, cartID, result string, finishedAt time.Time) error
    MarkInterrupted(ctx context.Context, beforeTime time.Time) error
}

func New(cfg config.PlayoutConfig, store TransmissionStore, log *slog.Logger) *Consumer

// Run bloqueia até ctx ser cancelado. Reconecta com backoff se a conexão cair.
func (c *Consumer) Run(ctx context.Context) error
```

### Lógica de processamento de eventos

```go
switch event.Type {
case "NowPlayingChanged":
    // ignora se title e path estão vazios (engine idle)
    // INSERT entry com status=PLAYING
case "ItemFinished":
    // UPDATE WHERE queue_item_id → status=FINISHED/SKIPPED/FAILED
case "CartStarted":
    // INSERT entry com type=CART, status=PLAYING, queue_item_id=cart_id
case "CartStopped":
    // UPDATE WHERE asset_id=cart_id → status=FINISHED/STOPPED
}
```

### Reconexão com backoff

```go
delay := cfg.ReconnectDelay
for {
    err := c.connect(ctx)
    if ctx.Err() != nil { return nil }
    c.log.Warn("WebSocket disconnected, reconnecting", "delay", delay, "error", err)
    c.store.MarkInterrupted(ctx, time.Now())
    select {
    case <-ctx.Done(): return nil
    case <-time.After(delay):
    }
    delay = min(delay*2, 60*time.Second) // backoff cap: 60s
}
```

---

## API — novos endpoints

### Listar log de transmissão

```
GET /v1/transmission-log
```

**Query params:**

| Parâmetro | Tipo | Descrição |
|---|---|---|
| `from` | string | ISO 8601 ou `YYYY-MM-DD`. Default: início do dia atual |
| `to` | string | ISO 8601 ou `YYYY-MM-DD`. Default: fim do dia atual |
| `type` | string | Filtro por tipo: `MUSIC`, `SPOT`, `CART`, etc. (opcional) |
| `status` | string | `PLAYING`, `FINISHED`, `SKIPPED`, `FAILED`, `INTERRUPTED` (opcional) |
| `q` | string | Busca por título ou artista (opcional) |
| `limit` | int | Máximo de resultados. Default: 100, máx: 500 |
| `offset` | int | Paginação. Default: 0 |

**Resposta:**
```json
{
  "ok": true,
  "data": {
    "entries": [
      {
        "id":                 "01JK...",
        "queue_item_id":      "qi-uuid",
        "asset_id":           "track-uuid",
        "path":               "/audio/musicas/Elis Regina - Como Nossos Pais.mp3",
        "title":              "Como Nossos Pais",
        "artist":             "Elis Regina",
        "type":               "MUSIC",
        "duration_ms":        243000,
        "duration_played_ms": 238000,
        "result":             "finished",
        "status":             "FINISHED",
        "started_at":         "2026-07-20T08:00:00Z",
        "finished_at":        "2026-07-20T08:03:58Z",
        "break_id":           "",
        "break_title":        "",
        "break_role":         "",
        "break_position":     0
      }
    ],
    "total":  142,
    "limit":  100,
    "offset": 0
  }
}
```

---

### Exportar log em CSV

```
GET /v1/transmission-log/export?from=2026-07-20&to=2026-07-20&format=csv
```

Retorna resposta com `Content-Type: text/csv` e header
`Content-Disposition: attachment; filename="transmission_log_2026-07-20.csv"`.

**Colunas do CSV:**

```
id,started_at,finished_at,duration_played_ms,title,artist,type,result,status,break_id,break_role,path
```

---

### Resumo do dia (para exibição no player)

```
GET /v1/transmission-log/summary?date=2026-07-20
```

**Resposta:**
```json
{
  "ok": true,
  "data": {
    "date":        "2026-07-20",
    "total":       148,
    "by_type": {
      "MUSIC":     92,
      "VINHETA":   18,
      "JINGLE":    12,
      "SPOT":      20,
      "CART":       6
    },
    "total_played_ms": 18720000
  }
}
```

---

## Registro de rotas (`server.go`)

```go
// Transmission log
mux.HandleFunc("GET /v1/transmission-log",         handlers.ListTransmissionLog(s.tls))
mux.HandleFunc("GET /v1/transmission-log/export",  handlers.ExportTransmissionLog(s.tls))
mux.HandleFunc("GET /v1/transmission-log/summary", handlers.GetTransmissionLogSummary(s.tls))
```

---

## Injeção em `main.go`

```go
// Transmission log store
transmissionStore := store.NewTransmissionLogStore(db)

// Log consumer (inicia em goroutine com shutdown via ctx)
if cfg.Playout.URL != "" {
    consumer := logconsumer.New(cfg.Playout, transmissionStore, logging.With(log, "logconsumer"))
    go func() {
        if err := consumer.Run(ctx); err != nil {
            slog.Error("log consumer stopped with error", "error", err)
        }
    }()
    log.Info("transmission log consumer started", "playout_url", cfg.Playout.URL)
}

// Registrar transmissionStore no servidor API
srv := api.New(cfg.API, trackStore, playlistStore, breakStore, hotkeyStore, idxSvc,
    categoryStore, clockStore, separationStore, rotationLogStore, transmissionStore, gen,
    logging.With(log, "api"))
```

---

## Player UI — painel de Histórico

Novo painel acessível via aba **"Histórico"** no drawer lateral (após "Rotação").

### Layout

```
[ Playlists ] [ Breaks ] [ Botoneira ] [ Rotação ] [ Histórico ]
                                                         ↑ nova aba
```

### Sub-componentes

**Barra de filtros:**
```
[Data: 20/07/2026] [Tipo: Todos ▾] [Q: buscar título ou artista…] [Buscar]
```

**Tabela de entradas:**
```
INÍCIO    TÍTULO                    ARTISTA          TIPO     DURAÇÃO   STATUS
08:00:00  Como Nossos Pais          Elis Regina      MUSIC    3:58      ✅
08:04:02  Vinheta Manhã 01          —                VINHETA  0:28      ✅
08:04:30  Bloco Comercial 1         —                SPOT     0:30      ✅
...
```

- Ícone de tipo (mesmo padrão da fila de reprodução)
- Status colorido: verde (FINISHED), amarelo (SKIPPED), vermelho (FAILED/INTERRUPTED), azul piscante (PLAYING)
- Linha atual (PLAYING) destacada com borda ciano

**Rodapé:**
```
142 entradas · Tempo total: 5h 12m 00s       [ ↓ Exportar CSV ]
```

**Botão "Exportar CSV":** abre `GET /v1/transmission-log/export` com os filtros ativos.

### Atualização automática

O painel escuta o WebSocket do Playout Engine. Ao receber `NowPlayingChanged`, se o
filtro de data for o dia atual, re-busca `GET /v1/transmission-log` para atualizar
a lista em tempo real — sem polling periódico.

---

## Fases de implementação

### Fase 1 — Migração e Store

1. Criar `internal/store/migrations/005_transmission_log.sql`
2. Registrar migration 005 em `db.go`
3. Implementar `TransmissionLogStore` com todos os métodos
4. Testes com banco `:memory:` — cenários: open/close, MarkInterrupted, List com filtros, ExportCSV

### Fase 2 — Log Consumer

1. Adicionar `PlayoutConfig` em `config.go` com defaults
2. Implementar `internal/logconsumer/consumer.go`
   - WebSocket client usando `golang.org/x/net/websocket` ou `nhooyr.io/websocket`
   - Parsing do envelope `{"type": "...", "payload": {...}}`
   - Handlers para os 4 eventos relevantes
   - Backoff de reconexão
   - `MarkInterrupted` no reconnect
3. Testes do consumer com mock do store e canal de eventos simulado

### Fase 3 — API e Rotas

1. Implementar `handlers/transmission_log.go` (3 handlers)
2. Definir interface `TransmissionLogStore` no pacote `handlers`
3. Registrar rotas em `server.go`
4. Injetar `transmissionStore` e consumer em `main.go`
5. Testes com `httptest.NewRecorder`

### Fase 4 — Player UI

1. Adicionar aba "Histórico" no drawer (`player/player.html`)
2. Implementar barra de filtros (data, tipo, busca)
3. Implementar tabela de entradas com ícones de tipo e status colorido
4. Implementar rodapé com totais e botão de exportação CSV
5. Implementar atualização automática via evento WebSocket

---

## Pontos de atenção

### `NowPlayingChanged` vs `ItemStarted`

`ItemStartedPayload` contém apenas `queue_item_id` e `asset_id` — sem título, artista
ou tipo. `NowPlayingChangedPayload` contém todos os metadados. O consumer deve usar
`NowPlayingChanged` para abrir entradas, **não** `ItemStarted`.

### Entradas duplicadas no reconnect

Se o consumer reconectar durante reprodução de uma faixa, receberá `NowPlayingChanged`
novamente. Usar `INSERT OR IGNORE` com `queue_item_id` como chave de idempotência para
evitar duplicatas.

### Carts da botoneira

O `cart_id` no Engine é gerado internamente e não coincide com o `asset_id` da track.
O consumer usa o `cart_id` como `queue_item_id` para abrir e fechar a entrada de cart.

### Consumer opcional

Se `cfg.Playout.URL` estiver vazio, o consumer não é iniciado. O log permanece vazio
mas o serviço funciona normalmente. Isso permite rodar o Library Service isolado para
testes sem o Engine disponível.

### Biblioteca WebSocket

Verificar qual biblioteca WebSocket já está em uso no projeto (playout usa gorilla ou
nhooyr). Preferir reutilizar a mesma para consistência do `go.work`.

---

## Definição de pronto

- `go test ./...` passa sem erros
- `go vet ./...` sem avisos
- `go test -race ./...` sem data races
- Consumer reconecta corretamente e marca entradas como INTERRUPTED
- `GET /v1/transmission-log` filtra corretamente por data, tipo e busca
- `GET /v1/transmission-log/export` gera CSV com todas as colunas
- Player UI exibe o histórico do dia atual ao abrir a aba
- Player UI exporta CSV com os filtros ativos
- Entradas de cart aparecem com type=CART na tabela
- Entradas de break aparecem com break_role e break_title preenchidos
