# CLAUDE.md — Radio Library Service

## Papel do Claude Code

Você está implementando o **Library Service** em Go para o sistema de rádio.
É um serviço independente responsável por indexar, armazenar e expor metadados
da biblioteca de áudio (músicas, vinhetas, jingles e spots).

---

## Regras fundamentais

1. **Nunca importar nenhum pacote de `radio-playout-engine`.**
2. **Este serviço não toca áudio.** Apenas gerencia metadados e caminhos.
3. **A API nunca chama o playout engine diretamente.**
4. **A integração com o engine é feita exclusivamente pela UI (HTML) via HTTP.**
5. **Não use ORM.** Use `database/sql` puro com queries SQL explícitas.
6. **Não use framework HTTP.** Use `net/http` puro com `http.ServeMux`.
7. **Todo I/O recebe `context.Context` como primeiro argumento.**
8. **Erros devem ser contextualizados com `fmt.Errorf("...: %w", err)`.**
9. **Goroutines devem ter caminho claro de encerramento via context ou canal.**
10. **`type` de uma track vem exclusivamente do mapeamento diretório→tipo da config.**
11. **`duration_ms` vem sempre do ffprobe — nunca inferido do nome do arquivo.**

## Tipos de asset

- `MUSIC` — músicas
- `VINHETA` — vinhetas
- `JINGLE` — jingles institucionais
- `SPOT` — spots comerciais

`HORA_CERTA` é gerenciada pelo playout engine — fora do escopo deste serviço.

## Banco de dados

- SQLite via `modernc.org/sqlite` (sem CGO, portável).
- `db.SetMaxOpenConns(1)` — SQLite não suporta escritas concorrentes.
- `PRAGMA journal_mode=WAL` — habilitar na abertura.
- `PRAGMA foreign_keys=ON` — garantir integridade referencial.
- Migrations em `internal/store/migrations/` com arquivos `.sql` numerados.

## Convenção de nomenclatura de arquivos (fallback sem tags ID3)

- Músicas:  `[Categoria] Artista - Título.mp3`
- Vinhetas: `[Categoria] Título.mp3`
- Jingles:  `[Categoria] Rádio - Título.mp3`
- Spots:    `[Categoria] Anunciante - Título.mp3`

Lógica de extração:
1. Tags ID3/Vorbis via ffprobe (prioridade máxima)
2. Extrai `[Categoria]` do início do nome
3. Split por ` - `: esquerda = artista, direita = título
4. Sem ` - `: título = nome completo, artista = vazio

## Padrão de resposta HTTP

Sempre JSON com envelope:
```json
{"ok": true,  "data": ...}
{"ok": false, "error": "codigo_snake", "message": "descrição humana"}
```

## Testes

- Store: banco `:memory:` via `store.Open(ctx, ":memory:")`
- Scanner: fixtures em `internal/scanner/testdata/`
- Handlers: `httptest.NewRecorder`

## Ordem de implementação por fase

1. Scaffold (config, logger, SQLite, main)
2. Scanner (ffprobe, nameparser, indexer)
3. API tracks (search, get, patch, artists)
4. API playlists (CRUD)
5. API breaks (CRUD + engine-payload)
6. Watcher fsnotify
7. API indexação (status + scan)
8. Integração no player HTML (toca apenas o radio-playout-engine/player-v5.html)

## O que NÃO fazer

- Não importar `radio-playout-engine`.
- Não chamar o playout engine diretamente.
- Não implementar upload de arquivos de áudio.
- Não tocar áudio.
- Não usar ORM ou framework HTTP.
- Não usar `time.Sleep` como mecanismo de sincronização.
- Não iniciar goroutines sem caminho claro de shutdown.
