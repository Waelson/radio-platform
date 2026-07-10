# Plano: Modal de Busca Avançada — Player UI ✅ IMPLEMENTADO

## Contexto

O drawer lateral de Biblioteca já oferece busca simples por texto e filtro por aba (Áudio / Playlists / Blocos). Este plano adiciona um **modal de busca avançada** acessível a partir da aba **Áudio** do drawer, sem remover nenhuma funcionalidade existente.

**Pré-requisito:** as fases do `library/docs/plans/plan-advanced-search.md` devem estar implementadas antes de iniciar a UI.

---

## Layout proposto

### Botão no drawer — aba Áudio

O botão "Busca Avançada" aparece no topo da aba Áudio, ao lado da barra de busca simples existente:

```
┌──────────────────────────────────────────────────────┐
│  BIBLIOTECA                                    [✕]   │
│                                                      │
│  [ Áudio ] [ Playlists ] [ Blocos ]                  │
│                                                      │
│  ┌────────────────────────────┐  ┌────────────────┐  │
│  │ 🔍 Buscar...               │  │ Busca Avançada │  │
│  └────────────────────────────┘  └────────────────┘  │
│                                                      │
│  Faixa A        Artista X   3:58  [◎] [+ Fila]      │
│  Faixa B        Artista Y   4:12  [◎] [+ Fila]      │
│  Faixa C        Artista Z   2:44  [◎] [+ Fila]      │
│  ...                                                 │
└──────────────────────────────────────────────────────┘
```

---

### Modal de busca avançada — layout completo

```
┌─────────────────────────────────────────────────────────────────────┐
│                                                                     │
│  BUSCA AVANÇADA DE ÁUDIO                                      [✕]  │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                     │
│  ┌──────────────────────────┐  ┌──────────────────────────────┐    │
│  │ NOME / TÍTULO            │  │ TIPO                         │    │
│  │ ┌────────────────────┐   │  │ ┌──────────────────────────┐ │    │
│  │ │ 🔍 ex: João...     │   │  │ │ Todos os tipos        ▾  │ │    │
│  │ └────────────────────┘   │  │ └──────────────────────────┘ │    │
│  └──────────────────────────┘  └──────────────────────────────┘    │
│                                                                     │
│  ┌──────────────────────────┐  ┌──────────────────────────────┐    │
│  │ ARTISTA                  │  │ ÁLBUM                        │    │
│  │ ┌────────────────────┐   │  │ ┌────────────────────────┐   │    │
│  │ │ ex: Artista X...   │   │  │ │ ex: Greatest Hits...   │   │    │
│  │ └────────────────────┘   │  │ └────────────────────────┘   │    │
│  └──────────────────────────┘  └──────────────────────────────┘    │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  [  Buscar  ]                          [ Limpar filtros ]    │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ─────────────────────────────────────────────────────────────────  │
│  RESULTADOS  •  127 faixas encontradas                              │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ TÍTULO              ARTISTA       ÁLBUM    TIPO   DUR  AÇÕES │  │
│  │ ─────────────────────────────────────────────────────────── │  │
│  │ Faixa Exemplo A     Artista X     Album Y  MUSIC  3:58       │  │
│  │                                              [◎ Preview]     │  │
│  │                                              [+ Fila]        │  │
│  │                                              [⠿ Arrastar]    │  │
│  │ ─────────────────────────────────────────────────────────── │  │
│  │ Faixa Exemplo B     Artista Z     —          JINGLE 1:12     │  │
│  │                                              [◎ Preview]     │  │
│  │                                              [+ Fila]        │  │
│  │                                              [⠿ Arrastar]    │  │
│  │ ─────────────────────────────────────────────────────────── │  │
│  │ Faixa Exemplo C     Artista W     Album K   SPOT   0:30      │  │
│  │                                              [◎ Preview]     │  │
│  │                                              [+ Fila]        │  │
│  │                                              [⠿ Arrastar]    │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  [ ← Anterior ]   Página 1 de 7   [ Próxima → ]                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Arquitetura do fluxo

```
[Botão "Busca Avançada" no drawer]
        │
        ▼
 Modal abre com campos vazios
        │
        ▼
 Operador preenche filtros (nome, tipo, artista, álbum)
        │
        ▼
 [Buscar] → GET /v1/tracks?q=...&type=...&artist=...&album=...&limit=20&offset=0
        │
        ├── Renderiza lista de resultados com título, artista, álbum, tipo, duração
        │
        ├── [◎ Preview] → mesmo mecanismo do drawer atual (POST /v1/preview/play)
        │
        ├── [+ Fila]    → POST /v1/queue/enqueue (mesmo do drawer atual)
        │
        └── [⠿ Arrastar] → drag-and-drop para posição específica na fila principal
```

---

## Paginação

- **Página**: 20 itens por página (configurável como constante JS)
- **Controles**: botões "← Anterior" / "Próxima →" + indicador "Página X de Y"
- **Total**: lido do header `X-Total-Count` ou campo `total` na resposta do Library Service
- **Offset**: `(página - 1) * limite`
- **Reset**: ao alterar qualquer filtro e clicar Buscar, volta para página 1

---

## Preview no modal

Reutiliza exatamente o mesmo mecanismo do drawer atual:
- `POST /v1/preview/play` com o path do áudio
- Eventos `PreviewProgress`, `PreviewStateChanged` via WebSocket já atualizam o painel de preview existente
- Apenas um preview ativo por vez — ao clicar `◎` em outro item, o anterior para automaticamente

---

## Drag-and-drop para posição na fila

O ícone `⠿` no resultado permite arrastar o item para uma posição específica da fila principal.

**Comportamento:**
- O modal NÃO fecha durante o drag
- A fila principal (col-player) aceita o drop entre itens (mesmo mecanismo de reorder já existente)
- Após o drop, o item é enfileirado na posição alvo via `POST /v1/queue/reorder-item` ou `POST /v1/queue/insert-after`
- Toast de confirmação: `✓ Faixa inserida na fila`

**Risco:** drag de dentro do modal para a fila exige que o modal não bloqueie eventos de `dragover` na fila. Solução: modal com `pointer-events` controlados ou overlay transparente durante drag.

---

## Riscos e mitigações

### 1. Busca lenta com milhares de faixas
**Mitigação:** índices no banco (`idx_tracks_title`, `idx_tracks_album`) — cobertos no plano do Library Service. Paginação server-side de 20 itens.

### 2. Conflito de preview: modal + drawer abertos simultaneamente
**Mitigação:** o estado do preview já é global (`_preview` object em `player.html`). Qualquer novo `◎` chama `previewPlay()` que para o anterior. Sem mudança necessária.

### 3. Drag do modal para a fila bloqueado pelo overlay do modal
**Mitigação:** usar `pointer-events: none` no backdrop do modal durante o drag (`dragstart` → `dragend`). Testar em Electron.

### 4. Modal fullscreen vs. fila visível para drop
**Mitigação:** modal em largura parcial (ex: 80% da tela) deixando a fila visível à direita, ou fechar o modal automaticamente ao iniciar o drag e manter o item "flutuando".

### 5. Total de resultados ausente na API atual
**Mitigação:** o Library Service precisa retornar `total` na resposta ou no header `X-Total-Count`. Isso é uma mudança adicional no handler de tracks.

---

## Endpoints consumidos

| Método | Endpoint | Quando |
|---|---|---|
| `GET` | `/v1/tracks?q=&type=&artist=&album=&limit=20&offset=N` | Ao clicar Buscar |
| `POST` | `/v1/preview/play` | Botão `◎ Preview` |
| `POST` | `/v1/queue/enqueue` | Botão `+ Fila` |
| `POST` | `/v1/queue/insert-after` | Drop em posição específica |

---

## Fases de implementação

### Fase 1 — Botão no drawer e esqueleto do modal

**1.1** Localizar o bloco da aba Áudio no drawer lateral em `player.html`.

**1.2** Adicionar botão `Busca Avançada` ao lado da barra de busca simples existente.

**1.3** Criar o elemento `<div id="advSearchModal" class="adv-modal hidden">` no final do `<body>`, com estrutura:
- Header (título + botão fechar)
- Seção de filtros (4 campos: nome, tipo, artista, álbum)
- Botões Buscar / Limpar
- Tabela de resultados (inicialmente vazia)
- Controles de paginação

**1.4** Adicionar CSS: `.adv-modal`, `.adv-backdrop`, `.adv-filters`, `.adv-results`, `.adv-pagination` — integrado ao tema escuro existente (`--bg`, `--bg2`, `--border`, `--cyan`).

**Entrega:** modal abre e fecha; campos visíveis; sem busca ainda.

---

### Fase 2 — Busca e renderização de resultados

**2.1** Declarar estado: `_advPage = 1`, `_advTotal = 0`, `_advFilters = {}`, `ADV_LIMIT = 20`.

**2.2** Implementar `advSearch()`:
- Lê os 4 campos de filtro
- Monta query string com `limit` e `offset`
- `GET` no Library Service (`LIBRARY_URL + '/v1/tracks?...'`)
- Chama `advRender(tracks, total)`

**2.3** Implementar `advRender(tracks, total)`:
- Limpa tabela de resultados
- Para cada faixa: linha com título, artista, álbum, tipo (badge colorido), duração formatada
- Botões `◎` e `+ Fila` por linha

**2.4** Implementar controles de paginação: `advPrev()`, `advNext()`, atualiza indicador "Página X de Y".

**2.5** Botão `Limpar filtros`: zera campos, reseta página, limpa tabela.

**Entrega:** busca funcional com filtros combinados e paginação.

---

### Fase 3 — Preview integrado

**3.1** No botão `◎` de cada linha de resultado, chamar `previewPlay()` com o `path` da faixa — exatamente o mesmo mecanismo do drawer.

**3.2** Validar que o painel de preview existente (abaixo do item no drawer ou painel inline) atualiza corretamente mesmo com o modal aberto.

**3.3** Highlight visual na linha cujo preview está ativo (classe CSS `adv-row--previewing`).

**Entrega:** preview funciona dentro do modal sem conflito com o drawer.

---

### Fase 4 — Enfileirar e arrastar

**4.1** Botão `+ Fila` por linha: chama `POST /v1/queue/enqueue` com o item — exibir toast de confirmação.

**4.2** Handle de drag `⠿` por linha:
- `draggable="true"` na linha ou no handle
- `dragstart`: armazena `path`, `title`, `type` etc. no `_dnd` global
- Ao iniciar drag: `pointer-events: none` no backdrop do modal para liberar a fila como drop target
- `dragend`: restaura `pointer-events`

**4.3** Garantir que a fila principal (`col-player`) aceita drops vindos do modal (o listener `dragover` / `drop` da fila já existe — verificar se o `_dnd.type` precisa de novo valor ex: `'search-result'`).

**Entrega:** operador pode arrastar resultado diretamente para posição na fila.

---

### Fase 5 — Total de resultados (requer mudança no Library Service)

**5.1** O Library Service precisa retornar o total de registros sem paginação. Estratégia: campo `total` no JSON de resposta ou header `X-Total-Count`.

**5.2** Na UI: ler o total, calcular número de páginas, exibir `"127 faixas encontradas"` e `"Página X de Y"`.

**Entrega:** paginação completa com indicadores corretos.

---

## Resumo de arquivos modificados

| Arquivo | Mudanças |
|---|---|
| `player/player.html` | Botão no drawer; HTML do modal; CSS do modal; JS: `advSearch()`, `advRender()`, `advPrev()`, `advNext()`, integração drag |
| `library/internal/store/track_store.go` | Campo `total` na resposta (Fase 5) |
| `library/internal/api/handlers/tracks.go` | Retornar `total` no JSON (Fase 5) |

---

## Checklist de validação

- [ ] Botão "Busca Avançada" visível na aba Áudio do drawer
- [ ] Modal abre ao clicar o botão; fecha com `✕` ou `Esc`
- [ ] Busca por nome retorna resultados corretos
- [ ] Busca por tipo filtra corretamente (incluindo EFEITOS)
- [ ] Busca por artista funciona isolada e combinada com outros filtros
- [ ] Busca por álbum funciona (pré-requisito: Library Service atualizado)
- [ ] Paginação avança e recua corretamente; "Página X de Y" correto
- [ ] `◎ Preview` toca o áudio sem fechar o modal
- [ ] `+ Fila` enfileira e exibe toast de confirmação
- [ ] Drag `⠿` insere o item na posição correta da fila principal
- [ ] Nenhum conflito visual com o drawer lateral quando ambos estão abertos
- [ ] Performance aceitável com 1000+ faixas (busca < 300ms)
