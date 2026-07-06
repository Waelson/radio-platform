# Roadmap — Radio Playout Engine

## Implementado

| Feature | Detalhes |
|---|---|
| Bootstrap, config, logger | Config loader (flags > env > yaml > defaults), slog estruturado |
| Health / Status / Build / Info | `GET /v1/health`, `/v1/ready`, `/v1/status`, `/v1/build`, `/v1/info` |
| Command Bus + Event Bus + Dispatcher | Envelope tipado, dispatcher com handlers registráveis, eventos publicados no bus |
| Queue Manager | `enqueue`, `insert-next`, `insert-after`, `clear`, `remove`, `move`, `reorder` |
| Persistência da fila | FileStore JSON, restore on start, clear on stop |
| NullOutput + FileOutput | Para testes e CI; sem dependências externas |
| FFmpegDecoder | Subprocesso isolado; PCM float32 48kHz stereo |
| Playback completo | play, pause, resume, stop, skip, progress, now playing |
| Crossfade automático | Crossfade linear + auto-crossfade por análise de energia (threshold configurável) |
| Modo Assist | Operador controla manualmente; engine aguarda `PLAY` |
| Modo Panic | Panic bed em loop; auto-panic por silêncio configurável |
| Hot Buttons | overlay, interrupt, after-current; ducking com gain atômico |
| Audio Health Monitor | RMS, peak, detecção de silêncio, underrun count, eventos `AudioHealthChanged` |
| VU Meter EBU R128 | LUFS integrado, peak hold configurável, eventos `VUMeterUpdated` |
| WebSocket de eventos | `/v1/events`; fan-out para clientes; eventos críticos não descartados |
| Máquina de estados | STARTING → IDLE → PLAYING → PAUSED → ASSIST → PANIC → STOPPING → ERROR |
| PortAudio driver | Linux / Windows / macOS; build tag `portaudio`; `id == name` |
| CoreAudio driver (macOS) | AudioQueue nativo; ID = `kAudioDevicePropertyDeviceUID` (estável); resolução UID → nome → default |
| WASAPI driver (Windows) | COM shared-mode; ID = `IMMDevice::GetId()` GUID (estável); resolução GUID → nome → default; build tag `wasapi` |
| `GET /v1/devices` | Listagem live de dispositivos; campo `host_api`; `Cache-Control: no-store` |
| Campo `host_api` | Expõe host API subjacente (ALSA, PulseAudio, JACK, CoreAudio, WASAPI) para avaliação de estabilidade no Linux |
| Preview player (cue) | Player isolado do pipeline principal; play, pause, resume, stop, seek |
| Lock de instância | Previne dois processos com mesmo `engine.id`; Unix: lock file; Windows: mutex nomeado |
| Hora Certa | Anuncia hora + minuto com arquivos de áudio configuráveis; gain_db ajustável |
| RadioCore.app (macOS) | Systray (ícone verde/vermelho), webview, first-run setup, logs em `~/RadioFlow/` |
| Métricas | `GET /v1/metrics`; contadores de eventos do bus |
| SPA de status | `GET /status`; painel HTML com auto-refresh 5s e detecção offline |

---

## Pendente

### Funcionais

| Feature | Descrição | Esforço |
|---|---|---|
| **Scheduling / programação horária** | Disparar itens em horário fixo (ex: jingle toda hora cheia); integração com `cron` interno ou agenda configurável | Médio |
| **Cue points automáticos por energia** | Detectar intro/outro do áudio por análise de RMS para posicionar `cue_in_ms` e `cue_out_ms` automaticamente | Alto |
| **Persistent device preferences** | Lembrar `device_id` por `engine.id` em disco; restaurar na próxima inicialização | Baixo |

### Qualidade / Hardening

| Item | Descrição | Esforço |
|---|---|---|
| **Testes de integração com FileOutput** | Verificar crossfade, ducking e panic bed comparando PCM de saída; garantia auditável de comportamento | Médio |
| **Race tests (`go test -race`)** | Garantia formal de ausência de data races em todos os pacotes | Baixo |
| **Testes de arquivos corrompidos** | Decoder recebe input inválido, truncado ou formato desconhecido; engine não trava nem entra em loop | Baixo |
| **Load tests da API** | Centenas de requests simultâneos sem degradar o pipeline de áudio; validação do isolamento entre goroutines | Baixo |

### Plataforma

| Item | Descrição | Esforço |
|---|---|---|
| **Validação WASAPI no Windows** | Compilar com `-tags wasapi` e testar em ambiente Windows real (MinGW-w64); validar IDs estáveis após rename de dispositivo | Médio |
| **Packaging Linux** | systemd unit file, script de instalação, pacote `.deb` / `.rpm` | Baixo |
| **Packaging Windows** | Instalador `.msi` ou `.exe` autocontido com WASAPI; equivalente ao RadioCore.app no macOS | Médio |
