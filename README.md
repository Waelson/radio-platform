# radio-platform

Monorepo contendo os serviços da plataforma de rádio RadioFlow.

## Serviços

| Diretório | Módulo Go | Descrição |
|-----------|-----------|-----------|
| `playout/` | `github.com/Waelson/radio-playout-engine` | Engine de playout de áudio (decode, mix, crossfade, fila, WebSocket) |
| `library/` | `github.com/Waelson/radio-library-service` | Catálogo de áudio, playlists, botoneira e rotação musical automática |
| `player/`  | —                                          | App Electron — interface do operador de rádio |

## Arquitetura

```
[player/ — Electron]
        ↓ HTTP / WebSocket
[playout/ — Engine]      [library/ — Library Service]
        ↑                        ↑
  fila de áudio            catálogo + rotação
```

O Player consulta o Library Service para montar playlists e gerar rotação automática, e envia os paths dos arquivos ao Playout Engine via API. O Engine nunca acessa o banco de dados — recebe apenas paths e metadados já resolvidos.

## Funcionalidades principais

### Playout Engine (`playout/`)
- Pipeline de áudio: decode, mix, crossfade, normalização
- Fila de reprodução com suporte a múltiplos tipos de faixa
- Modo pânico e modo assistido
- API REST + WebSocket para controle em tempo real
- Suporte a cart (botoneira), preview (CUE) e breaks comerciais

### Library Service (`library/`)
- Indexação automática do acervo (scan + watcher fsnotify)
- Busca de faixas com filtros por tipo, artista, título e álbum
- Playlists e blocos comerciais (breaks) com ordenação
- Botoneira (hotkeys): perfis e botões configuráveis
- **Rotação musical automática (clock rotation):**
  - Categorias de faixas com associação M:N
  - Clocks (templates de 60 min com slots ordenados)
  - Grade de programação 24×7 (weekday × hour → clock)
  - Regras de separação mínima por artista, álbum, categoria
  - Gerador de playlist com 3 níveis de fallback e warnings
  - Log de rotação histórico para separação entre sessões

### Player (`player/`)
- Interface Electron para o operador de rádio
- Controles de reprodução, fila, crossfade e volume
- Biblioteca integrada (busca + enfileiramento)
- Botoneira visual com paletas de cores
- Aba de Rotação: gerenciamento de categorias, clocks, grade, regras e geração de playlist

## Pré-requisitos

- Go 1.24+
- ffmpeg (playout engine — decode e probe de áudio)
- Node.js + Electron (player)

## Como rodar cada serviço

### Playout Engine

```bash
cd playout
make build-coreaudio        # macOS com CoreAudio
./playout-engine --startup=cli
```

Ou via bundle macOS:

```bash
make dist-mac
open dist/Playout.app
```

### Library Service

```bash
cd library
make build
./library-service
```

Porta padrão: `8081`. Configurável via `config.yaml` ou variáveis de ambiente.

### Player

```bash
cd player
npm install
npm start
```

## Targets do Makefile raiz

| Target | Descrição |
|--------|-----------|
| `make build-playout` | Compila o playout engine (coreaudio) |
| `make build-library` | Compila o library-service |
| `make test-all` | Roda testes de ambos os serviços Go |
| `make dist-mac` | Gera o bundle macOS `playout/dist/Playout.app` |
| `make clean` | Remove binários de ambos os serviços |

## Go Workspaces

O arquivo `go.work` na raiz resolve os módulos localmente sem depender do GitHub:

```bash
go work init ./playout ./library
```

Permite desenvolver features que tocam os dois serviços simultaneamente sem publicar versões intermediárias.

> Em produção cada serviço é buildado e deployado separadamente — o `go.work` é exclusivo para desenvolvimento local.

## Estrutura

```
radio-platform/
├── go.work
├── go.work.sum
├── Makefile
├── README.md
├── CLAUDE.md
├── playout/                    ← radio-playout-engine
│   ├── go.mod
│   ├── Makefile
│   ├── cmd/playout-engine/
│   └── internal/
├── library/                    ← radio-library-service
│   ├── go.mod
│   ├── Makefile
│   ├── README.md               ← API REST completa + rotação
│   ├── cmd/library-service/
│   └── internal/
│       ├── store/              ← SQLite (tracks, playlists, breaks, hotkeys, rotation)
│       ├── scanner/            ← indexação + watcher fsnotify
│       ├── scheduler/          ← gerador de playlist (clock rotation)
│       ├── indexsvc/           ← serviço de indexação assíncrona
│       └── api/handlers/       ← handlers HTTP
└── player/                     ← app Electron
    ├── main.js
    ├── player.html             ← UI principal do operador
    └── icons/
```

## Documentação

- `library/README.md` — API REST completa do Library Service, incluindo todos os endpoints de rotação, contratos JSON, modelo de dados e algoritmo do gerador
- `library/docs/plans/` — planos técnicos de cada feature implementada
- `player/docs/` — benchmarks e planos da UI do Player
