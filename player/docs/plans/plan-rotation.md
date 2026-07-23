# Plano — Tela de Rotação (Clock Rotation)

**Branch:** `feature/clock-rotation`
**Data:** 2026-07-22

## Objetivo

Migrar a funcionalidade de Clock Rotation do drawer da versão antiga do player
(`player.html.bak-playnow`) para uma nova tela workspace no rail do `player.html`
atual, seguindo o mesmo padrão visual e de navegação já estabelecido pela tela
de Catálogo.

---

## Referência de implementação

| Fonte | Localização | Uso |
|---|---|---|
| Lógica JS completa | `player.html.bak-playnow` linhas 6009–6743 | Reutilizar integralmente |
| CSS dos componentes | `player.html.bak-playnow` linhas 1079–1172 | Reutilizar integralmente |
| HTML dos panes | `player.html.bak-playnow` linhas 2499–2641 | Reutilizar com ajuste de container |
| Endpoints do backend | `library/docs/plans/plan-clock-rotation.md` | Já implementados |

---

## Diferenca entre a versao antiga e a nova

| Aspecto | Versao antiga (bak) | Nova implementacao |
|---|---|---|
| Container | Drawer — aba `rotacao` (250px fixo) | Workspace full-width (`#view-rotacao`) |
| Ativacao | `drwOpenTab('rotacao')` → `rotInit()` | `setView('rotacao')` → `rotInit()` |
| Inicializacao | Condicional por tab do drawer | Condicional por view no rail |
| Reinit apos auth | Via `_libOnline` watcher | Via `onAuthSuccess()` (mesmo padrao do catalogo) |
| Sub-navegacao | `.rot-subnav` dentro do drawer | `.rot-subnav` dentro do workspace |

O JS e o HTML dos panes sao identicos — apenas o wrapper muda.

---

## Fases

### Fase 1 — CSS: adicionar estilos da rotacao

Copiar os blocos de CSS do `player.html.bak-playnow` para `player.html`:

**Blocos a copiar (linhas da bak):**

- Linha 823: `#rotPaneGerar .rot-btn-sm:hover...`
- Linhas 1079–1172: todo o bloco `/* Drawer — Aba Rotacao */` ate `.rot-gen-enqueue-bar`

Os seletores CSS sao independentes do container — nenhuma alteracao necessaria.

### Fase 2 — HTML: adicionar `#view-rotacao` no workspace

Inserir apos o `<section id="view-catalogo">` existente:

```html
<section class="workspace" id="view-rotacao">
  <div class="rot-panel" style="display:flex;">

    <!-- Sub-navegacao -->
    <div class="rot-subnav">
      <button class="rot-subbtn active" id="rotTabCategorias"  onclick="rotSwitchPane('categorias')">Categorias</button>
      <button class="rot-subbtn"        id="rotTabClocks"      onclick="rotSwitchPane('clocks')">Clocks</button>
      <button class="rot-subbtn"        id="rotTabGrade"       onclick="rotSwitchPane('grade')">Grade</button>
      <button class="rot-subbtn"        id="rotTabRegras"      onclick="rotSwitchPane('regras')">Regras</button>
      <button class="rot-subbtn"        id="rotTabGerar"       onclick="rotSwitchPane('gerar')">Gerar</button>
    </div>

    <!-- Pane: Categorias -->
    <div class="rot-pane active" id="rotPaneCategorias">
      <!-- [conteudo identico ao bak, linhas 2509–2531] -->
    </div>

    <!-- Pane: Clocks -->
    <div class="rot-pane" id="rotPaneClocks">
      <!-- [conteudo identico ao bak, linhas 2533–2552] -->
    </div>

    <!-- Pane: Grade 24x7 -->
    <div class="rot-pane" id="rotPaneGrade" style="position:relative;">
      <!-- [conteudo identico ao bak, linhas 2555–2570] -->
    </div>

    <!-- Pane: Regras de separacao -->
    <div class="rot-pane" id="rotPaneRegras">
      <!-- [conteudo identico ao bak, linhas 2573–2597] -->
    </div>

    <!-- Pane: Gerar playlist -->
    <div class="rot-pane" id="rotPaneGerar" style="flex-direction:column;">
      <!-- [conteudo identico ao bak, linhas 2600–2640] -->
    </div>

    <!-- Floating clock picker (grade) -->
    <div id="rotGridPicker" style="display:none;position:fixed;z-index:9999;...">
      <div id="rotGridPickerList"></div>
    </div>

  </div>
</section>
```

### Fase 3 — HTML: adicionar botao no rail

Inserir apos o botao `data-view="catalogo"` no `.rail`:

```html
<button class="rail-btn" data-view="rotacao" title="Rotacao">
  <span class="ico">&#x27F3;</span><span class="lbl">Rotacao</span>
</button>
```

O icone `&#x27F3;` (⟳) representa rotacao/ciclo. Alternativas discutiveis:
- `&#x21BB;` (↻) — seta circular horaria
- `&#x1F501;` (🔁) — loop (emoji, evitar se possivel)
- `&#x2941;` (⥁) — ciclo com seta
- Texto: `ROT` em vez de icone

### Fase 4 — JS: adicionar funcoes de rotacao

Copiar integralmente o bloco JS do `player.html.bak-playnow` linhas 6009–6743
para `player.html`, logo apos o bloco de funcoes do Catalogo.

**Variaveis de estado a copiar:**

```javascript
let _rotInited      = false;
let _rotCategories  = [];
let _rotClocks      = [];
let _rotGrid        = [];
let _rotRules       = [];
let _rotGenItems    = [];
let _rotPane        = 'categorias';

const DAYS = ['Dom','Seg','Ter','Qua','Qui','Sex','Sab'];
```

**Funcoes a copiar (sem alteracao de logica):**

| Funcao | Responsabilidade |
|---|---|
| `rotInit()` | Inicializa uma vez, restaura subpane salvo |
| `rotSwitchPane(pane)` | Troca sub-aba, salva em localStorage |
| `_rotRefreshPane(pane)` | Dispara load correto por pane ativo |
| `rotCatLoad()` | GET `/v1/categories` |
| `rotCatRender()` | Renderiza lista de categorias |
| `rotCatToggle(id)` | Expande/colapsa card de categoria |
| `rotCatShowNew()` / `rotCatHideNew()` | Toggle form de nova categoria |
| `rotCatCreate()` | POST `/v1/categories` |
| `rotCatDelete(id, name)` | DELETE `/v1/categories/:id` |
| `rotCatRemoveTrack(catId, trackId, btn)` | DELETE `/v1/categories/:id/tracks/:trackId` |
| `rotCatSearch(catId)` | GET `/v1/tracks?q=...` para busca inline |
| `rotCatAddTrack(catId, trackId, btn)` | POST `/v1/categories/:id/tracks` |
| `rotClockLoad()` | GET `/v1/clocks` |
| `rotClockRender()` | Renderiza lista de clocks com slots |
| `rotSlotTypeChanged(clockId)` | Habilita/desabilita select de categoria por tipo |
| `rotClockToggle(id)` | Expande/colapsa card de clock |
| `rotClockShowNew()` / `rotClockHideNew()` | Toggle form de novo clock |
| `rotClockCreate()` | POST `/v1/clocks` |
| `rotClockDelete(id, name)` | DELETE `/v1/clocks/:id` |
| `rotClockRefreshSlots(clockId)` | GET `/v1/clocks/:id/slots` |
| `rotSlotAdd(clockId)` | POST `/v1/clocks/:id/slots` |
| `rotSlotDelete(clockId, slotId)` | DELETE `/v1/clocks/:id/slots/:slotId` |
| `rotGridLoad()` | GET `/v1/schedule/clock-grid` |
| `rotGridRender()` | Renderiza tabela 7x24 |
| `rotGridCellClick(weekday, hour, btnEl)` | Abre picker flutuante |
| `rotGridClosePicker()` | Fecha picker |
| `rotGridPickSelect(clockId)` | PUT `/v1/schedule/clock-grid` |
| `rotRulesLoad()` | GET `/v1/schedule/separation-rules` |
| `rotRulesRender()` | Renderiza lista de regras |
| `rotRuleShowNew()` / `rotRuleHideNew()` | Toggle form de nova regra |
| `rotRuleCreate()` | POST `/v1/schedule/separation-rules` |
| `rotRuleDelete(id)` | DELETE `/v1/schedule/separation-rules/:id` |
| `rotGenInitForm()` | Pre-preenche data/hora com valor atual |
| `rotGenUpdatePreview()` | Atualiza label de preview do periodo |
| `_rotBuildFromStr()` | Monta string ISO para o request |
| `rotGenerate()` | POST `/v1/schedule/generate` |
| `rotCueItem(idx)` | Abre cueFloat para preview de item gerado |
| `rotEnqueue()` | Enfileira itens gerados via POST `/v1/queue/enqueue` |

### Fase 5 — JS: integracao com setView e onAuthSuccess

**5a — Inicializacao via setView**

A funcao `setView` ja existe no player. Adicionar o gatilho de `rotInit()` dentro dela,
apos o mesmo padrao usado pelo catalogo:

```javascript
// dentro de setView(view, push), apos a troca de workspace:
if (view === 'rotacao' && typeof rotInit === 'function') rotInit();
```

**5b — Reload apos autenticacao**

Em `player/auth/login-overlay.js`, dentro de `onAuthSuccess()`, adicionar apos
o bloco do catalogo:

```javascript
if (document.querySelector('#view-rotacao.active')) {
  if (typeof rotInit === 'function') try { rotInit() } catch {}
  if (typeof _rotRefreshPane === 'function') try { _rotRefreshPane(_rotPane) } catch {}
}
```

**5c — Remover dependencia de `_libOnline`**

Na versao antiga, `_rotRefreshPane` era chamado quando `_libOnline` mudava de
`false` para `true`. Na nova versao, isso e tratado pelo `onAuthSuccess()`.
O bloco abaixo da bak pode ser removido ou mantido como fallback:

```javascript
// bak linha 4730 — pode ser omitido na nova versao:
if (!wasOnline && _libOnline && _rotInited) {
  _rotRefreshPane(_rotPane);
}
```

---

## Estrutura final dos arquivos modificados

```
player/
  player.html          — CSS + HTML + JS da rotacao
  auth/
    login-overlay.js   — onAuthSuccess com gatilho de rotacao
```

---

## Criterios de aceite

- [ ] Clicar em "Rotacao" no rail exibe a workspace `#view-rotacao`
- [ ] Sub-abas Categorias / Clocks / Grade / Regras / Gerar funcionam
- [ ] A sub-aba ativa e restaurada ao recarregar (localStorage `rotPane`)
- [ ] Criar, expandir e deletar categorias funciona
- [ ] Adicionar e remover faixas de uma categoria funciona
- [ ] Criar clocks e adicionar/remover slots funciona
- [ ] Grade 7x24 renderiza e permite atribuir clocks por celula
- [ ] Regras de separacao podem ser criadas e removidas
- [ ] Gerar playlist retorna lista de faixas para o periodo configurado
- [ ] Botao "Enfileirar no Player" insere todas as faixas na fila
- [ ] Botao CUE (🎧) abre cueFloat para preview de cada faixa gerada
- [ ] Login/refresh nao quebra a tela (onAuthSuccess recarrega)
- [ ] Nenhuma regressao nas telas "No ar" e "Catalogo"

---

## Notas de implementacao

1. **`DAYS` e `_DAYS_PT`** — a bak define `const DAYS` na linha 6019 e
   `const _DAYS_PT` na linha 6537. Verificar se `DAYS` ja existe no player.html
   antes de copiar para evitar redeclaracao.

2. **`escHtml`** — a funcao `escHtml()` ja existe no player.html atual. Nao
   re-declarar.

3. **`showToast`** — ja existe. Nao re-declarar.

4. **`LIBRARY_URL` e `PLAYOUT_URL`** — ja definidos no player.html. Nao
   re-declarar.

5. **`_libOnline`** — verificar se essa variavel existe no player.html atual.
   Se nao existir, substituir os guards `if (!_libOnline) return;` por
   `if (!sessionManager?.isAuthenticated()) return;`.

6. **`cueToggleQueueItem` e `_queueItemMap`** — usados em `rotCueItem()`.
   Ja existem no player.html.

7. **Icone SVG nos resultados de Gerar** — a bak usa `icons/vinheta.svg` etc.
   Verificar se esses assets existem no diretorio do player. Se nao existirem,
   substituir por badges de texto como no catalogo.
