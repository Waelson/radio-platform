# Plano: Visual Brilhoso das Linhas da Fila

## Contexto

As linhas da fila (`queue-row`) têm atualmente um visual flat e discreto:
fundo escuro sólido `#0e1f23`, borda fina e quase invisível, sem profundidade
nem reflexo. O objetivo é elevar o visual para um estilo **glossy** — com
reflexo de luz no topo do card, borda iluminada, linha dourada na base e
badges de tipo mais vibrantes — inspirado na referência visual fornecida.

---

## Referência visual (elementos a capturar)

A partir da imagem de referência:

```
┌────────────────────────────────────────────────────────────────┐  ← borda iluminada (cyan/amber)
│░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░│  ← reflexo de luz (gradiente top)
│                                                                │
│  [Ícone]   Vinheta InfoAudio 03                  [Vinhetas]  🎧│
│                                                                │
│______________________________________________________________  │  ← linha dourada na base
└────────────────────────────────────────────────────────────────┘
```

**Características:**
1. **Reflexo de luz no topo** — gradiente semi-transparente branco/cyan na metade
   superior do card, simulando iluminação de cima para baixo (efeito glass)
2. **Borda iluminada** — borda com opacidade e glow mais intensos; borda-esquerda
   colorida por tipo (cyan para músicas, roxo para vinhetas, laranja para hora certa)
3. **Linha dourada na base** — faixa âmbar/dourada na borda inferior do card,
   funcionando como acento visual e separador
4. **Badge de tipo vibrante** — fundo sólido colorido com texto escuro (em vez do
   fundo quase transparente atual)
5. **Glow externo no item atual** — `box-shadow` com brilho cyan quando `is-current`
6. **Fundo com gradiente sutil** — gradiente do topo (ligeiramente mais claro) para
   a base (mais escuro), reforçando a profundidade

---

## Layout atual vs. novo

### Estado normal (QUEUED)

```
ATUAL:
┌─────────────────────────────────────────────────────────┐
│  background: #0e1f23 (sólido)                           │
│  border: 1px solid rgba(69,211,202,0.18)                │
│  border-left: 4px solid transparent                     │
└─────────────────────────────────────────────────────────┘

NOVO:
╔═════════════════════════════════════════════════════════╗  ← borda cyan 1px (mais viva)
║░░░░░░░░░░ reflexo glass (gradiente top 30%) ░░░░░░░░░░║
║                                                         ║
║  [idx]  Track Name              [badge tipo]  [dur]  ✕ ║
║                                                         ║
║▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄║  ← linha âmbar base (2px)
╚═════════════════════════════════════════════════════════╝
```

### Estado atual (PLAYING / is-current)

```
NOVO:
╔═════════════════════════════════════════════════════════╗  ← borda cyan intensa
║░░░░░░░░░░ reflexo glass mais vivo ░░░░░░░░░░░░░░░░░░░░║
║                                                         ║
║  [EQ]   Track Name              [ATUAL] [badge] [dur]  ║
║  ────────────────────────────── progress bar ────────── ║
║▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄║  ← linha âmbar mais intensa
╚═════════════════════════════════════════════════════════╝
   ↑ box-shadow glow externo cyan
```

---

## Estratégia técnica

Todas as mudanças são **exclusivamente CSS** — sem alteração no HTML gerado por
`renderItemRow()` nem no HTML de `renderBreakRows()`.

### Técnica do reflexo glass

Usar `background` com múltiplas camadas via gradiente:

```css
background:
  linear-gradient(180deg,
    rgba(255,255,255,0.04) 0%,    /* reflexo de luz no topo */
    rgba(255,255,255,0.00) 40%    /* fade para transparente */
  ),
  linear-gradient(180deg,
    #0e2428 0%,                    /* topo levemente mais claro */
    #09181b 100%                   /* base mais escura */
  );
```

### Técnica da linha âmbar na base

Usar `box-shadow` inset na base:

```css
box-shadow:
  inset 0 -2px 0 rgba(180, 130, 20, 0.55),   /* linha dourada base */
  0 1px 0 rgba(180, 130, 20, 0.15);           /* glow suave abaixo */
```

### Badge de tipo vibrante

Substituir o fundo quase transparente por um fundo sólido escuro com texto vivo,
por classe de tipo. Os tipos existentes são definidos no CSS via `.q-type.MUSIC`,
`.q-type.VINHETA` etc., mas o badge atual usa `.q-badge` genérico. A mudança
adiciona variantes por cor de tipo no próprio `.q-badge` herdando do `.queue-row`.

---

## Fases de implementação

---

### Fase 1 — Fundo gradiente + reflexo glass

**Objetivo:** substituir o fundo sólido por gradiente com reflexo de luz no topo.

**1.1** Substituir `background: #0e1f23` do `.queue-row` por multicamadas:

```css
.queue-row {
  background:
    linear-gradient(180deg, rgba(255,255,255,0.04) 0%, rgba(255,255,255,0) 38%),
    linear-gradient(180deg, #0e2428 0%, #091a1e 100%);
}
```

**1.2** Ajustar variantes por tipo (`hora-certa`, `vinheta`) com o mesmo padrão:

```css
.queue-row.hora-certa {
  background:
    linear-gradient(180deg, rgba(255,160,30,0.06) 0%, rgba(255,160,30,0) 38%),
    linear-gradient(180deg, #1c0e00 0%, #110900 100%);
}

.queue-row.vinheta {
  background:
    linear-gradient(180deg, rgba(122,77,255,0.08) 0%, rgba(122,77,255,0) 38%),
    linear-gradient(180deg, #130f25 0%, #0d0a1a 100%);
}
```

**1.3** Ajustar hover com leve aumento no reflexo:

```css
.queue-row:hover {
  background:
    linear-gradient(180deg, rgba(255,255,255,0.07) 0%, rgba(255,255,255,0) 38%),
    linear-gradient(180deg, #112b30 0%, #0b1e22 100%);
}
```

**Entrega:** cards com profundidade visual — topo ligeiramente iluminado,
base mais escura, sem linha dourada ainda.

---

### Fase 2 — Linha âmbar na base + borda iluminada

**Objetivo:** adicionar a linha dourada na base de cada card e intensificar a borda.

**2.1** Adicionar `box-shadow` no `.queue-row`:

```css
.queue-row {
  border: 1px solid rgba(69,211,202,0.28);
  box-shadow:
    inset 0 -2px 0 rgba(160, 115, 15, 0.50),
    inset 0  1px 0 rgba(255,255,255,0.04);
}
```

A `inset 0 -2px 0` cria a linha dourada na base dentro do próprio elemento.
A `inset 0 1px 0` reforça o brilho no topo, complementando o gradiente da Fase 1.

**2.2** Intensificar a borda esquerda colorida no item atual:

```css
.queue-row.is-current {
  border-left: 4px solid #2dd4c8;
  box-shadow:
    inset 0 -2px 0 rgba(45, 212, 200, 0.45),
    inset 0  1px 0 rgba(255,255,255,0.06),
    0 0 0 1px rgba(45,212,200,0.20),
    0 0 16px rgba(45,212,200,0.10);
}
```

**2.3** Ajustar linha âmbar nas variantes `hora-certa` e `vinheta`:

```css
.queue-row.hora-certa {
  box-shadow:
    inset 0 -2px 0 rgba(255, 120, 0, 0.55),
    inset 0  1px 0 rgba(255,160,30,0.06);
}

.queue-row.vinheta {
  box-shadow:
    inset 0 -2px 0 rgba(122, 77, 255, 0.50),
    inset 0  1px 0 rgba(122,77,255,0.06);
}
```

**Entrega:** linha colorida na base de cada card, condizente com o tipo do item.
Cards com profundidade e separação visual clara.

---

### Fase 3 — Badge de tipo vibrante

**Objetivo:** tornar o badge de tipo mais visível e colorido, com fundo sólido
escuro e texto vivo, como na referência.

**3.1** O `.q-badge` atual tem fundo quase transparente. Aumentar contraste:

```css
.q-badge {
  background: rgba(45,212,200,0.15);
  border: 1px solid rgba(45,212,200,0.35);
  color: #2dd4c8;
  font-weight: 900;
}
```

**3.2** Adicionar variantes por tipo de linha (herdadas do `.queue-row`):

```css
.queue-row.hora-certa .q-badge {
  background: rgba(255,110,0,0.20);
  border-color: rgba(255,110,0,0.45);
  color: #ff8c00;
}

.queue-row.vinheta .q-badge {
  background: rgba(122,77,255,0.18);
  border-color: rgba(122,77,255,0.40);
  color: #b39dff;
}
```

**3.3** Ajustar o `.q-status-pill` para ter o mesmo nível de contraste:

```css
.q-status-pill.live {
  background: rgba(45,212,200,0.15);
  border-color: rgba(45,212,200,0.35);
  font-weight: 800;
}

.q-status-pill.waiting {
  background: rgba(243,193,95,0.12);
  border-color: rgba(243,193,95,0.30);
  font-weight: 800;
}
```

**Entrega:** badges e pills mais legíveis e vibrantes, condizentes com o tipo.

---

### Fase 4 — Glow externo no item atual + hover refinado

**Objetivo:** destacar o item em reprodução com brilho externo suave e
refinar o hover para ser mais responsivo ao tema glossy.

**4.1** Intensificar o glow no `.queue-row.is-current`:

```css
.queue-row.is-current {
  background:
    linear-gradient(180deg, rgba(45,212,200,0.10) 0%, rgba(45,212,200,0) 40%),
    linear-gradient(180deg, #0f2e33 0%, #091e22 100%);
  border-color: rgba(45,212,200,0.55);
  box-shadow:
    inset 0 -2px 0 rgba(45,212,200,0.50),
    inset 0  1px 0 rgba(255,255,255,0.06),
    0 0 0 1px rgba(45,212,200,0.18),
    0 0 20px rgba(45,212,200,0.08);
}
```

**4.2** Ajustar hover para dar feedback glass consistente:

```css
.queue-row:hover {
  background:
    linear-gradient(180deg, rgba(255,255,255,0.07) 0%, rgba(255,255,255,0) 40%),
    linear-gradient(180deg, #112b30 0%, #0b1e22 100%);
  border-color: rgba(69,211,202,0.38);
}
```

**4.3** Garantir que `.is-played` e `.is-failed` mantenham opacidade reduzida
sem perder o efeito glass (a opacidade `0.38` já existe e funciona como overlay).

**Entrega:** item em reprodução visualmente destacado com glow suave; hover
responsivo e consistente com o estilo glass do card.

---

## Resumo de arquivos modificados

| Arquivo | Fase | Mudanças |
|---|---|---|
| `player/player.html` | 1 | CSS: `.queue-row` com background gradiente + reflexo glass; variantes por tipo |
| `player/player.html` | 2 | CSS: `box-shadow` inset para linha âmbar na base; borda iluminada; variantes |
| `player/player.html` | 3 | CSS: `.q-badge` e `.q-status-pill` mais vibrantes; variantes por tipo |
| `player/player.html` | 4 | CSS: glow externo `.is-current`; hover refinado |

Nenhum arquivo novo. Nenhuma mudança no JS ou no HTML gerado dinamicamente.

---

## Riscos e mitigações

### 1. Box-shadow inset conflita com border-left colorida

**Risco:** `box-shadow` e `border-left` competem visualmente na borda esquerda,
podendo criar artefatos ou sobreposição estranha.

**Mitigação:** a `border-left` ocupa 4px fora do `box-shadow` (que é `inset`).
Não há sobreposição — são camadas distintas. Validar visualmente no Electron.

---

### 2. Múltiplos gradientes em `background` — performance

**Risco:** múltiplas camadas de gradiente em dezenas de itens na fila podem
impactar performance de renderização.

**Mitigação:** em Chromium (Electron), gradientes CSS são renderizados na GPU
e têm custo negligenciável em listas de até algumas centenas de itens. A fila
raramente excede 50 itens na prática de uma rádio.

---

### 3. Contraste dos badges em diferentes tipos pode ficar fraco

**Risco:** a combinação fundo + texto pode não atingir contraste suficiente para
leitura rápida em cabine (luz ambiente variável).

**Mitigação:** os valores de cor das fases 3 foram escolhidos com contraste
explícito fundo/texto. Validar em tela real e ajustar opacidades se necessário.

---

### 4. `.is-played` e `.is-failed` perdem identidade visual

**Risco:** a opacidade `0.38` em `.is-played` reduz o efeito glass a ponto de
ficar indistinguível de um card normal em telas escuras.

**Mitigação:** a opacidade permanece como está (comportamento intencional —
itens tocados devem recuar visualmente). O efeito glass fica proporcional.

---

## Checklist de validação

### Fase 1
- [ ] Cards com gradiente sutil topo→base (topo levemente mais claro)
- [ ] Reflexo glass visível no topo de cada card
- [ ] Variantes `hora-certa` e `vinheta` com reflexo na cor do tipo

### Fase 2
- [ ] Linha âmbar/dourada visível na base de cada card padrão
- [ ] Linha muda de cor para cyan em `is-current`, laranja em `hora-certa`, roxo em `vinheta`
- [ ] Borda esquerda colorida mais intensa em `is-current`

### Fase 3
- [ ] Badge de tipo (`.q-badge`) mais contrastado e colorido
- [ ] Status pill (`Na fila`, `Tocando`) mais legível
- [ ] Variantes de badge por tipo funcionando corretamente

### Fase 4
- [ ] Item atual (`is-current`) tem glow externo suave visível
- [ ] Hover de cards normais responsivo e glass
- [ ] Cards `is-played` mantêm opacidade reduzida (recuam sem sumir)
