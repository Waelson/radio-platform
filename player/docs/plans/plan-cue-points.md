# Plano de Implementação — Marcadores de Intro / Outro / Cue Points

**Data:** julho de 2026
**Branch:** `feature/cue-points`
**Status:** Em planejamento

---

## 1. Por que esta feature é importante — problemas de negócio que resolve

### 1.1 O problema central

Em uma emissora de rádio profissional, a qualidade percebida pelo ouvinte depende de transições precisas entre os elementos da programação. Sem marcadores, o RadioFlow opera com regras fixas e cegas: o crossfade inicia sempre a N segundos do fim, a hora certa é agendada com margem de segurança empírica, e o locutor nunca sabe com certeza quantos segundos de intro instrumental tem antes da letra começar.

Isso causa cinco problemas concretos e mensuráveis:

**Problema 1 — Crossfade em cima da letra.**
O engine inicia o fade de saída em ponto fixo (ex.: 5 s antes do fim). Se a música tem letra até o último segundo, o fade corta a voz do cantor. Ouvintes percebem e reclamam. Programadores de rádio chamam isso de "matar a letra" — é um sinal imediato de automação amadora.

**Problema 2 — Hora Certa fora do segundo.**
A hora certa (ex.: "São exatamente 10 horas") deve tocar exatamente na virada do minuto. Hoje o engine usa uma janela de tolerância, o que significa que a hora certa pode entrar 1–3 segundos antes ou depois do momento exato. Em emissoras formais (all-news, esportiva, jornalística), isso é inaceitável.

**Problema 3 — Locutor fala demais ou de menos no intro.**
O locutor precisa saber exatamente quantos segundos de intro instrumental a música tem para "falar sobre a música" sem sobrepor a letra. Sem essa informação exibida na tela, o locutor usa intuição — e frequentemente erra, gerando silêncio desnecessário ou sobreposição com a voz do cantor.

**Problema 4 — Silêncio inicial nos arquivos.**
Muitos arquivos MP3 têm silêncio no início por artefato de encoding (especialmente LAME com gapless). Sem cue in, o engine toca esse silêncio para o ar. O ouvinte percebe como uma "falha" entre as músicas.

**Problema 5 — Triagem de catálogo lenta.**
Sem marcadores de intro, o programador precisa ouvir cada música do início para descobrir onde a letra começa. Com um catálogo de centenas de músicas novas por mês, isso consome horas de trabalho manual.

### 1.2 Impacto no negócio

| Problema | Impacto sem marcadores | Com marcadores |
|----------|----------------------|----------------|
| Crossfade cortando letra | Percepção de amadorismo; insatisfação do ouvinte | Transições musicalmente corretas |
| Hora certa imprecisa | Credibilidade editorial comprometida | Pontualidade ao segundo |
| Locutor sem countdown | Erros de locução ao vivo | Operação ao vivo confiável |
| Silêncio inicial | "Falha" audível entre músicas | Reprodução limpa desde o primeiro sample |
| Triagem manual | Horas de trabalho por lote de importação | Triagem em segundos com auto-detecção |

---

## 2. Requisitos de negócio

### 2.1 Funcionais obrigatórios

**RN-01** — O sistema deve suportar quatro marcadores por faixa: `cue_in`, `intro`, `outro` e `cue_out`, todos em milissegundos.

**RN-02** — O operador deve poder editar os marcadores manualmente via editor visual com waveform.

**RN-03** — O sistema deve detectar automaticamente o `cue_in` (fim do silêncio inicial) via análise de áudio na importação.

**RN-04** — O Playout Engine deve usar `cue_in` como ponto de início da reprodução (seek inicial).

**RN-05** — O Playout Engine deve iniciar o crossfade no `outro` em vez de tempo fixo antes do fim.

**RN-06** — O sistema deve exibir um contador regressivo de intro no painel "Now Playing" enquanto a faixa atual tem intro não concluído.

**RN-07** — O sistema deve usar `cue_out` para sincronizar a Hora Certa: o engine calcula com antecedência o momento exato em que o fade deve iniciar para que a hora certa entre na virada do minuto.

**RN-08** — Os marcadores devem ser persistidos no banco de dados do Library Service, não no arquivo de áudio.

**RN-09** — Faixas sem marcadores devem continuar funcionando com o comportamento atual (fallback para lógica de tempo fixo).

### 2.2 Funcionais desejáveis

**RN-10** — Sugestão automática de `intro` via análise de envelope de áudio (detecção de onset vocal).

**RN-11** — Atalho de teclado para acionar o editor de cue points a partir da lista de faixas.

**RN-12** — Exibição da duração do intro na lista de faixas do catálogo (coluna "Intro").

**RN-13** — Evento WebSocket `IntroCountdown` publicado pelo engine a cada segundo durante o intro.

### 2.3 Não funcionais

**RNF-01** — A leitura dos marcadores não deve adicionar latência perceptível ao início da reprodução (máximo 10 ms adicional).

**RNF-02** — O editor de waveform deve renderizar faixas de até 60 minutos sem degradação de performance.

**RNF-03** — Marcadores devem ser preservados quando a faixa é re-analisada para loudness.

---

## 3. Fluxo de utilização

### 3.1 Fluxo de configuração inicial (programador)

```
1. Programador abre a aba "Catálogo" no Player
2. Seleciona uma faixa na lista
3. Clica no botão de edição de cue points [CUE] ou pressiona a tecla C
4. Modal "Editor de Cue Points" abre com a waveform da faixa
5. Sistema sugere automaticamente cue_in (detectado na importação)
6. Programador ajusta visualmente os marcadores arrastando as linhas:
   - Linha azul: cue_in (onde a reprodução começa)
   - Linha verde: intro (onde a letra / voz começa)
   - Linha laranja: outro (onde o fade de saída deve iniciar)
   - Linha vermelha: cue_out (onde a reprodução termina)
7. Programador clica "Salvar"
8. Library Service persiste os marcadores via PUT /v1/tracks/:id/cuepoints
9. Modal fecha
```

### 3.2 Fluxo de reprodução automática (engine)

```
1. Operador inicia reprodução / faixa entra na fila
2. Player consulta Library Service: GET /v1/tracks/:id → recebe marcadores
3. Player envia faixa ao Playout Engine com cue_in, intro, outro, cue_out no payload
4. Engine faz seek para cue_in ao decodar o arquivo
5. Engine publica evento WebSocket "ItemStarted" com intro_ms (duração do intro)
6. Player exibe contador regressivo de intro no painel Now Playing
7. Engine publica "IntroCountdown" a cada segundo até intro=0
8. Quando posição atinge outro_ms, engine inicia crossfade para a próxima faixa
9. Reprodução para em cue_out_ms (ou no fim real do arquivo, o que vier primeiro)
```

### 3.3 Fluxo de auto-detecção na importação

```
1. Novo arquivo detectado pelo watch folder
2. Library Service extrai metadados (ID3, duração)
3. Library Service roda ffmpeg silencedetect no arquivo
4. Detecta fim do silêncio inicial → grava como cue_in_ms sugerido
5. Faixa entra no catálogo com cue_in_ms preenchido automaticamente
6. intro_ms, outro_ms e cue_out_ms ficam NULL (a definir pelo programador)
```

### 3.4 Fluxo de Hora Certa sincronizada

```
1. Evento de Hora Certa agendado para XX:00:00
2. Engine sabe que faixa atual tem cue_out_ms = 187500 e posição atual = 180000
3. Tempo restante de reprodução = (cue_out_ms - pos) / playback_rate = 7.5 s
4. Engine calcula: se Hora Certa é em 30 s e faixa dura mais 7.5 s,
   deve iniciar o fade agora e colocar a próxima faixa como Hora Certa
5. Com outro_ms definido, o crossfade começa no ponto musical correto
```

---

## 4. Como os concorrentes resolvem o problema

### 4.1 RCS Zetta (referência internacional premium)

O Zetta implementa o sistema mais completo do mercado. A ferramenta de marcação suporta **trim-in, intro, segue (ponto de crossfade), trim-out e volume points**. O módulo "Marks Analysis" detecta automaticamente trim-in, trim-out e segue points incluindo normalização de loudness por estação — tudo em batch.

Diferencial exclusivo do Zetta: **cue points não-áudio** — triggers que disparam ações externas (acender luz de estúdio, enviar MIDI, acionar relay de GPI) em momentos específicos da faixa. Esses triggers podem ser "travados" (`locked`) de forma que, quando o arquivo de áudio é substituído (ex.: noticiário atualizado a cada hora), o timestamp do trigger não precisa ser reconfigurado.

No painel on-air, o Zetta exibe o progresso da faixa em três cores: verde (tocando), amarelo (no intro — locutor pode falar), vermelho (finalizando).

### 4.2 mAirList (referência europeia profissional)

O mAirList tem um **Cue Editor gráfico** com visualização de waveform completa. Marcadores suportados:
- **Cue In:** ponto onde a reprodução começa (elimina silêncio inicial)
- **Ramp (Intro):** fim da intro instrumental; exibido como countdown no painel on-air
- **Fade Out:** ponto onde o engine inicia o fade automático
- **Cue Out:** ponto onde a reprodução para completamente
- **Anchor:** ponto especial para cálculos de backtiming — quando definido, o motor usa esse ponto (e não o início da faixa) como referência para calcular quando iniciar a reprodução de forma que o âncora coincida com um horário fixo (útil para hora certa e tops de hora)

O **Auto Cue** detecta silêncio inicial e final automaticamente na importação via análise de nível de áudio. Marcadores podem ser salvos em tags do arquivo (formatos selecionados) ou em arquivos XML de metadados externos.

Durante a transmissão, o mAirList exibe um **grande countdown regressivo** enquanto a faixa está no intro — o locutor vê claramente quanto tempo tem.

### 4.3 RadioBOSS (popular em emissoras médias)

O RadioBOSS define intro como "onde os vocais começam" e outro como "onde os vocais terminam". O ponto de crossfade é configurado separadamente por tipo de áudio (músicas vs. jingles). Os marcadores são armazenados em **tags APEv2** dentro do próprio arquivo MP3/FLAC, o que permite portabilidade entre estações.

Destaque: o **RadioForge** — ferramenta companion que usa IA de separação vocal (similar ao Spleeter/Demucs) para detectar automaticamente onde a voz começa e termina. Os resultados são gravados nas tags APEv2 que o RadioBOSS lê diretamente. Isso elimina completamente o trabalho manual de marcação para a maioria das músicas.

O countdown de intro aparece no painel do operador com destaque visual. A configuração de "Auto Intro" leva em conta os marcadores de início/fim de faixa para calcular o fading correto.

### 4.4 PlayIt Live (gratuito, popular em web rádios)

O PlayIt Live suporta quatro marcadores: **cue in, intro, early out/extro e cue out**. Adicionalmente, suporta **hook start e hook end** — uma região de alguns segundos que representa o "refrão" ou trecho mais reconhecível da música. Esse hook é usado para criar "teasers" automáticos: o sistema pode tocar automaticamente 15 segundos do hook de uma faixa futura antes de ela entrar na fila.

Marcadores em faixas migradas de outros sistemas (ex.: PlayoutONE) são importados automaticamente via mapeamento de banco de dados.

**Hard Fixed Time Markers** garantem que a próxima faixa comece exatamente em um horário determinado, independentemente de onde a faixa anterior estiver.

Os cue points desaparecem se o arquivo de áudio for movido ou renomeado — limitação conhecida documentada no FAQ oficial.

### 4.5 RadioDJ (open source, comunidade)

O RadioDJ armazena quatro marcadores em banco de dados (MySQL): **start, intro, outro e end**. Na importação, o sistema calcula automaticamente start e end via análise de duração, mas intro e outro devem ser definidos manualmente via CUE Editor.

Durante a transmissão, o painel on-air exibe:
- Countdown de intro (se intro definido) — "tempo que o locutor tem para falar"
- Countdown de outro (padrão: 10 s antes do fim) — "tempo para o locutor começar a falar sobre a próxima música"

O CUE Editor é um painel de campos numéricos — sem visualização de waveform. O operador digita os valores em segundos ou usa os botões "Set" para capturar a posição atual de reprodução.

### 4.6 RadioPro Prime (brasileiro)

O RadioPro Prime inclui um editor para **intro, refrão (chorus), identificação da emissora e pontos de início/fim**. A documentação menciona "marcadores para intro, refrão, identificação da emissora e pontos de início e fim de execução". A ferramenta é integrada ao módulo de gerenciamento de mídia e os marcadores são usados pelo motor de automação para sincronização de hora certa e crossfade por tipo de mídia.

### 4.7 Comparativo resumido

| Feature | RCS Zetta | mAirList | RadioBOSS | PlayIt Live | RadioDJ | RadioPro |
|---------|:---------:|:--------:|:---------:|:-----------:|:-------:|:--------:|
| Cue In (pulo de silêncio) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Intro (countdown locutor) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Outro / Segue (crossfade preciso) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Cue Out (fim de reprodução) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Editor visual com waveform | ✅ | ✅ | ✅ | ✅ | ✅ (básico) | ✅ |
| Auto-detecção de silêncio | ✅ | ✅ | ✅ | ✅ | Parcial | — |
| Auto-detecção de intro vocal | ✅ | — | ✅ (IA) | — | — | — |
| Hook / teaser region | — | — | — | ✅ | — | — |
| Triggers não-áudio | ✅ | — | — | — | — | — |
| Armazenamento em tags | — | Opcional | ✅ APEv2 | — | — | — |
| Backtiming (Anchor) | — | ✅ | — | ✅ (Hard Fixed) | — | — |
| Countdown on-air | ✅ (3 cores) | ✅ (grande) | ✅ | ✅ | ✅ | ✅ |

---

## 5. Proposta de solução

### 5.1 Visão geral

A solução é dividida em três componentes:

1. **Library Service** — armazena os marcadores por faixa, expõe API para leitura/escrita e detecta `cue_in` automaticamente na importação via `ffmpeg silencedetect`.

2. **Playout Engine** — consome os marcadores no `QueueItem`, faz seek para `cue_in`, usa `outro_ms` como ponto de disparo do crossfade, e publica eventos WebSocket de countdown de intro.

3. **Player (Electron)** — exibe o editor visual de cue points com waveform (via **wavesurfer.js**), mostra o countdown de intro no painel Now Playing e transmite os marcadores ao engine via payload de enqueue.

### 5.2 Marcadores definidos

| Marcador | Campo | Descrição | Auto-detectado |
|----------|-------|-----------|----------------|
| Cue In | `cue_in_ms` | Ponto de início da reprodução (pula silêncio inicial). Default: 0. | Sim (silencedetect) |
| Intro | `intro_ms` | Posição onde a letra/voz começa. Usado para countdown do locutor. | Não (manual) |
| Outro | `outro_ms` | Posição onde o crossfade de saída deve iniciar. | Não (manual) |
| Cue Out | `cue_out_ms` | Posição onde a reprodução para. Default: duração real da faixa. | Não (manual) |

### 5.3 Regras de fallback

- Se `outro_ms` for NULL → crossfade inicia no tempo fixo configurado (comportamento atual).
- Se `cue_in_ms` for NULL ou 0 → reprodução inicia do frame 0 (comportamento atual).
- Se `cue_out_ms` for NULL → reprodução termina no EOF real do arquivo.
- Se `intro_ms` for NULL → countdown de intro não é exibido.

### 5.4 Decisão técnica: wavesurfer.js

Para o editor visual de waveform no Electron, a biblioteca **wavesurfer.js** foi escolhida pelos seguintes critérios:

- Open source (BSD-3), sem custo.
- Renderização em Canvas — não usa DOM para cada sample, performático para arquivos longos.
- Plugin nativo de **Regions** — permite criar regiões arrastáveis que mapeiam diretamente para os 4 marcadores.
- Decodifica o áudio localmente via Web Audio API — não precisa de servidor para gerar waveform.
- Amplamente usado em projetos Electron e web apps de áudio profissional.
- API simples: `wavesurfer.addRegion({ start, end, color, drag, resize })`.

### 5.5 Auto-detecção de silêncio

Comando ffmpeg para detectar fim do silêncio inicial:

```bash
ffmpeg -hide_banner -vn -i "arquivo.mp3" \
  -af "silencedetect=n=-50dB:d=0.1" \
  -f null - 2>&1
```

Saída parseada:
```
[silencedetect @ 0x...] silence_end: 0.432 | silence_duration: 0.432
```

O valor `silence_end` em segundos × 1000 = `cue_in_ms`.

Threshold: `-50dB` (silêncio inicial de codificação é tipicamente abaixo de -60 dBFS; -50dB captura também ruídos de fita em gravações analógicas antigas sem falsos positivos em músicas com intro suave).

Duração mínima: `0.1 s` (100 ms) — evita detectar micro-silêncios entre notas como silêncio inicial.

---

## 6. Impacto na UI e telas novas

### 6.1 Painel Now Playing — countdown de intro

**Impacto:** adição de uma quarta linha na coluna esquerda do painel Now Playing (ao lado de STATUS, TIPO e ÁUDIO), visível apenas quando a faixa tem `intro_ms` definido e a posição atual ainda está antes do intro. O restante do painel permanece inalterado.

**Estado normal (sem intro definido ou intro já encerrado) — painel atual:**

```
┌──────────────────────┬──────────────────────────────────┬────────────────────┐
│  ⊙  STATUS  TOCANDO  │                                  │  RESTANTE          │
│                      │   LA ISLA BONITA                 │                    │
│  ◈  TIPO    MÚSICA   │   HOUSE AFRO                     │      02:37         │
│                      │                                  │                    │
│  ≈  ÁUDIO  48kHz/St  │                                  │  A SEGUIR          │
│                      │                                  │  LAMBADA           │
└──────────────────────┴──────────────────────────────────┴────────────────────┘
│  00:24 / 03:02  ●━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  13.3%       │
└────────────────────────────────────────────────────────────────────────────────┘
```

**Estado com intro ativo (amarelo — locutor tem tempo):**

```
┌──────────────────────┬──────────────────────────────────┬────────────────────┐
│  ⊙  STATUS  TOCANDO  │                                  │  RESTANTE          │
│                      │   LA ISLA BONITA                 │                    │
│  ◈  TIPO    MÚSICA   │   HOUSE AFRO                     │      02:37         │
│                      │                                  │                    │
│  ≈  ÁUDIO  48kHz/St  │                                  │  A SEGUIR          │
│                      │                                  │  LAMBADA           │
│  ▶  INTRO   00:12    │  ← linha nova, cor amarela       │                    │
└──────────────────────┴──────────────────────────────────┴────────────────────┘
│  00:05 / 03:02  ●━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━   2.8%       │
└────────────────────────────────────────────────────────────────────────────────┘
```

**Estado com intro urgente (vermelho — menos de 5 s):**

```
┌──────────────────────┬──────────────────────────────────┬────────────────────┐
│  ⊙  STATUS  TOCANDO  │                                  │  RESTANTE          │
│                      │   LA ISLA BONITA                 │                    │
│  ◈  TIPO    MÚSICA   │   HOUSE AFRO                     │      02:37         │
│                      │                                  │                    │
│  ≈  ÁUDIO  48kHz/St  │                                  │  A SEGUIR          │
│                      │                                  │  LAMBADA           │
│  ▶  INTRO   00:03    │  ← vermelho (urgência)           │                    │
└──────────────────────┴──────────────────────────────────┴────────────────────┘
│  00:14 / 03:02  ●━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━   7.7%       │
└────────────────────────────────────────────────────────────────────────────────┘
```

- Linha `▶  INTRO  MM:SS` aparece apenas quando `intro_ms` está definido E posição < `intro_ms`.
- Cor amarela enquanto restam mais de 5 s; muda para vermelho nos últimos 5 s.
- Quando a posição cruza `intro_ms`, a linha some e o painel volta ao visual de 3 linhas.

### 6.2 Lista de faixas no catálogo — coluna Intro

**Impacto:** adição da coluna "Intro" na tabela de faixas do catálogo, mostrando a duração da intro em formato `MM:SS`. Coluna opcional (toggle via preferências).

```
┌──────┬──────────────────────────────┬─────────┬───────┬──────────┐
│  #   │  Título / Artista            │ Duração │ Intro │  Tipo    │
├──────┼──────────────────────────────┼─────────┼───────┼──────────┤
│  1   │  Aquarela — Toquinho         │  3:42   │ 00:18 │  MUSIC   │
│  2   │  País Tropical — Jorge Ben   │  3:15   │  —    │  MUSIC   │ ← sem intro
│  3   │  Jingle Verão 2026           │  0:30   │ 00:04 │  JINGLE  │
└──────┴──────────────────────────────┴─────────┴───────┴──────────┘
```

- `—` indica que `intro_ms` não foi definido para a faixa.
- Ícone de lápis [CUE] ao final de cada linha para abrir o editor.

### 6.3 Modal — Editor de Cue Points (tela nova)

```
┌─────────────────────────────────────────────────────────────────────────┐
│  EDITOR DE CUE POINTS                                              [X]  │
│  Chega de Saudade — João Gilberto   (2:31)                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ◀ ▶  ⏹   Volume ───●─────  Zoom [ - ] ──────● [ + ]                 │
│                                                                         │
│  00:00        00:30        01:00        01:30        02:00        02:31 │
│  │                                                                   │  │
│  ▼ cue_in                                                             │  │
│  ╔══════════════════════════════════════════════════════════════════╗ │  │
│  ║▁▂▄▅▆▇█▇▆▅▃▂▁▁▂▃▄▅▆▇███▇▆▅▄▃▂▁▂▃▄▅▆▇▇▆▅▄▃▂▁▁▂▃▄▅▆▇▆▅▄▃▂▁▁▁▁▁▁▁║ │  │
│  ╚══════════════════════════════════════════════════════════════════╝ │  │
│       ▲ intro                        ▲ outro              ▲ cue_out    │
│                                                                         │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ │
│  │  CUE IN      │ │   INTRO      │ │   OUTRO      │ │   CUE OUT    │ │
│  │  00:00.432   │ │  00:18.200   │ │  02:15.000   │ │  02:31.000   │ │
│  │  [◀ CAPTURAR]│ │  [◀ CAPTURAR]│ │  [◀ CAPTURAR]│ │  [◀ CAPTURAR]│ │
│  │  [LIMPAR]    │ │  [LIMPAR]    │ │  [LIMPAR]    │ │  [LIMPAR]    │ │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘ │
│                                                                         │
│         [TOCAR DO CUE IN]    [TOCAR DO INTRO]    [TOCAR DO OUTRO]      │
│                                                                         │
│              [ CANCELAR ]              [ SALVAR MARCADORES ]            │
└─────────────────────────────────────────────────────────────────────────┘
```

**Legenda das cores na waveform:**

| Cor | Marcador |
|-----|----------|
| Azul | cue_in — linha vertical |
| Verde | intro — linha vertical |
| Laranja | outro — linha vertical |
| Vermelho | cue_out — linha vertical |
| Cinza escuro | Região antes de cue_in (não reproduzida) |
| Cinza claro | Região após cue_out (não reproduzida) |

**Interações:**
- Arrastar qualquer linha colorida reposiciona o marcador.
- Botão "CAPTURAR" define o marcador na posição atual de reprodução.
- Botão "LIMPAR" remove o marcador (volta para comportamento de fallback).
- Botões "TOCAR DO X" fazem seek e iniciam reprodução a partir daquele marcador.
- Zoom amplia a visualização horizontal (útil para precisão em silêncios curtos).

### 6.4 Atalho de teclado

Tecla `C` (enquanto uma faixa está selecionada na lista do catálogo) abre o Editor de Cue Points.

---

## 7. Modelo de dados

### 7.1 Alteração na tabela `tracks` (Library Service)

```sql
-- Migration 011_cue_points.sql
ALTER TABLE tracks ADD COLUMN cue_in_ms  INTEGER DEFAULT NULL;
ALTER TABLE tracks ADD COLUMN intro_ms   INTEGER DEFAULT NULL;
ALTER TABLE tracks ADD COLUMN outro_ms   INTEGER DEFAULT NULL;
ALTER TABLE tracks ADD COLUMN cue_out_ms INTEGER DEFAULT NULL;
```

Todos os campos são nullable — NULL significa "não definido", e o sistema aplica fallback para o comportamento atual.

### 7.2 Estrutura completa relevante da tabela `tracks`

```sql
CREATE TABLE tracks (
    id              TEXT PRIMARY KEY,
    path            TEXT NOT NULL,
    title           TEXT,
    artist          TEXT,
    album           TEXT,
    type            TEXT,
    duration_ms     INTEGER,

    -- Loudness (existente)
    loudness_lufs   REAL,
    true_peak_dbtp  REAL,
    loudness_status TEXT DEFAULT 'pending',

    -- Cue points (novos)
    cue_in_ms       INTEGER DEFAULT NULL,  -- ms: onde a reprodução inicia
    intro_ms        INTEGER DEFAULT NULL,  -- ms: onde a letra/voz começa
    outro_ms        INTEGER DEFAULT NULL,  -- ms: onde o crossfade inicia
    cue_out_ms      INTEGER DEFAULT NULL,  -- ms: onde a reprodução termina

    created_at      TEXT,
    updated_at      TEXT
);
```

### 7.3 Índices

Não são necessários índices adicionais — os cue points são sempre lidos junto com a faixa por `id`.

---

## 8. Endpoints e contratos

### 8.1 Novo endpoint — Salvar marcadores

```
PUT /v1/tracks/:id/cuepoints
```

**Descrição:** Define ou atualiza os marcadores de cue points de uma faixa. Campos omitidos no body não são alterados. Campos enviados como `null` removem o marcador (volta para fallback).

**Request body:**
```json
{
  "cue_in_ms":  432,
  "intro_ms":   18200,
  "outro_ms":   135000,
  "cue_out_ms": 151000
}
```

**Response 200:**
```json
{
  "ok": true,
  "track_id": "01KX1CHPD2XQ5NFDNSBC8QV0ZR",
  "cue_in_ms":  432,
  "intro_ms":   18200,
  "outro_ms":   135000,
  "cue_out_ms": 151000
}
```

**Response 400** — valores inválidos (ex.: intro_ms > cue_out_ms):
```json
{
  "ok": false,
  "error": "invalid_cuepoints",
  "message": "intro_ms (18200) must be less than cue_out_ms (15000)"
}
```

**Validações:**
- `cue_in_ms >= 0`
- `cue_out_ms <= duration_ms` (se duration_ms conhecido)
- `cue_in_ms < intro_ms < outro_ms < cue_out_ms` (quando todos definidos)

---

### 8.2 Endpoint existente impactado — GET /v1/tracks/:id

**Descrição:** Adiciona os campos de cue points na resposta.

**Response 200 (atualizado):**
```json
{
  "id": "01KX1CHPD2XQ5NFDNSBC8QV0ZR",
  "path": "/media/musicas/chega-de-saudade.mp3",
  "title": "Chega de Saudade",
  "artist": "João Gilberto",
  "duration_ms": 151000,
  "loudness_lufs": -16.2,
  "gain_db": 0.2,
  "cue_in_ms": 432,
  "intro_ms": 18200,
  "outro_ms": 135000,
  "cue_out_ms": 151000
}
```

---

### 8.3 Endpoint existente impactado — GET /v1/tracks (listagem)

**Descrição:** Adiciona coluna `intro_ms` na listagem para exibição na coluna "Intro" do catálogo.

**Response 200 (item atualizado):**
```json
{
  "id": "...",
  "title": "Chega de Saudade",
  "artist": "João Gilberto",
  "duration_ms": 151000,
  "intro_ms": 18200,
  "gain_db": 0.2,
  "cue_in_ms": 432
}
```

---

### 8.4 Endpoints existentes impactados — enqueue / schedule

**Endpoints:** `POST /v1/queue/enqueue`, `GET /v1/schedule/generate`, `GET /v1/hotkeys/profile/:id`

**Descrição:** Os cue points devem ser incluídos em todos os payloads que descrevem faixas a serem reproduzidas, para que o Playout Engine os receba sem precisar consultar o Library Service separadamente.

**Adição nos itens de faixa:**
```json
{
  "path": "/media/musicas/chega-de-saudade.mp3",
  "title": "Chega de Saudade",
  "gain_db": 0.2,
  "cue_in_ms": 432,
  "intro_ms": 18200,
  "outro_ms": 135000,
  "cue_out_ms": 151000
}
```

---

### 8.5 Playout Engine — command types impactados

**`QueueItemInput`** em `playout/internal/commands/types.go`:

```go
type QueueItemInput struct {
    // ... campos existentes ...
    CueInMS  int64 // ms: ponto de início (seek inicial); 0 = início do arquivo
    IntroMS  int64 // ms: fim da intro; 0 = sem intro definido
    OutroMS  int64 // ms: ponto de início do crossfade; 0 = usa lógica atual de tempo fixo
    CueOutMS int64 // ms: ponto de fim; 0 = EOF do arquivo
}
```

---

### 8.6 Novo evento WebSocket — IntroCountdown

**Evento:** `cart.intro_countdown` / `playback.intro_countdown`

**Descrição:** Publicado pelo Playout Engine a cada segundo enquanto a faixa atual está no período de intro (posição < intro_ms). Permite que o player exiba o contador regressivo em tempo real.

**Payload:**
```json
{
  "event": "playback.intro_countdown",
  "payload": {
    "queue_item_id": "01KX...",
    "title": "Chega de Saudade",
    "intro_ms": 18200,
    "position_ms": 10500,
    "remaining_ms": 7700
  }
}
```

Quando `remaining_ms` atinge 0, o engine publica um último evento com `remaining_ms: 0` e para de emitir.

---

## 9. Detalhamento técnico da implementação

### 9.1 Library Service

#### Migration (011_cue_points.sql)
Quatro colunas nullable adicionadas à tabela `tracks`. Migração não-destrutiva.

#### Auto-detecção de cue_in na importação

No `scanner/indexer.go`, após a extração de metadados existente:

```go
func detectCueIn(path string) int64 {
    out, err := exec.Command("ffmpeg",
        "-hide_banner", "-vn",
        "-i", path,
        "-af", "silencedetect=n=-50dB:d=0.1",
        "-f", "null", "-",
    ).CombinedOutput()
    if err != nil {
        return 0
    }
    // Parse: silence_end: 0.432
    re := regexp.MustCompile(`silence_end: ([\d.]+)`)
    m := re.FindStringSubmatch(string(out))
    if m == nil {
        return 0
    }
    f, _ := strconv.ParseFloat(m[1], 64)
    return int64(f * 1000)
}
```

#### Handler PUT /v1/tracks/:id/cuepoints

Novo arquivo `library/internal/api/handlers/cuepoints.go`:

```go
func SaveCuePoints(ts TrackStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")
        var req store.CuePoints
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
            return
        }
        if err := req.Validate(); err != nil {
            writeError(w, http.StatusBadRequest, "invalid_cuepoints", err.Error())
            return
        }
        if err := ts.SaveCuePoints(r.Context(), id, req); err != nil {
            writeError(w, http.StatusInternalServerError, "store_error", err.Error())
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"ok": true, "track_id": id})
    }
}
```

#### Store — SaveCuePoints

```go
func (s *TrackStore) SaveCuePoints(ctx context.Context, id string, cp CuePoints) error {
    _, err := s.db.ExecContext(ctx, `
        UPDATE tracks
        SET cue_in_ms = ?, intro_ms = ?, outro_ms = ?, cue_out_ms = ?,
            updated_at = datetime('now')
        WHERE id = ?`,
        cp.CueInMS, cp.IntroMS, cp.OutroMS, cp.CueOutMS, id,
    )
    return err
}
```

### 9.2 Playout Engine

#### Seek inicial para cue_in

No decoder FFmpeg (`internal/audio/decoder/ffmpeg.go`), ao abrir o stream, se `cueInMS > 0`:

```go
if src.CueInMS > 0 {
    seekSecs := float64(src.CueInMS) / 1000.0
    args = append([]string{"-ss", fmt.Sprintf("%.3f", seekSecs)}, args...)
}
```

O flag `-ss` antes do `-i` é fast seek (keyframe) — preciso o suficiente para silêncio inicial (que é sempre no começo de um keyframe).

#### Crossfade em outro_ms

No `playback/manager.go`, a lógica de decisão de crossfade:

```go
func shouldStartCrossfade(posMS, outroMS, cueOutMS, fixedFadeMS int64) bool {
    if outroMS > 0 {
        return posMS >= outroMS
    }
    // fallback: tempo fixo antes do cue_out ou EOF
    end := cueOutMS
    if end == 0 {
        end = durationMS
    }
    return posMS >= end - fixedFadeMS
}
```

#### Parada em cue_out_ms

No loop de playback, verificar a cada frame:

```go
if cueOutMS > 0 && posMS >= cueOutMS {
    send(intMsg{kind: intEnded})
    return
}
```

#### Evento IntroCountdown

No loop de eventos do playback manager, publicar a cada segundo enquanto posição < intro_ms:

```go
if introMS > 0 && posMS < introMS {
    remaining := introMS - posMS
    p.evtBus.Publish(events.New(events.EvtIntroCountdown, events.IntroCountdownPayload{
        QueueItemID: itemID,
        Title:       title,
        IntroMS:     introMS,
        PositionMS:  posMS,
        RemainingMS: remaining,
    }))
}
```

### 9.3 Player (Electron)

#### wavesurfer.js — integração

```html
<!-- No modal de cue points -->
<script src="wavesurfer.min.js"></script>
<script src="wavesurfer.regions.min.js"></script>
```

```javascript
const ws = WaveSurfer.create({
  container: '#waveform',
  waveColor: '#4a9eff',
  progressColor: '#1a5fbf',
  height: 80,
  plugins: [WaveSurfer.Regions.create({ dragSelection: false })]
});

ws.load('file://' + track.path);

ws.on('ready', () => {
  if (track.cue_in_ms)  addMarker('cue_in',  track.cue_in_ms  / 1000, '#4a9eff');
  if (track.intro_ms)   addMarker('intro',   track.intro_ms   / 1000, '#2ec44e');
  if (track.outro_ms)   addMarker('outro',   track.outro_ms   / 1000, '#f0a500');
  if (track.cue_out_ms) addMarker('cue_out', track.cue_out_ms / 1000, '#e53935');
});

function addMarker(id, timeSec, color) {
  ws.addRegion({
    id,
    start: timeSec,
    end:   timeSec + 0.01,  // largura mínima para linha vertical
    color: color + '80',    // semi-transparente
    drag: true,
    resize: false,
  });
}
```

#### Contador de intro no Now Playing

```javascript
ws.on('ws-message', (evt) => {
  const msg = JSON.parse(evt.data);
  if (msg.event === 'playback.intro_countdown') {
    const rem = msg.payload.remaining_ms;
    document.getElementById('intro-countdown').textContent =
      rem > 0 ? formatMS(rem) : '';
    document.getElementById('intro-countdown').style.color =
      rem < 5000 ? '#e53935' : '#f0a500';
  }
});
```

---

## 10. Riscos e mitigações

| # | Risco | Probabilidade | Impacto | Mitigação |
|---|-------|:-------------:|:-------:|-----------|
| R1 | Auto-detecção de silêncio com falso positivo em músicas com intro suave (violino, piano pianíssimo) detecta erroneamente os primeiros segundos como silêncio | Média | Médio | Limitar o threshold a -50 dB (conservador); exibir o valor sugerido ao operador como "sugestão" e não aplicar automaticamente sem revisão |
| R2 | Performance do editor de waveform em faixas longas (60+ min, ex.: missas, transmissões ao vivo) | Baixa | Alto | wavesurfer.js renderiza via Canvas com downsampling — testar com arquivo de 2h; se lento, implementar waveform pré-computado e cacheado no Library Service |
| R3 | seek FFmpeg com `-ss` antes de `-i` não é frame-accurate para todos os formatos (MP3 VBR) | Alta | Baixo | Para cue_in que tipicamente é < 1 s de silêncio, a imprecisão é irrelevante (< 50 ms). Para uso futuro com cue_in musical, implementar `-accurate_seek` |
| R4 | Marcadores salvos invalidados por re-importação do mesmo arquivo (scanner sobrescreve) | Média | Alto | Scanner não sobrescreve campos de cue points se já existirem — apenas preenche se NULL. `cue_in_ms` auto-detectado nunca sobrescreve valor manual. |
| R5 | outro_ms definido após o EOF real do arquivo (erro de operador) | Baixa | Médio | Validação no PUT /v1/tracks/:id/cuepoints: `outro_ms < duration_ms`. Se duration_ms não disponível, validar no engine e logar aviso sem parar a reprodução. |
| R6 | Engine recebe outro_ms mas a faixa já passou desse ponto quando o item é carregado (seek > outro_ms) | Muito baixa | Alto | Engine verifica: se `cueInMS >= outroMS`, ignora outro e usa fallback. Logar aviso. |
| R7 | wavesurfer.js carrega áudio via `file://` no Electron — possível bloqueio de Content Security Policy | Média | Alto | Configurar CSP do Electron para permitir `file://` na origem do waveform. Alternativa: usar `app.getPath('temp')` para copiar o arquivo e servir via protocolo customizado. |

---

## 11. Detalhamento das fases

### Fase 1 — Setup e modelo de dados (Library Service)

**Objetivo:** criar a infraestrutura de dados sem alterar nenhum comportamento existente.

1. Branch `feature/cue-points` criada a partir de `main` (ja feito).
2. Criar `library/internal/store/migrations/011_cue_points.sql` com os quatro campos nullable.
3. Registrar a migration no loader de migrations do `db.go`.
4. Adicionar `CueInMS`, `IntroMS`, `OutroMS`, `CueOutMS *int64` à struct `store.Track`.
5. Atualizar o `trackScanner` para ler os novos campos do SELECT.
6. Atualizar `GET /v1/tracks/:id` para incluir os campos na resposta JSON.
7. Atualizar `GET /v1/tracks` (listagem) para incluir `cue_in_ms` e `intro_ms`.
8. Escrever testes unitários para o scanner com campos nullable.
9. `go test ./... && go vet ./...`

**Entregável:** Library Service compila com os novos campos; respostas da API já incluem os cue points (todos NULL por enquanto).

---

### Fase 2 — API de cue points (Library Service)

**Objetivo:** permitir salvar e limpar marcadores via API.

1. Criar `store.CuePoints` struct com método `Validate()`.
2. Implementar `store.TrackStore.SaveCuePoints()`.
3. Criar `handlers/cuepoints.go` com handler `SaveCuePoints`.
4. Registrar rota `PUT /v1/tracks/{id}/cuepoints` no `server.go`.
5. Escrever testes de handler para casos: sucesso, validação inválida, track não encontrada.
6. `go test ./... && go vet ./...`

**Entregável:** `PUT /v1/tracks/:id/cuepoints` funcional e testado.

---

### Fase 3 — Auto-detecção de cue_in na importação (Library Service)

**Objetivo:** preencher `cue_in_ms` automaticamente ao importar novas faixas.

1. Implementar `detectCueIn(path string) int64` em `scanner/indexer.go`.
2. Chamar `detectCueIn` durante a indexação, gravar resultado se > 0 e campo ainda for NULL.
3. Adicionar flag de configuração `auto_detect_cue_in: bool` no config (default: true).
4. Escrever teste com arquivo de áudio de fixture (silêncio + sinal).
5. `go test ./... && go vet ./...`

**Entregável:** Faixas novas chegam com `cue_in_ms` preenchido automaticamente.

---

### Fase 4 — Propagação de cue points nos payloads de enqueue (Library Service + Player)

**Objetivo:** todos os endpoints que produzem itens para reprodução devem incluir os cue points.

1. Atualizar `QueueItemInput` no `commands/types.go` do Playout Engine com `CueInMS`, `IntroMS`, `OutroMS`, `CueOutMS int64`.
2. Atualizar `GET /v1/schedule/generate` para incluir cue points nas faixas geradas.
3. Atualizar `GET /v1/hotkeys/profile/:id` para incluir cue points nos botões.
4. Atualizar `POST /v1/queue/enqueue` para aceitar os campos opcionais.
5. Atualizar `player.html` para passar cue points em todos os sites de enqueue.
6. `go test ./...` em library e playout.

**Entregável:** Cue points fluem do banco até o payload que chega ao Playout Engine.

---

### Fase 5 — Uso dos cue points no Playout Engine

**Objetivo:** engine usa cue_in para seek, outro para crossfade e cue_out para parada.

1. Passar `CueInMS` para o decoder FFmpeg como flag `-ss` na abertura do stream.
2. Atualizar a lógica de crossfade para usar `OutroMS` quando definido.
3. Adicionar verificação de `CueOutMS` no loop de reprodução.
4. Implementar publicação do evento `IntroCountdown` via Event Bus.
5. Definir `events.EvtIntroCountdown` e `events.IntroCountdownPayload`.
6. Propagar o evento via WebSocket no handler existente.
7. Escrever testes de integração para seek, crossfade e cue_out.
8. `go test -race ./...`

**Entregável:** Engine usa marcadores na reprodução; evento IntroCountdown publicado.

---

### Fase 6 — Editor visual de cue points no Player (UI)

**Objetivo:** operador pode visualizar e editar marcadores via waveform.

1. Integrar `wavesurfer.js` e plugin `wavesurfer.regions.js` no Player.
2. Criar modal `cue-editor-modal` em `player.html` com layout descrito na seção 6.3.
3. Implementar carregamento da waveform via `file://` + fallback de CSP.
4. Implementar drag de marcadores com atualização em tempo real dos campos de tempo.
5. Implementar botões CAPTURAR, LIMPAR e TOCAR DO X.
6. Implementar chamada `PUT /v1/tracks/:id/cuepoints` ao salvar.
7. Implementar abertura do modal via ícone [CUE] na lista e atalho de teclado `C`.
8. Testar manualmente com faixas de diferentes formatos (MP3, FLAC, WAV).

**Entregável:** Editor de cue points visual completo e funcional.

---

### Fase 7 — Countdown de intro no Now Playing

**Objetivo:** operador vê o contador regressivo em tempo real no painel de transmissão.

1. Consumir evento WebSocket `playback.intro_countdown` no player.
2. Renderizar bloco de countdown no painel Now Playing conforme layout da seção 6.1.
3. Mudar cor de amarelo para vermelho nos últimos 5 s.
4. Ocultar o bloco quando `remaining_ms = 0` ou faixa não tem `intro_ms`.
5. Adicionar coluna "Intro" na tabela de faixas do catálogo (toggle opcional).
6. Testar com faixas reais em transmissão ao vivo.

**Entregável:** Countdown visível no painel on-air; coluna Intro no catálogo.

---

### Fase 8 — Testes, ajustes e PR

**Objetivo:** garantir qualidade e integrar na main.

1. `go test ./...` em todos os módulos.
2. `go test -race ./...` no playout.
3. Testes manuais com catálogo real (vinhetas, músicas, jingles, spots).
4. Verificar fallback (faixas sem marcadores continuam funcionando).
5. Atualizar `docs/benchmark.md` — marcar "Marcadores de intro/outro/cue" como ✅.
6. Commit e PR para `main`.

---

## 12. Pontos adicionais importantes

### 12.1 Compatibilidade retroativa

Todos os quatro campos são nullable no banco e opcionais nos payloads. Faixas sem marcadores funcionam exatamente como hoje — sem nenhuma alteração de comportamento. Isso garante que a feature pode ser deployada sem impacto em operações em andamento.

### 12.2 Ordenação de marcadores

A UI deve garantir e o backend deve validar que a ordem lógica é sempre respeitada:

```
0 ≤ cue_in_ms ≤ intro_ms ≤ outro_ms ≤ cue_out_ms ≤ duration_ms
```

### 12.3 Hora Certa — integração futura

Com `outro_ms` disponível, a Hora Certa pode ser sincronizada com precisão de segundo. Essa integração é um aprimoramento do módulo de agendamento (`scheduler`) e será tratada como task separada após a entrega dos marcadores. O modelo de dados desta fase já suporta isso sem alterações adicionais.

### 12.4 Futura auto-detecção de intro vocal

O benchmark mostra que o RadioBOSS usa IA de separação vocal (via RadioForge) para detectar automaticamente `intro_ms`. Isso é uma evolução natural após a fase 3. A implementação usaria um modelo de source separation leve (ex.: Demucs tiny exportado como ONNX) rodando localmente no Library Service. Não está no escopo desta entrega, mas o modelo de dados e a API já estão preparados para receber esse valor de forma automática.

### 12.5 Exportação para tags APEv2 / ID3

O RadioBOSS armazena os marcadores nas próprias tags do arquivo MP3 para portabilidade. No RadioFlow, a decisão arquitetural é armazenar no banco (não no arquivo) por três razões: (1) não altera o arquivo original; (2) não depende de formato de arquivo; (3) é mais simples de manter consistente. Uma exportação opcional para ID3 TXXX ou APEv2 pode ser implementada no futuro como utilitário CLI.

---

## 13. Dependências externas

| Dependência | Versão recomendada | Propósito | Licença |
|---|---|---|---|
| wavesurfer.js | 7.x | Editor de waveform no Player | BSD-3 |
| wavesurfer.js Regions plugin | 7.x | Marcadores arrastáveis | BSD-3 |
| ffmpeg | 6.x+ (já presente) | silencedetect, seek | LGPL |

Sem novas dependências no Library Service nem no Playout Engine (Go puro).

---

*Plano gerado em julho de 2026. Branch: `feature/cue-points`.*
