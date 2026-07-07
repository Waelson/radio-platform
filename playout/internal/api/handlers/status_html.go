package handlers

import (
	"net/http"
	"time"

	"github.com/Waelson/radio-playout-engine/internal/state"
)

// StatusHTML returns a handler for GET /status that serves a client-side SPA.
// All data is fetched by JavaScript via /v1/info, /v1/health and /v1/status,
// so the page remains functional (showing "offline") even when the engine restarts.
func StatusHTML(_ int, _ string, _ time.Time, _ *state.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(statusPage)
	}
}

var statusPage = []byte(`<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>RadioCore — Status</title>
  <style>
    :root {
      --bg: #070807;
      --line: rgba(255,255,255,.08);
      --line-soft: rgba(255,255,255,.055);
      --green: #00ff80;
      --text: #e8eee9;
      --muted: #8a948d;
      --mono: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
      --font: Inter, "Segoe UI", Roboto, Arial, sans-serif;
    }
    * { box-sizing: border-box; }
    html, body { margin: 0; padding: 0; height: 100%; overflow: hidden; }
    body {
      background:
        radial-gradient(circle at 45% 0%, rgba(0,255,128,.08), transparent 30%),
        radial-gradient(circle at 90% 18%, rgba(29,140,255,.035), transparent 30%),
        var(--bg);
      color: var(--text);
      font-family: var(--font);
      font-size: 15px;
      padding: 14px;
    }
    h1 { margin: 0 0 12px; font-size: 20px; font-weight: 900; letter-spacing: -.03em; }

    /* Hero: 3 cards side by side */
    .hero {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 8px;
      margin-bottom: 8px;
    }
    .hero-cell {
      padding: 11px 12px;
      border: 1px solid var(--line);
      border-radius: 10px;
      background:
        radial-gradient(circle at 50% 0%, rgba(0,255,128,.06), transparent 55%),
        linear-gradient(180deg, rgba(15,18,16,.96), rgba(8,11,9,.98));
      box-shadow: 0 0 14px rgba(0,255,128,.06), inset 0 1px 0 rgba(255,255,255,.025);
    }
    .eyebrow {
      color: var(--muted);
      font-size: 11px;
      letter-spacing: .18em;
      text-transform: uppercase;
      font-weight: 900;
      margin-bottom: 7px;
    }
    .status-pill {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 4px 8px;
      border-radius: 999px;
      border: 1px solid rgba(255,255,255,.10);
      background: rgba(255,255,255,.05);
      font-size: 11px;
      font-weight: 900;
      letter-spacing: .08em;
      text-transform: uppercase;
      margin-bottom: 7px;
    }
    .pulse {
      width: 7px; height: 7px;
      border-radius: 50%;
      background: #b8bdba;
      box-shadow: 0 0 8px rgba(255,255,255,.12);
      transition: background .4s, box-shadow .4s;
      flex-shrink: 0;
    }
    .hero-value {
      margin: 0 0 4px;
      font-size: 15px;
      font-weight: 900;
      letter-spacing: -.01em;
      line-height: 1.2;
      word-break: break-all;
    }
    .hero-spacer { height: 14px; }
    .hero-desc {
      color: var(--muted);
      font-size: 12px;
      line-height: 1.3;
    }

    /* Cards: 2 columns */
    .cards {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 8px;
    }
    .card {
      border: 1px solid var(--line);
      border-radius: 10px;
      overflow: hidden;
      background: linear-gradient(180deg, rgba(15,18,16,.96), rgba(8,11,9,.98));
      box-shadow: inset 0 1px 0 rgba(255,255,255,.025);
    }
    .card-title {
      padding: 9px 11px;
      border-bottom: 1px solid var(--line);
      color: var(--muted);
      font-size: 11px;
      font-weight: 900;
      letter-spacing: .22em;
      text-transform: uppercase;
    }
    .card-body { padding: 2px 11px 6px; }
    .row {
      display: flex;
      flex-direction: column;
      gap: 1px;
      padding: 6px 0;
      border-bottom: 1px solid var(--line-soft);
    }
    .row:last-child { border-bottom: none; }
    .k { color: var(--muted); font-size: 11px; font-weight: 900; letter-spacing: .08em; text-transform: uppercase; }
    .v { color: var(--text); font-weight: 800; font-size: 14px; word-break: break-all; line-height: 1.3; }
    .v.mono { font-family: var(--mono); font-size: 13px; }
  </style>
</head>
<body>
  <h1>RadioCore — Status</h1>

  <div class="hero">
    <div class="hero-cell">
      <div class="eyebrow">Instância</div>
      <div class="hero-spacer"></div>
      <div style="display:flex;align-items:center;gap:7px;">
        <span id="pulse" class="pulse"></span>
        <div id="heroState" class="hero-value">—</div>
      </div>
      <div id="heroDesc" class="hero-desc">Conectando…</div>
    </div>
    <div class="hero-cell">
      <div class="eyebrow">Em atividade</div>
      <div class="hero-spacer"></div>
      <div id="heroUptime" class="hero-value">—</div>
      <div class="hero-desc">Desde o último início.</div>
    </div>
    <div class="hero-cell">
      <div class="eyebrow">Atualizado em</div>
      <div class="hero-spacer"></div>
      <div id="heroNow" class="hero-value">—</div>
      <div class="hero-desc">Atualiza a cada 5s.</div>
    </div>
  </div>

  <div class="cards">
    <div class="card">
      <div class="card-title">Processo</div>
      <div class="card-body">
        <div class="row"><div class="k">Sistema</div><div id="os" class="v mono">—</div></div>
        <div class="row"><div class="k">Uptime</div><div id="uptime" class="v">—</div></div>
        <div class="row"><div class="k">PID</div><div id="pid" class="v mono">—</div></div>
        <div class="row"><div class="k">Versão</div><div id="version" class="v mono">—</div></div>
      </div>
    </div>
    <div class="card">
      <div class="card-title">Conectividade</div>
      <div class="card-body">
        <div class="row"><div class="k">IP da rede</div><div id="localIp" class="v mono">—</div></div>
        <div class="row"><div class="k">Porta</div><div id="port" class="v mono">—</div></div>
        <div class="row"><div class="k">API</div><div id="api" class="v mono">—</div></div>
        <div class="row"><div class="k">Eventos</div><div id="events" class="v mono">—</div></div>
      </div>
    </div>
  </div>

  <script>
    const $ = id => document.getElementById(id);
    let startTime = null;

    const stateDesc = {
      IDLE:     'Carregada, sem reprodução.',
      PLAYING:  'Reproduzindo áudio.',
      PAUSED:   'Reprodução pausada.',
      ASSIST:   'Modo assistência ativo.',
      PANIC:    'Modo pânico — emergência.',
      STOPPING: 'Encerrando reprodução.',
      STARTING: 'Inicializando subsistemas.',
      ERROR:    'Erro crítico encontrado.',
    };

    function pulseColor(state, online) {
      if (!online) return ['#ff3b30', '0 0 10px rgba(255,59,48,.5)'];
      if (['PLAYING','ASSIST'].includes(state)) return ['#00ff80','0 0 10px rgba(0,255,128,.6)'];
      if (state === 'PAUSED') return ['#ff9500','0 0 10px rgba(255,149,0,.6)'];
      if (['ERROR','PANIC'].includes(state)) return ['#ff3b30','0 0 10px rgba(255,59,48,.5)'];
      return ['#b8bdba','0 0 10px rgba(255,255,255,.12)'];
    }

    function formatUptime(ms) {
      const s = Math.floor(ms / 1000);
      const h = Math.floor(s / 3600);
      const m = Math.floor((s % 3600) / 60);
      if (h > 0) return h + 'h ' + m + 'm';
      if (m > 0) return m + 'm ' + (s % 60) + 's';
      return s + 's';
    }

    function now() {
      return new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    }

    function setOnline(online, state) {
      const [color, glow] = pulseColor(state || '', online);
      const dot = $('pulse');
      dot.style.background = color;
      dot.style.boxShadow = glow;
      $('heroState').style.color = color;
      if (!online) {
        $('heroState').textContent = 'OFFLINE';
        $('heroDesc').textContent = 'Engine não responde. Reconectando…';
      }
    }

    function updateUI(status) {
      const state = status.state || '—';
      const uptime = startTime ? formatUptime(Date.now() - startTime.getTime()) : '—';
      $('heroState').textContent = state;
      $('heroDesc').textContent = stateDesc[state] || '';
      $('heroUptime').textContent = uptime;
      $('heroNow').textContent = now();
      $('uptime').textContent = uptime;
    }

    async function loadInfo() {
      const d = await fetch('/v1/info').then(r => r.json());
      startTime = new Date(d.start_time);
      $('pid').textContent = d.pid;
      $('version').textContent = d.version;
      $('os').textContent = d.os || '—';
      $('localIp').textContent = d.local_ip || '—';
      const host = location.host;
      $('port').textContent = location.port || '80';
      $('api').textContent = location.origin;
      $('events').textContent = 'ws://' + host + '/v1/events';
    }

    async function poll() {
      try {
        const [, status] = await Promise.all([
          fetch('/v1/health').then(r => { if (!r.ok) throw new Error(); return r.json(); }),
          fetch('/v1/status').then(r => { if (!r.ok) throw new Error(); return r.json(); }),
        ]);
        setOnline(true, status.state);
        updateUI(status);
      } catch {
        setOnline(false);
      }
      setTimeout(poll, 5000);
    }

    loadInfo().catch(() => {}).then(poll);
  </script>
</body>
</html>
`)
