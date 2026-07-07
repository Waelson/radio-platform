# Plano: UI de Configuração — SPA via Systray

## Contexto

O systray já possui a opção "Status" que abre `GET /status` — uma SPA com HTML/CSS/JS
inline num `[]byte` dentro de um handler Go, sem dependências externas e sem build step.

A opção "Configuração" seguirá o mesmo padrão: `GET /config` serve a SPA; botões de
ação chamam endpoints REST existentes ou novos. Ao salvar, o usuário é orientado a
reiniciar o serviço.

---

## Decisões de design

| Ponto | Decisão |
|---|---|
| Botão "Reiniciar agora" | Removido — usuário é instruído a reiniciar pelo systray |
| File picker | Botão "Procurar" chama `POST /v1/config/browse` — Go abre dialog nativo |
| Dropdown de dispositivos | `<select>` populado via `GET /v1/devices` |
| Dispositivos player vs preview | Dois dropdowns **independentes** — ambos de `GET /v1/devices` |
| Sidebar | Agrupado: Logging + Segurança + Admin em um único painel (~11 itens) |
| sample_rate / channels / buffer_frames | Editáveis com aviso `⚠ Requer reinicialização` |
| Monitoramento da engine | SPA faz polling de `GET /v1/health` a cada 5s em background |
| Driver `coreaudio` | Exibido apenas quando o SO for macOS — detectado via `GET /v1/info` |
| Botão Salvar | Desabilitado quando engine está offline; reabilitado ao reconectar |
| Snapshot de config | `PUT /v1/config` salva backup do YAML anterior em `<config>.bak` (apenas o último) |

### Nota: restart após salvar

A SPA não controla o ciclo de vida do engine. Após salvar, o usuário é
instruído a reiniciar o serviço manualmente pelo menu do systray.
Isso mantém a UI simples e evita que a SPA precise monitorar o processo.

### Nota: monitoramento em background

A SPA mantém um ticker de 5s chamando `GET /v1/health`. Enquanto a engine
responde, o formulário permanece editável. Quando a engine para ou é reiniciada:

- Banner de alerta é exibido no topo da página
- Botão "Salvar" é desabilitado (classe `disabled`, `pointer-events: none`)
- Quando `GET /v1/health` voltar a responder com 200, o banner some e o
  botão "Salvar" é reabilitado automaticamente

Isso garante que o operador não consiga salvar uma config enquanto a engine
está sendo reiniciada e o arquivo YAML pode estar sendo lido/escrito.

---

## Padrão técnico obrigatório (herdado da tela de Status)

| Aspecto | Regra |
|---|---|
| HTML/CSS/JS | Inline num `[]byte` em `handlers/config_html.go` |
| Dependências externas | Nenhuma — zero npm, zero CDN |
| Tema | Dark `#070807`, acento verde `#00ff80`, fonte Inter |
| Dados | Buscados via `fetch()` em endpoints REST novos |
| Handler Go | `ConfigHTML(...)` registrado em `GET /config` |

---

## Arquitetura da funcionalidade

```
Systray → "Configuração" → abre /config no webview

GET /v1/info                 ← detecta o SO (campo "os": "darwin" | "linux" | "windows")
GET /v1/config/current       ← lê o YAML atual, retorna JSON com config completa
GET /v1/devices              ← lista dispositivos para popular os <select>
POST /v1/config/browse       ← Go abre file/folder dialog nativo, retorna path
PUT /v1/config               ← recebe JSON editado, valida, reescreve o YAML
```

---

## Etapas de implementação

### Etapa 1 — HTML standalone para validação visual

**Objetivo:** produzir `config-ui-preview.html` que o usuário abre no browser para
validar layout, navegação e UX **antes** de qualquer código Go ser escrito.

**O que fazer:**
- HTML completo, CSS e JS inline, zero dependências externas
- Todos os 11 painéis navegáveis via sidebar
- Valores preenchidos com os dados do YAML atual (hardcoded — só para preview)
- Botão "Salvar" exibe o banner de restart (sem chamar API real)
- Dropdowns de dispositivo com opções de exemplo hardcoded
- Salvar em: `playout/docs/plans/config-ui-preview.html`

**Critério de aceite:** usuário aprova o HTML antes de continuar para a Etapa 2.

---

### Etapa 2 — Backend: `GET /v1/config/current`

**Arquivo:** `internal/api/handlers/config.go`

Retorna o config atual como JSON para popular o formulário:

```json
GET /v1/config/current
→ 200 { "engine": {...}, "api": {...}, "audio": {...}, ... }
```

- Handler recebe `*config.Config` por valor no registro (snapshot do startup)
- Serializa para JSON com `encoding/json`
- Registrar: `GET /v1/config/current`

---

### Etapa 3 — Backend: `POST /v1/config/browse`

**Arquivo:** `internal/api/handlers/config.go`

Abre um dialog nativo de seleção de arquivo ou pasta e retorna o path:

```
POST /v1/config/browse
Body: { "type": "file" }   ou   { "type": "dir" }
→ 200 { "path": "/Users/waelson/media/panic/bed.mp3" }
→ 200 { "path": "" }   (usuário cancelou)
```

**Implementação:**
- macOS: `osascript -e 'choose file'` via `os/exec` (sem dependência externa)
- Windows: PowerShell `[System.Windows.Forms.OpenFileDialog]` via `os/exec`
- Linux: `zenity --file-selection` via `os/exec` (zenity é padrão em GNOME)
- Handler registrado: `POST /v1/config/browse`

---

### Etapa 4 — Backend: `PUT /v1/config`

**Arquivo:** `internal/api/handlers/config.go`

Recebe o JSON editado, valida, faz backup do arquivo anterior e reescreve o YAML:

```
PUT /v1/config
Body: { "engine": {...}, "api": {...}, ... }
→ 200 { "ok": true }
→ 400 { "ok": false, "error": "invalid_value", "message": "..." }
```

**Sequência de escrita (atômica do ponto de vista do operador):**
1. Decodifica o body como `config.Config`
2. Chama `config.validate()` — retorna 400 se inválido (nada é escrito)
3. Lê o conteúdo atual do YAML (`os.ReadFile`)
4. Sobrescreve `<config-path>.bak` com o conteúdo anterior (`os.WriteFile`)
5. Serializa o novo config para YAML com `gopkg.in/yaml.v3`
6. Escreve o novo YAML em `<config-path>`

**Snapshot:** arquivo `<config-path>.bak` no mesmo diretório do YAML principal.
Apenas o último backup é mantido — cada `PUT /v1/config` sobrescreve o `.bak`.
Exemplo: `playout-engine.yaml` → backup em `playout-engine.yaml.bak`.

**Não aplica a config em runtime** — apenas persiste no arquivo.
O operador reinicia para efetivar as mudanças.

Handler registrado: `PUT /v1/config`

---

### Etapa 5 — Handler Go da SPA: `GET /config`

**Arquivo:** `internal/api/handlers/config_html.go`

Segue exatamente o padrão de `status_html.go`:

```go
func ConfigHTML() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Header().Set("Cache-Control", "no-store")
        _, _ = w.Write(configPage)
    }
}

var configPage = []byte(`<!DOCTYPE html>...`)
```

JS da página:
1. `GET /v1/info` ao carregar → detecta o SO; se `os !== "darwin"`, oculta a opção `coreaudio` nos dois seletores de driver (Áudio e Preview)
2. `GET /v1/config/current` ao carregar → popula o formulário
3. `GET /v1/devices` ao carregar → popula os dois `<select>` de dispositivo
4. `POST /v1/config/browse` ao clicar "Procurar" → preenche o campo de path
5. `PUT /v1/config` ao clicar "Salvar" → exibe banner instruindo restart pelo systray
6. `GET /v1/health` a cada 5s em background → controla estado do botão Salvar e banner de status

---

### Etapa 6 — Registro no servidor

**`internal/api/server.go`:**
```go
mux.HandleFunc("GET /config",              handlers.ConfigHTML())
mux.HandleFunc("GET /v1/config/current",   handlers.GetCurrentConfig(cfg))
mux.HandleFunc("POST /v1/config/browse",   handlers.BrowsePath())
mux.HandleFunc("PUT /v1/config",           handlers.UpdateConfig(configPath))
```

---

### Etapa 7 — Systray: item "Configuração"

**`cmd/playout-engine/systray/`:**

- Adicionar item "Configuração" no menu que abre `http://localhost:<port>/config`
- O restart após salvar é feito pelo operador via systray (opção existente de restart/quit)

---

### Etapa 8 — Testes

**`internal/api/handlers/config_test.go`:**

```
TestGetCurrentConfig_ReturnsJSON
TestUpdateConfig_ValidPayload_RewritesYAML
TestUpdateConfig_ValidPayload_CreatesBackup      ← verifica que .bak foi criado com conteúdo anterior
TestUpdateConfig_ValidPayload_OverwritesBackup   ← segunda chamada sobrescreve o .bak anterior
TestUpdateConfig_InvalidJSON_Returns400_NoBackup ← falha de parse não cria .bak
TestUpdateConfig_InvalidPort_Returns400_NoWrite  ← validação falha → nenhum arquivo escrito
TestUpdateConfig_InvalidDriver_Returns400
TestUpdateConfig_InvalidLogLevel_Returns400
TestBrowsePath_ReturnsEmptyOnCancel
```

---

## Sidebar — estrutura final (11 itens)

```
Engine
API
Áudio
Reprodução
Saúde
Panic
Logging / Segurança / Admin
Fila
Hora Certa
Preview
Scheduler
```

---

## Layout da UI (esboços por seção)

### Estrutura geral — engine online

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│  RadioCore — Configuração                                   ● Online   [×]      │
├──────────────────┬──────────────────────────────────────────────────────────────┤
│                  │                                                              │
│  Engine        ◀ │  [conteúdo da seção ativa]                                  │
│  API             │                                                              │
│  Áudio           │                                                              │
│  Reprodução      │                                                              │
│  Saúde           │                                                              │
│  Panic           │                                                              │
│  Log/Seg/Admin   │                                                              │
│  Fila            │                                                              │
│  Hora Certa      │                                                              │
│  Preview         │                                                              │
│  Scheduler       │                                                              │
│                  │                                                              │
├──────────────────┴──────────────────────────────────────────────────────────────┤
│                                               [Cancelar]  [Salvar]             │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Estrutura geral — engine offline

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│  RadioCore — Configuração                                   ○ Offline  [×]      │
├──────────────────────────────────────────────────────────────────────────────────┤
│  ⚠  Engine offline — pode estar reiniciando. Aguarde para salvar alterações.   │
├──────────────────┬──────────────────────────────────────────────────────────────┤
│                  │                                                              │
│  Engine        ◀ │  [conteúdo da seção ativa — somente leitura]                │
│  API             │                                                              │
│  Áudio           │                                                              │
│  Reprodução      │                                                              │
│  Saúde           │                                                              │
│  Panic           │                                                              │
│  Log/Seg/Admin   │                                                              │
│  Fila            │                                                              │
│  Hora Certa      │                                                              │
│  Preview         │                                                              │
│  Scheduler       │                                                              │
│                  │                                                              │
├──────────────────┴──────────────────────────────────────────────────────────────┤
│                                               [Cancelar]  [Salvar ···]         │
└─────────────────────────────────────────────────────────────────────────────────┘
```
`[Salvar ···]` = botão desabilitado (cor esmaecida, cursor `not-allowed`).
Quando `GET /v1/health` voltar a responder 200, o banner some e `[Salvar]` é reabilitado.

---

### Engine

```
│  Engine                                                                        │
│  ──────────────────────────────────────────────────────────────────────────── │
│  ID da instância                                                               │
│  ┌──────────────────────────────────────────────┐                             │
│  │ studio-a-main                                │                             │
│  └──────────────────────────────────────────────┘                             │
│  Identificador único do engine e do arquivo de snapshot.                       │
│                                                                                │
│  [■] Bloquear instância duplicada                                              │
│      Impede que uma segunda instância com o mesmo ID seja iniciada.            │
│                                                                                │
│  ⚠ Alterações nesta seção requerem reinicialização.                           │
```

---

### API

```
│  API                                                                           │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Host                                Porta                                     │
│  ┌───────────────────────────┐       ┌──────────┐                             │
│  │ 127.0.0.1                 │       │ 8080     │                             │
│  └───────────────────────────┘       └──────────┘                             │
│  ⚠ Alterar host ou porta requer reinicialização.                               │
│                                                                                │
│  CORS                                                                          │
│  [■] Habilitar CORS                                                            │
│                                                                                │
│  Origens permitidas                                                            │
│  ┌──────────────────────────────────────────────┐                             │
│  │ http://localhost:3000                        │                             │
│  │ http://localhost:3333                        │                             │
│  │ http://localhost:5173                        │                             │
│  │ http://localhost:8080                        │                             │
│  └──────────────────────────────────────────────┘                             │
│  [+ Adicionar origem]  [− Remover selecionada]                                 │
```

---

### Áudio

```
│  Áudio                                                                         │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Driver de saída principal                                                     │
│  ( ) null   ( ) file   (•) coreaudio*  ( ) portaudio                          │
│  * coreaudio exibido apenas em macOS (detectado via GET /v1/info)             │
│  ⚠ Alterar driver requer reinicialização.                                      │
│                                                                                │
│  Dispositivo de saída principal                                                │
│  ┌──────────────────────────────────────────────────────────────────────┐     │
│  │ Alto-falantes (MacBook Pro)                                     [▼] │     │
│  └──────────────────────────────────────────────────────────────────────┘     │
│  [□] Usar NullOutput se o dispositivo falhar ao abrir                          │
│                                                                                │
│  Taxa de amostragem     ⚠        Canais     ⚠       Buffer (frames)  ⚠       │
│  ┌────────────────┐              ┌────────┐          ┌────────────────┐       │
│  │ 48000       Hz │              │ 2      │          │ 2048           │       │
│  └────────────────┘              └────────┘          └────────────────┘       │
│  Menor buffer = menor latência. Maior buffer = maior estabilidade.             │
```

---

### Reprodução

```
│  Reprodução                                                                    │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Crossfade padrão                    Fade ao parar                             │
│  ┌──────────────────┐                ┌──────────────────┐                     │
│  │ 8000          ms │                │ 300           ms │                     │
│  └──────────────────┘                └──────────────────┘                     │
│  0 = desabilita crossfade automático                                           │
│                                                                                │
│  Falhas consecutivas máximas                                                   │
│  ┌──────────────────┐                                                          │
│  │ 3                │                                                          │
│  └──────────────────┘                                                          │
│                                                                                │
│  Auto crossfade por energia                                                    │
│  [■] Habilitado                                                                │
│                                                                                │
│  Threshold de energia        Janela mínima          Janela máxima              │
│  ┌────────────────┐          ┌─────────────┐        ┌─────────────┐           │
│  │ -18.0    dBFS  │          │ 2000     ms │        │ 20000    ms │           │
│  └────────────────┘          └─────────────┘        └─────────────┘           │
│                                                                                │
│  Buffers consecutivos para confirmar                                           │
│  ┌──────────────────┐                                                          │
│  │ 8                │                                                          │
│  └──────────────────┘                                                          │
```

---

### Saúde do Áudio

```
│  Saúde do Áudio                                                                │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Intervalo de progresso              Intervalo de saúde                        │
│  ┌──────────────────┐                ┌──────────────────┐                     │
│  │ 500           ms │                │ 500           ms │                     │
│  └──────────────────┘                └──────────────────┘                     │
│                                                                                │
│  Threshold de silêncio               Duração de silêncio                       │
│  ┌──────────────────┐                ┌──────────────────┐                     │
│  │ -60.0      dBFS  │                │ 2000          ms │                     │
│  └──────────────────┘                └──────────────────┘                     │
│                                                                                │
│  VU Meter                                                                      │
│  [■] Habilitado                                                                │
│                                                                                │
│  Intervalo VU Meter                  Peak hold                                 │
│  ┌──────────────────┐                ┌──────────────────┐                     │
│  │ 100           ms │                │ 3000          ms │                     │
│  └──────────────────┘                └──────────────────┘                     │
```

---

### Panic

```
│  Panic                                                                         │
│  ──────────────────────────────────────────────────────────────────────────── │
│  [■] Modo Panic habilitado                                                     │
│                                                                                │
│  Arquivo de cama (panic bed)                                                   │
│  ┌──────────────────────────────────────────────┐  [Procurar]                 │
│  │ /Users/waelson/.../panic/panic.mp3           │                             │
│  └──────────────────────────────────────────────┘                             │
│                                                                                │
│  Auto-panic por silêncio                                                       │
│  [□] Entrar em panic automaticamente ao detectar silêncio sustentado          │
│                                                                                │
│  Threshold de silêncio               Duração mínima                            │
│  ┌──────────────────┐                ┌──────────────────┐                     │
│  │ -60.0      dBFS  │                │ 2000          ms │                     │
│  └──────────────────┘                └──────────────────┘                     │
```

---

### Logging / Segurança / Admin

```
│  Logging                                                                       │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Nível                                                                         │
│  ( ) error   ( ) warn   ( ) info   (•) debug                                  │
│                                                                                │
│  Formato                                                                       │
│  (•) text — legível por humanos                                                │
│  ( ) json — estruturado para ferramentas (Loki, Datadog)                       │
│                                                                                │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Segurança                                                                     │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Diretórios de áudio permitidos                                                │
│  ┌──────────────────────────────────────────────┐                             │
│  │ (vazio — sem restrição de paths)             │                             │
│  └──────────────────────────────────────────────┘                             │
│  [+ Adicionar pasta]  [− Remover selecionada]                                  │
│  ⚠ Deixar vazio não é recomendado em produção                                 │
│                                                                                │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Admin                                                                         │
│  ──────────────────────────────────────────────────────────────────────────── │
│  [■] Habilitar shutdown remoto (POST /v1/admin/shutdown)                       │
│      Não expor em produção.                                                    │
```

---

### Fila

```
│  Fila                                                                          │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Persistência                                                                  │
│  [■] Salvar e restaurar fila entre reinicializações                            │
│                                                                                │
│  Arquivo de snapshot                                                           │
│  ┌──────────────────────────────────────────────┐  [Procurar]                 │
│  │ .../playout-studio-a-main-queue.json         │                             │
│  └──────────────────────────────────────────────┘                             │
│  Vazio = /tmp/playout-<engine-id>-queue.json                                  │
│                                                                                │
│  [■] Restaurar fila ao iniciar                                                 │
│  [□] Apagar snapshot ao encerrar normalmente                                   │
│      (persistir apenas em caso de crash)                                       │
```

---

### Hora Certa

```
│  Hora Certa                                                                    │
│  ──────────────────────────────────────────────────────────────────────────── │
│  Pasta de horas                                                                │
│  ┌──────────────────────────────────────────────┐  [Procurar pasta]           │
│  │ media/hora_certa/hours_dir                   │                             │
│  └──────────────────────────────────────────────┘                             │
│                                                                                │
│  Pasta de minutos                                                              │
│  ┌──────────────────────────────────────────────┐  [Procurar pasta]           │
│  │ media/hora_certa/minutes_dir                 │                             │
│  └──────────────────────────────────────────────┘                             │
│                                                                                │
│  Padrão de arquivo — hora         Padrão de arquivo — minuto                  │
│  ┌──────────────────────┐         ┌──────────────────────┐                   │
│  │ HRS{HH}.mp3          │         │ MIN{MM}.mp3          │                   │
│  └──────────────────────┘         └──────────────────────┘                   │
│  {HH} = hora 00–23                {MM} = minuto 00–59                         │
│                                                                                │
│  Ganho padrão                                                                  │
│  ┌──────────────────┐                                                          │
│  │ 0.0          dB  │                                                          │
│  └──────────────────┘                                                          │
```

---

### Preview (Cue)

```
│  Preview (Cue)                                                                 │
│  ──────────────────────────────────────────────────────────────────────────── │
│  [■] Habilitar preview de áudio                                                │
│      Permite ouvir áudio em dispositivo separado sem interferir no sinal.      │
│                                                                                │
│  Driver de saída do preview                                                    │
│  ( ) null   (•) coreaudio*  ( ) portaudio                                      │
│  * coreaudio exibido apenas em macOS (detectado via GET /v1/info)             │
│  ⚠ Diferente do driver de saída principal (seção Áudio).                      │
│                                                                                │
│  Dispositivo de preview (cue)                                                  │
│  ┌──────────────────────────────────────────────────────────────────────┐     │
│  │ AirPods Pro 2 2025                                              [▼] │     │
│  └──────────────────────────────────────────────────────────────────────┘     │
│  Deve ser diferente do dispositivo de saída principal.                         │
│  Vazio = dispositivo padrão do driver.                                         │
│  ⚠ Alterar dispositivo requer reinicialização.                                 │
```

---

### Scheduler

```
│  Scheduler                                                                     │
│  ──────────────────────────────────────────────────────────────────────────── │
│  [■] Habilitar scheduler de programação horária                                │
│                                                                                │
│  Timezone                                                                      │
│  ┌──────────────────────────────────────────────┐                             │
│  │ America/Sao_Paulo                            │                             │
│  └──────────────────────────────────────────────┘                             │
│  Vazio = timezone do sistema operacional                                       │
│                                                                                │
│  Arquivo de schedule                                                           │
│  ┌──────────────────────────────────────────────┐  [Procurar]                 │
│  │ (vazio — padrão: ~/RadioFlow/schedule.json)  │                             │
│  └──────────────────────────────────────────────┘                             │
│                                                                                │
│  Tolerância de atraso (missed threshold)                                       │
│  ┌──────────────────┐                                                          │
│  │ 5000          ms │                                                          │
│  └──────────────────┘                                                          │
│  Entradas atrasadas além desse tempo são marcadas como MISSED.                 │
│                                                                                │
│  ⚠ Alterações nesta seção requerem reinicialização.                           │
```

---

### Banner pós-salvar

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│  ✓  Configuração salva com sucesso.                                             │
│     Para aplicar as alterações, reinicie o RadioCore pelo menu do systray.     │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Legenda de controles

| Símbolo | Controle |
|---|---|
| `[■]` | Checkbox marcado (true) |
| `[□]` | Checkbox desmarcado (false) |
| `(•)` | Radio button selecionado |
| `( )` | Radio button não selecionado |
| `[▼]` | `<select>` populado via `GET /v1/devices` |
| `┌──┐ / └──┘` | Campo de texto editável (`<input>`) |
| `[Procurar]` | Chama `POST /v1/config/browse` — dialog nativo via Go |
| `[Procurar pasta]` | Idem, com `type: "dir"` |
| `⚠` | Campo que requer reinicialização para efetivar |

---

## Resumo dos arquivos

| Etapa | Arquivo | Ação |
|---|---|---|
| 1 | `docs/plans/config-ui-preview.html` | GERAR — HTML standalone para validação visual |
| 2 | `internal/api/handlers/config.go` | CRIAR — `GetCurrentConfig` |
| 3 | `internal/api/handlers/config.go` | ADICIONAR — `BrowsePath` (dialog nativo) |
| 4 | `internal/api/handlers/config.go` | ADICIONAR — `UpdateConfig` (com backup `.bak`) |
| 4 | `internal/api/handlers/config_test.go` | CRIAR — testes dos 3 handlers (incluindo backup) |
| 5 | `internal/api/handlers/config_html.go` | CRIAR — SPA inline (com polling de health + controle do botão Salvar) |
| 6 | `internal/api/server.go` | MODIFICAR — registrar 4 novos endpoints |
| 7 | `cmd/playout-engine/systray/` | MODIFICAR — adicionar item "Configuração" no menu |
