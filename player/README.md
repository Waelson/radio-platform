# RadioFlow — Player

![Electron](https://img.shields.io/badge/Electron-35+-47848F?logo=electron&logoColor=white)
![Plataformas](https://img.shields.io/badge/plataformas-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)

Interface de operação ao vivo para o [RadioCore Playout Engine](../playout/README.md) e o [Radio Library Service](../library/README.md).
Construída em **Electron + HTML/CSS/JS puro**, sem frameworks — um único arquivo `player.html` que se conecta aos serviços via REST e WebSocket.

---

## Índice

- [Visão geral](#visão-geral)
- [Pré-requisitos](#pré-requisitos)
- [Instalação e execução](#instalação-e-execução)
- [Build e distribuição](#build-e-distribuição)
- [Configuração de URLs](#configuração-de-urls)
- [Interface](#interface)
- [Funcionalidades](#funcionalidades)
- [Controle de volume](#controle-de-volume)
- [Estrutura do projeto](#estrutura-do-projeto)

---

## Visão geral

O RadioFlow é a interface de operação ao vivo do sistema de automação de rádio. Conecta-se a dois serviços independentes:

| Serviço | Protocolo | Função |
|---|---|---|
| RadioCore (Playout Engine) | REST + WebSocket | Controle de playback, fila, modos, volume, preview |
| Radio Library Service | REST | Catálogo de áudios, playlists e blocos comerciais |

A comunicação é **unidirecional via eventos**: o engine publica estados via WebSocket e o player reage, sem polling.

---

## Pré-requisitos

- [Node.js](https://nodejs.org/) >= 18
- [RadioCore](../playout/README.md) em execução (porta `8080` por padrão)
- [Radio Library Service](../library/README.md) em execução (porta `8081`) — opcional para o Catálogo e Biblioteca

---

## Instalação e execução

```bash
cd radio-platform/player

# Instalar dependências
npm install

# Iniciar em modo desenvolvimento
npm start
```

O app abre em modo fullscreen conectado em `http://127.0.0.1:8080`.

---

## Build e distribuição

```bash
# macOS (arm64) — gera .app e .dmg
npm run build
# → dist/mac-arm64/RadioFlow.app
# → dist/RadioFlow-0.1.0-arm64.dmg
```

O `electron-builder` gera `.AppImage` (Linux) e instalador NSIS `.exe` (Windows) conforme a plataforma de build.

---

## Configuração de URLs

Por padrão o player conecta em:

| Serviço | URL padrão |
|---|---|
| Playout Engine (REST) | `http://127.0.0.1:8080` |
| Playout Engine (WebSocket) | `ws://127.0.0.1:8080/v1/events` |
| Library Service | `http://127.0.0.1:8081` |

Para apontar para endereços diferentes, use a query string ao carregar `player.html`:

```
player.html?api=http://192.168.1.10:8080&ws=ws://192.168.1.10:8080/v1/events&lib=http://192.168.1.10:8081
```

---

## Interface

### Header

O cabeçalho permanente exibe:

- **Logotipo RadioFlow** à esquerda
- **Card de status** com os campos:
  - Engine — identificador do engine conectado (truncado com reticências)
  - Estado — badge com o estado atual (`IDLE`, `PLAYING`, `PAUSED`, `PANIC`, etc.)
  - Modo — badge com o modo ativo (`AUTO`, `ASSIST`, `PANIC`)
  - Playout — indicador `On-line` / `Off-line` com ponto colorido
  - Library — indicador `On-line` / `Off-line` com ponto colorido
- **Relógio centralizado** com:
  - Hora `HH:MM:SS` em fonte monoespaçada
  - Data por extenso em pt-BR (ex.: `quarta-feira, 9 de julho de 2026`)
  - Temperatura e umidade em tempo real via [Open-Meteo](https://open-meteo.com/) (geolocalização automática; fallback para São Paulo)

### Layout principal

```
┌──────────────────────────────────────────────────────────────────┐
│                            HEADER                                │
├───────────────┬─────────────────────────────┬────────────────────┤
│   col-left    │        col-center           │    col-right       │
│               │                             │                    │
│  Now Playing  │   Fila de Reprodução        │  Biblioteca /      │
│  Loudness     │   (queue list)              │  drawer lateral    │
│  Saúde Áudio  │                             │                    │
│  Volume       │                             │                    │
│  VU Meters    │                             │                    │
└───────────────┴─────────────────────────────┴────────────────────┘
```

---

## Funcionalidades

### Fila de reprodução

- Visualização em tempo real da fila via eventos WebSocket (`QueueChanged`)
- Item atual em destaque com barra de progresso animada
- Badge de estado por item: `Tocando`, `Próxima`, `NA FILA`, `Tocada`, `Pulada`, `Erro`
- Barra de rolagem customizada sempre visível (flat, sem overlay nativo do SO)
- Suporte a tipos: `MUSIC`, `JINGLE`, `VINHETA`, `SPOT`, `HORA_CERTA`

**Blocos comerciais** são exibidos como um único grupo visual com borda laranja, agrupando o header do bloco e cada spot, em vez de linhas independentes.

**Reordenação** (modo ASSIST) — drag-and-drop e botões ↑ / ↓ por item; remoção com ✕.

### Menu de contexto (modo AUTO)

Clique com o botão direito sobre qualquer item da fila (exceto o em reprodução) para exibir o menu:

- **Excluir da fila** — remove o item imediatamente; para blocos comerciais, exige confirmação e remove todos os spots do bloco em sequência.

### CUE — Preview de auditória

Permite ouvir um áudio da fila antes que ele seja reproduzido no ar, sem interromper a transmissão. O áudio de preview é roteado para o **dispositivo de saída separado** configurado no RadioCore.

**Acionamento pela fila:**

1. Itens com status `QUEUED` ou `PRELOADING` exibem o botão `◎` na coluna de ações.
2. Ao clicar, um **painel flutuante** aparece no canto inferior direito com:
   - Título e artista do áudio
   - Barra de progresso com seek (clique para saltar)
   - Controles Play / Pause / Stop
   - Tempo decorrido e duração total
3. O botão `◎` na row pulsa (animação de escala + brilho ciano) enquanto o CUE está ativo.
4. Clicar novamente no botão ativo (ou no Stop do painel) encerra o preview.

**Acionamento pelo Catálogo:**

- Botão `◎` em cada faixa abre um painel inline abaixo do item com os mesmos controles.
- Progresso atualizado em tempo real via evento WebSocket `PreviewProgress`.

### Controles de playback

- Play / Pause / Stop / Skip
- Barra de progresso do item atual com tempo decorrido e restante
- Crossfade visual — item anterior em fade-out enquanto o próximo inicia

### Modo ASSIST

- Operação manual: o engine aguarda confirmação antes de avançar para o próximo item
- Botão **Avançar** para confirmar o próximo item
- Banner visual persistente enquanto em modo ASSIST
- Enfileiramento de Hora Certa pelo operador
- Limpeza total da fila

### Modo PANIC

- Entra em loop de áudio de emergência configurado no RadioCore
- Banner vermelho de alerta em tela cheia
- Saída controlada via botão "Sair do Panic"

### Loudness / Saúde do Áudio

- Medição EBU R128 em tempo real: Momentary (M), Short-term (S), Integrated (I)
- Indicadores de pico e LUFS
- Monitor de saúde: estado do decodificador, buffer, latência e alertas de silêncio

### VU Meter

- Barras L e R com medição RMS em tempo real
- Peak hold com decay visual
- Alerta de clipping

### Catálogo (modal)

Acessado pelo botão **♪ Catálogo** na barra de controles. Abre um pop-up flutuante (arrastável) para busca de faixas no Library Service.

- Filtros: texto livre (título/artista), tipo (`MUSIC`, `VINHETA`, `JINGLE`, `SPOT`), artista e álbum
- Resultados paginados em tabela com título, artista, tipo e duração
- Botão `+ Fila` por faixa para enfileirar diretamente
- Botão `◎` por faixa para CUE inline (preview antes de enfileirar)

### Biblioteca (drawer lateral)

Acessado pelo botão **☰ Biblioteca** na barra de controles. Abre um painel lateral com duas abas:

| Aba | Conteúdo |
|---|---|
| Playlists | Lista de playlists cadastradas; enfileiramento completo com um clique |
| Breaks | Lista de blocos comerciais; enfileiramento com um clique |

---

## Controle de volume

Dois sliders independentes na coluna esquerda:

| Slider | Canal | Endpoint |
|---|---|---|
| Principal | Fila de reprodução | `PUT /v1/playback/volume` |
| Player (CUE) | Preview / auditória | `PUT /v1/preview/volume` |

Ambos operam de `0%` (mudo) a `100%` (sem atenuação). O valor é convertido para escala linear `[0.0, 1.0]` antes de ser enviado.

**Inicialização** — os sliders são sincronizados com o engine via `StateSnapshot` (WebSocket) ao conectar, com fallback para `GET /v1/playback/volume` e `GET /v1/preview/volume`.

**Sincronização** — eventos `VolumeChanged` e `PreviewVolumeChanged` mantêm os sliders atualizados entre múltiplos clientes simultaneamente. Durante arrasto, o evento é ignorado para evitar salto visual.

**Preview desabilitado** — se o RadioCore iniciar com `preview.enabled: false`, o endpoint retorna `503` e o slider de CUE é desabilitado automaticamente.

---

## Estrutura do projeto

```
player/
├── main.js           processo principal Electron — cria BrowserWindow fullscreen
├── player.html       toda a UI: HTML + CSS + JS em arquivo único
├── package.json      dependências e configuração do electron-builder
├── icon.icns         ícone macOS
├── icon.png          ícone base
├── logo.svg          logotipo RadioFlow
├── radio-flow.png    imagem do header
└── dist/             artefatos gerados pelo build
```

---

## Dependências

| Pacote | Versão | Papel |
|---|---|---|
| `electron` | ^35 | Runtime desktop |
| `electron-builder` | ^25 | Empacotamento e distribuição |

Nenhuma dependência de runtime além do Electron — toda a lógica de UI está em JS vanilla no `player.html`.
