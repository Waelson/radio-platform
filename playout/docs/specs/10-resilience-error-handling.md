# 10 — Resiliência e Tratamento de Erros

## Objetivo

Garantir que falhas sejam previsíveis, observáveis e tratadas sem derrubar o processo sempre que possível.

## Princípios

1. O pipeline de áudio não pode depender da UI.
2. Erro de um item não deve derrubar a fila inteira.
3. Panic deve ter prioridade máxima.
4. Falhas devem gerar eventos e logs estruturados.
5. Componentes devem ter timeout e contexto.

## Classes de erro

### Payload inválido

Origem: API.

Ação:

- HTTP 400.
- `CommandRejected`.
- Não alterar estado.

### Comando inválido para estado atual

Exemplo: `RESUME` quando `IDLE`.

Ação:

- ACK rejeitado.
- Evento `CommandRejected`.

### Arquivo inexistente

Origem: decoder open.

Ação:

- Marcar item como `FAILED`.
- Publicar `DecoderError`.
- Tentar próximo item.

### Decoder falhou no meio

Ação:

- Marcar item como `FAILED`.
- Tentar próximo item.
- Se fila vazia, entrar em panic se configurado.

### Buffer underrun

Origem: output buffer.

Ação:

- Incrementar contador.
- Publicar `AudioHealthChanged`.
- Se ultrapassar limite, publicar alerta.
- Se crítico, entrar em panic.

### Silêncio detectado

Origem: health monitor.

Ação:

- Se `silence_duration_ms > threshold`, publicar alerta.
- Se `auto_panic_on_silence=true`, entrar em panic.

### Output device falhou

Ação:

- Tentar reabrir se configurado.
- Se não recuperar, entrar em `ERROR`.

## Panic Mode

Panic é o mecanismo de sobrevivência.

Ao entrar em panic:

1. Publicar `PanicEntered`.
2. Parar decoders normais.
3. Limpar/ignorar pipeline principal.
4. Tocar bed de segurança.
5. Rejeitar comandos não permitidos.

## Bed de segurança

Configuração:

```yaml
panic:
  enabled: true
  bed_path: "/library/beds/panic-bed.mp3"
  auto_on_silence: true
  silence_threshold_dbfs: -60
  silence_duration_ms: 2000
```

## Timeouts

- Command handling: 2s.
- Decoder open: 5s.
- Output open: 5s.
- Shutdown: 10s.

## Retentativas

Retentativa automática deve ser conservadora.

Permitido:

- reabrir output device 1 a 3 vezes;
- tentar próximo item se arquivo falhar.

Não permitido:

- loop infinito de retry;
- retry bloqueando o audio thread;
- retry sem log.

## Circuit breaker interno

Se muitos itens falharem consecutivamente:

```yaml
playback:
  max_consecutive_item_failures: 3
```

Ação:

- entrar em panic ou error.

## Eventos de erro

```json
{
  "type": "PlaybackError",
  "payload": {
    "code": "decoder_open_failed",
    "message": "file not found",
    "queue_item_id": "qi_001",
    "recoverable": true
  }
}
```

## Logs

Todo erro deve conter:

- `error_code`
- `component`
- `command_id` se aplicável
- `queue_item_id` se aplicável
- `asset_id` se aplicável
- `recoverable`
- `action_taken`
