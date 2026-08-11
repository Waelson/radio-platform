# Plano — Tela de Catálogo

**Branch:** `feature/catalogo`
**Data:** 2026-07-22

## Objetivo

Adicionar um novo ícone "Catálogo" no rail lateral do player que, ao ser clicado,
substitui a área de conteúdo pela tela de Catálogo — inicialmente vazia, apenas
para validar o padrão visual e de navegação.

---

## Referência de comportamento

**Arquivo:** `player/player-v2.html` — item "Produção" no rail.

### Como funciona no player-v2

1. Cada botão do rail possui `data-view="<nome>"`:
   ```html
   <button class="rail-btn" data-view="production">
     <span class="ico">▦</span><span class="lbl">Produção</span>
   </button>
   ```

2. Cada tela é uma `<section class="workspace" id="view-<nome>">`.
   Apenas a tela ativa tem a classe `active` (visível); as demais ficam ocultas.

3. A função `setView(view)` gerencia a troca:
   ```javascript
   function setView(view, push = true) {
     document.querySelectorAll('.workspace').forEach(w => w.classList.remove('active'))
     document.querySelector(`#view-${view}`)?.classList.add('active')
     document.querySelectorAll('.rail-btn[data-view]').forEach(b =>
       b.classList.toggle('active', b.dataset.view === view)
     )
     if (push) localStorage.setItem('rfView', view)
   }
   ```

4. Os botões do rail disparam `setView` no click:
   ```javascript
   document.querySelectorAll('.rail-btn[data-view]').forEach(btn =>
     btn.addEventListener('click', () => setView(btn.dataset.view))
   )
   ```

5. Na inicialização, restaura a última view salva (ou `'onair'` como padrão):
   ```javascript
   setView(localStorage.getItem('rfView') || 'onair', false)
   ```

---

## Estado atual do player.html

| Aspecto | Situação hoje |
|---|---|
| Rail | Existe (`.rail`), mas botões usam `onclick` hardcoded, sem `data-view` |
| "No ar" | Sempre ativo, hardcoded via classe `.active` no HTML |
| "Produção" | Abre `advModalOpen()` (modal/drawer), não troca de tela |
| Sistema de views | Não existe — tela única dentro de `.main-cols` |

---

## Fases

### Fase 1 — Visual: novo ícone no rail + tela vazia (esta fase)

**Objetivo:** validar visualmente o padrão de navegação. Nenhum conteúdo real.

**O que muda em `player/player.html`:**

#### 1a — CSS: adicionar `.workspace`

```css
/* Views (troca de tela pelo rail) */
.workspace         { display: none; flex: 1; min-height: 0; overflow: hidden; }
.workspace.active  { display: flex; }
```

#### 1b — HTML: envolver a tela atual em `#view-onair`

O bloco `.main-cols` (que contém toda a UI "No ar") é envolto em:
```html
<section class="workspace active" id="view-onair">
  <!-- conteúdo atual: .main-cols + tudo que está dentro de .content-wrap -->
</section>
```

#### 1c — HTML: adicionar `#view-catalogo` (vazia)

```html
<section class="workspace" id="view-catalogo">
  <div class="catalogo-empty-state">
    <span class="catalogo-icon">◫</span>
    <p class="catalogo-title">Catálogo</p>
    <p class="catalogo-sub">Em construção</p>
  </div>
</section>
```

CSS do empty state:
```css
.catalogo-empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; flex: 1; gap: 10px;
  color: rgba(255,255,255,.2);
}
.catalogo-icon { font-size: 48px; }
.catalogo-title { font-size: 18px; font-weight: 700; margin: 0; }
.catalogo-sub   { font-size: 13px; margin: 0; }
```

#### 1d — HTML: atualizar botões do rail

```html
<!-- antes -->
<button class="rail-btn active" title="No ar">
<button class="rail-btn" title="Produção" onclick="advModalOpen()">

<!-- depois -->
<button class="rail-btn" data-view="onair" title="No ar">
<button class="rail-btn" data-view="producao" title="Produção" onclick="advModalOpen()">
<button class="rail-btn" data-view="catalogo" title="Catálogo">
  <span class="ico">◫</span><span class="lbl">Catálogo</span>
</button>
```

> Nota: "Produção" mantém o `onclick="advModalOpen()"` por enquanto —
> o botão usa `data-view="producao"` apenas para que o estado visual do rail
> seja atualizado; o modal é aberto como antes.
> O botão "No ar" perde o hardcoded `.active` do HTML — passa a ser controlado
> pelo `setView`.

#### 1e — JS: adicionar `setView` e inicialização

```javascript
// ── View switching (rail) ─────────────────────────────────────────────
function setView(view, push = true) {
  document.querySelectorAll('.workspace').forEach(w => w.classList.remove('active'))
  document.querySelector('#view-' + view)?.classList.add('active')
  document.querySelectorAll('.rail-btn[data-view]').forEach(b =>
    b.classList.toggle('active', b.dataset.view === view)
  )
  if (push) localStorage.setItem('rfView', view)
}

// Inicializa na view salva ou "onair"
setView(localStorage.getItem('rfView') || 'onair', false)

// Bind dos botões data-view que não têm ação própria
document.querySelectorAll('.rail-btn[data-view]').forEach(btn => {
  if (!btn.onclick) btn.addEventListener('click', () => setView(btn.dataset.view))
})
```

> Botões com `onclick` já definido (como "Produção") não recebem o listener
> do `setView` — evita conflito. O highlight do rail é atualizado pelo `setView`
> quando o usuário clicar em "Catálogo".

---

### Fase 2 — Conteúdo: busca e listagem do catálogo

*(a definir após validação visual da Fase 1)*

Ideias iniciais:
- Campo de busca por título, artista, tipo
- Tabela com resultados (tracks do library service)
- Preview de áudio ao clicar
- Botão "Inserir na fila" por item

---

## Critérios de aceite da Fase 1

- Clicar em "Catálogo" no rail → tela principal some, aparece tela vazia com
  ícone e label "Em construção".
- Clicar em "No ar" no rail → volta para a tela principal normalmente.
- O botão ativo no rail fica destacado (`.active`) a cada troca.
- A última view ativa é restaurada ao recarregar o app.
- "Produção" continua abrindo o modal como antes.
- Nenhuma regressão na tela "No ar".
