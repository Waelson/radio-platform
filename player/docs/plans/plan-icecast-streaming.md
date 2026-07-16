# Plano de Implementação — Streaming Icecast / SHOUTcast

**Data:** julho de 2026
**Branch:** `feature/icecast-streaming`
**Status:** Em planejamento

---

## 1. Por que esta feature é crítica — problemas de negócio que resolve

### 1.1 O contexto do mercado brasileiro

O rádio brasileiro está passando por uma transformação estrutural. Dados de 2025–2026 revelam:

- O consumo de rádio via streaming cresceu **186%** nos últimos quatro anos no Brasil.
- **7,4 milhões de ouvintes** em 13 regiões metropolitanas já consomem rádio exclusivamente pela internet.
- **30% das emissoras FM** transmitem simultaneamente no ar e na web, com muitas registrando **mais de 50% da audiência no ambiente digital**.
- A Rádio MEC e Rádio Nacional fecharam 2025 com crescimento histórico de audiência — o streaming foi o motor.
- Para 2026, a expectativa é que o áudio programático (publicidade digital em streaming) seja o principal driver de receita para emissoras de médio e pequeno porte.

**Conclusão direta:** uma ferramenta de automação de rádio que não transmite para a internet em 2026 é invisível para a maior parte do mercado. Sem streaming, o RadioFlow não é uma alternativa real para nenhuma emissora que queira alcançar ouvintes além do dial.

### 1.2 Os cinco problemas de negócio que a feature resolve

**Problema 1 — Alcance zero fora do dial.**
Emissoras web-only (sem concessão FM/AM) **só existem através do streaming**. São centenas de emissoras no Brasil que usam Icecast ou SHOUTcast como único veículo de transmissão. Sem essa feature, o RadioFlow é inutilizável para esse segmento inteiro.

**Problema 2 — Dupla transmissão (simultcasting).**
Emissoras FM tradicionais querem transmitir simultaneamente no ar e na web. Hoje o operador precisa de um software separado (BUTT, DarkIce, etc.) rodando em paralelo — o que cria risco de desconexão, latência não sincronizada e experiência fragmentada.

**Problema 3 — Metadados para o ouvinte.**
Quando o Icecast/SHOUTcast não recebe metadados, players de internet (VLC, apps de rádio, agregadores como TuneIn) mostram "Stream" ou nada. O ouvinte não sabe o que está tocando. Isso reduz engajamento e impossibilita integração com plataformas de descoberta musical.

**Problema 4 — Múltiplos streams simultâneos.**
Emissoras profissionais precisam de vários bitrates simultâneos: alta qualidade para desktop (192kbps MP3 ou Opus), média qualidade para móvel (96kbps), baixa qualidade para conexões lentas (64kbps). Sem gestão centralizada, isso exige um encoder separado por stream.

**Problema 5 — Monitoramento e reconexão.**
Servidores Icecast/SHOUTcast caem. Conexões de internet oscilam. Sem reconexão automática, o stream some do ar até alguém perceber manualmente — que pode ser horas depois.

### 1.3 Impacto no negócio

| Problema | Sem streaming | Com streaming |
|----------|--------------|---------------|
| Alcance geográfico | Apenas área de cobertura FM | Global |
| Público web-only | Zero | 100% do mercado web |
| Metadados em players | "Stream" anônimo | Título + artista em tempo real |
| Dupla transmissão | Software externo separado | Integrado, gerenciado pelo RadioFlow |
| Monitoramento | Manual, reativo | Automático, proativo |

---

## 2. Como os concorrentes resolvem este problema

### 2.1 RadioBOSS (DJSoft) — benchmark nacional e internacional

O RadioBOSS tem suporte **nativo e completo** para streaming desde a versão 4.x:

- **Protocolos suportados:** SHOUTcast v1, SHOUTcast v2, Icecast 2, Windows Media Services, RTMP.
- **Formatos de áudio:** MP3 (todos), AAC, Ogg Vorbis, Ogg Opus.
- **Metadados:** envio automático de título/artista ao servidor de streaming, atualização a cada troca de faixa. Suporte a upload via FTP, HTTP GET/POST, arquivo texto e XML.
- **Múltiplos destinos:** até N streams simultâneos com configurações independentes de servidor, bitrate e formato.
- **Reconexão:** automática com intervalo configurável.
- **Testes de conexão:** botão "Test" na configuração antes de ativar o stream.
- **Integração com a automação:** os metadados enviados ao Icecast são os mesmos exibidos no painel — não há disparidade.

**Diferencial do RadioBOSS:** o encoder roda dentro do mesmo processo, usando a mesma pipeline de áudio do player. Não há processo externo visível para o operador.

### 2.2 mAirList — referência alemã

mAirList tem um módulo de encoder integrado chamado simplesmente "Encoder":

- **Protocolos:** Icecast 2, SHOUTcast v1 e v2.
- **Formatos:** MP3 (SHOUTcast e Icecast), Ogg Vorbis, Ogg Opus (apenas Icecast).
- **Múltiplas conexões simultâneas:** sem limite documentado.
- **Modo headless:** pode rodar o encoder sem nenhum hardware de áudio — direto de arquivos ou pipeline — ideal para servidores em datacenter.
- **Fallback de stream:** se o stream principal cair, pode automaticamente redirecionar para um mount de fallback no mesmo servidor Icecast.
- **Integração com agendamento:** events no clock podem ligar/desligar streams específicos (ex.: silenciar stream de patrocinador fora do horário contratado).

### 2.3 RCS Zetta — referência enterprise

RCS Zetta não tem encoder nativo de Icecast/SHOUTcast embutido. A abordagem é via **integração com encoders externos**:

- O Rocket Broadcaster lê os metadados de título/artista do Zetta via formato XML proprietário (Zetta Lite XML) e os envia ao Icecast/SHOUTcast.
- O áudio é capturado da placa de som da estação, não da pipeline digital do software.
- Esta abordagem introduz latência adicional (D/A → A/D) e depende de hardware de áudio dedicado.

**Ponto fraco do Zetta:** a dependência de ferramenta externa torna a configuração mais complexa e aumenta pontos de falha.

### 2.4 Liquidsoap — referência open-source

Usado pela **Radio France** para streaming de 77 estações de rádio:

- Linguagem de scripting própria que define fontes de áudio, efeitos e saídas.
- Suporte nativo a Icecast, SHOUTcast, RTMP, HLS.
- Pipeline de DSP completo incluindo normalização, compressão, limiting e EQ.
- Reconexão automática, fallback entre fontes, prioridades.
- Integrado com FFmpeg para encoders adicionais.

**Ponto fraco do Liquidsoap:** curva de aprendizado altíssima para operadores não-técnicos. Configuração via arquivos de script, não via interface gráfica.

### 2.5 PlayIt Live — sem suporte nativo

PlayIt Live **não tem suporte nativo a streaming**. Usuários precisam de BUTT, DarkIce ou outro encoder externo. Este é um dos principais pontos fracos listados em reviews — e uma das razões pelas quais o RadioFlow já está à frente desta solução.

### 2.6 Tabela comparativa de streaming

| Capacidade | RadioFlow (atual) | RadioBOSS | mAirList | RCS Zetta | Liquidsoap |
|---|---|---|---|---|---|
| Icecast 2 nativo | 🔲 | ✅ | ✅ | ⚠️ externo | ✅ |
| SHOUTcast v1 | 🔲 | ✅ | ✅ | ⚠️ externo | ✅ |
| SHOUTcast v2 | 🔲 | ✅ | ✅ | ⚠️ externo | ✅ |
| Múltiplos streams | 🔲 | ✅ | ✅ | ⚠️ externo | ✅ |
| Metadados automáticos | 🔲 | ✅ | ✅ | ⚠️ via XML | ✅ |
| Reconexão automática | 🔲 | ✅ | ✅ | ⚠️ no encoder | ✅ |
| MP3 | 🔲 | ✅ | ✅ | ⚠️ externo | ✅ |
| Ogg/Opus | 🔲 | ✅ | ✅ | 🔲 | ✅ |
| AAC | 🔲 | ✅ | 🔲 | ⚠️ externo | ✅ |
| Stats / listeners | 🔲 | ✅ | ✅ | ⚠️ via painel | ✅ |
| UI integrada | 🔲 | ✅ | ✅ | ⚠️ parcial | 🔲 script |

---

## 3. Requisitos de negócio

### 3.1 Funcionais obrigatórios

**RN-01** — O sistema deve suportar transmissão simultânea ao Icecast 2.

**RN-02** — O sistema deve suportar transmissão simultânea ao SHOUTcast v1 e v2.

**RN-03** — O operador deve poder configurar múltiplos destinos de streaming independentes (mínimo 4 simultâneos).

**RN-04** — Cada destino deve ter configuração independente de: host, porta, mount point, senha, formato de áudio e bitrate.

**RN-05** — O sistema deve enviar automaticamente metadados de título e artista da faixa atual ao servidor de streaming a cada troca de faixa.

**RN-06** — O sistema deve reconectar automaticamente ao servidor de streaming após desconexão, com intervalo configurável.

**RN-07** — O operador deve poder conectar e desconectar cada stream independentemente via UI, sem interromper o áudio local.

**RN-08** — O streaming deve funcionar de forma completamente independente do áudio local — uma falha no stream não deve interromper a reprodução no estúdio.

**RN-09** — Faixas sem marcadores de cue points devem ser transmitidas normalmente (do início ao fim).

**RN-10** — O sistema deve publicar eventos WebSocket quando o estado de cada stream mudar (conectado, desconectado, erro).

### 3.2 Funcionais desejáveis

**RN-11** — Exibição do número de ouvintes conectados em tempo real (quando o servidor Icecast expuser a API de estatísticas).

**RN-12** — Suporte a múltiplos formatos: MP3, Ogg Vorbis, Ogg Opus, AAC-LC.

**RN-13** — Botão "Testar conexão" antes de salvar a configuração.

**RN-14** — Badge de status de streaming visível no painel principal do player (sem precisar abrir a tela de configuração).

**RN-15** — Log de eventos de streaming (conexão, desconexão, erros) persistido no Library Service junto ao log de transmissão.

**RN-16** — Suporte a reconexão com backoff exponencial (1s, 2s, 4s, 8s, 30s, 60s, ...).

### 3.3 Não funcionais

**RNF-01** — Latência do stream em relação ao áudio local: máximo 10 segundos (aceitável para rádio internet; inerente ao buffering de streaming).

**RNF-02** — Uma falha de streaming (processo FFmpeg terminado, servidor indisponível) não deve causar underrun no buffer de áudio local.

**RNF-03** — O overhead de CPU do encoder não deve ultrapassar 15% do núcleo em operação normal (stream 128kbps MP3).

**RNF-04** — O sistema deve suportar reconexão sem reinicialização do RadioCore.

**RNF-05** — Configuração deve ser persistida entre reinicializações do engine.

---

## 4. Fluxo de utilização

### 4.1 Fluxo de configuração inicial (operador técnico)

```
1. Operador abre o player → clica em "Streaming" no menu lateral
2. Tela de Streaming exibe lista vazia (nenhum servidor configurado)
3. Operador clica em [+ Adicionar servidor]
4. Modal de configuração abre
5. Operador preenche: tipo (Icecast), host, porta, mount, senha, formato (MP3), bitrate (128kbps)
6. Operador clica em [Testar Conexão]
   → Sistema tenta conectar brevemente ao servidor
   → Exibe: "✅ Servidor encontrado e credenciais válidas" ou "❌ Falha: connection refused"
7. Operador clica em [Salvar]
8. Stream aparece na lista com status "Desconectado"
9. Operador clica em [Conectar]
10. Status muda para "Conectando..." → "● LIVE"
11. Badge de LIVE aparece no header do player
```

### 4.2 Fluxo durante transmissão ao vivo

```
1. Operador inicia reprodução normal (fila ou rotação automática)
2. Engine toca a faixa localmente (CoreAudio)
3. Simultaneamente: os mesmos PCM frames são enviados ao encoder de streaming
4. Encoder FFmpeg converte para MP3 128kbps e envia ao Icecast
5. A cada troca de faixa: engine atualiza metadados no servidor
   → VLC / app de rádio do ouvinte exibe "Artista — Título"
6. Operador pode ver: "247 ouvintes conectados" (se Icecast expõe stats)
```

### 4.3 Fluxo de reconexão automática

```
1. Servidor Icecast fica indisponível (reinício, falha de rede)
2. FFmpeg detecta erro de conexão e termina
3. StreamingManager detecta encerramento do processo
4. Publica evento WebSocket: StreamingDisconnected {reason: "connection lost"}
5. Badge no player muda para "● RECONECTANDO" (amarelo piscante)
6. StreamingManager aguarda o intervalo de backoff (ex.: 5s, 10s, 20s...)
7. Tenta reconectar: novo processo FFmpeg com as mesmas configurações
8. Se bem-sucedido: publica StreamingConnected, badge volta a "● LIVE"
9. Se falhar 10 vezes consecutivas: muda para "● OFFLINE" e para de tentar
   (operador pode clicar em [Reconectar] manualmente)
```

### 4.4 Fluxo de múltiplos streams

```
1. Operador configura 3 streams: 192kbps (alta), 96kbps (móvel), 64kbps (2G)
2. Todos conectam simultaneamente
3. A mesma pipeline de PCM alimenta os 3 encoders em paralelo
4. Se o stream de 64kbps cair, os outros dois continuam sem impacto
5. Tela de streaming mostra status individual de cada um
```

---

## 5. Proposta de solução técnica

### 5.1 Visão geral da abordagem

A solução adota uma **arquitetura de tap na pipeline de áudio**: após o mixer processar os frames PCM e antes de enviá-los ao output device local (CoreAudio), os frames são copiados para um canal Go que alimenta um conjunto de **StreamingTargets**, cada um gerenciando um processo FFmpeg filho responsável por encodar e transmitir ao servidor de streaming.

Esta abordagem:
- **Isola completamente** o streaming do áudio local — falhas no streaming não afetam o output principal.
- **Reutiliza FFmpeg** já presente no sistema (decoder), sem nova dependência.
- **Escala para N streams** sem overhead linear perceptível até ~8 streams simultâneos.
- **Evita CGO** — sem libshout, sem bindings C.
- **Encapsula** toda a lógica de reconexão, metadata e protocolo dentro do StreamingManager.
- **Library Service é o dono da configuração** — credenciais e parâmetros de streaming são persistidos no SQLite do Library Service, nunca no `config.yaml` do playout. O Playout Engine recebe a configuração completa (incluindo senha) apenas no momento do connect, mantendo-a somente em memória durante a transmissão.

### 5.2 Por que FFmpeg como encoder

O RadioCore já usa FFmpeg como decoder (subprocesso via pipe). Usar FFmpeg também como encoder streaming é consistente e traz:

- **Suporte nativo a Icecast e SHOUTcast** sem implementar o protocolo HTTP ICY manualmente.
- **Todos os codecs** (MP3 via libmp3lame, Opus via libopus, Vorbis via libvorbis, AAC via libfdk-aac ou aac nativo).
- **Metadados ICY** via opção `-ice_name`, `-ice_description`, `-ice_genre`, e via atualização do título com `-metadata title=`.
- **Reconexão**: o StreamingManager simplesmente mata e reinicia o processo FFmpeg.

Comando FFmpeg para stream MP3 128kbps para Icecast:
```bash
ffmpeg -hide_banner -loglevel warning \
  -f f32le -ar 48000 -ac 2 -i pipe:0 \
  -c:a libmp3lame -b:a 128k -ar 44100 \
  -f mp3 \
  -ice_name "Rádio XYZ" \
  -ice_description "A melhor rádio do Brasil" \
  -ice_genre "Pop" \
  -content_type audio/mpeg \
  icecast://source:senha@stream.xyz.com:8000/ao-vivo
```

Comando para SHOUTcast v1:
```bash
ffmpeg -f f32le -ar 48000 -ac 2 -i pipe:0 \
  -c:a libmp3lame -b:a 128k \
  -f mp3 \
  icecast://source:senha@stream.xyz.com:8000/stream
  # SHOUTcast v1 usa o mesmo protocolo com senha no campo source
```

Atualização de metadados em tempo real (sem reiniciar o stream):
```
PUT http://source:senha@host:port/admin/metadata?mount=/ao-vivo&mode=updinfo&song=Artista%20-%20Titulo
```

---

## 6. Desenho de arquitetura — alto nível

```
┌──────────────────────────────────┐
│        LIBRARY SERVICE           │
│  SQLite: streaming_targets       │
│  GET/POST/PUT/DELETE /v1/streaming│
└──────────┬───────────────────────┘
           │ config completa (com senha)
           ▼
┌──────────────────────────────────┐
│         PLAYER (Electron)        │
│  1. Busca config no Library      │
│  2. Exibe UI de streaming        │
│  3. POST /v1/streaming/:id/connect│
│     body: { config completa }   │
└──────────┬───────────────────────┘
           │ HTTP + WebSocket
           ▼
╔══════════════════════════════════════════════════════════════╗
║                    RADIOCORE PLAYOUT ENGINE                  ║
║                                                              ║
║  ┌──────────────┐    ┌──────────────┐    ┌───────────────┐  ║
║  │  API Server  │───▶│ Command Bus  │───▶│  Dispatcher   │  ║
║  └──────────────┘    └──────────────┘    └───────┬───────┘  ║
║         ▲                                        │           ║
║         │ WebSocket                              ▼           ║
║  ┌──────────────┐    ┌──────────────┐    ┌───────────────┐  ║
║  │  Event Bus   │◀───│ State Mgr    │◀───│Playback Mgr   │  ║
║  └──────────────┘    └──────────────┘    └───────┬───────┘  ║
║                                                   │ PCM      ║
║                                          ┌────────▼───────┐  ║
║                                          │    Mixer        │  ║
║                                          └────────┬───────┘  ║
║                                    ┌──────────────┤           ║
║                                    │              │ tap       ║
║                                    ▼              ▼           ║
║                          ┌──────────────┐ ┌─────────────┐   ║
║                          │ CoreAudio /  │ │  Streaming  │   ║
║                          │ PortAudio    │ │   Manager   │   ║
║                          └──────────────┘ └──────┬──────┘   ║
║                            (local speakers)       │           ║
║                                          ┌────────┴────────┐  ║
║                                          │   PCM Fan-out   │  ║
║                                          └┬──────┬──────┬──┘  ║
║                                           │      │      │      ║
║                                    ┌──────▼┐ ┌───▼──┐ ┌▼──┐  ║
║                                    │Target1│ │Target│ │...│  ║
║                                    │FFmpeg │ │FFmpeg│ │   │  ║
║                                    └───────┘ └──────┘ └───┘  ║
║                                        │        │             ║
║                                        ▼        ▼             ║
║                                   Icecast   SHOUTcast         ║
║                                   Server    Server            ║
╚══════════════════════════════════════════════════════════════╝
```

### 5.3 Componentes novos

**Playout Engine (`playout/`):**

```
/internal/streaming/
  manager.go        — StreamingManager: ciclo de vida de N targets
  target.go         — StreamingTarget: processo FFmpeg individual
  types.go          — StreamingTargetConfig (in-memory) e StreamingTargetStatus
  metadata.go       — atualização de metadados via HTTP ao servidor
  reconnect.go      — estratégia de backoff exponencial
  stats.go          — polling de estatísticas do Icecast (listeners, bitrate)

/internal/api/handlers/
  streaming.go      — handlers REST (connect/disconnect/test/stats apenas — sem CRUD)

/internal/events/
  types.go          — novos eventos: StreamingConnected, StreamingDisconnected, StreamingStats
```

> Nota: o Playout **não tem `config.go` de streaming** nem bloco `streaming:` no `config.yaml`. A configuração chega somente no momento do connect, no corpo da requisição.

**Library Service (`library/`):**

```
/internal/store/
  streaming_store.go  — CRUD de destinos no SQLite (streaming_targets table)

/internal/api/handlers/
  streaming.go        — handlers REST: GET/POST/PUT/DELETE /v1/streaming

/migrations/
  NNNN_create_streaming_targets.sql  — migration de criação da tabela
```

---

## 7. Modelo de dados

### 7.1 Configuração — SQLite no Library Service

A configuração de streaming é persistida no banco SQLite do Library Service, nunca em YAML. O Playout Engine não armazena configurações de streaming em disco.

**Migration SQL (`library/migrations/NNNN_create_streaming_targets.sql`):**

```sql
CREATE TABLE IF NOT EXISTS streaming_targets (
    id                   TEXT PRIMARY KEY,           -- ULID gerado pelo Library
    name                 TEXT NOT NULL,
    enabled              INTEGER NOT NULL DEFAULT 1,
    type                 TEXT NOT NULL,              -- 'icecast' | 'shoutcast_v1' | 'shoutcast_v2'
    host                 TEXT NOT NULL,
    port                 INTEGER NOT NULL,
    mount                TEXT NOT NULL DEFAULT '',   -- apenas Icecast
    password             TEXT NOT NULL,              -- armazenado em plaintext no SQLite local
    format               TEXT NOT NULL DEFAULT 'mp3',-- 'mp3' | 'ogg_vorbis' | 'ogg_opus' | 'aac'
    bitrate_kbps         INTEGER NOT NULL DEFAULT 128,
    sample_rate          INTEGER NOT NULL DEFAULT 44100,
    channels             INTEGER NOT NULL DEFAULT 2,
    send_metadata        INTEGER NOT NULL DEFAULT 1,
    station_name         TEXT NOT NULL DEFAULT '',
    station_description  TEXT NOT NULL DEFAULT '',
    station_genre        TEXT NOT NULL DEFAULT '',
    station_url          TEXT NOT NULL DEFAULT '',
    reconnect_enabled    INTEGER NOT NULL DEFAULT 1,
    reconnect_max_retries INTEGER NOT NULL DEFAULT 0,-- 0 = infinito
    reconnect_initial_delay_sec INTEGER NOT NULL DEFAULT 2,
    reconnect_max_delay_sec     INTEGER NOT NULL DEFAULT 60,
    reconnect_backoff_multiplier REAL NOT NULL DEFAULT 2.0,
    auto_connect         INTEGER NOT NULL DEFAULT 0, -- conectar ao iniciar o playout
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT NOT NULL DEFAULT (datetime('now'))
);
```

> O campo `password` é armazenado em plaintext no SQLite local (arquivo protegido por permissões do SO). Não é enviado em respostas GET a não ser quando o Player solicita a configuração completa para repassar ao Playout no momento do connect.

### 7.2 Estruturas Go

**Library Service (`library/internal/store/streaming_store.go`):**

```go
// StreamingTarget — entidade persistida no SQLite do Library Service
type StreamingTarget struct {
    ID                 string  `json:"id"`
    Name               string  `json:"name"`
    Enabled            bool    `json:"enabled"`
    Type               string  `json:"type"`     // "icecast" | "shoutcast_v1" | "shoutcast_v2"
    Host               string  `json:"host"`
    Port               int     `json:"port"`
    Mount              string  `json:"mount"`
    Password           string  `json:"password"` // omitido em respostas GET normais
    Format             string  `json:"format"`   // "mp3" | "ogg_vorbis" | "ogg_opus" | "aac"
    BitrateKbps        int     `json:"bitrate_kbps"`
    SampleRate         int     `json:"sample_rate"`
    Channels           int     `json:"channels"`
    SendMetadata       bool    `json:"send_metadata"`
    StationName        string  `json:"station_name"`
    StationDescription string  `json:"station_description"`
    StationGenre       string  `json:"station_genre"`
    StationURL         string  `json:"station_url"`
    Reconnect          ReconnectConfig `json:"reconnect"`
    AutoConnect        bool    `json:"auto_connect"`
    CreatedAt          string  `json:"created_at"`
    UpdatedAt          string  `json:"updated_at"`
}

type ReconnectConfig struct {
    Enabled           bool    `json:"enabled"`
    MaxRetries        int     `json:"max_retries"`
    InitialDelaySec   int     `json:"initial_delay_sec"`
    MaxDelaySec       int     `json:"max_delay_sec"`
    BackoffMultiplier float64 `json:"backoff_multiplier"`
}
```

**Playout Engine (`playout/internal/streaming/types.go`):**

```go
// StreamingTargetConfig — configuração em memória recebida no connect.
// Nunca persistida em disco pelo Playout.
type StreamingTargetConfig struct {
    ID                 string
    Name               string
    Type               string  // "icecast" | "shoutcast_v1" | "shoutcast_v2"
    Host               string
    Port               int
    Mount              string
    Password           string  // mantido apenas em memória durante a transmissão
    Format             string
    BitrateKbps        int
    SampleRate         int
    Channels           int
    SendMetadata       bool
    StationName        string
    StationDescription string
    StationGenre       string
    StationURL         string
    Reconnect          ReconnectConfig
}

type ReconnectConfig struct {
    Enabled           bool
    MaxRetries        int
    InitialDelaySec   int
    MaxDelaySec       int
    BackoffMultiplier float64
}

// StreamingTargetStatus — estado em tempo real (em memória)
type StreamingTargetStatus struct {
    ID            string     `json:"id"`
    State         string     `json:"state"` // "connected" | "connecting" | "disconnected" | "error"
    ConnectedAt   *time.Time `json:"connected_at,omitempty"`
    LastErrorMsg  string     `json:"last_error,omitempty"`
    RetryCount    int        `json:"retry_count"`
    NextRetryAt   *time.Time `json:"next_retry_at,omitempty"`
    Listeners     int        `json:"listeners"`
    BytesSent     int64      `json:"bytes_sent"`
    UptimeMS      int64      `json:"uptime_ms"`
    CurrentTitle  string     `json:"current_title"`
    CurrentArtist string     `json:"current_artist"`
}
```

---

## 8. Endpoints e contratos

### 8.1 Library Service — GET /v1/streaming

Lista todos os destinos de streaming configurados (sem senha no response).

**Response 200:**
```json
{
  "ok": true,
  "data": [
    {
      "id": "stream-principal",
      "name": "Stream Principal",
      "type": "icecast",
      "host": "stream.minharadio.com.br",
      "port": 8000,
      "mount": "/ao-vivo",
      "format": "mp3",
      "bitrate_kbps": 128,
      "enabled": true,
      "auto_connect": false,
      "send_metadata": true,
      "station_name": "Rádio XYZ",
      "reconnect": { "enabled": true, "initial_delay_sec": 2, "max_delay_sec": 60, "backoff_multiplier": 2.0 }
    }
  ]
}
```

> O campo `password` **não** é retornado neste endpoint. Apenas no endpoint de connect interno descrito abaixo.

---

### 8.2 Library Service — POST /v1/streaming

Cria e persiste um novo destino de streaming no SQLite.

**Body:**
```json
{
  "name": "Stream Principal",
  "type": "icecast",
  "host": "stream.minharadio.com.br",
  "port": 8000,
  "mount": "/ao-vivo",
  "password": "supersenha",
  "format": "mp3",
  "bitrate_kbps": 128,
  "send_metadata": true,
  "station_name": "Rádio XYZ",
  "reconnect": { "enabled": true, "initial_delay_sec": 2, "max_delay_sec": 60 }
}
```

**Response 201:**
```json
{ "ok": true, "data": { "id": "01J5X...", "name": "Stream Principal", ... } }
```

O ID é gerado pelo Library Service (ULID).

---

### 8.3 Library Service — PUT /v1/streaming/:id

Atualiza a configuração de um destino no SQLite. O Player é responsável por notificar o Playout se o stream estiver conectado (desconectar e reconectar com nova config).

---

### 8.4 Library Service — DELETE /v1/streaming/:id

Remove o destino do SQLite.

**Response 200:**
```json
{ "ok": true }
```

---

### 8.5 Playout Engine — POST /v1/streaming/:id/connect

Conecta ao servidor de streaming. O **Player busca a configuração completa (com senha) no Library Service** e a envia no body desta requisição. O Playout mantém a config somente em memória.

**Body (config completa vinda do Library):**
```json
{
  "id": "stream-principal",
  "name": "Stream Principal",
  "type": "icecast",
  "host": "stream.minharadio.com.br",
  "port": 8000,
  "mount": "/ao-vivo",
  "password": "supersenha",
  "format": "mp3",
  "bitrate_kbps": 128,
  "send_metadata": true,
  "station_name": "Rádio XYZ",
  "reconnect": { "enabled": true, "initial_delay_sec": 2, "max_delay_sec": 60, "backoff_multiplier": 2.0 }
}
```

**Response 200:**
```json
{ "ok": true, "data": { "state": "connecting" } }
```

**Response 409 — já conectado:**
```json
{ "ok": false, "error": "already_connected", "message": "stream já está conectado" }
```

---

### 8.6 POST /v1/streaming/:id/disconnect

Desconecta o stream e cancela reconexão automática até o próximo connect explícito.

**Response 200:**
```json
{ "ok": true, "data": { "state": "disconnected" } }
```

---

### 8.7 POST /v1/streaming/:id/test

Testa a conexão com o servidor sem iniciar o stream de fato. Útil antes de salvar a configuração.

**Body:** mesma estrutura do POST /v1/streaming (pode ser uma config ainda não salva).

**Response 200:**
```json
{ "ok": true, "data": { "reachable": true, "server_version": "Icecast 2.4.4", "latency_ms": 42 } }
```

**Response 200 (falha de conexão):**
```json
{ "ok": true, "data": { "reachable": false, "error": "connection refused" } }
```

---

### 8.8 GET /v1/streaming/:id/stats

Retorna estatísticas detalhadas do stream (polling da API de administração do Icecast).

**Response 200:**
```json
{
  "ok": true,
  "data": {
    "id": "stream-principal",
    "listeners": 247,
    "listener_peak": 512,
    "bytes_sent": 138412032,
    "uptime_ms": 15960000,
    "bitrate_kbps": 128,
    "server_type": "Icecast",
    "server_version": "2.4.4"
  }
}
```

---

### 8.9 Novos eventos WebSocket

```json
// Conectado com sucesso
{ "type": "StreamingConnected", "payload": {
    "target_id": "stream-principal",
    "name": "Stream Principal",
    "host": "stream.minharadio.com.br",
    "mount": "/ao-vivo",
    "format": "mp3",
    "bitrate_kbps": 128
}}

// Desconectado (por falha ou comando)
{ "type": "StreamingDisconnected", "payload": {
    "target_id": "stream-principal",
    "reason": "connection lost",    // ou "manual" para desconexão pelo operador
    "retry_in_ms": 2000
}}

// Erro irrecuperável (retries esgotados)
{ "type": "StreamingError", "payload": {
    "target_id": "stream-principal",
    "error": "max retries exceeded",
    "retry_count": 10
}}

// Metadados atualizados no servidor
{ "type": "StreamingMetadataUpdated", "payload": {
    "target_id": "stream-principal",
    "title": "Bam Bam",
    "artist": "Sister Nancy"
}}

// Estatísticas periódicas (a cada 30s)
{ "type": "StreamingStats", "payload": {
    "target_id": "stream-principal",
    "listeners": 247,
    "bytes_sent": 138412032,
    "uptime_ms": 15960000
}}
```

---

### 8.11 Endpoints existentes impactados

| Serviço | Endpoint | Impacto |
|---|---|---|
| Playout | `GET /v1/status` | Adicionar bloco `streaming: [{id, state, listeners}]` no response (apenas estado em memória, sem config) |
| Playout | Event `NowPlayingChanged` | StreamingManager escuta este evento para atualizar metadados no Icecast |
| Library | `GET /v1/streaming` | Novo endpoint (CRUD) — sem senha no response |

---

## 9. Impacto na UI — novas telas e modificações

### 9.1 Badge de streaming no header do player

O header atual ganha um badge de status visível em todas as abas:

```
┌─────────────────────────────────────────────────────────────────────┐
│ 🎙 RadioFlow  │ AUTO │ [⏸] [⏭] [🔇] [VOL ████░] │ ● LIVE 247♪ │
└─────────────────────────────────────────────────────────────────────┘
                                                       ↑
                                            clicável → abre modal de streaming
```

- **Verde pulsante** `● LIVE 247♪` — ao menos 1 stream conectado.
- **Amarelo piscante** `● RECONECTANDO` — tentando reconectar.
- **Cinza** `○ OFFLINE` — nenhum stream ativo.
- Número `247♪` é o total de ouvintes somados de todos os streams.

---

### 9.2 Aba "Streaming" no drawer lateral

Nova aba no drawer lateral (ao lado de Fila, Botoneira, Catálogo, Histórico):

```
┌────────────────────────────────────────────────────────────────────┐
│  ≡  Fila   Botoneira   Catálogo   Histórico   📡 Streaming        │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  📡 STREAMING AO VIVO                    [+ Adicionar servidor]    │
│                                                                     │
│ ┌─────────────────────────────────────────────────────────────┐    │
│ │ ● Stream Principal                               [LIVE]     │    │
│ │   stream.minharadio.com.br:8000/ao-vivo                     │    │
│ │   MP3 128kbps · 247 ouvintes · uptime: 4h23m               │    │
│ │   Sister Nancy — Bam Bam                                    │    │
│ │                              [Desconectar]  [✏ Editar]     │    │
│ └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│ ┌─────────────────────────────────────────────────────────────┐    │
│ │ ○ Stream Backup                                [OFF]        │    │
│ │   backup.minharadio.com.br:8000/backup                     │    │
│ │   MP3 96kbps                                               │    │
│ │                              [Conectar]     [✏ Editar]     │    │
│ └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│ ┌─────────────────────────────────────────────────────────────┐    │
│ │ ⚠ Stream Opus                         [RECONECTANDO...]    │    │
│ │   stream2.xyz.com:8000/opus                                 │    │
│ │   Ogg Opus 64kbps · Tentativa 3/∞ · próxima em 8s         │    │
│ │                              [Cancelar]     [✏ Editar]     │    │
│ └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
└────────────────────────────────────────────────────────────────────┘
```

**Cores dos cards:**
- Verde (borda esquerda) = conectado.
- Amarelo (borda esquerda piscante) = reconectando.
- Cinza (borda esquerda) = desconectado.
- Vermelho (borda esquerda) = erro permanente (retries esgotados).

---

### 9.3 Modal de Adicionar / Editar servidor

```
┌──────────────────────────────────────────────────────────────────┐
│  Configurar Servidor de Streaming                          [✕]   │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  Nome / Apelido                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Stream Principal                                           │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  Tipo de servidor                                                 │
│  (●) Icecast 2    ( ) SHOUTcast v1    ( ) SHOUTcast v2          │
│                                                                   │
│  Host / IP                                    Porta              │
│  ┌──────────────────────────────────┐  ┌──────────────────┐      │
│  │ stream.minharadio.com.br         │  │ 8000             │      │
│  └──────────────────────────────────┘  └──────────────────┘      │
│                                                                   │
│  Mount Point  (Icecast)               Senha                      │
│  ┌──────────────────────────────────┐  ┌──────────────────┐      │
│  │ /ao-vivo                         │  │ ••••••••         │      │
│  └──────────────────────────────────┘  └──────────────────┘      │
│                                                                   │
│  Formato de áudio             Bitrate                            │
│  ┌──────────────────────┐    ┌──────────────────────────────┐   │
│  │ MP3            ▼     │    │ 128 kbps               ▼     │   │
│  └──────────────────────┘    └──────────────────────────────┘   │
│  (MP3 / Ogg Vorbis / Ogg Opus / AAC)                            │
│                                                                   │
│  ─── Informações da Estação ────────────────────────────────── ─ │
│  Nome da rádio               Gênero                              │
│  ┌──────────────────────────┐ ┌────────────────────────────┐    │
│  │ Rádio XYZ                │ │ Pop                        │    │
│  └──────────────────────────┘ └────────────────────────────┘    │
│                                                                   │
│  ─── Comportamento ─────────────────────────────────────────── ─ │
│  [✓] Enviar metadados (título/artista) ao servidor               │
│  [✓] Reconexão automática após desconexão                        │
│       Intervalo inicial: [2s]  Máximo: [60s]  Multiplicador: [2×]│
│  [ ] Conectar automaticamente ao iniciar o RadioCore             │
│                                                                   │
│                   [Cancelar]  [🔌 Testar Conexão]  [💾 Salvar]  │
└──────────────────────────────────────────────────────────────────┘
```

**Comportamento do botão [Testar Conexão]:**
- Mostra spinner "Testando..."
- Em caso de sucesso: `✅ Servidor Icecast 2.4.4 respondeu em 38ms`
- Em caso de falha: `❌ Falha: connection refused (porta 8000 fechada)`

---

### 9.4 Notificações toast no player

Eventos de streaming geram toasts discretos no canto inferior direito:
```
┌────────────────────────────────────┐
│ ● Stream Principal conectado       │  ← verde, desaparece em 3s
└────────────────────────────────────┘

┌────────────────────────────────────┐
│ ⚠ Stream Backup desconectado       │  ← amarelo, desaparece em 5s
│   Reconectando em 8s...            │
└────────────────────────────────────┘
```

---

## 10. Detalhamento técnico da implementação

### 10.1 Tap de PCM na pipeline de áudio

O Mixer atual escreve frames para um único `OutputDevice`. Vamos adicionar um **tap** não-bloqueante:

```go
// Mixer adiciona um canal opcional de tap
type Mixer struct {
    // ...campos existentes...
    streamingTap chan<- []float32  // nil quando streaming inativo
}

// Em cada iteração do loop de áudio:
func (m *Mixer) writeFrame(frames []float32) {
    m.output.Write(ctx, frames)          // output principal (CoreAudio)

    if m.streamingTap != nil {
        select {
        case m.streamingTap <- copyOf(frames): // cópia para evitar race
        default: // tap cheio: descarta frame (prefere não bloquear áudio)
        }
    }
}
```

O canal tem buffer de `tap_buffer_frames` frames. O `select/default` garante que o audio loop nunca bloqueia se o streaming estiver lento.

### 10.2 StreamingManager

```go
// StreamingManager não carrega config de arquivo — recebe configs em memória via AddTarget.
type StreamingManager struct {
    targets  map[string]*StreamingTarget
    tapCh    chan []float32
    eventBus events.Bus
    mu       sync.RWMutex
}

func (sm *StreamingManager) Run(ctx context.Context) {
    for {
        select {
        case frames := <-sm.tapCh:
            sm.fanOut(frames) // distribui para todos os targets ativos
        case <-ctx.Done():
            sm.stopAll()
            return
        }
    }
}

func (sm *StreamingManager) fanOut(frames []float32) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    for _, t := range sm.targets {
        if t.IsConnected() {
            t.Write(frames) // non-blocking: descarta se buffer cheio
        }
    }
}
```

### 10.3 StreamingTarget (processo FFmpeg)

```go
type StreamingTarget struct {
    cfg      StreamingTargetConfig
    status   StreamingTargetStatus
    cmd      *exec.Cmd
    stdin    io.WriteCloser
    writeCh  chan []float32
    stopCh   chan struct{}
    mu       sync.Mutex
}

func (t *StreamingTarget) Connect(ctx context.Context) error {
    args := t.buildFFmpegArgs()
    t.cmd = exec.CommandContext(ctx, "ffmpeg", args...)
    t.stdin, _ = t.cmd.StdinPipe()
    t.cmd.Start()

    go t.writeLoop()
    go t.watchProcess(ctx) // detecta saída do FFmpeg e aciona reconexão
    return nil
}

func (t *StreamingTarget) buildFFmpegArgs() []string {
    url := t.buildStreamURL()
    return []string{
        "-hide_banner", "-loglevel", "warning",
        "-f", "f32le", "-ar", "48000", "-ac", "2", "-i", "pipe:0",
        "-c:a", t.encoder(), "-b:a", fmt.Sprintf("%dk", t.cfg.BitrateKbps),
        "-ar", strconv.Itoa(t.cfg.SampleRate),
        "-ice_name", t.cfg.StationName,
        "-ice_genre", t.cfg.StationGenre,
        "-content_type", t.contentType(),
        "-f", t.ffmpegFormat(),
        url,
    }
}

func (t *StreamingTarget) buildStreamURL() string {
    switch t.cfg.Type {
    case "icecast":
        return fmt.Sprintf("icecast://source:%s@%s:%d%s",
            t.cfg.Password, t.cfg.Host, t.cfg.Port, t.cfg.Mount)
    case "shoutcast_v1":
        return fmt.Sprintf("icecast://:%s@%s:%d/stream",
            t.cfg.Password, t.cfg.Host, t.cfg.Port)
    }
}

// Write é chamado pelo fanOut — non-blocking
func (t *StreamingTarget) Write(frames []float32) {
    select {
    case t.writeCh <- frames:
    default: // buffer cheio: descarta
    }
}
```

### 10.4 Atualização de metadados

```go
// MetadataUpdater — envia título/artista ao Icecast via HTTP GET
type MetadataUpdater struct{}

func (u *MetadataUpdater) Update(ctx context.Context, cfg StreamingTargetConfig, artist, title string) error {
    song := url.QueryEscape(artist + " - " + title)
    endpoint := fmt.Sprintf("http://%s:%d/admin/metadata?mount=%s&mode=updinfo&song=%s",
        cfg.Host, cfg.Port, cfg.Mount, song)

    req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
    req.SetBasicAuth("source", cfg.Password)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return fmt.Errorf("metadata update: %w", err)
    }
    defer resp.Body.Close()
    return nil
}
```

### 10.5 Reconexão com backoff exponencial

```go
type ReconnectStrategy struct {
    cfg     ReconnectConfig
    retries int
    delay   time.Duration
}

func (r *ReconnectStrategy) Next() (time.Duration, bool) {
    if r.cfg.MaxRetries > 0 && r.retries >= r.cfg.MaxRetries {
        return 0, false // stop
    }
    if r.retries == 0 {
        r.delay = time.Duration(r.cfg.InitialDelaySec) * time.Second
    } else {
        r.delay = time.Duration(float64(r.delay) * r.cfg.BackoffMultiplier)
        if r.delay > time.Duration(r.cfg.MaxDelaySec)*time.Second {
            r.delay = time.Duration(r.cfg.MaxDelaySec) * time.Second
        }
    }
    r.retries++
    return r.delay, true
}
```

### 10.6 Polling de estatísticas Icecast

O Icecast 2 expõe uma API XML de status em `/admin/stats`. O StreamingManager consulta esse endpoint a cada 30 segundos:

```
GET http://admin:password@host:port/admin/stats
```

O response XML contém `<listeners>`, `<listener_peak>`, `<server_version>`, etc.

---

## 11. Fases de implementação

### Fase 1 — Library Service: CRUD de streaming targets

1. Criar branch `feature/icecast-streaming` a partir de `main`.
2. Criar migration SQL `library/migrations/NNNN_create_streaming_targets.sql`.
3. Implementar `library/internal/store/streaming_store.go` com `Create`, `List`, `Get`, `Update`, `Delete`.
4. Implementar `library/internal/api/handlers/streaming.go` com os 4 handlers REST.
5. Registrar rotas no servidor do Library Service.
6. Escrever testes unitários do store (SQLite in-memory).

**Entregável:** CRUD de destinos de streaming persistido no Library Service (sem playout envolvido).

---

### Fase 1b — Setup do Playout Engine

1. Criar estrutura de diretórios: `playout/internal/streaming/`.
2. Criar `types.go` com `StreamingTargetConfig` (in-memory, sem yaml tags) e `StreamingTargetStatus`.
3. **Não adicionar** bloco `streaming:` ao `config.yaml` — o Playout não persiste config de streaming.

**Entregável:** tipos base do playout de streaming definidos.

---

### Fase 2 — StreamingTarget: FFmpeg subprocess

1. Implementar `target.go` com ciclo de vida: `Connect() → writeLoop() → watchProcess() → Disconnect()`.
2. Implementar `buildFFmpegArgs()` para Icecast (MP3).
3. Implementar `writeLoop()` — float32 → stdin do FFmpeg com conversão de endianness se necessário.
4. Implementar `watchProcess()` — detecta saída do FFmpeg e aciona canal de erro.
5. Testes unitários com um servidor Icecast mock (HTTP server simples que aceita PUT).

**Entregável:** um único target conecta ao Icecast e transmite PCM como MP3.

---

### Fase 3 — StreamingManager e PCM tap

1. Implementar `manager.go`: inicialização, `Run()`, `fanOut()`, `AddTarget()`, `RemoveTarget()`.
2. Modificar o Mixer para expor um `SetStreamingTap(ch chan<- []float32)`.
3. Conectar StreamingManager ao Mixer via tap no `cmd/playout-engine/main.go`.
4. Implementar publicação de eventos via Event Bus: `StreamingConnected`, `StreamingDisconnected`.
5. Testes: múltiplos targets em paralelo, drop de frames quando buffer cheio.

**Entregável:** áudio local e streaming funcionando simultaneamente; eventos no WebSocket.

---

### Fase 4 — Reconexão automática

1. Implementar `reconnect.go` com `ReconnectStrategy` e backoff exponencial.
2. Integrar reconexão no `watchProcess()` do StreamingTarget.
3. Publicar evento `StreamingDisconnected` com `retry_in_ms` antes de cada tentativa.
4. Publicar `StreamingError` quando retries são esgotados.
5. Testes: simular falha do processo FFmpeg, verificar reconexão e backoff.

**Entregável:** reconexão automática com backoff; eventos corretos publicados.

---

### Fase 5 — Metadados e atualização em tempo real

1. Implementar `metadata.go` com `MetadataUpdater.Update()`.
2. StreamingManager escuta evento `NowPlayingChanged` do Event Bus.
3. Ao receber `NowPlayingChanged`, dispara `MetadataUpdater.Update()` para todos os targets conectados.
4. Publicar `StreamingMetadataUpdated` após sucesso.
5. Suporte a SHOUTcast v1 (metadados via ICY protocol no PUT body).
6. Testes: mock HTTP de `/admin/metadata`, verificar chamada na troca de faixa.

**Entregável:** metadados de título/artista atualizados no Icecast a cada faixa.

---

### Fase 6 — Playout API REST (connect/disconnect/test/stats)

1. Implementar `playout/internal/api/handlers/streaming.go` com:
   - `POST /v1/streaming/:id/connect` — recebe config completa no body, sem CRUD.
   - `POST /v1/streaming/:id/disconnect`
   - `POST /v1/streaming/:id/test` — tenta conexão TCP ao host:porta e retorna resultado.
   - `GET /v1/streaming/:id/stats` — polling Icecast XML.
2. Registrar rotas em `server.go`.
3. Atualizar `GET /v1/status` para incluir bloco `streaming` (estado em memória, sem config).
4. Testes de handlers com `httptest.NewRecorder`.

**Entregável:** Playout expõe endpoints de controle de stream (sem persistência).

---

### Fase 7 — Formatos adicionais: Ogg Opus, Ogg Vorbis, AAC

1. Estender `buildFFmpegArgs()` para os 4 formatos.
2. Validar que os codecs necessários estão disponíveis no FFmpeg instalado.
3. Adicionar validação de formato no endpoint de criação/edição.
4. Testes: argumentos FFmpeg corretos para cada formato.

**Entregável:** todos os formatos suportados.

---

### Fase 8 — Estatísticas e polling de ouvintes

1. Implementar `stats.go` — client HTTP para `/admin/stats` XML do Icecast.
2. StreamingManager faz polling a cada 30s para cada target conectado.
3. Publicar `StreamingStats` via WebSocket.
4. Testes com mock XML do Icecast.

**Entregável:** número de ouvintes visível em tempo real via WebSocket.

---

### Fase 9 — UI no Player

1. Adicionar nova aba "Streaming" no drawer lateral.
2. Implementar a lista de targets com status colorido.
3. Implementar o modal de Adicionar/Editar servidor.
4. Implementar botão [Testar Conexão] com feedback visual.
5. Implementar badge `● LIVE 247♪` no header.
6. Implementar toasts de conexão/desconexão.
7. Consumir WebSocket events: `StreamingConnected`, `StreamingDisconnected`, `StreamingStats`.

**Entregável:** UI completa para gerenciamento de streams.

---

### Fase 10 — Testes, ajustes e PR

1. `go test ./...` e `go test -race ./...` em playout e library.
2. `go vet ./...`.
3. Testes manuais: Icecast real + VLC + app de rádio.
4. Teste de resistência: simular 4h de transmissão contínua.
5. Teste de reconexão: matar o processo Icecast e verificar recovery.
6. Atualizar `docs/benchmark.md` — marcar "Streaming Icecast/SHOUTcast" como ✅.
7. Commit e PR para `main`.

---

## 12. Riscos e mitigações

| Risco | Probabilidade | Impacto | Mitigação |
|---|---|---|---|
| FFmpeg não tem `libmp3lame` na instalação do usuário | Alta | Alto | Detectar na inicialização via `ffmpeg -encoders \| grep mp3lame`; exibir mensagem clara de instalação |
| Bloqueio do audio loop por I/O de streaming | Média | Crítico | Canal com `select/default` garante que o fanOut nunca bloqueia; tap é sempre descartável |
| Servidor Icecast não tem API `/admin/stats` (SHOUTcast) | Alta | Baixo | Stats de listeners são opcionais; stream funciona sem elas |
| Latência do stream causa out-of-sync com áudio local | Baixa | Baixo | Inerente ao streaming; documentado como comportamento esperado (5–10s) |
| Senha de streaming exposta em logs do Playout | Média | Alto | Redact de senha em todos os logs do Playout; senha nunca salva em disco pelo engine; Library Service protege o SQLite via permissões do SO |
| SQLite do Library corrompido (senha perdida) | Baixa | Médio | SQLite WAL mode; backups recomendados; senha pode ser recadastrada pelo operador |
| Float32 endianness — plataformas big-endian | Baixa | Médio | Testar em Linux x86-64; documentar suporte |
| SHOUTcast v2 protocolo binário diferente | Média | Médio | Usar FFmpeg como abstração; FFmpeg suporta SHOUTcast v2 nativamente via `icecast://` |
| Múltiplos streams usando 100% CPU de um núcleo | Baixa | Médio | Monitor de CPU no status; documentar limite prático de ~4–6 streams em hardware moderno |

---

## 13. Pontos diferenciais competitivos

### 13.1 O que faremos melhor que os concorrentes

**1. Metadados ricos (além de título/artista)**
Enquanto RadioBOSS envia apenas `Artista — Título`, o RadioFlow pode enviar metadados estendidos ao Icecast:
- Nome do programa atual (ex.: "Manhã Brasileira")
- Tipo de áudio (MUSIC, JINGLE, SPOT)
- Duração restante da faixa
- Próxima faixa (lookahead)

Isso permite que apps de rádio integrados (próprios ou de terceiros) exibam informações muito mais ricas.

**2. Stream condicional por tipo de áudio**
Possibilidade futura: configurar por target quais tipos de áudio transmitir. Exemplo: um stream de "música pura" que interrompe o streaming durante spots comerciais (substituindo por arquivo de fallback). Nenhum concorrente oferece isso de forma integrada.

**3. Dashboard de streaming integrado ao histórico**
Combinar os dados de `StreamingStats` (ouvintes por hora) com o `transmission_log` (o que tocou) para gerar relatórios de "pico de audiência por faixa". Dado de enorme valor para programadores.

**4. Interface em português, sem jargão técnico**
RadioBOSS e mAirList têm interfaces de streaming com terminologia de engenharia de rede. O RadioFlow usará linguagem de rádio: "Conectar", "Desconectar", "Ao vivo", "Ouvintes".

### 13.2 Pontos de atenção para o futuro

- **HLS (HTTP Live Streaming):** crescente adoção em apps móveis. FFmpeg suporta HLS; pode ser adicionado como `type: "hls"` sem mudança na arquitetura.
- **RTMP:** para streamers que usam OBS como fonte de voz ao vivo. A mesma arquitetura de tap suporta RTMP como destino FFmpeg.
- **Relay:** receber um stream de entrada (locutor remoto) e relay + mixar com a automação. Evolução natural do StreamingManager.

---

## 14. Dependências e pré-requisitos

| Dependência | Tipo | Observação |
|---|---|---|
| FFmpeg com `libmp3lame` | Runtime | Verificar na inicialização |
| FFmpeg com `libopus` | Runtime (opcional) | Para formato Ogg Opus |
| Servidor Icecast 2.4+ | Infraestrutura | Responsabilidade da emissora |
| Go 1.24+ | Build | Já exigido pelo projeto |
| Nenhuma nova dependência Go | — | Implementação com stdlib + `os/exec` |

---

*Fontes consultadas para este plano:*
- [RadioBOSS — Broadcasting Internet Radio](https://manual.djsoft.net/radioboss/en/broadcasting_internet_radio.htm)
- [mAirList — Encoder Connections](https://mairlist.docs.mairlist.com/features/encoder/connections/)
- [Icecast Protocol Specification](https://gist.github.com/ePirat/adc3b8ba00d85b7e3870)
- [FFmpeg Commands for Streaming 2026](https://www.shoutcastnet.com/school/ffmpeg.php)
- [Liquidsoap Radio Automation 2026](https://www.shoutcastnet.com/blogs/liquidsoap.php)
- [Streaming de rádio avança no Brasil — Sindiradio](https://www.sindiradio.org.br/noticias/item/streaming-de-radio-avanca-em-consumo-e-torna-se-estrategico-para-emissoras-brasileiras.html)
- [Rádio Web cresce no Brasil 2025](https://portal.saladanoticia.com.br/noticia/24679/radio-web-cresce-no-brasil-emissoras-brasileiras-ganham-destaque-em-rankings-internacionais-em-2025)
- [goicy — AAC/MP3 Icecast client in Go](https://github.com/stunndard/goicy)
- [AzuraCast Streaming Software](https://www.azuracast.com/docs/user-guide/streaming-software/)
