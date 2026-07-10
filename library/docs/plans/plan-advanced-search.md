# Plano: Busca Avançada — Library Service ✅ IMPLEMENTADO

## Contexto

O modal de busca avançada do player precisa filtrar faixas por **nome**, **tipo**, **artista** e **álbum** com múltiplos filtros simultâneos e paginação server-side. O Library Service já armazena todos esses campos, mas:

- `album` não pode ser filtrado (ausente do `SearchQuery` e do handler)
- `EFEITOS` não é um tipo válido (CHECK constraint não inclui)
- `album` não tem índice (lento para milhares de faixas)
- `title` não tem índice (busca `LIKE` em texto livre — lento)

Este plano cobre exclusivamente as mudanças no **Library Service**.

---

## Fase 1 — Migrations de banco

### 1.1 — Criar `002_advanced_search.sql`

Arquivo: `internal/store/migrations/002_advanced_search.sql`

```sql
-- Adiciona EFEITOS como tipo válido (recriar tabela pois SQLite não suporta ALTER CHECK)
-- Estratégia: remover CHECK constraint via recriação da tabela com RENAME

CREATE TABLE IF NOT EXISTS tracks_new (
    id          TEXT PRIMARY KEY,
    path        TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL DEFAULT '',
    artist      TEXT NOT NULL DEFAULT '',
    album       TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL CHECK(type IN ('MUSIC','VINHETA','JINGLE','SPOT','EFEITOS')),
    duration_ms INTEGER NOT NULL DEFAULT 0,
    category    TEXT,
    indexed_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO tracks_new(id, path, title, artist, album, type, duration_ms, category, indexed_at)
SELECT id, path, title, artist, COALESCE(album,''), type, duration_ms, category, indexed_at
FROM tracks;

DROP TABLE tracks;
ALTER TABLE tracks_new RENAME TO tracks;

-- Recriar índices existentes
CREATE INDEX IF NOT EXISTS idx_tracks_type     ON tracks(type);
CREATE INDEX IF NOT EXISTS idx_tracks_artist   ON tracks(artist);
CREATE INDEX IF NOT EXISTS idx_tracks_category ON tracks(category);

-- Novos índices para busca avançada
CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album);
CREATE INDEX IF NOT EXISTS idx_tracks_title ON tracks(title);
```

> **Por quê recriar a tabela?** O SQLite não suporta `ALTER TABLE ... ALTER COLUMN` nem modificação de CHECK constraints. A estratégia de rename é a idiomática e preserva todos os dados.

### 1.2 — Atualizar `db.go`

Remover a migration inline de `ALTER TABLE tracks ADD COLUMN album` (já foi absorvida pela `002`). Garantir que o runner de migrations execute os arquivos em ordem numérica.

---

## Fase 2 — `SearchQuery` e filtro por álbum

### 2.1 — MODIFICAR `internal/store/track_store.go`

Adicionar `Album` ao `SearchQuery`:

```go
type SearchQuery struct {
    Q        string // full-text: title e artist
    Type     string
    Artist   string
    Album    string // novo
    Category string
    Limit    int
    Offset   int
}
```

### 2.2 — MODIFICAR método `Search()`

Adicionar cláusula condicional para álbum na query dinâmica:

```go
if q.Album != "" {
    conditions = append(conditions, "album LIKE ?")
    args = append(args, "%"+q.Album+"%")
}
```

---

## Fase 3 — Handler HTTP

### 3.1 — MODIFICAR `internal/api/handlers/tracks.go`

Adicionar leitura de `?album=` no handler `SearchTracks`:

```go
sq := store.SearchQuery{
    Q:        q.Get("q"),
    Type:     q.Get("type"),
    Artist:   q.Get("artist"),
    Album:    q.Get("album"),   // novo
    Category: q.Get("category"),
    Limit:    limit,
    Offset:   offset,
}
```

---

## Fase 4 — Tipos válidos na config e scanner

### 4.1 — VERIFICAR `internal/config/config.go`

Confirmar se `EFEITOS` precisa ser mapeado em `dir → type` na configuração de roots. Se o scanner usa o enum de tipos para validar, adicionar `"EFEITOS"` ao conjunto permitido.

### 4.2 — VERIFICAR `internal/scanner/indexer.go`

Confirmar que o indexer aceita `EFEITOS` como tipo ao processar diretórios mapeados.

---

## Endpoints afetados

### `GET /v1/tracks`

Parâmetros de query após esta fase:

| Parâmetro | Tipo | Descrição |
|---|---|---|
| `q` | string | Busca livre em título e artista |
| `type` | string | `MUSIC` \| `VINHETA` \| `JINGLE` \| `SPOT` \| `EFEITOS` |
| `artist` | string | Filtro exato por artista (`LIKE %valor%`) |
| `album` | string | Filtro por álbum (`LIKE %valor%`) — **novo** |
| `category` | string | Filtro por categoria |
| `limit` | int | Padrão 50, máximo 200 |
| `offset` | int | Para paginação |

---

## Arquivos modificados

| Arquivo | Ação |
|---|---|
| `internal/store/migrations/002_advanced_search.sql` | CRIAR |
| `internal/store/db.go` | Remover `ALTER TABLE` inline do album; garantir runner de migrations |
| `internal/store/track_store.go` | Adicionar `Album` ao `SearchQuery`; adicionar cláusula no `Search()` |
| `internal/api/handlers/tracks.go` | Ler `?album=` e passar ao `SearchQuery` |
| `internal/config/config.go` | Verificar/adicionar `EFEITOS` |
| `internal/scanner/indexer.go` | Verificar/aceitar `EFEITOS` |

---

## Verificação

```bash
# Build e testes
go build ./...
go test ./...

# Testar filtro por álbum
curl "http://localhost:8081/v1/tracks?album=Best+Of"

# Testar novo tipo
curl "http://localhost:8081/v1/tracks?type=EFEITOS"

# Testar filtros combinados
curl "http://localhost:8081/v1/tracks?type=MUSIC&artist=Beatles&album=Abbey+Road&limit=20&offset=0"
```
