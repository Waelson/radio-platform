# 17 — Scheduler / Programação Horária

## Objetivo

Permitir que o operador configure uma grade horária com entradas que disparam automaticamente em horários pré-definidos — seja de forma recorrente (via expressão cron) ou como evento único (one-shot via `fire_at`).

O Scheduler não acessa o pipeline de áudio diretamente. Ao disparar, ele envia comandos ao **Command Bus** exatamente como se o operador tivesse pressionado um botão na UI. O resultado é visível via eventos WebSocket.

---

## Conceitos fundamentais

### Entrada (`Entry`)

Cada entrada da grade possui:

| Campo | Descrição |
|---|---|
| `id` | ID único gerado pelo engine (ULID) |
| `name` | Nome legível (ex: "Noticiário das 10h") |
| `enabled` | Se `false`, o scheduler ignora a entrada completamente |
| `cron_expr` | Expressão cron recorrente (mutuamente exclusivo com `fire_at`) |
| `fire_at` | Data/hora de disparo único em RFC 3339 (mutuamente exclusivo com `cron_expr`) |
| `trigger_mode` | Como o item entra na reprodução |
| `item` | Item de playback (path, tipo, título, duração etc.) |
| `created_at` | Quando a entrada foi criada |
| `last_fired_at` | Último disparo bem-sucedido |
| `next_fire_at` | Próximo disparo calculado (somente para entradas cron) |

---

## Modo recorrente: `cron_expr`

Usa expressões cron de **5 campos** no formato padrão:

```
┌─── minuto      (0–59)
│ ┌─── hora       (0–23)
│ │ ┌─── dia do mês (1–31)
│ │ │ ┌─── mês       (1–12)
│ │ │ │ ┌─── dia da semana (0–7, 0 e 7 = domingo)
│ │ │ │ │
* * * * *
```

### Exemplos

| Expressão | Significado |
|---|---|
| `0 10 * * *` | Todo dia às 10h00 |
| `30 7 * * 1-5` | Segunda a sexta às 07h30 |
| `0 */2 * * *` | A cada 2 horas |
| `0 6,12,18 * * *` | Às 06h, 12h e 18h todos os dias |
| `0 8 * * 0` | Domingos às 08h |

### Timezone

As expressões cron são avaliadas no **timezone configurado em `scheduler.timezone`**:

```yaml
scheduler:
  timezone: "America/Sao_Paulo"   # BRT / BRST automático
```

Se `timezone` estiver vazio, o engine usa o timezone do sistema operacional.

Exemplos de timezones válidos: `"America/Sao_Paulo"`, `"America/Manaus"`, `"America/Belem"`, `"America/Fortaleza"`, `"UTC"`.

Uma entrada cron **nunca é auto-desabilitada** — dispara indefinidamente enquanto `enabled: true`.

---

## Modo one-shot: `fire_at`

Define uma **data e hora exatas** para disparo único. Após disparar, a entrada é **auto-desabilitada** (`enabled: false`) e não dispara novamente.

### Formato

O valor deve ser uma string **RFC 3339** (subconjunto do ISO 8601):

```
2026-07-06T16:31:00-03:00
│          │        │
│          │        └─ offset de timezone: -03:00 (Brasília)
│          └─ horário local: 16h31min
└─ data: 6 de julho de 2026
```

**Formatos aceitos:**

| Formato | Exemplo | Observação |
|---|---|---|
| Com offset explícito | `2026-07-06T16:31:00-03:00` | Recomendado — sem ambiguidade |
| UTC (sufixo Z) | `2026-07-06T19:31:00Z` | Equivalente ao exemplo acima |
| Sem timezone | `2026-07-06T16:31:00` | Interpreta no `scheduler.timezone` configurado |

> **Recomendação:** sempre inclua o offset de timezone no `fire_at` para evitar ambiguidade entre horário de verão e horário padrão.

### Limiar de atraso (`missed_threshold_ms`)

Se o engine estava parado quando o `fire_at` passou e reiniciar com atraso:

- **Atraso < `missed_threshold_ms`** → dispara normalmente
- **Atraso ≥ `missed_threshold_ms`** → marca como `MISSED`, **não dispara**

Isso evita que, ao reiniciar o engine horas depois de uma queda, itens antigos sejam disparados fora de contexto.

```yaml
scheduler:
  missed_threshold_ms: 5000   # 5 segundos (padrão)
```

Com `missed_threshold_ms: 0`, o engine sempre tenta disparar mesmo com qualquer atraso.

### Diferença entre `cron_expr` e `fire_at`

| | `cron_expr` | `fire_at` |
|---|---|---|
| Recorrência | Sim — repete no próximo horário | Não — dispara uma vez |
| Auto-desabilita | Não | Sim, após disparar |
| Timezone | Via `scheduler.timezone` | Embutido no valor (offset) ou via `scheduler.timezone` |
| `next_fire_at` na API | Calculado automaticamente | Retorna o próprio `fire_at` até disparar |

---

## Modos de disparo (`trigger_mode`)

Define como o item agendado interage com o que está tocando no momento do disparo:

| Modo | Estado do engine | Comportamento |
|---|---|---|
| `INTERRUPT` | PLAYING / PAUSED | Para o item atual imediatamente e inicia o agendado |
| `INTERRUPT` | IDLE | Inicia o item diretamente |
| `AFTER_CURRENT` | PLAYING / PAUSED | Insere como próximo da fila; aguarda o item atual terminar |
| `AFTER_CURRENT` | IDLE | Inicia o item diretamente |
| `CROSSFADE` | PLAYING | Inicia com crossfade sobre o final do item atual |
| `CROSSFADE` | IDLE | Inicia o item diretamente |
| `SKIP_IF_BUSY` | PLAYING / PAUSED | **Não dispara** — marca como MISSED |
| `SKIP_IF_BUSY` | IDLE | Inicia o item diretamente |

**Estado PANIC:** em qualquer `trigger_mode`, se o engine estiver em modo `PANIC`, o disparo é sempre marcado como **MISSED**. O scheduler nunca interfere com a cama de emergência.

---

## Persistência

A grade é salva em disco automaticamente a cada mutação (add, update, remove, enable, disable). O arquivo é um JSON com lista de entradas:

```yaml
scheduler:
  store_path: ""   # padrão: ~/RadioFlow/schedule.json
```

**Escrita atômica:** o engine grava em um arquivo `.tmp` e então renomeia, evitando corrupção em caso de crash.

**Restore on start:** ao iniciar, o engine restaura todas as entradas do arquivo. Entradas cron são re-registradas normalmente. Entradas `fire_at` passam pela verificação de `missed_threshold_ms`.

---

## Backpressure e prioridade de eventos

| Evento | Prioridade | Pode ser descartado? |
|---|---|---|
| `ScheduleEntryFired` | Baixa | Sim, sob carga extrema |
| `ScheduleEntryMissed` | Baixa | Sim, sob carga extrema |
| `ScheduleEntryAdded` | Normal | Não — evento discreto, raramente publicado |
| `ScheduleEntryRemoved` | Normal | Não — evento discreto, raramente publicado |
| `ScheduleEntryUpdated` | Normal | Não — evento discreto, raramente publicado |

---

## Configuração completa

```yaml
scheduler:
  # Habilita ou desabilita o scheduler por completo.
  enabled: true

  # Timezone para avaliação das expressões cron.
  # Deixe vazio para usar o timezone do sistema operacional.
  # Exemplos: "America/Sao_Paulo", "America/Manaus", "UTC"
  timezone: "America/Sao_Paulo"

  # Caminho do arquivo de persistência.
  # Vazio = ~/RadioFlow/schedule.json
  store_path: ""

  # Tolerância de atraso para entradas fire_at.
  # Se o horário de disparo passou há mais que este valor (ms),
  # a entrada é marcada como MISSED em vez de disparar.
  # Use 0 para sempre disparar independentemente do atraso.
  missed_threshold_ms: 5000
```

---

## Exemplos de uso

### Noticiário diário às 10h (cron recorrente)

```json
POST /v1/schedule
{
  "name": "Noticiário das 10h",
  "enabled": true,
  "cron_expr": "0 10 * * *",
  "trigger_mode": "CROSSFADE",
  "item": {
    "path": "/library/spots/noticiao-10h.mp3",
    "type": "spots",
    "title": "Noticiário das 10h",
    "duration_ms": 180000
  }
}
```

### Jingle especial em horário exato (one-shot)

```json
POST /v1/schedule
{
  "name": "Jingle especial 6 jul",
  "enabled": true,
  "fire_at": "2026-07-06T16:31:00-03:00",
  "trigger_mode": "AFTER_CURRENT",
  "item": {
    "path": "/library/jingles/especial.mp3",
    "type": "jingles",
    "title": "Jingle Especial",
    "duration_ms": 30000
  }
}
```

### Hora Certa a cada hora

```json
POST /v1/schedule
{
  "name": "Hora Certa",
  "enabled": true,
  "cron_expr": "0 * * * *",
  "trigger_mode": "INTERRUPT",
  "item": {
    "type": "HORA_CERTA",
    "title": "Hora Certa"
  }
}
```

---

## Eventos WebSocket relacionados

Ver `04-events-websocket.md` — seção "Eventos de Scheduler".

## Endpoints REST relacionados

Ver `03-api-rest.md` — seção "Schedule".
