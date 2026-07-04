# 99 — Decisões Arquiteturais

## ADR-001 — Engine separado da UI

Decisão: o Playout Engine será processo separado.

Motivo:

- UI pode travar sem afetar áudio.
- Engine pode ser testado isoladamente.
- Menor acoplamento.

## ADR-002 — REST para comandos e WebSocket para eventos

Decisão:

- REST para comandos.
- WebSocket para eventos.

Motivo:

- Simples para UI.
- Fácil de testar com curl.
- Eventos live sem polling agressivo.

## ADR-003 — Fila em memória na primeira versão

Decisão: não usar banco no Engine inicialmente.

Motivo:

- Engine deve ser pequeno.
- Persistência pertence a outro serviço ou versão futura.
- Reduz risco no hot path.

## ADR-004 — Engine recebe itens já resolvidos

Decisão: Engine não conhece biblioteca.

Recebe:

- asset_id;
- path;
- tipo;
- duração;
- metadata operacional.

Motivo:

- Desacoplamento.
- Engine reutilizável.
- UI/Scheduler podem evoluir separadamente.

## ADR-005 — Audio core por interfaces

Decisão: decoder, mixer e output por interfaces.

Motivo:

- Testabilidade.
- Suporte multiplataforma.
- Troca futura de tecnologia.

## ADR-006 — FFmpeg como decoder inicial

Decisão: primeira implementação de decoder usa FFmpeg.

Motivo:

- Suporte robusto a MP3/WAV.
- Menor risco com arquivos variados.
- Simplicidade para MVP.

Trade-off:

- Dependência externa.
- Necessário validar ffmpeg no startup.

## ADR-007 — Formato interno float32 estéreo 48kHz

Decisão: mixer usa float32 interleaved stereo 48kHz.

Motivo:

- Mixagem mais simples.
- Menos risco de clipping intermediário.
- Padrão adequado para rádio local.

## ADR-008 — Panic mode como prioridade máxima

Decisão: panic interrompe tudo e toca bed de segurança.

Motivo:

- Rádio não pode ficar muda.
- Operação precisa de mecanismo simples de sobrevivência.

## ADR-009 — Observabilidade desde o início

Decisão: health, status, eventos e logs desde o MVP.

Motivo:

- Debug de áudio é difícil.
- Estado deve ser visível para UI.
- Facilita desenvolvimento com Claude Code.

## ADR-010 — Não implementar ingestão no Engine

Decisão: o Engine não recebe upload nem cadastra arquivos.

Motivo:

- Upload/análise é pesado.
- Pode competir com áudio.
- Deve ser responsabilidade de outro serviço.
