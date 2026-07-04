# 08 — Crossfade, Ducking e Botoneira

## Crossfade

### Definição

Crossfade é a transição em que o item atual reduz volume enquanto o próximo aumenta volume.

```text
A: 100% ─────────────╲
                      ╲
B: 0%   ──────────────╱──────── 100%
```

### Regra inicial

Para `MUSIC → MUSIC`:

- usar `CROSSFADE`.
- duração padrão: 8000ms.

Para `MUSIC → SPOT`:

- evitar crossfade.
- usar fade out curto ou cut.

Para `SPOT → SPOT`:

- usar cut.

Para `PANIC`:

- cut imediato.

## Crossfade por tempo

A primeira versão deve iniciar crossfade quando faltar `duration_ms` para o fim do item atual.

Exemplo:

```text
position >= cue_out_ms - crossfade_duration_ms
```

Se `cue_out_ms` não for fornecido:

```text
cue_out_ms = duration_ms
```

## Crossfade por envelope

Futuro: usar análise de áudio para encontrar melhor ponto.

Campos reservados:

```json
{
  "recommended_xfade_start_ms": 238000
}
```

## Algoritmo de crossfade

Para cada frame durante o fade:

```text
progress = elapsed / fade_duration
main_gain = 1 - progress
next_gain = progress
out = main * main_gain + next * next_gain
```

Usar equal-power crossfade futuramente:

```text
main_gain = cos(progress * PI / 2)
next_gain = sin(progress * PI / 2)
```

MVP pode usar linear.

## Ducking

### Definição

Ducking reduz o volume do canal principal quando outro canal prioritário toca.

Exemplo:

- Música toca a 0 dB.
- Hot button dispara vinheta.
- Música reduz para -8 dB.
- Vinheta toca.
- Ao terminar, música volta para 0 dB.

### Parâmetros

```json
{
  "duck_main": true,
  "duck_gain_db": -8,
  "attack_ms": 150,
  "release_ms": 500
}
```

## Botoneira / Hot Buttons

### Objetivo

Permitir disparo instantâneo de áudios sem mexer diretamente na fila principal.

### Modos de execução

```text
OVERLAY
INTERRUPT
AFTER_CURRENT
```

#### OVERLAY

Toca sobre o programa.

Pode aplicar ducking.

#### INTERRUPT

Interrompe o programa atual.

Usar com cuidado.

#### AFTER_CURRENT

Insere item após o atual.

Equivalente a `insert-next`.

## Prioridades

Ordem sugerida:

```text
PANIC > INTERRUPT HOT BUTTON > SPOT MANDATORY > HOT BUTTON OVERLAY > MUSIC
```

## Eventos

### HotButtonTriggered

```json
{
  "type": "HotButtonTriggered",
  "payload": {
    "button_id": "hb_001",
    "asset_id": "vinheta_001",
    "play_mode": "OVERLAY"
  }
}
```

### DuckingStarted

```json
{
  "type": "DuckingStarted",
  "payload": {
    "target_channel": "main",
    "gain_db": -8
  }
}
```

### DuckingEnded

```json
{
  "type": "DuckingEnded",
  "payload": {
    "target_channel": "main"
  }
}
```

## Restrições

- Hot buttons não devem bloquear o thread de áudio.
- Áudio da botoneira deve ser pré-validado quando possível.
- Se hot button falhar, publicar evento e manter programa principal.
- Panic sempre interrompe hot buttons.
