# CLAUDE.md — radio-platform

## Visão geral

Este monorepo contém os serviços da plataforma de rádio:

- **`playout/`** — Playout Engine em Go. Processo separado responsável por toda a pipeline de áudio: decode, mix, crossfade, panic mode, playback de fila. Expõe uma API REST + WebSocket. Não possui UI própria.
- **`library/`** — Library Service em Go. Gerencia o catálogo de músicas e playlists via API REST. Usa SQLite como banco.
- **`player/`** — Futuro app Electron. Interface visual do player. Consumirá a API do playout engine e do library-service.

## Relação entre os serviços

```
[player/ — Electron]
       ↓ HTTP / WebSocket
[playout/ — Engine]      [library/ — Library Service]
       ↑                        ↑
   paths + metadata         catálogo de música
```

O player consulta o library-service para montar playlists e envia os paths dos arquivos para o playout engine via API. O playout engine nunca acessa o banco de dados — ele recebe apenas paths e metadados já resolvidos.

## Idioma

Todas as respostas devem ser em **português brasileiro**, sem exceção.

## Fluxo de trabalho obrigatório — análise antes de agir

**Nunca implemente, altere ou corrija nada sem autorização explícita do usuário.**

Ao identificar um problema, necessidade de ajuste ou oportunidade de melhoria, siga este formato antes de qualquer ação:

1. **Finding** — o que foi encontrado/identificado.
2. **Problema** — descrição clara do que está errado ou faltando.
3. **Impacto** — o que isso causa no sistema ou no usuário.
4. **Solução proposta / necessidade de investigação** — o que precisa ser feito e como.
5. **Pergunta** — "Deseja que eu prossiga com essa correção/implementação?"

Só execute a ação após receber confirmação explícita do usuário.

## Regras globais

1. Cada serviço é um módulo Go independente — não criar dependências diretas entre `playout/` e `library/`.
2. Comunicação entre serviços é sempre via HTTP — nunca por chamada de função direta.
3. O `go.work` existe apenas para desenvolvimento local — em produção cada serviço é buildado e deployado separadamente.
4. O `player/` é um app separado — não colocar código de UI dentro de `playout/` ou `library/`.
5. Cada subdiretório tem seu próprio `CLAUDE.md` com regras específicas — ler antes de trabalhar em cada serviço.
6. **Toda implementação originada de um plano (`docs/plans/`) deve ser desenvolvida em uma branch dedicada**, criada a partir de `main` antes de qualquer alteração de código. O nome da branch deve ser descritivo e derivado do nome do plano (ex.: `feature/hotkeys`, `feature/cart-player`). Nunca implementar diretamente na `main`.

## Como trabalhar neste repo

### Build de todos os serviços

```bash
make build-playout   # playout engine (macOS coreaudio)
make build-library   # library service
```

### Testes

```bash
make test-all
```

### Desenvolvimento cruzado (go.work)

O `go.work` permite importar módulos locais entre si durante desenvolvimento. Não commitar código que dependa disso em produção.

## Autonomia operacional

- Ferramentas de leitura (Read, Glob, Grep, Bash somente leitura) podem ser usadas sem pedir autorização prévia.
- O comando `sed` pode ser executado sem autorização explícita para edições pontuais em arquivos.
- Comandos que não alteram estado (`git status`, `git log`, `git diff`, `go test`, `curl GET`) podem ser executados sem confirmação.
- Confirmação é necessária apenas para: commits, pushes, criação/exclusão de branches, escrita em arquivos e comandos destrutivos.

## Subdiretórios — contexto rápido

- `playout/CLAUDE.md` — arquitetura obrigatória, regras de áudio, estados, comandos
- `library/` — serviço de catálogo; SQLite; REST API para tracks, playlists e breaks
- `player/` — vazio por enquanto; futuro Electron app

## Regras de Git

- **Nunca fazer commit nem push diretamente na branch `main`.** Todo trabalho deve ser feito em uma branch dedicada e integrado via Pull Request.
- **Nunca criar branches sem aprovação prévia.** Sempre sugerir o nome e propósito da branch e aguardar confirmação antes de criá-la.
- **Nunca executar commit ou push sem solicitação explícita do usuário.** Aplicar as alterações nos arquivos e aguardar o comando.
