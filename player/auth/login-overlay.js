/**
 * login-overlay.js
 * Injeta e controla o overlay de autenticação no player.
 *
 * Telas (state machine):
 *   T1 — Login (email + senha)
 *   T2 — Esqueci a senha (email)
 *   T3 — Verificar código (6 dígitos)
 *   T4 — Nova senha (reset confirm)
 *   T5 — Forçar troca de senha (force_change_pwd)
 *   T6 — Handover (trocar operador)
 */
;(function () {
  'use strict'

  // ── CSS ──────────────────────────────────────────────────────────────────────

  const CSS = `
#auth-overlay {
  display: none;
  position: fixed;
  inset: 0;
  z-index: 9000;
  background: rgba(6, 18, 26, 0.82);
  backdrop-filter: blur(6px);
  align-items: center;
  justify-content: center;
}
#auth-overlay.visible {
  display: flex;
}
body.auth-locked .app {
  filter: blur(3px) brightness(0.5);
  pointer-events: none;
  user-select: none;
}
.ao-card {
  background: #0d2233;
  border: 1px solid #1e3d55;
  border-radius: 10px;
  width: 360px;
  padding: 32px 32px 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  box-shadow: 0 24px 64px rgba(0,0,0,0.6);
}
.ao-logo {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0px;
  margin-bottom: 4px;
  width: 100%;
}
.ao-logo img { height: 52px; width: auto; margin-right: 20px; }
.ao-logo-titles {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-left: -20px;
}
.ao-logo-text { font-size: 22px; font-weight: 700; color: #c8e6f5; letter-spacing: 0.5px; line-height: 1; }
.ao-logo-sub  { font-size: 10px; color: #4a6478; font-weight: 600; letter-spacing: 2px; }
.ao-title { font-size: 14px; font-weight: 600; color: #7abed6; text-transform: uppercase; letter-spacing: 1px; }
.ao-field { display: flex; flex-direction: column; gap: 5px; }
.ao-field label { font-size: 11px; color: #4a6478; font-weight: 600; text-transform: uppercase; letter-spacing: 0.8px; }
.ao-field input {
  background: #071620;
  border: 1px solid #1e3d55;
  border-radius: 5px;
  color: #c8e6f5;
  font-size: 13px;
  padding: 9px 11px;
  outline: none;
  transition: border-color 0.15s;
}
.ao-field input:focus { border-color: #2a7fba; }
.ao-field input::placeholder { color: #2a4a5e; }
.ao-error {
  font-size: 12px;
  color: #e05555;
  min-height: 16px;
  display: none;
}
.ao-error.visible { display: block; }
.ao-btn {
  background: #1a6fa3;
  border: none;
  border-radius: 5px;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  padding: 10px;
  cursor: pointer;
  transition: background 0.15s;
  width: 100%;
}
.ao-btn:hover:not(:disabled) { background: #2a8fcb; }
.ao-btn:disabled { opacity: 0.45; cursor: not-allowed; }
.ao-btn.ao-secondary {
  background: transparent;
  border: 1px solid #1e3d55;
  color: #4a6478;
}
.ao-btn.ao-secondary:hover:not(:disabled) { border-color: #2a7fba; color: #7abed6; }
.ao-link {
  font-size: 11px;
  color: #2a7fba;
  cursor: pointer;
  text-align: center;
  text-decoration: underline;
}
.ao-link:hover { color: #7abed6; }
.ao-row { display: flex; gap: 10px; }
.ao-row .ao-btn { flex: 1; }
.ao-hint { font-size: 11px; color: #4a6478; text-align: center; line-height: 1.5; }
.ao-screen { display: none; flex-direction: column; gap: 14px; }
.ao-screen.active { display: flex; }
/* user chip in topbar */
#authUserChip {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 4px 10px;
  border-radius: 20px;
  border: 1px solid #1e3d55;
  background: #071620;
  transition: border-color 0.15s;
}
#authUserChip:hover { border-color: #2a7fba; }
.auc-avatar {
  width: 26px; height: 26px; border-radius: 50%;
  background: #1a6fa3;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 700; color: #fff; flex-shrink: 0;
}
.auc-name  { font-size: 12px; font-weight: 600; color: #c8e6f5; }
.auc-role  { font-size: 10px; color: #4a6478; }
.auc-arrow { font-size: 10px; color: #4a6478; }
#authUserMenu {
  display: none;
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  background: #0d2233;
  border: 1px solid #1e3d55;
  border-radius: 6px;
  padding: 4px 0;
  min-width: 160px;
  z-index: 8000;
  box-shadow: 0 8px 24px rgba(0,0,0,0.5);
}
#authUserMenu.open { display: block; }
.aum-item {
  padding: 8px 14px;
  font-size: 12px;
  color: #7abed6;
  cursor: pointer;
  display: block;
}
.aum-item:hover { background: #0a1e2e; color: #c8e6f5; }
.aum-item.danger { color: #e05555; }
#authChipWrap { position: relative; }
`

  // ── DOM helpers ───────────────────────────────────────────────────────────────

  function injectStyles() {
    const el = document.createElement('style')
    el.textContent = CSS
    document.head.appendChild(el)
  }

  function injectOverlay() {
    const div = document.createElement('div')
    div.id = 'auth-overlay'
    div.innerHTML = `
<div class="ao-card">
  <div class="ao-logo">
    <img src="audion-logo.png" alt="Audion">
    <div class="ao-logo-titles">
      <div class="ao-logo-text">Audion Play</div>
      <div class="ao-logo-sub">BROADCAST SUITE</div>
    </div>
  </div>

  <!-- T1: Login -->
  <div class="ao-screen active" id="aoT1">
    <div class="ao-title">Acesso ao sistema</div>
    <div class="ao-field">
      <label>E-mail</label>
      <input id="aoEmail" type="email" placeholder="operador@radio.com" autocomplete="username"/>
    </div>
    <div class="ao-field">
      <label>Senha</label>
      <input id="aoPassword" type="password" placeholder="••••••••" autocomplete="current-password"/>
    </div>
    <div class="ao-error" id="aoLoginErr"></div>
    <button class="ao-btn" id="aoLoginBtn">Entrar</button>
    <span class="ao-link" id="aoForgotLink">Esqueci minha senha</span>
  </div>

  <!-- T2: Esqueci a senha -->
  <div class="ao-screen" id="aoT2">
    <div class="ao-title">Recuperar senha</div>
    <div class="ao-hint">Informe seu e-mail e enviaremos um código de verificação.</div>
    <div class="ao-field">
      <label>E-mail</label>
      <input id="aoResetEmail" type="email" placeholder="operador@radio.com"/>
    </div>
    <div class="ao-error" id="aoResetEmailErr"></div>
    <div class="ao-row">
      <button class="ao-btn ao-secondary" id="aoResetEmailBack">Voltar</button>
      <button class="ao-btn" id="aoResetEmailBtn">Enviar código</button>
    </div>
  </div>

  <!-- T3: Verificar código -->
  <div class="ao-screen" id="aoT3">
    <div class="ao-title">Verificar código</div>
    <div class="ao-hint" id="aoCodeHint">Digite o código de 6 dígitos enviado para seu e-mail.</div>
    <div class="ao-field">
      <label>Código</label>
      <input id="aoCode" type="text" maxlength="6" placeholder="000000" autocomplete="one-time-code"/>
    </div>
    <div class="ao-error" id="aoCodeErr"></div>
    <div class="ao-row">
      <button class="ao-btn ao-secondary" id="aoCodeBack">Voltar</button>
      <button class="ao-btn" id="aoCodeBtn">Verificar</button>
    </div>
    <span class="ao-link" id="aoResendLink">Reenviar código</span>
  </div>

  <!-- T4: Nova senha (reset confirm) -->
  <div class="ao-screen" id="aoT4">
    <div class="ao-title">Nova senha</div>
    <div class="ao-field">
      <label>Nova senha</label>
      <input id="aoNewPwd1" type="password" placeholder="••••••••"/>
    </div>
    <div class="ao-field">
      <label>Confirmar nova senha</label>
      <input id="aoNewPwd2" type="password" placeholder="••••••••"/>
    </div>
    <div class="ao-error" id="aoNewPwdErr"></div>
    <button class="ao-btn" id="aoNewPwdBtn">Redefinir senha</button>
  </div>

  <!-- T5: Troca de senha obrigatória (force_change_pwd) -->
  <div class="ao-screen" id="aoT5">
    <div class="ao-title">Troca de senha obrigatória</div>
    <div class="ao-hint">Por segurança, você deve definir uma nova senha antes de continuar.</div>
    <div class="ao-field">
      <label>Senha atual (temporária)</label>
      <input id="aoForceCur" type="password" placeholder="••••••••"/>
    </div>
    <div class="ao-field">
      <label>Nova senha</label>
      <input id="aoForceNew1" type="password" placeholder="••••••••"/>
    </div>
    <div class="ao-field">
      <label>Confirmar nova senha</label>
      <input id="aoForceNew2" type="password" placeholder="••••••••"/>
    </div>
    <div class="ao-error" id="aoForceErr"></div>
    <button class="ao-btn" id="aoForceBtn">Salvar nova senha</button>
  </div>

  <!-- T6: Handover -->
  <div class="ao-screen" id="aoT6">
    <div class="ao-title">Troca de operador</div>
    <div class="ao-hint">O playout continua no ar. Faça login com outro perfil.</div>
    <div class="ao-field">
      <label>E-mail</label>
      <input id="aoHOEmail" type="email" placeholder="operador@radio.com"/>
    </div>
    <div class="ao-field">
      <label>Senha</label>
      <input id="aoHOPassword" type="password" placeholder="••••••••"/>
    </div>
    <div class="ao-error" id="aoHOErr"></div>
    <div class="ao-row">
      <button class="ao-btn ao-secondary" id="aoHOCancel">Cancelar</button>
      <button class="ao-btn" id="aoHOBtn">Entrar</button>
    </div>
  </div>
</div>`

    document.body.insertBefore(div, document.body.firstChild)
  }

  function injectUserChip() {
    const topbar = document.querySelector('.topbar')
    if (!topbar) return

    const wrap = document.createElement('div')
    wrap.id = 'authChipWrap'
    wrap.innerHTML = `
<div id="authUserChip">
  <div class="auc-avatar" id="aucAvatar">?</div>
  <div>
    <div class="auc-name" id="aucName">—</div>
    <div class="auc-role" id="aucRole">—</div>
  </div>
  <span class="auc-arrow">▾</span>
</div>
<div id="authUserMenu">
  <span class="aum-item" id="aumChangePwd">Alterar senha</span>
  <span class="aum-item danger" id="aumSwitch">Trocar operador</span>
  <span class="aum-item danger" id="aumLogout">Sair</span>
</div>`

    // Insert before clock-block
    const clock = topbar.querySelector('.clock-block')
    if (clock) topbar.insertBefore(wrap, clock)
    else topbar.appendChild(wrap)
  }

  // ── State machine ─────────────────────────────────────────────────────────────

  let _libUrl     = ''
  let _resetEmail = ''
  let _resetToken = ''

  function showScreen(id) {
    document.querySelectorAll('.ao-screen').forEach(s => s.classList.remove('active'))
    const el = document.getElementById(id)
    if (el) { el.classList.add('active'); const inp = el.querySelector('input'); if (inp) setTimeout(() => inp.focus(), 50) }
  }

  function showOverlay(screen) {
    document.body.classList.add('auth-locked')
    const ov = document.getElementById('auth-overlay')
    if (ov) ov.classList.add('visible')
    showScreen(screen || 'aoT1')
  }

  function hideOverlay() {
    document.body.classList.remove('auth-locked')
    const ov = document.getElementById('auth-overlay')
    if (ov) ov.classList.remove('visible')
  }

  function setError(id, msg) {
    const el = document.getElementById(id)
    if (!el) return
    el.textContent = msg || ''
    el.classList.toggle('visible', !!msg)
  }

  function setLoading(btnId, loading) {
    const btn = document.getElementById(btnId)
    if (btn) btn.disabled = loading
  }

  // ── Auth success ──────────────────────────────────────────────────────────────

  function onAuthSuccess(claims) {
    hideOverlay()
    updateChip(claims)
    // Initialise panels that require Library auth
    if (typeof stmInit       === 'function') try { stmInit()       } catch {}
    if (typeof hkpInit       === 'function') try { hkpInit()       } catch {}
    // Refresh engine-side data
    if (typeof fetchQueue    === 'function') try { fetchQueue()    } catch {}
    if (typeof fetchStatus   === 'function') try { fetchStatus()   } catch {}
    if (typeof stmFetchStatuses === 'function') try { stmFetchStatuses() } catch {}
  }

  function updateChip(claims) {
    if (!claims) return
    const name   = claims.name || claims.email || '?'
    const initials = name.split(/\s+/).map(w => w[0] || '').join('').substring(0, 2).toUpperCase() || '?'
    const el = document.getElementById('aucAvatar'); if (el) el.textContent = initials
    const nm = document.getElementById('aucName');   if (nm) nm.textContent = name
    const rl = document.getElementById('aucRole');   if (rl) rl.textContent = (claims.role || '').toUpperCase()
  }

  // ── Event wiring ──────────────────────────────────────────────────────────────

  function wireEvents() {
    // T1 — Login
    document.getElementById('aoLoginBtn').addEventListener('click', async () => {
      setError('aoLoginErr', '')
      const email = document.getElementById('aoEmail').value.trim()
      const pwd   = document.getElementById('aoPassword').value
      if (!email || !pwd) { setError('aoLoginErr', 'Preencha e-mail e senha.'); return }
      setLoading('aoLoginBtn', true)
      const r = await window.sessionManager.login(email, pwd)
      setLoading('aoLoginBtn', false)
      if (!r.ok) { setError('aoLoginErr', r.message); return }
      if (r.claims?.force_change_pwd) {
        showScreen('aoT5')
        return
      }
      onAuthSuccess(r.claims)
    })

    document.getElementById('aoPassword').addEventListener('keydown', e => {
      if (e.key === 'Enter') document.getElementById('aoLoginBtn').click()
    })

    document.getElementById('aoForgotLink').addEventListener('click', () => {
      setError('aoLoginErr', '')
      showScreen('aoT2')
    })

    // T2 — Esqueci a senha
    document.getElementById('aoResetEmailBack').addEventListener('click', () => showScreen('aoT1'))
    document.getElementById('aoResetEmailBtn').addEventListener('click', async () => {
      setError('aoResetEmailErr', '')
      const email = document.getElementById('aoResetEmail').value.trim()
      if (!email) { setError('aoResetEmailErr', 'Informe seu e-mail.'); return }
      setLoading('aoResetEmailBtn', true)
      const r = await window.sessionManager.resetRequest(email)
      setLoading('aoResetEmailBtn', false)
      if (!r.ok) { setError('aoResetEmailErr', r.message); return }
      _resetEmail = email
      document.getElementById('aoCodeHint').textContent =
        `Digite o código de 6 dígitos enviado para ${email}.`
      showScreen('aoT3')
    })

    // T3 — Verificar código
    document.getElementById('aoCodeBack').addEventListener('click', () => showScreen('aoT2'))
    document.getElementById('aoCodeBtn').addEventListener('click', async () => {
      setError('aoCodeErr', '')
      const code = document.getElementById('aoCode').value.trim()
      if (code.length !== 6) { setError('aoCodeErr', 'O código deve ter 6 dígitos.'); return }
      setLoading('aoCodeBtn', true)
      const r = await window.sessionManager.resetVerify(_resetEmail, code)
      setLoading('aoCodeBtn', false)
      if (!r.ok) { setError('aoCodeErr', r.message); return }
      _resetToken = r.resetToken
      showScreen('aoT4')
    })

    document.getElementById('aoResendLink').addEventListener('click', async () => {
      setError('aoCodeErr', '')
      const r = await window.sessionManager.resetRequest(_resetEmail)
      if (!r.ok) { setError('aoCodeErr', r.message) }
      else { setError('aoCodeErr', ''); document.getElementById('aoCode').value = '' }
    })

    // T4 — Nova senha (reset confirm)
    document.getElementById('aoNewPwdBtn').addEventListener('click', async () => {
      setError('aoNewPwdErr', '')
      const p1 = document.getElementById('aoNewPwd1').value
      const p2 = document.getElementById('aoNewPwd2').value
      if (!p1) { setError('aoNewPwdErr', 'Informe a nova senha.'); return }
      if (p1 !== p2) { setError('aoNewPwdErr', 'As senhas não coincidem.'); return }
      setLoading('aoNewPwdBtn', true)
      const r = await window.sessionManager.resetConfirm(_resetToken, p1)
      setLoading('aoNewPwdBtn', false)
      if (!r.ok) { setError('aoNewPwdErr', r.message); return }
      onAuthSuccess(r.claims)
    })

    // T5 — Troca de senha obrigatória
    document.getElementById('aoForceBtn').addEventListener('click', async () => {
      setError('aoForceErr', '')
      const cur = document.getElementById('aoForceCur').value
      const p1  = document.getElementById('aoForceNew1').value
      const p2  = document.getElementById('aoForceNew2').value
      if (!cur || !p1) { setError('aoForceErr', 'Preencha todos os campos.'); return }
      if (p1 !== p2) { setError('aoForceErr', 'As senhas não coincidem.'); return }
      setLoading('aoForceBtn', true)
      const r = await window.sessionManager.changePwd(cur, p1)
      setLoading('aoForceBtn', false)
      if (!r.ok) { setError('aoForceErr', r.message); return }
      onAuthSuccess(r.claims)
    })

    // T6 — Handover
    document.getElementById('aoHOCancel').addEventListener('click', () => hideOverlay())
    document.getElementById('aoHOBtn').addEventListener('click', async () => {
      setError('aoHOErr', '')
      const email = document.getElementById('aoHOEmail').value.trim()
      const pwd   = document.getElementById('aoHOPassword').value
      if (!email || !pwd) { setError('aoHOErr', 'Preencha e-mail e senha.'); return }
      setLoading('aoHOBtn', true)
      const r = await window.sessionManager.login(email, pwd)
      setLoading('aoHOBtn', false)
      if (!r.ok) { setError('aoHOErr', r.message); return }
      if (r.claims?.force_change_pwd) { showScreen('aoT5'); return }
      onAuthSuccess(r.claims)
    })

    // User chip menu
    document.getElementById('authUserChip').addEventListener('click', (e) => {
      e.stopPropagation()
      document.getElementById('authUserMenu').classList.toggle('open')
    })
    document.addEventListener('click', () => {
      const m = document.getElementById('authUserMenu')
      if (m) m.classList.remove('open')
    })

    document.getElementById('aumChangePwd').addEventListener('click', () => {
      document.getElementById('authUserMenu').classList.remove('open')
      showOverlay('aoT5')
      document.getElementById('aoForceCur').value  = ''
      document.getElementById('aoForceNew1').value = ''
      document.getElementById('aoForceNew2').value = ''
      setError('aoForceErr', '')
    })

    document.getElementById('aumSwitch').addEventListener('click', async () => {
      document.getElementById('authUserMenu').classList.remove('open')
      await window.sessionManager.switchUser()
      showOverlay('aoT6')
    })

    document.getElementById('aumLogout').addEventListener('click', async () => {
      document.getElementById('authUserMenu').classList.remove('open')
      await window.sessionManager.logout()
      document.getElementById('aoEmail').value    = ''
      document.getElementById('aoPassword').value = ''
      setError('aoLoginErr', '')
      showOverlay('aoT1')
    })
  }

  // ── Public API ────────────────────────────────────────────────────────────────

  const loginOverlay = {
    async init(libUrl) {
      _libUrl = libUrl
      injectStyles()
      injectOverlay()
      injectUserChip()
      wireEvents()

      // Always require the user to enter their password on startup.
      // Pre-fill email from saved session if available, but never skip login.
      const claims = await window.sessionManager.init(libUrl)
      if (claims && claims.email) {
        document.getElementById('aoEmail').value = claims.email
      }
      showOverlay('aoT1')
    },
  }

  window.loginOverlay = loginOverlay
})()
