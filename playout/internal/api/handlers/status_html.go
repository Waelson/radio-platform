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
  <title>Radio Playout Engine • Status da Engine</title>
  <style>
    :root {
      --bg: #070807;
      --surface: #101211;
      --line: rgba(0, 255, 128, .22);
      --line-strong: rgba(0, 255, 128, .55);
      --green: #00ff80;
      --green-soft: rgba(0, 255, 128, .11);
      --red: #ff3b30;
      --text: #e8eee9;
      --muted: #8a948d;
      --muted-2: #5b655e;
      --shadow: rgba(0, 255, 128, .12);
      --mono: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
      --font: Inter, "Segoe UI", Roboto, Arial, sans-serif;
    }
    * { box-sizing: border-box; }
    html, body { height: 100%; }
    body {
      margin: 0;
      min-height: 100vh;
      background:
        radial-gradient(circle at 46% 0%, rgba(0, 255, 128, .08), transparent 28%),
        radial-gradient(circle at 90% 20%, rgba(29, 140, 255, .05), transparent 32%),
        #070807;
      color: var(--text);
      font-family: var(--font);
      overflow: hidden;
    }
    .app { width: 100vw; height: 100vh; display: block; }
    .main { min-width: 0; min-height: 0; overflow: auto; padding: 18px; }
    .page-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      gap: 24px;
      margin-bottom: 18px;
    }
    .page-header h1 { margin: 0 0 8px; font-size: 26px; letter-spacing: -.02em; }
    .hero {
      border: 1px solid var(--line-strong);
      border-radius: 16px;
      overflow: hidden;
      background:
        radial-gradient(circle at 50% 0%, rgba(0,255,128,.08), transparent 55%),
        linear-gradient(180deg, rgba(8,16,11,.96), rgba(8,13,10,.96));
      box-shadow: 0 0 26px var(--shadow), inset 0 1px 0 rgba(255,255,255,.03);
      margin-bottom: 18px;
    }
    .hero-top {
      display: grid;
      grid-template-columns: 1.3fr 1fr 0.8fr;
      border-bottom: 1px solid rgba(255,255,255,.08);
    }
    .hero-block { padding: 22px 24px; min-height: 124px; }
    .hero-block + .hero-block { border-left: 1px solid rgba(255,255,255,.08); }
    .eyebrow {
      color: var(--muted);
      font-size: 11px;
      letter-spacing: .24em;
      text-transform: uppercase;
      margin-bottom: 12px;
    }
    .hero-value { font-size: 38px; font-weight: 900; letter-spacing: -.03em; margin: 0 0 8px; }
    .hero-sub { color: var(--muted); font-size: 14px; line-height: 1.45; }
    .status-line {
      display: inline-flex;
      align-items: center;
      gap: 10px;
      padding: 7px 11px;
      border-radius: 999px;
      background: rgba(255,255,255,.05);
      border: 1px solid rgba(255,255,255,.08);
      margin-bottom: 12px;
      color: var(--text);
      font-weight: 800;
      letter-spacing: .09em;
      text-transform: uppercase;
      font-size: 12px;
    }
    .pulse {
      width: 10px;
      height: 10px;
      border-radius: 50%;
      background: #b8bdba;
      box-shadow: 0 0 14px rgba(255,255,255,.12);
      transition: background .4s, box-shadow .4s;
    }
    .refresh-note {
      padding: 14px 20px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 20px;
      color: var(--muted);
      font-size: 13px;
      background: rgba(255,255,255,.015);
    }
    .grid { display: grid; grid-template-columns: 1.05fr 1fr; gap: 18px; }
    .panel {
      border: 1px solid rgba(255,255,255,.08);
      border-radius: 14px;
      background: linear-gradient(180deg, rgba(16,18,17,.96), rgba(10,12,11,.96));
      box-shadow: inset 0 1px 0 rgba(255,255,255,.03);
      overflow: hidden;
    }
    .panel-head {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 18px;
      padding: 16px 18px;
      border-bottom: 1px solid rgba(255,255,255,.08);
    }
    .panel-title {
      margin: 0;
      font-size: 13px;
      letter-spacing: .22em;
      text-transform: uppercase;
      color: var(--muted);
    }
    .panel-body { padding: 8px 18px 18px; }
    .kv {
      display: grid;
      grid-template-columns: 220px 1fr;
      gap: 18px;
      align-items: center;
      min-height: 58px;
      border-bottom: 1px solid rgba(255,255,255,.05);
      padding: 14px 0;
    }
    .kv:last-child { border-bottom: none; }
    .k { color: var(--muted); font-size: 13px; letter-spacing: .08em; text-transform: uppercase; }
    .v { color: var(--text); font-weight: 800; font-size: 17px; word-break: break-word; }
    .v.mono { font-family: var(--mono); font-weight: 700; font-size: 15px; }
    @media (max-width: 1100px) {
      .hero-top { grid-template-columns: 1fr; }
      .hero-block + .hero-block { border-left: none; border-top: 1px solid rgba(255,255,255,.08); }
      .grid { grid-template-columns: 1fr; }
    }
    @media (max-width: 800px) {
      body { overflow: auto; }
      .app { display: block; height: auto; }
    }
  </style>
</head>
<body>
  <div class="app">
    <main class="main">
      <div class="page-header">
        <div><h1>Status da Engine</h1></div>
      </div>

      <section class="hero">
        <div class="hero-top">
          <div class="hero-block">
            <div class="eyebrow">Instância</div>
            <div class="status-line"><span id="pulse" class="pulse"></span> Estado da engine</div>
            <h2 id="heroState" class="hero-value">—</h2>
            <div id="heroDesc" class="hero-sub">Conectando…</div>
          </div>
          <div class="hero-block">
            <div class="eyebrow">Tempo em atividade</div>
            <h2 id="heroUptime" class="hero-value">—</h2>
            <div class="hero-sub">Tempo desde o último início da instância atual.</div>
          </div>
          <div class="hero-block">
            <div class="eyebrow">Última atualização</div>
            <h2 id="heroNow" class="hero-value">—</h2>
            <div class="hero-sub">A tela atualiza automaticamente a cada 5 segundos.</div>
          </div>
        </div>
        <div class="refresh-note">
          <span>Atualiza a cada 5s</span>
          <span id="refreshNote"></span>
        </div>
      </section>

      <div class="grid">
        <section class="panel">
          <div class="panel-head"><h2 class="panel-title">Informações do processo</h2></div>
          <div class="panel-body">
            <div class="kv"><div class="k">Estado</div><div id="state" class="v">—</div></div>
            <div class="kv"><div class="k">Uptime</div><div id="uptime" class="v">—</div></div>
            <div class="kv"><div class="k">PID</div><div id="pid" class="v mono">—</div></div>
            <div class="kv"><div class="k">Versão</div><div id="version" class="v mono">—</div></div>
            <div class="kv"><div class="k">Último erro</div><div id="lastError" class="v">—</div></div>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head"><h2 class="panel-title">Conectividade</h2></div>
          <div class="panel-body">
            <div class="kv"><div class="k">IP da rede</div><div id="localIp" class="v mono">—</div></div>
            <div class="kv"><div class="k">Porta</div><div id="port" class="v mono">—</div></div>
            <div class="kv"><div class="k">API</div><div id="api" class="v mono">—</div></div>
            <div class="kv"><div class="k">Eventos</div><div id="events" class="v mono">—</div></div>
          </div>
        </section>
      </div>
    </main>
  </div>

  <script>
    const $ = id => document.getElementById(id);
    let startTime = null;

    const stateDesc = {
      IDLE:     'A engine está carregada, porém sem reprodução em andamento.',
      PLAYING:  'A engine está reproduzindo áudio.',
      PAUSED:   'A reprodução está pausada.',
      ASSIST:   'Modo assistência ativo — controle manual da fila.',
      PANIC:    'Modo pânico ativo — cama de emergência em reprodução.',
      STOPPING: 'A engine está encerrando a reprodução atual.',
      STARTING: 'A engine está inicializando os subsistemas.',
      ERROR:    'A engine encontrou um erro crítico.',
    };

    function pulseColor(state, online) {
      if (!online) return ['#ff3b30', '0 0 14px rgba(255,59,48,.5)'];
      if (['PLAYING','PAUSED','ASSIST'].includes(state)) return ['#00ff80','0 0 14px rgba(0,255,128,.6)'];
      if (['ERROR','PANIC'].includes(state)) return ['#ff3b30','0 0 14px rgba(255,59,48,.5)'];
      return ['#b8bdba','0 0 14px rgba(255,255,255,.12)'];
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
      if (!online) {
        $('heroState').textContent = 'OFFLINE';
        $('heroDesc').textContent = 'Engine não está respondendo. Tentando reconectar…';
        $('state').textContent = 'OFFLINE';
        $('refreshNote').textContent = 'Última tentativa: ' + now();
      }
    }

    function updateUI(status) {
      const state = status.state || '—';
      const uptime = startTime ? formatUptime(Date.now() - startTime.getTime()) : '—';

      $('heroState').textContent = state;
      $('heroDesc').textContent = stateDesc[state] || '';
      $('heroUptime').textContent = uptime;
      $('heroNow').textContent = now();
      $('state').textContent = state;
      $('uptime').textContent = uptime;
      $('lastError').textContent = status.error || 'Nenhum';
      $('refreshNote').textContent = '';
    }

    async function loadInfo() {
      const d = await fetch('/v1/info').then(r => r.json());
      startTime = new Date(d.start_time);
      $('pid').textContent = d.pid;
      $('version').textContent = d.version;
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
