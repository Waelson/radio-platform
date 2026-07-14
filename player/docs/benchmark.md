# Benchmark RadioFlow — Comparação com Soluções de Mercado

**Data:** julho de 2026
**Versão RadioFlow:** 0.4.0 (em desenvolvimento ativo)

---

## 1. Escopo e metodologia

Este documento compara o RadioFlow com as principais soluções de automação de rádio disponíveis no mercado, com ênfase em produtos brasileiros. O objetivo é identificar lacunas de funcionalidade e priorizar o roadmap.

**Critério de avaliação por célula:**

| Símbolo | Significado |
|---------|-------------|
| `✅` | Implementado e funcional |
| `🔄` | Planejado (plano em `docs/plans/`) |
| `🔲` | Não implementado, sem plano |
| `—` | Não se aplica ao produto |

---

## 2. Produtos comparados

### Brasileiros

| Produto | Empresa | Perfil |
|---------|---------|--------|
| **RadioPro Prime** | RadioPro Soluções | 15+ anos no mercado, 1.000+ emissoras, suporte a toque e duplo monitor |
| **EBRcart2** | EBRaudio | Cart machine digital, foco em sonoplastia ao vivo, used em rádio, teatro e shows |
| **AudioMaster** | Access Web | Sistema modular para emissoras com foco no mercado regional brasileiro |

### Internacionais (referências de mercado)

| Produto | Empresa | Perfil |
|---------|---------|--------|
| **RCS Zetta** | RCS Sound Software | Padrão de mercado em grandes emissoras, full-featured, altíssimo custo |
| **RadioBOSS** | DJSoft.Net | Popular entre emissoras pequenas e médias, Windows, preço acessível |
| **mAirList** | mAirList GmbH | Profissional europeu, alta customização, usado em rádios públicas e comerciais |
| **PlayIt Live** | PlayIt Software | Gratuito, leve, foco em live assist e internet radio |
| **RadioDJ** | Comunidade | Open source, Windows, sem custo, ecosystem de plugins |

---

## 3. Tabela comparativa por área funcional

### 3.1 Reprodução e controle de fila

| Feature | RadioFlow | RadioPro | EBRcart2 | RCS Zetta | RadioBOSS | mAirList | PlayIt Live | RadioDJ |
|---------|:---------:|:--------:|:--------:|:---------:|:---------:|:--------:|:-----------:|:-------:|
| Play / Pause / Stop | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Skip de faixa | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Fila de reprodução dinâmica | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Drag-and-drop na fila | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Crossfade configurável por tipo | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Marcadores de intro/outro/cue | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Tempo estimado da fila | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Preview (CUE) antes de tocar | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Skimming (ouvir entrada rápida) | 🔲 | ✅ | — | ✅ | 🔲 | ✅ | 🔲 | 🔲 |
| Multi-deck (2+ players simultâneos) | 🔲 | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | ✅ |

### 3.2 Modos de operação

| Feature | RadioFlow | RadioPro | EBRcart2 | RCS Zetta | RadioBOSS | mAirList | PlayIt Live | RadioDJ |
|---------|:---------:|:--------:|:--------:|:---------:|:---------:|:--------:|:-----------:|:-------:|
| Modo AUTO (piloto automático) | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Modo ASSIST (operador controla avanço) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Modo PANIC (interrupção de emergência) | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | 🔲 |
| Failover / backup de transmissão | 🔲 | ✅ | — | ✅ | ✅ | ✅ | 🔲 | 🔲 |
| Voice tracking (gravação de offs) | 🔲 | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |

### 3.3 Botoneira (cart machine / hotkeys)

| Feature | RadioFlow | RadioPro | EBRcart2 | RCS Zetta | RadioBOSS | mAirList | PlayIt Live | RadioDJ |
|---------|:---------:|:--------:|:--------:|:---------:|:---------:|:--------:|:-----------:|:-------:|
| Botoneira com acionamento instantâneo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Múltiplos perfis de botoneira | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Botoneira em janela flutuante | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | — | ✅ |
| Botoneira integrada ao painel lateral | ✅ | — | — | ✅ | ✅ | ✅ | — | 🔲 |
| Preview (CUE) dos botões | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Stop individual por botão | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Cores por tipo de áudio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Ducking automático ao acionar botão | 🔲 | ✅ | — | ✅ | ✅ | ✅ | 🔲 | 🔲 |
| Atalhos de teclado para botões | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Controle via hardware (GPI) | 🔲 | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | 🔲 |

### 3.4 Agendamento

| Feature | RadioFlow | RadioPro | EBRcart2 | RCS Zetta | RadioBOSS | mAirList | PlayIt Live | RadioDJ |
|---------|:---------:|:--------:|:--------:|:---------:|:---------:|:--------:|:-----------:|:-------:|
| Agendamento por horário (cron) | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Countdown para próximo evento | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | 🔲 |
| Hora Certa automática | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Inserção automática de jingles | 🔲 | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Rotação musical por formato (clock) | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Agendamento de breaks comerciais | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Grade semanal / programação futura | 🔲 | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Trigger após faixa atual (AFTER_CURRENT) | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | 🔲 |
| Trigger de interrupção (INTERRUPT) | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |

### 3.5 Biblioteca e catálogo

| Feature | RadioFlow | RadioPro | EBRcart2 | RCS Zetta | RadioBOSS | mAirList | PlayIt Live | RadioDJ |
|---------|:---------:|:--------:|:--------:|:---------:|:---------:|:--------:|:-----------:|:-------:|
| Biblioteca de áudios (SQLite) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Busca simples por texto | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Busca avançada (tipo, artista, álbum) | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Playlists salvas | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Breaks / Blocos comerciais | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Múltiplos tipos de áudio | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Importação automática de pasta | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Extração automática de metadados (ID3 + nome de arquivo) | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Análise de loudness na importação | ✅ | 🔲 | — | ✅ | ✅ | ✅ | 🔲 | 🔲 |
| Gerador automático de playlist | 🔲 | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |

### 3.6 Áudio técnico e monitoramento

| Feature | RadioFlow | RadioPro | EBRcart2 | RCS Zetta | RadioBOSS | mAirList | PlayIt Live | RadioDJ |
|---------|:---------:|:--------:|:--------:|:---------:|:---------:|:--------:|:-----------:|:-------:|
| VU Meters (L/R) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Loudness EBU R128 (LUFS) | ✅ | 🔲 | — | ✅ | ✅ | ✅ | 🔲 | 🔲 |
| Controle de volume principal | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Controle de volume CUE separado | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Normalização automática de volume | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Detecção de silêncio | ✅ | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Underrun / saúde do buffer | ✅ | 🔲 | — | ✅ | 🔲 | ✅ | 🔲 | 🔲 |
| Equalização / processamento de áudio | 🔲 | 🔲 | — | ✅ | ✅ | ✅ | 🔲 | 🔲 |
| Suporte a múltiplas saídas de áudio | 🔲 | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Envio de RDS | 🔲 | ✅ | — | ✅ | ✅ | ✅ | 🔲 | 🔲 |

### 3.7 Integração e conectividade

| Feature | RadioFlow | RadioPro | EBRcart2 | RCS Zetta | RadioBOSS | mAirList | PlayIt Live | RadioDJ |
|---------|:---------:|:--------:|:--------:|:---------:|:---------:|:--------:|:-----------:|:-------:|
| REST API aberta | ✅ | — | — | — | — | ✅ | — | — |
| WebSocket (eventos em tempo real) | ✅ | — | — | ✅ | — | — | — | — |
| Integração com streaming (Icecast/SHOUTcast) | 🔲 | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Controle remoto via web | 🔲 | — | — | ✅ | ✅ | ✅ | 🔲 | — |
| Multi-estúdio / multi-instância | 🔲 | ✅ | — | ✅ | 🔲 | ✅ | — | — |
| Controle por hardware (mesas de corte) | 🔲 | ✅ | ✅ | ✅ | ✅ | ✅ | — | — |
| API para traffic systems (publicidade) | 🔲 | ✅ | — | ✅ | 🔲 | ✅ | 🔲 | — |

### 3.8 Gestão operacional e relatórios

| Feature | RadioFlow | RadioPro | EBRcart2 | RCS Zetta | RadioBOSS | mAirList | PlayIt Live | RadioDJ |
|---------|:---------:|:--------:|:--------:|:---------:|:---------:|:--------:|:-----------:|:-------:|
| Log de transmissão (o que tocou e quando) | ✅ | ✅ | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| Relatório ECAD (direitos autorais) | ✅ | ✅ | — | — | — | — | — | — |
| Prova de veiculação (declaração de comerciais) | 🔲 | ✅ | — | ✅ | ✅ | ✅ | 🔲 | — |
| Gestão de contratos comerciais | 🔲 | ✅ | — | ✅ | 🔲 | ✅ | 🔲 | — |
| Relatório de programação futura | 🔲 | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Notificações (datas comemorativas, feriados) | 🔲 | ✅ | — | — | 🔲 | — | — | — |
| Pedidos musicais / promoções | 🔲 | ✅ | — | — | — | — | — | — |

### 3.9 Interface e usabilidade

| Feature | RadioFlow | RadioPro | EBRcart2 | RCS Zetta | RadioBOSS | mAirList | PlayIt Live | RadioDJ |
|---------|:---------:|:--------:|:--------:|:---------:|:---------:|:--------:|:-----------:|:-------:|
| App desktop (instalável) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Suporte a macOS | ✅ | — | — | — | — | — | — | — |
| Suporte a Windows | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Suporte a Linux | ✅ | — | — | — | — | ✅ | — | — |
| Interface em português | ✅ | ✅ | ✅ | 🔲 | — | — | — | — |
| Suporte a touch screen | 🔲 | ✅ | ✅ | ✅ | ✅ | ✅ | 🔲 | 🔲 |
| Duplo monitor / layout customizável | 🔲 | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |
| Gestão de usuários e permissões | 🔲 | ✅ | — | ✅ | 🔲 | ✅ | 🔲 | — |
| Temas / skin customizável | 🔲 | ✅ | — | ✅ | ✅ | ✅ | 🔲 | ✅ |

---

## 4. Pontos fortes do RadioFlow

Apesar de estar em desenvolvimento inicial, o RadioFlow já apresenta vantagens diferenciadas frente a soluções consolidadas:

1. **Arquitetura de microserviços:** separação clara entre Playout Engine (Go), Library Service (Go) e Player (Electron). Facilita deploys independentes, escalabilidade e testes isolados — algo que nenhum concorrente analisado oferece nativamente.

2. **REST API aberta + WebSocket:** qualquer cliente externo pode consumir a API. Nenhum dos concorrentes tradicionais (RadioBOSS, RadioPro, EBRcart) expõe uma API pública documentada.

3. **Monitoramento EBU R128:** loudness por LUFS já implementado desde o início — algo que o RadioPro não tem e que RCS Zetta e mAirList cobram como premium.

4. **Cross-platform nativo:** macOS, Windows e Linux via Electron. Nenhum concorrente brasileiro suporta macOS ou Linux.

5. **Interface moderna em português:** UI dark-mode com feedback visual em tempo real, sem depender de tradução parcial ou plugin.

6. **Botoneira em janela flutuante + drawer integrado:** solução dual que permite uso com ou sem tela secundária — diferencial frente a PlayIt Live e RadioDJ.

---

## 5. Lacunas prioritárias

As lacunas abaixo foram identificadas como mais impactantes para adoção em emissoras brasileiras, ordenadas por criticidade:

### Alta prioridade (bloqueantes para uso em produção)

| # | Lacuna | Justificativa |
|---|--------|---------------|
| ~~1~~ | ~~**Log de transmissão**~~ | ~~Obrigatório para prestação de contas a anunciantes e ECAD. Todo concorrente tem.~~ ✅ **Implementado no Playout Engine + Library Service.** |
| 2 | **Integração com streaming (Icecast/SHOUTcast)** | Emissoras de internet dependem disso. RadioPro, RadioBOSS e PlayIt Live têm. |
| ~~3~~ | ~~**Importação automática de pasta**~~ | ~~Sem isso, adicionar áudios ao catálogo é manual — inviável para operação contínua.~~ ✅ **Já implementado no Library Service.** |
| ~~4~~ | ~~**Normalização automática de volume**~~ | ~~Sem normalização, o volume varia faixa a faixa — problema grave em emissoras.~~ ✅ **Implementado no Library Service + Playout Engine (EBU R128, por tipo de áudio, gain_db propagado end-to-end até o PCM do cart player).** |
| ~~5~~ | ~~**Marcadores de intro/outro/cue**~~ | ~~Permite crossfade preciso e hora certa sincronizada com a entrada da voz do locutor.~~ ✅ **Implementado no Library Service + Playout Engine + Player (cue_in/intro/outro/cue_out, editor visual de waveform, auto-detecção de silêncio, crossfade preciso no outro_ms, countdown de intro no Now Playing).** |

---

#### Detalhamento — Alta Prioridade

---

##### ~~1. Log de transmissão~~ ✅ Já implementado

> **Este item foi removido das lacunas.** O pipeline completo de log de transmissão está implementado e em produção. O detalhamento abaixo é mantido como referência do que foi entregue.

**O que foi implementado:**
O Playout Engine escreve um arquivo JSONL por hora em disco (`transmission_{date}_{hour}.jsonl`) com cada faixa tocada, incluindo `engine_id`, `asset_id`, `path`, `title`, `artist`, `type`, `isrc`, `composer`, `publisher`, `duration_ms`, `duration_played_ms`, `result`, campos de break e timestamps. O Library Service importa esses arquivos automaticamente via polling com grace period configurável, persiste na tabela `transmission_log` (SQLite) e expõe API REST com filtros, paginação, exportação CSV e exportação ECAD. O Player exibe o histórico na aba **Histórico** com filtros por período, tipo, status e busca por título/artista.

**O que é:**
Registro automático e persistente de tudo que foi ao ar: cada faixa tocada, horário de início, horário de término, duração real, tipo de áudio e operador responsável. Funciona como um "diário de bordo" da emissora.

**Por que é crítico:**
- **Prestação de contas a anunciantes:** o cliente de publicidade exige prova de que seu comercial foi ao ar no horário contratado. Sem log, a emissora não tem como comprovar.
- **Declaração ao ECAD:** emissoras brasileiras são obrigadas por lei a declarar mensalmente ao ECAD (Escritório Central de Arrecadação e Distribuição) todas as músicas executadas, com título, artista, duração e horário. O log é a fonte primária dessas informações.
- **Auditoria interna:** permite que a gerência verifique se a programação foi executada conforme planejado, identificando falhas, silêncios prolongados ou desvios de grade.
- **Resolução de disputas:** conflitos com anunciantes sobre veiculação são resolvidos com base no log.

**Como os concorrentes implementam:**
- **RCS Zetta:** log completo com exportação para traffic systems externos (WideOrbit, Natural Log). Integração nativa com relatórios de auditoria.
- **RadioBOSS:** log de tudo que tocou, exportável em CSV/TXT, com filtros por período, tipo e playlist. Inclui trilhas de cart machine e players auxiliares.
- **mAirList:** log em banco de dados local, com exportação para relatórios de veiculação e integração com sistemas de faturamento.
- **RadioPro:** log com exportação de relatório ECAD no formato aceito pelo órgão, relatório de prova de veiculação para anunciantes e histórico de programação.

**O que implementar no RadioFlow:**

O Playout Engine já publica eventos `ItemStarted` e `ItemFinished` via WebSocket com todos os metadados da faixa. O que falta é:

1. **Persistência no Library Service:** gravar cada evento `ItemStarted`/`ItemFinished` em uma tabela `transmission_log` (SQLite), com campos: `id`, `started_at`, `finished_at`, `duration_played_ms`, `path`, `title`, `artist`, `type`, `playlist_id`, `break_id`, `operator`.
2. **API de consulta:** `GET /v1/log?from=&to=&type=&limit=&offset=` para o player consultar e exibir.
3. **Exportação:** endpoint `GET /v1/log/export?format=csv` para download.
4. **UI no player:** painel ou modal de histórico de transmissão com filtros por data, tipo e busca por título.

**Componentes afetados:** Playout Engine (consumidor de eventos), Library Service (persistência + API), Player UI (painel de histórico).

---

##### 2. Integração com streaming (Icecast / SHOUTcast)

**O que é:**
Capacidade de enviar o sinal de áudio em tempo real para um servidor de streaming, tornando a rádio acessível via internet (player no navegador, apps de rádio, Spotify-like, etc.). Icecast e SHOUTcast são os dois protocolos dominantes no mercado.

**Por que é crítico:**
- Emissoras de internet (web rádios) dependem 100% disso para existir. Sem streaming, a rádio não chega ao ouvinte.
- Emissoras FM tradicionais cada vez mais transmitem simultaneamente via internet para alcançar ouvintes fora do alcance do sinal.
- Plataformas de rádio online (TuneIn, Rádio.com.br, Vagalume.FM) exigem um mount point Icecast ou SHOUTcast para indexar a emissora.
- É uma das primeiras perguntas que qualquer emissora faz ao avaliar um software de automação.

**Como os concorrentes implementam:**
- **RadioBOSS:** integração nativa com Icecast e SHOUTcast. Configuração de bitrate, codec (MP3, AAC, Opus), metadados enviados automaticamente com o título da faixa atual.
- **PlayIt Live:** módulo de streaming integrado. Suporte a múltiplos mount points simultaneamente (ex: qualidade alta para web, qualidade baixa para mobile).
- **mAirList:** encoder interno com suporte a MP3/AAC, envio de metadados via protocolo Icecast/SHOUTcast, e integração com encoders externos via protocolo SAM.
- **RadioPro:** streaming nativo com suporte a múltiplos servidores simultaneamente.

**O que implementar no RadioFlow:**

O Playout Engine já produz áudio PCM float32 internamente. O que falta é um componente de encoding e envio:

1. **Encoder:** converter PCM float32 → MP3 ou AAC via FFmpeg ou biblioteca nativa (LAME, libfdk-aac).
2. **Cliente Icecast/SHOUTcast:** implementar o protocolo de source client (HTTP PUT para Icecast 2, protocolo legado para SHOUTcast 1.x). Bibliotecas existentes: `libshout` (C), ou implementação direta do protocolo HTTP.
3. **Metadados dinâmicos:** atualizar o `StreamTitle` no mount point a cada troca de faixa (evento `ItemStarted`).
4. **Configuração no player:** UI para configurar servidor, porta, senha, mount point, bitrate e codec.
5. **Múltiplos perfis de streaming:** diferentes bitrates para diferentes audiências (128kbps MP3 para web, 64kbps AAC para mobile).

**Componentes afetados:** Playout Engine (novo módulo `audio/streaming`), Player UI (configuração de streaming).

---

##### ~~3. Importação automática de pasta (auto-importer)~~ ✅ Já implementado

> **Este item foi removido das lacunas.** O Library Service já implementa importação automática via watch folder. O detalhamento abaixo é mantido como referência do que foi entregue.

**O que foi implementado:**
Monitoramento contínuo de pastas do sistema de arquivos via `fsnotify`. Quando um novo arquivo de áudio é detectado, o Library Service extrai metadados, verifica o formato, calcula a duração via FFprobe e insere automaticamente no catálogo (`tracks`) sem intervenção manual. Suporte a regras de categorização por pasta (ex: `/watch/musicas` → tipo `MUSIC`) e configuração via API (`/v1/library/watch-folders`).

A extração de metadados suporta dois modos, configuráveis por pasta:
- **ID3 tags:** lê os campos embutidos no arquivo de áudio (título, artista, álbum, ano, gênero).
- **Nome do arquivo:** extrai metadados a partir do padrão do nome do arquivo (ex: `Artista - Título.mp3`), útil quando os arquivos chegam sem tags ID3 preenchidas — comum em material de produtoras e agências brasileiras que entregam áudio com nomenclatura padronizada mas sem tags.

---

##### ~~4. Normalização automática de volume~~ ✅ Já implementado

> **Este item foi removido das lacunas.** A normalização EBU R128 end-to-end está implementada e em produção. O detalhamento abaixo é mantido como referência do que foi entregue.

**O que foi implementado:**
O Library Service analisa cada faixa com `ffmpeg ebur128` e armazena `loudness_lufs` e `true_peak_dbtp` na tabela `tracks`. A análise é disparada automaticamente na importação e pode ser re-executada via `POST /v1/loudness/analyze`. As configurações de normalização (target por tipo de áudio, ceiling dBTP, habilitado/desabilitado) ficam na tabela `normalization_settings` e são gerenciadas via `GET/PUT /v1/normalization/settings`. O `gain_db` é calculado dinamicamente (target − loudness, limitado ao max_gain_db) e retornado em todos os endpoints que produzem faixas para reprodução: `/v1/schedule/generate` (fila principal), `/v1/hotkeys/profile/:id` (botoneira). O player envia o `gain_db` ao Playout Engine, que o aplica como multiplicador linear (`10^(dB/20)`) no hot path de PCM — tanto no pipeline principal quanto no cart player da botoneira. Targets configuráveis por tipo: MUSIC, JINGLE, VINHETA, SPOT.

**O que é:**
Ajuste automático do ganho de reprodução de cada faixa para que todas soem no mesmo nível de volume percebido, independentemente de como foram gravadas ou masterizadas. O padrão de referência adotado pelo mercado de broadcast é o **EBU R128**, que define o loudness em LUFS (Loudness Units Full Scale). Targets comuns: −23 LUFS (broadcast europeu), −16 LUFS (rádio AM/FM), −14 LUFS (plataformas de streaming).

A normalização não altera o arquivo de áudio em disco — ela aplica um ganho digital em tempo real no mixer do engine durante a reprodução, de forma transparente para o operador.

**O problema sem normalização:**

Faixas de épocas e origens diferentes chegam ao catálogo com níveis de loudness radicalmente distintos:

```
Música gravada nos anos 70:     −22 LUFS  (dinâmica ampla, volume baixo)
MPB atual:                      −14 LUFS  (masterização moderna)
Spot comercial de agência:       −8 LUFS  (loudness war, compressão pesada)
Jingle produzido pela rádio:    −12 LUFS
```

Sem normalização, a sequência acima causa variações brutais de volume. O ouvinte aumenta o som para ouvir a música dos anos 70 e leva um susto com o spot comercial. Isso é inaceitável em transmissão profissional e é o principal indicador de amadorismo técnico de uma emissora.

**Como funciona o cálculo de ganho:**

Com o `loudness_lufs` de cada faixa já gravado no banco (pré-requisito: item 19 — análise de loudness na importação), o ganho de correção é calculado no momento em que a faixa é carregada para reprodução:

```
gain_db = target_lufs − track.loudness_lufs

Exemplo:
  target_lufs       = −16.0 LUFS  (configuração da emissora)
  track.loudness_lufs = −22.0 LUFS  (música dos anos 70)
  gain_db           = −16.0 − (−22.0) = +6.0 dB  → aumentar

  target_lufs       = −16.0 LUFS
  track.loudness_lufs =  −8.0 LUFS  (spot comercial pesado)
  gain_db           = −16.0 − (−8.0) = −8.0 dB  → diminuir
```

O ganho é aplicado no pipeline PCM do mixer multiplicando cada sample:

```go
linearGain := math.Pow(10, gainDB/20)
for i, sample := range buf {
    buf[i] = sample * float32(linearGain)
}
```

**Proteção contra clipping:**

Ao aplicar um ganho positivo (aumentar volume), é possível ultrapassar 0 dBFS e causar distorção digital (clipping). A proteção é feita via **true peak limiting**: o ganho máximo é limitado de forma que o true peak da faixa (campo `true_peak_dbtp` gravado na análise) não ultrapasse um ceiling configurável (ex: −1 dBTP).

```
gain_db_max = ceiling_dbtp − track.true_peak_dbtp
gain_db_aplicado = min(gain_db_calculado, gain_db_max)
```

**Targets por tipo de áudio:**

Diferentes tipos de áudio podem ter targets diferentes — é comum em emissoras profissionais aplicar menos normalização em spots (para preservar a intenção criativa da agência) e normalização plena em músicas:

| Tipo | Target sugerido | Justificativa |
|------|----------------|---------------|
| MUSIC | −16 LUFS | Padrão de rádio FM; soa natural |
| JINGLE | −16 LUFS | Mesmo nível da música |
| VINHETA | −14 LUFS | Levemente mais alto para identidade da emissora |
| SPOT | −18 LUFS | Agências entregam material pesado; redução suave evita conflito |
| HORA_CERTA | −16 LUFS | Consistente com a programação |
| EFEITO | −14 LUFS | Precisa ser perceptível sobre a música |

**Como os concorrentes implementam:**

- **RCS Zetta:** normalização por EBU R128 Loudness Gated. LUFS calculado na importação, ganho aplicado em tempo real com true peak limiting configurável. Target por tipo de carteira (music, spot, promo). Opção de normalizar apenas a parte "falada" (gated) ignorando silêncios — mais preciso para faixas com intro longa em silêncio.
- **RadioBOSS:** "Automatic Volume Leveling" com duas modalidades: ReplayGain (padrão mais antigo, menos preciso) ou LUFS (EBU R128). Ganho por faixa aplicado no player. Target global configurável. Opção de re-analisar o catálogo inteiro em batch.
- **mAirList:** normalização por ReplayGain e EBU R128 selecionável por instalação. Análise offline na importação, ganho em tempo real no mixer. Suporte a loudness range (LRA) como critério auxiliar — faixas com LRA muito alto (ex: peça clássica com pianíssimo e fortíssimo) recebem compressão suave antes do ganho.
- **RadioPro:** normalização por tipo de mídia (música, comercial, jingle) com ajuste de ganho fixo por tipo — abordagem simplificada sem medição individual por faixa. Funciona razoavelmente bem quando o catálogo é homogêneo, mas falha com material de loudness muito variado.
- **PlayIt Live / RadioDJ:** sem normalização nativa. Dependem de pré-processamento externo do catálogo (MP3Gain, fre:ac, Audacity em batch) antes de importar.

**O que implementar no RadioFlow:**

O Playout Engine já tem mixer interno operando em PCM float32, e o Library Service já tem a análise de loudness na importação (item 19). A normalização é a conexão entre os dois.

1. **Leitura do LUFS ao carregar a faixa:** no momento em que o Playout Engine recebe um `QueueItem` para reprodução, consultar o Library Service via `GET /v1/tracks/:id` para obter `loudness_lufs` e `true_peak_dbtp`. Caso o campo seja `NULL` (faixa ainda não analisada), aplicar ganho 0 (sem correção) e logar um aviso.

2. **Cálculo e aplicação do ganho no mixer:** calcular `gain_db` conforme a fórmula acima, respeitando o ceiling de true peak. Aplicar o ganho como multiplicador linear em cada frame PCM antes de enviar ao output. O ganho deve ser aplicado suavemente com rampa de alguns milissegundos ao iniciar a faixa — evitar clique de ganho abrupto na transição.

3. **Configuração de targets por tipo:** `GET/PUT /v1/config/normalization` com estrutura:
   ```json
   {
     "enabled": true,
     "ceiling_dbtp": -1.0,
     "targets": {
       "MUSIC":      -16.0,
       "JINGLE":     -16.0,
       "VINHETA":    -14.0,
       "SPOT":       -18.0,
       "HORA_CERTA": -16.0,
       "EFEITO":     -14.0
     }
   }
   ```

4. **Evento WebSocket:** publicar `NormalizationApplied` ao iniciar cada faixa, com `{ "track_id": "...", "loudness_lufs": -22.0, "gain_db": 6.0, "target_lufs": -16.0 }` — permite que o painel do operador exiba o ganho aplicado em tempo real, útil para diagnóstico.

5. **Fallback para faixas sem LUFS:** se `loudness_lufs` for `NULL`, enfileirar a faixa para análise imediata no Library Service (prioridade alta na fila de workers do item 19) e reproduzir sem normalização. Publicar evento `NormalizationSkipped` com motivo `"loudness_not_analyzed"`.

6. **UI no player:** toggle de normalização (liga/desliga globalmente) no painel de configurações. Exibição do ganho aplicado na faixa atual no painel "Now Playing" (ex: `NORM +6.0 dB`). Alerta visual quando uma faixa toca sem normalização (LUFS ausente).

**Dependência crítica:** este item depende diretamente do **item 19 (análise de loudness na importação)**. Sem o LUFS gravado no banco para cada faixa, não há como calcular o ganho de correção. Os dois itens devem ser implementados em conjunto ou em sequência imediata.

**Componentes afetados:** Playout Engine (cálculo de ganho no carregamento da faixa, aplicação no mixer PCM, evento `NormalizationApplied`), Library Service (campo `loudness_lufs` e `true_peak_dbtp` na tabela `tracks`, endpoint de configuração de targets), Player UI (toggle de normalização, exibição de ganho no Now Playing, alerta de faixa sem análise).

---

##### 5. Marcadores de intro / outro / cue point

**O que é:**
Pontos temporais marcados dentro de um arquivo de áudio que definem momentos específicos relevantes para a produção radiofônica:

- **Intro (entrada):** ponto onde a voz do locutor pode entrar sobre a música (geralmente onde a letra começa, após a introdução instrumental). Permite que o locutor "fale sobre a música" no tempo certo.
- **Outro (saída):** ponto onde a música começa a fazer fade out ou onde o próximo áudio deve iniciar o crossfade. Evita crossfade no meio da letra.
- **Cue point:** ponto de início de reprodução (diferente do início real do arquivo — útil para pular silêncios iniciais ou intro longa).
- **Ponto de intro vocal (PFL):** duração da parte instrumental no início, usada para calcular quando o locutor deve parar de falar.

**Por que é crítico:**
- **Profissionalismo ao vivo:** locutores de rádio dependem do marcador de intro para saber exatamente quantos segundos têm para falar antes da letra começar. Sem isso, ou falam demais (cortam a letra) ou ficam em silêncio desnecessário.
- **Crossfade preciso:** o crossfade atual do RadioFlow começa em ponto fixo (ex: 5s antes do fim). Com marcador de outro, o crossfade inicia exatamente onde a música termina de forma musical, evitando cortar a letra ou fazer fade no meio de um acorde.
- **Hora Certa sincronizada:** a hora certa deve tocar exatamente na virada do minuto. Com marcador de outro na música anterior, o engine sabe com precisão quando iniciar o fade para que a hora certa entre no segundo certo.
- **Eliminação de silêncio inicial:** muitos arquivos MP3 têm silêncio no início (artefato de encoding). O cue point elimina esse silêncio sem editar o arquivo.

**Como os concorrentes implementam:**
- **RCS Zetta:** editor de marcadores integrado com visualização de waveform. Suporte a intro, outro, hook (refrão), e segue points. Marcadores gravados em banco de dados, não no arquivo.
- **RadioBOSS:** marcadores de intro e outro editáveis por faixa via waveform editor. Exibe countdown do intro no painel principal para o locutor ("Intro: 00:12").
- **mAirList:** cue points configuráveis (CUE IN, CUE OUT, INTRO, EXTRO). Editor visual de waveform. Integração com hardware de faders para auto-fade no ponto de extro.
- **RadioPro:** "marcadores para intro, refrão, identificação da emissora e pontos de início e fim" — editor próprio integrado ao sistema.
- **PlayIt Live:** marcadores básicos de cue in/out por faixa.

**O que implementar no RadioFlow:**

1. **Modelo de dados:** adicionar campos na tabela `tracks` do Library Service: `cue_in_ms` (ponto de início), `cue_out_ms` (ponto de fim / início do fade), `intro_ms` (duração da intro instrumental), `outro_ms` (ponto onde começa o fade de saída).
2. **API de marcadores:** `PUT /v1/tracks/:id/cuepoints` para salvar marcadores via API.
3. **Editor no player:** visualizador de waveform (via Web Audio API + Canvas) com drag para posicionar marcadores. Exibir duração do intro com destaque visual no painel "Now Playing".
4. **Uso no Playout Engine:**
   - Ao iniciar reprodução: `seek` para `cue_in_ms` via FFmpeg (flag `-ss`).
   - Crossfade: iniciar no `cue_out_ms` em vez de tempo fixo antes do fim.
   - Countdown de intro: publicar evento WebSocket `IntroCountdown` com os milissegundos restantes até o fim da intro — exibido no painel do operador.
5. **Extração automática de silêncio:** ao importar (item 3), detectar silêncio inicial via FFmpeg (`silencedetect`) e sugerir `cue_in_ms` automaticamente.

**Componentes afetados:** Library Service (campos na tabela `tracks`, API de cuepoints), Playout Engine (seek no decoder, lógica de crossfade, evento `IntroCountdown`), Player UI (editor de waveform, countdown de intro no Now Playing).

---

#### Detalhamento — Itens adicionais do comparativo

---

##### Skimming (ouvir entrada rápida)

**O que é:**
Audição automática e padronizada de uma faixa em dois trechos: **alguns segundos do início** seguidos de **alguns segundos antes do fim**, sem interação adicional do operador. O sistema pula automaticamente de um trecho ao outro e para. Tempo típico: 10–15 segundos no início + 10–15 segundos antes do fim.

É diferente do Preview/CUE existente no RadioFlow: o CUE toca do início e o operador para quando quiser. O skimming é **automático, padronizado e não-linear** — serve para revisão rápida em volume, sem exigir atenção contínua.

**Para que serve na prática:**

- **Revisão de catálogo em massa:** um programador recebe 150 músicas novas de uma gravadora. Com skimming, ouve cada uma em ~25 segundos (em vez de 3–4 minutos), descartando as inadequadas e aprovando as boas para o catálogo.
- **Verificação de integridade:** confirmar que o arquivo não está corrompido, sem silêncio excessivo no início, sem clique de encoding, sem corte abrupto no fim — tudo sem ouvir a faixa inteira.
- **Definição de marcadores de intro:** ao ouvir o início, o programador identifica visualmente (com apoio da waveform) onde a letra começa, para depois definir o marcador de intro (item 5 da alta prioridade).
- **Triagem de spots e jingles:** verificar se o material publicitário entregado tem o volume correto e termina de forma limpa — algo que precisa ser feito para cada novo material recebido de agências.

**Como os concorrentes implementam:**
- **RCS Zetta:** botão "Skim" na biblioteca. Toca 10s do início, pula para 10s antes do `cue_out`, para. Configurável por instalação.
- **mAirList:** "Preview mode: intro + outro". Parâmetros de duração configuráveis (padrão: 15s + 15s). Atalho de teclado dedicado.
- **RadioPro:** "pré-escuta" rápida integrada ao catálogo, com avanço automático ao ponto de saída.
- **RadioBOSS:** sem skimming nativo; o operador usa o preview comum e avança manualmente pelo slider.

**O que implementar no RadioFlow:**

1. **Endpoint de skim no Playout Engine:** `POST /v1/preview/skim` com `{ "path": "...", "intro_secs": 12, "outro_secs": 12 }`. O engine toca `intro_secs` segundos do início via player de preview, depois faz seek para `duration - outro_secs` e toca o restante, então para automaticamente.
2. **Uso dos marcadores:** se a faixa já tiver `cue_in_ms` e `cue_out_ms` definidos (item 5), o skim usa esses pontos em vez de tempo fixo — tornando a audição ainda mais precisa.
3. **UI na biblioteca:** botão de skim (ícone distinto do CUE) em cada linha da lista de áudios e nos resultados da busca avançada. Exibir indicador de qual trecho está tocando ("INÍCIO" / "FIM").
4. **Atalho de teclado:** `S` ou `Shift+Space` enquanto uma faixa está selecionada na biblioteca.

**Componentes afetados:** Playout Engine (lógica de skim no player de preview), Player UI (botão de skim na biblioteca e busca avançada).

---

##### Multi-deck (2+ players simultâneos)

**O que é:**
Dois ou mais players de áudio independentes que podem estar **tocando ao mesmo tempo para o ar**, com volumes controlados separadamente pelo operador — como as CDJs de um DJ ou os toca-discos de um estúdio analógico. Cada deck tem seus próprios controles (play, pause, stop, volume, CUE) e ambos alimentam a saída principal simultaneamente.

```
┌─────────────────────────────┐   ┌─────────────────────────────┐
│  DECK A  ▶ NO AR            │   │  DECK B  ⏸ CARREGADO        │
│  Música do Verão            │   │  Notícia das 10h             │
│  ████████████░░░  02:14     │   │  ░░░░░░░░░░░░░░░  00:00      │
│  Vol ──────────●  80%       │   │  Vol ──────────●  100%       │
│  [▶] [⏸] [⏹] [CUE]        │   │  [▶] [⏸] [⏹] [CUE]        │
└─────────────────────────────┘   └─────────────────────────────┘
             ↘                               ↙
        ┌─────────────────────────────────────┐
        │   MIXER  (crossfade manual)          │
        │   A ████████████░░░░  B ░░░░░░░░░░  │
        └─────────────────────────────────────┘
                         ↓
                    SAÍDA PARA O AR
```

**Diferença em relação ao que o RadioFlow tem hoje:**
O RadioFlow possui um player principal (fila de reprodução) e um player de CUE/preview — mas o CUE **não vai para o ar**, serve apenas para audição privada do operador. No multi-deck, **ambos os decks alimentam a saída principal** e o operador faz o crossfade entre eles manualmente.

**Para que serve na prática:**

- **Programas ao vivo com operador de áudio:** o apresentador faz a locução enquanto o operador gerencia a música de fundo no Deck A e sobe uma nota sonora no Deck B no momento certo — sem depender de agendamento automático.
- **Crossfade manual preciso:** o operador ouve a próxima faixa no CUE do Deck B enquanto o Deck A toca, decide o momento exato da transição e faz o fade cruzado — muito mais preciso que o crossfade automático por tempo fixo.
- **Coberturas ao vivo e entradas externas:** Deck A com música de espera, Deck B com áudio do repórter externo ou da mesa de som — o operador escolhe qual está no ar a cada momento.
- **Programas de entretenimento:** dois DJs alternando faixas, vinhetas de entrada e saída em decks diferentes, efeitos sonoros disparados em paralelo à música.
- **Substituição de faixa no ar:** se uma música precisa ser trocada de urgência, o operador carrega a substituta no Deck B, ajusta o volume e faz o corte — sem silêncio.

**Como os concorrentes implementam:**
- **RCS Zetta:** dois decks completos na tela principal com controles independentes, crossfader visual entre eles, e integração com mesas de áudio físicas via protocolo de controle.
- **mAirList:** até 4 decks configuráveis, com crossfader de tela, atalhos de teclado por deck, e suporte a controle via hardware (faders MIDI/HID).
- **RadioBOSS:** dois players principais com crossfade manual via slider, além dos players auxiliares (AUX) para efeitos e jingles.
- **RadioDJ:** Deck A e Deck B com crossfader central, suporte a controladores MIDI externos.
- **EBRcart2:** foco em cart machine, mas permite múltiplos players simultâneos por design — é um deck por botão.
- **PlayIt Live:** um único deck principal; o multi-deck é ausente, o que é sua principal limitação para estúdios ao vivo.

**O que implementar no RadioFlow:**

1. **Múltiplas sessões de playback no Engine:** o Playout Engine atualmente tem uma sessão de playback única. Seria necessário suporte a `N` sessões independentes, cada uma com seu próprio decoder, buffer e controle de volume — misturadas no mixer antes de chegar ao output.
2. **API por deck:** `POST /v1/decks/:id/play`, `POST /v1/decks/:id/stop`, `PUT /v1/decks/:id/volume` — cada deck tratado como uma entidade independente.
3. **Mixer com N canais:** o mixer atual assume uma faixa principal + crossfade. Com multi-deck, precisa somar `N` streams PCM com ganhos independentes.
4. **UI com múltiplos decks:** dois blocos de controle lado a lado no `col-player`, cada um com waveform, progresso, volume e controles próprios. Crossfader central entre Deck A e Deck B.
5. **Integração com o Modo ASSIST:** o Modo ASSIST atual usa a fila linear. Com multi-deck, o ASSIST poderia avançar a fila para o Deck B enquanto o Deck A ainda está tocando — permitindo o operador escolher o momento da transição.

**Impacto na arquitetura:** esta é a mudança estrutural mais significativa do roadmap. Requer refatoração do Playout Engine para suportar múltiplas sessões de playback concorrentes e um mixer de N canais. Recomenda-se tratar como uma versão major (v2.0) após a consolidação das features de alta prioridade.

**Componentes afetados:** Playout Engine (sessões múltiplas, mixer N canais, API por deck), Player UI (layout de multi-deck, crossfader visual).

---

### Média prioridade (diferenciais competitivos)

| # | Lacuna | Justificativa |
|---|--------|---------------|
| 6 | **Failover / backup de transmissão** | Dead air é o pior evento para uma emissora. Mecanismo de contingência automático é obrigatório em operação 24h. |
| 7 | **Voice tracking** | Permite que locutores gravem offs antecipados — essencial para emissoras sem operador 24h. |
| ~~8~~ | ~~**Rotação musical por formato (clock)**~~ | ~~Padrão em emissoras AM/FM. Garante equilíbrio de tipos de áudio ao longo do dia.~~ ✅ **Implementado no Library Service + Player (clocks, categorias musicais, grade semanal de clocks, gerador de playlist com regras de separação, aba Rotação no player).** |
| 9 | **Grade semanal / programação futura** | Operador precisa visualizar e editar a programação dos próximos dias. |
| 10 | **Prova de veiculação** | Relatório para anunciantes provando que os comerciais foram ao ar. |
| 11 | **Gestão de usuários e permissões** | Múltiplos operadores com níveis de acesso diferentes (locutor, técnico, gerente). |
| 12 | **Ducking automático** | Baixar a música automaticamente quando um hot button é acionado. |
| ~~13~~ | ~~**Atalhos de teclado para a botoneira**~~ | ~~Operadores de rádio usam teclado intensivamente para agilidade.~~ ✅ **Implementado no Player + Playout Engine (botoneira com perfis, atalhos de teclado configuráveis por botão, CUE preview na botoneira, drawer lateral integrado).** |

---

#### Detalhamento — Média prioridade

---

##### 6. Failover / backup de transmissão

**O que é:**
Conjunto de mecanismos de contingência que entram em ação automaticamente quando algo dá errado na transmissão principal, garantindo que o ouvinte nunca ouça silêncio. Funciona em duas camadas: proteção interna (dentro do próprio software) e proteção externa (máquina reserva).

Em rádio profissional, silêncio no ar é chamado de **dead air** — o pior evento operacional possível. Causa perda de audiência imediata, pode gerar multa regulatória da ANATEL (para emissoras FM/AM) e danos permanentes à imagem da emissora.

**Camadas de proteção:**

**Camada 1 — Dead air recovery (interna):**
O engine monitora continuamente o nível de saída de áudio. Se detectar silêncio por mais de N segundos (configurável, ex: 10s), aciona automaticamente uma playlist de emergência local — músicas genéricas em loop, sem depender de nenhuma lógica de agendamento ou fila.

```
Saída de áudio = silêncio por 10s
        ↓
Engine aciona playlist de emergência (loop local)
        ↓
Alerta visual no painel do operador
        ↓
Operador restaura a programação normal manualmente
```

**Camada 2 — Queue empty fallback (interna):**
Quando a fila de reprodução esvazia e não há agendamento iminente configurado, em vez de parar, o engine passa a tocar automaticamente uma playlist de preenchimento (fill music). Diferente do dead air recovery — age de forma proativa antes do silêncio acontecer.

```
Fila esvazia + nenhum evento agendado nos próximos Xs
        ↓
Engine carrega playlist de preenchimento (fill music)
        ↓
Toca em loop até operador recompor a fila
```

**Camada 3 — Hot standby (externa):**
Um segundo computador rodando o RadioFlow em paralelo monitora o servidor principal via heartbeat (`GET /v1/health`). Se o principal parar de responder por N segundos consecutivos, o backup assume automaticamente o sinal de áudio — via roteamento por hardware (comutador de áudio) ou por protocolo de streaming (assumindo o mount point no servidor Icecast).

```
Servidor Principal ──────────────► Transmissor / Icecast
        │                                  ▲
        │ heartbeat a cada 5s              │
        ▼                                  │
Servidor Backup monitora                   │
        │                                  │
        └── sem resposta por 15s ──────────┘
            Backup assume a saída
```

**Camada 4 — Stream reconnect (para web rádios):**
Ao perder a conexão com o servidor Icecast/SHOUTcast, o engine tenta reconectar automaticamente com back-off exponencial (ex: 5s, 10s, 20s, 40s...), sem intervenção humana.

**Como os concorrentes implementam:**
- **RCS Zetta:** dead air detection configurável por tipo de saída. Hot standby nativo com failover automático entre instâncias via protocolo proprietário. Playlist de emergência configurável por estúdio.
- **RadioBOSS:** "Emergency Playlist" — ao detectar silêncio ou fila vazia, aciona playlist configurada. Sem hot standby nativo; backup externo via replicação de configuração.
- **mAirList:** "Silence detection" com ação configurável (play emergency file, switch input). Suporte a hot standby via módulo de rede entre instâncias.
- **RadioPro:** playlist de preenchimento automático e detecção de silêncio. Failover externo via hardware.
- **PlayIt Live / RadioDJ:** sem failover nativo. Dependem de soluções externas (scripts, hardware de comutação).

**O que implementar no RadioFlow:**

O Playout Engine já tem a base necessária: monitora silêncio (campo `Silêncio` no painel de saúde) e expõe `GET /v1/health`. O que falta é transformar essa detecção em ação automática:

1. **Dead air recovery:** configurar um threshold de silêncio (`silence_threshold_ms`) no engine. Ao ultrapassar, publicar evento `DeadAirDetected` no Event Bus e acionar automaticamente a playlist de emergência configurada via `PUT /v1/config/emergency-playlist`.

2. **Queue empty fallback:** ao esvaziar a fila sem agendamento iminente, verificar se há uma fill playlist configurada. Se sim, carregar e tocar em loop com prioridade mínima — qualquer item enfileirado manualmente interrompe o fill imediatamente.

3. **Fill playlist configurável:** `GET/PUT /v1/config/fill-playlist` — lista de paths de áudio usados como preenchimento. Separada da fila de reprodução e dos perfis de botoneira.

4. **Heartbeat e hot standby:** o endpoint `GET /v1/health` já existe. Documentar o protocolo de failover para que uma segunda instância do RadioFlow possa monitorá-lo e assumir em caso de falha. Publicar evento `EngineStarted` ao inicializar, para que o backup saiba que o principal voltou.

5. **Stream reconnect:** ao implementar o módulo de streaming (item 2 da alta prioridade), incluir lógica de reconexão automática com back-off exponencial e alerta visual no painel do operador.

6. **Alertas no painel:** evento `DeadAirDetected` deve disparar alerta visual destacado no player (banner vermelho piscante) e som de alerta audível no monitor do operador — independentemente de o failover ter entrado em ação.

**Componentes afetados:** Playout Engine (silence detector com ação automática, fill playlist, configuração de emergência), Player UI (configuração de fill playlist e emergency playlist, banner de alerta de dead air), Library Service (sem mudança necessária).

---

##### 7. Voice tracking (gravação de offs)

**O que é:**
Voice tracking é a capacidade de o locutor gravar antecipadamente suas falas (chamadas de **offs** ou **voice tracks**) encaixadas entre as músicas da grade, de modo que a programação soe ao vivo mesmo sendo 100% automatizada. O locutor abre o software, vê a sequência de músicas programadas, grava um off entre a faixa A e a faixa B ouvindo o fim de A e o início de B em tempo real, e sai. O engine depois toca tudo na sequência correta, misturando a voz gravada com a música.

**O mecanismo básico:**

```
Grade programada:
  [Faixa A — 3:45]  →  [Off do locutor]  →  [Faixa B — 4:12]

Durante a gravação do off:
  Locutor ouve: ... fim de Faixa A (últimos 10s) ...
                [GRAVA] "Você ouviu Artista X, agora é hora de..."
                ... início de Faixa B (primeiros 10s) ...

Durante a transmissão:
  Engine toca Faixa A → fade → off gravado sobre a música → Faixa B
```

O locutor nunca precisa estar presente no momento da transmissão. Uma emissora com voice tracking soa como se tivesse locutor ao vivo 24 horas, com custo operacional de algumas horas de gravação por semana.

**Por que é importante:**

- **Emissoras sem operador 24h:** a maioria das rádios brasileiras de pequeno e médio porte não tem locutor na madrugada e nos fins de semana. Sem voice tracking, essas horas soam robotizadas (só música, sem personalidade). Com voice tracking, a emissora mantém identidade e voz humana em qualquer horário.
- **Eficiência operacional:** um locutor grava os offs de 4 horas de programação em 30–40 minutos. Sem voice tracking, seria necessário estar no estúdio as 4 horas inteiras.
- **Gravação remota:** locutores podem gravar de casa, de outro cidade, ou enquanto viajam — o arquivo de voz é enviado ao servidor e encaixado automaticamente na grade.
- **Consistência:** offs gravados podem ser revisados e regravados antes de ir ao ar, ao contrário do ao vivo onde erros são permanentes.
- **Personalização em escala:** uma rede de rádios com 10 afiliadas pode ter um único locutor gravando offs personalizados para cada praça, com referências locais, sem se deslocar.

**Como os concorrentes implementam:**

- **RCS Zetta:** voice tracking integrado com visualização de waveform. O locutor vê a grade, clica no espaço entre duas faixas, ouve o fim da anterior e o início da próxima em crossfade, e grava. O off é salvo automaticamente na posição correta da grade. Suporte a gravação remota via Zetta2GO (navegador). Permite adicionar beds musicais (música de fundo) durante a gravação com fade automático ao parar.
- **mAirList:** módulo de voice tracking com interface separada. Grava o off em WAV, aplica normalização automática, e insere na posição da grade. Suporte a gravação remota via módulo de rede.
- **RadioBOSS:** voice tracking via "Voice Track Recorder". Interface simples: ouve o fim da faixa anterior, grava, ouve o início da próxima. Salva MP3 ou WAV. Sem suporte nativo a gravação remota.
- **RadioDJ:** plugin de voice tracking da comunidade. Funcional mas sem interface polida. Sem gravação remota.
- **RadioPro:** voice tracking integrado. Gravação local com visualização da grade. Sem detalhes sobre gravação remota.
- **PlayIt Live:** sem voice tracking nativo. É uma das principais razões pelas quais PlayIt Live não é usado em emissoras profissionais 24h.

**Fluxo detalhado de uma sessão de voice tracking:**

```
1. Locutor abre o painel de Voice Tracking
2. Visualiza a grade do dia (sequência de faixas programadas)
3. Clica no espaço entre Faixa A e Faixa B
4. Pressiona "Gravar"
5. Ouve automaticamente: últimos 10s de Faixa A (com música)
6. Microfone abre: locutor fala o off
7. Ouve automaticamente: primeiros 10s de Faixa B (com música)
8. Pressiona "Parar"
9. Ouve playback do off completo (A → voz → B) para aprovação
10. Aprova ou regrava
11. Off salvo e encaixado na grade automaticamente
```

**O que implementar no RadioFlow:**

Esta é a feature de maior complexidade de UX do roadmap. Requer coordenação entre três componentes.

1. **Modelo de dados no Library Service:** nova tabela `voice_tracks` com campos `id`, `grid_slot_id` (posição na grade), `path` (arquivo gravado), `duration_ms`, `recorded_at`, `recorded_by`. Associação com a grade de agendamento existente.

2. **API de gravação no Playout Engine:** o engine já tem um player de preview com entrada de microfone não implementada. O que falta:
   - `POST /v1/voice-track/start` — inicia sessão de gravação: toca os últimos `N` segundos da faixa anterior no monitor do locutor, abre o input de microfone, começa a gravar para arquivo temporário.
   - `POST /v1/voice-track/stop` — para a gravação, toca os primeiros `N` segundos da próxima faixa, retorna o arquivo gravado.
   - `POST /v1/voice-track/preview` — toca o off completo com fade das faixas adjacentes para aprovação.
   - `POST /v1/voice-track/save` — move o arquivo temporário para o catálogo e associa à posição da grade.

3. **Captura de microfone:** o Playout Engine precisará de acesso a um dispositivo de entrada de áudio (microfone). Via FFmpeg com `-f avfoundation` (macOS), `-f alsa` (Linux) ou `-f dshow` (Windows) como fonte de gravação.

4. **UI no player — painel de Voice Tracking:**
   - Visualização da grade do dia em ordem cronológica.
   - Cada "slot" entre faixas exibe o off gravado (se existir) ou um botão "Gravar off".
   - Interface de gravação: waveform do off em tempo real, botões Gravar / Parar / Ouvir / Regravar / Salvar.
   - Seleção de dispositivo de microfone.
   - Indicador de nível de entrada (VU meter do microfone).

5. **Reprodução pelo engine:** ao chegar no ponto de um voice track na grade, o engine toca o off sobre o fade de saída da faixa anterior (mix automático), e inicia a próxima faixa no ponto configurado (intro do off termina, faixa B começa).

6. **Gravação remota (fase futura):** como o RadioFlow já tem REST API e WebSocket, a gravação remota pode ser feita via interface web sem instalar o app Electron — o locutor acessa via navegador, grava usando a Web Audio API (`getUserMedia`), e o arquivo é enviado via `POST /v1/voice-track/upload`.

**Dependências:** esta feature depende dos **marcadores de intro/outro** (item 5 da alta prioridade) para funcionar com precisão — sem eles, o ponto de crossfade com o off é estimado por tempo fixo, o que pode soar artificial.

**Componentes afetados:** Playout Engine (captura de microfone, API de voice tracking, reprodução integrada à grade), Library Service (tabela `voice_tracks`, associação com grade), Player UI (painel de voice tracking, waveform de gravação, VU meter de entrada).

---

##### 12. Ducking automático ao acionar botão

**O que é:**
Redução automática do volume da faixa principal no momento em que um botão da botoneira é acionado, seguida de restauração gradual ao volume original quando o áudio do botão termina. O nome vem do inglês *to duck* — a música "se abaixa" para dar espaço ao áudio que entrou.

```
Volume da música principal:

100% ──────────────┐                    ┌──────────────
                   │◄── fade down ──►   │◄── fade up ──►
 30% ──────────────┘____________________┘
                   ↑                    ↑
             botão acionado        áudio do botão termina
            (~300ms de fade)          (~800ms de fade)
```

Sem ducking, a música e o áudio do botão tocam no mesmo volume e brigam entre si — o resultado soa amador e confuso para o ouvinte. Com ducking, a transição é limpa e o áudio do botão (vinheta, spot, efeito) aparece com clareza sem cortar a música abruptamente.

**Variações de comportamento:**

| Modo | Descrição | Quando usar |
|------|-----------|-------------|
| **Duck & hold** | Música baixa enquanto o botão toca, volta ao terminar | Vinhetas, spots, jingles |
| **Duck & cut** | Música para completamente, volta ao terminar | Offs de locutor, notícias |
| **Duck & loop** | Música baixa indefinidamente até o operador restaurar | Entrevistas ao vivo, blocos |
| **No duck** | Música mantém volume, áudio do botão entra sobre ela | Efeitos sonoros, trilhas |

O modo de ducking pode ser configurado por tipo de áudio ou por botão individualmente.

**Parâmetros configuráveis:**

- **Nível de duck:** quanto o volume cai (ex: de 100% para 30%)
- **Fade down:** velocidade com que o volume cai ao acionar o botão (ex: 300ms)
- **Fade up:** velocidade com que o volume volta ao terminar (ex: 800ms — mais lento que a descida, para soar natural)
- **Delay de duck:** quantos ms esperar antes de começar o fade down (permite que o botão já esteja audível antes de a música ceder espaço)

**Como os concorrentes implementam:**

- **RCS Zetta:** ducking configurável por tipo de cart (hotkey). Parâmetros de fade down, fade up e nível de duck definidos globalmente ou por botão. Integração com mesas de áudio físicas — o ducking pode ser controlado por GPI em vez de software.
- **mAirList:** ducking nativo com fade configurável. O cartwall tem opção de duck automático por faixa, e o locutor pode ativar/desativar o ducking em tempo real via tecla de atalho.
- **RadioBOSS:** "Voice-Over" mode — ao acionar um cart, a música baixa para o nível configurado. Fade in/out configurável em milissegundos. Opção de ducking apenas para certos players auxiliares.
- **RadioPro:** "controle fino de mixagem por tipo de mídia" inclui ducking ao acionar botões de vinhetas e spots.
- **EBRcart2:** suporte a fade in/out por canal, mas sem ducking automático integrado — o operador controla os faders manualmente.
- **PlayIt Live / RadioDJ:** sem ducking automático nativo.

**Por que é importante no contexto brasileiro:**

Em emissoras brasileiras, a botoneira é usada intensivamente durante programas ao vivo: o apresentador aciona vinhetas de passagem, spots curtos, efeitos sonoros e trilhas de fundo várias vezes por hora. Sem ducking, cada acionamento exige que o operador de áudio abaixe e suba o fader da música manualmente — operação que distrai, pode ser atrasada e frequentemente resulta em momentos onde os dois áudios competem no ar.

Com ducking automático, o operador aciona o botão e o sistema cuida da mixagem. O apresentador ganha autonomia para operar a botoneira sem um operador de áudio dedicado.

**O que implementar no RadioFlow:**

O Playout Engine já tem um mixer interno que combina o player principal com o player de preview (CUE). O ducking requer expandir esse mixer para detectar atividade na botoneira e aplicar ganho dinâmico no canal principal.

1. **Ganho dinâmico no mixer:** o mixer atual aplica ganho fixo por canal. Para ducking, o ganho do canal principal precisa ser modulável em tempo real com envelope de fade (interpolação linear ou exponencial entre o volume atual e o volume alvo, ao longo do tempo de fade configurado).

2. **Integração com a botoneira:** ao receber `CmdTriggerHotButton`, o engine verifica o modo de ducking configurado para aquele botão e, se ativo, inicia o fade down imediatamente. Ao receber `CartStopped` ou `CartFinished`, inicia o fade up.

3. **Configuração por botão:** adicionar campo `duck_mode` (`none`, `duck`, `cut`, `loop`) e `duck_level` (0.0–1.0) no modelo de botão da botoneira. API: `PUT /v1/hotkeys/profiles/:id/buttons/:btnId` já existente — adicionar os novos campos.

4. **Configuração global:** `GET/PUT /v1/config/ducking` com defaults globais (`duck_level: 0.3`, `fade_down_ms: 300`, `fade_up_ms: 800`, `delay_ms: 0`) que servem como fallback quando o botão não tem configuração própria.

5. **UI no player:** toggle de ducking por botão no editor de perfis da botoneira. Slider de nível de duck. Preview do comportamento antes de salvar.

6. **Estado no WebSocket:** publicar evento `DuckingActive` (com nível atual) e `DuckingRestored` para que o painel do operador mostre visualmente quando o ducking está ativo — útil para depuração e para o operador saber o estado do mixer em tempo real.

**Componentes afetados:** Playout Engine (mixer com ganho dinâmico e envelope de fade, integração com eventos da botoneira), Library Service (campos `duck_mode` e `duck_level` no modelo de botão), Player UI (editor de configuração de ducking por botão, indicador visual de ducking ativo).

### Baixa prioridade (diferenciais futuros)

| # | Lacuna | Justificativa |
|---|--------|---------------|
| 13 | **Controle via hardware GPI** | Integração com mesas de corte físicas, botões e consoles de broadcast. |
| 14 | **Suporte a RDS** | Envia nome da música para o painel do carro / receptor FM. |
| 15 | **Relatório ECAD** | Específico Brasil — declaração mensal de músicas executadas para pagamento de direitos. |
| 16 | **Controle remoto via web** | Permite operação remota do estúdio. RCS Zetta e mAirList têm (Zetta2GO). |
| 17 | **Pedidos musicais / promoções** | RadioPro tem; útil para rádios comunitárias e programas interativos. |
| 18 | **Multi-estúdio** | Operação de múltiplos estúdios a partir de uma única instância do servidor. |
| 19 | **Análise de loudness na importação** | Escanear LUFS automaticamente ao importar novos áudios para o catálogo. |
| 20 | **Suporte a touch screen** | Relevante para tablets de cabine. RadioPro e RadioBOSS suportam. |

---

#### Detalhamento — Baixa prioridade (continuação)

---

##### 19. Análise de loudness na importação

**O que é:**
Processo de medir o loudness (volume percebido) de cada arquivo de áudio **no momento da importação para o catálogo**, gravando o resultado no banco de dados para uso posterior. O valor calculado é o **LUFS integrado** (Loudness Units Full Scale, padrão EBU R128) — o nível médio de volume percebido ao longo de toda a duração da faixa.

O cálculo é feito offline, uma única vez por arquivo, antes do primeiro uso. A partir daí, o valor fica disponível instantaneamente no banco sempre que o engine precisar.

**Como funciona tecnicamente:**

Durante o pipeline de importação, após extrair metadados e calcular duração, o Library Service executa o analisador de loudness sobre o arquivo completo:

```bash
ffmpeg -i musica.mp3 -filter:a ebur128=peak=true -f null -
```

O FFmpeg processa o arquivo inteiro e retorna:

```
Integrated loudness:  -14.2 LUFS   ← valor principal
True peak:             -1.1 dBTP   ← pico verdadeiro
LRA (loudness range):   6.3 LU     ← variação dinâmica
```

O valor de LUFS integrado é gravado na tabela `tracks` no campo `loudness_lufs`. O true peak e o LRA podem ser armazenados em campos adicionais para uso futuro.

**Custo de processamento:**

Calcular LUFS integrado exige processar o arquivo **de ponta a ponta** — o algoritmo não pode fazer amostras parciais e dar um resultado confiável. O tempo de processamento é proporcional à duração do áudio:

| Duração da faixa | Tempo de análise (CPU moderna) |
|-----------------|-------------------------------|
| Spot de 30s | ~0.5–1s |
| Jingle de 1min | ~1–2s |
| Música de 4min | ~4–8s |
| Programa de 1h | ~1–2min |

Por isso a análise é sempre feita **offline na importação**, nunca em tempo real durante a reprodução. Quando a faixa vai ao ar, o valor já está gravado e o engine aplica o ganho em microssegundos.

**Para que serve o valor gravado — dependências:**

O LUFS gravado por si só não faz nada visível ao operador. Ele é a **matéria-prima** consumida por outras features:

| Feature dependente | Como consome `loudness_lufs` |
|-------------------|------------------------------|
| **Normalização automática de volume** (item 4 — alta prioridade) | `gain_db = target_lufs − track.loudness_lufs`. Esse ganho é aplicado no mixer do engine ao reproduzir a faixa, fazendo todas as músicas soarem no mesmo nível. Sem o LUFS gravado, a normalização é impossível. |
| **Rotação musical por formato** | O gerador de playlist pode usar o LUFS como critério de ordenação — evitar colocar uma faixa de −6 LUFS (masterização pesada) logo após uma de −20 LUFS (gravação antiga), o que causaria salto de volume perceptível mesmo com normalização ativa. |
| **Triagem e controle de qualidade do catálogo** | O operador pode filtrar faixas por faixa de loudness na biblioteca: identificar material fora do padrão da emissora (ex: acima de −9 LUFS = muito alto, abaixo de −24 LUFS = muito baixo), candidatos a remasterização antes de ir ao ar. |
| **Relatório de qualidade técnica** | Lista exportável de faixas com LUFS fora da faixa aceitável — útil para o setor técnico da emissora auditar o catálogo. |

**Como os concorrentes implementam:**

- **RCS Zetta:** análise de loudness integrada ao processo de importação. Calcula LUFS integrado, true peak e LRA por arquivo. Usa o resultado diretamente no módulo de normalização durante a reprodução. Target configurável por tipo de áudio (música vs. spot vs. jingle). Exibe o LUFS de cada faixa no editor de propriedades do áudio.
- **RadioBOSS:** análise via ReplayGain ou LUFS por faixa, executada em background após a importação. Fila de análise assíncrona: arquivos importados ficam na fila e são analisados em segundo plano sem bloquear o sistema. Exibe o valor calculado na ficha de cada faixa.
- **mAirList:** análise EBU R128 integrada ao importador. Suporte a análise em lote (batch) para re-analisar o catálogo inteiro quando o target muda. Exibe LUFS, LRA e true peak na janela de propriedades da faixa.
- **RadioPro:** sem análise de loudness por faixa declarada. Usa ajuste de ganho por tipo de mídia (forma simplificada sem medição individual).
- **PlayIt Live / RadioDJ:** sem análise de loudness nativa. Dependem de ferramentas externas (MP3Gain, fre:ac) para pré-processar o catálogo antes de importar.

**O que implementar no RadioFlow:**

O Library Service já tem o pipeline de importação funcionando (watch folder + ID3 + FFprobe). A análise de loudness é uma etapa adicional nesse mesmo pipeline.

1. **Etapa de análise no pipeline de importação:** após calcular a duração via FFprobe, adicionar chamada ao FFmpeg com filtro `ebur128`. Parsear a saída para extrair os valores de LUFS integrado, true peak e LRA. Campos na tabela `tracks`: `loudness_lufs REAL`, `true_peak_dbtp REAL`, `lra_lu REAL`, `loudness_analyzed_at DATETIME`.

2. **Fila de análise assíncrona:** a análise não deve bloquear o pipeline de importação nem a resposta da API. Implementar uma goroutine worker (ou pool de workers) que consome uma fila de arquivos pendentes de análise. Arquivos recém-importados entram na fila com status `pending`; ao terminar a análise, o status muda para `analyzed`.

3. **Análise de arquivos existentes (batch):** endpoint `POST /v1/library/analyze-loudness` que enfileira todos os tracks com `loudness_lufs IS NULL` para análise em background. Permite analisar catálogos já existentes sem reimportar os arquivos.

4. **API de status da fila:** `GET /v1/library/analyze-loudness/status` retorna quantos arquivos estão pendentes, em análise e concluídos — para o operador acompanhar o progresso ao analisar catálogos grandes.

5. **Controle de concorrência:** limitar o número de análises simultâneas (ex: máximo 2 workers) para não saturar CPU durante a operação da emissora. Configurável via `PUT /v1/config/loudness-analyzer`.

6. **UI no player:** exibir o valor de LUFS na ficha de cada faixa na biblioteca. Badge visual de status (`pending` / `analyzed` / `out-of-range`). Botão "Analisar catálogo" com barra de progresso. Filtro na busca avançada por faixa de LUFS (ex: "mostrar faixas com LUFS > −10").

**Dependência crítica:** este item é pré-requisito direto da **normalização automática de volume** (item 4 da alta prioridade). A normalização só funciona se o LUFS de cada faixa estiver gravado no banco. Recomenda-se implementar este item imediatamente antes ou em conjunto com a normalização.

**Componentes afetados:** Library Service (etapa de análise no pipeline de importação, fila de workers, campos na tabela `tracks`, endpoints de análise batch e status), Player UI (exibição de LUFS na biblioteca, filtro por loudness, painel de progresso de análise).

---

#### Detalhamento — Baixa prioridade

---

##### 13. Controle via hardware GPI

**O que é:**
GPI significa **General Purpose Interface** — entradas e saídas de contato elétrico (sinais digitais simples: fechado/aberto) presentes em mesas de áudio, consoles e painéis de controle profissionais de broadcast. Permitem que o hardware físico do estúdio comande o software de automação, e vice-versa, sem passar pelo mouse ou teclado.

Existem dois sentidos de comunicação:

- **GPI IN (entrada):** sinal vindo do hardware para o software. Um fader aberto na mesa, um botão pressionado no console ou um sensor externo fecha um circuito elétrico que o software interpreta como um comando.
- **GPO OUT (saída):** sinal saindo do software para o hardware. O software acende uma luz "NO AR", liga um relé de transmissão ou dispara um alarme externo.

```
Mesa de áudio / console físico
        │
        │  GPI IN  →  software recebe e executa comando
        ▼
┌──────────────────────────────┐
│  Interface GPI               │
│  (USB, serial RS-232/RS-485, │
│   PCI, Ethernet)             │
│  ex: Broadcast Tools,        │
│      Axia xNode, Sonifex,    │
│      GPIO Solutions          │
└──────────────────────────────┘
        ▲
        │  GPO OUT  ←  software envia sinal para hardware
        │
Transmissor / luz NO AR / relé / alarme externo
```

**Exemplos reais de uso em estúdio:**

| Ação do hardware | Comando no software |
|-----------------|---------------------|
| Abrir fader do microfone na mesa | Pausar música / entrar em modo ao vivo |
| Fechar fader do microfone | Retomar música automaticamente |
| Pressionar botão físico no console | Acionar slot da botoneira |
| Sensor de silêncio no transmissor | Acionar modo PANIC / playlist de emergência |
| Botão de SKIP no painel de cabine | Comando Skip no engine |
| Software inicia reprodução | Acender luz vermelha "NO AR" no estúdio |
| Software detecta dead air | Ligar relé que troca para transmissor backup |
| Faixa chega ao ponto de intro | Piscar luz para locutor saber que pode falar |

**Por que é relevante:**

- **Operação sem mouse:** em ambiente de transmissão ao vivo, o operador precisa das mãos livres. Consoles físicos com botões e faders permitem operação muito mais rápida e segura do que clicar em tela.
- **Integração com o estúdio físico:** emissoras profissionais já têm mesas de áudio (Wheatstone, Axia, RCF, Riedel, SSL) com GPI embutido. O software de automação precisa falar com esse hardware para ser adotado nesses ambientes.
- **Automação de sinal:** GPO permite que o software comande equipamentos externos — transmissores, processadores de áudio, sistemas de monitoramento — sem intervenção humana.
- **Conformidade com fluxos de trabalho existentes:** emissoras migram de softwares legados e não querem mudar a operação da mesa física. O novo software precisa aceitar os mesmos sinais GPI que o anterior.

**Como os concorrentes implementam:**

- **RCS Zetta:** suporte nativo a GPI via interfaces Axia (protocolo Livewire), Wheatstone e hardware genérico via GPIO Solutions. Mapeamento de qualquer GPI IN para qualquer comando do Zetta (play, stop, skip, cart, etc.). GPO OUT configurável para qualquer evento do engine. Integração com consoles de áudio por IP (AoIP).
- **mAirList:** suporte a GPI via portas seriais (RS-232), interfaces USB-GPIO e controladores MIDI/HID. Editor visual de mapeamento GPI→comando e evento→GPO. Suporte a GPI em rede via protocolo mAirList Remote.
- **RadioBOSS:** suporte a GPI via porta serial COM e interfaces USB. Mapeamento básico de GPI IN para comandos de playback e botoneira.
- **RadioPro:** GPI via hardware serial e USB, integrado ao fluxo de operação do estúdio.
- **EBRcart2:** suporte a GPI configurável por canal, usado para acionamento de carts por botões físicos externos.
- **PlayIt Live / RadioDJ:** sem suporte a GPI nativo.

**Protocolos e interfaces mais comuns:**

| Interface | Protocolo | Uso típico |
|-----------|-----------|------------|
| Porta serial RS-232 | Sinais TTL de contato seco | Hardware legado, transmissores |
| USB-GPIO | HID ou serial virtual | Painéis de botões físicos, interfaces econômicas |
| MIDI | Nota on/off, CC | Controladores de DJ, consoles modernos |
| Axia Livewire | TCP/IP (protocolo proprietário) | Consoles Axia / Wheatstone em rede |
| Ethernet GPIO | TCP/IP genérico | Interfaces profissionais de rack |

**O que implementar no RadioFlow:**

Esta feature é de baixa prioridade porque requer hardware físico específico para testar e o mercado-alvo inicial (pequenas e médias emissoras) frequentemente opera sem console profissional. Quando implementada:

1. **Camada de abstração GPI:** módulo `internal/gpi` no Playout Engine com interface `GPIAdapter` — permite adicionar novos tipos de hardware sem alterar o core. Implementações iniciais: serial (RS-232 via `/dev/ttyUSB0` ou `COM1`) e USB-HID genérico.

2. **Mapeamento GPI IN → Command Bus:** arquivo de configuração (JSON/TOML) que define qual sinal GPI dispara qual comando: `{ "gpi_in": 1, "command": "CmdPlay" }`, `{ "gpi_in": 2, "command": "CmdTriggerHotButton", "payload": { "button_id": "btn_01" } }`. O módulo GPI lê o sinal e publica o comando no Command Bus existente — sem acoplamento com a lógica de playback.

3. **Mapeamento Event Bus → GPO OUT:** configuração análoga no sentido inverso: qualquer evento do Event Bus pode acionar uma saída GPO. `{ "event": "ItemStarted", "gpo_out": 1 }` → acende luz NO AR. `{ "event": "DeadAirDetected", "gpo_out": 2 }` → liga relé de emergência.

4. **API de configuração:** `GET/PUT /v1/config/gpi` para configurar os mapeamentos sem reiniciar o engine. `GET /v1/gpi/status` para monitorar o estado atual de cada linha GPI (útil para diagnóstico).

5. **UI no player:** painel de configuração GPI com visualização do estado de cada linha em tempo real — verde (fechado) / cinza (aberto). Editor de mapeamentos GPI IN e GPO OUT com seletor de comando/evento.

6. **Suporte a MIDI:** MIDI é o protocolo mais acessível para pequenas emissoras (controladores baratos, sem necessidade de interface profissional). Implementar `MIDIAdapter` que mapeia Note On/Off e Control Change para o mesmo Command Bus — aumenta significativamente o alcance da feature sem hardware caro.

**Componentes afetados:** Playout Engine (novo módulo `internal/gpi`, adaptadores por protocolo, integração com Command Bus e Event Bus), Player UI (painel de configuração e monitoramento de GPI).

---

##### ~~Rotação musical por formato (clock)~~ ✅ Já implementado

**O que é:**
Sistema que define quais tipos e categorias de música tocam em qual ordem e proporção ao longo do dia, garantindo identidade musical consistente independentemente de quem está operando. O elemento central é o **clock** (relógio de programação): um template de 60 minutos que especifica exatamente o que deve tocar em cada janela de tempo dentro de uma hora — categorias musicais, tipos de áudio, jingles, spots e vinhetas, na ordem e proporção definidas pela direção artística da emissora.

**Estrutura de um clock:**

```
CLOCK "Manhã Adulto" — template de 60 minutos

 Slot  Duração  Categoria / Tipo
 ────  ───────  ────────────────────────────────
  1     ~4min   MPB Clássica (1970–1990)
  2     ~0:30   Vinheta de passagem
  3     ~4min   Pop Nacional Atual
  4     ~3:30   Sertanejo Universitário
  5     ~1min   Spot comercial bloco A
  6     ~1min   Spot comercial bloco A
  7     ~4min   Rock Nacional
  8     ~0:20   Jingle da emissora
  9     ~4min   MPB Atual
 10     ~3:30   Samba / Pagode
 11     ~1min   Spot comercial bloco B
 12     ~4min   Pop Internacional
 13     ~0:30   Hora Certa (se :00)
 ...    ...     ...
```

O gerador de playlist preenche cada slot escolhendo automaticamente uma faixa da categoria correta, respeitando regras de separação como: não repetir o mesmo artista nas últimas 2 horas, não repetir a mesma música em menos de 72 horas, não colocar dois artistas do mesmo gênero consecutivos.

**Componentes do sistema:**

| Componente | Função |
|-----------|--------|
| **Categorias musicais** | Grupos de faixas com características comuns (gênero, época, energia). Cada faixa pertence a uma ou mais categorias. |
| **Clock** | Template de 60 min com slots ordenados, cada slot apontando para uma categoria ou tipo fixo. |
| **Grade de clocks** | Matriz hora × dia-da-semana: qual clock usar às 6h de segunda, às 22h de sábado, etc. Manhã, tarde, noite e madrugada podem ter clocks completamente diferentes. |
| **Regras de separação** | Restrições que o gerador respeita: separação mínima por artista, por título, por categoria, por BPM, por energia. |
| **Gerador de playlist** | Motor que executa o clock para as próximas N horas, escolhendo faixas do catálogo que satisfaçam todas as restrições. |
| **Log de rotação** | Histórico de quais faixas foram usadas em cada slot, base para as regras de separação. |

**Por que é importante:**

- **Identidade musical:** sem rotação, a mesma música pode tocar duas vezes em uma hora, o catálogo de 5.000 faixas se concentra nas 200 mais acessadas, músicas pesadas e lentas se alternam sem critério. Com rotação, a emissora soa consistente em qualquer horário e com qualquer operador.
- **Obrigação regulatória (Brasil):** emissoras FM são obrigadas pela ANATEL e pelo Regulamento dos Serviços de Radiodifusão a cumprir cotas de música brasileira (mínimo de 70% de música nacional em determinados horários). A rotação por formato é o mecanismo que garante essa conformidade de forma automática e auditável.
- **Gerenciamento comercial:** spots e blocos comerciais precisam ser distribuídos nos momentos certos do clock — não podem aparecer dois blocos seguidos nem faltar nos horários vendidos aos anunciantes.
- **Programação desassistida:** emissoras que funcionam 24h sem operador noturno dependem inteiramente do gerador de playlist para manter a grade coerente enquanto ninguém está presente.

**Como os concorrentes implementam:**

- **RCS Zetta:** sistema de rotação completo integrado ao GSelector (produto separado da RCS, especificamente para music scheduling). Clocks visuais em formato de "pizza" editáveis por drag-and-drop. Regras de separação por artista, título, álbum, humor, BPM, energia, gênero e qualquer campo customizado. Integração nativa entre GSelector e Zetta: o GSelector gera a playlist que o Zetta executa.
- **mAirList:** scheduler de rotação integrado. Clocks configuráveis por hora e dia da semana. Regras de separação por artista e título. Gerador que preenche automaticamente a grade para as próximas 24h. Menos sofisticado que GSelector mas suficiente para emissoras de médio porte.
- **RadioBOSS:** gerador de playlist com categorias e regras de separação. Clock chamado de "Format Clock" com slots configuráveis. Regras por artista (mínimo de separação em horas) e por título (mínimo em horas). Sem controle de BPM ou energia.
- **RadioDJ:** sistema de categorias e rotação integrado. Editor de "Rotações" com definição de proporção por categoria. Regras de separação por artista e título. Gerador automático de lista. Uma das features mais completas entre os gratuitos.
- **RadioPro:** "geração automática de programação musical e comercial" com categorias e clocks. Específico para o mercado brasileiro com suporte à cota de música nacional.
- **PlayIt Live:** sem rotação por formato nativa. Dependência de playlist manual ou plugins externos.

**O que implementar no RadioFlow:**

Esta é a feature de maior impacto para emissoras AM/FM e a mais complexa do roadmap após o multi-deck. Envolve um novo subsistema completo no Library Service.

1. **Modelo de dados — Categorias:**
   Nova tabela `categories` no Library Service: `id`, `name`, `description`, `color`. Tabela de associação `track_categories` (`track_id`, `category_id`). Uma faixa pode pertencer a múltiplas categorias (ex: "MPB Clássica" e "Lenta").

2. **Modelo de dados — Clock:**
   Tabela `clocks`: `id`, `name`. Tabela `clock_slots`: `id`, `clock_id`, `position`, `type` (`category` | `jingle` | `spot` | `vinheta` | `hora_certa`), `category_id` (nullable), `duration_hint_ms`. Um clock é uma lista ordenada de slots.

3. **Modelo de dados — Grade de clocks:**
   Tabela `clock_schedule`: `hour` (0–23), `weekday` (0–6), `clock_id`. Matriz 24×7 definindo qual clock usar em cada hora de cada dia da semana.

4. **Modelo de dados — Regras de separação:**
   Tabela `separation_rules`: `field` (`artist` | `title` | `category`), `min_separation_min` (separação mínima em minutos). O gerador consulta o log de rotação para verificar se a faixa candidata viola alguma regra.

5. **Gerador de playlist:**
   Novo serviço `internal/scheduler/generator` no Library Service. Para cada slot do clock da hora seguinte, escolhe uma faixa da categoria correta que: (a) satisfaz as regras de separação consultando o log de rotação, (b) não foi usada recentemente, (c) tem duração compatível com o slot. Algoritmo: tenta até N candidatos aleatórios da categoria; se nenhum passar, relaxa a regra menos crítica e tenta novamente; se ainda falhar, usa a faixa menos recente da categoria independente de separação.

6. **API de geração:**
   `POST /v1/schedule/generate?hours=4` — gera e retorna a playlist das próximas N horas com base nos clocks configurados. O player pode consumir essa playlist, exibi-la ao operador para aprovação e enfileirá-la no Playout Engine via `POST /v1/queue/enqueue`.

7. **UI no player:**
   - Editor de categorias: criar, renomear, associar faixas.
   - Editor de clocks: interface visual com lista de slots ordenados, tipo e categoria de cada slot.
   - Grade de clocks: matriz 24×7 com seletor de clock por célula.
   - Configuração de regras de separação.
   - Painel de geração: botão "Gerar próximas X horas", visualização da playlist gerada, botão "Enfileirar".

8. **Integração com a cota de música brasileira:**
   Campo `is_brazilian` na tabela `tracks`. O gerador pode ser configurado para respeitar proporção mínima de faixas brasileiras por clock, gerando relatório de conformidade.

**Dependências:** esta feature depende do **log de transmissão** (item 1 da alta prioridade) para as regras de separação — o gerador precisa saber o que tocou nas últimas N horas para evitar repetições. Sem o log, as regras de separação só funcionam dentro da sessão atual.

**Componentes afetados:** Library Service (tabelas `categories`, `clocks`, `clock_slots`, `clock_schedule`, `separation_rules`; módulo `internal/scheduler/generator`; API de geração), Player UI (editores de categoria, clock e grade; painel de geração de playlist).

---

## 6. Posicionamento sugerido

Com base no benchmark, o RadioFlow tem potencial para se posicionar como:

> **Plataforma de automação de rádio open-source, cross-platform e API-first, voltada para emissoras brasileiras de pequeno e médio porte que precisam de controle técnico avançado sem depender de software proprietário legado.**

Esse nicho não é ocupado por nenhum concorrente analisado:
- RadioPro e EBRcart são proprietários, Windows-only e sem API.
- RadioDJ é open-source mas Windows-only, sem loudness e sem API.
- RCS Zetta é o padrão técnico, mas tem custo proibitivo para pequenas emissoras.
- mAirList é forte tecnicamente mas não tem adaptação ao mercado brasileiro (ECAD, Voz do Brasil, português).

---

##### Equalização / processamento de áudio

**O que é:**
Conjunto de tratamentos aplicados ao **sinal de áudio já mixado**, imediatamente antes de ser enviado ao output (transmissor FM/AM, encoder de streaming ou saída de linha). Diferente da normalização — que ajusta o ganho de cada faixa individualmente antes de tocar — o processamento de áudio age sobre o **sinal total combinado** de tudo que está saindo da emissora naquele instante.

O objetivo é garantir que a emissora soe profissional, consistente e otimizada para o meio de transmissão, independentemente do conteúdo que estiver tocando.

**Posição no pipeline de áudio:**

```
Fila / Botoneira / Voz do locutor
            ↓
    Mixer (normalização, crossfade, ducking)
            ↓
  ┌─────────────────────────────────────┐
  │   PROCESSAMENTO DE ÁUDIO           │
  │                                     │
  │   EQ → Compressor → AGC → Limiter  │
  └─────────────────────────────────────┘
            ↓
    Output (alto-falante / encoder Icecast / transmissor)
```

**Módulos que compõem o processamento:**

**1. Equalizador paramétrico (EQ)**
Ajusta o balanço de frequências do sinal. Cada banda de frequência pode ser amplificada ou atenuada de forma independente. Casos de uso em rádio:
- Realçar graves (80–120 Hz) e presença (3–5 kHz) para compensar a resposta do receptor de carro
- Cortar frequências de 200–400 Hz ("lama") que acumulam ao somar vários canais
- Reduzir agudos acima de 15 kHz antes da compressão MP3 (evita artefatos de encoding)

```
dB
+4  |    ╭──╮                    ╭──
 0  |───╯    ╰────────────╮─────╯
-2  |                      ╰─╮
-6  |                         ╰────
    20  100  500  1k   3k  8k  16k  Hz
         ↑                ↑
      graves            presença
```

**2. Compressor de dinâmica**
Reduz a diferença entre os momentos mais altos e mais baixos do sinal (range dinâmico). Com compressão, uma voz baixa e uma música alta ficam em nível similar — o ouvinte no carro em ambiente ruidoso consegue ouvir tudo com clareza sem precisar mexer no volume.

Parâmetros principais:
- **Threshold:** nível acima do qual a compressão começa (ex: −18 dBFS)
- **Ratio:** quanto comprimir acima do threshold (ex: 4:1 = a cada 4 dB acima do threshold, apenas 1 dB passa)
- **Attack:** velocidade com que a compressão entra (ms) — ataque rápido corta transientes, lento preserva
- **Release:** velocidade com que a compressão sai (ms)

**3. Limitador (Hard Limiter)**
Garante que o sinal **nunca ultrapasse 0 dBFS**, evitando distorção digital (clipping). É a última linha de defesa antes do encoder ou transmissor. Obrigatório em qualquer emissora profissional — uma única amostra clipada causa distorção audível e degradação do codec de streaming.

Na prática funciona como um compressor com ratio infinito acima do threshold (ex: −0.3 dBFS): qualquer coisa acima desse nível é cortada imediatamente.

**4. AGC — Automatic Gain Control**
Ajuste automático de ganho de longo prazo (segundos a minutos) que mantém o nível médio do sinal dentro de uma faixa estável ao longo do tempo. Diferente do compressor (que age em milissegundos), o AGC corrige variações lentas — como a diferença de nível entre um programa de música e um programa de entrevistas. O ouvinte não percebe a variação porque ela é gradual.

**5. Processamento multiband**
Divide o sinal em 3–5 bandas de frequência (graves, médio-graves, médios, médio-agudos, agudos), aplica compressão e ganho independente em cada banda, e recombina. É a técnica usada pelos processadores de áudio profissionais de broadcast (Orban Optimod, Wheatstone, Omnia) que custam dezenas de milhares de dólares. O resultado é um sinal denso, "cheia" e consistente que caracteriza o som de grandes emissoras FM.

**Por que é baixa prioridade para o RadioFlow:**

- **Emissoras FM/AM com transmissor físico:** já têm o processador de áudio **no rack de hardware**, entre o computador e o transmissor. O software não precisa fazer nada — o sinal sai da placa de áudio do computador e o hardware cuida do resto. Implementar processamento no software seria redundante e poderia conflitar com o hardware.
- **Complexidade de implementação:** processamento de áudio de qualidade broadcast é extremamente complexo. O Orban Optimod (referência da indústria) tem décadas de desenvolvimento e algoritmos proprietários. Uma implementação de software que tente reproduzir isso sem hardware dedicado dificilmente atingirá a mesma qualidade.
- **Web rádios:** o encoder de streaming (MP3/AAC) já aplica compressão espectral própria. Um EQ básico e um limiter simples são suficientes para a maioria dos casos e podem ser feitos com ferramentas externas (VST plugins via pipeline de áudio do SO).
- O RadioFlow já entrega EBU R128 em tempo real, loudness por faixa e normalização automática — que resolvem os problemas mais impactantes de volume sem a complexidade do processamento dinâmico.

O valor real de implementar processamento no RadioFlow é para **web rádios pequenas sem hardware externo** que querem soar mais profissionais sem investir em rack de equipamentos.

**Como os concorrentes implementam:**

- **RCS Zetta:** sem processamento de áudio interno. A arquitetura do Zetta assume que há hardware de processamento externo entre o software e o transmissor. Integra-se com processadores via GPI/GPIO para controle, mas não processa o sinal.
- **RadioBOSS:** EQ paramétrico de 10 bandas e compressor básico integrados. Limiter configurável. Sem processamento multiband. Suficiente para web rádios pequenas.
- **mAirList:** EQ e compressor integrados via cadeia de efeitos DSP. Suporte a plugins VST (Windows), o que permite usar processadores de qualidade profissional de terceiros dentro do mAirList. É o diferencial mais significativo: qualquer plugin VST de broadcast pode ser encadeado no sinal de saída.
- **RadioPro:** sem processamento de áudio interno declarado.
- **RadioDJ:** sem processamento de áudio interno.
- **PlayIt Live:** sem processamento de áudio interno.

**O que implementar no RadioFlow:**

A abordagem mais eficiente é uma cadeia de processamento modular e extensível, similar ao mAirList, em vez de tentar implementar cada algoritmo DSP do zero.

1. **Cadeia de processamento modular:** novo módulo `internal/audio/processing` no Playout Engine. Interface `Processor` com método `Process(buf []float32, sampleRate int) []float32`. A saída do mixer passa por uma cadeia de `[]Processor` antes de chegar ao output — cada processador lê o buffer, aplica seu algoritmo e passa adiante.

2. **Limiter (prioridade máxima dentro do escopo):** implementação própria de hard limiter com lookahead de 5ms. É o módulo mais simples e o mais necessário — garante que nenhum sample ultrapasse o ceiling configurável (ex: −0.1 dBFS) mesmo após ganhos de normalização. Algoritmo: peak detector com janela de lookahead + gain reduction suave para evitar distorção de limitação.

3. **EQ paramétrico básico:** filtros biquad (implementação padrão de áudio digital) para até 8 bandas, cada uma configurável com tipo (peaking, high shelf, low shelf, notch), frequência, gain (dB) e Q. Algoritmo bem documentado e de baixa complexidade — coeficientes calculados offline, aplicação por amostra via Direct Form II.

4. **Compressor de dinâmica simples:** compressor feedforward single-band com parâmetros threshold, ratio, attack, release e knee. Suficiente para controle básico de dinâmica em web rádios.

5. **Configuração via API:** `GET/PUT /v1/config/processing` com a cadeia completa de processadores e seus parâmetros. A cadeia é reconfigurável em tempo real sem interromper a reprodução — nova configuração é aplicada no próximo buffer de processamento.

6. **Suporte a VST (fase futura, Windows/Linux):** via biblioteca `purego` ou CGo com a SDK do VST3, permitir que plugins VST de terceiros sejam inseridos na cadeia de processamento. Abre o RadioFlow para o ecossistema completo de plugins de broadcast profissional sem precisar reimplementar algoritmos complexos.

7. **UI no player:** painel de processamento de áudio com visualizador de curva EQ (canvas), controles de compressor com visualização de gain reduction em tempo real (indicador de GR em dBFS), toggle por módulo e preset de configurações (ex: "FM standard", "Web radio", "Flat").

**Componentes afetados:** Playout Engine (módulo `internal/audio/processing`, cadeia de processadores, integração na saída do mixer), Player UI (painel de EQ e compressor, visualizador de curva, indicador de gain reduction).

---

##### Envio de RDS

**O que é:**
RDS (**Radio Data System**, norma IEC 62106) é um protocolo que permite às emissoras FM transmitir dados digitais junto com o sinal de áudio analógico, sem interferir na qualidade sonora. Usa um subportador de 57 kHz — inaudível — que carrega pacotes de dados em paralelo ao áudio. É o que faz o painel do carro, o receptor de mesa ou o app de rádio exibir o nome da emissora, o título da música e o artista em tempo real.

No contexto do software de automação, o papel do RadioFlow é **enviar os metadados atualizados** para o encoder RDS a cada troca de faixa. O RadioFlow não gera o sinal RDS em si — isso é responsabilidade do encoder de hardware ou software externo.

**Campos RDS relevantes para automação:**

| Campo | Tamanho | Conteúdo | Atualização |
|-------|---------|----------|-------------|
| **PS — Programme Service** | 8 chars | Nome fixo da emissora (ex: `RADIOFLW`) | Estático ou por programa |
| **RT — RadioText** | 64 chars | Texto rolante livre (ex: `Legião Urbana - Faroeste Caboclo`) | A cada troca de faixa |
| **RT+ — RadioText Plus** | estruturado | Artista e título em campos separados, para receptores modernos exibirem formatado | A cada troca de faixa |
| **PTY — Programme Type** | 5 bits | Tipo de programação (Rock, News, Jazz, Sport...) | Por programa / horário |
| **PI — Programme Identifier** | 16 bits | Código único da emissora registrado na ANATEL | Estático |
| **CT — Clock Time** | timestamp | Hora atual — receptores podem sincronizar o relógio pelo sinal FM | A cada minuto |
| **AF — Alternative Frequencies** | lista | Frequências das retransmissoras da mesma rede | Estático |
| **TA/TP — Traffic** | flags | Interrompe outros programas no receptor para anúncio de trânsito | Sob demanda |

**Arquitetura do fluxo:**

```
RadioFlow
    │
    │  A cada ItemStarted:
    │  { PS: "RADIOFLW", RT: "Legião Urbana - Faroeste Caboclo",
    │    RT+: { artist: "Legião Urbana", title: "Faroeste Caboclo" } }
    │
    │  Protocolo: TCP/IP (porta configurável) ou Serial (RS-232)
    ▼
Encoder RDS
(hardware: Pira.net, Quartz, P.H. Engineering, Broadcast Warehouse)
(software: PiRa32, RDS-Sharp)
    │
    │  Subportador de 57 kHz injetado no sinal FM composto
    ▼
Transmissor FM ──► Antena ──► Receptor (carro, mesa, celular)
                                        ↓
                              Painel exibe:
                              "LEGIÃO URBANA"
                              "Faroeste Caboclo"
```

**Protocolos de comunicação com o encoder:**

Os encoders RDS do mercado suportam um ou mais protocolos para receber dados do software de automação:

| Protocolo | Descrição | Encoders que suportam |
|-----------|-----------|----------------------|
| **UECP (Universal Encoder Communication Protocol)** | Protocolo binário padronizado (EBU Tech 3244). É o padrão da indústria — a maioria dos encoders profissionais suporta. Comunicação via serial ou TCP. | Quartz, P.H. Engineering, Broadcast Warehouse, DigiStar |
| **ASCII simples via serial/TCP** | Comandos de texto simples (ex: `PS=RADIOFLW\r\n`, `RT=Legião Urbana - Faroeste Caboclo\r\n`). Protocolo proprietário de cada fabricante, mas muito comum por ser fácil de implementar. | Pira.net, maioria dos encoders econômicos brasileiros |
| **HTTP REST** | Alguns encoders modernos expõem uma API REST para receber dados. Menos comum, mas crescente. | Encoders baseados em Linux embarcado |

**Encoders populares no mercado brasileiro:**

- **Pira.net RDS Encoder:** fabricante brasileiro, protocolo ASCII via serial ou UDP, muito usado em emissoras regionais. Documentação pública disponível.
- **Quartz RDS:** protocolo UECP, padrão em emissoras de grande porte.
- **P.H. Engineering (RDS-BMC):** serial UECP, popular na Europa e em emissoras brasileiras maiores.
- **Software encoder (PiRa32 / RDS-Sharp):** rodam no próprio PC ou em Raspberry Pi, recebem dados via TCP e injetam no sinal de áudio via placa de som — opção de baixo custo para web rádios que transmitem FM localmente.

**Por que é baixa prioridade:**

- RDS é exclusivo de emissoras FM físicas. Web rádios (que são o perfil inicial do RadioFlow) não têm transmissor FM e não usam RDS.
- A implementação do lado do RadioFlow é relativamente simples — o trabalho complexo já está feito pelo encoder de hardware. O RadioFlow só precisa de um cliente serial/TCP que envie strings de texto a cada troca de faixa.
- O evento `ItemStarted` já carrega todos os metadados necessários (título, artista, tipo). A integração com RDS é uma aplicação direta desse evento.

**Como os concorrentes implementam:**

- **RCS Zetta:** suporte nativo a UECP via serial e TCP. Envia PS, RT, RT+, PTY e CT automaticamente. Configuração de mapeamento de campos por tipo de programação (música diferente de notícia diferente de publicidade).
- **RadioBOSS:** suporte a múltiplos encoders via serial e TCP. Protocolos: UECP, ASCII Pira.net e outros. Configuração de template de RT com variáveis (ex: `%artist% - %title%`). Envia dados a cada troca de faixa e durante spots.
- **mAirList:** plugin de RDS com suporte a UECP e ASCII. Template configurável de RT. Suporte a RT+ (artista e título em campos separados).
- **RadioPro:** envio de RDS via serial com suporte ao encoder Pira.net — foco no mercado brasileiro.
- **PlayIt Live / RadioDJ:** sem suporte nativo a RDS.

**O que implementar no RadioFlow:**

1. **Cliente RDS no Playout Engine:** novo módulo `internal/rds` com interface `RDSClient` e implementações por protocolo:
   - `ASciIClient` — ASCII simples via TCP ou serial, compatível com Pira.net e encoders econômicos.
   - `UECPClient` — protocolo binário UECP (EBU Tech 3244), compatível com encoders profissionais.

2. **Gatilho por evento:** consumir `ItemStarted` no Event Bus. Ao receber, compor os campos RDS com os metadados da faixa e enviar ao encoder. Template configurável para o campo RT: `"{artist} - {title}"`, `"Tocando: {title}"`, `"{type}: {title}"`, etc.

3. **Configuração via API:** `GET/PUT /v1/config/rds` com:
   ```json
   {
     "enabled": true,
     "protocol": "ascii",
     "host": "192.168.1.50",
     "port": 7000,
     "ps": "RADIOFLW",
     "rt_template": "{artist} - {title}",
     "pty": 4,
     "send_ct": true
   }
   ```

4. **CT — Clock Time:** enviar o horário atual a cada minuto (evento de tick já existente no engine) para que receptores sincronizem o relógio.

5. **Limpeza ao parar:** ao receber `SessionEnded` ou `EngineStopped`, enviar RT vazio ou mensagem configurável (ex: `"Aguarde - voltamos em instantes"`) para evitar que o painel do receptor fique exibindo a última faixa indefinidamente.

6. **UI no player:** painel de configuração RDS com campo de template RT, preview do texto que será enviado, indicador de status de conexão com o encoder (conectado / desconectado / erro).

7. **Reconexão automática:** se a conexão com o encoder cair (encoder reiniciou, cabo desconectado), tentar reconectar com back-off exponencial — sem bloquear o pipeline de áudio.

**Componentes afetados:** Playout Engine (módulo `internal/rds`, consumidor de `ItemStarted` e tick de minuto), Player UI (painel de configuração RDS, status de conexão).

---

##### Controle remoto via web

**O que é:**
Capacidade de operar a emissora a partir de qualquer dispositivo com navegador — sem instalar o app Electron, sem estar fisicamente no estúdio. O operador acessa uma URL, autentica-se e tem acesso aos controles essenciais: play, pause, skip, fila, botoneira, volume e status em tempo real, com a mesma experiência do painel local.

**Diferença em relação ao app Electron atual:**

O app Electron é uma janela desktop que roda localmente na máquina do estúdio e se conecta ao Playout Engine via HTTP/WebSocket na rede local. O controle remoto via web é conceitualmente a mesma interface, mas servida como página web e acessível de qualquer dispositivo via internet:

```
[Estúdio — rede local]
App Electron ──► ws://localhost:8080 ──► Playout Engine
(desktop, mesma máquina)

[Remoto — internet]
Navegador ──► HTTPS ──► Proxy reverso ──► Playout Engine
(celular, tablet, laptop em qualquer lugar)
```

A boa notícia estrutural do RadioFlow: o `player.html` já é HTML puro conectado ao engine via REST e WebSocket. Tecnicamente ele já pode rodar em um navegador remoto se o engine estiver exposto na internet. O que falta é a camada de segurança, autenticação e servir a interface como aplicação web.

**Casos de uso:**

- **Locutor remoto:** apresentador que faz um programa de casa acompanha a fila, aciona a botoneira e vê o que está no ar — sem ir ao estúdio
- **Gerente de programação:** supervisor verifica do celular se a emissora está no ar e o que está tocando, sem depender do operador de plantão
- **Suporte emergencial:** técnico recebe alerta de dead air às 3h da manhã e restaura a operação remotamente — sem sair de casa
- **Web rádios distribuídas:** emissoras onde o "estúdio" é um servidor na nuvem e os operadores estão em cidades diferentes
- **Segunda tela:** operador usa o celular como painel de botoneira enquanto opera a fila no computador principal

**Níveis de acesso remoto:**

Não todos os operadores remotos precisam dos mesmos controles. Uma hierarquia típica:

| Perfil | O que pode fazer remotamente |
|--------|------------------------------|
| **Visualizador** | Ver o que está tocando, status do engine, fila (somente leitura) |
| **Locutor** | Acionar botoneira, ver fila, preview CUE — sem alterar programação |
| **Operador** | Tudo acima + play/pause/stop/skip, editar fila, trocar perfil de botoneira |
| **Administrador** | Tudo acima + configurações, agendamento, gestão de usuários |

**Como os concorrentes implementam:**

- **RCS Zetta — Zetta2GO:** suite de aplicações web que expõe o painel de controle do Zetta em qualquer navegador. Inclui controle de playback, hotkeys, voice tracking remoto e visualização da grade. Autenticação por usuário com perfis de acesso. Separação entre interface de operador (ao vivo) e interface de programação (editar grade futura).
- **mAirList — Remote Module:** módulo pago que expõe uma interface web simplificada via servidor HTTP embutido. Acesso a play/pause/skip, fila e carts. Autenticação por senha. Interface responsiva para tablet.
- **RadioBOSS — Web Interface:** interface web básica incluída na versão Advanced. Permite controle de playback e visualização da fila via navegador na rede local ou internet (com configuração de porta). Sem autenticação robusta — mais adequada para rede interna.
- **RadioPro:** sem controle remoto via web documentado. Operação presencial apenas.
- **PlayIt Live:** módulo "Remote Management" pago. Acesso remoto a controles básicos via browser. Foco em operação de estações de internet sem operador presencial.
- **RadioDJ:** sem controle remoto nativo.

**O que implementar no RadioFlow:**

A arquitetura do RadioFlow já tem todos os blocos fundamentais. A implementação é mais sobre segurança, serving e UX do que sobre novas features do engine.

1. **Servidor web embutido no Playout Engine:** adicionar um endpoint `GET /` no servidor HTTP existente que sirva o `player.html` (e seus assets: ícones, SVGs, logo) diretamente via HTTP. O engine já tem um servidor HTTP em `internal/api` — basta adicionar um handler `http.FileServer` para os assets estáticos. Com isso, qualquer navegador que acesse `http://engine-host:8080` recebe a interface completa.

2. **Autenticação:** o engine atualmente não tem autenticação. Para exposição segura na internet, é imprescindível:
   - **JWT (JSON Web Tokens):** endpoint `POST /v1/auth/login` com `{ "username": "...", "password": "..." }` retorna um token JWT assinado. Todas as rotas da API e a conexão WebSocket exigem o token no header `Authorization: Bearer <token>`.
   - **Refresh token:** tokens de acesso com expiração curta (ex: 15 min) + refresh token de longa duração (ex: 7 dias), para que sessões longas de operação não sejam interrompidas.
   - **Roles:** campo `role` no payload do JWT (`viewer`, `operator`, `admin`) usado pelo engine para autorizar ou rejeitar cada comando recebido.

3. **HTTPS obrigatório para acesso remoto:** o WebSocket moderno exige `wss://` (WebSocket Seguro) quando a página é servida via HTTPS. Opções:
   - **Proxy reverso com TLS:** Nginx ou Caddy na frente do engine, com certificado Let's Encrypt. O engine continua falando HTTP/WS internamente.
   - **TLS nativo no engine:** configurar `tls.Config` no servidor Go com certificado local ou ACME. Mais simples para deploys simples sem proxy.

4. **Interface web responsiva:** o `player.html` atual é otimizado para desktop wide-screen (três colunas). Para acesso remoto via celular ou tablet, criar uma variante responsiva com layout adaptativo:
   - Mobile: coluna única com Now Playing + controles essenciais + botoneira
   - Tablet: duas colunas (player + fila ou player + botoneira)
   - Desktop: layout atual completo de três colunas

5. **Perfis de interface por role:** ao carregar o `player.html` via web, o servidor injeta o role do usuário autenticado. A interface oculta controles não permitidos (ex: visualizador não vê botões de play/stop; locutor não vê configurações). Isso é cosmético — a autorização real é feita no backend, mas reduz erros e confusão.

6. **Gestão de usuários:** `GET/POST/PUT/DELETE /v1/users` para criar e gerenciar operadores com nome, senha (hash bcrypt) e role. Configurável via painel de administração no próprio player.html.

7. **Segurança de rede:** documentar e recomendar que o engine nunca seja exposto diretamente na internet sem TLS e autenticação. Para ambientes de baixo risco (rede local da emissora), o acesso sem autenticação pode ser mantido como opção configurável (`auth.enabled: false` no config).

8. **URL de acesso configurável:** `GET/PUT /v1/config/remote-access` com `{ "enabled": true, "base_url": "https://radio.example.com.br" }` — usado para gerar links de acesso compartilháveis e para configurar o WebSocket URL corretamente quando acessado por trás de proxy reverso.

**Dependência:** este item depende de **Gestão de usuários e permissões** (item 11 da média prioridade) para funcionar com segurança. Sem controle de acesso, expor o engine na internet permite que qualquer pessoa controle a emissora.

**Vantagem estrutural do RadioFlow:** por ser API-first com REST e WebSocket desde o início, o controle remoto via web é a feature com melhor custo-benefício de implementação do roadmap. O engine já é stateless por design — múltiplos clientes (Electron local + navegador remoto + tablet de locutor) podem se conectar simultaneamente ao mesmo engine sem conflito, com sincronização automática via WebSocket.

**Componentes afetados:** Playout Engine (serving de assets estáticos, autenticação JWT, autorização por role no middleware HTTP e WebSocket, TLS opcional), Player UI (layout responsivo, ocultação de controles por role, tela de login), Library Service (idem para autenticação nas rotas de biblioteca).

---

##### Multi-estúdio / multi-instância

**O que é:**
Capacidade de operar múltiplos estúdios independentes a partir de uma única infraestrutura — cada estúdio com sua própria fila, botoneira, agendamento e saída de áudio, mas com catálogo compartilhado, relatórios consolidados e monitoramento centralizado. Aplica-se tanto a múltiplos estúdios dentro de uma mesma emissora quanto a redes de afiliadas geograficamente distribuídas.

**Cenários de uso:**

**Cenário 1 — Múltiplos estúdios na mesma emissora:**
```
┌──────────────────────────────────────────────────────────┐
│  EMISSORA FM 104.5                                       │
│                                                          │
│  Estúdio A ──► Engine A ──► Transmissor FM principal     │
│  Estúdio B ──► Engine B ──► Webcast (streaming)          │
│  Estúdio C ──► Engine C ──► Backup / emergência          │
│                                                          │
│  Library Service central ◄── todos compartilham          │
│  Painel de monitoramento ◄── status de todos em tempo real│
└──────────────────────────────────────────────────────────┘
```

**Cenário 2 — Rede de afiliadas:**
```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│ Afiliada SP │  │ Afiliada RJ │  │ Afiliada BH │
│ Engine SP   │  │ Engine RJ   │  │ Engine BH   │
│ 104.5 FM    │  │  97.1 FM    │  │  98.3 FM    │
└──────┬──────┘  └──────┬──────┘  └──────┬──────┘
       │                │                │
       └────────────────┴────────────────┘
                        │
              ┌─────────▼──────────┐
              │  Servidor central   │
              │  Library Service    │
              │  Painel de rede     │
              │  Relatórios         │
              └────────────────────┘
```

**Cenário 3 — Cadeia de programação (networking):**
Uma rede nacional transmite um programa ao vivo a partir de um estúdio central, e as afiliadas inserem comerciais locais nos breaks previstos — cada afiliada entra com seu bloco local e depois retorna ao sinal da rede. O multi-estúdio gerencia automaticamente essa troca de fonte.

**Dois modelos de arquitetura:**

**Modelo 1 — Múltiplas instâncias independentes (recomendado):**
Cada estúdio roda uma instância separada do Playout Engine. As instâncias são independentes entre si — a falha de uma não afeta as outras. O Library Service é compartilhado (único banco de dados central). Um painel central agrega o status de todas as instâncias via polling das APIs `/v1/health` e `/v1/status`.

```
Engine A (estúdio A) ──► porta 8080
Engine B (estúdio B) ──► porta 8081   ──► Library Service :9090
Engine C (estúdio C) ──► porta 8082
          ↑ ↑ ↑
    Painel central consulta todos
```

Vantagens: isolamento de falhas, deployment independente por estúdio, escalabilidade horizontal. Desvantagem: recursos duplicados (cada engine tem seu próprio processo, decoder, mixer).

**Modelo 2 — Engine único com múltiplos outputs (complexo):**
Uma única instância do Playout Engine gerencia múltiplas "salas de reprodução" simultâneas, cada uma com fila, agendamento e output de áudio independentes. Mais eficiente em memória e CPU, mas muito mais complexo de implementar — requer refatoração profunda do core do engine para tornar cada sessão de playback uma entidade isolada.

Para o RadioFlow, o **Modelo 1 é o caminho recomendado** por ser compatível com a arquitetura atual e não exigir mudanças no core do engine.

**Como os concorrentes implementam:**

- **RCS Zetta:** multi-estúdio nativo via "Zetta Workgroup". Múltiplas instâncias do engine compartilham um banco de dados SQL Server central. Cada instância é um "estúdio" com ID único. O painel central ("Master Control") monitora todos os estúdios em tempo real. Suporte a networking para redes de afiliadas: estúdio central transmite e afiliadas inserem comerciais locais automaticamente nos breaks.
- **mAirList:** multi-instância via instalações separadas compartilhando um banco de dados PostgreSQL central. Painel de supervisão via módulo "Central Control". Sem suporte nativo a networking de afiliadas — requer configuração manual.
- **RadioBOSS:** sem multi-estúdio nativo. Cada instalação é completamente independente. Para redes de afiliadas, é necessário gerenciar cada instalação separadamente.
- **RadioPro:** suporte a múltiplos estúdios com banco de dados compartilhado. Foco no mercado brasileiro de redes regionais. Relatórios consolidados por rede.
- **PlayIt Live / RadioDJ:** sem multi-estúdio. Cada instalação é isolada.

**O que implementar no RadioFlow:**

A arquitetura atual do RadioFlow já está parcialmente preparada para o Modelo 1 — o Library Service é um serviço separado do engine por design. O que falta é a camada de coordenação central.

1. **Identificação de instância:** adicionar campo `studio_id` e `studio_name` na configuração do Playout Engine (`config.yaml`). Expor via `GET /v1/health` e `GET /v1/status`. Com isso, o painel central consegue distinguir cada instância.

2. **Painel central de monitoramento (novo serviço ou módulo do player):**
   Lista configurável de engines (`studio_id`, `host`, `port`). Para cada engine, exibe em tempo real: estado (PLAYING/IDLE/ERROR), modo (AUTO/ASSIST/PANIC), faixa atual, tempo restante e status de saúde. Atualizado via polling de `GET /v1/status` a cada segundo ou via WebSocket se disponível.

   ```
   ┌─────────────────────────────────────────────────────────┐
   │  REDE DE ESTÚDIOS — MONITORAMENTO CENTRAL               │
   ├─────────────┬─────────────┬─────────────┬───────────────┤
   │  Estúdio A  │  Estúdio B  │  Estúdio C  │  Afiliada SP  │
   │  ● PLAYING  │  ● PLAYING  │  ○ IDLE     │  ● PLAYING    │
   │  AUTO       │  ASSIST     │  —          │  AUTO         │
   │  MPB Atual  │  Spot Bloco │  —          │  Sertanejo    │
   │  02:14 ▶   │  00:28 ▶   │  —          │  03:45 ▶     │
   └─────────────┴─────────────┴─────────────┴───────────────┘
   ```

3. **Library Service compartilhado:** o Library Service já é um processo separado por design — múltiplos engines já podem apontar para o mesmo Library Service. O que falta é documentar e validar esse cenário, garantindo que operações concorrentes de múltiplos engines (enqueue simultâneo, log de transmissão de múltiplas instâncias) sejam thread-safe no SQLite (modo WAL) ou migrar para PostgreSQL para cargas mais altas.

4. **Relatórios consolidados por rede:** `GET /v1/log?studio_id=all` no Library Service retorna o log de transmissão de todos os engines que gravaram nele, com filtro por `studio_id`. Permite relatórios ECAD e prova de veiculação consolidados para toda a rede em uma única exportação.

5. **Agendamento compartilhado com inserção local:** para redes de afiliadas, o agendamento central define a grade principal (programas nacionais), e cada afiliada adiciona seus próprios eventos locais (comerciais regionais, hora certa local). Implementado como dois layers de grade: `scope: national` (herdada do servidor central) e `scope: local` (definida pela afiliada). Eventos locais têm prioridade sobre os nacionais nos horários configurados.

6. **Acesso ao painel central via controle remoto web:** o painel central de monitoramento é naturalmente uma aplicação web — sem cliente instalado, acessível de qualquer dispositivo. Depende da feature de **controle remoto via web** (item 16) para autenticação e serving seguro.

7. **Configuração de topologia:** `GET/PUT /v1/config/network` no Library Service define a lista de engines da rede, seus IDs, nomes e endereços. Usado pelo painel central para saber quais engines monitorar e pelo sistema de agendamento para saber onde distribuir eventos nacionais.

**Dependências:**
- **Controle remoto via web** (item 16): o painel central é por natureza uma interface web acessada remotamente
- **Log de transmissão** (item 1): relatórios consolidados dependem de logs por `studio_id`
- **Gestão de usuários e permissões** (item 11): operadores de afiliadas não devem ter acesso ao painel central completo

**Recomendação de roadmap:** implementar o multi-estúdio após as features de alta prioridade e após o controle remoto via web estar funcionando. O Modelo 1 (múltiplas instâncias + painel central) pode ser entregue como uma feature do player sem nenhuma mudança no Playout Engine — é essencialmente um painel de dashboard que consome as APIs já existentes de múltiplos engines simultaneamente.

**Componentes afetados:** Playout Engine (`studio_id` na configuração e nas respostas de `/v1/health` e `/v1/status`), Library Service (campo `studio_id` no log de transmissão, suporte a SQLite WAL para acesso concorrente, endpoint de relatório consolidado, configuração de topologia de rede), Player UI (novo painel central de monitoramento multi-estúdio, agendamento com scope nacional/local).

---

##### Controle por hardware (mesas de corte)

**O que é:**
Integração bidirecional entre o software de automação e o console de áudio físico do estúdio (mesa de corte), usando protocolos de comunicação ricos que vão além do simples GPI liga/desliga. O console transmite posição de faders, estado de botões e dados contínuos para o software; o software responde movendo faders motorizado, acendendo indicadores e enviando metering de volta ao console — criando um fluxo de controle totalmente integrado entre hardware e software.

**Diferença em relação ao GPI (item 13):**

| Aspecto | GPI simples | Protocolo de console |
|---------|------------|---------------------|
| Tipo de sinal | Binário (on/off) | Contínuo (valores 0–100%, estados múltiplos) |
| Direção | Predominantemente unidirecional | Bidirecional simultâneo |
| Informação por canal | 1 bit | Posição de fader, estado de botão, metering, nome |
| Faders motorizados | Não | Sim — software move o fader fisicamente |
| Metering no console | Não | Sim — VU meters do console refletem saída do engine |
| Configuração | Arquivo de mapeamento simples | Protocolo de descoberta automática de superfície |
| Custo de hardware | Qualquer botão/relé | Console de broadcast (R$ 15k–R$ 500k+) |

**Como funciona na prática:**

```
[Mesa de corte — Axia Element]
  Fader canal 1: Música principal
  Fader canal 2: Microfone locutor
  Botão ON canal 1 → abre fader → engine inicia música
  Botão OFF canal 1 → fecha fader → engine pausa música
  Fader canal 3: Cart machine → aciona botoneira slot do engine
  VU meters → exibem nível de saída do engine em tempo real
  Fader motorizado canal 1 → se move quando engine faz ducking

           ↕ protocolo Axia Livewire+ (TCP/IP)

[RadioFlow — Playout Engine]
  Recebe: fader ON/OFF, posição, botões de cart
  Envia: nível de saída (metering), nome da faixa atual, estado
```

**Protocolos relevantes:**

| Protocolo | Fabricante / Origem | Prevalência no Brasil | Características |
|-----------|--------------------|-----------------------|-----------------|
| **Axia Livewire+** | Telos Alliance | Alta (emissoras médias e grandes) | Protocolo IP dominante. Audio + controle + metering em uma rede Ethernet. Consoles Axia Element, iQ, Radius. |
| **WheatNet-IP** | Wheatstone | Média | Protocolo proprietário Wheatstone. Consoles L-Series, E-Series. Usado em redes de afiliadas. |
| **Ember+** | Lawo / Grass Valley / comunidade | Baixa (crescendo) | Protocolo aberto de controle de parâmetros. Usado por Lawo, Studer, Calrec. Tendência em broadcast europeu. |
| **MIDI / HID** | Universal | Alta (estúdios pequenos) | Controladores DJ e superfícies de controle econômicas. Mapeamento de Note On/Off e Control Change para comandos do engine. O mais acessível — R$ 300–R$ 3.000. |
| **OSC (Open Sound Control)** | Comunidade | Baixa | UDP/IP, usado por Reaper, alguns consoles modernos. Flexível mas sem padronização de parâmetros. |
| **Serial RS-232 + protocolo proprietário** | Vários | Alta (legado) | Consoles analógicos mais antigos com saída serial para automação. Protocolo ASCII específico por fabricante. |

**Cenários de integração por porte de emissora:**

| Porte | Console típico | Protocolo | Integração RadioFlow |
|-------|---------------|-----------|---------------------|
| Web rádio / pequena | Sem console físico | — | Não se aplica |
| Pequena / comunitária | Console analógico com GPI | Serial / GPI | Via item 13 (GPI) |
| Média | Console digital com GPI ou MIDI | MIDI / GPI | Via MIDI adapter |
| Grande / rede | Console IP (Axia, Wheatstone) | Livewire+ / WheatNet | Protocolo nativo |
| Broadcast premium | Console Lawo / Studer / Calrec | Ember+ / AES70 | Protocolo nativo (fase futura) |

**Como os concorrentes implementam:**

- **RCS Zetta:** integração nativa com Axia Livewire+ (protocolo completo — áudio, controle e metering). Suporte a WheatNet-IP. Mapeamento visual de faders e botões do console para ações do Zetta. Faders motorizados respondem ao ducking e ao crossfade do engine. É o benchmark da indústria nesse quesito.
- **mAirList:** suporte a MIDI completo (Note On/Off, Control Change, pitchbend para posição de fader). Plugin de superfície de controle configurável. Suporte a OSC. Sem suporte nativo a Axia ou Wheatstone — requer bridge externa.
- **RadioBOSS:** suporte a MIDI e a alguns consoles via protocolo serial. Sem suporte a Axia Livewire+ ou WheatNet-IP.
- **RadioPro:** integração com mesas via GPI serial e MIDI. Foco em consoles analógicos brasileiros. Sem suporte a consoles IP.
- **EBRcart2:** controle por hardware via GPI e MIDI. Cada botão do cart pode ser mapeado para um botão físico externo.
- **PlayIt Live / RadioDJ:** sem suporte a consoles além de MIDI básico.

**O que implementar no RadioFlow:**

O caminho de menor resistência é construir sobre a camada de abstração GPI já planejada (item 13), adicionando adaptadores de protocolo mais ricos para consoles específicos.

1. **Adaptador MIDI (prioridade imediata dentro do escopo):** o MIDI é o protocolo mais acessível e cobre a maioria das emissoras pequenas e médias. Implementar `MIDIAdapter` no módulo `internal/gpi` usando a biblioteca `gitlab.com/gomidi/midi/v2` (pura Go, sem CGo):
   - **Note On** → acionar comando (ex: Note 60 = Play, Note 61 = Stop, Note 62–73 = Slots da botoneira)
   - **Control Change** → valor contínuo (ex: CC 7 = volume principal, CC 8 = volume preview)
   - **MIDI Out** → enviar feedback visual ao controlador (LEDs nos botões piscam quando item está tocando)
   - Mapeamento configurável via arquivo JSON: `{ "note": 60, "command": "CmdPlay" }`, `{ "cc": 7, "action": "volume_main" }`

2. **Adaptador OSC:** protocolo simples baseado em UDP, implementado em puro Go sem dependências externas. Mapeamento de endpoints OSC para o Command Bus: `POST /engine/play`, `/engine/stop`, `/engine/hotkey/1`, etc. Permite integração com Reaper, TouchOSC (tablet como controle) e consoles modernos com suporte a OSC.

3. **Adaptador Axia Livewire+ (para emissoras com console Axia):** protocolo TCP/IP documentado pela Telos Alliance. O engine se conecta ao Axia node via TCP e recebe eventos de fader e botão no formato Livewire. Envia metering de volta ao console a cada frame de áudio. É o adaptador de maior impacto em emissoras de médio e grande porte no Brasil.
   - Descoberta automática de surfaces via multicast
   - Mapeamento de shows profiles: perfis de mapeamento diferentes para cada programa (manhã, tarde, ao vivo)

4. **Controle de fader motorizado via ducking:** quando o engine executa ducking automático (item 12), publicar o nível de ganho atual via o adaptador ativo (MIDI CC, OSC, Livewire+). Se o console tiver faders motorizados, o fader se move fisicamente na mesa — o operador vê o ducking acontecer no hardware sem olhar para o monitor.

5. **Surface discovery e configuração automática:** ao conectar um console IP (Axia, WheatNet), o RadioFlow identifica automaticamente os canais disponíveis e sugere um mapeamento padrão ao operador. Configuração manual disponível via painel de controle de hardware no player.

6. **UI no player — painel de superfície de controle:** visualização do mapeamento atual (qual botão/fader do hardware faz o quê), editor de mapeamento drag-and-drop, status de conexão por adaptador, log de eventos de hardware para diagnóstico.

7. **Adaptador Ember+ (fase futura):** protocolo aberto em crescimento no mercado europeu e em emissoras brasileiras maiores (Lawo, Studer). Biblioteca Go disponível (`github.com/dufourgilles/emberlib`). Implementar como adaptador plugável quando houver demanda.

**Relação com o item GPI (item 13):**
Este item e o GPI (item 13) compartilham a mesma camada de abstração `internal/gpi` e a mesma interface `GPIAdapter`. O GPI cobre o caso simples (contato elétrico), enquanto este item cobre protocolos ricos sobre IP e MIDI. Na prática, implementar ambos simultaneamente faz sentido — o MIDI adapter, em especial, é mais simples que o GPI serial e já cobre a maior parte das emissoras de pequeno e médio porte.

**Componentes afetados:** Playout Engine (adaptadores MIDI, OSC e Axia Livewire+ no módulo `internal/gpi`; publicação de metering e estado para hardware; integração com ducking para faders motorizados), Player UI (painel de superfície de controle, editor de mapeamento, status de conexão por adaptador).

---

##### API para traffic systems (publicidade)

**O que é:**
Traffic system é o software de gestão comercial de uma emissora: controla o inventário de spots, os contratos de anunciantes, a programação de inserções e o faturamento. O software de automação (RadioFlow) é o executor — ele recebe o bloco de comerciais já montado (broadcast log) e o reproduz; depois informa ao traffic system exatamente o que foi ao ar (as-run log) para fins de faturamento e compliance.

**Fluxo de dados:**

```
Traffic System (WideOrbit, Natural Log, Dalet, etc.)
        │
        │  broadcast log (o que deve ir ao ar)
        │  formato: XML, JSON, CSV, texto fixo, FTP
        ▼
[RadioFlow — importer de grade]
        │
        │  executa os spots na hora certa
        ▼
[RadioFlow — log de transmissão (as-run log)]
        │
        │  o que efetivamente foi ao ar (ID, horário real, duração real)
        │  formato: XML, JSON, CSV, texto fixo
        ▼
Traffic System
        │
        ├── confirma inserções para faturamento
        ├── gera relatório ANATEL (Brasil)
        └── processa ECAD/UBC para direitos autorais
```

**Por que é crítico:**
Em uma emissora comercial, publicidade é a principal fonte de receita. Sem integração com o traffic system:
- O operador importa a grade manualmente, sujeito a erros.
- O log de as-run precisa ser conferido e corrigido à mão.
- A fatura ao anunciante pode não bater com o que foi ao ar → devolução de verba.
- A obrigação de declaração à ANATEL (emissoras licenciadas no Brasil) fica em risco.

Rádios com 1–2 locutores e 50+ spots/dia não conseguem operar sem essa automação.

**Formatos e protocolos usados no mercado:**

| Formato / protocolo | Quem usa | Direção |
|---------------------|----------|---------|
| ADS (Automation Data Standard) v2 XML | RCS Zetta, Dalet, WideOrbit | broadcast log → automação |
| GDCP (Generic Data Communication Protocol) | WideOrbit ↔ automação | bidirecional |
| Natural Log XML / CSV fixo | Natural Log ↔ sistemas regionais | broadcast log → automação |
| As-run log CSV/TSV (formato proprietário) | cada sistema define o seu | automação → traffic |
| FTP / SFTP / pasta compartilhada | mecanismo de transporte mais comum | ambos |
| REST API (JSON) | sistemas modernos, WideOrbit v6+ | bidirecional |

**Conteúdo típico do broadcast log (entrada):**

```json
{
  "date": "2026-07-15",
  "segments": [
    {
      "scheduled_time": "08:00:00",
      "type": "COMMERCIAL",
      "spot_id": "SPOT-2034",
      "filename": "spot_cerveja_abc.mp3",
      "duration_ms": 30000,
      "advertiser": "Cerveja ABC",
      "contract_id": "CT-0412",
      "mandatory": true
    }
  ]
}
```

**Conteúdo típico do as-run log (saída):**

```json
{
  "date": "2026-07-15",
  "station_id": "RF-SP-01",
  "log": [
    {
      "spot_id": "SPOT-2034",
      "scheduled_time": "08:00:00",
      "actual_start": "08:00:03",
      "actual_end": "08:00:33",
      "duration_ms": 30012,
      "status": "PLAYED",
      "filename": "spot_cerveja_abc.mp3"
    }
  ]
}
```

**Status vs. concorrentes:**

| Produto | Integração com traffic |
|---------|------------------------|
| RadioPro | ✅ Integração com sistemas nacionais (Natural Log, TVS) |
| RCS Zetta | ✅ ADS XML nativo, GDCP bidirecional, REST API v6 |
| mAirList | ✅ Plugin de importação de broadcast log (formatos configuráveis) |
| RadioBOSS | 🔲 Não possui integração nativa com traffic systems |
| PlayIt Live | 🔲 Focado em rádios pequenas; sem traffic system |
| RadioDJ | 🔲 Open-source; sem integração de traffic |
| AudioMaster (EBR) | ✅ Sistema próprio integrado (EBRcart2 + traffic interno) |
| **RadioFlow** | 🔲 **Não implementado** |

**O que implementar no RadioFlow:**

1. **Definir o modelo de grade comercial no Library Service**
   - Nova tabela `commercial_log` com campos: `date`, `scheduled_time`, `spot_id`, `filename`, `duration_ms`, `advertiser`, `contract_id`, `mandatory`, `status` (SCHEDULED / PLAYED / MISSED / SKIPPED).
   - Endpoint `POST /v1/commercial-log/import` que aceita JSON, CSV ou XML (multipart upload).
   - Endpoint `GET /v1/commercial-log?date=YYYY-MM-DD` para o Playout Engine consultar a grade do dia.

2. **Importador de broadcast log no Library Service**
   - Módulo `internal/traffic/importer` com adaptadores para:
     - JSON nativo RadioFlow (formato próprio).
     - CSV de largura fixa (Natural Log e similares brasileiros).
     - ADS XML (compatibilidade com RCS Zetta e WideOrbit).
   - Validação de campos obrigatórios e de existência do arquivo de áudio antes de confirmar a importação.
   - Endpoint `GET /v1/commercial-log/import/status/:job_id` para acompanhar importações grandes via polling.

3. **Integração do Playout Engine com a grade comercial**
   - O Playout Engine consulta `GET /v1/commercial-log?date=YYYY-MM-DD` ao inicializar o dia.
   - No modo AUTO, quando o relógio interno chega no horário de um spot com `mandatory: true`, o Engine interrompe a música corrente (cross-fade curto) e injeta o spot na cabeça da fila.
   - Após a reprodução, publica evento `SpotPlayed` com `actual_start`, `actual_end` e `duration_ms` reais.

4. **Gerador de as-run log no Library Service**
   - O Library Service consome o evento `SpotPlayed` via WebSocket ou endpoint de push `POST /v1/as-run` do Playout Engine.
   - Atualiza o campo `status` do registro no `commercial_log`.
   - Endpoint `GET /v1/as-run/export?date=YYYY-MM-DD&format=json|csv|xml` para exportação.
   - Exportação agendada automática via cron interno (ex.: 23:59 do dia, gera arquivo e deposita em pasta configurável ou envia via SFTP).

5. **Relatório ANATEL / ECAD (compliance Brasil)**
   - Endpoint `GET /v1/reports/transmission-log?date=YYYY-MM-DD` já existente deve incluir os comerciais executados.
   - Novo endpoint `GET /v1/reports/ecad?month=YYYY-MM` consolidando músicas e jingles executados para declaração à ECAD/UBC.
   - Campo `is_music` no `commercial_log` para distinguir spots de jingles musicais (sujeitos a ECAD).

6. **Player UI — painel de grade comercial**
   - Visualização da grade do dia com status (SCHEDULED / PLAYED / MISSED) em cores distintas.
   - Upload manual de broadcast log (drag-and-drop de arquivo CSV/XML/JSON).
   - Botão "Exportar as-run" disponível a partir do meio-dia para supervisores.
   - Alerta visual quando um spot `mandatory: true` estiver dentro de 2 minutos e a fila não o contiver.

7. **Configuração de integração automática**
   - Suporte a pasta monitorada (`watch_dir`) onde o traffic system deposita o broadcast log automaticamente.
   - Configuração de SFTP remoto para buscar o log em sistemas legados.
   - Reprocessamento de importação sem duplicar registros (idempotência via `spot_id` + `date` + `scheduled_time`).

**Componentes afetados:** Library Service (tabela `commercial_log`, módulo `internal/traffic/importer`, exportador de as-run, endpoints de compliance ANATEL/ECAD), Playout Engine (consulta de grade comercial, injeção de spots mandatórios na fila, evento `SpotPlayed`, push de as-run para o Library Service), Player UI (painel de grade comercial, upload de broadcast log, exportação de as-run, alertas de spot mandatório pendente).

---

## 7. Fontes consultadas

- [RadioPro Prime](https://www.radiopro.com.br/radiopro-site/software-para-emissoras-de-radio-prime/) — funcionalidades e mercado brasileiro
- [EBRaudio / EBRcart2](https://www.ebraudio.com/radioautomation_p.htm) — cart machine digital brasileiro
- [RCS Zetta](https://www.rcsworks.com/zetta/) — referência técnica de mercado
- [RadioBOSS](https://manual.djsoft.net/radioboss/en/) — manual completo de funcionalidades
- [mAirList](https://www.mairlist.com/en/products/radio-automation/) — produto europeu profissional
- [PlayIt Live](https://www.playitsoftware.com/Products/Live) — solução gratuita para internet radio
- [CloudRadio — 20 Best Broadcasting Software](https://www.cloudrad.io/blog/radio-broadcasting-software) — panorama geral
- [Tudo Para Rádios — Automação paga](https://www.tudopraradios.com.br/operacional/automacao-de-radios/) — mercado brasileiro
