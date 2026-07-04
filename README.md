# radio-platform

Monorepo contendo os serviços da plataforma de rádio.

## Serviços

| Diretório | Módulo Go | Descrição |
|-----------|-----------|-----------|
| `playout/` | `github.com/Waelson/radio-playout-engine` | Engine de playout de áudio para rádio FM/AM |
| `library/` | `github.com/Waelson/radio-library-service` | Serviço de biblioteca de músicas e playlists |
| `player/`  | —                                          | Futuro app Electron para o player (placeholder) |

## Pré-requisitos

- Go 1.24+
- ffmpeg (para o playout engine)
- SQLite (para o library-service)

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

## Targets do Makefile raiz

| Target | Descrição |
|--------|-----------|
| `make build-playout` | Compila o playout engine (coreaudio) |
| `make build-library` | Compila o library-service |
| `make test-all` | Roda testes de ambos os serviços |
| `make dist-mac` | Gera o bundle macOS `playout/dist/Playout.app` |
| `make clean` | Remove binários de ambos os serviços |

## Go Workspaces

O arquivo `go.work` na raiz resolve os módulos localmente sem depender do GitHub:

```bash
go work init ./playout ./library
```

Isso permite desenvolver features que tocam os dois serviços simultaneamente sem publicar versões intermediárias.

## Estrutura

```
radio-platform/
├── go.work
├── go.work.sum
├── Makefile
├── .gitignore
├── README.md
├── CLAUDE.md
├── playout/          ← radio-playout-engine
│   ├── go.mod
│   ├── Makefile
│   ├── cmd/playout-engine/
│   └── internal/
├── library/          ← radio-library-service
│   ├── go.mod
│   ├── Makefile
│   ├── cmd/library-service/
│   └── internal/
└── player/           ← futuro app Electron
    └── .gitkeep
```
