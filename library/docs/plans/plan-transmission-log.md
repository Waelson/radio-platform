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
**Abordagem escolhida:** B — Arquivos append-only no Playout Engine

---

## Contexto atual

### Playout Engine
O Engine já publica eventos no Event Bus (`internal/events`) a cada transição de faixa.
Os eventos relevantes e seus payloads são:

| Evento | Payload relevante |
|--------|-------------------|
| `NowPlayingChanged` | `queue_item_id`, `asset_id`, `path`, `title`, `artist`, `type`, `duration_ms`, `isrc`, `composer`, `publisher`, `break_id`, `break_title`, `break_position`, `break_total`, `break_role` |
| `ItemFinished` | `queue_item_id`, `asset_id`, `result`, `duration_played_ms` |
| `CartStarted` | `cart_id`, `path`, `title`, `artist`, `duration_ms` |
| `CartStopped` | `cart_id`, `reason` |

O Engine **não persiste nada** — apenas emite eventos. A Abordagem B flexibiliza essa
regra pontualmente, introduzindo escrita de arquivos JSONL como persistência leve e
isolada do pipeline de áudio.

### Library Service
Já possui:
- Padrão de migração em `internal/store/migrations/*.sql` (próxima: `005`)
- Padrão de store com `database/sql` puro
- Padrão de handler retornando `http.HandlerFunc`
- Config estruturada em `internal/config/config.go`
- `main.go` com injeção de dependências e goroutines com shutdown via context

### Config
A URL do Playout Engine **não existe** ainda em `Config`. Será adicionada (seção
`playout`) junto com o diretório de logs compartilhado.

---

## Abordagem arquitetural — Arquivos append-only no Playout Engine

O Playout Engine escreve arquivos JSONL organizados por dia e hora no filesystem local.
Um processo importador (goroutine no Library Service) lê os arquivos de horas passadas,
persiste no SQLite e os exclui após confirmação da importação.

```
[Playout Engine]
  ↓ append JSONL
[filesystem: {log_dir}/{file_name_template}]     ← todos os arquivos na raiz
  ↓ (leitura periódica — goroutine importador)
[Library Service — file importer]
  ↓ INSERT em lote + move para {log_dir}/processados/
[SQLite — transmission_log]
  ↓ API REST
[Player UI]
```

#### Prós

| # | Vantagem |
|---|----------|
| 1 | **Durabilidade total** — Engine persiste localmente. Library Service pode ficar offline por horas. |
| 2 | **Zero perda de dados** — arquivo excluído somente após confirmação de importação bem-sucedida. |
| 3 | **Auditabilidade natural** — arquivos append-only são imutáveis. |
| 4 | **Independência de rede** — Engine e Library Service independentes. |
| 5 | **Reimportação pontual** — reimportar apenas o período afetado. |
| 6 | **Backup implícito** — arquivos JSONL podem ser copiados antes da importação. |

#### Contras

| # | Desvantagem |
|---|-------------|
| 1 | **Flexibiliza regra de "sem persistência" no Engine** — decisão consciente e aceita. |
| 2 | **Latência no log** — dado aparece no SQLite com até ~1h15min de atraso. |
| 3 | **Processo importador adicional** — nova goroutine no Library Service. |
| 4 | **Acesso compartilhado ao filesystem** — Engine e importer precisam enxergar o mesmo diretório. |

> **Decisão:** conformidade ECAD sem lacunas é requisito não negociável. A
> flexibilização da regra de "sem persistência" no Engine é pontual e isolada —
> o LogWriter não toca o pipeline de áudio.

---

## Requisitos do ECAD

### O que é o ECAD

O **Escritório Central de Arrecadação e Distribuição** é a entidade responsável pela
arrecadação e distribuição de direitos autorais de execução pública no Brasil. Emissoras
de rádio (AM, FM e Web) são obrigadas por lei (Lei nº 9.610/1998) a declarar
mensalmente ao ECAD todas as obras musicais executadas.

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
| **ISRC** | Recomendado | Identifica univocamente a gravação |
| **Editora / Publisher** | Opcional | Nome da editora musical |

> **Nota sobre tipos:** o ECAD é relevante apenas para tipos `MUSIC`, `JINGLE` e
> `VINHETA`. Spots (`SPOT`) são veiculação publicitária (CENP/Conar). A exportação
> ECAD filtra automaticamente apenas os tipos relevantes.

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

Esses dados são fixos por instalação e mantidos na tabela `settings` do banco de dados.

### Campos adicionais na tabela `tracks`

Adicionados na **migration 005**:

```sql
ALTER TABLE tracks ADD COLUMN isrc      TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN composer  TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN publisher TEXT NOT NULL DEFAULT '';
```

- `isrc` — tag ID3 `TSRC` ou Vorbis `ISRC` via ffprobe
- `composer` — tag ID3 `TCOM` ou Vorbis `COMPOSER` via ffprobe
- `publisher` — tag ID3 `TPUB` ou Vorbis `ORGANIZATION` via ffprobe

### Formato do arquivo de declaração ECAD

CSV com separador ponto-e-vírgula (`;`), encoding UTF-8 sem BOM.

#### Linha de cabeçalho (registro tipo `H`)

```
H;NOME_EMISSORA;CNPJ;MUNICIPIO;UF;FREQUENCIA;TIPO_EMISSORA;PERIODO_INI;PERIODO_FIM
```

**Exemplo:**
```
H;Radio Exemplo FM;12.345.678/0001-90;São Paulo;SP;98.5 MHz;FM;01/07/2026;31/07/2026
```

#### Linhas de detalhe (registro tipo `D`)

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
D;20/07/2026;08:04:02;00:28;Vinheta Manhã;Radio Exemplo;;;M;R
D;20/07/2026;08:10:30;04:22;Garota de Ipanema;Tom Jobim;Tom Jobim / Vinícius de Moraes;BR-ABC-63-00001;M;R
```

#### Endpoint de exportação ECAD

```
GET /v1/transmission-log/export/ecad?from=2026-07-01&to=2026-07-31
```

- `Content-Type: text/csv; charset=utf-8`
- `Content-Disposition: attachment; filename="ecad_2026-07_declaracao.csv"`
- Filtra: `type IN ('MUSIC','JINGLE','VINHETA')`, `status = 'FINISHED'`, `duration_played_ms > 0`
- Ordena por `started_at ASC`

---

## Estratégia de coleta — Abordagem B (Arquivos append-only)

### Visão geral do fluxo completo

```
[Audio Pipeline — hot path]
        │
        │  bus.Publish() — já existe, sem mudança
        ↓
[Event Bus — bus.go]
        │
        │  select { case ch <- evt: default: drop }  ← nunca bloqueia
        ↓
[LogWriter.ch — chan Event, buffer 256]
        │
        │  goroutine dedicada LogWriter.run()
        ↓
[In-memory pending map: queue_item_id → PendingEntry]
        │
        │  ao receber ItemFinished → entrada completa
        ↓
[os.OpenFile(O_APPEND) + json.Marshal + f.Write + f.Sync()]
        │
        ↓
[filesystem: {log_dir}/{file_name_template}]
        │
        │  Library Service — fileimporter goroutine (poll a cada 5min)
        ↓
[SQLite — transmission_log]
        │
        ↓
[API REST + Player UI]
```

---

## Detalhamento técnico — LogWriter (Playout Engine)

### Por que o hot path não é afetado

Esta é a garantia central do design. Três camadas de isolamento protegem o áudio:

#### Camada 1 — O Event Bus já é não-bloqueante

O código existente em `internal/events/bus.go` já garante:

```go
// Publish — código existente, sem modificação necessária
for _, s := range subs {
    select {
    case s.ch <- evt:   // entrega ao subscriber
    default:            // subscriber lento → descarta silenciosamente
        if IsCritical(evt.Type) && b.log != nil {
            b.log.Warn("event bus: slow consumer dropped critical event", ...)
        }
    }
}
```

O LogWriter é apenas mais um subscriber registrado via `bus.Subscribe(256)`. Se o
canal do LogWriter estiver cheio (disco lento, falha de I/O), o evento é descartado
com `default`. O áudio continua sem nenhum impacto.

#### Camada 2 — `ItemFinished` não é publicado no hot path de áudio

O hot path do Engine é a goroutine que decodifica samples PCM e escreve no output
device. `ItemFinished` é publicado pelo **PlaybackManager** ao detectar EOF do
decoder — em uma goroutine de controle separada da goroutine de áudio. Mesmo que
houvesse alguma latência no Publish, o impacto seria no controle, nunca no áudio.

#### Camada 3 — File I/O ocorre exclusivamente na goroutine LogWriter.run()

A goroutine `LogWriter.run()` é a única proprietária dos handles de arquivo. Nenhuma
outra goroutine do Engine acessa esses `*os.File`. Portanto:
- Sem mutex para acesso ao arquivo corrente.
- Sem contenção entre a goroutine de áudio e a goroutine de log.
- Disco lento bloqueia apenas o LogWriter — que consome da sua própria fila de eventos.

### Estrutura de dados

```go
// playout/internal/transmissionlog/writer.go

package transmissionlog

// LogEntry é o registro gravado em cada linha do arquivo JSONL.
// Uma entrada representa uma faixa completamente reproduzida (ou interrompida).
// Escrita uma única vez, após o término da faixa (ItemFinished / CartStopped).
//
// isrc, composer e publisher são incluídos aqui porque chegam ao Engine via
// payload do comando ENQUEUE e são propagados no NowPlayingChangedPayload.
// O importer os grava diretamente no SQLite — sem consultas adicionais.
type LogEntry struct {
    StartedAt        time.Time `json:"started_at"`
    FinishedAt       time.Time `json:"finished_at"`
    QueueItemID      string    `json:"queue_item_id"`
    AssetID          string    `json:"asset_id"`
    Title            string    `json:"title"`
    Artist           string    `json:"artist"`
    Type             string    `json:"type"`                // MUSIC|JINGLE|VINHETA|SPOT|CART
    DurationMS       int64     `json:"duration_ms"`
    DurationPlayedMS int64     `json:"duration_played_ms"`
    Result           string    `json:"result"`              // finished|skipped|failed
    ISRC             string    `json:"isrc,omitempty"`
    Composer         string    `json:"composer,omitempty"`
    Publisher        string    `json:"publisher,omitempty"`
    BreakID          string    `json:"break_id,omitempty"`
    BreakTitle       string    `json:"break_title,omitempty"`
    BreakRole        string    `json:"break_role,omitempty"` // open|spot|close
    BreakPosition    int       `json:"break_position,omitempty"`
}

// pendingEntry é mantido em memória entre NowPlayingChanged e ItemFinished.
// Acessado exclusivamente pela goroutine run() — sem mutex necessário.
type pendingEntry struct {
    startedAt time.Time
    meta      events.NowPlayingChangedPayload
}

type Writer struct {
    dir string
    bus *events.Bus
    log *slog.Logger
}

func New(dir string, bus *events.Bus, log *slog.Logger) *Writer {
    return &Writer{dir: dir, bus: bus, log: log}
}
```

> **Decisão de design:** o processo de importação é totalmente independente — sem
> consultas de enriquecimento, sem JOINs entre tabelas. Para isso, `isrc`, `composer`
> e `publisher` devem chegar ao Engine via payload do comando `ENQUEUE` e ser
> propagados no `NowPlayingChangedPayload`. O LogWriter os captura em `NowPlayingChanged`
> e os grava diretamente no JSONL. O importer insere sem nenhuma consulta adicional.

### Lógica do LogWriter.run()

```go
func (w *Writer) Run(ctx context.Context) error {
    ch, cancel := w.bus.Subscribe(256)
    defer cancel()

    // Estado interno — exclusivo desta goroutine, sem mutex
    pending     := make(map[string]pendingEntry) // queue_item_id → pendingEntry
    cartPending := make(map[string]pendingEntry) // cart_id → pendingEntry

    var (
        curFile *os.File
        curHour = -1
        curDay  string
    )

    closeFile := func() {
        if curFile != nil {
            curFile.Sync()
            curFile.Close()
            curFile = nil
            curHour = -1
        }
    }
    defer closeFile()

    writeEntry := func(entry LogEntry) {
        t    := entry.FinishedAt.UTC()
        day  := t.Format("20060102") // yyyyMMdd
        hour := t.Hour()

        if curFile == nil || day != curDay || hour != curHour {
            closeFile()
            name := buildFileName(w.cfg.FileNameTemplate, day, hour)
            path := filepath.Join(w.dir, name)
            f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
            if err != nil {
                w.log.Error("transmissionlog: open failed", "path", path, "err", err)
                return // descarta esta entrada; próximas tentarão reabrir
            }
            curFile = f
            curDay  = day
            curHour = hour
        }

        line, _ := json.Marshal(entry)
        line = append(line, '\n')
        if _, err := curFile.Write(line); err != nil {
            w.log.Error("transmissionlog: write failed", "err", err)
            closeFile() // força reabertura na próxima entrada
            return
        }
        if err := curFile.Sync(); err != nil {
            w.log.Warn("transmissionlog: sync failed", "err", err)
        }
    }

    for {
        select {
        case <-ctx.Done():
            return nil

        case evt := <-ch:
            switch evt.Type {

            case events.EvtNowPlayingChanged:
                p, ok := evt.Payload.(events.NowPlayingChangedPayload)
                if !ok || p.QueueItemID == "" || p.Title == "" {
                    continue // engine transitando para IDLE
                }
                pending[p.QueueItemID] = pendingEntry{
                    startedAt: evt.Timestamp,
                    meta:      p,
                }

            case events.EvtItemFinished:
                p, ok := evt.Payload.(events.ItemFinishedPayload)
                if !ok {
                    continue
                }
                pe, found := pending[p.QueueItemID]
                if !found {
                    // Engine reiniciou durante a faixa — sem NowPlayingChanged anterior.
                    // Sem metadados suficientes para o ECAD; entrada ignorada.
                    continue
                }
                delete(pending, p.QueueItemID)
                writeEntry(LogEntry{
                    StartedAt:        pe.startedAt,
                    FinishedAt:       evt.Timestamp,
                    QueueItemID:      p.QueueItemID,
                    AssetID:          p.AssetID,
                    Title:            pe.meta.Title,
                    Artist:           pe.meta.Artist,
                    Type:             pe.meta.Type,
                    DurationMS:       pe.meta.DurationMS,
                    DurationPlayedMS: p.DurationPlayedMS,
                    Result:           p.Result,
                    BreakID:          pe.meta.BreakID,
                    BreakTitle:       pe.meta.BreakTitle,
                    BreakRole:        pe.meta.BreakRole,
                    BreakPosition:    pe.meta.BreakPosition,
                })

            case events.EvtCartStarted:
                p, ok := evt.Payload.(events.CartStartedPayload)
                if !ok {
                    continue
                }
                cartPending[p.CartID] = pendingEntry{
                    startedAt: evt.Timestamp,
                    meta: events.NowPlayingChangedPayload{
                        QueueItemID: p.CartID,
                        AssetID:     p.CartID,
                        Title:       p.Title,
                        Artist:      p.Artist,
                        Type:        "CART",
                        DurationMS:  p.DurationMS,
                    },
                }

            case events.EvtCartStopped:
                p, ok := evt.Payload.(events.CartStoppedPayload)
                if !ok {
                    continue
                }
                pe, found := cartPending[p.CartID]
                if !found {
                    continue
                }
                delete(cartPending, p.CartID)
                result := "finished"
                if p.Reason == "manual" {
                    result = "skipped"
                }
                writeEntry(LogEntry{
                    StartedAt:        pe.startedAt,
                    FinishedAt:       evt.Timestamp,
                    QueueItemID:      pe.meta.QueueItemID,
                    AssetID:          pe.meta.AssetID,
                    Title:            pe.meta.Title,
                    Artist:           pe.meta.Artist,
                    Type:             "CART",
                    DurationMS:       pe.meta.DurationMS,
                    DurationPlayedMS: evt.Timestamp.Sub(pe.startedAt).Milliseconds(),
                    Result:           result,
                })
            }
        }
    }
}
```

### Segurança das escritas em arquivo

| Mecanismo | Garantia |
|-----------|----------|
| `O_APPEND\|O_CREATE\|O_WRONLY` | Kernel posiciona o cursor no fim antes de cada `Write()` — garantia POSIX. Sem risco de sobrescrever dados anteriores. |
| Goroutine única escrevendo | Sem concorrência no arquivo corrente. Sem mutex necessário. |
| `f.Sync()` por linha | Dado chega ao disco antes de retornar. Em crash imediato após Write, a linha está durável. |
| Rotação de arquivo | Ao mudar de hora, fecha (Sync + Close) e abre novo. Cada arquivo gerado pelo template é um segmento completo e fechado. |
| Falha de Write/Sync → closeFile() | Força reabertura na próxima entrada. A entrada com falha é perdida; as seguintes são gravadas normalmente. |
| Canal cheio → drop silencioso | Event Bus descarta via `default`. Audio continua. Warning logado. |

> **Por que `Sync()` por linha e não `bufio.Writer`:** um buffer acumula várias linhas
> antes do syscall. Em crash entre o buffer e o flush, linhas são perdidas sem rastro.
> Na frequência de um rádio (1 faixa a cada 3–5 min), o custo de um `Sync()` por
> linha é inferior a 1ms — irrelevante. A durabilidade garante o compromisso com o ECAD.

### Determinação do arquivo pelo `FinishedAt`

O arquivo recebe entradas pelo **horário em que a faixa terminou** (`FinishedAt`):

```
faixa começa às 08:58 → NowPlayingChanged (startedAt = 08:58)
faixa termina às 09:02 → ItemFinished (finishedAt = 09:02)
→ linha gravada em 09.jsonl (hora de finished_at)
```

Consequência: `started_at` pode ser de uma hora diferente do arquivo que a contém.
Isso é correto — o nome do arquivo é apenas um mecanismo de particionamento. O campo
`started_at` dentro da linha contém o horário real de início, que é o que importa
para o ECAD.

### Configuração no Playout Engine

```go
// playout/internal/config/config.go — adição
type TransmissionLogConfig struct {
    Enabled          bool   `yaml:"enabled"`            // false por padrão — opt-in explícito
    Dir              string `yaml:"dir"`                // default: "./transmission-logs"
    FileNameTemplate string `yaml:"file_name_template"` // default: "transmission_{date}_{hour}.jsonl"
}
```

```yaml
# playout/config.yaml — nova seção
transmission_log:
  enabled: true
  dir: "/var/radioflow/transmission-logs"
  file_name_template: "transmission_{date}_{hour}.jsonl"
```

O template suporta dois placeholders:
- `{date}` — substituído por `yyyyMMdd` (ex: `20260720`, UTC)
- `{hour}` — substituído por `HH` zero-preenchido (UTC)

Todos os arquivos são gerados na **raiz** do diretório configurado — sem subdivisão
por data. Após importação pelo Library Service, o arquivo é movido para o subdiretório
`processados/` (criado automaticamente se não existir).

Estrutura de diretório em operação:
```
/var/radioflow/transmission-logs/
  transmission_20260720_08.jsonl   ← aguardando importação
  transmission_20260720_09.jsonl   ← aguardando importação
  processados/
    transmission_20260719_22.jsonl  ← já importado
    transmission_20260720_07.jsonl  ← já importado
```

```go
// buildFileName substitui os placeholders {date} e {hour} no template.
// Ex: "transmission_{date}_{hour}.jsonl" → "transmission_20260720_08.jsonl"
func buildFileName(template, date string, hour int) string {
    s := strings.ReplaceAll(template, "{date}", date)
    s  = strings.ReplaceAll(s, "{hour}", fmt.Sprintf("%02d", hour))
    return s
}
```

O `Writer` só é instanciado e iniciado se `cfg.TransmissionLog.Enabled == true`.
Sem config → sem goroutine → zero impacto no Engine.

---

## Detalhamento técnico — File Importer (Library Service)

### Responsabilidades

O `fileimporter` é uma goroutine do Library Service que:

1. Escaneia periodicamente a **raiz** do diretório de logs.
2. Filtra apenas arquivos cujo nome satisfaz o glob derivado do `file_name_template`.
3. Identifica arquivos seguros para importação (fora do grace period).
4. Para cada arquivo: registra o início da tentativa em `transmission_import_log`.
5. Lê e parseia cada linha JSONL.
6. Insere em lote no SQLite (transação única por arquivo) — sem consultas adicionais.
7. Move o arquivo para `processados/` após COMMIT confirmado.
8. Atualiza o registro de `transmission_import_log` com resultado final (`success` ou `failed`).
9. Ao final de cada ciclo, exclui de `processados/` arquivos com `mtime` anterior a `retention_days`.

> O importer é totalmente independente: não consulta `tracks`, não faz JOINs.
> Todos os campos necessários (incluindo `isrc`, `composer`, `publisher`) chegam
> prontos no JSONL, gerados pelo Engine a partir do payload do comando `ENQUEUE`.

### Grace period — a regra de segurança

O importer nunca toca um arquivo que pode ainda estar sendo escrito pelo Engine.

O importer deriva um glob a partir do template configurado, substituindo os
placeholders por `*`. Apenas arquivos cujo nome satisfaz o glob são processados —
qualquer outro arquivo no diretório é ignorado silenciosamente:

```go
// buildGlob converte o template em um glob para filepath.Match.
// "transmission_{date}_{hour}.jsonl" → "transmission_*_*.jsonl"
// O importer usa esse glob para filtrar apenas arquivos da raiz do diretório;
// o subdiretório "processados/" e qualquer outro arquivo são ignorados.
func buildGlob(template string) string {
    s := strings.ReplaceAll(template, "{date}", "*")
    s  = strings.ReplaceAll(s, "{hour}", "*")
    return s
}
```

**Regra:** um arquivo elegível (nome bate com o glob) só é importado quando:

```
time.Since(fileInfo.ModTime()) >= grace_period
```

`grace_period` padrão: **15 minutos**.

**Por que 15 minutos é suficiente:**
- Faixas de rádio têm duração típica de 3–5 minutos.
- O `mtime` do arquivo reflete a última linha escrita (última faixa que terminou).
- Se `mtime` está há 15 min no passado, nenhuma faixa em andamento pode ter
  `FinishedAt` nesse arquivo — pois o Engine só escreve após o término da faixa.
- O Engine fecha e reabre o arquivo a cada mudança de dia ou hora (`curDay != day || curHour != hour`).
- O importer varre apenas a raiz do diretório — o subdiretório `processados/` é ignorado.
  Após a rotação, o arquivo anterior não é mais tocado.

```go
func isEligible(fi os.FileInfo, now time.Time, grace time.Duration) bool {
    return now.Sub(fi.ModTime()) >= grace
}
```

### Protocolo de importação — zero perda de dados

```
Para cada arquivo JSONL elegível:
  1. INSERT em transmission_import_log (status=running, started_at=now, file_name=nome)
  2. Abrir e ler todas as linhas (bufio.Scanner) → records_total = nº de linhas lidas
  3. Parsear cada linha como LogEntry (linhas malformadas → log warning + skip)
  4. Preencher entry.ImportFileName = nome do arquivo (ex: "transmission_20260720_08.jsonl")
  5. Iniciar transação SQLite
  6. Para cada entrada válida:
       INSERT OR IGNORE INTO transmission_log (...) VALUES (...)
       (conflito em queue_item_id → no-op; não incrementa records_imported)
  7. COMMIT
  8. Se COMMIT OK:
       → os.MkdirAll(processados/) + os.Rename(arquivo → processados/arquivo)
       → UPDATE transmission_import_log SET status=success, finished_at=now,
                                            records_imported=N, error_message=''
  9. Se COMMIT falhar:
       → deixar arquivo na raiz (retry no próximo ciclo)
       → UPDATE transmission_import_log SET status=failed, finished_at=now,
                                            error_message=err.Error()
 10. Se os.Rename falhar após COMMIT:
       → UPDATE transmission_import_log SET status=failed, finished_at=now,
                                            records_imported=N, error_message=err.Error()
       (arquivo permanece na raiz; na próxima rodada INSERT OR IGNORE = no-op, Rename é re-tentado)
```

Nenhuma consulta ao banco é feita durante a importação. Todos os campos já chegam
completos no JSONL — o importer apenas lê, parseia, insere e move.

**Idempotência:** `queue_item_id` tem constraint `UNIQUE` em `transmission_log`.
Re-importar o mesmo arquivo não gera duplicatas — `INSERT OR IGNORE` é no-op para
linhas já existentes.

**Crash entre COMMIT e Rename:** arquivo permanece na raiz. Na próxima rodada, é
reimportado com todos os INSERTs sendo no-ops e então movido para `processados/`.
Sem perda, sem duplicata.

**Arquivo já em `processados/`:** o importer varre apenas a raiz do diretório.
Arquivos em `processados/` nunca são reprocessados — servem como histórico auditável.

### Limpeza automática de arquivos processados

Ao final de cada ciclo de poll, o importer executa uma varredura em `processados/`
e exclui os arquivos cujo `mtime` é anterior ao limite de retenção:

```go
func (imp *Importer) purgeProcessed(now time.Time) {
    retention := imp.cfg.RetentionDaysOrDefault() // mínimo 7
    cutoff    := now.AddDate(0, 0, -retention)

    entries, err := os.ReadDir(filepath.Join(imp.cfg.Dir, "processados"))
    if err != nil {
        return // diretório pode não existir ainda; silencioso
    }
    for _, e := range entries {
        if e.IsDir() {
            continue
        }
        info, err := e.Info()
        if err != nil {
            continue
        }
        if info.ModTime().Before(cutoff) {
            path := filepath.Join(imp.cfg.Dir, "processados", e.Name())
            if err := os.Remove(path); err != nil {
                imp.log.Warn("transmissionlog: purge failed", "file", e.Name(), "err", err)
            } else {
                imp.log.Info("transmissionlog: purged processed file", "file", e.Name())
            }
        }
    }
}
```

**Regras de retenção:**

| Configuração | Comportamento |
|---|---|
| `retention_days` não informado | Assume `30` dias |
| `retention_days < 7` | Elevado silenciosamente para `7` (mínimo absoluto) |
| `retention_days >= 7` | Aplicado como configurado |

O mínimo de 7 dias garante que arquivos recentes não sejam excluídos antes de uma
eventual auditoria ou reimportação manual em caso de incidente.

### Interface do importer

```go
// library/internal/fileimporter/importer.go

type LogStore interface {
    BulkInsert(ctx context.Context, entries []store.TransmissionLogEntry) error
}

type ImportLogStore interface {
    StartImport(ctx context.Context, fileName string) (id string, err error)
    FinishImport(ctx context.Context, id string, recordsTotal, recordsImported int) error
    FailImport(ctx context.Context, id string, recordsTotal int, errMsg string) error
}

type Settings interface {
    TransmissionLogDir(ctx context.Context) (string, error)
    TransmissionLogFileNameTemplate(ctx context.Context) (string, error)
    TransmissionLogPollInterval(ctx context.Context) (time.Duration, error)
    TransmissionLogGracePeriod(ctx context.Context) (time.Duration, error)
    TransmissionLogRetentionDays(ctx context.Context) (int, error)
}

// New não recebe TrackQuerier — o importer não consulta outras tabelas.
// Configurações são lidas do banco a cada ciclo via Settings.
func New(settings Settings, store LogStore, importLog ImportLogStore, log *slog.Logger) *Importer
func (imp *Importer) Run(ctx context.Context) error
```

### Exemplo de `BulkInsert`

```go
func (s *TransmissionLogStore) BulkInsert(ctx context.Context, entries []TransmissionLogEntry) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("transmission_log bulk_insert: begin: %w", err)
    }
    defer tx.Rollback()

    stmt, err := tx.PrepareContext(ctx, `
        INSERT OR IGNORE INTO transmission_log
            (id, queue_item_id, asset_id, title, artist, type,
             duration_ms, duration_played_ms, result, status,
             started_at, finished_at,
             break_id, break_title, break_role, break_position,
             isrc, composer, publisher, import_file_name)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
    `)
    if err != nil {
        return fmt.Errorf("transmission_log bulk_insert: prepare: %w", err)
    }
    defer stmt.Close()

    for _, e := range entries {
        if _, err := stmt.ExecContext(ctx,
            e.ID, e.QueueItemID, e.AssetID, e.Title, e.Artist, e.Type,
            e.DurationMS, e.DurationPlayedMS, e.Result, "FINISHED",
            e.StartedAt, e.FinishedAt,
            e.BreakID, e.BreakTitle, e.BreakRole, e.BreakPosition,
            e.ISRC, e.Composer, e.Publisher, e.ImportFileName,
        ); err != nil {
            return fmt.Errorf("transmission_log bulk_insert: exec: %w", err)
        }
    }
    return tx.Commit()
}
```

### Configuração no Library Service

As configurações do log de transmissão **não ficam em `config.yaml`** — são mantidas
na tabela `settings` do banco de dados (ver Migration 006). Isso permite alterá-las
em tempo de execução sem reiniciar o serviço.

```go
// library/internal/store/settings_store.go

// SettingsStore fornece acesso tipado à tabela key→value.
type SettingsStore struct{ db *sql.DB }

func (s *SettingsStore) Get(ctx context.Context, key string) (string, error)
func (s *SettingsStore) Set(ctx context.Context, key, value string) error

// Helpers tipados para as configurações do log de transmissão.
func (s *SettingsStore) TransmissionLogDir(ctx context.Context) (string, error)
func (s *SettingsStore) TransmissionLogFileNameTemplate(ctx context.Context) (string, error)
func (s *SettingsStore) TransmissionLogPollInterval(ctx context.Context) (time.Duration, error)
func (s *SettingsStore) TransmissionLogGracePeriod(ctx context.Context) (time.Duration, error)
func (s *SettingsStore) TransmissionLogRetentionDays(ctx context.Context) (int, error)
```

O importer carrega as configurações do banco **a cada ciclo de poll** — mudanças
feitas via API têm efeito sem restart.

**Regra de retenção:** `retention_days` com valor abaixo de `7` é rejeitado na
camada da API com erro `400 Bad Request`. O `SettingsStore` aplica a mesma validação
como segunda linha de defesa.

---

## Modelo de dados

### Migration 005 — transmission_log + campos ECAD em tracks

Arquivo: `library/internal/store/migrations/005_transmission_log.sql`

### Migration 006 — transmission_import_log (registro de tentativas de importação)

Arquivo: `library/internal/store/migrations/006_transmission_import_log.sql`

```sql
CREATE TABLE IF NOT EXISTS transmission_import_log (
    id              TEXT     PRIMARY KEY,
    file_name       TEXT     NOT NULL,
    started_at      DATETIME NOT NULL,
    finished_at     DATETIME,
    status          TEXT     NOT NULL DEFAULT 'running', -- running|success|failed
    records_total   INTEGER  NOT NULL DEFAULT 0,         -- linhas lidas no arquivo
    records_imported INTEGER NOT NULL DEFAULT 0,         -- INSERTs efetivados (excluindo OR IGNORE)
    error_message   TEXT     NOT NULL DEFAULT ''         -- preenchido apenas em status=failed
);

CREATE INDEX IF NOT EXISTS idx_import_log_started_at ON transmission_import_log(started_at);
CREATE INDEX IF NOT EXISTS idx_import_log_status     ON transmission_import_log(status);
```

> Cada tentativa de processar um arquivo gera uma linha nesta tabela, independentemente
> do resultado. Tentativas repetidas do mesmo arquivo (retry após falha) geram linhas
> distintas — o histórico completo de tentativas é preservado.

### Migration 007 — settings (key → value)

Arquivo: `library/internal/store/migrations/007_settings.sql`

```sql
CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Valores padrão das configurações do log de transmissão
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('transmission_log.dir',                '/var/radioflow/transmission-logs'),
    ('transmission_log.file_name_template', 'transmission_{date}_{hour}.jsonl'),
    ('transmission_log.poll_interval',      '5m'),
    ('transmission_log.grace_period',       '15m'),
    ('transmission_log.retention_days',     '30'),
    ('station.name',                        ''),
    ('station.cnpj',                        ''),
    ('station.frequency',                   ''),
    ('station.type',                        'FM'),
    ('station.city',                        ''),
    ('station.state',                       '');
```

> A tabela `settings` é genérica e extensível — qualquer futura configuração
> operacional pode ser adicionada com um novo par `key/value` sem nova migration.

---

```sql
-- Campos adicionais na tabela tracks para suporte ao ECAD
ALTER TABLE tracks ADD COLUMN isrc      TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN composer  TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN publisher TEXT NOT NULL DEFAULT '';

-- Log de transmissão
-- queue_item_id é UNIQUE: garante idempotência no BulkInsert (INSERT OR IGNORE)
CREATE TABLE IF NOT EXISTS transmission_log (
    id                 TEXT     PRIMARY KEY,
    queue_item_id      TEXT     NOT NULL DEFAULT '' UNIQUE,
    asset_id           TEXT     NOT NULL DEFAULT '',
    path               TEXT     NOT NULL DEFAULT '',
    title              TEXT     NOT NULL DEFAULT '',
    artist             TEXT     NOT NULL DEFAULT '',
    type               TEXT     NOT NULL DEFAULT '',   -- MUSIC|JINGLE|VINHETA|SPOT|CART
    isrc               TEXT     NOT NULL DEFAULT '',   -- enriquecido pelo importer
    composer           TEXT     NOT NULL DEFAULT '',   -- enriquecido pelo importer
    publisher          TEXT     NOT NULL DEFAULT '',   -- enriquecido pelo importer
    duration_ms        INTEGER  NOT NULL DEFAULT 0,
    duration_played_ms INTEGER  NOT NULL DEFAULT 0,
    result             TEXT     NOT NULL DEFAULT '',   -- finished|skipped|failed
    status             TEXT     NOT NULL DEFAULT 'FINISHED',
    started_at         DATETIME NOT NULL,
    finished_at        DATETIME,
    break_id           TEXT     NOT NULL DEFAULT '',
    break_title        TEXT     NOT NULL DEFAULT '',
    break_role         TEXT     NOT NULL DEFAULT '',   -- open|spot|close
    break_position     INTEGER  NOT NULL DEFAULT 0,
    import_file_name   TEXT     NOT NULL DEFAULT ''    -- nome do arquivo JSONL de origem
);

CREATE INDEX IF NOT EXISTS idx_transmission_log_started_at ON transmission_log(started_at);
CREATE INDEX IF NOT EXISTS idx_transmission_log_type       ON transmission_log(type);
CREATE INDEX IF NOT EXISTS idx_transmission_log_status     ON transmission_log(status);
CREATE INDEX IF NOT EXISTS idx_transmission_log_asset_id   ON transmission_log(asset_id);
```

---

## Estrutura de pacotes

### Playout Engine (mudanças)

```
playout/
  internal/
    transmissionlog/
      writer.go          ← LogWriter: subscriber do Event Bus, escrita JSONL
      writer_test.go
    config/
      config.go          ← adicionar TransmissionLogConfig (dir + file_name_template + enabled)
  cmd/playout-engine/
    main.go              ← instanciar e iniciar Writer se enabled=true
```

### Library Service (mudanças)

```
library/
  internal/
    store/
      migrations/
        005_transmission_log.sql       ← migration com ALTER TABLE + CREATE TABLE
        006_transmission_import_log.sql ← registro de tentativas de importação
        007_settings.sql               ← tabela settings com valores padrão
      transmission_log_store.go        ← TransmissionLogStore
      transmission_import_log_store.go ← TransmissionImportLogStore (Start/Finish/Fail)
      settings_store.go                ← SettingsStore (Get/Set + helpers tipados)
    fileimporter/
      importer.go                      ← goroutine de importação periódica
      importer_test.go
    api/
      handlers/
        transmission_log.go            ← handlers GET + exportações
        settings.go                    ← handlers GET/PUT /v1/settings
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
    ImportFileName   string // nome do arquivo JSONL que originou esta entrada
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

func (s *TransmissionLogStore) BulkInsert(ctx context.Context, entries []TransmissionLogEntry) error
func (s *TransmissionLogStore) List(ctx context.Context, q TransmissionLogQuery) ([]TransmissionLogEntry, int, error)
func (s *TransmissionLogStore) Summary(ctx context.Context, date time.Time) (TransmissionLogSummary, error)
func (s *TransmissionLogStore) ExportCSV(ctx context.Context, from, to time.Time, w io.Writer) error
func (s *TransmissionLogStore) ExportECAD(ctx context.Context, from, to time.Time, station config.StationConfig, w io.Writer) error
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

### Endpoints de settings

```
GET  /v1/settings                    → lista todas as chaves e valores
GET  /v1/settings/{key}              → retorna o valor de uma chave
PUT  /v1/settings/{key}              → atualiza o valor de uma chave
```

**Exemplo — leitura:**
```json
GET /v1/settings/transmission_log.retention_days
{
  "ok": true,
  "data": { "key": "transmission_log.retention_days", "value": "30", "updated_at": "2026-07-13T10:00:00Z" }
}
```

**Exemplo — atualização:**
```json
PUT /v1/settings/transmission_log.retention_days
Body: { "value": "60" }

200 → { "ok": true }
400 → { "ok": false, "error": "invalid_value", "message": "retention_days mínimo é 7" }
```

A validação de `retention_days >= 7` é aplicada no handler antes de chamar o store.

### Registro de rotas

```go
mux.HandleFunc("GET /v1/transmission-log",              handlers.ListTransmissionLog(s.tls))
mux.HandleFunc("GET /v1/transmission-log/export",       handlers.ExportTransmissionLog(s.tls))
mux.HandleFunc("GET /v1/transmission-log/export/ecad",  handlers.ExportECAD(s.tls, s.settings))
mux.HandleFunc("GET /v1/transmission-log/summary",      handlers.GetTransmissionLogSummary(s.tls))
mux.HandleFunc("GET /v1/transmission-log/imports",      handlers.ListImportLog(s.ils))
mux.HandleFunc("GET /v1/settings",                      handlers.ListSettings(s.settings))
mux.HandleFunc("GET /v1/settings/{key}",                handlers.GetSetting(s.settings))
mux.HandleFunc("PUT /v1/settings/{key}",                handlers.UpdateSetting(s.settings))
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

- Status: verde (FINISHED), amarelo (SKIPPED), vermelho (FAILED)
- Sem status PLAYING em tempo real — o log reflete entradas já importadas (~1h15min de atraso)
- Para ver a faixa atual, o operador usa a fila principal do player

**Rodapé:**
```
142 entradas · 5h 12m 00s    [ ↓ CSV ]  [ ↓ ECAD ]
```

O botão **ECAD** abre `GET /v1/transmission-log/export/ecad` com o mês filtrado.

---

## Fases de implementação

---

### Fase 1 — Extensão do protocolo ENQUEUE (Playout Engine)

**1.1 — Payload do comando ENQUEUE**
- Adicionar campos `isrc`, `composer`, `publisher` à struct do comando `ENQUEUE`
- Testes: comando desserializado com e sem esses campos (retrocompatível)

**1.2 — Propagação em `NowPlayingChangedPayload`**
- Adicionar `isrc`, `composer`, `publisher` em `events/types.go` → `NowPlayingChangedPayload`
- Testes: publicar evento com os campos; verificar que subscribers recebem os valores

**1.3 — Atualização do Player**
- Ao enfileirar via `POST /v1/queue/enqueue`, incluir `isrc`, `composer`, `publisher`
  lidos do Library Service junto com os demais metadados da faixa

---

### Fase 2 — LogWriter (Playout Engine)

**2.1 — Structs e tipos**
- Criar `playout/internal/transmissionlog/writer.go`
- Definir `LogEntry` (com todos os campos incluindo `isrc`, `composer`, `publisher`)
- Definir `pendingEntry` (estado em memória entre `NowPlayingChanged` e `ItemFinished`)

**2.2 — Processamento de eventos**
- Implementar `Writer.Run(ctx)` com subscriber no Event Bus (buffer 256)
- Lógica dos pending maps: `NowPlayingChanged` → insert; `ItemFinished` → pop + write
- Lógica dos cart pending maps: `CartStarted` → insert; `CartStopped` → pop + write
- Testes: publicar sequência de eventos → verificar linhas JSONL em `t.TempDir()`
- Testes: `ItemFinished` sem `NowPlayingChanged` correspondente → linha ignorada
- Testes: canal cheio não bloqueia outros subscribers do Bus

**2.3 — Rotação de arquivo e escrita**
- Implementar `buildFileName(template, date, hour)` com placeholders `{date}` e `{hour}`
- Implementar `openFile`, `closeFile`, rotação por hora (`curDay != day || curHour != hour`)
- Escrita com `O_APPEND|O_CREATE|O_WRONLY` + `Sync()` por linha
- Testes: entrada com `FinishedAt` em hora diferente → novo arquivo criado
- Testes: falha de I/O → `closeFile()` + próxima entrada reabre sem panic

**2.4 — Config e integração**
- Adicionar `TransmissionLogConfig` (`enabled`, `dir`, `file_name_template`) em `config.go`
- Instanciar e iniciar `Writer` em `main.go` somente se `enabled = true`
- Testes: shutdown limpo via context cancelado + `Sync()+Close()` no arquivo corrente

---

### Fase 3 — Migrations (Library Service)

**3.1 — Migration 005: `transmission_log` + campos ECAD em `tracks`**
- `ALTER TABLE tracks ADD COLUMN isrc/composer/publisher`
- `CREATE TABLE transmission_log` com `UNIQUE(queue_item_id)` e campo `import_file_name`
- Índices em `started_at`, `type`, `status`, `asset_id`
- Registrar em `db.go`

**3.2 — Migration 006: `transmission_import_log`**
- `CREATE TABLE transmission_import_log` (`id`, `file_name`, `started_at`, `finished_at`, `status`, `records_total`, `records_imported`, `error_message`)
- Índices em `started_at`, `status`
- Registrar em `db.go`

**3.3 — Migration 007: `settings`**
- `CREATE TABLE settings` (`key`, `value`, `updated_at`)
- `INSERT OR IGNORE` dos valores padrão para todas as chaves de `transmission_log.*` e `station.*`
- Registrar em `db.go`

---

### Fase 4 — Stores (Library Service)

**4.1 — `TransmissionLogStore`**
- `BulkInsert` com `INSERT OR IGNORE` e `import_file_name`
- `List` com filtros (`from`, `to`, `type`, `status`, `search`, `limit`, `offset`)
- `Summary` por data (total + by_type + total_played_ms)
- `ExportCSV` (writer streaming, sem carregar tudo em memória)
- `ExportECAD` (filtro automático MUSIC/JINGLE/VINHETA + header H + linhas D)
- Testes: BulkInsert idempotente, List com cada filtro, ExportECAD formato correto

**4.2 — `TransmissionImportLogStore`**
- `StartImport(fileName)` → INSERT com `status=running`, retorna `id`
- `FinishImport(id, recordsTotal, recordsImported)` → UPDATE `status=success`
- `FailImport(id, recordsTotal, errMsg)` → UPDATE `status=failed`
- `List(limit, offset)` → listagem paginada para o endpoint de histórico
- Testes: sequência start→finish, start→fail, múltiplos retries do mesmo arquivo

**4.3 — `SettingsStore`**
- `Get(key)` e `Set(key, value)` genéricos
- Helpers tipados: `TransmissionLogDir`, `TransmissionLogFileNameTemplate`, `TransmissionLogPollInterval`, `TransmissionLogGracePeriod`, `TransmissionLogRetentionDays`
- `RetentionDaysOrDefault()` — eleva valores `< 7` para `7`
- Helpers para `station.*` (usados em `ExportECAD`)
- Testes: Get/Set, fallback ao padrão quando chave não existe, RetentionDaysOrDefault

**4.4 — Scanner: extração de ISRC, composer, publisher**
- Extrair tags `TSRC` (ISRC), `TCOM` (composer), `TPUB` (publisher) via ffprobe
- Persistir em `tracks.isrc`, `tracks.composer`, `tracks.publisher`
- Testes: fixture com arquivo contendo as tags → verificar extração correta

**4.4.1 — Exposição nos endpoints existentes de tracks**
- Adicionar `ISRC`, `Composer`, `Publisher` à struct `Track` no store
- Atualizar mapeamento nos handlers `GET /v1/tracks`, `GET /v1/tracks/{id}` e `PATCH /v1/tracks/{id}` para incluir os novos campos na resposta e aceitar atualização via PATCH
- A partir desta etapa o Player consegue ler `isrc/composer/publisher` do Library Service e incluí-los no payload do `ENQUEUE` (prerequisito da Fase 1.3)
- Testes: `GET /v1/tracks/{id}` retorna `isrc`, `composer`, `publisher`; `PATCH` atualiza corretamente

---

### Fase 5 — File Importer (Library Service)

**5.1 — Filtro e elegibilidade**
- `buildGlob(template)` — converte template em glob (`transmission_*_*.jsonl`)
- Varredura da raiz do diretório — ignorar `processados/` e arquivos que não batem o glob
- `isEligible(fi, now, grace)` — `time.Since(mtime) >= gracePeriod`
- Testes: arquivo recente → não elegível; arquivo antigo → elegível; `processados/` → ignorado

**5.2 — Parse de JSONL**
- Leitura linha a linha com `bufio.Scanner`
- Parse de cada linha como `LogEntry`; linhas malformadas → log warning + skip
- Preenchimento de `entry.ImportFileName` com o nome do arquivo
- Testes: arquivo com linhas válidas, inválidas e mistas

**5.3 — Protocolo de importação**
- `StartImport` → BulkInsert (transação) → Rename para `processados/` → `FinishImport`
- Em falha no COMMIT → `FailImport` + arquivo permanece na raiz
- Em falha no Rename após COMMIT → `FailImport` + arquivo permanece na raiz (retry no próximo ciclo: INSERT OR IGNORE = no-op)
- Testes: importação completa, falha no COMMIT, falha no Rename, retry idempotente

**5.4 — Limpeza de `processados/`**
- `purgeProcessed(now)` — lista `processados/`, exclui arquivos com `mtime < cutoff`
- Executado ao final de cada ciclo de poll
- `cutoff = now - RetentionDaysOrDefault() * 24h`
- Testes: arquivo expirado → excluído; arquivo dentro da retenção → preservado

**5.5 — Loop principal e integração**
- `Run(ctx)` com ticker de `PollInterval` (lido do `SettingsStore` a cada ciclo)
- Iniciar importer em `main.go` somente se `transmission_log.dir` configurado no banco
- Testes: shutdown limpo via context; ciclo completo end-to-end com arquivos reais

---

### Fase 6 — API: Log de Transmissão (Library Service)

**6.1 — `GET /v1/transmission-log`**
- Handler com query params: `from`, `to`, `type`, `status`, `q`, `limit`, `offset`
- Resposta paginada com `total`
- Testes: filtros individuais e combinados

**6.2 — `GET /v1/transmission-log/summary`**
- Handler com query param `date`
- Resposta: `total`, `by_type`, `total_played_ms`
- Testes: dia com e sem registros

**6.3 — `GET /v1/transmission-log/export`**
- Streaming CSV com `Content-Disposition`
- Todas as colunas incluindo `import_file_name`
- Testes: CSV gerado com cabeçalho e linhas corretas

**6.4 — `GET /v1/transmission-log/export/ecad`**
- Streaming CSV no formato ECAD (linha H + linhas D)
- Filtro automático: MUSIC/JINGLE/VINHETA, FINISHED, `duration_played_ms > 0`
- Dados da emissora lidos do `SettingsStore` (`station.*`)
- Testes: formato H/D, filtro de tipos, ordenação por `started_at`

**6.5 — `GET /v1/transmission-log/imports`**
- Listagem paginada do `transmission_import_log`
- Query params: `status`, `limit`, `offset`
- Testes: listagem com e sem filtro de status

**6.6 — Registro de rotas e injeção**
- Registrar todas as rotas em `server.go`
- Injetar `TransmissionLogStore`, `TransmissionImportLogStore` e `SettingsStore` em `main.go`

---

### Fase 7 — API: Settings (Library Service)

**7.1 — `GET /v1/settings`**
- Lista todas as chaves e valores com `updated_at`
- Testes: retorna todas as chaves pré-populadas pela migration

**7.2 — `GET /v1/settings/{key}`**
- Retorna valor de uma chave; `404` se chave não existir
- Testes: chave existente, chave inexistente

**7.3 — `PUT /v1/settings/{key}`**
- Atualiza valor; valida `retention_days >= 7` antes de salvar
- `400` se valor inválido; `404` se chave desconhecida
- Testes: atualização válida, `retention_days = 3` → 400, chave inexistente → 404

**7.4 — Registro de rotas**
- Registrar em `server.go`; injetar `SettingsStore`

---

### Fase 8 — Player UI

**8.1 — Aba "Histórico"**
- Nova aba no drawer após "Rotação"
- Tabela com colunas: início, título, artista, tipo, duração, status
- Ícone de tipo (padrão da fila); cor de status (verde/amarelo/vermelho)

**8.2 — Filtros e paginação**
- Barra: data, tipo, busca por título/artista
- Paginação ou scroll infinito
- Rodapé: total de entradas + duração total reproduzida

**8.3 — Exportação**
- Botão `↓ CSV` → `GET /v1/transmission-log/export` com filtros ativos
- Botão `↓ ECAD` → `GET /v1/transmission-log/export/ecad` com o mês do filtro ativo

**8.4 — Histórico de importações**
- Sub-seção ou modal mostrando as últimas tentativas de importação
- Colunas: arquivo, início, status, registros importados, erro (se houver)

---

## Pontos de atenção

### `ItemFinished` sem `NowPlayingChanged` correspondente

Ocorre quando o Engine reinicia durante a reprodução de uma faixa. O `pending` map
não tem a entrada. A linha é ignorada — sem metadados suficientes para o ECAD.
O Engine emitirá `NowPlayingChanged` para a próxima faixa normalmente.

### `isrc`, `composer`, `publisher` no JSONL

Esses campos chegam ao Engine via payload do comando `ENQUEUE` (enviado pelo Player,
que os lê do Library Service antes de enfileirar). O Engine os propaga no
`NowPlayingChangedPayload` e o LogWriter os captura ao montar a `LogEntry`.

O importer insere esses valores diretamente no SQLite — sem consultas adicionais.
O snapshot ECAD é fixado no momento da execução (valores vigentes quando o Player
enfileirou a faixa), não no momento da exportação.

### Cart entries e `queue_item_id`

Carts são reproduzidos fora da fila principal (sem `queue_item_id`). O `cart_id`
é usado como substituto no campo `queue_item_id` do LogEntry e da tabela
`transmission_log`. O UNIQUE constraint garante idempotência normalmente.

### Engine e Library Service no mesmo host

Para a implantação atual (processo local), ambos acessam o mesmo diretório via path
local. Para deploy em containers separados, o diretório deve ser um volume
compartilhado (bind mount ou NFS). Isso é uma restrição conhecida da Abordagem B
e deve constar na documentação de operações.

---

## Definição de pronto

### Playout Engine
- `go test ./...` e `go vet ./...` sem erros
- `go test -race ./...` sem data races
- `isrc`, `composer`, `publisher` aceitos no ENQUEUE e propagados em `NowPlayingChangedPayload`
- LogWriter subscreve o Event Bus sem alterar comportamento dos outros subscribers
- Arquivos JSONL criados na raiz do diretório com nome derivado do template configurado
- Rotação correta ao mudar de hora — arquivo anterior fechado com Sync+Close
- Canal cheio não bloqueia o pipeline de áudio

### Library Service
- `go test ./...` e `go vet ./...` sem erros
- `go test -race ./...` sem data races
- Migrations 005, 006 e 007 aplicadas corretamente em banco limpo e em banco existente
- Scanner extrai `isrc`, `composer`, `publisher` via ffprobe e persiste em `tracks`
- `BulkInsert` idempotente: re-importar o mesmo arquivo não gera duplicatas (`UNIQUE queue_item_id`)
- `import_file_name` preenchido em todas as linhas de `transmission_log`
- Toda tentativa de importação registrada em `transmission_import_log` (success ou failed)
- Arquivo movido para `processados/` somente após COMMIT confirmado
- Importer respeita grace period e ignora arquivo da hora corrente
- `purgeProcessed()` exclui apenas arquivos além do `retention_days` (mínimo 7)
- Configurações lidas do banco a cada ciclo — mudanças via API têm efeito sem restart
- `GET /v1/transmission-log` filtra por data, tipo, status e busca
- `GET /v1/transmission-log/summary` retorna totais por tipo e duração
- `GET /v1/transmission-log/export` gera CSV completo com streaming
- `GET /v1/transmission-log/export/ecad` gera formato ECAD (H + linhas D), apenas MUSIC/JINGLE/VINHETA FINISHED
- `GET /v1/transmission-log/imports` lista histórico de tentativas de importação
- `PUT /v1/settings/transmission_log.retention_days` com valor `< 7` retorna 400
- Spots auditáveis via filtro `type=SPOT` nos endpoints de log e exportação

### Player UI
- Aba "Histórico" exibe log com filtros e paginação
- Exportação CSV e ECAD funcionais com filtros ativos
- Histórico de importações visível (últimas tentativas com status e erro)
