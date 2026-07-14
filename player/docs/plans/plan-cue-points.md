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

**~~RN-07~~** — ~~O sistema deve usar `cue_out` para sincronizar a Hora Certa: o engine calcula com antecedência o momento exato em que o fade deve iniciar para que a hora certa entre na virada do minuto.~~

> **Fora de escopo desta feature.** A sincronização da Hora Certa com `cue_out_ms` é uma evolução futura do módulo de agendamento e **não será implementada nesta entrega**. Os marcadores desta feature apenas fornecem os dados (`cue_out_ms`) que a Hora Certa poderá consumir no futuro — sem nenhuma alteração no módulo de agendamento agora.

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

| Marcador | Campo | Auto-detectado |
|----------|-------|:--------------:|
| Cue In | `cue_in_ms` | Sim (silencedetect) |
| Intro | `intro_ms` | Não (manual) |
| Outro | `outro_ms` | Não (manual) |
| Cue Out | `cue_out_ms` | Não (manual) |

---

#### Visualização da posição dos marcadores na linha do tempo

```
Arquivo de áudio (exemplo: 3:02)
│
0:00                                                              3:02
├──────────────────────────────────────────────────────────────────┤
│  silêncio │◄── intro instrumental ──►│◄── corpo da música ──►│fade│
│           │                          │                        │   │
▲           ▲                          ▲                        ▲
cue_in_ms   intro_ms               (conteúdo)              outro_ms  cue_out_ms
```

---

#### cue_in_ms — Ponto de início da reprodução

**O que é:**
O momento exato, em milissegundos, a partir do qual o Playout Engine começa a reproduzir o arquivo. Tudo antes desse ponto é ignorado — o decoder faz um seek direto para essa posição.

**Por que existe:**
Arquivos de áudio frequentemente contêm silêncio no início. Isso ocorre por dois motivos: (1) artefato de codificação MP3 — o encoder LAME insere frames silenciosos no início para garantir sincronismo de decodificação; (2) gravações analógicas digitalizadas que incluem o "rabo" do silêncio anterior à gravação. Esse silêncio chega ao ar e o ouvinte percebe como uma "falha" ou "pausa estranha" entre músicas.

**Exemplo prático:**
```
Arquivo: Aquarela - Toquinho.mp3
Duração real:  3:42
Silêncio inicial detectado: 0,432 s

cue_in_ms = 432

Resultado: o engine começa a reproduzir no frame 432 ms,
eliminando completamente o silêncio antes de o áudio chegar ao ar.
```

**Quem define:** detectado automaticamente pelo `ffmpeg silencedetect` na importação. Pode ser ajustado manualmente no editor de waveform.

**Fallback:** se NULL ou 0, o engine reproduz desde o frame 0 (comportamento atual).

---

#### intro_ms — Fim da introdução instrumental

**O que é:**
O momento, em milissegundos, em que a voz do cantor ou o conteúdo principal da faixa começa — ou seja, onde termina a introdução puramente instrumental. Esse marcador não afeta a reprodução em si; seu único uso é informar o locutor quanto tempo de intro ainda resta para ele falar ao microfone.

**Por que existe:**
Na operação ao vivo de uma rádio, é prática universal o locutor "falar sobre a música" enquanto ela começa — apresentando o artista, o nome da música, fazendo uma observação. Para isso, o locutor precisa saber com precisão quantos segundos de intro instrumental tem antes da letra do cantor começar. Sem essa informação visível, o locutor usa intuição e frequentemente comete dois erros:
- **Fala demais:** a voz do locutor sobrepõe a letra do cantor — erro grave, perceptível ao ouvinte.
- **Para cedo:** fica em silêncio desnecessário esperando a letra — desperdiça o intro e soa amador.

**Exemplo prático:**
```
Música: La Isla Bonita — Madonna
intro_ms = 17800  (17,8 segundos de intro instrumental)

Painel do locutor enquanto a música toca:
  ▶  INTRO   00:17   (amarelo)
  ▶  INTRO   00:09   (amarelo)
  ▶  INTRO   00:04   (vermelho — urgência)
  [linha some — locutor deve parar de falar]
```

**Quem define:** manual, via editor de waveform. O programador ouve a música e posiciona o marcador no ponto exato onde a letra começa.

**Fallback:** se NULL, o contador de intro não é exibido no painel. Nenhum impacto na reprodução.

---

#### outro_ms — Ponto de início do crossfade de saída

**O que é:**
O momento, em milissegundos, em que o Playout Engine deve iniciar o crossfade para a próxima faixa. A partir desse ponto, o volume da faixa atual começa a diminuir enquanto o volume da próxima faixa começa a aumentar.

**Por que existe:**
Sem esse marcador, o engine usa uma regra fixa: "inicia o crossfade N segundos antes do fim do arquivo". Esse comportamento é cego — não sabe se nesses últimos N segundos ainda há letra sendo cantada, um acorde final ressoando ou já é silêncio. Isso causa dois problemas comuns:
- **Crossfade cortando a letra:** o fade começa enquanto o cantor ainda está cantando — o ouvinte percebe a voz "sumindo" de forma artificial no meio da frase.
- **Crossfade atrasado demais:** a música já terminou (silêncio) e a próxima ainda não começou — há uma brecha de silêncio audível.

Com `outro_ms`, o programador marca exatamente onde a música termina de forma musical (geralmente o último acorde antes do fade natural ou do silêncio final), e o engine usa esse ponto como gatilho preciso para o crossfade.

**Exemplo prático:**
```
Música: Emoções — Roberto Carlos   (duração: 4:12 = 252.000 ms)
O cantor termina a última nota em 4:05 (245.000 ms)
Os últimos 7 segundos são reverb decaindo até o silêncio

outro_ms = 245000

Resultado: o crossfade começa em 4:05, quando a música já terminou
musicalmente, e a próxima faixa entra de forma natural e precisa.
```

**Quem define:** manual, via editor de waveform.

**Fallback:** se NULL, o engine usa a lógica atual de tempo fixo antes do EOF (comportamento existente).

---

#### cue_out_ms — Ponto de término da reprodução

**O que é:**
O momento, em milissegundos, em que o Playout Engine para completamente a reprodução da faixa, independentemente de haver mais áudio no arquivo após esse ponto. Tudo após `cue_out_ms` é descartado.

**Por que existe:**
Muitos arquivos de áudio têm conteúdo indesejado após o ponto de término musical:
- **Silêncio longo no final:** codificação com padding.
- **Ruído de tape:** gravações analógicas com chiado residual após o fade.
- **Conteúdo extra:** em arquivos de rádio antigos, às vezes há um "take" de ensaio ou conversa de estúdio após o fade da música.
- **Fade incompleto:** o arquivo foi exportado antes do silence completo — há um último transiente audível que, se reproduzido, soa como um "estalo".

`cue_out_ms` garante que o engine para exatamente onde o programador quer, sem tocar nada além desse ponto.

**Relação com outro_ms:**
Os dois marcadores trabalham juntos. `outro_ms` define quando o crossfade **começa**. `cue_out_ms` define quando a reprodução **termina**. Em uma configuração típica:

```
outro_ms   = 245.000 ms  → crossfade começa aqui
cue_out_ms = 248.000 ms  → reprodução para aqui (3 s de fade)

Durante esses 3 s, a faixa está em fade out E a próxima já está
em fade in — as duas tocando simultaneamente no mixer.
```

**Quem define:** manual, via editor de waveform.

**Fallback:** se NULL, o engine reproduz até o EOF real do arquivo (comportamento atual).

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

### 8.2 Novo endpoint — Re-análise retroativa de cue_in

```
POST /v1/tracks/reanalyze-cuepoints
```

**Descrição:** Dispara a análise retroativa de `cue_in_ms` para todas as faixas que já estavam no catálogo antes da feature de cue points ser implantada — ou seja, faixas com `cue_in_ms = NULL`. Funciona de forma análoga ao `POST /v1/loudness/analyze`: enfileira as faixas pendentes e processa em background via worker, sem bloquear a API.

**Motivação:** O scanner só roda `silencedetect` em arquivos **novos** detectados pelo watch folder. Faixas importadas antes da Fase 3 desta feature terão `cue_in_ms = NULL` indefinidamente, a menos que sejam re-analisadas explicitamente. Este endpoint resolve isso sem exigir que o operador reimporte o catálogo.

**Comportamento:**
- Busca todas as faixas com `cue_in_ms IS NULL`.
- Enfileira para análise via `silencedetect`.
- Processa em background (mesmo worker da análise de loudness, ou worker dedicado).
- Faixas com `cue_in_ms` já definido (inclusive `0`) **não são reprocessadas** — o valor manual do operador é preservado.

**Request body:** nenhum (ou body vazio `{}`).

**Response 200:**
```json
{
  "ok": true,
  "enqueued": 124
}
```

**Response 200 — nenhuma faixa pendente:**
```json
{
  "ok": true,
  "enqueued": 0,
  "message": "all tracks already have cue_in_ms defined"
}
```

**Progresso:** consultável via `GET /v1/tracks/reanalyze-cuepoints/status` (mesmo padrão do `/v1/loudness/status`):

```json
{
  "running": true,
  "counts": {
    "pending":   98,
    "analyzing":  2,
    "done":      24,
    "error":      0
  }
}
```

**Regra importante:** o endpoint **nunca sobrescreve** um `cue_in_ms` já definido pelo operador, mesmo que seja diferente do valor que o `silencedetect` detectaria. O valor manual tem precedência absoluta.

---

### 8.4 Endpoint existente impactado — GET /v1/tracks/:id

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

### 8.5 Endpoint existente impactado — GET /v1/tracks (listagem)

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

### 8.6 Endpoints existentes impactados — todos os que produzem faixas para reprodução

Os cue points devem estar presentes em **todos** os endpoints do Library Service que retornam faixas destinadas à reprodução, e o Player deve repassá-los ao Playout Engine em todos os sites de enqueue. Isso garante que o engine sempre receba os marcadores sem precisar consultar o Library Service separadamente.

**Campos adicionados em todos os itens de faixa:**
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

**Campos com valor `null` ou `0` indicam marcador não definido** — o Playout Engine aplica o fallback correspondente.

---

#### Endpoints do Library Service impactados

| Endpoint | Retorna faixas? | Impacto |
|----------|:--------------:|---------|
| `GET /v1/tracks` | ✅ | Adicionar `cue_in_ms` e `intro_ms` (os 4 campos na resposta de item) |
| `GET /v1/tracks/:id` | ✅ | Adicionar os 4 campos |
| `GET /v1/playlists/:id` | ✅ | Adicionar os 4 campos em cada item da playlist |
| `GET /v1/breaks/:id` | ✅ | Adicionar os 4 campos em cada spot/open/close do bloco |
| `GET /v1/schedule/generate` | ✅ | Adicionar os 4 campos em cada faixa gerada |
| `GET /v1/hotkeys/profiles/:id` | ✅ | Adicionar os 4 campos em cada botão |
| `GET /v1/categories/:id/tracks` | ✅ | Adicionar os 4 campos em cada faixa listada |

---

#### Sites de enqueue no Player (`player.html` e `hotkeys.html`) impactados

| Ação no Player | Endpoint do Playout chamado | O que deve incluir |
|----------------|----------------------------|--------------------|
| Enfileirar faixa do catálogo | `POST /v1/queue/enqueue` | `cue_in_ms`, `intro_ms`, `outro_ms`, `cue_out_ms` |
| Enfileirar playlist | `POST /v1/queue/enqueue` | idem, para cada item da playlist |
| Enfileirar bloco comercial | `POST /v1/queue/enqueue-break` | idem, para open, spots e close |
| Inserir próxima faixa | `POST /v1/queue/insert-next` | idem |
| Inserir após item | `POST /v1/queue/insert-after` | idem |
| Enfileirar via gerador de rotação | `POST /v1/queue/enqueue` | idem |
| Acionar botão da botoneira | `POST /v1/cart/play` | idem |

---

### 8.7 Playout Engine — command types impactados

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

### 8.8 Novo evento WebSocket — IntroCountdown

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

### Fase 4 — Re-análise retroativa de cue_in para faixas existentes (Library Service)

**Objetivo:** preencher `cue_in_ms` automaticamente em faixas que já estavam no catálogo antes da feature, sem exigir reimportação.

1. Implementar worker de re-análise retroativa — varre faixas com `cue_in_ms IS NULL` e roda `silencedetect` em cada uma.
2. Adicionar tabela ou coluna de status de re-análise (ex.: `cue_in_status TEXT DEFAULT 'pending'`) para acompanhamento de progresso.
3. Criar handler `POST /v1/tracks/reanalyze-cuepoints` que enfileira as faixas pendentes e inicia o worker.
4. Criar handler `GET /v1/tracks/reanalyze-cuepoints/status` que retorna contagens por status.
5. Registrar as rotas no `server.go`.
6. Garantir que o worker **nunca sobrescreve** `cue_in_ms` já definido (NULL check antes de gravar).
7. Escrever testes para o handler e para a regra de não-sobrescrita.
8. `go test ./... && go vet ./...`

**Entregável:** operador pode disparar `POST /v1/tracks/reanalyze-cuepoints` após o deploy e acompanhar o progresso via `GET /v1/tracks/reanalyze-cuepoints/status`. Faixas existentes recebem `cue_in_ms` automaticamente; valores definidos manualmente são preservados.

---

### Fase 5 — Propagação de cue points nos payloads de enqueue (Library Service + Player)

**Objetivo:** todos os endpoints do Library Service que produzem faixas para reprodução devem incluir os 4 campos de cue points, e todos os sites de enqueue no Player devem repassá-los ao Playout Engine.

#### 5.1 — Playout Engine: ampliar QueueItemInput

1. Adicionar `CueInMS`, `IntroMS`, `OutroMS`, `CueOutMS int64` ao `QueueItemInput` em `playout/internal/commands/types.go`.
2. `go build ./...` no playout para confirmar compilação.

#### 5.2 — Library Service: endpoints que retornam faixas

Atualizar cada handler para incluir os 4 campos em todos os itens de faixa retornados:

| # | Endpoint | Arquivo de handler |
|---|----------|--------------------|
| 1 | `GET /v1/tracks` | `handlers/tracks.go` |
| 2 | `GET /v1/tracks/:id` | `handlers/tracks.go` |
| 3 | `GET /v1/playlists/:id` | `handlers/playlists.go` |
| 4 | `GET /v1/breaks/:id` | `handlers/breaks.go` |
| 5 | `GET /v1/schedule/generate` | `handlers/schedule.go` |
| 6 | `GET /v1/hotkeys/profiles/:id` | `handlers/hotkeys.go` |
| 7 | `GET /v1/categories/:id/tracks` | `handlers/categories.go` |

#### 5.3 — Player: sites de enqueue

Atualizar `player.html` e `hotkeys.html` para incluir `cue_in_ms`, `intro_ms`, `outro_ms`, `cue_out_ms` em todos os payloads enviados ao Playout Engine:

| # | Ação | Endpoint do Playout | Arquivo |
|---|------|---------------------|---------|
| 1 | Enfileirar faixa do catálogo | `POST /v1/queue/enqueue` | `player.html` |
| 2 | Enfileirar playlist | `POST /v1/queue/enqueue` | `player.html` |
| 3 | Enfileirar bloco comercial | `POST /v1/queue/enqueue-break` | `player.html` |
| 4 | Inserir próxima faixa | `POST /v1/queue/insert-next` | `player.html` |
| 5 | Inserir após item | `POST /v1/queue/insert-after` | `player.html` |
| 6 | Enfileirar via gerador de rotação | `POST /v1/queue/enqueue` | `player.html` |
| 7 | Acionar botão da botoneira | `POST /v1/cart/play` | `hotkeys.html` |

#### 5.4 — Testes e validação

- `go test ./...` em library e playout.
- Verificar manualmente via `curl` que `GET /v1/breaks/:id` e `GET /v1/playlists/:id` retornam os 4 campos.

**Entregável:** Cue points fluem do banco → Library Service → Player → Playout Engine em todos os caminhos de reprodução.

---

### Fase 6 — Uso dos cue points no Playout Engine

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

### Fase 7 — Editor visual de cue points no Player (UI)

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

### Fase 8 — Countdown de intro no Now Playing

**Objetivo:** operador vê o contador regressivo em tempo real no painel de transmissão.

1. Consumir evento WebSocket `playback.intro_countdown` no player.
2. Renderizar bloco de countdown no painel Now Playing conforme layout da seção 6.1.
3. Mudar cor de amarelo para vermelho nos últimos 5 s.
4. Ocultar o bloco quando `remaining_ms = 0` ou faixa não tem `intro_ms`.
5. Adicionar coluna "Intro" na tabela de faixas do catálogo (toggle opcional).
6. Testar com faixas reais em transmissão ao vivo.

**Entregável:** Countdown visível no painel on-air; coluna Intro no catálogo.

---

### Fase 9 — Testes, ajustes e PR

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

### 12.3 Hora Certa — fora de escopo

**Não haverá nenhuma alteração ou implementação relacionada à Hora Certa nesta feature.**

O modelo de dados (`cue_out_ms`, `outro_ms`) fornece naturalmente os dados que um futuro módulo de sincronização de Hora Certa precisaria consumir. Essa integração é uma evolução independente do módulo de agendamento e será tratada como iniciativa separada em momento oportuno.

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
