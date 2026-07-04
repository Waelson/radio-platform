# 05 — Máquina de Estados

## Estados principais

```text
STARTING
IDLE
PLAYING
PAUSED
ASSIST
PANIC
STOPPING
ERROR
```

## Modos

`mode` é diferente de `state`.

Estados representam execução atual. Modos representam comportamento operacional.

Modos:

```text
AUTO
ASSIST
PANIC
```

## Estado STARTING

Usado durante inicialização.

Transições:

- `STARTING → IDLE` quando API e áudio estiverem prontos.
- `STARTING → ERROR` se output de áudio falhar e configuração exigir output real.

## Estado IDLE

Engine ativo, sem áudio tocando.

Comandos aceitos:

- `ENQUEUE`
- `PLAY`
- `CLEAR_QUEUE`
- `ENTER_PANIC`

Transições:

- `IDLE → PLAYING` quando `PLAY` for aceito e houver fila.
- `IDLE → PANIC` quando `ENTER_PANIC` for aceito.

## Estado PLAYING

Engine tocando item da fila.

Comandos aceitos:

- `PAUSE`
- `STOP`
- `SKIP`
- `ENQUEUE`
- `INSERT_NEXT`
- `ENTER_ASSIST`
- `ENTER_PANIC`
- `TRIGGER_HOT_BUTTON`

Transições:

- `PLAYING → PAUSED`
- `PLAYING → IDLE` se fila terminar.
- `PLAYING → ASSIST` se operador ativar Assist Mode.
- `PLAYING → PANIC` se panic for acionado ou falha crítica ocorrer.
- `PLAYING → ERROR` se falha irrecuperável ocorrer.

## Estado PAUSED

Pipeline pausado.

Comandos aceitos:

- `RESUME`
- `STOP`
- `ENTER_PANIC`

Transições:

- `PAUSED → PLAYING`
- `PAUSED → IDLE`
- `PAUSED → PANIC`

## Estado ASSIST

Modo de operação assistida.

Diferenças:

- Operador pode inserir itens com prioridade.
- Scheduler externo pode controlar menos decisões.
- Itens obrigatórios continuam protegidos.

Comandos aceitos:

- `RETURN_AUTO`
- `SKIP`
- `STOP`
- `INSERT_NEXT`
- `TRIGGER_HOT_BUTTON`
- `ENTER_PANIC`

Transições:

- `ASSIST → PLAYING` quando retornar ao modo automático.
- `ASSIST → PANIC` quando panic for acionado.
- `ASSIST → IDLE` quando fila terminar.

## Estado PANIC

Estado de emergência.

Comportamento:

- Pipeline principal é interrompido.
- Buffers podem ser limpos.
- Áudio de segurança é tocado.
- Fila principal fica suspensa.
- Comandos normais podem ser rejeitados.

Comandos aceitos:

- `EXIT_PANIC`
- `STOP`
- `STATUS`

Transições:

- `PANIC → IDLE` se sair sem retomar.
- `PANIC → PLAYING` se retomar fila.
- `PANIC → ERROR` se panic bed falhar.

## Estado ERROR

Estado de falha.

Ações:

- Publicar alerta crítico.
- Expor erro em `/v1/status`.
- Permitir tentativa de reset.

Comandos aceitos:

- `RESET`
- `ENTER_PANIC`
- `STATUS`

## Tabela de transições

| Estado atual | Evento | Próximo estado | Observação |
|---|---|---|---|
| STARTING | EngineReady | IDLE | Inicialização OK |
| IDLE | Play | PLAYING | Se fila não vazia |
| PLAYING | Pause | PAUSED | Pausa pipeline |
| PAUSED | Resume | PLAYING | Retoma pipeline |
| PLAYING | QueueEmpty | IDLE | Fim natural |
| PLAYING | EnterAssist | ASSIST | Operação manual assistida |
| ASSIST | ReturnAuto | PLAYING | Volta ao automático |
| PLAYING | EnterPanic | PANIC | Emergência |
| ASSIST | EnterPanic | PANIC | Emergência |
| PANIC | ExitPanic | IDLE/PLAYING | Conforme payload |
| Any | FatalError | ERROR | Falha irrecuperável |
| ERROR | Reset | IDLE | Se reset OK |

## Regras de proteção

- `SKIP` pode ser rejeitado em item `mandatory=true`.
- `STOP` sempre é aceito, exceto se o Engine estiver `STOPPING`.
- `ENTER_PANIC` sempre tem prioridade máxima.
- `TRIGGER_HOT_BUTTON` pode ser rejeitado se output não estiver pronto.
- Eventos de UI nunca mudam estado diretamente; sempre viram comandos.
