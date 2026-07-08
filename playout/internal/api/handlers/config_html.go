package handlers

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func init() {
	logoURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(logoPNG)
	configPage = []byte(strings.ReplaceAll(configPageTpl, "{{LOGO_URI}}", logoURI))
}

// ConfigHTML serves the Configuration SPA at GET /config.
// All data is fetched via REST endpoints; the page degrades gracefully when offline.
func ConfigHTML() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(configPage)
	}
}

var configPage []byte

var configPageTpl = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>RadioCore</title>
  <style>
    :root {
      --bg:        #070807;
      --surface:   rgba(15,18,16,.97);
      --surface2:  rgba(10,12,10,.98);
      --line:      rgba(255,255,255,.08);
      --line-soft: rgba(255,255,255,.045);
      --green:     #00ff80;
      --text:      #e8eee9;
      --muted:     #8a948d;
      --danger:    #ff3b30;
      --warning:   #ff9500;
      --mono:      "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
      --font:      Inter, "Segoe UI", Roboto, Arial, sans-serif;
      --sidebar-w: 160px;
      --header-h:  46px;
      --footer-h:  54px;
      --radius:    8px;
    }
    * { box-sizing: border-box; }
    html, body { height: 100%; margin: 0; padding: 0; overflow: hidden; }
    body {
      display: flex; flex-direction: column;
      background: radial-gradient(circle at 15% 0%, rgba(0,255,128,.06), transparent 35%), var(--bg);
      color: var(--text); font-family: var(--font); font-size: 15px;
    }

    /* Header */
    header {
      height: var(--header-h); flex-shrink: 0;
      display: flex; align-items: center; justify-content: space-between;
      padding: 0 18px; border-bottom: 1px solid var(--line);
      background: linear-gradient(180deg, rgba(12,15,12,.99), rgba(7,8,7,.98));
    }
    .header-logo { width: 50px; height: 50px; border-radius: 6px; flex-shrink: 0; }
    header h1 { margin: 0; font-size: 17px; font-weight: 900; letter-spacing: -.025em; }
    .engine-indicator { display: flex; align-items: center; gap: 7px; }
    .engine-dot {
      width: 8px; height: 8px; border-radius: 50%;
      background: var(--green); box-shadow: 0 0 8px rgba(0,255,128,.7);
      transition: background .4s, box-shadow .4s;
    }
    .engine-dot.offline { background: var(--danger); box-shadow: 0 0 8px rgba(255,59,48,.5); }
    .engine-label { font-size: 13px; font-weight: 700; color: var(--muted); transition: color .3s; }
    .engine-label.offline { color: var(--danger); }

    /* Offline banner */
    #offlineBanner {
      flex-shrink: 0; display: none; padding: 9px 18px;
      background: rgba(255,149,0,.10); border-bottom: 1px solid rgba(255,149,0,.25);
      color: var(--warning); font-size: 14px; font-weight: 700; letter-spacing: .01em;
    }
    #offlineBanner.visible { display: block; }

    /* Layout */
    .layout { flex: 1; display: flex; overflow: hidden; min-height: 0; }

    /* Sidebar */
    .sidebar {
      width: var(--sidebar-w); flex-shrink: 0;
      border-right: 1px solid var(--line); overflow-y: auto;
      padding: 10px 0; background: var(--surface2);
    }
    .nav-item {
      padding: 8px 14px; font-size: 14px; font-weight: 600; color: var(--muted);
      cursor: pointer; border-left: 2px solid transparent;
      transition: color .15s, background .15s, border-color .15s; user-select: none;
    }
    .nav-item:hover { color: var(--text); background: rgba(255,255,255,.03); }
    .nav-item.active { color: var(--green); border-left-color: var(--green); background: rgba(0,255,128,.05); }

    /* Content */
    .content { flex: 1; overflow-y: auto; padding: 22px 28px; min-width: 0; }
    .panel { display: none; max-width: 640px; }
    .panel.active { display: block; }

    .section-title {
      font-size: 13px; font-weight: 900; letter-spacing: .18em;
      text-transform: uppercase; color: var(--muted);
      padding-bottom: 10px; margin-bottom: 20px; border-bottom: 1px solid var(--line);
    }
    .subsection {
      font-size: 12px; font-weight: 900; letter-spacing: .14em;
      text-transform: uppercase; color: var(--muted);
      padding-bottom: 8px; margin: 22px 0 14px; border-bottom: 1px solid var(--line-soft);
    }

    /* Form controls */
    .field { margin-bottom: 18px; }
    .field-row { display: flex; gap: 16px; margin-bottom: 18px; flex-wrap: wrap; }
    .field-row .field { flex: 1; min-width: 120px; margin-bottom: 0; }
    .lbl {
      display: block; font-size: 12px; font-weight: 900; letter-spacing: .10em;
      text-transform: uppercase; color: var(--muted); margin-bottom: 7px;
    }
    .hint { font-size: 12px; color: var(--muted); margin-top: 5px; line-height: 1.45; }
    .warn { font-size: 12px; color: var(--warning); font-weight: 700; margin-top: 5px; }

    input[type="text"], input[type="number"], select {
      width: 100%; background: rgba(255,255,255,.05); border: 1px solid var(--line);
      border-radius: var(--radius); color: var(--text); font-family: var(--font);
      font-size: 14px; padding: 7px 10px; outline: none;
      transition: border-color .15s, background .15s; appearance: none; -webkit-appearance: none;
    }
    input[type="text"]:focus, input[type="number"]:focus, select:focus {
      border-color: rgba(0,255,128,.4); background: rgba(255,255,255,.07);
    }
    input[type="number"] { font-family: var(--mono); }
    input::placeholder { color: var(--muted); opacity: .6; }

    select {
      background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6'%3E%3Cpath d='M0 0l5 6 5-6z' fill='%238a948d'/%3E%3C/svg%3E");
      background-repeat: no-repeat; background-position: right 10px center; padding-right: 28px; cursor: pointer;
    }
    select option { background: #0f120f; color: var(--text); }

    .unit-row { display: flex; align-items: stretch; }
    .unit-row input { border-radius: var(--radius) 0 0 var(--radius); flex: 1; }
    .unit-badge {
      background: rgba(255,255,255,.055); border: 1px solid var(--line); border-left: none;
      border-radius: 0 var(--radius) var(--radius) 0; padding: 7px 9px;
      font-size: 12px; font-weight: 700; color: var(--muted); white-space: nowrap;
      font-family: var(--mono); display: flex; align-items: center;
    }

    .check-row {
      display: flex; align-items: flex-start; gap: 9px;
      margin-bottom: 12px; cursor: pointer; user-select: none;
    }
    .check-row input[type="checkbox"] {
      width: 15px; height: 15px; accent-color: var(--green); cursor: pointer; margin-top: 2px; flex-shrink: 0;
    }
    .check-lbl { font-size: 14px; font-weight: 600; line-height: 1.3; }
    .check-desc { font-size: 12px; color: var(--muted); margin-top: 3px; line-height: 1.4; }

    .radio-group { display: flex; flex-wrap: wrap; gap: 7px; }
    .radio-pill {
      display: flex; align-items: center; gap: 6px; padding: 6px 12px;
      border: 1px solid var(--line); border-radius: 999px;
      cursor: pointer; font-size: 13px; font-weight: 700; color: var(--muted);
      transition: border-color .15s, color .15s, background .15s; user-select: none;
    }
    .radio-pill:hover { border-color: rgba(255,255,255,.18); color: var(--text); }
    .radio-pill.selected { border-color: rgba(0,255,128,.4); color: var(--green); background: rgba(0,255,128,.07); }
    .radio-pill input[type="radio"] { accent-color: var(--green); cursor: pointer; }

    .picker-row { display: flex; gap: 8px; align-items: center; }
    .picker-row input { flex: 1; font-family: var(--mono); font-size: 13px; }
    .btn-browse {
      flex-shrink: 0; white-space: nowrap; padding: 7px 12px;
      background: rgba(255,255,255,.06); border: 1px solid var(--line);
      border-radius: var(--radius); color: var(--text);
      font-size: 13px; font-weight: 700; cursor: pointer; font-family: var(--font);
      transition: background .15s, border-color .15s;
    }
    .btn-browse:hover { background: rgba(255,255,255,.11); border-color: rgba(255,255,255,.16); }

    .list-box {
      border: 1px solid var(--line); border-radius: var(--radius);
      overflow: hidden; min-height: 58px; margin-bottom: 8px;
    }
    .list-empty { padding: 14px; color: var(--muted); font-size: 13px; font-style: italic; text-align: center; }
    .list-item {
      padding: 7px 11px; font-size: 13px; font-family: var(--mono);
      border-bottom: 1px solid var(--line-soft); cursor: pointer; transition: background .1s;
    }
    .list-item:last-child { border-bottom: none; }
    .list-item:hover { background: rgba(255,255,255,.04); }
    .list-item.selected { background: rgba(0,255,128,.08); color: var(--green); }
    .list-actions { display: flex; gap: 8px; }
    .btn-list {
      padding: 5px 11px; background: rgba(255,255,255,.05); border: 1px solid var(--line);
      border-radius: var(--radius); color: var(--text);
      font-size: 13px; font-weight: 700; cursor: pointer; font-family: var(--font);
      transition: background .15s;
    }
    .btn-list:hover { background: rgba(255,255,255,.10); }
    .btn-list.danger:hover { background: rgba(255,59,48,.12); border-color: rgba(255,59,48,.3); color: var(--danger); }

    /* Footer */
    footer {
      height: var(--footer-h); flex-shrink: 0;
      display: flex; align-items: center; justify-content: flex-end; gap: 10px;
      padding: 0 20px; border-top: 1px solid var(--line);
      background: linear-gradient(0deg, rgba(12,15,12,.99), rgba(7,8,7,.98));
    }
    .footer-hint { flex: 1; font-size: 12px; color: var(--muted); font-style: italic; }
    .btn-cancel {
      padding: 8px 18px; background: transparent; border: 1px solid var(--line);
      border-radius: var(--radius); color: var(--muted);
      font-size: 14px; font-weight: 700; cursor: pointer; font-family: var(--font);
      transition: border-color .15s, color .15s;
    }
    .btn-cancel:hover { border-color: rgba(255,255,255,.2); color: var(--text); }
    .btn-save {
      padding: 8px 24px; background: rgba(0,255,128,.12); border: 1px solid rgba(0,255,128,.35);
      border-radius: var(--radius); color: var(--green);
      font-size: 14px; font-weight: 900; letter-spacing: .02em;
      cursor: pointer; font-family: var(--font);
      transition: background .15s, border-color .15s, opacity .2s, box-shadow .2s;
    }
    .btn-save:hover:not(:disabled) {
      background: rgba(0,255,128,.22); border-color: rgba(0,255,128,.55);
      box-shadow: 0 0 12px rgba(0,255,128,.15);
    }
    .btn-save:disabled { opacity: .3; cursor: not-allowed; pointer-events: none; }

    /* Save banner */
    #saveBanner {
      position: fixed; bottom: calc(var(--footer-h) + 14px);
      left: 50%; transform: translateX(-50%) translateY(14px);
      background: rgba(0,255,128,.11); border: 1px solid rgba(0,255,128,.3);
      border-radius: var(--radius); padding: 12px 22px;
      font-size: 14px; font-weight: 700; color: var(--green);
      opacity: 0; transition: opacity .3s, transform .3s;
      pointer-events: none; white-space: nowrap; z-index: 200;
      box-shadow: 0 4px 24px rgba(0,0,0,.4);
    }
    #saveBanner.visible { opacity: 1; transform: translateX(-50%) translateY(0); }

    /* Discard confirmation bar */
    #discardBar {
      flex-shrink: 0; display: none;
      padding: 9px 18px; gap: 12px;
      align-items: center;
      background: rgba(255,149,0,.10); border-bottom: 1px solid rgba(255,149,0,.25);
      color: var(--warning); font-size: 14px; font-weight: 700;
    }
    #discardBar.visible { display: flex; }
    .btn-discard-confirm {
      padding: 4px 14px;
      background: rgba(255,149,0,.18); border: 1px solid rgba(255,149,0,.4);
      border-radius: var(--radius); color: var(--warning);
      font-size: 13px; font-weight: 900; cursor: pointer; font-family: var(--font);
      transition: background .15s;
    }
    .btn-discard-confirm:hover { background: rgba(255,149,0,.32); }
    .btn-discard-cancel {
      padding: 4px 14px;
      background: transparent; border: 1px solid var(--line);
      border-radius: var(--radius); color: var(--muted);
      font-size: 13px; font-weight: 700; cursor: pointer; font-family: var(--font);
      transition: background .15s, color .15s;
    }
    .btn-discard-cancel:hover { background: rgba(255,255,255,.06); color: var(--text); }

    /* Error banner */
    #errorBanner {
      position: fixed; bottom: calc(var(--footer-h) + 14px);
      left: 50%; transform: translateX(-50%) translateY(14px);
      background: rgba(255,59,48,.11); border: 1px solid rgba(255,59,48,.3);
      border-radius: var(--radius); padding: 12px 22px;
      font-size: 14px; font-weight: 700; color: var(--danger);
      opacity: 0; transition: opacity .3s, transform .3s;
      pointer-events: none; white-space: nowrap; z-index: 200;
      box-shadow: 0 4px 24px rgba(0,0,0,.4); max-width: 480px; text-align: center;
    }
    #errorBanner.visible { opacity: 1; transform: translateX(-50%) translateY(0); }
  </style>
</head>
<body>

<header>
  <div style="display:flex;align-items:center;gap:10px;">
    <img src="{{LOGO_URI}}" alt="RadioCore" class="header-logo">
    <h1>Configuração</h1>
  </div>
  <div class="engine-indicator">
    <div id="engineDot" class="engine-dot"></div>
    <span id="engineLabel" class="engine-label">Online</span>
  </div>
</header>

<div id="offlineBanner">
  &nbsp; Engine offline &#8212; pode estar reiniciando. Aguarde para salvar alterações.
</div>

<div id="discardBar">
  &#9888;&nbsp; Descartar todas as alterações não salvas?
  <button class="btn-discard-confirm" onclick="confirmDiscard()">Sim, descartar</button>
  <button class="btn-discard-cancel"  onclick="hideDiscardBar()">Não</button>
</div>

<div class="layout">
  <nav class="sidebar">
    <div class="nav-item active"  data-s="engine">Engine</div>
    <div class="nav-item"         data-s="api">API</div>
    <div class="nav-item"         data-s="audio">Áudio</div>
    <div class="nav-item"         data-s="playback">Reprodução</div>
    <div class="nav-item"         data-s="health">Saúde</div>
    <div class="nav-item"         data-s="panic">Panic</div>
    <div class="nav-item"         data-s="log">Log / Seg / Admin</div>
    <div class="nav-item"         data-s="queue">Fila</div>
    <div class="nav-item"         data-s="horacerta">Hora Certa</div>
    <div class="nav-item"         data-s="preview">Preview</div>
    <div class="nav-item"         data-s="scheduler">Scheduler</div>
  </nav>

  <main class="content">

    <!-- ENGINE -->
    <div id="p-engine" class="panel active">
      <div class="section-title">Engine</div>
      <div class="field">
        <label class="lbl">ID da instância</label>
        <input id="engine-id" type="text" />
        <div class="hint">Identificador único do engine. Usado no arquivo de snapshot e no lock de instância.</div>
      </div>
      <label class="check-row">
        <input id="engine-instance-lock" type="checkbox" />
        <div>
          <div class="check-lbl">Bloquear instância duplicada</div>
          <div class="check-desc">Impede que uma segunda instância com o mesmo ID seja iniciada simultaneamente.</div>
        </div>
      </label>
    </div>

    <!-- API -->
    <div id="p-api" class="panel">
      <div class="section-title">API</div>
      <div class="field-row">
        <div class="field">
          <label class="lbl">Host</label>
          <input id="api-host" type="text" />
        </div>
        <div class="field" style="max-width:120px">
          <label class="lbl">Porta</label>
          <input id="api-port" type="number" min="1" max="65535" />
        </div>
      </div>
      <label class="check-row">
        <input id="api-cors-enabled" type="checkbox" />
        <div>
          <div class="check-lbl">Habilitar CORS</div>
          <div class="check-desc">Permite requisições cross-origin da UI de controle.</div>
        </div>
      </label>
      <div class="field" style="margin-top:14px">
        <label class="lbl">Origens permitidas</label>
        <div class="list-box" id="cors-box"></div>
        <div class="list-actions">
          <button class="btn-list" onclick="addCors()">+ Adicionar</button>
          <button class="btn-list danger" onclick="removeCors()">&#8722; Remover selecionada</button>
        </div>
      </div>
    </div>

    <!-- ÁUDIO -->
    <div id="p-audio" class="panel">
      <div class="section-title">Áudio</div>
      <div class="field">
        <label class="lbl">Dispositivo de saída principal</label>
        <select id="audio-device"></select>
      </div>
      <label class="check-row">
        <input id="audio-allow-null" type="checkbox" />
        <div>
          <div class="check-lbl">Usar NullOutput se o dispositivo falhar ao abrir</div>
          <div class="check-desc">Degrada graciosamente em vez de encerrar com erro.</div>
        </div>
      </label>
      <div class="field-row" style="margin-top:16px">
        <div class="field">
          <label class="lbl">Taxa de amostragem</label>
          <div class="unit-row"><input id="audio-sample-rate" type="number" /><span class="unit-badge">Hz</span></div>
        </div>
        <div class="field">
          <label class="lbl">Canais</label>
          <input id="audio-channels" type="number" min="1" max="8" />
        </div>
        <div class="field">
          <label class="lbl">Buffer</label>
          <div class="unit-row"><input id="audio-buffer-frames" type="number" /><span class="unit-badge">frames</span></div>
        </div>
      </div>
      <div class="hint">Menor buffer = menor latência. Maior buffer = maior estabilidade em sistemas com carga.</div>
    </div>

    <!-- REPRODUÇÃO -->
    <div id="p-playback" class="panel">
      <div class="section-title">Reprodução</div>
      <div class="field-row">
        <div class="field">
          <label class="lbl">Crossfade padrão</label>
          <div class="unit-row"><input id="pb-crossfade" type="number" min="0" /><span class="unit-badge">ms</span></div>
          <div class="hint">0 = desabilita crossfade automático.</div>
        </div>
        <div class="field">
          <label class="lbl">Fade ao parar</label>
          <div class="unit-row"><input id="pb-stop-fade" type="number" min="0" /><span class="unit-badge">ms</span></div>
        </div>
      </div>
      <div class="field">
        <label class="lbl">Pré-carregamento do próximo item</label>
        <div class="unit-row" style="max-width:200px"><input id="pb-preload" type="number" min="0" /><span class="unit-badge">ms</span></div>
      </div>
      <div class="field">
        <label class="lbl">Falhas consecutivas máximas</label>
        <input id="pb-max-failures" type="number" min="1" max="20" style="max-width:100px" />
      </div>
      <div class="subsection">Auto crossfade por energia</div>
      <label class="check-row">
        <input id="pb-auto-xfade" type="checkbox" onchange="toggleAutoXfade(this.checked)" />
        <div>
          <div class="check-lbl">Habilitado</div>
          <div class="check-desc">Crossfade disparado quando a energia do áudio cair abaixo do threshold por N buffers consecutivos.</div>
        </div>
      </label>
      <div id="auto-xfade-fields">
        <div class="field-row">
          <div class="field">
            <label class="lbl">Threshold de energia</label>
            <div class="unit-row"><input id="pb-energy-thresh" type="number" step="0.5" /><span class="unit-badge">dBFS</span></div>
          </div>
          <div class="field">
            <label class="lbl">Janela mínima</label>
            <div class="unit-row"><input id="pb-min-end" type="number" min="0" /><span class="unit-badge">ms</span></div>
          </div>
          <div class="field">
            <label class="lbl">Janela máxima</label>
            <div class="unit-row"><input id="pb-max-end" type="number" min="0" /><span class="unit-badge">ms</span></div>
          </div>
        </div>
        <div class="field">
          <label class="lbl">Buffers consecutivos para confirmar</label>
          <input id="pb-hold-frames" type="number" min="1" style="max-width:100px" />
        </div>
      </div>
    </div>

    <!-- SAÚDE -->
    <div id="p-health" class="panel">
      <div class="section-title">Saúde do Áudio</div>
      <div class="field-row">
        <div class="field">
          <label class="lbl">Intervalo de progresso</label>
          <div class="unit-row"><input id="health-progress-ms" type="number" min="100" /><span class="unit-badge">ms</span></div>
          <div class="hint">Frequência do evento ProgressChanged.</div>
        </div>
        <div class="field">
          <label class="lbl">Intervalo de saúde</label>
          <div class="unit-row"><input id="health-interval-ms" type="number" min="100" /><span class="unit-badge">ms</span></div>
          <div class="hint">Frequência do monitor de RMS, pico e silêncio.</div>
        </div>
      </div>
      <div class="field-row">
        <div class="field">
          <label class="lbl">Threshold de silêncio</label>
          <div class="unit-row"><input id="health-silence-thresh" type="number" step="1" /><span class="unit-badge">dBFS</span></div>
        </div>
        <div class="field">
          <label class="lbl">Duração de silêncio</label>
          <div class="unit-row"><input id="health-silence-ms" type="number" min="100" /><span class="unit-badge">ms</span></div>
        </div>
      </div>
      <div class="subsection">VU Meter</div>
      <label class="check-row">
        <input id="health-vu-enabled" type="checkbox" />
        <div>
          <div class="check-lbl">Habilitado</div>
          <div class="check-desc">Publica eventos VUMeter via WebSocket com RMS, Peak Hold, LUFS momentâneo e separação L/R.</div>
        </div>
      </label>
      <div class="field-row" style="margin-top:10px">
        <div class="field">
          <label class="lbl">Intervalo VU Meter</label>
          <div class="unit-row"><input id="health-vu-ms" type="number" min="50" /><span class="unit-badge">ms</span></div>
        </div>
        <div class="field">
          <label class="lbl">Peak hold</label>
          <div class="unit-row"><input id="health-peak-hold" type="number" min="0" /><span class="unit-badge">ms</span></div>
        </div>
      </div>
    </div>

    <!-- PANIC -->
    <div id="p-panic" class="panel">
      <div class="section-title">Panic</div>
      <label class="check-row">
        <input id="panic-enabled" type="checkbox" />
        <div>
          <div class="check-lbl">Modo Panic habilitado</div>
          <div class="check-desc">Permite entrar em PANIC via comando ou automaticamente por silêncio detectado.</div>
        </div>
      </label>
      <div class="field" style="margin-top:14px">
        <label class="lbl">Arquivo de cama (panic bed)</label>
        <div class="picker-row">
          <input id="panic-bed" type="text" />
          <button class="btn-browse" onclick="browse('panic-bed','file')">Procurar</button>
        </div>
        <div class="hint">Áudio tocado em loop enquanto o engine estiver em modo PANIC.</div>
      </div>
      <label class="check-row">
        <input id="panic-auto" type="checkbox" />
        <div>
          <div class="check-lbl">Auto-panic por silêncio</div>
          <div class="check-desc">Entra em PANIC automaticamente ao detectar silêncio sustentado.</div>
        </div>
      </label>
      <div class="field-row" style="margin-top:6px">
        <div class="field">
          <label class="lbl">Threshold de silêncio</label>
          <div class="unit-row"><input id="panic-silence-thresh" type="number" step="1" /><span class="unit-badge">dBFS</span></div>
        </div>
        <div class="field">
          <label class="lbl">Duração mínima</label>
          <div class="unit-row"><input id="panic-silence-ms" type="number" min="100" /><span class="unit-badge">ms</span></div>
        </div>
      </div>
    </div>

    <!-- LOG / SEG / ADMIN -->
    <div id="p-log" class="panel">
      <div class="section-title">Logging / Segurança / Admin</div>
      <div class="subsection" style="margin-top:0">Logging</div>
      <div class="field">
        <label class="lbl">Nível</label>
        <div class="radio-group" data-name="log-level">
          <label class="radio-pill"><input type="radio" name="log-level" value="error" /> error</label>
          <label class="radio-pill"><input type="radio" name="log-level" value="warn"  /> warn</label>
          <label class="radio-pill"><input type="radio" name="log-level" value="info"  /> info</label>
          <label class="radio-pill"><input type="radio" name="log-level" value="debug" /> debug</label>
        </div>
      </div>
      <div class="field">
        <label class="lbl">Formato</label>
        <div class="radio-group" data-name="log-format">
          <label class="radio-pill"><input type="radio" name="log-format" value="text" /> text &#8212; legível por humanos</label>
          <label class="radio-pill"><input type="radio" name="log-format" value="json" /> json &#8212; estruturado (Loki, Datadog)</label>
        </div>
      </div>
      <div class="subsection">Segurança</div>
      <div class="field">
        <label class="lbl">Diretórios de áudio permitidos</label>
        <div class="list-box" id="roots-box"></div>
        <div class="list-actions">
          <button class="btn-list" onclick="addRoot()">+ Adicionar pasta</button>
          <button class="btn-list danger" onclick="removeRoot()">&#8722; Remover selecionada</button>
        </div>
        <div class="warn">&#9888; Deixar vazio não é recomendado em produção — qualquer path será aceito.</div>
      </div>
      <div class="subsection">Admin</div>
      <label class="check-row">
        <input id="admin-shutdown" type="checkbox" />
        <div>
          <div class="check-lbl">Habilitar shutdown remoto</div>
          <div class="check-desc">Expõe POST /v1/admin/shutdown. Não habilitar em produção exposta à rede.</div>
        </div>
      </label>
    </div>

    <!-- FILA -->
    <div id="p-queue" class="panel">
      <div class="section-title">Fila</div>
      <div class="subsection" style="margin-top:0">Persistência</div>
      <label class="check-row">
        <input id="queue-persist" type="checkbox" />
        <div>
          <div class="check-lbl">Salvar e restaurar fila entre reinicializações</div>
          <div class="check-desc">O estado da fila é persistido a cada mutação e restaurado na inicialização.</div>
        </div>
      </label>
      <div class="field" style="margin-top:14px">
        <label class="lbl">Arquivo de snapshot</label>
        <div class="picker-row">
          <input id="queue-path" type="text" placeholder="/tmp/playout-&lt;engine-id&gt;-queue.json" />
          <button class="btn-browse" onclick="browse('queue-path','file')">Procurar</button>
        </div>
        <div class="hint">Vazio = /tmp/playout-&lt;engine-id&gt;-queue.json</div>
      </div>
      <label class="check-row">
        <input id="queue-restore" type="checkbox" />
        <div>
          <div class="check-lbl">Restaurar fila ao iniciar</div>
          <div class="check-desc">Recarrega os itens da fila a partir do snapshot na inicialização.</div>
        </div>
      </label>
      <label class="check-row">
        <input id="queue-clear" type="checkbox" />
        <div>
          <div class="check-lbl">Apagar snapshot ao encerrar normalmente</div>
          <div class="check-desc">Persistir apenas em caso de crash. Reinicializações normais começam com fila vazia.</div>
        </div>
      </label>
    </div>

    <!-- HORA CERTA -->
    <div id="p-horacerta" class="panel">
      <div class="section-title">Hora Certa</div>
      <div class="field">
        <label class="lbl">Pasta de horas</label>
        <div class="picker-row">
          <input id="hc-hours-dir" type="text" />
          <button class="btn-browse" onclick="browse('hc-hours-dir','dir')">Procurar pasta</button>
        </div>
        <div class="hint">Contém os arquivos HRS00.mp3 … HRS23.mp3</div>
      </div>
      <div class="field">
        <label class="lbl">Pasta de minutos</label>
        <div class="picker-row">
          <input id="hc-minutes-dir" type="text" />
          <button class="btn-browse" onclick="browse('hc-minutes-dir','dir')">Procurar pasta</button>
        </div>
        <div class="hint">Contém os arquivos MIN00.mp3 … MIN59.mp3. Pode ser a mesma pasta que horas.</div>
      </div>
      <div class="field-row">
        <div class="field">
          <label class="lbl">Padrão — hora</label>
          <input id="hc-hour-pattern" type="text" />
          <div class="hint">{HH} = hora com zero à esquerda (00–23)</div>
        </div>
        <div class="field">
          <label class="lbl">Padrão — minuto</label>
          <input id="hc-minute-pattern" type="text" />
          <div class="hint">{MM} = minuto com zero à esquerda (00–59)</div>
        </div>
      </div>
      <div class="field">
        <label class="lbl">Ganho padrão</label>
        <div class="unit-row" style="max-width:130px">
          <input id="hc-gain" type="number" step="0.5" />
          <span class="unit-badge">dB</span>
        </div>
        <div class="hint">0 = unity gain.</div>
      </div>
    </div>

    <!-- PREVIEW -->
    <div id="p-preview" class="panel">
      <div class="section-title">Preview (Cue)</div>
      <label class="check-row">
        <input id="prev-enabled" type="checkbox" />
        <div>
          <div class="check-lbl">Habilitar preview de áudio</div>
          <div class="check-desc">Permite ouvir um áudio em dispositivo separado antes de colocá-lo na fila, sem interferir no sinal ao ar.</div>
        </div>
      </label>
      <div class="field" style="margin-top:16px">
        <label class="lbl">Dispositivo de preview (cue)</label>
        <select id="prev-device"></select>
        <div class="hint">Deve ser diferente do dispositivo de saída principal. Vazio = padrão do driver.</div>
      </div>
    </div>

    <!-- SCHEDULER -->
    <div id="p-scheduler" class="panel">
      <div class="section-title">Scheduler</div>
      <label class="check-row">
        <input id="sched-enabled" type="checkbox" />
        <div>
          <div class="check-lbl">Habilitar scheduler de programação horária</div>
          <div class="check-desc">Quando desabilitado, nenhuma entrada é avaliada mesmo que esteja registrada.</div>
        </div>
      </label>
      <div class="field" style="margin-top:16px">
        <label class="lbl">Timezone</label>
        <input id="sched-tz" type="text" placeholder="America/Sao_Paulo" />
        <div class="hint">Timezone IANA. Vazio = timezone do sistema operacional.</div>
      </div>
      <div class="field">
        <label class="lbl">Arquivo de schedule</label>
        <div class="picker-row">
          <input id="sched-path" type="text" placeholder="~/RadioFlow/schedule.json" />
          <button class="btn-browse" onclick="browse('sched-path','file')">Procurar</button>
        </div>
      </div>
      <div class="field">
        <label class="lbl">Tolerância de atraso (missed threshold)</label>
        <div class="unit-row" style="max-width:180px">
          <input id="sched-missed" type="number" min="0" />
          <span class="unit-badge">ms</span>
        </div>
        <div class="hint">Entradas atrasadas além desse tempo são marcadas MISSED em vez de disparar com atraso.</div>
      </div>
    </div>

  </main>
</div>

<footer>
  <span class="footer-hint">As alterações só terão efeito após reiniciar o RadioCore pelo menu do systray.</span>
  <button class="btn-cancel" onclick="cancelCfg()">Descartar Alterações</button>
  <button class="btn-save" id="saveBtn" onclick="saveCfg()">Salvar</button>
</footer>

<div id="saveBanner">&#10003; Configuração salva. Reinicie o RadioCore pelo menu do systray.</div>
<div id="errorBanner"></div>

<script>
  // ── State ────────────────────────────────────────────────────
  var engineOnline  = true;
  var saveBannerTimer = null;
  var errorBannerTimer = null;
  var corsOrigins   = [];
  var corsSelected  = -1;
  var secRoots      = [];
  var rootSelected  = -1;
  var audioDeviceID = '';
  var prevDeviceID  = '';

  // ── Navigation ───────────────────────────────────────────────
  document.querySelectorAll('.nav-item').forEach(function(item) {
    item.addEventListener('click', function() {
      document.querySelectorAll('.nav-item').forEach(function(n) { n.classList.remove('active'); });
      document.querySelectorAll('.panel').forEach(function(p) { p.classList.remove('active'); });
      item.classList.add('active');
      document.getElementById('p-' + item.dataset.s).classList.add('active');
    });
  });

  // ── Radio pills ──────────────────────────────────────────────
  document.querySelectorAll('.radio-group').forEach(function(group) {
    group.querySelectorAll('.radio-pill').forEach(function(pill) {
      pill.addEventListener('click', function() {
        group.querySelectorAll('.radio-pill').forEach(function(p) { p.classList.remove('selected'); });
        pill.classList.add('selected');
      });
    });
  });

  // ── Auto-crossfade dim ───────────────────────────────────────
  function toggleAutoXfade(on) {
    var fields = document.getElementById('auto-xfade-fields');
    fields.style.opacity = on ? '1' : '0.35';
    fields.style.pointerEvents = on ? '' : 'none';
  }

  // ── Engine health polling ────────────────────────────────────
  function setEngine(online) {
    engineOnline = online;
    var dot    = document.getElementById('engineDot');
    var label  = document.getElementById('engineLabel');
    var banner = document.getElementById('offlineBanner');
    var btn    = document.getElementById('saveBtn');
    dot.classList.toggle('offline', !online);
    label.classList.toggle('offline', !online);
    label.textContent = online ? 'Online' : 'Offline';
    banner.classList.toggle('visible', !online);
    btn.disabled = !online;
  }

  function startHealthPolling() {
    function poll() {
      fetch('/v1/health')
        .then(function(r) { setEngine(r.ok); })
        .catch(function() { setEngine(false); });
      setTimeout(poll, 5000);
    }
    poll();
  }

  // ── OS detection ─────────────────────────────────────────────
  function applyOSRules(os) {
    var isDarwin = os && os.indexOf('darwin') === 0;
    ['pill-audio-coreaudio', 'pill-prev-coreaudio'].forEach(function(id) {
      var pill = document.getElementById(id);
      if (!pill) return;
      pill.style.display = isDarwin ? '' : 'none';
      if (!isDarwin) {
        var radio = pill.querySelector('input[type="radio"]');
        if (radio && radio.checked) {
          radio.checked = false;
          pill.classList.remove('selected');
          var group = pill.closest('.radio-group');
          if (group) {
            var first = group.querySelector('.radio-pill');
            if (first) {
              first.classList.add('selected');
              var fr = first.querySelector('input[type="radio"]');
              if (fr) fr.checked = true;
            }
          }
        }
      }
    });
  }

  // ── Helpers ──────────────────────────────────────────────────
  function setVal(id, v) {
    var el = document.getElementById(id);
    if (el && v !== undefined && v !== null) el.value = v;
  }
  function setCheck(id, v) {
    var el = document.getElementById(id);
    if (el) el.checked = !!v;
  }
  function setRadio(name, v) {
    if (v === undefined || v === null) return;
    var group = document.querySelector('.radio-group[data-name="' + name + '"]');
    if (!group) return;
    group.querySelectorAll('.radio-pill').forEach(function(pill) {
      var radio = pill.querySelector('input[type="radio"]');
      var match = radio && radio.value === v;
      pill.classList.toggle('selected', match);
      if (radio) radio.checked = match;
    });
  }
  function getVal(id) {
    var el = document.getElementById(id);
    return el ? el.value : '';
  }
  function getCheck(id) {
    var el = document.getElementById(id);
    return el ? el.checked : false;
  }
  function getRadio(name) {
    var group = document.querySelector('.radio-group[data-name="' + name + '"]');
    if (!group) return '';
    var checked = group.querySelector('input[type="radio"]:checked');
    return checked ? checked.value : '';
  }
  function safeInt(v, def) { var n = parseInt(v, 10); return isNaN(n) ? def : n; }
  function safeFloat(v, def) { var n = parseFloat(v); return isNaN(n) ? def : n; }

  // ── Load info (OS detection) ──────────────────────────────────
  function loadInfo() {
    return fetch('/v1/info')
      .then(function(r) { return r.json(); })
      .then(function(d) { applyOSRules(d.os || ''); })
      .catch(function() {});
  }

  // ── Load config ──────────────────────────────────────────────
  function loadConfig() {
    return fetch('/v1/config/current')
      .then(function(r) { if (!r.ok) throw new Error('config unavailable'); return r.json(); })
      .then(function(cfg) { populateForm(cfg); })
      .catch(function() {});
  }

  function populateForm(cfg) {
    var e = cfg.engine || {};
    setVal('engine-id', e.id);
    setCheck('engine-instance-lock', e.instance_lock);

    var a = cfg.api || {};
    setVal('api-host', a.host);
    setVal('api-port', a.port);
    setCheck('api-cors-enabled', a.cors && a.cors.enabled);
    corsOrigins = (a.cors && a.cors.allowed_origins) || [];
    corsSelected = -1;
    renderCors();

    var au = cfg.audio || {};
    var auo = au.output || {};
    audioDeviceID = auo.device_id || '';
    setCheck('audio-allow-null', auo.allow_null_output);
    setVal('audio-sample-rate', au.sample_rate);
    setVal('audio-channels', au.channels);
    setVal('audio-buffer-frames', au.buffer_frames);

    var pb = cfg.playback || {};
    setVal('pb-crossfade', pb.default_crossfade_ms);
    setVal('pb-stop-fade', pb.default_stop_fade_ms);
    setVal('pb-preload', pb.preload_next_ms);
    setVal('pb-max-failures', pb.max_consecutive_item_failures);
    setCheck('pb-auto-xfade', pb.auto_crossfade_enabled);
    toggleAutoXfade(!!pb.auto_crossfade_enabled);
    setVal('pb-energy-thresh', pb.auto_crossfade_energy_threshold_dbfs);
    setVal('pb-min-end', pb.auto_crossfade_min_before_end_ms);
    setVal('pb-max-end', pb.auto_crossfade_max_before_end_ms);
    setVal('pb-hold-frames', pb.auto_crossfade_hold_frames);

    var h = cfg.health || {};
    setVal('health-progress-ms', h.progress_interval_ms);
    setVal('health-interval-ms', h.audio_health_interval_ms);
    setVal('health-silence-thresh', h.silence_threshold_dbfs);
    setVal('health-silence-ms', h.silence_duration_ms);
    setCheck('health-vu-enabled', h.vu_meter_enabled);
    setVal('health-vu-ms', h.vu_meter_interval_ms);
    setVal('health-peak-hold', h.peak_hold_ms);

    var p = cfg.panic || {};
    setCheck('panic-enabled', p.enabled);
    setVal('panic-bed', p.bed_path);
    setCheck('panic-auto', p.auto_on_silence);
    setVal('panic-silence-thresh', p.silence_threshold_dbfs);
    setVal('panic-silence-ms', p.silence_duration_ms);

    var l = cfg.logging || {};
    setRadio('log-level', l.level);
    setRadio('log-format', l.format);

    secRoots = (cfg.security && cfg.security.allowed_roots) || [];
    rootSelected = -1;
    renderRoots();

    setCheck('admin-shutdown', cfg.admin && cfg.admin.shutdown_enabled);

    var qp = (cfg.queue && cfg.queue.persistence) || {};
    setCheck('queue-persist', qp.enabled);
    setVal('queue-path', qp.path);
    setCheck('queue-restore', qp.restore_on_start);
    setCheck('queue-clear', qp.clear_on_stop);

    var hc = cfg.hora_certa || {};
    setVal('hc-hours-dir', hc.hours_dir);
    setVal('hc-minutes-dir', hc.minutes_dir);
    setVal('hc-hour-pattern', hc.hour_pattern);
    setVal('hc-minute-pattern', hc.minute_pattern);
    setVal('hc-gain', hc.gain_db);

    var pv = cfg.preview || {};
    setCheck('prev-enabled', pv.enabled);
    prevDeviceID = pv.output_device || '';

    var sc = cfg.scheduler || {};
    setCheck('sched-enabled', sc.enabled);
    setVal('sched-tz', sc.timezone);
    setVal('sched-path', sc.store_path);
    setVal('sched-missed', sc.missed_threshold_ms);
  }

  // ── Load devices ─────────────────────────────────────────────
  function loadDevices() {
    return fetch('/v1/devices')
      .then(function(r) { return r.json(); })
      .then(function(resp) {
        var devs = Array.isArray(resp) ? resp : (Array.isArray(resp.devices) ? resp.devices : []);
        populateDeviceSelect('audio-device', devs, audioDeviceID);
        populateDeviceSelect('prev-device', devs, prevDeviceID);
      })
      .catch(function() {});
  }

  function populateDeviceSelect(id, devs, selectedID) {
    var sel = document.getElementById(id);
    if (!sel) return;
    sel.innerHTML = '<option value="">&#8212; padrão do driver &#8212;</option>';
    devs.forEach(function(d) {
      var opt = document.createElement('option');
      opt.value = d.id || d.name || '';
      opt.textContent = d.name + (d.is_default ? ' (padrão)' : '');
      if (opt.value && (opt.value === selectedID || d.name === selectedID)) opt.selected = true;
      sel.appendChild(opt);
    });
  }

  // ── Collect form → config object ─────────────────────────────
  function collectForm() {
    return {
      engine: {
        id: getVal('engine-id'),
        instance_lock: getCheck('engine-instance-lock')
      },
      api: {
        host: getVal('api-host'),
        port: safeInt(getVal('api-port'), 8080),
        cors: {
          enabled: getCheck('api-cors-enabled'),
          allowed_origins: corsOrigins.slice()
        }
      },
      audio: {
        sample_rate:   safeInt(getVal('audio-sample-rate'), 48000),
        channels:      safeInt(getVal('audio-channels'), 2),
        buffer_frames: safeInt(getVal('audio-buffer-frames'), 2048),
        output: {
          device_id:        getVal('audio-device') || 'default',
          allow_null_output: getCheck('audio-allow-null')
        }
      },
      playback: {
        default_crossfade_ms:                safeInt(getVal('pb-crossfade'), 0),
        default_stop_fade_ms:                safeInt(getVal('pb-stop-fade'), 0),
        preload_next_ms:                     safeInt(getVal('pb-preload'), 0),
        max_consecutive_item_failures:       safeInt(getVal('pb-max-failures'), 3),
        auto_crossfade_enabled:              getCheck('pb-auto-xfade'),
        auto_crossfade_energy_threshold_dbfs: safeFloat(getVal('pb-energy-thresh'), -18),
        auto_crossfade_min_before_end_ms:    safeInt(getVal('pb-min-end'), 0),
        auto_crossfade_max_before_end_ms:    safeInt(getVal('pb-max-end'), 0),
        auto_crossfade_hold_frames:          safeInt(getVal('pb-hold-frames'), 0)
      },
      health: {
        progress_interval_ms:    safeInt(getVal('health-progress-ms'), 500),
        audio_health_interval_ms: safeInt(getVal('health-interval-ms'), 500),
        silence_threshold_dbfs:  safeFloat(getVal('health-silence-thresh'), -60),
        silence_duration_ms:     safeInt(getVal('health-silence-ms'), 2000),
        vu_meter_enabled:        getCheck('health-vu-enabled'),
        vu_meter_interval_ms:    safeInt(getVal('health-vu-ms'), 100),
        peak_hold_ms:            safeInt(getVal('health-peak-hold'), 3000)
      },
      panic: {
        enabled:               getCheck('panic-enabled'),
        bed_path:              getVal('panic-bed'),
        auto_on_silence:       getCheck('panic-auto'),
        silence_threshold_dbfs: safeFloat(getVal('panic-silence-thresh'), -60),
        silence_duration_ms:   safeInt(getVal('panic-silence-ms'), 2000)
      },
      logging: {
        level:  getRadio('log-level')  || 'info',
        format: getRadio('log-format') || 'json'
      },
      security: {
        allowed_roots: secRoots.slice()
      },
      admin: {
        shutdown_enabled: getCheck('admin-shutdown')
      },
      queue: {
        persistence: {
          enabled:         getCheck('queue-persist'),
          path:            getVal('queue-path'),
          restore_on_start: getCheck('queue-restore'),
          clear_on_stop:   getCheck('queue-clear')
        }
      },
      hora_certa: {
        hours_dir:      getVal('hc-hours-dir'),
        minutes_dir:    getVal('hc-minutes-dir'),
        hour_pattern:   getVal('hc-hour-pattern'),
        minute_pattern: getVal('hc-minute-pattern'),
        gain_db:        safeFloat(getVal('hc-gain'), 0)
      },
      preview: {
        enabled:       getCheck('prev-enabled'),
        output_device: getVal('prev-device')
      },
      scheduler: {
        enabled:            getCheck('sched-enabled'),
        timezone:           getVal('sched-tz'),
        store_path:         getVal('sched-path'),
        missed_threshold_ms: safeInt(getVal('sched-missed'), 5000)
      }
    };
  }

  // ── CORS list ────────────────────────────────────────────────
  function renderCors() {
    var box = document.getElementById('cors-box');
    if (!corsOrigins.length) {
      box.innerHTML = '<div class="list-empty">Nenhuma origem configurada.</div>';
      return;
    }
    box.innerHTML = corsOrigins.map(function(o, i) {
      return '<div class="list-item' + (i === corsSelected ? ' selected' : '') + '" onclick="selectCors(' + i + ')">' + o + '</div>';
    }).join('');
  }
  function selectCors(i) { corsSelected = corsSelected === i ? -1 : i; renderCors(); }
  function addCors() {
    var v = prompt('Nova origem (ex: http://meusite.local:3000):');
    if (v && v.trim()) { corsOrigins.push(v.trim()); corsSelected = -1; renderCors(); }
  }
  function removeCors() {
    if (corsSelected < 0) { alert('Selecione uma origem para remover.'); return; }
    corsOrigins.splice(corsSelected, 1); corsSelected = -1; renderCors();
  }

  // ── Security roots list ──────────────────────────────────────
  function renderRoots() {
    var box = document.getElementById('roots-box');
    if (!secRoots.length) {
      box.innerHTML = '<div class="list-empty">Sem restrição de paths — qualquer diretório permitido.</div>';
      return;
    }
    box.innerHTML = secRoots.map(function(r, i) {
      return '<div class="list-item' + (i === rootSelected ? ' selected' : '') + '" onclick="selectRoot(' + i + ')">' + r + '</div>';
    }).join('');
  }
  function selectRoot(i) { rootSelected = rootSelected === i ? -1 : i; renderRoots(); }
  function addRoot() {
    fetch('/v1/config/browse', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: 'dir' })
    })
    .then(function(r) { return r.json(); })
    .then(function(d) {
      if (d.path && d.path.trim()) {
        secRoots.push(d.path.trim());
        rootSelected = -1;
        renderRoots();
      }
    })
    .catch(function() {});
  }
  function removeRoot() {
    if (rootSelected < 0) { alert('Selecione um diretório para remover.'); return; }
    secRoots.splice(rootSelected, 1); rootSelected = -1; renderRoots();
  }

  // ── File picker ──────────────────────────────────────────────
  function browse(fieldId, type) {
    fetch('/v1/config/browse', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: type })
    })
    .then(function(r) { return r.json(); })
    .then(function(d) {
      if (d.path) document.getElementById(fieldId).value = d.path;
    })
    .catch(function() {});
  }

  // ── Save ─────────────────────────────────────────────────────
  function saveCfg() {
    if (!engineOnline) return;
    fetch('/v1/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(collectForm())
    })
    .then(function(r) { return r.json().then(function(d) { return { ok: r.ok, body: d }; }); })
    .then(function(res) {
      if (res.ok) {
        showBanner('save');
      } else {
        showBanner('error', res.body.message || 'Erro ao salvar configuração.');
      }
    })
    .catch(function(err) { showBanner('error', 'Falha na requisição: ' + err.message); });
  }

  function showBanner(type, msg) {
    if (type === 'save') {
      var b = document.getElementById('saveBanner');
      b.classList.add('visible');
      if (saveBannerTimer) clearTimeout(saveBannerTimer);
      saveBannerTimer = setTimeout(function() { b.classList.remove('visible'); }, 6000);
    } else {
      var eb = document.getElementById('errorBanner');
      eb.textContent = msg || 'Erro desconhecido.';
      eb.classList.add('visible');
      if (errorBannerTimer) clearTimeout(errorBannerTimer);
      errorBannerTimer = setTimeout(function() { eb.classList.remove('visible'); }, 7000);
    }
  }

  // ── Cancel ───────────────────────────────────────────────────
  function cancelCfg() {
    document.getElementById('discardBar').classList.add('visible');
  }
  function hideDiscardBar() {
    document.getElementById('discardBar').classList.remove('visible');
  }
  function confirmDiscard() {
    hideDiscardBar();
    loadConfig().then(loadDevices);
  }

  // ── Init ─────────────────────────────────────────────────────
  loadInfo().then(function() {
    return Promise.all([loadConfig(), loadDevices()]);
  }).then(function() {
    startHealthPolling();
  });
</script>
</body>
</html>
`
