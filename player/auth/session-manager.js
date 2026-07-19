/**
 * session-manager.js
 * Gerencia o token JWT no renderer: armazenamento (via main process),
 * refresh silencioso e rate-limit client-side para reset de senha.
 */
;(function () {
  'use strict'

  // ── JWT helpers ─────────────────────────────────────────────────────────────

  function jwtDecode(token) {
    try {
      const parts   = token.split('.')
      if (parts.length !== 3) return null
      const payload = parts[1].replace(/-/g, '+').replace(/_/g, '/')
      return JSON.parse(atob(payload))
    } catch {
      return null
    }
  }

  function jwtExpiresAt(token) {
    const claims = jwtDecode(token)
    return claims ? claims.exp * 1000 : 0   // ms
  }

  // ── State ───────────────────────────────────────────────────────────────────

  let _token    = null   // current JWT string
  let _claims   = null   // decoded payload
  let _libUrl   = ''     // library service base URL
  let _refreshTimer = null

  // Client-side rate limit: email → timestamp of last reset-request sent
  const _resetSentAt = new Map()
  const RESET_COOLDOWN_MS = 60_000   // mirrors server-side 60 s

  // ── Internal helpers ─────────────────────────────────────────────────────────

  function _store(token) {
    _token  = token
    _claims = token ? jwtDecode(token) : null
    _scheduleRefresh()
  }

  function _scheduleRefresh() {
    if (_refreshTimer) { clearTimeout(_refreshTimer); _refreshTimer = null }
    if (!_token) return
    const exp  = jwtExpiresAt(_token)
    const now  = Date.now()
    const msToRefresh = (exp - now) - 60 * 60 * 1000   // 1 h before expiry
    if (msToRefresh <= 0) return
    _refreshTimer = setTimeout(async () => {
      try {
        const res = await window.electronAPI.authRefresh(_libUrl, _token)
        if (res.status === 200 && res.body?.data?.token) {
          _store(res.body.data.token)
        }
      } catch { /* network failure — user will be asked to re-login on next 401 */ }
    }, msToRefresh)
  }

  // ── Public API ───────────────────────────────────────────────────────────────

  const sessionManager = {

    /**
     * Inicializa: carrega token salvo do disco (via main process).
     * Retorna as claims se o token ainda for válido, ou null.
     */
    async init(libUrl) {
      _libUrl = libUrl
      const saved = await window.electronAPI.authGetSession()
      if (saved) {
        const exp = jwtExpiresAt(saved)
        if (exp > Date.now()) {
          _store(saved)
          return _claims
        }
      }
      return null
    },

    async login(email, password) {
      const res = await window.electronAPI.authLogin(_libUrl, email, password)
      if (res.status === 200 && res.body?.data?.token) {
        _store(res.body.data.token)
        return { ok: true, claims: _claims }
      }
      const msg = res.body?.message || 'Credenciais inválidas.'
      return { ok: false, message: msg, status: res.status }
    },

    async logout() {
      _store(null)
      await window.electronAPI.authLogout()
    },

    /**
     * Troca de operador (handover): não desconecta o playout.
     * Apenas desloga o usuário atual e exibe o overlay de login.
     */
    async switchUser() {
      await this.logout()
    },

    async resetRequest(email) {
      const lastSent = _resetSentAt.get(email) || 0
      if (Date.now() - lastSent < RESET_COOLDOWN_MS) {
        return { ok: false, message: 'Aguarde 60 segundos antes de solicitar outro código.' }
      }
      const res = await window.electronAPI.authResetRequest(_libUrl, email)
      if (res.status === 200) {
        _resetSentAt.set(email, Date.now())
        return { ok: true }
      }
      if (res.status === 429) {
        return { ok: false, message: 'Muitas tentativas. Aguarde e tente novamente.' }
      }
      return { ok: false, message: res.body?.message || 'Erro ao solicitar código.' }
    },

    async resetVerify(email, code) {
      const res = await window.electronAPI.authResetVerify(_libUrl, email, code)
      if (res.status === 200 && res.body?.data?.reset_token) {
        return { ok: true, resetToken: res.body.data.reset_token }
      }
      return { ok: false, message: res.body?.message || 'Código inválido.' }
    },

    async resetConfirm(resetToken, newPwd) {
      const res = await window.electronAPI.authResetConfirm(_libUrl, resetToken, newPwd)
      if (res.status === 200 && res.body?.data?.token) {
        _store(res.body.data.token)
        return { ok: true, claims: _claims }
      }
      return { ok: false, message: res.body?.message || 'Erro ao redefinir senha.' }
    },

    async changePwd(curPwd, newPwd) {
      const res = await window.electronAPI.authChangePwd(_libUrl, _token, curPwd, newPwd)
      if (res.status === 200 && res.body?.data?.token) {
        _store(res.body.data.token)
        return { ok: true, claims: _claims }
      }
      return { ok: false, message: res.body?.message || 'Erro ao alterar senha.' }
    },

    getToken()         { return _token },
    getClaims()        { return _claims },
    isAuthenticated()  { return !!_token && jwtExpiresAt(_token) > Date.now() },
  }

  window.sessionManager = sessionManager
})()
