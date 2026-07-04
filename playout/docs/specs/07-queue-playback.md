# 07 — Fila e Playback

## Objetivo

Definir como o Engine recebe itens, mantém fila em memória e executa áudio.

## Queue Item

Estrutura conceitual:

```go
type QueueItem struct {
    QueueItemID string
    AssetID string
    Path string
    Type AssetType
    Title string
    Artist string
    DurationMS int64
    CueInMS int64
    CueOutMS int64
    Transition TransitionSpec
    Mandatory bool
    Metadata map[string]string
}
```

Tipos:

```text
MUSIC
SPOT
JINGLE
BED
EFFECT
VOICE
UNKNOWN
```

## TransitionSpec

```go
type TransitionSpec struct {
    Type string // CUT, FADE_OUT, CROSSFADE, HARD
    DurationMS int64
}
```

## Estados do item

```text
QUEUED
PRELOADING
PLAYING
FADING_OUT
PLAYED
SKIPPED
FAILED
MISSED
```

## Fila em memória

Na primeira versão, a fila é exclusivamente em memória.

Regras:

- Itens recebem `queue_item_id` no enqueue.
- Item atual é separado da fila pendente.
- `ClearQueue` não deve remover item em execução, salvo se `stop=true`.
- Todas as alterações emitem `QueueChanged`.

## Execução automática

Loop conceitual:

```text
while engine running:
  if state == PLAYING and no current item:
    pop next item
    start decoder
    set current
    publish NowPlayingChanged

  if current item finished:
    mark played
    clear current
    continue
```

## Avanço para próxima música

Sem crossfade:

```text
item A toca até EOF
Engine marca A como PLAYED
Engine inicia B
```

Com crossfade:

```text
quando position(A) >= cue_out(A) - crossfade_duration:
  pre-load B
  iniciar canal next
  aplicar fade out em A e fade in em B
  ao final do crossfade:
    A é PLAYED
    B vira main
```

## Preload

O Engine deve tentar pré-carregar o próximo item antes do ponto de transição.

MVP:

- Preload 2 segundos antes do crossfade.

Recomendado:

- Sempre manter próximo item validado.

## Skip

Comando `SKIP`:

- Se item atual for obrigatório e política bloquear, rejeitar.
- Caso contrário, aplicar transição configurada.
- Marcar item como `SKIPPED`.
- Avançar para próximo item.

## Stop

Comando `STOP`:

- Parar item atual.
- Opcionalmente limpar fila.
- Publicar `PlayerStateChanged`.

## Pause/Resume

Pausar deve interromper avanço do playback, mas não deve destruir a fila.

## Item obrigatório

`mandatory=true` indica que o item não deve ser pulado em operação normal.

O Engine pode rejeitar:

- skip;
- remove;
- reorder;
- clear parcial.

## Logs de execução

Cada item deve gerar resultado:

```text
PLAYED
SKIPPED
FAILED
MISSED
INTERRUPTED_BY_PANIC
```

Evento:

```json
{
  "type": "ItemFinished",
  "payload": {
    "queue_item_id": "qi_001",
    "asset_id": "asset_123",
    "result": "PLAYED",
    "duration_played_ms": 240000
  }
}
```
