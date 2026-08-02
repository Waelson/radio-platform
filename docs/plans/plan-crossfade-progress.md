# Plano: Correção do Progresso Durante Crossfade

**Status:** proposta
**Módulos impactados:** `playout/internal/audio/output/coreaudio/`, `playout/internal/audio/mixer/`, `playout/internal/playback/`
**Branch sugerida:** `fix/crossfade-progress`

---

## 1. O Problema

### 1.1 Sintoma

Durante a transição entre dois áudios (crossfade), a barra de progresso e as informações do "Now Playing" passam a exibir dados da **Música B** (próxima) enquanto o hardware ainda está reproduzindo majoritariamente a **Música A** (atual). O operador vê a faixa B com progresso avançando desde o início enquanto ouve o fade final da faixa A.

### 1.2 Causa Raiz

O problema tem dois componentes interligados:

#### Componente 1 — `framesTotal` não é posição de reprodução

O contador `framesTotal` em `playback/manager.go` rastreia quantos frames foram **escritos no ring buffer** pelo goroutine Go, não quantos frames o hardware CoreAudio já reproduziu. A diferença entre as duas posições é a **ocupação atual do ring buffer**.

O ring buffer do CoreAudio tem capacidade de `2^18 = 262.144 samples` a 48kHz estéreo (1 sample = 2 floats). Isso equivale a:

```
262.144 samples / 48.000 Hz = 2,73 s de áudio pré-carregado
```

#### Componente 2 — FFmpeg decodifica 50-100x mais rápido que o tempo real

O decoder FFmpeg grava frames no ring buffer muito mais rápido do que o hardware os consome. Na prática, o ring buffer fica **quase sempre cheio** (≈ 2,73 s de conteúdo pendente). O hardware fica permanentemente ~2,73 s atrás da posição de escrita Go.

#### Como os dois componentes interagem no crossfade

Para um crossfade de 3.000 ms:

1. `runPlayLoop` detecta `posMS >= xStartMS` → inicia a escrita das frames de crossfade.
2. FFmpeg decodifica os 3.000 ms de crossfade **quase instantaneamente** e preenche o ring.
3. Em poucos milissegundos de tempo real, `xFrames >= xTotal` → Go terminou de **escrever** o crossfade.
4. `sessionLoop` chama `startItem(MusicB, framesTotal - xFramesDone)` imediatamente.
5. Neste instante, o hardware está apenas ≈ 270 ms adentro do crossfade (3.000 ms escritos − 2.730 ms no buffer = 270 ms reproduzidos).
6. Música A ainda está tocando com ≈ 91% de volume (fade quase no início).
7. A UI já mostra progresso da Música B desde o segundo 0 enquanto o operador ouve predominantemente a Música A.

```
Escrita Go:   [-------- Música A --------][=== Crossfade (3s) ===][ Música B ... ]
                                                                  ^
                                                            startItem() dispara aqui
                                                            (xFrames >= xTotal)

Hardware:     [-------- Música A --------][== Xf: só 270ms ===]
                                                              ^
                                                     posição real do hardware
                                                     (hardware 2,73s atrás)
```

### 1.3 Arquivos e linhas relevantes

| Arquivo | Linha | Papel |
|---|---|---|
| `playout/internal/playback/manager.go` | `runPlayLoop` → `xFrames >= xTotal` | Condição que dispara o início de Music B |
| `playout/internal/playback/manager.go` | `sessionLoop` → `startItem(nextItem, ...)` | Chama `startItem` baseado em posição de escrita |
| `playout/internal/playback/manager.go` | `progressLoop` → `playedFrames = framesTotal - startFrame` | Calcula progresso com posição de escrita, não de reprodução |
| `playout/internal/audio/output/coreaudio/ring.c` | `caRingWrite`, `caRingDrain` | Ring buffer SPSC em C |
| `playout/internal/audio/output/coreaudio/coreaudio.go` | `Write()` | Incrementa `framesTotal` após gravar no ring |
| `playout/internal/audio/mixer/mixer.go` | `FlushAudio()` | Delega para o device subjacente |

---

## 2. Solução Proposta

### 2.1 Estratégia: expor ocupação do ring buffer e corrigir as posições

A correção mais cirúrgica e de menor risco é **expor a ocupação atual do ring buffer** e subtrair esse valor nos dois pontos onde `framesTotal` é interpretado como posição de reprodução:

1. **`progressLoop`** — para exibir progresso correto.
2. **`startItem` em `sessionLoop`** — para que o `startFrame` de Música B seja baseado em quando o hardware vai começar a tocá-la, não quando Go terminou de escrever.

Não alteramos a velocidade de decodificação nem o tamanho do ring buffer — o comportamento de áudio permanece idêntico.

### 2.2 Alternativas consideradas e descartadas

| Alternativa | Motivo da descarte |
|---|---|
| **Rate-limit o decoder** (escrever no ritmo do hardware) | Risco de underrun em picos de carga de CPU; requer mecanismo de controle de fluxo complexo; muda o comportamento de toda a pipeline. |
| **Usar AudioQueue timestamp** (`AudioQueueGetCurrentTime`) | API sujeita a drift e jitter de hardware; complexidade de integração C↔Go; ganho marginal vs. ring occupancy. |
| **Aumentar tamanho do ring para esconder o problema** | Piora o delay de Stop/Skip; não resolve a causa raiz. |

---

## 3. Implementação Detalhada

### Fase 1 — Expor ocupação do ring buffer na camada C

**Arquivo:** `playout/internal/audio/output/coreaudio/ring.c`

Adicionar função:

```c
// caRingOccupancy returns the number of float samples currently in the ring.
size_t caRingOccupancy(SPSCRing *r) {
    size_t w = atomic_load_explicit(&r->write_pos, memory_order_acquire);
    size_t rd = atomic_load_explicit(&r->read_pos, memory_order_acquire);
    return (w - rd) & (r->cap - 1);
}
```

**Arquivo:** `playout/internal/audio/output/coreaudio/ring.h`

Declarar:

```c
size_t caRingOccupancy(SPSCRing *r);
```

### Fase 2 — Expor no adapter CoreAudio Go

**Arquivo:** `playout/internal/audio/output/coreaudio/coreaudio.go`

Adicionar método:

```go
// RingOccupancy returns the number of float32 samples currently buffered
// in the hardware ring — i.e., written by Go but not yet played by CoreAudio.
// Dividing by (sampleRate * channels) gives the lag in seconds.
func (d *Device) RingOccupancy() int64 {
    if d.ring == nil {
        return 0
    }
    return int64(C.caRingOccupancy(d.ring))
}
```

### Fase 3 — Expor no Mixer

**Arquivo:** `playout/internal/audio/mixer/mixer.go`

Adicionar método de delegação (mesmo padrão de `FlushAudio`/`PauseAudio`):

```go
// RingOccupancy returns the number of float32 samples buffered in the
// underlying hardware ring (written but not yet played).
// Returns 0 if the device does not support this query.
func (m *Mixer) RingOccupancy() int64 {
    type occupancyReader interface{ RingOccupancy() int64 }
    if r, ok := m.out.(occupancyReader); ok {
        return r.RingOccupancy()
    }
    return 0
}
```

### Fase 4 — Corrigir `progressLoop` no playback manager

**Arquivo:** `playout/internal/playback/manager.go`

Adicionar interface local:

```go
// outputOccupancyReader queries how many float32 samples are buffered
// in the hardware ring (written by Go, not yet played by CoreAudio).
type outputOccupancyReader interface {
    RingOccupancy() int64
}
```

Adicionar função helper:

```go
// ringOccupancy returns the current ring buffer occupancy in frames (not samples).
// Returns 0 if the output device does not support the query.
func (m *Manager) ringOccupancy() int64 {
    if r, ok := m.out.(outputOccupancyReader); ok {
        // Ring returns float32 samples; divide by channels to get frames.
        return r.RingOccupancy() / int64(m.outputChannels())
    }
    return 0
}
```

Corrigir `progressLoop` — substituir:

```go
// ANTES
playedFrames := m.framesTotal.Load() - startFrame
```

por:

```go
// DEPOIS
playedFrames := m.framesTotal.Load() - startFrame - m.ringOccupancy()
if playedFrames < 0 {
    playedFrames = 0
}
```

### Fase 5 — Corrigir `startItem` no crossfade

**Arquivo:** `playout/internal/playback/manager.go`

No ponto onde `xFrames >= xTotal` em `runPlayLoop`/`sessionLoop`, o `startFrame` passado para `startItem(Music B)` deve refletir quando o hardware **vai começar** a reproduzir Music B, não quando Go terminou de escrever.

Localizar a chamada (aprox.):

```go
// ANTES — startFrame baseado em posição de escrita Go
m.startItem(nextItem, m.framesTotal.Load()-xFramesDone)
```

Substituir por:

```go
// DEPOIS — startFrame ajustado pela ocupação do ring no momento do disparo
ringAtXfadeEnd := m.ringOccupancy()
m.startItem(nextItem, m.framesTotal.Load()-xFramesDone+ringAtXfadeEnd)
```

**Por que somar `ringAtXfadeEnd`?**

`startFrame` é a posição de escrita Go quando Music B começa a ser escrita. O hardware só vai tocar o frame `startFrame` daqui a `ringAtXfadeEnd` frames. Logo, o `startFrame` do ponto de vista do hardware é `startFrame + ringAtXfadeEnd`. O `progressLoop` calcula `framesTotal - startFrame - ringOccupancy()`; com a correção em `startFrame`, o progresso será 0 no momento correto (quando o hardware de fato inicia Music B).

---

## 4. Verificação e Testes

### 4.1 Teste manual

1. Enfileirar duas músicas (A e B).
2. Configurar crossfade de 3.000 ms.
3. Observar o "Now Playing":
   - **Antes da correção:** progresso de B avança enquanto áudio de A ainda domina.
   - **Após a correção:** "Now Playing" permanece em A até que o hardware execute a transição; só então muda para B.

### 4.2 Teste com crossfade longo (10.000 ms)

Crossfade mais longo exacerba o bug (fica mais visível). Usar para validar que a correção escala corretamente com o tamanho do xfade.

### 4.3 Testes automatizados

```bash
cd playout && go test ./...
cd playout && go vet ./...
```

Nenhum teste automatizado existente cobre timing de hardware (depende de CoreAudio). A correção pode ser validada por:
- Teste unitário em `mixer_test.go` verificando que `RingOccupancy()` retorna 0 quando o device não suporta (NullOutput).
- Teste de `progressLoop` com mock de `outputOccupancyReader` retornando valor fixo, verificando que `playedFrames` é subtraído corretamente.

---

## 5. Impacto e Riscos

| Aspecto | Avaliação |
|---|---|
| Impacto em áudio | Nenhum — não altera pipeline de escrita/leitura |
| Impacto em API/WebSocket | Apenas progresso e metadados do Now Playing ficam mais precisos |
| Risco de regressão | Baixo — mudanças isoladas no cálculo de `playedFrames` e `startFrame` |
| Compatibilidade | `RingOccupancy()` retorna 0 em NullOutput e qualquer device sem suporte — comportamento degradado graciosamente |
| Performance | `caRingOccupancy()` é uma leitura atômica — custo negligenciável no hot path |

---

## 6. Fases de Entrega

| Fase | O que faz | Arquivos |
|---|---|---|
| 1 | `caRingOccupancy()` em C | `ring.c`, `ring.h` |
| 2 | `RingOccupancy()` no adapter CoreAudio | `coreaudio.go` |
| 3 | `RingOccupancy()` no Mixer | `mixer.go` |
| 4 | Correção do `progressLoop` | `playback/manager.go` |
| 5 | Correção do `startItem` no crossfade | `playback/manager.go` |

Todas as fases são independentes e podem ser revisadas separadamente, mas devem ser entregues juntas (a correção só é completa com as cinco fases).

---

## 7. Pergunta

Deseja que eu prossiga com a implementação deste plano?
