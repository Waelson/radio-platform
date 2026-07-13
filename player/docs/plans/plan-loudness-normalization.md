# Plano — Normalização Automática de Volume (Loudness Normalization)

**Projeto:** RadioFlow
**Serviço principal:** Library Service + Playout Engine + Player UI
**Data:** julho de 2026
**Prioridade:** Alta (bloqueante para uso em produção)
**Status:** Planejamento

---

## 1. Por que essa feature é importante

### O problema do ouvinte

Em uma emissora sem normalização de volume, o ouvinte experimenta uma montanha-russa sonora: uma música gravada nos anos 90 toca a −18 LUFS, a próxima — masterizada na era do "loudness race" dos anos 2000 — toca a −8 LUFS, e o comercial seguinte, otimizado para chamar atenção, chega a −4 LUFS. O ouvinte é forçado a ajustar o volume do rádio várias vezes por hora. Isso degrada a experiência e aumenta o risco de abandono da emissora.

### O problema operacional

Operadores de rádio sem normalização passam um tempo significativo ajustando manualmente o fader do mixer entre faixas. Em emissoras no modo AUTO (piloto automático), essa variação de volume acontece sem qualquer intervenção — é um problema grave que afeta a percepção de qualidade profissional da emissora.

### O problema regulatório

O padrão internacional ITU-R BS.1770-4, adotado no Brasil pelo setor de radiodifusão, recomenda que o loudness integrado das transmissões fique em torno de **−23 LUFS** (televisão) ou **−16 LUFS** (rádio/streaming). Emissoras que aspiram a transmitir via plataformas de streaming (TuneIn, Vagalume.FM, Rádio.com.br) precisam demonstrar conformidade com esses padrões para indexação.

### Problemas de negócio resolvidos

| Problema | Impacto | Como a feature resolve |
|----------|---------|------------------------|
| Volume inconsistente entre faixas | Experiência degradada do ouvinte; reclamações de anunciantes | Gain offset calculado por faixa, aplicado no mixer antes de reproduzir |
| Tempo de operador ajustando fader | Custo operacional; erro humano | Normalização automática libera o operador para outras tarefas |
| Comerciais "barulhentos" | Anunciantes querem destaque; ouvintes reclamam | Target configurável por tipo de áudio (MUSIC vs. SPOT) |
| Risco de distorção após ganhos | Clipping digital prejudica qualidade | Hard limiter embutido no playout engine impede ultrapassagem do ceiling |
| Não conformidade com padrões de streaming | Rejeição em plataformas de indexação | Target alinhável a −16 LUFS (rádio) ou −23 LUFS (broadcast) |

---

## 2. Requisitos de negócio

### Funcionais

**RF-01** — O sistema deve analisar automaticamente o loudness integrado (LUFS) de cada faixa ao indexá-la na biblioteca, sem intervenção manual do operador.

**RF-02** — O loudness medido deve ser persistido por faixa no banco de dados para ser reutilizado em toda reprodução futura sem nova análise.

**RF-03** — O operador deve poder configurar um target de loudness global (ex: −16 LUFS) e um target específico por tipo de áudio (MUSIC, JINGLE, VINHETA, SPOT).

**RF-04** — O Library Service deve calcular o `gain_db` correto e incluí-lo na resposta da API de faixas (`GET /v1/tracks`), com base em `target_lufs − track.loudness_lufs`. O player apenas repassa o valor recebido no ENQUEUE, sem nenhum cálculo próprio.

**RF-05** — Faixas ainda não analisadas devem ser enfileiradas em uma fila de análise de baixa prioridade e reproduzidas sem normalização até o loudness estar disponível.

**RF-06** — O operador deve poder re-analisar toda a biblioteca ou faixas individuais via UI.

**RF-07** — O operador deve poder desativar a normalização globalmente ou por tipo de áudio.

**RF-08** — O painel de biblioteca deve exibir o loudness (LUFS) de cada faixa e seu status de análise (pendente, analisado, erro).

**RF-09** — O sistema deve proteger contra clipping digital: o gain aplicado nunca deve ultrapassar o ceiling configurável (padrão: −1 dBTP).

### Não-funcionais

**RNF-01** — A análise de loudness não deve bloquear a indexação da faixa. A faixa deve ficar disponível para uso imediato, com análise ocorrendo em background.

**RNF-02** — O worker de análise deve ser limitado em concorrência (padrão: 2 workers) para não impactar o desempenho de reprodução durante análise de bibliotecas grandes.

**RNF-03** — A análise de uma faixa de 4 minutos não deve levar mais de 10 segundos em hardware de emissora típico.

**RNF-04** — A aplicação do gain no mixer do Playout Engine deve ser zero-allocation (já implementado via `applyGain`).

---

## 3. Fluxo de utilização

### 3.1 Fluxo de primeira utilização

> Não há painel de configuração de normalização nesta fase. Os parâmetros
> (target LUFS por tipo, ceiling, max_gain_db) são definidos com valores padrão
> na tabela `settings` do Library Service via migration de inicialização.
> O administrador pode ajustá-los diretamente via `PUT /v1/settings`.

```
Library Service inicia → migration insere defaults na tabela settings
    (normalization.enabled=true, target_lufs=-16, target_lufs_spot=-14, etc.)
    ↓
Faixas já indexadas têm loudness_status = 'pending'
    ↓
LoudnessWorker inicia automaticamente com o serviço
    ↓
Worker re-enfileira todas as faixas com status 'pending' encontradas no banco
    ↓
Operador abre a aba Biblioteca (Library Service)
    ↓
Barra de progresso discreta aparece no topo da aba:
    "Analisando loudness: 847 / 2.341 faixas"
    ↓
Worker processa em background (2 workers concorrentes por padrão)
    ↓
Ao término: barra de progresso desaparece; colunas LUFS e Status na listagem
    mostram os valores medidos por faixa

> Nenhuma alteração na interface de reprodução do player (controles, fila, header).
```

### 3.2 Fluxo de reprodução normalizada

```
Player enfileira faixa → consulta Library Service
    ↓
Library Service retorna track com loudness_lufs = −12.4 E gain_db = −3.6 (calculado pelo handler)
    ↓
Player repassa gain_db: −3.6 diretamente no ENQUEUE — sem cálculo no player
    ↓
Playout Engine aplica applyGain(buf, dBToLinear(−3.6)) no hot path
    ↓
Faixa reproduzida no nível correto sem distorção
```

### 3.3 Fluxo de nova faixa importada

```
Arquivo novo detectado pelo scanner → indexado imediatamente
    ↓
loudness_lufs = NULL (ainda não analisado)
    ↓
Track adicionada à fila de análise de baixa prioridade
    ↓
Worker analisa via ffmpeg ebur128 em background
    ↓
UPDATE tracks SET loudness_lufs = −14.2 WHERE id = ?
    ↓
Próxima vez que a faixa for enfileirada, Library Service já retorna gain_db correto
    ↓
Se a faixa for enfileirada ANTES da análise: Library Service retorna gain_db = 0 (sem normalização)
    ↓
Evento WebSocket notifica o player: faixa analisada (opcional, fase futura)
```

### 3.4 Fluxo de faixa com erro de análise

```
ffmpeg falha ao analisar (arquivo corrompido, codec exótico)
    ↓
loudness_lufs = NULL, loudness_status = 'error', loudness_error = "mensagem"
    ↓
Library Service retorna gain_db = 0 → faixa reproduzida sem normalização
    ↓
UI exibe ícone de aviso na linha da faixa
    ↓
Operador pode tentar re-análise manual
```

---

## 4. Como os concorrentes resolvem o problema

### 4.1 RadioBOSS (DJSoft.Net) — Internacional

**Abordagem:** análise EBU R128 sob demanda e automática na importação.

- Usa o padrão EBU R128 (ITU-R BS.1770) para medir loudness integrado em LUFS.
- **Não re-encoda** o arquivo — apenas armazena o valor de correção no banco de dados.
- Target padrão configurável; default: **−9 LUFS** (conservador, para não alterar drasticamente arquivos não normalizados).
- O operador pode normalizar em lote via "Tools → Process Tracks → Normalize".
- Faixa individual pode ser normalizada pela "Track Tool" com botão "Auto" ao lado do slider de nível.
- Correção é **reversível** a qualquer momento — sem degradação de qualidade pois o arquivo original não é alterado.
- Suporte a normalização automática ao adicionar novas faixas à biblioteca (quando scheduled updates estão ativos).

**Fonte:** [RadioBOSS Normalization Documentation](https://www.radioboss.fm/support/radioboss-cloud/normalization/)

### 4.2 RCS Zetta — Internacional (padrão premium)

**Abordagem:** múltiplos métodos de normalização por percepção, todos armazenados por asset.

- Calcula loudness **percebido** (não pico), levando em conta como o ouvido humano interpreta o som.
- Suporta múltiplos algoritmos simultaneamente: EBU R128, ReplayGain, Peak.
- **Todos os valores são armazenados no asset** — se o operador mudar o método de normalização na configuração da estação, não precisa re-analisar a biblioteca; o sistema usa o valor já calculado para o novo método.
- Motivação principal: combater o "loudness race" — músicas masterizadas cada vez mais altas para parecerem mais chamativas que a concorrência.
- Configuração centralizada em "Station Configuration → Normalization".

**Fonte:** [New Normalization Methods in Zetta 2.9](https://www.rcsworks.com/new-in-new-normalization-methods-in-zetta-2-9/)

### 4.3 mAirList — Internacional (profissional europeu)

**Abordagem:** normalização R128 integrada ao banco de dados, com suporte a ReplayGain tags.

- Implementou R128 normalisation na versão 6.2.
- Armazena o valor de loudness no banco de dados local; lê ReplayGain tags existentes nos arquivos.
- Discussões na comunidade indicam suporte a normalização de WAV files diretamente no banco.
- Foco em conformidade com padrões europeus de broadcast (EBU R128, −23 LUFS para televisão).

**Fonte:** [mAirList Community Forum — Loudness Normalization](https://community.mairlist.com/t/loudness-normalization/11677)

### 4.4 PlayIt Live — Internacional (gratuito)

**Abordagem:** análise em background com ITU-R BS.1770, gain aplicado automaticamente na reprodução.

- Analisa faixas em background usando o algoritmo **ITU-R BS.1770** (LUFS normalization).
- Target padrão: **−16 LUFS** (mais adequado para rádio/streaming que os −23 LUFS de TV).
- Calcula um "offset gain" por faixa e armazena no banco.
- Na reprodução, aplica o gain automaticamente — o operador não precisa fazer nada após a configuração inicial.
- Análise pode levar horas para bibliotecas grandes — roda em background sem interromper a operação.
- Novas faixas adicionadas são **processadas automaticamente em background**.

**Fonte:** [PlayIt Live — Loudness Analysis](https://www.playitsoftware.com/Features/Live/LoudnessAnalysis)

### 4.5 RadioPro Prime — Nacional (Brasil)

**Abordagem:** normalização integrada, com suporte a relatórios e conformidade com padrão brasileiro.

- Software utilizado em 1.000+ emissoras brasileiras.
- Oferece normalização de volume integrada ao módulo de áudio.
- O padrão adotado no Brasil (ITU-R BS.1770-4) recomenda **−23 LUFS** para TV e aproximadamente **−16 LUFS** para rádio/streaming.
- Emissoras que usam processadores digitais (Orban, Omnia) precisam coordenar o loudness do software com o processador para evitar over-compression.

**Fonte:** [RadioPro Prime](https://www.radiopro.com.br/radiopro-site/software-para-emissoras-de-radio-prime/) | [LUFS e Rádios Brasileiras](https://juniorpinheirovoz.com.br/lufs-e-a-qualidade-do-audio-por-que-as-radios-precisam-respeitar-esse-padrao/)

### 4.6 Resumo comparativo

| Solução | Padrão | Target padrão | Análise automática | Armazena por asset | Sem re-encode |
|---------|--------|---------------|--------------------|--------------------|---------------|
| RadioBOSS | EBU R128 | −9 LUFS | Sim (import) | Sim | Sim |
| RCS Zetta | EBU R128 + ReplayGain | Configurável | Sim | Sim (múltiplos métodos) | Sim |
| mAirList | EBU R128 | −23 LUFS | Sim | Sim | Sim |
| PlayIt Live | ITU-R BS.1770 | −16 LUFS | Sim (background) | Sim | Sim |
| RadioPro | ITU-R BS.1770 | −16/−23 LUFS | Sim | Sim | Sim |
| **RadioFlow** | **EBU R128** | **−16 LUFS** | **Sim (background)** | **Sim** | **Sim** |

---

## 5. Proposta de solução

### Princípios de design

1. **Sem re-encode.** O arquivo de áudio original nunca é alterado. O loudness é medido uma única vez e armazenado no banco. O gain é aplicado em tempo real no mixer do Playout Engine.
2. **Não bloqueia a indexação.** A faixa fica disponível imediatamente após o scan. A análise de loudness ocorre em background via worker pool.
3. **Arquitetura já suporta gain.** O `QueueItem.GainDB` já existe e o `applyGain()` já está implementado no Playout Engine. O RadioFlow só precisa popular esse campo corretamente.
4. **Target configurável.** O operador define o target global e, opcionalmente, targets por tipo de áudio via Settings API já existente.
5. **Cálculo centralizado no servidor.** O `gain_db` é calculado pelo handler do Library Service ao servir a faixa — o player apenas repassa o valor no ENQUEUE. Isso garante consistência para qualquer cliente futuro (web, mobile, etc.) sem duplicar a lógica.
6. **Fallback seguro.** Faixas sem loudness analisado recebem `gain_db = 0` na resposta da API — sem normalização, sem erro no player.

### Fluxo técnico completo

```
[Scanner / Watcher]
    → indexa arquivo (track.loudness_lufs = NULL)
    → enfileira ID na LoudnessQueue (canal Go com buffer)

[LoudnessWorker pool — 2 workers concorrentes]
    → dequeue track ID
    → executa: ffmpeg -i <path> -af ebur128=peak=true -f null - 2>&1
    → parseia "Integrated loudness: I: −14.2 LUFS" do stderr
    → UPDATE tracks SET loudness_lufs = −14.2, loudness_status = 'done' WHERE id = ?

[Player — no momento do ENQUEUE]
    → GET /v1/tracks/{id} → recebe loudness_lufs = −14.2 E gain_db = −1.8 (já calculado)
    → repassa gain_db diretamente no ENQUEUE — sem nenhum cálculo no player

[Playout Engine — hot path]
    → applyGain(buf, dBToLinear(−1.8)) [já implementado]
    → áudio reproduzido no nível correto
```

### Análise com ffmpeg

O ffmpeg já é dependência obrigatória do projeto. O filtro `ebur128` mede loudness integrado conforme EBU R128 / ITU-R BS.1770:

```bash
# Comando de análise (não produz output de áudio — apenas mede)
ffmpeg -i /path/to/track.mp3 \
       -af "ebur128=peak=true" \
       -f null - 2>&1 | grep "I:"
# Output: "    I:         -14.2 LUFS"
```

**Vantagens sobre `loudnorm` (dois passes):**
- `ebur128` é um único passe — mais rápido para análise de biblioteca
- Não precisa re-codificar — apenas mede
- Output parseable e determinístico

### Cálculo de gain

```
gain_db = target_lufs − track.loudness_lufs

Exemplo:
  target_lufs = −16.0
  track.loudness_lufs = −12.4  (faixa masterizada alto)
  gain_db = −16.0 − (−12.4) = −3.6 dB  → atenua a faixa

  target_lufs = −16.0
  track.loudness_lufs = −22.0  (faixa gravada baixo)
  gain_db = −16.0 − (−22.0) = +6.0 dB  → amplifica a faixa

Ceiling (proteção contra clipping):
  max_gain_db = ceiling_dbtp − track.true_peak_dbtp
  gain_db = min(gain_db, max_gain_db)
  (implementação simplificada: cap em +12 dB por padrão)
```

---

## 6. Impacto na UI e telas novas

### 6.1 Painel de Configurações — fora do escopo desta fase

Não haverá tela de configuração de normalização nesta fase. Os parâmetros de normalização são gerenciados exclusivamente via tabela `settings` do Library Service (já existente, migration 007), com valores padrão inseridos na migration de inicialização.

O operador que precisar ajustar os targets pode fazê-lo diretamente pela API (`PUT /v1/settings`) ou via ferramenta de administração futura. A UI de configuração é considerada uma fase posterior.

---

### 6.2 Biblioteca de Faixas — colunas adicionais

Na tela de busca/listagem de faixas do Library Service (player, aba Biblioteca), duas novas colunas opcionais:

```
┌────────┬────────────────────────────┬──────────┬──────────┬────────────┐
│ Tipo   │ Título / Artista           │ Duração  │  LUFS    │ Status     │
├────────┼────────────────────────────┼──────────┼──────────┼────────────┤
│ MUSIC  │ Pétalas Neon               │ 3:23     │ −14.2    │ ✓          │
│        │ Noda de Cajú               │          │          │            │
├────────┼────────────────────────────┼──────────┼──────────┼────────────┤
│ MUSIC  │ Meu Neguinho               │ 4:17     │ −11.8    │ ✓          │
│        │ Limão com Mel              │          │          │            │
├────────┼────────────────────────────┼──────────┼──────────┼────────────┤
│ SPOT   │ Promoção Supermercado      │ 0:30     │ −8.4     │ ✓          │
│        │                            │          │          │            │
├────────┼────────────────────────────┼──────────┼──────────┼────────────┤
│ MUSIC  │ Forró da Saudade           │ 3:51     │  —       │ ⏳ análise │
│        │ Trio Nordestino            │          │          │            │
├────────┼────────────────────────────┼──────────┼──────────┼────────────┤
│ VINHETA│ Abertura Manhã             │ 0:08     │  —       │ ⚠ erro     │
│        │                            │          │          │            │
└────────┴────────────────────────────┴──────────┴──────────┴────────────┘

  Legenda:  ✓ Analisado   ⏳ Pendente   ⚠ Erro de análise
            [⚙ Configurar colunas]  [Analisar selecionadas]
```

**Explicação:**
- Coluna **LUFS** mostra o loudness medido. Faixas muito acima do target ficam destacadas em amarelo (ex: SPOT em −8.4 quando target é −14).
- Coluna **Status** indica o estado da análise: analisado (✓), pendente (⏳), erro (⚠).
- Clique direito na faixa → menu contextual → "Re-analisar loudness".
- Seleção múltipla → "Analisar selecionadas" no rodapé.
- Colunas são opcionais — podem ser ocultadas pelo operador.

---

### 6.3 Tooltip de gain na fila de reprodução — fora do escopo desta fase

Não haverá alteração na interface de reprodução do player (fila, controles, header, tooltip de faixa). O player continua sem mudanças visuais — apenas recebe `gain_db` da API e repassa no ENQUEUE.

---

## 7. Modelo de dados

### 7.1 Migration 009 — loudness em tracks

```sql
-- 009_loudness.sql

-- loudness_lufs: loudness integrado medido pelo ffmpeg ebur128 (EBU R128 / ITU-R BS.1770)
-- NULL = ainda não analisado ou análise com erro
ALTER TABLE tracks ADD COLUMN loudness_lufs    REAL;

-- true_peak_dbtp: pico verdadeiro (dBTP) para cálculo de ceiling anti-clipping
ALTER TABLE tracks ADD COLUMN true_peak_dbtp  REAL;

-- loudness_status: 'pending' | 'analyzing' | 'done' | 'error'
ALTER TABLE tracks ADD COLUMN loudness_status TEXT NOT NULL DEFAULT 'pending';

-- loudness_error: mensagem de erro quando loudness_status = 'error'
ALTER TABLE tracks ADD COLUMN loudness_error  TEXT;

-- loudness_analyzed_at: quando a análise foi concluída
ALTER TABLE tracks ADD COLUMN loudness_analyzed_at DATETIME;

-- Índice para o worker encontrar rapidamente faixas pendentes
CREATE INDEX IF NOT EXISTS idx_tracks_loudness_status ON tracks(loudness_status);
```

### 7.2 Settings — chaves de normalização (tabela `settings` do Library Service)

Como o Library Service é o responsável por indexação, análise de áudio e cálculo do `gain_db`, os parâmetros de normalização residem exclusivamente na tabela `settings` **do Library Service** (já existente, migration 007). Não há nenhuma chave de normalização no Playout Engine.

As chaves abaixo são inseridas com seus valores padrão na migration de inicialização via `INSERT OR IGNORE` — nunca sobrescrevem valores já configurados:

| Chave | Valor padrão | Descrição |
|-------|-------------|-----------|
| `normalization.enabled` | `true` | Ativa/desativa a normalização globalmente |
| `normalization.target_lufs` | `-16.0` | Target global de loudness em LUFS |
| `normalization.ceiling_dbtp` | `-1.0` | Ceiling anti-clipping em dBTP |
| `normalization.max_gain_db` | `12.0` | Ganho máximo permitido (proteção contra ruído) |
| `normalization.per_type_enabled` | `false` | Ativa targets por tipo |
| `normalization.target_lufs_music` | `-16.0` | Target LUFS para MUSIC |
| `normalization.target_lufs_jingle` | `-16.0` | Target LUFS para JINGLE |
| `normalization.target_lufs_vinheta` | `-18.0` | Target LUFS para VINHETA |
| `normalization.target_lufs_spot` | `-14.0` | Target LUFS para SPOT |
| `normalization.worker_concurrency` | `2` | Número de workers concorrentes |

### 7.3 Track struct — campos adicionados

```go
// Track representa um arquivo de áudio indexado.
type Track struct {
    // ... campos existentes ...

    // Loudness — EBU R128 / ITU-R BS.1770
    LoudnessLUFS       *float64   // nil = não analisado; valor em LUFS (ex: -14.2)
    TruePeakDBTP       *float64   // nil = não analisado; pico verdadeiro em dBTP
    LoudnessStatus     string     // "pending" | "analyzing" | "done" | "error"
    LoudnessError      string     // mensagem de erro, se LoudnessStatus == "error"
    LoudnessAnalyzedAt *time.Time // quando a análise foi concluída
}
```

---

## 8. Endpoints e contratos

### 8.1 Endpoints existentes impactados

#### `GET /v1/tracks/{id}` — track por ID
**Impacto:** resposta passa a incluir campos de loudness.

**Resposta após a mudança:**
```json
{
  "ok": true,
  "data": {
    "id": "01KX1CHN2Q05JYMCBKH8P1P1WQ",
    "path": "/musicas/Noda de Caju - Petalas Neon.mp3",
    "title": "Pétalas Neon",
    "artist": "Noda de Cajú",
    "type": "MUSIC",
    "duration_ms": 203559,
    "loudness_lufs": -14.2,
    "true_peak_dbtp": -1.8,
    "loudness_status": "done",
    "loudness_analyzed_at": "2026-07-13T18:00:00Z",
    "gain_db": -1.8
  }
}
```
*`gain_db` é calculado pelo handler com base em `loudness_lufs` e nas configurações de normalização ativas. Retorna `0` quando `loudness_lufs` é `null` ou quando a normalização está desativada. O player deve repassar este valor diretamente no ENQUEUE — sem nenhum cálculo adicional.*

*Campos `loudness_lufs` e `true_peak_dbtp` são `null` quando não analisados.*

#### `GET /v1/tracks` (search) — listagem de faixas
**Impacto:** todos os itens da lista passam a incluir campos de loudness.

**Novos filtros de query:**
| Parâmetro | Tipo | Descrição |
|-----------|------|-----------|
| `loudness_status` | string | Filtra por status: `pending`, `done`, `error` |
| `loudness_min` | float | Faixas com LUFS ≥ valor (ex: mostra faixas muito altas) |
| `loudness_max` | float | Faixas com LUFS ≤ valor |

---

### 8.2 Endpoints novos — Library Service

#### `GET /v1/loudness/status`
Retorna o estado atual do worker de análise de loudness.

**Resposta:**
```json
{
  "ok": true,
  "data": {
    "total": 2341,
    "done": 1847,
    "pending": 490,
    "analyzing": 2,
    "error": 4,
    "worker_running": true,
    "estimated_remaining_seconds": 1470
  }
}
```

---

#### `POST /v1/loudness/analyze`
Enfileira faixas para análise (ou re-análise). Retorna imediatamente — análise ocorre em background.

**Body (todos opcionais — sem body = analisa todas as pendentes):**
```json
{
  "track_ids": ["id1", "id2"],  // IDs específicos (opcional)
  "type": "MUSIC",               // analisa por tipo (opcional)
  "reanalyze": false             // true = re-analisa mesmo as já analisadas
}
```

**Resposta:**
```json
{
  "ok": true,
  "data": {
    "enqueued": 494,
    "message": "494 faixas enfileiradas para análise"
  }
}
```

---

#### `POST /v1/loudness/analyze/{track_id}`
Enfileira uma única faixa para análise imediata (alta prioridade na fila). Útil para análise após edição manual.

**Resposta:**
```json
{
  "ok": true,
  "data": {
    "track_id": "01KX1CHN2Q05JYMCBKH8P1P1WQ",
    "message": "Faixa enfileirada para análise"
  }
}
```

---

#### `DELETE /v1/loudness/analyze` (cancelar)
Cancela a análise em andamento. Faixas já analisadas mantêm seus valores.

**Resposta:**
```json
{
  "ok": true,
  "data": { "message": "Análise cancelada. 847 faixas já processadas mantêm seus valores." }
}
```

---

#### `GET /v1/settings` — ampliado (endpoint existente)
As novas chaves de normalização já aparecem automaticamente, pois a settings store é genérica.

**Exemplo de response relevante:**
```json
{
  "ok": true,
  "data": {
    "normalization.enabled": "true",
    "normalization.target_lufs": "-16.0",
    "normalization.ceiling_dbtp": "-1.0",
    "normalization.max_gain_db": "12.0",
    "normalization.per_type_enabled": "false",
    "normalization.target_lufs_music": "-16.0",
    "normalization.target_lufs_spot": "-14.0",
    "normalization.target_lufs_vinheta": "-18.0",
    "normalization.target_lufs_jingle": "-16.0",
    "normalization.worker_concurrency": "2"
  }
}
```

---

#### `PUT /v1/settings` (existente) — sem mudança de contrato
Usado para salvar as configurações de normalização. Sem mudança de contrato.

---

### 8.3 Impacto no ENQUEUE do Playout Engine

O Library Service calcula o `gain_db` e o devolve na resposta da API. O player **não faz nenhum cálculo** — apenas lê `gain_db` da resposta e repassa no ENQUEUE. Não há mudança no contrato do Playout Engine — o campo `gain_db` já existe no schema de ENQUEUE.

`gain_db` é calculado e repassado para **todos os tipos de áudio e todos os pontos de enfileiramento**, incluindo blocos comerciais (SPOT), jingles e vinhetas:

| Ponto de ENQUEUE | Fonte do `gain_db` |
|---|---|
| `advEnqueue` (busca avançada) | `track.gain_db` via `GET /v1/tracks/{id}` |
| `rotGen` (rotação de clock) | `item.track.gain_db` via `GET /v1/tracks/{id}` |
| `libEnqueuePlaylist` (playlist) | `track.gain_db` via `GET /v1/tracks/{id}` |
| `libEnqueueBreak` (bloco comercial) | `slot.track.gain_db` via `GET /v1/breaks/{id}` — **o handler de breaks também deve calcular `gain_db` por slot** |

> O handler de `GET /v1/breaks/{id}` deve expandir cada slot com os dados completos da faixa — incluindo `gain_db` calculado via `calcGainDB()` — da mesma forma que o handler de `GET /v1/tracks`.

**Exemplo de ENQUEUE de bloco comercial com normalização:**
```json
POST /v1/queue/enqueue
{
  "items": [
    {
      "asset_id": "01KX1CHN2Q05JYMCBKH8P1P1WQ",
      "path": "/spots/Promo Supermercado.mp3",
      "title": "Promoção Supermercado",
      "type": "SPOT",
      "duration_ms": 30000,
      "gain_db": -5.6
    },
    {
      "asset_id": "01KX1CHN2Q05JYMCBKH8P1P1WR",
      "path": "/jingles/Abertura.mp3",
      "title": "Abertura Manhã",
      "type": "JINGLE",
      "duration_ms": 8000,
      "gain_db": 2.1
    }
  ]
}
```

**Exemplo de ENQUEUE de faixa comum:**
```json
POST /v1/queue/enqueue
{
  "items": [{
    "asset_id": "01KX1CHN2Q05JYMCBKH8P1P1WQ",
    "path": "/musicas/Noda de Caju - Petalas Neon.mp3",
    "title": "Pétalas Neon",
    "artist": "Noda de Cajú",
    "type": "MUSIC",
    "duration_ms": 203559,
    "gain_db": -1.8
  }]
}
```

---

## 9. Detalhamento técnico da implementação

### 9.1 Library Service — LoudnessAnalyzer

Novo pacote `library/internal/loudness/`:

```go
// analyzer.go
package loudness

import (
    "context"
    "fmt"
    "os/exec"
    "regexp"
    "strconv"
)

// Result contém os valores medidos para uma faixa.
type Result struct {
    IntegratedLUFS float64
    TruePeakDBTP   float64
}

// Analyze mede o loudness integrado e o true peak de um arquivo de áudio
// usando o filtro ebur128 do ffmpeg. Não altera o arquivo.
func Analyze(ctx context.Context, ffmpegPath, filePath string) (Result, error) {
    if ffmpegPath == "" {
        ffmpegPath = "ffmpeg"
    }
    cmd := exec.CommandContext(ctx, ffmpegPath,
        "-i", filePath,
        "-af", "ebur128=peak=true",
        "-f", "null", "-",
    )
    // ffmpeg escreve os resultados no stderr
    out, _ := cmd.CombinedOutput()
    return parseEBUR128Output(string(out))
}

var (
    reIntegrated = regexp.MustCompile(`I:\s+([-\d.]+)\s+LUFS`)
    reTruePeak   = regexp.MustCompile(`True peak:\s+Peak:\s+([-\d.]+)\s+dBFS`)
)

func parseEBUR128Output(output string) (Result, error) {
    mI := reIntegrated.FindStringSubmatch(output)
    if mI == nil {
        return Result{}, fmt.Errorf("loudness: integrated LUFS not found in ffmpeg output")
    }
    integrated, err := strconv.ParseFloat(mI[1], 64)
    if err != nil {
        return Result{}, fmt.Errorf("loudness: parse integrated: %w", err)
    }
    r := Result{IntegratedLUFS: integrated}

    // True peak é opcional — alguns codecs não reportam
    if mTP := reTruePeak.FindStringSubmatch(output); mTP != nil {
        if tp, err := strconv.ParseFloat(mTP[1], 64); err == nil {
            r.TruePeakDBTP = tp
        }
    }
    return r, nil
}
```

### 9.2 Library Service — LoudnessWorker

```go
// worker.go
package loudness

// Worker processa a fila de análise de loudness em background.
// Concorrência controlada por um semáforo (canal Go).
type Worker struct {
    store      TrackStore  // interface: UpdateLoudness(ctx, id, result)
    queue      chan string  // track IDs a analisar
    ffmpegPath string
    log        *slog.Logger
    sem        chan struct{} // semáforo de concorrência
}

func NewWorker(store TrackStore, concurrency int, ffmpegPath string, log *slog.Logger) *Worker {
    return &Worker{
        store:      store,
        queue:      make(chan string, 10_000),
        ffmpegPath: ffmpegPath,
        log:        log,
        sem:        make(chan struct{}, concurrency),
    }
}

// Enqueue adiciona um track ID à fila de análise. Non-blocking: descarta se cheia.
func (w *Worker) Enqueue(id string) {
    select {
    case w.queue <- id:
    default:
        w.log.Warn("loudness: fila cheia, análise descartada", "track_id", id)
    }
}

// Run processa a fila até ctx ser cancelado.
func (w *Worker) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case id := <-w.queue:
            w.sem <- struct{}{} // adquire slot
            go func(trackID string) {
                defer func() { <-w.sem }() // libera slot
                w.analyze(ctx, trackID)
            }(id)
        }
    }
}

func (w *Worker) analyze(ctx context.Context, trackID string) {
    track, err := w.store.FindByID(ctx, trackID)
    if err != nil { /* log e retorna */ return }

    // Marca como "analyzing" para o status endpoint
    _ = w.store.UpdateLoudnessStatus(ctx, trackID, "analyzing", "")

    result, err := Analyze(ctx, w.ffmpegPath, track.Path)
    if err != nil {
        _ = w.store.UpdateLoudnessStatus(ctx, trackID, "error", err.Error())
        return
    }
    _ = w.store.UpdateLoudness(ctx, trackID, result)
}
```

### 9.3 Library Service — TrackStore: novos métodos

```go
// UpdateLoudness atualiza loudness_lufs, true_peak_dbtp e loudness_status = 'done'.
func (s *TrackStore) UpdateLoudness(ctx context.Context, id string, r loudness.Result) error

// UpdateLoudnessStatus atualiza apenas o status e a mensagem de erro.
func (s *TrackStore) UpdateLoudnessStatus(ctx context.Context, id, status, errMsg string) error

// CountByLoudnessStatus retorna contagens agrupadas por status (para o endpoint /v1/loudness/status).
func (s *TrackStore) CountByLoudnessStatus(ctx context.Context) (map[string]int, error)

// ListPendingLoudness retorna IDs de faixas com loudness_status IN ('pending', 'error') LIMIT n.
func (s *TrackStore) ListPendingLoudness(ctx context.Context, limit int) ([]string, error)
```

### 9.4 Library Service — cálculo de gain_db no handler

O cálculo é feito dentro do handler HTTP de tracks, em Go, antes de serializar a resposta. O player não carrega settings nem realiza nenhum cálculo.

```go
// calcGainDB calcula o gain a aplicar na reprodução dado o loudness medido
// e as configurações de normalização ativas. Retorna 0.0 se a normalização
// estiver desativada ou se loudnessLUFS for nil (faixa não analisada).
func calcGainDB(loudnessLUFS *float64, trackType string, s NormSettings) float64 {
    if !s.Enabled || loudnessLUFS == nil {
        return 0.0
    }

    target := s.TargetLUFS // target global
    if s.PerTypeEnabled {
        switch trackType {
        case "MUSIC":   target = s.TargetMusic
        case "JINGLE":  target = s.TargetJingle
        case "VINHETA": target = s.TargetVinheta
        case "SPOT":    target = s.TargetSpot
        }
    }

    gainDB := target - *loudnessLUFS

    // Limita ganho máximo positivo (proteção contra amplificação de ruído)
    if gainDB > s.MaxGainDB {
        gainDB = s.MaxGainDB
    }

    // Arredonda para 1 casa decimal
    return math.Round(gainDB*10) / 10
}

// NormSettings carrega as chaves relevantes da SettingsStore.
type NormSettings struct {
    Enabled        bool
    TargetLUFS     float64
    PerTypeEnabled bool
    TargetMusic    float64
    TargetJingle   float64
    TargetVinheta  float64
    TargetSpot     float64
    MaxGainDB      float64
}
```

O handler de `GET /v1/tracks/{id}` (e de listagem) carrega o `NormSettings` da `SettingsStore` e inclui `gain_db` na resposta JSON. O player lê `track.gain_db` e repassa diretamente no ENQUEUE — **zero lógica de normalização no frontend**.

### 9.5 Integração ao fluxo de indexação

No `indexer.go`, após o `Upsert()` da faixa, enfileira para análise:

```go
// Após upsert bem-sucedido:
if s.loudnessWorker != nil {
    s.loudnessWorker.Enqueue(track.ID)
}
```

### 9.6 Startup — análise das faixas pendentes

No `main.go` do Library Service, ao iniciar, o worker carrega as faixas pendentes:

```go
// Após iniciar o LoudnessWorker:
ids, _ := trackStore.ListPendingLoudness(ctx, 10_000)
for _, id := range ids {
    loudnessWorker.Enqueue(id)
}
```

---

## 10. Riscos e mitigações

| Risco | Probabilidade | Impacto | Mitigação |
|-------|:---:|:---:|---------|
| ffmpeg não disponível no PATH do operador | Média | Alto | Verificar presença do ffmpeg no startup; logar aviso claro; documentar requisito |
| Análise consome CPU excessiva e degrada áudio | Média | Alto | Semáforo de concorrência (padrão: 2 workers); nice/ionice no subprocesso ffmpeg |
| Faixa corrompida trava o worker | Baixa | Médio | Timeout de contexto por análise (30s); status = 'error'; worker continua para próxima |
| LUFS desatualizado após edição do arquivo | Baixa | Médio | PATCH /v1/tracks/{id} e re-análise; watcher de fsnotify detecta modificação |
| Ganho positivo grande amplifica ruído de fundo | Média | Médio | `max_gain_db` configurável (padrão +12 dB); flag na UI para faixas muito silenciosas |
| Clipping digital após gain positivo | Baixa | Alto | Ceiling configurável; true peak medido e considerado no cálculo de ceiling |
| Fila de análise perdida em restart do serviço | Baixa | Baixo | Status 'pending'/'analyzing' persiste no SQLite; no startup, faixas pending/analyzing são re-enfileiradas |
| Incompatibilidade de ffmpeg com codec exótico | Baixa | Baixo | Captura de erro; loudness_status = 'error'; faixa reproduzida sem normalização |
| Operador desativa normalização sem perceber | Baixa | Médio | Normalização controlada pela chave `normalization.enabled` na tabela `settings` do Library Service; quando desativada, todos os `gain_db` retornam `0.0` pela API — sem necessidade de toggle na UI nesta fase |

---

## 11. Fases de implementação

### Fase 1 — Branch, migration e modelo de dados
**Objetivo:** preparar a infraestrutura sem alterar comportamento visível.

1. Solicitar aprovação para criar branch `feature/loudness-normalization` a partir da `main`.
2. Criar migration `009_loudness.sql` com os 5 novos campos em `tracks`.
3. Registrar migration em `db.go` com o padrão existente (`//go:embed` + `migrationDone`).
4. Adicionar campos `LoudnessLUFS`, `TruePeakDBTP`, `LoudnessStatus`, `LoudnessError`, `LoudnessAnalyzedAt` ao struct `Track`.
5. Atualizar `Upsert`, `FindByID`, `FindByPath`, `Search` e `scanTrack` no `track_store.go`.
6. Adicionar novos métodos ao `TrackStore`: `UpdateLoudness`, `UpdateLoudnessStatus`, `CountByLoudnessStatus`, `ListPendingLoudness`.
7. Adicionar chaves de normalização na migration de settings (ou via nova migration `010_normalization_settings.sql`).
8. Escrever testes unitários para os novos métodos do store (`:memory:`).
9. Build + testes verdes.

---

### Fase 2 — LoudnessAnalyzer e LoudnessWorker
**Objetivo:** análise real de loudness via ffmpeg em background.

1. Criar pacote `library/internal/loudness/` com `analyzer.go` e `worker.go`.
2. Implementar `Analyze()` com o filtro `ebur128=peak=true`.
3. Implementar parser de output do ffmpeg (regex para `I:` e `True peak:`).
4. Implementar `Worker` com semáforo de concorrência e fila buffered.
5. Integrar worker ao `indexer.go` — enfileira após upsert.
6. Integrar ao `main.go` — inicializa worker, re-enfileira faixas pending no startup.
7. Escrever testes para o parser e para o worker (mock do analyzer).
8. Build + testes verdes.

---

### Fase 3 — API de loudness no Library Service
**Objetivo:** expor endpoints para o player controlar e monitorar a análise.

1. Criar `library/internal/api/handlers/loudness.go` com:
   - `GET /v1/loudness/status`
   - `POST /v1/loudness/analyze`
   - `POST /v1/loudness/analyze/{id}`
   - `DELETE /v1/loudness/analyze`
2. Registrar rotas em `server.go`.
3. Adicionar filtros de loudness ao `GET /v1/tracks` (`loudness_status`, `loudness_min`, `loudness_max`).
4. Escrever testes de handler (`httptest.NewRecorder`).
5. Build + testes verdes.

---

### Fase 4 — Progresso de análise na aba Biblioteca
**Objetivo:** operador monitora o andamento da análise diretamente na aba Biblioteca — sem qualquer alteração na interface de reprodução do player.

> Não haverá painel de configuração de normalização nesta fase. Os parâmetros ficam na tabela `settings` do Library Service com valores padrão.

1. Adicionar indicador de progresso exclusivamente na aba Biblioteca (Library Service):
   - Barra de progresso discreta no topo da aba enquanto houver faixas pendentes.
   - Texto: `"Analisando loudness: 847 / 2.341 faixas"`.
   - Polling a cada 10s em `GET /v1/loudness/status`.
   - Botão "Analisar biblioteca" para disparar análise manual (`POST /v1/loudness/analyze`).
   - Botão "Cancelar análise" (`DELETE /v1/loudness/analyze`) visível apenas quando análise está rodando.
2. Nenhuma alteração nos demais elementos do player (fila, controles, header, tooltips).

---

### Fase 5 — gain_db no handler do Library Service e passagem no ENQUEUE
**Objetivo:** normalização ativa na reprodução para todos os tipos de áudio, com cálculo centralizado no servidor.

1. Implementar `calcGainDB()` e `NormSettings` no pacote de handlers do Library Service.
2. Adicionar campo `GainDB float64` à struct de resposta de track (JSON: `"gain_db"`).
3. Carregar `NormSettings` da `SettingsStore` e incluir `gain_db` calculado nos handlers:
   - `GET /v1/tracks` e `GET /v1/tracks/{id}` — faixas individuais.
   - `GET /v1/breaks/{id}` — expandir cada slot com os dados completos da faixa, incluindo `gain_db` calculado por slot. Blocos comerciais (SPOT, JINGLE, VINHETA) recebem normalização com seus targets específicos.
4. Atualizar os 4 pontos de ENQUEUE no `player.html` para ler `gain_db` e repassar — **sem nenhum cálculo no player**:
   - `advEnqueue` → `gain_db: t.gain_db || 0`
   - rotGen loop → `gain_db: item.track.gain_db || 0`
   - `libEnqueuePlaylist` → `gain_db: it.track?.gain_db || 0`
   - `libEnqueueBreak` → `gain_db: slot.track?.gain_db || 0`
5. Escrever testes unitários para `calcGainDB()` cobrindo: normalização ativa, desativada, per-type (MUSIC, SPOT, JINGLE, VINHETA), loudness nil, ceiling de ganho.
6. Testar manualmente com faixas de loudness conhecido, incluindo spots comerciais.

---

### Fase 6 — UI na biblioteca: colunas de loudness
**Objetivo:** visibilidade do loudness por faixa.

1. Adicionar colunas LUFS e Status na listagem de faixas (opcionais, toggle).
2. Highlight visual para faixas muito acima do target (ex: SPOT em −8 quando target é −14).
3. Tooltip de gain na fila de reprodução.
4. Menu contextual: "Re-analisar loudness" por faixa individual.
5. "Analisar selecionadas" para seleção múltipla.

---

### Fase 7 — Testes de integração, ajustes e PR
**Objetivo:** validação end-to-end e merge.

1. Testes manuais com biblioteca real de emissora (faixas de loudness variado).
2. Verificar que faixas não analisadas são reproduzidas sem erro (gain_db = 0).
3. Verificar que worker não impacta reprodução em andamento.
4. Atualizar `benchmark.md` — marcar item 4 como concluído.
5. Documentar configuração recomendada no README do library service.
6. Abrir PR para `main`.

---

## 12. Pontos adicionais

### 12.1 Dependência com Marcadores de intro/outro (item 5 do benchmark)

A normalização por si só é independente dos marcadores de cue. Porém, quando os marcadores de intro/outro forem implementados, o cálculo de loudness integrado poderá ser refinado para considerar apenas a região entre `cue_in` e `cue_out` (loudness da parte que realmente vai ao ar, não do silêncio ou da cauda). Isso é um refinamento futuro — na Fase 1 a análise é feita sobre o arquivo completo.

### 12.2 Compatibilidade com processadores externos (Orban, Omnia)

Emissoras que usam processadores de áudio externos (muito comum no Brasil) já aplicam normalização e limitação no sinal analógico. Para essas emissoras, recomenda-se um target mais conservador (ex: −20 LUFS) para não saturar o processador. Isso é configurável pelo operador.

### 12.3 Loudness race e spots comerciais

Spots comerciais frequentemente chegam masterizados em −6 a −8 LUFS para "parecerem mais altos". Com normalização ativa, um SPOT em −6 LUFS com target em −14 receberá `gain_db = −8 dB` — será atenuado em 8 dB, soando no mesmo nível das músicas. Isso pode gerar reclamações de anunciantes acostumados com o comportamento anterior. Recomenda-se comunicar a mudança e, se necessário, configurar um target ligeiramente mais alto para SPOT (ex: −14 em vez de −16).

### 12.4 Monitoramento pós-implantação

Após a implantação, recomenda-se monitorar:
- Número de faixas com `loudness_status = 'error'` (indica problemas de codec ou arquivo corrompido).
- Distribuição de LUFS da biblioteca (histograma) — útil para ajustar o target.
- Feedback de operadores sobre percepção de volume ao vivo.

### 12.5 ffmpeg como ferramenta de análise

O RadioFlow já usa ffmpeg como dependência obrigatória (decoder de áudio). Não há nova dependência a instalar. O mesmo binário usado para decode é usado para análise de loudness. Isso simplifica instalação e distribuição.

---

## Referências

- [RadioBOSS Normalization](https://www.radioboss.fm/support/radioboss-cloud/normalization/)
- [RCS Zetta — New Normalization Methods in Zetta 2.9](https://www.rcsworks.com/new-in-new-normalization-methods-in-zetta-2-9/)
- [mAirList Community — Loudness Normalization](https://community.mairlist.com/t/loudness-normalization/11677)
- [PlayIt Live — Loudness Analysis](https://www.playitsoftware.com/Features/Live/LoudnessAnalysis)
- [RadioBOSS — How to make RadioBOSS play music at the same volume](https://www.djsoft.net/community/threads/how-to-make-radioboss-play-music-at-the-same-volume.6375/)
- [EBU R128 — Loudness Normalisation Standard](https://tech.ebu.ch/publications/r128)
- [FFmpeg — Audio Loudness Normalization with ebur128](https://peterforgacs.github.io/2018/05/20/Audio-normalization-with-ffmpeg/)
- [LUFS e Rádios Brasileiras — Padrão ITU-R BS.1770](https://juniorpinheirovoz.com.br/lufs-e-a-qualidade-do-audio-por-que-as-radios-precisam-respeitar-esse-padrao/)
