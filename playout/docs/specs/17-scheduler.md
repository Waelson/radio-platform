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
| `trigger_mode` | Como o item/break entra na reprodução |
| `item` | Item de playback (path, tipo, título, duração etc.) — mutuamente exclusivo com `break` |
| `break` | Bloco comercial (open + spots + close) — mutuamente exclusivo com `item` |
| `created_at` | Quando a entrada foi criada |
| `last_fired_at` | Último disparo bem-sucedido |
| `next_fire_at` | Próximo disparo calculado (somente para entradas cron) |

> **Regra de exclusividade de conteúdo:** exatamente um de `item` ou `break` deve ser informado — nunca os dois ao mesmo tempo e nunca nenhum.

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

> `path` **deve ser vazio** — o engine resolve os arquivos de hora e minuto automaticamente no momento do disparo usando `hora_certa.hours_dir` e `hora_certa.minutes_dir`. Informar um path neste tipo de item é ignorado.

---

## Bloco Comercial

O scheduler suporta o agendamento de **blocos comerciais completos** — sequências de Open → Spots → Close que são inseridas na frente da fila de playback como uma unidade atômica.

### Estrutura do `break`

```json
{
  "break": {
    "title": "Bloco das 10h30",
    "open": {
      "path": "/library/jingles/break-open.mp3",
      "type": "jingles",
      "title": "Abertura do bloco",
      "duration_ms": 5000
    },
    "spots": [
      {
        "path": "/library/spots/anunciante-a.mp3",
        "type": "spots",
        "title": "Anunciante A",
        "duration_ms": 30000
      },
      {
        "path": "/library/spots/anunciante-b.mp3",
        "type": "spots",
        "title": "Anunciante B",
        "duration_ms": 30000
      }
    ],
    "close": {
      "path": "/library/jingles/break-close.mp3",
      "type": "jingles",
      "title": "Encerramento do bloco",
      "duration_ms": 5000
    }
  }
}
```

| Campo | Obrigatório | Descrição |
|---|---|---|
| `break.title` | não | Nome do bloco (aparece em logs e eventos) |
| `break.open` | não | Item de abertura; recebe `transition: CROSSFADE` automático |
| `break.spots` | **sim** (≥ 1) | Lista de spots; cada spot precisa de `path` |
| `break.close` | não | Item de encerramento |

### Regras

- `break` e `item` são **mutuamente exclusivos** — exatamente um deve ser informado
- `break.spots` deve ter ao menos **1 item**; cada spot requer o campo `path`
- Ao disparar, o engine envia `CmdInsertBreakNext` — o bloco é expandido e inserido **na frente da fila pendente** (não no final)
- O `break` compartilha os mesmos `trigger_mode` disponíveis para itens simples

### Exemplo completo — Bloco diário às 10h30

```json
POST /v1/schedule
{
  "name": "Bloco Comercial 10h30",
  "enabled": true,
  "cron_expr": "30 10 * * *",
  "trigger_mode": "AFTER_CURRENT",
  "break": {
    "title": "Bloco das 10h30",
    "open": {
      "path": "/library/jingles/break-open.mp3",
      "type": "jingles",
      "title": "Abertura"
    },
    "spots": [
      { "path": "/library/spots/anunciante-a.mp3", "type": "spots", "title": "Anunciante A", "duration_ms": 30000 },
      { "path": "/library/spots/anunciante-b.mp3", "type": "spots", "title": "Anunciante B", "duration_ms": 30000 }
    ],
    "close": {
      "path": "/library/jingles/break-close.mp3",
      "type": "jingles",
      "title": "Encerramento"
    }
  }
}
```

### Evento WebSocket ao disparar

Quando um bloco comercial dispara, o evento `ScheduleEntryFired` inclui campos específicos para break (ao invés de `asset_id` e `title`):

```json
{
  "type": "ScheduleEntryFired",
  "payload": {
    "entry_id": "sched_01JZ...",
    "entry_name": "Bloco Comercial 10h30",
    "trigger_mode": "AFTER_CURRENT",
    "break_title": "Bloco das 10h30",
    "spot_count": 2,
    "one_shot": false
  }
}
```

---

## Hora Certa

O tipo `HORA_CERTA` tem suporte nativo no scheduler. Quando uma entrada desse tipo dispara, o scheduler envia um `CmdInsertNext` com o item para a fila de playback. O playback manager detecta o tipo `HORA_CERTA` e resolve os arquivos de áudio correspondentes à hora e minuto atuais no momento em que o item é efetivamente tocado.

### Regras

- O campo `item.path` deve estar **vazio** — informar um path é um erro semântico (o engine ignora o path e resolve pela hora atual)
- O campo `item.type` deve ser exatamente `"HORA_CERTA"` (sensível a maiúsculas)
- A expressão cron `"0 * * * *"` dispara no início de cada hora — é a configuração recomendada
- O `trigger_mode` recomendado é `INTERRUPT`, para que o anúncio de hora certa interrompa imediatamente o que estiver tocando

### Fluxo de execução

```
cron "0 * * * *" dispara
  → scheduler envia CmdInsertNext {type: "HORA_CERTA"}
  → dispatcher insere item na frente da fila
  → playback manager inicia o item
  → openHoraCerta() resolve hora e minuto atuais
  → toca HRS{HH}.mp3 + MIN{MM}.mp3 em sequência
  → se MM=00 e MIN00.mp3 não existir, toca apenas HRS{HH}.mp3
```

### Pré-requisitos

O bloco `hora_certa` deve estar configurado no YAML:

```yaml
hora_certa:
  hours_dir:   "/library/horacerta/horas"
  minutes_dir: "/library/horacerta/minutos"
```

Os arquivos devem seguir a convenção de nomes `HRS{HH}.mp3` (ex: `HRS10.mp3`) e `MIN{MM}.mp3` (ex: `MIN30.mp3`).

---

## Eventos WebSocket relacionados

Ver `04-events-websocket.md` — seção "Eventos de Scheduler".

## Endpoints REST relacionados

Ver `03-api-rest.md` — seção "Schedule".
