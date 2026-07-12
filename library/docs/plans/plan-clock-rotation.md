# Plano — Rotação Musical por Formato (Clock)

## Visão geral

Implementar o sistema de **clock rotation** no RadioFlow: categorias musicais, clocks
com slots ordenados, grade horária 24×7, regras de separação e um gerador automático
de playlist. Com isso, a emissora programa o fim de semana (ou qualquer período) de
forma totalmente automatizada — sem montar playlist estática manualmente.

**Branch:** `feature/clock-rotation`
**Base:** `main`

---

## Contexto atual

O Library Service já possui:
- `tracks` com campo `category TEXT` (texto livre, legado)
- Stores para tracks, playlists, breaks, hotkeys
- Padrão de migração em `internal/store/migrations/*.sql` + `db.go`
- Padrão de handler: funções retornando `http.HandlerFunc`, registradas em `server.go`
- API envelope: `{"ok": true, "data": ...}` / `{"ok": false, "error": "...", "message": "..."}`

O Playout Engine já expõe `POST /v1/queue/enqueue` para receber faixas. O Player UI
consome o Library Service para montar o que é enfileirado no Engine.

O campo `category` existente na tabela `tracks` **não será removido** — continua
funcionando como filtro de busca. As categorias de rotação são um sistema separado,
com associação M:N.

---

## Modelo de dados

### Migration 004 — clock_rotation

Arquivo: `internal/store/migrations/004_clock_rotation.sql`

```sql
-- Categorias de rotação
CREATE TABLE IF NOT EXISTS categories (
    id          TEXT    PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE,
    description TEXT    NOT NULL DEFAULT '',
    color       TEXT    NOT NULL DEFAULT '#888888',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Associação M:N faixa ↔ categoria
CREATE TABLE IF NOT EXISTS track_categories (
    track_id    TEXT NOT NULL REFERENCES tracks(id)     ON DELETE CASCADE,
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (track_id, category_id)
);
CREATE INDEX IF NOT EXISTS idx_track_categories_category ON track_categories(category_id);

-- Clocks (templates de 60 min)
CREATE TABLE IF NOT EXISTS clocks (
    id         TEXT    PRIMARY KEY,
    name       TEXT    NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Slots ordenados dentro de um clock
CREATE TABLE IF NOT EXISTS clock_slots (
    id               TEXT    PRIMARY KEY,
    clock_id         TEXT    NOT NULL REFERENCES clocks(id) ON DELETE CASCADE,
    position         INTEGER NOT NULL,
    slot_type        TEXT    NOT NULL CHECK(slot_type IN ('CATEGORY','JINGLE','SPOT','VINHETA','HORA_CERTA','FIXED')),
    category_id      TEXT    REFERENCES categories(id) ON DELETE SET NULL,
    fixed_track_id   TEXT    REFERENCES tracks(id)     ON DELETE SET NULL,
    duration_hint_ms INTEGER NOT NULL DEFAULT 0,
    UNIQUE(clock_id, position)
);
CREATE INDEX IF NOT EXISTS idx_clock_slots_clock ON clock_slots(clock_id);

-- Grade 24x7: qual clock toca em cada hora de cada dia
-- weekday: 0=domingo .. 6=sábado
CREATE TABLE IF NOT EXISTS clock_schedule (
    weekday  INTEGER NOT NULL CHECK(weekday BETWEEN 0 AND 6),
    hour     INTEGER NOT NULL CHECK(hour    BETWEEN 0 AND 23),
    clock_id TEXT    REFERENCES clocks(id) ON DELETE SET NULL,
    PRIMARY KEY (weekday, hour)
);

-- Regras de separação mínima
CREATE TABLE IF NOT EXISTS separation_rules (
    id              TEXT    PRIMARY KEY,
    field           TEXT    NOT NULL CHECK(field IN ('artist','title','category','album')),
    min_sep_minutes INTEGER NOT NULL DEFAULT 60
);

-- Log append-only do que tocou em cada slot (base para separação entre sessões)
CREATE TABLE IF NOT EXISTS rotation_log (
    id          TEXT    PRIMARY KEY,
    track_id    TEXT    NOT NULL,
    played_at   DATETIME NOT NULL,
    clock_id    TEXT    NOT NULL DEFAULT '',
    slot_type   TEXT    NOT NULL DEFAULT '',
    category_id TEXT    NOT NULL DEFAULT '',
    artist      TEXT    NOT NULL DEFAULT '',
    title       TEXT    NOT NULL DEFAULT '',
    album       TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_rotation_log_played_at ON rotation_log(played_at);
CREATE INDEX IF NOT EXISTS idx_rotation_log_track_id  ON rotation_log(track_id);
CREATE INDEX IF NOT EXISTS idx_rotation_log_artist    ON rotation_log(artist);
```

---

## Estrutura de pacotes novos

```
library/
  internal/
    store/
      migrations/
        004_clock_rotation.sql          ← nova migração
      category_store.go                 ← CategoryStore (CRUD + associações)
      clock_store.go                    ← ClockStore (clocks + slots + grade)
      separation_store.go               ← SeparationRuleStore
      rotation_log_store.go             ← RotationLogStore (append + query)
    scheduler/
      generator.go                      ← algoritmo de geração de playlist
      generator_test.go
    api/
      handlers/
        categories.go                   ← handlers de /v1/categories
        clocks.go                       ← handlers de /v1/clocks
        schedule.go                     ← handlers de /v1/schedule/*
        rotation_log.go                 ← handler de /v1/rotation-log
```

---

## API — endpoints novos

### Categorias

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET`    | `/v1/categories`                          | Lista categorias com contagem de faixas |
| `POST`   | `/v1/categories`                          | Cria categoria |
| `GET`    | `/v1/categories/{id}`                     | Retorna categoria + contagem |
| `PUT`    | `/v1/categories/{id}`                     | Atualiza name/description/color |
| `DELETE` | `/v1/categories/{id}`                     | Remove (erro se referenciada por clock_slot) |
| `GET`    | `/v1/categories/{id}/tracks`              | Lista faixas na categoria (paginado) |
| `POST`   | `/v1/categories/{id}/tracks`              | Adiciona faixa(s) à categoria |
| `DELETE` | `/v1/categories/{id}/tracks/{track_id}`   | Remove faixa da categoria |
| `PUT`    | `/v1/tracks/{id}/categories`             | Substitui todas as categorias de uma faixa |

### Clocks

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET`    | `/v1/clocks`                              | Lista clocks |
| `POST`   | `/v1/clocks`                              | Cria clock |
| `GET`    | `/v1/clocks/{id}`                         | Retorna clock com slots |
| `PUT`    | `/v1/clocks/{id}`                         | Atualiza nome |
| `DELETE` | `/v1/clocks/{id}`                         | Remove (erro se na grade) |
| `POST`   | `/v1/clocks/{id}/slots`                   | Adiciona slot (appenda ao final) |
| `PUT`    | `/v1/clocks/{id}/slots/{slot_id}`         | Atualiza slot |
| `DELETE` | `/v1/clocks/{id}/slots/{slot_id}`         | Remove slot |
| `PUT`    | `/v1/clocks/{id}/slots/reorder`           | Reordena slots (array de IDs) |

### Grade de clocks

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET` | `/v1/schedule/clock-grid`  | Retorna matriz 7×24 (weekday × hour → clock_id, clock_name) |
| `PUT` | `/v1/schedule/clock-grid`  | Atualiza uma ou mais células `[{weekday, hour, clock_id}]` |

### Regras de separação

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET`    | `/v1/schedule/separation-rules`       | Lista regras |
| `POST`   | `/v1/schedule/separation-rules`       | Cria regra |
| `PUT`    | `/v1/schedule/separation-rules/{id}`  | Atualiza |
| `DELETE` | `/v1/schedule/separation-rules/{id}`  | Remove |

### Gerador e log

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `POST` | `/v1/schedule/generate`           | Gera playlist para as próximas N horas |
| `POST` | `/v1/rotation-log`                | Registra faixa executada (chamado pelo Player após enfileirar) |
| `GET`  | `/v1/rotation-log?date=YYYY-MM-DD`| Consulta log de rotação do dia |

---

## Contratos JSON

### `POST /v1/schedule/generate` — request

```json
{
  "from": "2026-07-19T08:00:00",
  "hours": 4
}
```

- `from`: ISO 8601 local (sem timezone). O servidor usa o horário do campo para
  determinar weekday e hour na grade. Se omitido, usa `now`.
- `hours`: número de horas a gerar. Máximo: 24. Default: 1.

### `POST /v1/schedule/generate` — response

```json
{
  "ok": true,
  "data": {
    "from": "2026-07-19T08:00:00",
    "to":   "2026-07-19T12:00:00",
    "hours": 4,
    "items": [
      {
        "hour":          8,
        "position":      1,
        "slot_id":       "01JF...",
        "slot_type":     "CATEGORY",
        "clock_id":      "01JE...",
        "clock_name":    "Manhã Adulto",
        "category_id":   "01JD...",
        "category_name": "MPB Clássica",
        "track": {
          "id":          "01JA...",
          "path":        "/audio/musicas/mpb/Elis Regina - Como Nossos Pais.mp3",
          "title":       "Como Nossos Pais",
          "artist":      "Elis Regina",
          "album":       "Falso Brilhante",
          "duration_ms": 243000
        }
      }
    ],
    "warnings": [
      "Hora 9, slot 3 (Rock Nacional): separação de artista relaxada — apenas 2 tracks disponíveis na categoria"
    ]
  }
}
```

### `PUT /v1/schedule/clock-grid` — request

```json
[
  {"weekday": 6, "hour": 8,  "clock_id": "01JE..."},
  {"weekday": 6, "hour": 9,  "clock_id": "01JE..."},
  {"weekday": 0, "hour": 22, "clock_id": null}
]
```

`clock_id: null` limpa a célula (sem clock programado nessa hora).

### `GET /v1/schedule/clock-grid` — response

```json
{
  "ok": true,
  "data": {
    "grid": [
      {
        "weekday": 0,
        "hour":    8,
        "clock_id":   "01JE...",
        "clock_name": "Manhã Adulto"
      }
    ]
  }
}
```

Retorna apenas células preenchidas. O Player UI preenche as vazias como "(sem clock)".

---

## Algoritmo do gerador (`internal/scheduler/generator`)

```
Input:
  from time.Time        — hora de início
  hours int             — quantas horas gerar

Output:
  []GeneratedItem       — itens resolvidos (um por slot)
  []string              — warnings

Para cada hora H em [from, from+hours):
  weekday, hour = from.Weekday(), H.Hour()
  clock = clock_schedule[weekday][hour]
  if clock == nil: skipa hora, emite warning

  Para cada slot S em clock.slots (ordenado por position):
    if slot_type == FIXED:
      item = {track: tracks[S.fixed_track_id]}
      continua

    if slot_type in {JINGLE, SPOT, VINHETA, HORA_CERTA}:
      candidates = tracks WHERE type == slot_type
    elif slot_type == CATEGORY:
      if S.category_id == nil: emite warning, pula
      candidates = tracks WHERE category == S.category_id (via track_categories)

    if len(candidates) == 0:
      emite warning "sem faixas na categoria X", pula slot

    Fase 1 — filtro estrito (todas as regras de separação ativas):
      filtered = candidates
      Para cada regra R em separation_rules:
        filtered = filtered WHERE NOT (field R do track aparece no rotation_log
                                       nas últimas R.min_sep_minutes minutos)
                                AND NOT (gerado nesta sessão nas últimas R.min_sep_minutes)

    Fase 2 — fallback parcial (se filtered vazio):
      relaxa a regra com menor min_sep_minutes (a menos crítica)
      refaz o filtro sem ela
      emite warning de relaxamento

    Fase 3 — fallback total (se ainda vazio):
      usa a faixa da categoria com played_at mais antiga no rotation_log
      (ou aleatória se nenhuma foi tocada antes)
      emite warning "separação ignorada"

    Escolha: random entre os candidatos filtrados
    Registra a escolha em memória (para respeitar separação dentro da sessão)
    Appenda item à lista de saída
```

### Interface pública do gerador

```go
package scheduler

type GeneratedItem struct {
    Hour        int
    Position    int
    SlotID      string
    SlotType    string
    ClockID     string
    ClockName   string
    CategoryID  string
    CategoryName string
    Track       TrackRef
}

type TrackRef struct {
    ID         string
    Path       string
    Title      string
    Artist     string
    Album      string
    DurationMS int64
}

type Generator struct {
    clocks    ClockQuerier
    tracks    TrackQuerier
    rotLog    RotationLogQuerier
    sepRules  SeparationRuleQuerier
}

func New(clocks ClockQuerier, tracks TrackQuerier, rotLog RotationLogQuerier, sepRules SeparationRuleQuerier) *Generator

func (g *Generator) Generate(ctx context.Context, from time.Time, hours int) ([]GeneratedItem, []string, error)
```

Todas as dependências são interfaces — testável sem banco.

---

## Stores — interfaces e tipos principais

### CategoryStore

```go
type Category struct {
    ID          string
    Name        string
    Description string
    Color       string
    TrackCount  int
    CreatedAt   time.Time
}

type CategoryStore struct { db *sql.DB }

func (s *CategoryStore) List(ctx context.Context) ([]Category, error)
func (s *CategoryStore) Create(ctx context.Context, name, description, color string) (Category, error)
func (s *CategoryStore) Get(ctx context.Context, id string) (Category, error)
func (s *CategoryStore) Update(ctx context.Context, id, name, description, color string) error
func (s *CategoryStore) Delete(ctx context.Context, id string) error  // erro se referenciada
func (s *CategoryStore) ListTracks(ctx context.Context, categoryID string, limit, offset int) ([]Track, error)
func (s *CategoryStore) AddTrack(ctx context.Context, categoryID, trackID string) error
func (s *CategoryStore) RemoveTrack(ctx context.Context, categoryID, trackID string) error
func (s *CategoryStore) SetTrackCategories(ctx context.Context, trackID string, categoryIDs []string) error
func (s *CategoryStore) ListByTrack(ctx context.Context, trackID string) ([]Category, error)
```

### ClockStore

```go
type Clock struct {
    ID        string
    Name      string
    Slots     []ClockSlot
    CreatedAt time.Time
}

type ClockSlot struct {
    ID              string
    ClockID         string
    Position        int
    SlotType        string
    CategoryID      string
    CategoryName    string
    FixedTrackID    string
    DurationHintMS  int64
}

type ScheduleCell struct {
    Weekday   int
    Hour      int
    ClockID   string
    ClockName string
}

type ClockStore struct { db *sql.DB }

func (s *ClockStore) List(ctx context.Context) ([]Clock, error)
func (s *ClockStore) Create(ctx context.Context, name string) (Clock, error)
func (s *ClockStore) Get(ctx context.Context, id string) (Clock, error)
func (s *ClockStore) Update(ctx context.Context, id, name string) error
func (s *ClockStore) Delete(ctx context.Context, id string) error  // erro se na grade
func (s *ClockStore) AddSlot(ctx context.Context, clockID string, slot ClockSlot) (ClockSlot, error)
func (s *ClockStore) UpdateSlot(ctx context.Context, slotID string, slot ClockSlot) error
func (s *ClockStore) DeleteSlot(ctx context.Context, slotID string) error
func (s *ClockStore) ReorderSlots(ctx context.Context, clockID string, orderedSlotIDs []string) error
func (s *ClockStore) GetGrid(ctx context.Context) ([]ScheduleCell, error)
func (s *ClockStore) SetGridCells(ctx context.Context, cells []ScheduleCell) error
func (s *ClockStore) GetClockForHour(ctx context.Context, weekday, hour int) (*Clock, error)
```

### SeparationRuleStore

```go
type SeparationRule struct {
    ID           string
    Field        string  // artist | title | category | album
    MinSepMinutes int
}

type SeparationRuleStore struct { db *sql.DB }

func (s *SeparationRuleStore) List(ctx context.Context) ([]SeparationRule, error)
func (s *SeparationRuleStore) Create(ctx context.Context, field string, minSepMinutes int) (SeparationRule, error)
func (s *SeparationRuleStore) Update(ctx context.Context, id string, field string, minSepMinutes int) error
func (s *SeparationRuleStore) Delete(ctx context.Context, id string) error
```

### RotationLogStore

```go
type RotationLogEntry struct {
    ID         string
    TrackID    string
    PlayedAt   time.Time
    ClockID    string
    SlotType   string
    CategoryID string
    Artist     string
    Title      string
    Album      string
}

type RotationLogStore struct { db *sql.DB }

func (s *RotationLogStore) Append(ctx context.Context, entry RotationLogEntry) error
func (s *RotationLogStore) ListByDate(ctx context.Context, date time.Time) ([]RotationLogEntry, error)
func (s *RotationLogStore) RecentByField(ctx context.Context, field, value string, since time.Time) ([]RotationLogEntry, error)
```

---

## Player UI — novos painéis

A UI é adicionada ao `player/player.html`. O ponto de entrada é um novo item no
menu lateral ou uma 4ª aba no drawer, conforme for mais natural no layout existente.

### Estrutura de painéis no drawer (proposta)

```
[ Playlists ]  [ Breaks ]  [ Botoneira ]  [ Rotação ]   ← 4ª aba nova
```

A aba Rotação contém sub-navegação interna:

```
[ Categorias ]  [ Clocks ]  [ Grade ]  [ Regras ]  [ Gerar ]
```

### Painel Categorias

- Lista de categorias com cor, nome e contagem de faixas.
- Botão "Nova categoria" → formulário inline (nome, cor com color picker, descrição).
- Clicar em categoria → expande e mostra faixas associadas (paginado, busca por título/artista).
- Cada faixa tem botão "Remover da categoria".
- Botão "Adicionar faixas" → abre busca de tracks (reutiliza a busca avançada existente), faixas selecionadas são adicionadas à categoria.
- Botão de excluir categoria (com confirmação; bloqueado se referenciada em slot).

### Painel Clocks

- Lista de clocks com nome e contagem de slots.
- Botão "Novo clock".
- Clicar em clock → abre editor de slots (lista ordenada por position).
- Cada slot mostra: posição, tipo (badge colorido), categoria/tipo de áudio.
- Botões: adicionar slot (formulário com tipo + categoria ou tipo fixo), remover slot, reordenar (drag-and-drop ou setas cima/baixo).

### Painel Grade

- Tabela 7 colunas (dias da semana) × 24 linhas (horas).
- Cada célula: nome do clock selecionado ou "(vazio)" em cinza.
- Clicar em célula → dropdown para selecionar o clock.
- Botão "Copiar linha" para aplicar o mesmo clock a todas as células de um dia.
- Células sem clock programado ficam com fundo mais escuro.

### Painel Regras

- Lista simples: campo + separação mínima (minutos).
- Botão "Nova regra" → select de field + input numérico.
- Botão excluir por linha.

### Painel Gerar

```
De:    [ 2026-07-19  ]  [ 08:00 ]
Horas: [ 4           ]

[ Gerar Playlist ]

─────────────────────────────────────────────────────
  08:00  |  Manhã Adulto  |  Slot 1 (MPB Clássica)
         |  Elis Regina — Como Nossos Pais  (4:03)
  08:04  |  Manhã Adulto  |  Slot 2 (Vinheta)
         |  Vinheta Manhã 01  (0:28)
  ...

  Avisos:
  ⚠ Hora 9, slot 3: separação de artista relaxada (poucos tracks em Rock Nacional)
─────────────────────────────────────────────────────

[ Enfileirar no Player ]
```

- "Gerar Playlist": chama `POST /v1/schedule/generate`, exibe resultado.
- "Enfileirar no Player": itera sobre os items gerados e chama `POST /v1/queue/enqueue`
  no Playout Engine para cada faixa.
- Após enfileirar, chama `POST /v1/rotation-log` para cada item registrando que foi
  programado (para que a próxima geração respeite a separação).

---

## Fases de implementação

### Fase 1 — Migração e stores (Library Service)

1. Criar `internal/store/migrations/004_clock_rotation.sql`.
2. Registrar migration 004 em `db.go` (padrão idêntico ao 003_hotkeys).
3. Implementar `category_store.go` com todos os métodos.
4. Implementar `clock_store.go` com todos os métodos.
5. Implementar `separation_store.go`.
6. Implementar `rotation_log_store.go`.
7. Testes de store com banco `:memory:`.

### Fase 2 — Gerador (Library Service)

1. Definir interfaces `ClockQuerier`, `TrackQuerier`, `RotationLogQuerier`, `SeparationRuleQuerier` no pacote `scheduler`.
2. Implementar `Generator.Generate()` com as 3 fases de fallback.
3. Testes unitários do gerador com mocks das interfaces:
   - Geração simples (catálogo amplo, sem separação).
   - Separação por artista respeitada.
   - Fallback parcial (relaxa regra menos crítica).
   - Fallback total (ignora separação, menos recente).
   - Clock ausente na grade (warning, skip da hora).
   - Categoria vazia (warning, skip do slot).

### Fase 3 — Handlers e rotas (Library Service)

1. Implementar `handlers/categories.go` (9 handlers).
2. Implementar `handlers/clocks.go` (9 handlers).
3. Implementar `handlers/schedule.go` (generate + grid + separation rules).
4. Implementar `handlers/rotation_log.go`.
5. Definir interfaces `CategoryStore`, `ClockStore`, `SeparationRuleStore`, `RotationLogStore`, `SchedulerService` no pacote `handlers`.
6. Registrar todas as rotas em `server.go`.
7. Injetar stores e generator em `main.go`.
8. Testes de handlers com `httptest.NewRecorder`.

### Fase 4 — Player UI (player.html)

1. Adicionar aba "Rotação" ao drawer.
2. Implementar sub-navegação interna (Categorias / Clocks / Grade / Regras / Gerar).
3. Painel Categorias: CRUD + associação de faixas.
4. Painel Clocks: CRUD + editor de slots com reordenação.
5. Painel Grade: tabela 7×24 clicável com dropdown de clock.
6. Painel Regras: CRUD simples.
7. Painel Gerar: formulário, exibição de resultado, botão de enfileirar + log de rotação.

---

## Pontos de atenção

### Dependência no log de rotação
O gerador é mais preciso com o log preenchido. Na primeira semana, o histórico estará
vazio e as regras de separação só funcionam dentro da sessão (memória do gerador). Isso
é aceitável — melhora progressivamente à medida que o log cresce.

### Clock sem faixas suficientes na categoria
Se uma categoria tem poucas faixas (ex.: 5 tracks, separação mínima de 2h, clock
gera 4 slots da categoria/hora), o gerador vai relaxar ou ignorar regras. O painel Gerar
exibe warnings. O operador deve ou adicionar mais faixas à categoria ou reduzir a
separação mínima.

### Hora sem clock programado na grade
O gerador pula a hora e emite warning. O Player UI destaca as células vazias na grade
com cor de alerta para facilitar o preenchimento.

### Reordenação de slots
Usa o mesmo padrão de `ReorderPlaylistItems`: recebe array de IDs na nova ordem,
atualiza `position` em transação.

### Não registrar no rotation_log dentro do gerador
O gerador apenas retorna a playlist sugerida — não persiste nada. O Player UI decide
o que enfileirar e depois chama `POST /v1/rotation-log` para cada item de fato
programado. Isso evita poluir o log com gerações descartadas.

### ULID para todos os IDs
Seguir o padrão já usado no projeto (`github.com/oklog/ulid/v2`).

---

## Definição de pronto

- `go test ./...` passa sem erros.
- `go vet ./...` sem avisos.
- `go test -race ./...` sem data races.
- Nenhum package cycle novo.
- Gerador testado com cenários de fallback.
- Grade 7×24 retornada e atualizada corretamente.
- Player UI consegue gerar playlist para as próximas 4h e enfileirar no Engine.
- Warnings de slots sem faixa disponível são exibidos ao operador.
