# Plano — Aba Botoneira no Drawer

## Objetivo

Adicionar uma terceira aba **Botoneira** ao drawer lateral do `player.html`, ao lado das abas existentes **Playlists** e **Breaks**.

A janela flutuante `hotkeys.html` (Electron BrowserWindow) **não será removida nem alterada**. As duas formas de acesso coexistem.

---

## Estrutura atual do drawer (preservada)

```
┌─────────────────────────────────────────────────┬────┐
│  [icon] BIBLIOTECA                              │ ✕  │
└─────────────────────────────────────────────────┴────┘
│  [ Playlists ]  [ Breaks ]                           │  ← abas existentes
├──────────────────────────────────────────────────────┤
│  conteúdo da aba ativa                               │
└──────────────────────────────────────────────────────┘
```

---

## Layout após a mudança

```
┌─────────────────────────────────────────────────┬────┐
│  [icon] BIBLIOTECA                              │ ✕  │
└─────────────────────────────────────────────────┴────┘
│  [ Playlists ]  [ Breaks ]  [ Botoneira ]            │  ← nova aba adicionada
├──────────────────────────────────────────────────┬───┤
│                                                  │
│  (aba Botoneira ativa)                           │
│                                                  │
│  Perfil:  [ Manhã ▾ ]                            │  ← dropdown de perfis
│                                                  │
│  ┌────────────────────┐  ┌────────────────────┐  │
│  │  ▶                 │  │  ▶                 │  │
│  │  Vinheta Entrada   │  │  BG Music          │  │
│  │  00:12             │  │  01:45             │  │
│  └────────────────────┘  └────────────────────┘  │
│  ┌────────────────────┐  ┌────────────────────┐  │
│  │  ▶                 │  │  ▶                 │  │
│  │  Efeito 1          │  │  Spot A            │  │
│  │  00:05             │  │  00:30             │  │
│  └────────────────────┘  └────────────────────┘  │
│  ┌────────────────────┐  ┌────────────────────┐  │
│  │  ▶                 │  │  ▶                 │  │
│  │  ...               │  │  ...               │  │
│  └────────────────────┘  └────────────────────┘  │
│                                                  │
│  (grid 2 colunas, scrollável)                    │
└──────────────────────────────────────────────────┘
```

### Detalhe de um botão

```
┌────────────────────────────────────┐
│  ▶  (ícone de status)              │
│                                    │
│  Vinheta Entrada                   │  ← label (truncado)
│  00:12                             │  ← duração
└────────────────────────────────────┘
  cor de fundo = tipo do áudio
  (mesma paleta usada em hotkeys.html)
```

---

## Escopo

### O que será implementado

1. **Nova aba `Botoneira`** — adicionada à `.lib-tabs` existente, sem impactar as abas Playlists e Breaks.
2. **Novo painel `#libPanelBotoneira`** — estrutura igual aos outros painéis do drawer.
3. **Dropdown de perfis** — lista os perfis via `GET /v1/hotkeys/profiles`. Sem botões de criar/editar/excluir.
4. **Grid de 2 colunas fixas** — ignora o campo `columns` do perfil; sempre 2 colunas.
5. **Acionamento de botão** — dispara `CmdTriggerHotButton` via `POST /v1/commands` (mesmo endpoint da janela flutuante).
6. **Feedback visual** — botão pisca brevemente ao ser acionado.
7. **Scroll vertical** — mesmo padrão da fila de reprodução: scrollbar nativa oculta (`scrollbar-width: none` / `::-webkit-scrollbar { display: none }`), com scrollbar customizada posicionada absolutamente à direita (track + thumb arrastável, mesmas classes `.q-sb-track` / `.q-sb-thumb`).
8. **Persistência** — `localStorage` guarda o último perfil selecionado no drawer.

### O que NÃO será implementado

- Criar, editar ou excluir perfis
- Criar, editar ou excluir botões
- Qualquer alteração em `hotkeys.html`, `main.js` ou `preload-hotkeys.js`
- Alteração nas abas Playlists e Breaks

---

## APIs utilizadas

| Método | Endpoint | Uso |
|--------|----------|-----|
| `GET` | `/v1/hotkeys/profiles` | Listar perfis disponíveis |
| `GET` | `/v1/hotkeys/profiles/:id` | Carregar botões do perfil selecionado |
| `POST` | `/v1/commands` | Disparar `CmdTriggerHotButton` ao clicar |

---

## Estrutura de dados (referência de hotkeys.html)

```js
// Perfil
{ id, name, columns, buttons: [] }

// Botão
{ id, label, color, path, title, type, volume, duration_ms }
```

---

## Mudanças em player.html

### HTML
- Adicionar `<button class="lib-tab" id="libTabBotoneira" onclick="libSwitchTab('botoneira')">Botoneira</button>` na `.lib-tabs`
- Adicionar `<div class="lib-panel" id="libPanelBotoneira">` com dropdown de perfis e div do grid

### CSS
- `.drw-hk-select` — estilo do dropdown de perfil
- `.drw-hk-scroll-host` — container relativo (`position: relative; flex: 1; display: flex; overflow: hidden`) — espelho de `.queue-scroll-host`
- `.drw-hk-wrap` — scroll interno (`flex: 1; overflow-y: scroll; scrollbar-width: none; padding-right: 30px`) — espelho de `.queue-list-wrap`
- `.drw-hk-wrap::-webkit-scrollbar { display: none }` — oculta scrollbar nativa
- `.drw-hk-grid` — grid 2 colunas (`display: grid; grid-template-columns: 1fr 1fr; gap: 8px`)
- `.drw-hk-btn` — botão individual (visual idêntico ao `.hk-btn` de `hotkeys.html`)
- `.drw-hk-btn.playing` — estado de reprodução (pulso/highlight)
- Reutilizar `.q-sb-track` e `.q-sb-thumb` para a scrollbar customizada

### JavaScript
- `drwHkLoadProfiles()` — carrega perfis e popula o dropdown
- `drwHkSelectProfile(id)` — carrega botões e renderiza o grid
- `drwHkRenderGrid()` — renderiza os botões com 2 colunas
- `drwHkTrigger(btnId)` — dispara `CmdTriggerHotButton`
- Atualizar `libSwitchTab()` para incluir o case `'botoneira'` e carregar perfis na primeira abertura

---

## Fases de implementação

### Fase 1 — Estrutura HTML e abas
- Adicionar `<button class="lib-tab" id="libTabBotoneira" ...>Botoneira</button>` na `.lib-tabs`
- Adicionar `<div class="lib-panel" id="libPanelBotoneira">` com:
  - área do dropdown de perfil
  - container de scroll (`.drw-hk-scroll-host` > `.drw-hk-wrap` > `.drw-hk-grid`)
  - track e thumb da scrollbar customizada
- Atualizar `libSwitchTab()` para incluir o case `'botoneira'`

### Fase 2 — CSS
- `.drw-hk-scroll-host`, `.drw-hk-wrap`, `.drw-hk-wrap::-webkit-scrollbar`
- `.drw-hk-grid` (2 colunas)
- `.drw-hk-select` (dropdown de perfil)
- `.drw-hk-btn` e `.drw-hk-btn.playing` (visual idêntico ao `hotkeys.html`)

### Fase 3 — Carregamento de perfis e botões
- `drwHkLoadProfiles()` — `GET /v1/hotkeys/profiles`, popula dropdown
- `drwHkSelectProfile(id)` — `GET /v1/hotkeys/profiles/:id`, armazena botões
- Carregar perfis automaticamente na primeira abertura da aba
- Persistir último perfil selecionado em `localStorage`

### Fase 4 — Renderização do grid
- `drwHkRenderGrid()` — renderiza botões em 2 colunas com visual de `hotkeys.html`

### Fase 5 — Acionamento e feedback visual
- `drwHkTrigger(btnId)` — `POST /v1/commands` com `CmdTriggerHotButton`
- Feedback visual: botão pisca brevemente ao ser acionado (`.drw-hk-btn.playing`)

### Fase 6 — Scrollbar customizada
- Lógica JS de sincronização do thumb com o scroll do `.drw-hk-wrap`
- Suporte a drag do thumb (mesma lógica usada na fila de reprodução)
