# RadioFlow — Player

Interface de operação para o [RadioCore Playout Engine](../playout/README.md).
Construída em **Electron + HTML/CSS/JS puro**, sem frameworks — uma única tela fullscreen que se conecta ao engine via REST e WebSocket.

---

## Índice

- [Visão geral](#visão-geral)
- [Pré-requisitos](#pré-requisitos)
- [Instalação e execução](#instalação-e-execução)
- [Build e distribuição](#build-e-distribuição)
- [Configuração de URLs](#configuração-de-urls)
- [Funcionalidades](#funcionalidades)
- [Estrutura do projeto](#estrutura-do-projeto)

---

## Visão geral

O RadioFlow é a interface de operação ao vivo do sistema de automação de rádio. Conecta-se ao **RadioCore** (playout engine) via:

- **REST** — envio de comandos (play, pause, fila, panic, hot buttons, preview)
- **WebSocket** (`GET /v1/events`) — recebimento de eventos em tempo real (estado, progresso, VU meter, fila)
- **Library Service** (`http://127.0.0.1:8081`) — busca e enfileiramento de faixas, playlists e blocos

---

## Pré-requisitos

- [Node.js](https://nodejs.org/) ≥ 18
- [RadioCore](../playout/README.md) em execução (porta `8080` por padrão)
- Radio Library Service em execução (porta `8081` por padrão) — opcional para a Biblioteca

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
# Gerar RadioFlow.app (macOS arm64)
npm run build
# → dist/mac-arm64/RadioFlow.app
# → dist/RadioFlow-0.1.0-arm64.dmg
```

O `electron-builder` gera também `.AppImage` (Linux) e `.exe` NSIS (Windows) conforme a plataforma.

---

## Configuração de URLs

Por padrão o player conecta em:

| Serviço | URL padrão |
|---|---|
| Playout Engine (REST) | `http://127.0.0.1:8080` |
| Playout Engine (WebSocket) | `ws://127.0.0.1:8080/v1/events` |
| Library Service | `http://127.0.0.1:8081` |

Para apontar para endereços diferentes, passe parâmetros na query string ao carregar o `player.html`:

```
player.html?api=http://192.168.1.10:8080&ws=ws://192.168.1.10:8080/v1/events&lib=http://192.168.1.10:8081
```

---

## Funcionalidades

### Fila de reprodução
- Visualização em tempo real da fila (item atual em destaque)
- Reordenação por drag-and-drop
- Remoção de itens individuais
- Suporte a tipos: `music`, `jingle`, `vinheta`, `hora_certa`
- Badge de estado por item: `QUEUED`, `PLAYING`, `PLAYED`, `SKIPPED`, `FAILED`

### Controles de playback
- Play / Pause / Stop / Skip
- Barra de progresso e tempo decorrido/restante do item atual
- Indicador de estado do engine (IDLE, PLAYING, PAUSED, PANIC)

### Modo ASSIST
- Ativa operação manual (o engine aguarda comando para avançar)
- Botão "Avançar" para confirmar próximo item
- Enfileiramento de Hora Certa e limpeza de fila pelo operador
- Banner visual de aviso enquanto em modo ASSIST

### Modo PANIC
- Entra em loop de áudio de emergência
- Banner vermelho de alerta
- Saída via botão "Sair do Panic"

### VU Meter
- Medição RMS em tempo real (canais L e R)
- Indicador de pico e nível LUFS
- Alerta visual de silêncio

### Hot Buttons
- Painel de botões configuráveis para disparo de áudios instantâneos
- Feedback visual de reprodução em andamento

### Biblioteca (drawer lateral)
- **Aba Áudio** — busca de faixas por título/artista; enfileiramento com duplo clique ou botão `+ Fila`
- **Aba Playlists** — enfileiramento de playlists completas
- **Aba Blocos** — enfileiramento de blocos comerciais

### Preview / CUE
- Botão `◎` em cada faixa da Biblioteca para ouvir o áudio antes de enfileirar
- Painel inline abaixo do item com Play / Pause / Stop e barra de progresso com seek
- Áudio roteado para dispositivo de saída separado (configurado no RadioCore)
- Progresso atualizado em tempo real via eventos WebSocket (`PreviewProgress`)

---

## Estrutura do projeto

```
player/
├── main.js          — processo principal Electron (cria BrowserWindow fullscreen)
├── player.html      — toda a UI: HTML + CSS + JS em arquivo único
├── package.json     — dependências e configuração do electron-builder
├── icon.icns        — ícone macOS
├── icon.png         — ícone base
├── logo.svg         — logotipo RadioFlow
└── dist/            — artefatos gerados pelo build
```
