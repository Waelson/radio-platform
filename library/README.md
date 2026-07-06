# Radio Library Service

Serviço HTTP de catálogo de áudio para o sistema de automação de rádio.
Fornece busca de faixas, playlists e blocos comerciais consumidos pelo [RadioFlow Player](../radio-platform/player/README.md).

---

## Índice

- [Visão geral](#visão-geral)
- [Porta padrão](#porta-padrão)
- [API REST](#api-rest)
  - [Health](#health)
  - [Faixas](#faixas)
  - [Playlists](#playlists)
  - [Blocos comerciais (Breaks)](#blocos-comerciais-breaks)
- [Contratos de resposta](#contratos-de-resposta)
- [Integração com o Player](#integração-com-o-player)

---

## Visão geral

O Radio Library Service é um serviço independente responsável por indexar e servir o acervo de áudio da rádio. O RadioFlow Player consulta este serviço para popular a Biblioteca (drawer lateral) e enfileirar conteúdo no RadioCore.

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
| `type` | string | Filtro por tipo: `music`, `jingle`, `vinheta`, etc. (opcional) |
| `limit` | int | Máximo de resultados (padrão: 50) |
| `offset` | int | Paginação (padrão: 0) |

**Resposta:**
```json
{
  "tracks": [
    {
      "path":        "/library/track01.mp3",
      "title":       "Nome da Faixa",
      "artist":      "Nome do Artista",
      "type":        "music",
      "duration_ms": 214500
    }
  ]
}
```

---

### Playlists

#### Listar playlists

```
GET /v1/playlists
```

**Resposta:**
```json
{
  "playlists": [
    { "id": "playlist-uuid", "name": "Manhã" }
  ]
}
```

#### Buscar itens de uma playlist

```
GET /v1/playlists/:id
```

**Resposta:**
```json
{
  "id": "playlist-uuid",
  "name": "Manhã",
  "items": [
    {
      "track": {
        "path":        "/library/track01.mp3",
        "title":       "Nome da Faixa",
        "artist":      "Nome do Artista",
        "type":        "music",
        "duration_ms": 214500
      }
    }
  ]
}
```

---

### Blocos comerciais (Breaks)

#### Listar blocos

```
GET /v1/breaks
```

**Resposta:**
```json
{
  "breaks": [
    { "id": "break-uuid", "name": "Bloco 1" }
  ]
}
```

#### Buscar payload de um bloco para o engine

```
GET /v1/breaks/:id?format=engine-payload
```

Retorna o bloco com estrutura pronta para envio ao endpoint `POST /v1/queue/enqueue-break` do RadioCore.

**Resposta:**
```json
{
  "name": "Bloco 1",
  "open": {
    "path": "/library/abertura.mp3",
    "type": "vinheta",
    "title": "Abertura do Bloco",
    "artist": "",
    "duration_ms": 8000
  },
  "close": {
    "path": "/library/encerramento.mp3",
    "type": "vinheta",
    "title": "Encerramento do Bloco",
    "artist": "",
    "duration_ms": 6000
  },
  "spots": [
    {
      "path":        "/library/spot01.mp3",
      "type":        "spot",
      "title":       "Anúncio 1",
      "artist":      "",
      "duration_ms": 30000
    }
  ]
}
```

---

## Contratos de resposta

Todos os objetos de faixa seguem o mesmo schema:

| Campo | Tipo | Descrição |
|---|---|---|
| `path` | string | Caminho absoluto do arquivo de áudio no sistema de arquivos |
| `title` | string | Título da faixa |
| `artist` | string | Artista (pode ser vazio) |
| `type` | string | Tipo: `music`, `jingle`, `vinheta`, `spot`, etc. |
| `duration_ms` | int | Duração em milissegundos |

---

## Integração com o Player

O RadioFlow Player consome esta API na aba **Biblioteca** (drawer lateral):

| Ação do operador | Chamada ao Library Service | Chamada ao RadioCore |
|---|---|---|
| Buscar faixas | `GET /v1/tracks?q=...` | — |
| Enfileirar faixa | — | `POST /v1/queue/enqueue` |
| Listar playlists | `GET /v1/playlists` | — |
| Enfileirar playlist | `GET /v1/playlists/:id` | `POST /v1/queue/enqueue` |
| Listar blocos | `GET /v1/breaks` | — |
| Enfileirar bloco | `GET /v1/breaks/:id?format=engine-payload` | `POST /v1/queue/enqueue-break` |
| Preview de faixa | — | `POST /v1/preview/play` |
