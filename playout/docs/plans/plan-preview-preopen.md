# Plano: Pre-open do dispositivo de preview para eliminar break no áudio principal

## Contexto

Ao iniciar um preview de áudio, o áudio principal sofre um leve corte. A investigação
identificou que o padrão atual de `Open()` / `Close()` por sessão de playback é a causa
raiz, especialmente crítico quando o dispositivo de preview é Bluetooth (A2DP).

Configuração atual do ambiente que revelou o problema:
- Áudio principal: `BuiltInSpeakerDevice` (CoreAudio)
- Preview: `EC-46-54-10-A5-4C:output` (dispositivo Bluetooth)

### Fluxo atual (problemático)

```
[play preview]
  → startSession()
      → probeDuration()          ← ffprobe síncrono, bloqueia event loop
      → go p.loop()
          → p.out.Open()         ← BT acorda, HAL do CoreAudio reconfigurado
          → p.out.Start()        ← streaming começa
          → ... áudio toca ...
          → p.out.Stop()         ← defer
          → p.out.Close()        ← BT dorme, HAL notificado novamente
```

Cada play de preview dispara um ciclo completo de wake/sleep do BT, causando
notificações HAL que interrompem brevemente o AudioQueue principal.

### Fluxo alvo (correto)

```
[engine start]
  → preview.Player.Run()
      → p.out.Open()             ← abre UMA VEZ, dispositivo fica warm
      → p.out.Start()            ← inicia streaming silencioso

[play preview]
  → startSession()
      → go p.loop()
          → p.out.Write()        ← envia frames (device já aberto e quente)
          → p.out.Stop()         ← para o streaming, device continua aberto

[engine stop]
  → p.out.Close()                ← fecha UMA VEZ
```

---

## Bug colateral identificado durante investigação

O campo `cfg.Preview.OutputDevice` (`EC-46-54-10-A5-4C:output`) **não está sendo
passado** para o `OutputConfig` dentro de `p.loop()`:

```go
// preview/player.go:250 — DeviceID sempre vazio
outCfg := output.OutputConfig{
    DeviceID:     "",   // ← deveria ser cfg.Preview.OutputDevice
    SampleRate:   p.sampleRate,
    ...
}
```

O preview está usando o dispositivo padrão do sistema, não o configurado. Isso será
corrigido na Fase 1 como pré-requisito.

---

## Fases de implementação

---

### Fase 1 — Corrigir DeviceID ausente no preview (pré-requisito)

**Objetivo:** Garantir que `cfg.Preview.OutputDevice` seja de fato usado ao abrir o
dispositivo de preview.

**Arquivos impactados:**
- `internal/preview/player.go`

**O que mudar:**

Adicionar campo `DeviceID string` em `AudioConfig`:

```go
type AudioConfig struct {
    DeviceID     string  // ← novo
    SampleRate   int
    Channels     int
    BufferFrames int
}
```

Armazenar em `Player`:

```go
type Player struct {
    ...
    deviceID   string  // ← novo
}
```

Usar no `outCfg` dentro de `loop()`:

```go
outCfg := output.OutputConfig{
    DeviceID:     p.deviceID,  // ← antes era ""
    SampleRate:   p.sampleRate,
    ...
}
```

Atualizar `main.go` para passar `cfg.Preview.OutputDevice`:

```go
prevPlayer := preview.New(evtBus, dec, previewOut, preview.AudioConfig{
    DeviceID:     cfg.Preview.OutputDevice,  // ← novo
    SampleRate:   cfg.Audio.SampleRate,
    ...
}, log)
```

**Risco:** Baixo. Mudança aditiva. Se `DeviceID` for vazio, `coreaudio.Open()` usa o
default do sistema — comportamento idêntico ao atual.

**Impacto:** Nenhum no áudio principal. Corrige silenciosamente um bug de configuração
ignorada.

---

### Fase 2 — Pre-open do dispositivo no `Run()` (fix principal)

**Objetivo:** Abrir o dispositivo de preview uma única vez ao iniciar o engine e
mantê-lo aberto durante toda a vida do processo.

**Arquivos impactados:**
- `internal/preview/player.go`

**O que mudar:**

Em `Run()`, antes do event loop, abrir e iniciar o dispositivo:

```go
func (p *Player) Run(ctx context.Context) {
    // Abre o dispositivo uma única vez.
    outCfg := output.OutputConfig{
        DeviceID:     p.deviceID,
        SampleRate:   p.sampleRate,
        Channels:     p.channels,
        BufferFrames: p.bufFrames,
    }
    if err := p.out.Open(ctx, outCfg); err != nil {
        p.log.Warn("preview: output device unavailable, preview disabled", "error", err)
        // Substitui por NullOutput para degradar graciosamente.
        p.out = &output.NullOutput{}
        _ = p.out.Open(ctx, outCfg)
    }
    if err := p.out.Start(ctx); err != nil {
        p.log.Warn("preview: output start failed", "error", err)
    }
    defer p.out.Stop(context.Background())
    defer p.out.Close()

    // Event loop continua igual...
}
```

Em `loop()`, remover `Open/Close`, manter apenas `Start/Stop`:

```go
func (p *Player) loop(ctx context.Context, ...) {
    // ... decoder open ...

    // REMOVIDO: p.out.Open() / defer p.out.Close()

    if err := p.out.Start(ctx); err != nil { ... }
    defer p.out.Stop(context.Background())

    // ... write loop ...
}
```

**Risco:** Médio. Ver seção de riscos detalhada abaixo.

**Impacto:** Elimina o ciclo Open/Close por sessão. O dispositivo BT fica em
streaming contínuo enquanto o engine estiver rodando.

---

### Fase 3 — Mover `probeDuration()` para background (melhoria complementar)

**Objetivo:** Remover o bloqueio síncrono de `ffprobe` do event loop do preview,
reduzindo a latência até o primeiro frame de áudio.

**Arquivos impactados:**
- `internal/preview/player.go`

**O que mudar:**

`probeDuration()` atualmente é chamado de forma **síncrona** em `startSession()`
antes de lançar `go p.loop()`. Isso bloqueia o event loop por 50–300 ms enquanto
o `ffprobe` roda.

Mover a chamada para dentro de `p.loop()` (já rodando em goroutine), logo após o
decoder ser aberto — o decoder já conhece o formato do arquivo e pode fornecer
duração sem um processo separado:

```go
// Dentro de loop(), após p.dec.Open():
durMS := probeDuration(path)   // ← move para cá, já é async
send(intMsg{kind: intDuration, durMS: durMS})
```

Alternativamente, chamar `ffprobe` apenas se o decoder não retornar duração, ou
eliminar `probeDuration()` completamente e obter a duração do próprio `ffprobe`
stream já em execução.

**Risco:** Baixo. A UI de preview já lida com `DurationMS: 0` (exibe barra
indeterminada); a duração atualiza quando chega.

**Impacto:** O event loop do preview não bloqueia mais na inicialização de uma sessão.
Latência perceptível até o primeiro frame reduz de ~200–500 ms para ~10–30 ms.

---

## Riscos detalhados

### R1 — Dispositivo BT mantido em streaming contínuo

**Descrição:** Com o pre-open, o dispositivo BT ficará em A2DP ativo enquanto o
engine estiver rodando, mesmo quando nenhum preview estiver tocando (enviando frames
de silêncio ou mantendo o AudioQueue parado).

**Consequências possíveis:**
- Consumo de bateria levemente maior no fone/caixa BT
- O dispositivo aparece "ocupado" no macOS, podendo conflitar com outros apps de áudio
- Se o dispositivo BT desconectar enquanto o engine roda, o `Write()` passará a falhar

**Mitigação:**
- Implementar reconexão automática: se `Write()` ou `Start()` falhar, tentar `Close()`
  + `Open()` novamente (com backoff) sem afetar o áudio principal
- Documentar o comportamento no README

---

### R2 — CoreAudio: `Stop()` sem `Close()` pode não liberar buffers corretamente

**Descrição:** O padrão atual usa `AudioQueueStop(immediate=true)` + `AudioQueueDispose`.
No novo padrão, entre sessões de preview apenas `AudioQueueStop` é chamado. O AudioQueue
fica em estado "stopped" mas os buffers C continuam alocados.

**Consequências possíveis:**
- Nenhuma, pois `AudioQueueStop` é um estado válido e reversível com `AudioQueueStart`
- Porém, se o buffer acumulador (`accum`) tiver dados parciais de uma sessão anterior,
  podem vazar para o início da próxima sessão (pop/click audível)

**Mitigação:**
- Garantir que `p.out.Stop()` chame `o.accumN = 0` (já feito em `RestartAudio()`,
  verificar se `Stop()` também faz isso)
- Enviar um buffer de silêncio curto antes de `Stop()` para drenar o acumulador

---

### R3 — Falha de Open na inicialização degrada preview silenciosamente

**Descrição:** Se o dispositivo configurado não estiver disponível quando o engine
inicia (BT não pareado, USB desconectado), o `Open()` falhará antes do event loop
começar.

**Consequências possíveis:**
- Preview totalmente inativo, sem feedback claro para o operador
- Se não houver fallback, qualquer comando de preview pode causar pânico (nil write)

**Mitigação:**
- Fallback para `NullOutput` se `Open()` falhar no `Run()` (ver Fase 2)
- Publicar evento de erro no EventBus para que a UI mostre o estado de preview inativo
- Implementar retry periódico de Open (ex: a cada 30s tentar reabrir o dispositivo)

---

### R4 — `NullOutput` comportamento diferente no novo padrão

**Descrição:** `NullOutput.Open()` não é idempotente — verificar se chamá-lo
uma vez no `Run()` e depois `Start()` / `Stop()` repetidamente em `loop()` é seguro.

**Consequências possíveis:** Nenhuma — `NullOutput.Start()` e `Stop()` são no-ops.
O comportamento em testes não muda.

**Mitigação:** Nenhuma necessária — verificado como seguro.

---

### R5 — Mudança no ciclo de vida quebra testes existentes

**Descrição:** Testes do `preview.Player` podem instanciar o player e chamar
`HandlePlay` sem passar pelo `Run()`, ou usar mocks de `OutputDevice` que esperam
`Open()` ser chamado no play.

**Consequências possíveis:** Testes falham com "output not open".

**Mitigação:**
- Revisar todos os testes do pacote `preview` antes da implementação
- Atualizar testes para chamar `Run()` (com ctx cancelado imediatamente) antes de
  simular comandos, ou usar um `NullOutput` que aceita `Write()` sem ter sido aberto

---

## Resumo das fases

| Fase | O que muda | Risco | Impacto no áudio principal |
|---|---|---|---|
| 1 | `DeviceID` passado corretamente ao `Open()` | Baixo | Nenhum |
| 2 | `Open()`/`Close()` movidos para `Run()` | Médio | Elimina o break |
| 3 | `probeDuration()` movido para background | Baixo | Nenhum (melhoria de latência do preview) |

As fases são independentes mas devem ser implementadas nesta ordem: a Fase 1
garante que o dispositivo correto seja usado antes de mantê-lo aberto
permanentemente na Fase 2.
