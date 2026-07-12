# Radio Library Service

Serviço HTTP de catálogo de áudio para o sistema de automação de rádio.
Fornece busca de faixas, playlists, blocos comerciais, botoneira e **rotação musical automática** consumidos pelo [RadioFlow Player](../player/).

---

## Índice

- [Visão geral](#visão-geral)
- [Porta padrão](#porta-padrão)
- [API REST](#api-rest)
  - [Health](#health)
  - [Faixas](#faixas)
  - [Playlists](#playlists)
  - [Blocos comerciais (Breaks)](#blocos-comerciais-breaks)
  - [Botoneira (Hotkeys)](#botoneira-hotkeys)
  - [Rotação Musical](#rotação-musical)
    - [Categorias](#categorias)
    - [Clocks](#clocks)
    - [Grade de clocks](#grade-de-clocks)
    - [Regras de separação](#regras-de-separação)
    - [Gerador de playlist](#gerador-de-playlist)
    - [Log de rotação](#log-de-rotação)
- [Modelo de dados — Rotação](#modelo-de-dados--rotação)
- [Algoritmo do gerador](#algoritmo-do-gerador)
- [Contratos de resposta](#contratos-de-resposta)
- [Integração com o Player](#integração-com-o-player)

---

## Visão geral

O Radio Library Service é um serviço independente responsável por indexar e servir o acervo de áudio da rádio. O RadioFlow Player consulta este serviço para popular a Biblioteca (drawer lateral), enfileirar conteúdo no RadioCore e **gerar programação automática** via clock rotation.

---

## Porta padrão

```
http://127.0.0.1:8081
```

Configurável no player via query string `?lib=http://<host>:<port>`.

---

## API REST

### Health

```
GET /v1/health
```

Verifica disponibilidade do serviço. Usado pelo player para indicar se a Biblioteca está online.

**Resposta:**
```json
{ "status": "ok" }
```

---

### Faixas

#### Buscar faixas

```
GET /v1/tracks?q=<termo>&type=<tipo>&limit=<n>&offset=<n>
```

| Parâmetro | Tipo | Descrição |
|---|---|---|
| `q` | string | Busca por título ou artista (opcional) |
| `type` | string | Filtro por tipo: `MUSIC`, `JINGLE`, `VINHETA`, `SPOT` (opcional) |
| `limit` | int | Máximo de resultados (padrão: 50) |
| `offset` | int | Paginação (padrão: 0) |

**Resposta:**
```json
{
  "ok": true,
  "data": {
    "tracks": [
      {
        "id":          "01JA...",
        "path":        "/library/track01.mp3",
        "title":       "Nome da Faixa",
        "artist":      "Nome do Artista",
        "type":        "MUSIC",
        "duration_ms": 214500
      }
    ]
  }
}
```

#### Obter faixa por ID

```
GET /v1/tracks/{id}
```

#### Atualizar metadados de uma faixa

```
PATCH /v1/tracks/{id}
```

Patch parcial — apenas os campos presentes no body são alterados.

#### Listar artistas

```
GET /v1/tracks/artists
```

---

### Playlists

#### Listar playlists

```
GET /v1/playlists
```

#### Obter playlist (com itens)

```
GET /v1/playlists/{id}
```

#### Criar playlist

```
POST /v1/playlists
```

#### Atualizar playlist

```
PUT /v1/playlists/{id}
```

#### Excluir playlist

```
DELETE /v1/playlists/{id}
```

#### Adicionar item a uma playlist

```
POST /v1/playlists/{id}/items
```

#### Remover item de uma playlist

```
DELETE /v1/playlists/{id}/items/{item_id}
```

#### Reordenar itens de uma playlist

```
PUT /v1/playlists/{id}/items/reorder
```

**Body:** `{ "item_ids": ["uuid-1", "uuid-2", "uuid-3"] }`

---

### Blocos comerciais (Breaks)

#### Listar blocos

```
GET /v1/breaks
```

#### Obter bloco

```
GET /v1/breaks/{id}
```

`?format=engine-payload` retorna a estrutura pronta para envio ao RadioCore (`POST /v1/queue/enqueue-break`).

#### Criar bloco

```
POST /v1/breaks
```

#### Atualizar bloco

```
PUT /v1/breaks/{id}
```

#### Excluir bloco

```
DELETE /v1/breaks/{id}
```

#### Adicionar item a um bloco

```
POST /v1/breaks/{id}/items
```

#### Remover item de um bloco

```
DELETE /v1/breaks/{id}/items/{item_id}
```

#### Reordenar itens de um bloco

```
PUT /v1/breaks/{id}/items/reorder
```

---

### Botoneira (Hotkeys)

A Botoneira é um painel de botões de ação rápida para disparar áudios curtos (carts) sem interromper o fluxo principal. Cada **perfil** agrupa um conjunto de **botões**; cada botão referencia uma faixa da biblioteca.

#### Perfis

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET`    | `/v1/hotkeys/profiles`                    | Lista perfis com contagem de botões |
| `POST`   | `/v1/hotkeys/profiles`                    | Cria perfil |
| `GET`    | `/v1/hotkeys/profiles/{id}`               | Retorna perfil com botões |
| `PUT`    | `/v1/hotkeys/profiles/{id}`               | Atualiza nome e colunas |
| `DELETE` | `/v1/hotkeys/profiles/{id}`               | Remove perfil e todos os botões (CASCADE) |
| `POST`   | `/v1/hotkeys/profiles/{id}/buttons`       | Adiciona botão ao perfil |
| `PUT`    | `/v1/hotkeys/profiles/{id}/buttons/reorder` | Reordena botões |
| `PATCH`  | `/v1/hotkeys/buttons/{id}`                | Patch parcial de um botão |
| `DELETE` | `/v1/hotkeys/buttons/{id}`                | Remove botão |

**Exemplo de resposta — `GET /v1/hotkeys/profiles/{id}`:**
```json
{
  "ok": true,
  "data": {
    "id": "uuid",
    "name": "Efeitos",
    "columns": 4,
    "buttons": [
      {
        "id":           "btn-uuid",
        "position":     1,
        "label":        "Aplausos",
        "sub_label":    "8s",
        "icon":         "👏",
        "palette":      2,
        "track_id":     "track-uuid",
        "track_path":   "/library/efeitos/aplausos.mp3",
        "track_title":  "Aplausos",
        "track_artist": "",
        "track_type":   "VINHETA",
        "duration_ms":  8000
      }
    ]
  }
}
```

> **Nota:** `track_id` usa `ON DELETE SET NULL` — se a faixa for removida da biblioteca, o botão permanece com os metadados em cache e continua funcional.

---

## Rotação Musical

O sistema de **clock rotation** permite que a emissora programe a grade horária de forma automática. O operador define **categorias** de faixas, **clocks** (templates de 60 min com slots ordenados), uma **grade 24×7** associando clocks a cada hora da semana, **regras de separação** mínima entre faixas e usa o **gerador** para produzir playlists automáticas para qualquer período futuro.

### Conceito

```
[Categorias]  ←  faixas agrupadas por estilo/tipo
     ↓
[Clocks]      ←  template de 60 min com N slots ordenados (categoria, jingle, vinheta...)
     ↓
[Grade 24×7]  ←  qual clock toca em cada hora de cada dia da semana
     ↓
[Gerador]     ←  resolve clock → slots → faixas (com regras de separação)
     ↓
[Player UI]   ←  exibe lista gerada, enfileira no RadioCore, registra no log de rotação
```

---

### Categorias

Agrupam faixas por estilo musical ou função (ex.: MPB, Rock Nacional, Vinhetas Manhã). Uma faixa pode pertencer a múltiplas categorias (relação M:N).

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET`    | `/v1/categories`                          | Lista categorias com contagem de faixas |
| `POST`   | `/v1/categories`                          | Cria categoria |
| `GET`    | `/v1/categories/{id}`                     | Retorna categoria com contagem |
| `PUT`    | `/v1/categories/{id}`                     | Atualiza name / description / color |
| `DELETE` | `/v1/categories/{id}`                     | Remove (erro se referenciada por algum clock slot) |
| `GET`    | `/v1/categories/{id}/tracks`              | Lista faixas na categoria (paginado) |
| `POST`   | `/v1/categories/{id}/tracks`              | Adiciona uma ou mais faixas à categoria |
| `DELETE` | `/v1/categories/{id}/tracks/{track_id}`   | Remove faixa da categoria |
| `PUT`    | `/v1/tracks/{id}/categories`             | Substitui todas as categorias de uma faixa |

**Body — `POST /v1/categories`:**
```json
{ "name": "MPB Clássica", "description": "MPB dos anos 70–90", "color": "#20e6ff" }
```

**Body — `POST /v1/categories/{id}/tracks`:**
```json
{ "track_ids": ["track-uuid-1", "track-uuid-2"] }
```

**Resposta — `GET /v1/categories`:**
```json
{
  "ok": true,
  "data": [
    {
      "id":          "01JD...",
      "name":        "MPB Clássica",
      "description": "MPB dos anos 70–90",
      "color":       "#20e6ff",
      "track_count": 42,
      "created_at":  "2026-07-01T10:00:00Z"
    }
  ]
}
```

---

### Clocks

Um clock é um template de 60 minutos composto por **slots ordenados**. Cada slot define o que deve tocar naquela posição: uma categoria musical, um tipo fixo de áudio (JINGLE, VINHETA, SPOT) ou uma faixa fixa.

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET`    | `/v1/clocks`                              | Lista clocks com contagem de slots |
| `POST`   | `/v1/clocks`                              | Cria clock |
| `GET`    | `/v1/clocks/{id}`                         | Retorna clock com todos os slots |
| `PUT`    | `/v1/clocks/{id}`                         | Atualiza nome |
| `DELETE` | `/v1/clocks/{id}`                         | Remove (erro se o clock estiver na grade) |
| `POST`   | `/v1/clocks/{id}/slots`                   | Adiciona slot ao final |
| `PUT`    | `/v1/clocks/{id}/slots/{slot_id}`         | Atualiza slot |
| `DELETE` | `/v1/clocks/{id}/slots/{slot_id}`         | Remove slot |
| `PUT`    | `/v1/clocks/{id}/slots/reorder`           | Reordena slots (array de IDs) |

**Tipos de slot válidos:** `CATEGORY` · `JINGLE` · `SPOT` · `VINHETA` · `HORA_CERTA` · `FIXED`

**Body — `POST /v1/clocks/{id}/slots`:**
```json
{
  "slot_type":       "CATEGORY",
  "category_id":     "01JD...",
  "duration_hint_ms": 0
}
```

Para `slot_type=FIXED`, usar `fixed_track_id` em vez de `category_id`.

**Resposta — `GET /v1/clocks/{id}`:**
```json
{
  "ok": true,
  "data": {
    "id":         "01JE...",
    "name":       "Manhã Adulto",
    "slot_count": 8,
    "slots": [
      {
        "id":               "01JF...",
        "clock_id":         "01JE...",
        "position":         1,
        "slot_type":        "CATEGORY",
        "category_id":      "01JD...",
        "category_name":    "MPB Clássica",
        "fixed_track_id":   "",
        "duration_hint_ms": 0
      },
      {
        "id":            "01JG...",
        "position":      2,
        "slot_type":     "VINHETA",
        "category_id":   "",
        "category_name": ""
      }
    ]
  }
}
```

---

### Grade de clocks

Matriz 7×24 que associa um clock a cada combinação dia-da-semana + hora. `weekday`: 0=Domingo … 6=Sábado.

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET` | `/v1/schedule/clock-grid` | Retorna todas as células preenchidas |
| `PUT` | `/v1/schedule/clock-grid` | Atualiza uma ou mais células |

**Body — `PUT /v1/schedule/clock-grid`:**
```json
[
  { "weekday": 1, "hour": 8,  "clock_id": "01JE..." },
  { "weekday": 1, "hour": 9,  "clock_id": "01JE..." },
  { "weekday": 0, "hour": 22, "clock_id": null }
]
```

`clock_id: null` limpa a célula (hora sem clock programado).

**Resposta — `GET /v1/schedule/clock-grid`:**
```json
{
  "ok": true,
  "data": {
    "grid": [
      {
        "weekday":    1,
        "hour":       8,
        "clock_id":   "01JE...",
        "clock_name": "Manhã Adulto"
      }
    ]
  }
}
```

Retorna apenas células preenchidas. O Player preenche as vazias como "(sem clock)".

---

### Regras de separação

Definem o intervalo mínimo (em minutos) entre faixas com o mesmo valor de um campo. O gerador as aplica ao selecionar faixas, com fallback progressivo quando o catálogo é insuficiente.

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET`    | `/v1/schedule/separation-rules`       | Lista regras |
| `POST`   | `/v1/schedule/separation-rules`       | Cria regra |
| `PUT`    | `/v1/schedule/separation-rules/{id}`  | Atualiza |
| `DELETE` | `/v1/schedule/separation-rules/{id}`  | Remove |

**Campos válidos:** `artist` · `title` · `album` · `category`

**Body — `POST /v1/schedule/separation-rules`:**
```json
{ "field": "artist", "min_sep_minutes": 60 }
```

**Resposta — `GET /v1/schedule/separation-rules`:**
```json
{
  "ok": true,
  "data": [
    { "id": "01JH...", "field": "artist", "min_sep_minutes": 60 },
    { "id": "01JI...", "field": "album",  "min_sep_minutes": 120 }
  ]
}
```

---

### Gerador de playlist

Gera uma lista de faixas resolvidas para um período futuro, baseada na grade de clocks, nas categorias e nas regras de separação.

```
POST /v1/schedule/generate
```

**Body:**
```json
{
  "from":  "2026-07-20T08:00:00",
  "hours": 4
}
```

| Campo | Tipo | Descrição |
|---|---|---|
| `from` | string | ISO 8601 local (sem timezone). Omitido = `now`. |
| `hours` | int | Horas a gerar. Máximo: 24. Default: 1. |

**Resposta:**
```json
{
  "ok": true,
  "data": {
    "from":  "2026-07-20T08:00:00",
    "to":    "2026-07-20T12:00:00",
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
          "duration_ms": 243000,
          "type":        "MUSIC"
        }
      }
    ],
    "warnings": [
      "Hora 9, slot 3 (Rock Nacional): separação de artista relaxada — apenas 2 faixas disponíveis na categoria"
    ]
  }
}
```

> O gerador **não persiste** nada. O Player UI decide o que enfileirar e depois chama `POST /v1/rotation-log` para registrar o que foi de fato programado.

---

### Log de rotação

Registro append-only do que foi programado (chamado pelo Player após enfileirar). Serve de base histórica para as regras de separação entre sessões.

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `POST` | `/v1/rotation-log`                 | Registra uma ou mais entradas |
| `GET`  | `/v1/rotation-log?date=YYYY-MM-DD` | Consulta log do dia |

**Body — `POST /v1/rotation-log`** (array):
```json
[
  {
    "track_id":    "01JA...",
    "played_at":   "2026-07-20T08:00:00Z",
    "clock_id":    "01JE...",
    "slot_type":   "CATEGORY",
    "category_id": "01JD...",
    "artist":      "Elis Regina",
    "title":       "Como Nossos Pais",
    "album":       "Falso Brilhante"
  }
]
```

**Resposta — `POST /v1/rotation-log`:**
```json
{ "ok": true, "count": 12 }
```

**Resposta — `GET /v1/rotation-log?date=2026-07-20`:**
```json
{
  "ok":   true,
  "date": "2026-07-20",
  "data": [
    {
      "id":          "01JK...",
      "track_id":    "01JA...",
      "played_at":   "2026-07-20T08:00:00Z",
      "clock_id":    "01JE...",
      "slot_type":   "CATEGORY",
      "category_id": "01JD...",
      "artist":      "Elis Regina",
      "title":       "Como Nossos Pais",
      "album":       "Falso Brilhante"
    }
  ]
}
```

---

## Modelo de dados — Rotação

Migration `004_clock_rotation.sql`:

```sql
-- Categorias de rotação
CREATE TABLE categories (
    id          TEXT     PRIMARY KEY,
    name        TEXT     NOT NULL UNIQUE,
    description TEXT     NOT NULL DEFAULT '',
    color       TEXT     NOT NULL DEFAULT '#888888',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Associação M:N faixa ↔ categoria
CREATE TABLE track_categories (
    track_id    TEXT NOT NULL REFERENCES tracks(id)     ON DELETE CASCADE,
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (track_id, category_id)
);

-- Clocks (templates de 60 min)
CREATE TABLE clocks (
    id         TEXT     PRIMARY KEY,
    name       TEXT     NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Slots ordenados dentro de um clock
CREATE TABLE clock_slots (
    id               TEXT    PRIMARY KEY,
    clock_id         TEXT    NOT NULL REFERENCES clocks(id) ON DELETE CASCADE,
    position         INTEGER NOT NULL,
    slot_type        TEXT    NOT NULL CHECK(slot_type IN ('CATEGORY','JINGLE','SPOT','VINHETA','HORA_CERTA','FIXED')),
    category_id      TEXT    REFERENCES categories(id) ON DELETE SET NULL,
    fixed_track_id   TEXT    REFERENCES tracks(id)     ON DELETE SET NULL,
    duration_hint_ms INTEGER NOT NULL DEFAULT 0,
    UNIQUE(clock_id, position)
);

-- Grade 24×7: qual clock toca em cada hora de cada dia
-- weekday: 0=Domingo … 6=Sábado
CREATE TABLE clock_schedule (
    weekday  INTEGER NOT NULL CHECK(weekday BETWEEN 0 AND 6),
    hour     INTEGER NOT NULL CHECK(hour    BETWEEN 0 AND 23),
    clock_id TEXT    REFERENCES clocks(id) ON DELETE SET NULL,
    PRIMARY KEY (weekday, hour)
);

-- Regras de separação mínima
CREATE TABLE separation_rules (
    id              TEXT    PRIMARY KEY,
    field           TEXT    NOT NULL CHECK(field IN ('artist','title','category','album')),
    min_sep_minutes INTEGER NOT NULL DEFAULT 60
);

-- Log append-only do que tocou
CREATE TABLE rotation_log (
    id          TEXT     PRIMARY KEY,
    track_id    TEXT     NOT NULL,
    played_at   DATETIME NOT NULL,
    clock_id    TEXT     NOT NULL DEFAULT '',
    slot_type   TEXT     NOT NULL DEFAULT '',
    category_id TEXT     NOT NULL DEFAULT '',
    artist      TEXT     NOT NULL DEFAULT '',
    title       TEXT     NOT NULL DEFAULT '',
    album       TEXT     NOT NULL DEFAULT ''
);
```

---

## Algoritmo do gerador

Implementado em `internal/scheduler/generator.go`. Recebe `from time.Time` e `hours int`, retorna `[]GeneratedItem` e `[]string` (warnings).

```
Para cada hora H em [from, from+hours):
  clock = grade[weekday(H)][hour(H)]
  se clock == nil → emite warning, pula hora

  Para cada slot S em clock.slots (ordem por position):
    se slot_type == FIXED:
      item = tracks[S.fixed_track_id]
      continua

    se slot_type in {JINGLE, SPOT, VINHETA, HORA_CERTA}:
      candidatos = tracks WHERE type == slot_type
    se slot_type == CATEGORY:
      candidatos = tracks na categoria S.category_id

    se candidatos vazio → emite warning, pula slot

    Fase 1 — filtro estrito (todas as regras de separação):
      remove candidatos que violam qualquer regra
      (considera rotation_log + faixas já escolhidas nesta sessão)

    Fase 2 — fallback parcial (se filtrado vazio):
      relaxa a regra com menor min_sep_minutes
      refaz o filtro sem ela
      emite warning de relaxamento

    Fase 3 — fallback total (se ainda vazio):
      usa a faixa com played_at mais antiga no log da categoria
      (ou aleatória se nenhuma foi tocada antes)
      emite warning "separação ignorada"

    Escolha: aleatória entre os candidatos restantes
    Registra em memória (para separação intra-sessão)
```

### Interface pública

```go
package scheduler

func New(
    clocks   ClockQuerier,
    tracks   TrackQuerier,
    sepRules SeparationQuerier,
    rotLog   RotationLogQuerier,
) *Generator

func (g *Generator) Generate(ctx context.Context, from time.Time, hours int) ([]GeneratedItem, []string, error)
```

### Tipos principais

```go
type TrackRef struct {
    ID         string
    Path       string
    Title      string
    Artist     string
    Album      string
    DurationMS int64
    Type       string // MUSIC | VINHETA | JINGLE | SPOT
    CategoryID string
}

type GeneratedItem struct {
    Hour         int
    Position     int
    SlotID       string
    SlotType     string
    ClockID      string
    ClockName    string
    CategoryID   string
    CategoryName string
    Track        TrackRef
}
```

Todas as dependências são interfaces — o gerador é testável sem banco.

---

## Contratos de resposta

### Envelope padrão

```json
{ "ok": true,  "data": ... }
{ "ok": false, "error": "codigo_snake", "message": "descrição humana" }
```

### Objeto de faixa

| Campo | Tipo | Descrição |
|---|---|---|
| `id` | string | ULID da faixa |
| `path` | string | Caminho absoluto no sistema de arquivos |
| `title` | string | Título da faixa |
| `artist` | string | Artista (pode ser vazio) |
| `album` | string | Álbum (pode ser vazio) |
| `type` | string | `MUSIC` · `JINGLE` · `VINHETA` · `SPOT` |
| `duration_ms` | int | Duração em milissegundos (via ffprobe) |

---

## Integração com o Player

O RadioFlow Player consome esta API na aba **Biblioteca** (drawer lateral) e na aba **Rotação**:

| Ação do operador | Chamada ao Library Service | Chamada ao RadioCore |
|---|---|---|
| Buscar faixas | `GET /v1/tracks?q=...` | — |
| Enfileirar faixa | — | `POST /v1/queue/enqueue` |
| Listar playlists | `GET /v1/playlists` | — |
| Enfileirar playlist | `GET /v1/playlists/:id` | `POST /v1/queue/enqueue` |
| Listar blocos | `GET /v1/breaks` | — |
| Enfileirar bloco | `GET /v1/breaks/:id?format=engine-payload` | `POST /v1/queue/enqueue-break` |
| Preview de faixa | — | `POST /v1/preview/play` |
| Listar perfis da botoneira | `GET /v1/hotkeys/profiles` | — |
| Criar/editar perfil | `POST/PUT /v1/hotkeys/profiles` | — |
| Adicionar/editar botão | `POST /v1/hotkeys/profiles/:id/buttons` | — |
| Disparar cart (botão) | — | `POST /v1/cart/play` |
| Gerenciar categorias | `GET/POST/PUT/DELETE /v1/categories` | — |
| Associar faixas a categoria | `POST /v1/categories/:id/tracks` | — |
| Gerenciar clocks e slots | `GET/POST/PUT/DELETE /v1/clocks` | — |
| Configurar grade 24×7 | `GET/PUT /v1/schedule/clock-grid` | — |
| Gerenciar regras de separação | `GET/POST/PUT/DELETE /v1/schedule/separation-rules` | — |
| Gerar playlist automática | `POST /v1/schedule/generate` | — |
| Enfileirar playlist gerada | — | `POST /v1/queue/enqueue` (por faixa) |
| Registrar rotação executada | `POST /v1/rotation-log` | — |
| Consultar log do dia | `GET /v1/rotation-log?date=YYYY-MM-DD` | — |

### Fluxo de rotação no Player

```
1. Operador configura Categorias → adiciona faixas
2. Operador cria Clocks → adiciona slots (tipo + categoria)
3. Operador preenche Grade 24×7 → associa clock por hora
4. Operador define Regras de separação (ex.: artista ≥ 60 min)
5. Aba Gerar → seleciona data/hora e quantidade de horas
6. Player chama POST /v1/schedule/generate → exibe lista com warnings
7. Operador clica "Enfileirar no Player"
8. Player itera sobre itens → POST /v1/queue/enqueue (RadioCore) por faixa
9. Player registra cada item → POST /v1/rotation-log (Library Service)
10. Próxima geração usa o log para respeitar separação histórica
```
