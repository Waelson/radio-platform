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

## Análise comparativa de abordagens arquiteturais

Duas abordagens foram consideradas para a coleta do log de transmissão.

---

### Abordagem A — WebSocket Consumer no Library Service (proposta original)

O Library Service abre uma conexão WebSocket com o Engine e processa os eventos
`NowPlayingChanged`, `ItemFinished`, `CartStarted` e `CartStopped` em tempo real,
persistindo diretamente no SQLite.

```
[Playout Engine]
  ↓ WebSocket (eventos)
[Library Service — logconsumer goroutine]
  ↓ INSERT/UPDATE
[SQLite — transmission_log]
  ↓ API REST
[Player UI]
```

#### Prós

| # | Vantagem |
|---|----------|
| 1 | **Nenhuma mudança no Playout Engine** — os eventos já são emitidos. Nenhum código novo no Engine. |
| 2 | **Log em tempo real** — status `PLAYING` disponível imediatamente. UI pode exibir "o que está tocando agora" diretamente do log. |
| 3 | **Um único componente novo** — apenas o consumer goroutine no Library Service. |
| 4 | **Sem gestão de arquivos** — sem locking, sem exclusão, sem risco de arquivo corrompido. |
| 5 | **Queries imediatas** — sem etapa de importação. O dado chega e já está consultável. |
| 6 | **Coerente com a arquitetura** — o Engine permanece sem persistência, conforme `CLAUDE.md`. |

#### Contras

| # | Desvantagem |
|---|-------------|
| 1 | **Perda de dados se o Library Service estiver offline** — eventos emitidos enquanto o consumer está desconectado são perdidos irrecuperavelmente. O Engine não os re-emite. |
| 2 | **Janelas cegas difíceis de auditar** — não há como saber precisamente o que foi perdido durante uma queda. Para declaração ECAD, isso é problemático. |
| 3 | **Restart do Engine durante a queda do consumer** — o evento `NowPlayingChanged` do início da faixa é perdido; a entrada fica sem metadados. |
| 4 | **Acoplamento de disponibilidade** — Library Service e Engine precisam estar ambos em operação para que o log seja completo. |

---

### Abordagem B — Arquivos append-only no Playout Engine (proposta alternativa)

O Playout Engine escreve arquivos JSONL organizados por dia e hora no filesystem local.
Um processo importador (goroutine no Library Service) lê os arquivos de horas passadas,
persiste no SQLite e os exclui após confirmação da importação.

```
[Playout Engine]
  ↓ append JSONL
[filesystem: logs/YYYY-MM-DD/HH.jsonl]
  ↓ (leitura periódica — goroutine importador)
[Library Service — file importer]
  ↓ INSERT em lote
[SQLite — transmission_log]
  ↓ API REST
[Player UI]
```

Estrutura dos arquivos:
```
logs/
  2026-07-20/
    00.jsonl   ← hora 00:00–00:59 (completo, importável)
    07.jsonl   ← hora 07:00–07:59 (completo, importável)
    08.jsonl   ← hora 08:00–08:59 (em escrita — Engine ainda na hora atual)
```

Cada linha do arquivo é um registro JSON completo, escrito **após** o término da faixa:

```json
{"started_at":"2026-07-20T08:00:00","finished_at":"2026-07-20T08:03:58","title":"Como Nossos Pais","artist":"Elis Regina","type":"MUSIC","duration_played_ms":238000,"result":"finished","isrc":"BRUM71200123","composer":"Milton Nascimento","break_id":"","break_role":""}
```

O importador processa apenas arquivos de **horas passadas** (nunca o arquivo da hora
corrente, pois o Engine ainda pode estar escrevendo nele). Após importar com sucesso,
o arquivo é excluído.

#### Prós

| # | Vantagem |
|---|----------|
| 1 | **Durabilidade total** — o Engine persiste localmente antes de qualquer outro serviço. O Library Service pode ficar offline por horas ou dias; os arquivos aguardam. |
| 2 | **Zero perda de dados** — o arquivo é excluído somente após confirmação de importação bem-sucedida no SQLite (transação atômica). |
| 3 | **Auditabilidade natural** — arquivos append-only são imutáveis; cada linha só é escrita uma vez, após o término da faixa. |
| 4 | **Independência de rede** — Engine e Library Service podem estar em hosts diferentes sem impacto. |
| 5 | **Reimportação pontual** — se um arquivo foi corrompido ou perdido, é possível reimportar apenas aquele período sem afetar o restante. |
| 6 | **Backup implícito** — os arquivos JSONL podem ser copiados para armazenamento externo antes da importação. |

#### Contras

| # | Desvantagem |
|---|-------------|
| 1 | **Viola regra arquitetural do Engine** — o `CLAUDE.md` do playout define explicitamente "Não use banco de dados no Engine nesta fase". Escrita de arquivos é persistência e representa um desvio dessa regra. Requer decisão consciente de flexibilizar o princípio. |
| 2 | **Latência no log** — o dado só aparece no SQLite após a importação (mínimo: no início da hora seguinte). A UI não pode exibir status `PLAYING` em tempo real a partir desse log. |
| 3 | **Processo importador adicional** — nova goroutine com lógica de: polling de diretório, locking, controle de arquivos já importados, tratamento de falha parcial. |
| 4 | **Risco na deleção** — se o arquivo for excluído antes de confirmada a gravação no SQLite (falha de energia, crash), há perda definitiva. Requer protocolo cuidadoso. |
| 5 | **Arquivo da hora corrente** — o Engine escreve continuamente; o importador precisa saber nunca tocar o arquivo da hora atual. Exige convenção clara e testada. |
| 6 | **Acesso compartilhado ao filesystem** — Engine e importador precisam enxergar o mesmo diretório. Complica deploy em containers separados ou hosts diferentes. |

---

### Comparativo resumido

| Critério | Abordagem A (WebSocket) | Abordagem B (Arquivos) |
|----------|:-----------------------:|:----------------------:|
| Durabilidade (sem Library Service) | ❌ Perde dados | ✅ Preserva dados |
| Conformidade ECAD (completude) | ⚠️ Depende de uptime | ✅ Garantida |
| Mudança no Playout Engine | ✅ Nenhuma | ⚠️ Necessária |
| Log em tempo real (status PLAYING) | ✅ Sim | ❌ Não |
| Complexidade de implementação | ✅ Baixa | ⚠️ Média |
| Coerência com arquitetura atual | ✅ Total | ⚠️ Flexibiliza regra |
| Operação em hosts separados | ✅ Fácil | ⚠️ Requer filesystem compartilhado |
| Auditabilidade do log bruto | ❌ Sem rastro externo | ✅ Arquivos JSONL |

### Recomendação

Para o contexto atual do RadioFlow (Engine e Library Service no mesmo host, operação
local), a **Abordagem A** é mais simples, coerente e não requer mudanças no Engine.
O risco de perda de dados é aceitável se o Library Service for monitorado e tiver
restart automático (systemd, supervisor, etc.).

Se a emissora exige conformidade absoluta com o ECAD — sem possibilidade de lacuna no
log —, a **Abordagem B** oferece garantias superiores, ao custo de flexibilizar a
regra de persistência do Engine.

> **Decisão pendente:** a escolha da abordagem deve ser feita antes de iniciar a
> implementação. O restante deste plano descreve a Abordagem A, com anotações sobre
> as diferenças da Abordagem B onde relevante.

---

## Requisitos do ECAD

### O que é o ECAD

O **Escritório Central de Arrecadação e Distribuição** é a entidade responsável pela
arrecadação e distribuição de direitos autorais de execução pública no Brasil. Emissoras
de rádio (AM, FM e Web) são obrigadas por lei (Lei nº 9.610/1998) a declarar
mensalmente ao ECAD todas as obras musicais executadas, com informações suficientes
para identificar a obra e calcular os royalties devidos aos autores e intérpretes.

### Dados exigidos por execução

| Campo | Obrigatoriedade | Descrição |
|-------|:--------------:|-----------|
| **Título da obra** | Obrigatório | Nome da música exatamente como registrada |
| **Intérprete / Artista** | Obrigatório | Nome do artista ou grupo |
| **Data de execução** | Obrigatório | `DD/MM/AAAA` |
| **Horário de início** | Obrigatório | `HH:MM:SS` |
| **Duração executada** | Obrigatório | Duração real reproduzida (`MM:SS`) |
| **Tipo de execução** | Obrigatório | `M` = Mecânica (gravação) / `V` = Ao Vivo |
| **Compositor(es)** | Recomendado | Necessário para distribuição correta dos royalties |
| **ISRC** | Recomendado | Identifica univocamente a gravação; elimina ambiguidades de título |
| **Editora / Publisher** | Opcional | Nome da editora musical |

> **Nota sobre tipos:** o ECAD é relevante apenas para **obras musicais** — tipos
> `MUSIC`, `JINGLE` e `VINHETA` com composição identificável. Spots comerciais (`SPOT`)
> são veiculação publicitária, regida pelo CENP/Conar, não pelo ECAD. Hora Certa
> (`HORA_CERTA`) não contém obra musical. A exportação ECAD deve filtrar apenas os
> tipos relevantes.

### Dados da emissora (cabeçalho do arquivo)

| Campo | Descrição |
|-------|-----------|
| Nome fantasia | Nome da rádio (ex: "Rádio Exemplo FM") |
| CNPJ | CNPJ da pessoa jurídica |
| Frequência | Ex: "98.5 MHz" |
| Tipo | FM / AM / WEB |
| Município | Cidade sede |
| UF | Estado (sigla) |
| Período declarado | Mês/ano de referência |

Esses dados são fixos por instalação e devem ser configuráveis no Library Service
(`config.yaml`, seção `station`).

### Campos adicionais na tabela `tracks`

Para suportar o ECAD, a tabela `tracks` precisa de campos que a indexação atual
não captura. Serão adicionados na **migration 005**:

```sql
ALTER TABLE tracks ADD COLUMN isrc     TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN composer TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN publisher TEXT NOT NULL DEFAULT '';
```

- `isrc` — lido da tag ID3 `TSRC` ou Vorbis `ISRC` via ffprobe
- `composer` — lido da tag ID3 `TCOM` ou Vorbis `COMPOSER` via ffprobe
- `publisher` — lido da tag ID3 `TPUB` ou Vorbis `ORGANIZATION` via ffprobe

O scanner deve extrair esses campos na indexação. Faixas já indexadas recebem os
valores ao próximo re-scan ou manualmente via `PATCH /v1/tracks/{id}`.

### Formato do arquivo de declaração ECAD

O ECAD aceita declaração eletrônica via seu portal (ECAD Online) em formato CSV com
separador ponto-e-vírgula (`; `), encoding UTF-8 sem BOM.

#### Estrutura do arquivo

```
logs/ecad/YYYY-MM_declaracao.csv
```

#### Linha de cabeçalho do arquivo (registro tipo `H`)

```
H;NOME_EMISSORA;CNPJ;MUNICIPIO;UF;FREQUENCIA;TIPO_EMISSORA;PERIODO_INI;PERIODO_FIM
```

**Exemplo:**
```
H;Radio Exemplo FM;12.345.678/0001-90;São Paulo;SP;98.5 MHz;FM;01/07/2026;31/07/2026
```

#### Linhas de detalhe (registro tipo `D`) — uma por execução musical

```
D;DATA;HORA_INICIO;DURACAO;TITULO;ARTISTA;COMPOSITOR;ISRC;TIPO_EXECUCAO;TIPO_UTILIZACAO
```

| Coluna | Formato | Exemplo |
|--------|---------|---------|
| `DATA` | `DD/MM/AAAA` | `20/07/2026` |
| `HORA_INICIO` | `HH:MM:SS` | `08:00:00` |
| `DURACAO` | `MM:SS` | `03:58` |
| `TITULO` | texto livre | `Como Nossos Pais` |
| `ARTISTA` | texto livre | `Elis Regina` |
| `COMPOSITOR` | texto livre | `Milton Nascimento` |
| `ISRC` | `CC-XXX-YY-NNNNN` ou vazio | `BR-UM7-12-00123` |
| `TIPO_EXECUCAO` | `M` ou `V` | `M` |
| `TIPO_UTILIZACAO` | `R` (radiodifusão) | `R` |

**Exemplo de arquivo completo:**

```csv
H;Radio Exemplo FM;12.345.678/0001-90;São Paulo;SP;98.5 MHz;FM;01/07/2026;31/07/2026
D;20/07/2026;08:00:00;03:58;Como Nossos Pais;Elis Regina;Milton Nascimento;BR-UM7-12-00123;M;R
D;20/07/2026;08:04:02;00:28;Vinheta Manhã;Radio Exemplo;;; M;R
D;20/07/2026;08:10:30;04:22;Garota de Ipanema;Tom Jobim;Tom Jobim / Vinícius de Moraes;BR-ABC-63-00001;M;R
D;20/07/2026;08:15:00;03:11;Águas de Março;Elis Regina / Tom Jobim;Tom Jobim;;M;R
```

#### Endpoint de exportação ECAD

```
GET /v1/transmission-log/export/ecad?from=2026-07-01&to=2026-07-31
```

- `Content-Type: text/csv; charset=utf-8`
- `Content-Disposition: attachment; filename="ecad_2026-07_declaracao.csv"`
- Filtra automaticamente apenas tipos `MUSIC`, `JINGLE` e `VINHETA`
- Ordena por `started_at` ASC
- Inclui apenas entradas com `status=FINISHED` (duração real > 0)
- Junta com tabela `tracks` para obter `isrc`, `composer`, `publisher`

---

## Estratégia de coleta (Abordagem A)

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

### Migration 005 — transmission_log + campos ECAD em tracks

Arquivo: `internal/store/migrations/005_transmission_log.sql`

```sql
-- Campos adicionais na tabela tracks para suporte ao ECAD
ALTER TABLE tracks ADD COLUMN isrc      TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN composer  TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN publisher TEXT NOT NULL DEFAULT '';

-- Log de transmissão
CREATE TABLE IF NOT EXISTS transmission_log (
    id                 TEXT     PRIMARY KEY,
    queue_item_id      TEXT     NOT NULL DEFAULT '',
    asset_id           TEXT     NOT NULL DEFAULT '',
    path               TEXT     NOT NULL DEFAULT '',
    title              TEXT     NOT NULL DEFAULT '',
    artist             TEXT     NOT NULL DEFAULT '',
    type               TEXT     NOT NULL DEFAULT '',   -- MUSIC|JINGLE|VINHETA|SPOT|CART|HORA_CERTA
    isrc               TEXT     NOT NULL DEFAULT '',   -- copiado de tracks.isrc no momento da execução
    composer           TEXT     NOT NULL DEFAULT '',   -- copiado de tracks.composer
    publisher          TEXT     NOT NULL DEFAULT '',   -- copiado de tracks.publisher
    duration_ms        INTEGER  NOT NULL DEFAULT 0,
    duration_played_ms INTEGER  NOT NULL DEFAULT 0,
    result             TEXT     NOT NULL DEFAULT '',   -- finished|skipped|failed|interrupted
    status             TEXT     NOT NULL DEFAULT 'PLAYING', -- PLAYING|FINISHED|SKIPPED|FAILED|INTERRUPTED
    started_at         DATETIME NOT NULL,
    finished_at        DATETIME,
    break_id           TEXT     NOT NULL DEFAULT '',
    break_title        TEXT     NOT NULL DEFAULT '',
    break_role         TEXT     NOT NULL DEFAULT '',   -- open|spot|close
    break_position     INTEGER  NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_transmission_log_started_at ON transmission_log(started_at);
CREATE INDEX IF NOT EXISTS idx_transmission_log_type       ON transmission_log(type);
CREATE INDEX IF NOT EXISTS idx_transmission_log_status     ON transmission_log(status);
CREATE INDEX IF NOT EXISTS idx_transmission_log_asset_id   ON transmission_log(asset_id);
```

> `isrc`, `composer` e `publisher` são copiados da tabela `tracks` no momento da
> abertura da entrada (`OpenEntry`). Isso garante que a declaração ECAD reflita os
> metadados vigentes na data da execução, mesmo que a faixa seja editada depois.

---

## Estrutura de pacotes novos

```
library/
  internal/
    config/
      config.go                        ← adicionar PlayoutConfig + StationConfig
    store/
      migrations/
        005_transmission_log.sql       ← migration com ALTER TABLE + CREATE TABLE
      transmission_log_store.go        ← TransmissionLogStore
    logconsumer/
      consumer.go                      ← goroutine WebSocket client
      consumer_test.go
    api/
      handlers/
        transmission_log.go            ← handlers GET + exportações
```

---

## Config — novas seções

```go
// PlayoutConfig holds connection settings for the Playout Engine.
type PlayoutConfig struct {
    URL            string        `yaml:"url"`             // ws://127.0.0.1:8080
    ReconnectDelay time.Duration `yaml:"reconnect_delay"` // default: 5s
}

// StationConfig holds broadcast station identification for regulatory reports.
type StationConfig struct {
    Name      string `yaml:"name"`       // ex: "Rádio Exemplo FM"
    CNPJ      string `yaml:"cnpj"`       // ex: "12.345.678/0001-90"
    Frequency string `yaml:"frequency"`  // ex: "98.5 MHz"
    Type      string `yaml:"type"`       // FM | AM | WEB
    City      string `yaml:"city"`
    State     string `yaml:"state"`      // sigla UF
}
```

---

## Store — TransmissionLogStore

```go
type TransmissionLogEntry struct {
    ID               string
    QueueItemID      string
    AssetID          string
    Path             string
    Title            string
    Artist           string
    Type             string
    ISRC             string
    Composer         string
    Publisher        string
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
    Search string
    Limit  int
    Offset int
}

func (s *TransmissionLogStore) OpenEntry(ctx context.Context, e TransmissionLogEntry) error
func (s *TransmissionLogStore) CloseEntry(ctx context.Context, queueItemID, result string, durationPlayedMS int64, finishedAt time.Time) error
func (s *TransmissionLogStore) CloseCartEntry(ctx context.Context, cartID, result string, finishedAt time.Time) error
func (s *TransmissionLogStore) MarkInterrupted(ctx context.Context, beforeTime time.Time) error
func (s *TransmissionLogStore) List(ctx context.Context, q TransmissionLogQuery) ([]TransmissionLogEntry, int, error)
func (s *TransmissionLogStore) ExportCSV(ctx context.Context, from, to time.Time, w io.Writer) error
func (s *TransmissionLogStore) ExportECAD(ctx context.Context, from, to time.Time, station config.StationConfig, w io.Writer) error
```

---

## Log Consumer

```go
package logconsumer

type TransmissionStore interface {
    OpenEntry(ctx context.Context, e store.TransmissionLogEntry) error
    CloseEntry(ctx context.Context, queueItemID, result string, durationPlayedMS int64, finishedAt time.Time) error
    CloseCartEntry(ctx context.Context, cartID, result string, finishedAt time.Time) error
    MarkInterrupted(ctx context.Context, beforeTime time.Time) error
}

func New(cfg config.PlayoutConfig, store TransmissionStore, log *slog.Logger) *Consumer
func (c *Consumer) Run(ctx context.Context) error
```

### Lógica de processamento de eventos

```go
switch event.Type {
case "NowPlayingChanged":
    // ignora se title e path estão vazios (engine idle)
    // busca isrc, composer, publisher em tracks WHERE asset_id
    // INSERT OR IGNORE entry (idempotência no reconnect)
case "ItemFinished":
    // UPDATE WHERE queue_item_id → status, duration_played_ms, finished_at
case "CartStarted":
    // INSERT entry (type=CART, status=PLAYING)
case "CartStopped":
    // UPDATE WHERE asset_id=cart_id → FINISHED/STOPPED
}
```

### Reconexão com backoff

```go
delay := cfg.ReconnectDelay  // 5s
for {
    connectTime := time.Now()
    err := c.connect(ctx)
    if ctx.Err() != nil { return nil }
    c.store.MarkInterrupted(ctx, connectTime) // fecha entradas orfãs
    select {
    case <-ctx.Done(): return nil
    case <-time.After(delay):
    }
    delay = min(delay*2, 60*time.Second)
}
```

---

## API — endpoints

### Listar log de transmissão

```
GET /v1/transmission-log?from=&to=&type=&status=&q=&limit=&offset=
```

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
        "title":              "Como Nossos Pais",
        "artist":             "Elis Regina",
        "composer":           "Milton Nascimento",
        "isrc":               "BR-UM7-12-00123",
        "type":               "MUSIC",
        "duration_ms":        243000,
        "duration_played_ms": 238000,
        "result":             "finished",
        "status":             "FINISHED",
        "started_at":         "2026-07-20T08:00:00Z",
        "finished_at":        "2026-07-20T08:03:58Z",
        "break_role":         ""
      }
    ],
    "total": 142,
    "limit": 100,
    "offset": 0
  }
}
```

### Exportar CSV (auditoria interna / anunciantes)

```
GET /v1/transmission-log/export?from=2026-07-20&to=2026-07-20
```

`Content-Type: text/csv` · `Content-Disposition: attachment; filename="transmission_log_2026-07-20.csv"`

Colunas:
```
id;started_at;finished_at;duration_played_ms;title;artist;composer;isrc;type;result;status;break_role;path
```

### Exportar declaração ECAD

```
GET /v1/transmission-log/export/ecad?from=2026-07-01&to=2026-07-31
```

`Content-Type: text/csv; charset=utf-8` · `Content-Disposition: attachment; filename="ecad_2026-07_declaracao.csv"`

Filtros internos automáticos:
- `type IN ('MUSIC','JINGLE','VINHETA')`
- `status = 'FINISHED'`
- `duration_played_ms > 0`
- Ordem: `started_at ASC`

### Resumo do dia

```
GET /v1/transmission-log/summary?date=2026-07-20
```

```json
{
  "ok": true,
  "data": {
    "date": "2026-07-20",
    "total": 148,
    "by_type": { "MUSIC": 92, "VINHETA": 18, "JINGLE": 12, "SPOT": 20, "CART": 6 },
    "total_played_ms": 18720000
  }
}
```

---

## Registro de rotas

```go
mux.HandleFunc("GET /v1/transmission-log",              handlers.ListTransmissionLog(s.tls))
mux.HandleFunc("GET /v1/transmission-log/export",       handlers.ExportTransmissionLog(s.tls))
mux.HandleFunc("GET /v1/transmission-log/export/ecad",  handlers.ExportECAD(s.tls, s.cfg.Station))
mux.HandleFunc("GET /v1/transmission-log/summary",      handlers.GetTransmissionLogSummary(s.tls))
```

---

## Player UI — painel de Histórico

Nova aba **"Histórico"** no drawer, após "Rotação".

**Barra de filtros:**
```
[Data: 20/07/2026] [Tipo: Todos ▾] [buscar título ou artista…] [Buscar]
```

**Tabela:**
```
INÍCIO    TÍTULO              ARTISTA        TIPO     DURAÇÃO  STATUS
08:00:00  Como Nossos Pais   Elis Regina    MUSIC    3:58     ✅
08:04:02  Vinheta Manhã 01   —              VINHETA  0:28     ✅
08:04:30  Anúncio X          —              SPOT     0:30     ✅
```

- Ícone de tipo (padrão da fila)
- Status: verde (FINISHED), amarelo (SKIPPED), vermelho (FAILED/INTERRUPTED), ciano pulsante (PLAYING)

**Rodapé:**
```
142 entradas · 5h 12m 00s    [ ↓ CSV ]  [ ↓ ECAD ]
```

O botão **ECAD** abre `GET /v1/transmission-log/export/ecad` com o mês filtrado.

**Atualização em tempo real:** ao receber `NowPlayingChanged` via WebSocket, re-busca
o log se o filtro de data for o dia atual.

---

## Fases de implementação

### Fase 1 — Migração e Store

1. Criar `internal/store/migrations/005_transmission_log.sql`
   - `ALTER TABLE tracks ADD COLUMN isrc/composer/publisher`
   - `CREATE TABLE transmission_log`
2. Registrar migration 005 em `db.go`
3. Atualizar scanner para extrair `TSRC`, `TCOM`, `TPUB` das tags ID3 via ffprobe
4. Implementar `TransmissionLogStore` com todos os métodos, incluindo `ExportECAD`
5. Adicionar `StationConfig` em `config.go`
6. Testes: open/close, MarkInterrupted, List com filtros, ExportCSV, ExportECAD

### Fase 2 — Log Consumer

1. Adicionar `PlayoutConfig` em `config.go`
2. Implementar `internal/logconsumer/consumer.go`
3. Testes com mock do store e canal de eventos simulado

### Fase 3 — API e Rotas

1. Implementar `handlers/transmission_log.go` (4 handlers)
2. Definir interface `TransmissionLogStore` no pacote `handlers`
3. Registrar rotas em `server.go`
4. Injetar stores e consumer em `main.go`
5. Testes com `httptest.NewRecorder`

### Fase 4 — Player UI

1. Adicionar aba "Histórico" no drawer
2. Implementar filtros, tabela e rodapé
3. Botões de exportação CSV e ECAD
4. Atualização em tempo real via WebSocket

---

## Pontos de atenção

### `NowPlayingChanged` vs `ItemStarted`
`ItemStartedPayload` não tem título nem artista. O consumer abre entradas via
`NowPlayingChanged`, que contém todos os metadados.

### Idempotência no reconnect
Ao reconectar, o Engine emite `NowPlayingChanged` com a faixa em reprodução. Usar
`INSERT OR IGNORE` com `queue_item_id` para evitar entrada duplicada.

### Snapshot de metadados ECAD
`isrc`, `composer` e `publisher` são copiados para `transmission_log` no momento da
execução — não buscados em `tracks` na hora da exportação. Garante que edições
posteriores na faixa não alterem declarações passadas.

### Consumer opcional
Se `cfg.Playout.URL` estiver vazio, o consumer não é iniciado. Serviço funciona
normalmente sem log de transmissão ativo.

### Abordagem B — diferenças de implementação
Se a decisão for pela Abordagem B (arquivos), as mudanças são:
- Playout Engine: goroutine de escrita de JSONL por hora em `internal/transmissionlog/`
- Library Service: substituir `logconsumer` por `fileimporter` (polling de diretório, importação de arquivos de horas passadas, exclusão após confirmação)
- O restante (store, API, Player UI) permanece idêntico

---

## Definição de pronto

- `go test ./...` passa sem erros
- `go vet ./...` sem avisos
- `go test -race ./...` sem data races
- Consumer reconecta e marca entradas como INTERRUPTED
- `GET /v1/transmission-log` filtra por data, tipo e busca
- `GET /v1/transmission-log/export` gera CSV com todas as colunas
- `GET /v1/transmission-log/export/ecad` gera arquivo no formato ECAD com cabeçalho H e linhas D, apenas para MUSIC/JINGLE/VINHETA com FINISHED
- Player UI exibe histórico do dia atual com atualização em tempo real
- Botão ECAD exporta o mês corrente do filtro ativo
- `isrc`, `composer`, `publisher` extraídos na indexação e registrados no log
