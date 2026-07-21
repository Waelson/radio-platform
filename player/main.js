const { app, BrowserWindow, ipcMain, safeStorage } = require('electron')
const path = require('path')
const fs   = require('fs')
const http = require('http')
const https = require('https')

// Set of all open hotkey windows — allows multiple simultaneous instances.
const hotkeyWindows = new Set()

// Reference to the main player window (used for close interception).
let mainWin = null

// When true the quit was approved by the renderer — skip interception.
let _quitting = false

// ── Session expiry watchdog ──────────────────────────────────────────────────

let _expiryTimer = null

/** Decode a JWT and return the exp field in milliseconds (0 if invalid). */
function jwtExpiresAt(token) {
  try {
    const payload = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    const claims  = JSON.parse(Buffer.from(payload, 'base64').toString('utf8'))
    return claims.exp ? claims.exp * 1000 : 0
  } catch {
    return 0
  }
}

/** Broadcast auth:session-expired to every open window. */
function broadcastExpired() {
  mainWin?.webContents.send('auth:session-expired')
  hotkeyWindows.forEach(w => { try { w.webContents.send('auth:session-expired') } catch {} })
}

/** Broadcast auth:session-renewed to every open hotkey window. */
function broadcastRenewed() {
  hotkeyWindows.forEach(w => { try { w.webContents.send('auth:session-renewed') } catch {} })
}

/**
 * Schedule a timer that fires exactly when the token expires and broadcasts
 * auth:session-expired to all windows. If the token is already expired, fires
 * immediately. Call this after every login, refresh, or app focus.
 */
function scheduleExpiryWatch(token) {
  if (_expiryTimer) { clearTimeout(_expiryTimer); _expiryTimer = null }
  if (!token) return
  const exp = jwtExpiresAt(token)
  if (!exp) return
  const msUntilExpiry = exp - Date.now()
  if (msUntilExpiry <= 0) {
    broadcastExpired()
    return
  }
  _expiryTimer = setTimeout(() => {
    _expiryTimer = null
    broadcastExpired()
  }, msUntilExpiry)
}

function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 800,
    fullscreen: true,
    title: 'Radio Player',
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.js'),
    },
  })

  mainWin = win
  win.loadFile(path.join(__dirname, 'player.html'))
  //win.webContents.openDevTools({ mode: 'detach' })

  // Intercept window X-button close — ask renderer to handle logout flow.
  win.on('close', e => {
    if (_quitting) return
    e.preventDefault()
    win.webContents.send('app:closing')
  })

  // Bloqueia window.open() no player principal — janelas de botoneira
  // são sempre abertas via IPC pelo processo principal, sem parent-child.
  win.webContents.setWindowOpenHandler(() => ({ action: 'deny' }))
}

// Intercept Command+Q on macOS — ask renderer to handle logout flow.
app.on('before-quit', e => {
  if (_quitting) return
  e.preventDefault()
  mainWin?.webContents.send('app:closing')
})

function createHotkeyWindow(opts) {
  const apiUrl = (opts && opts.api) || ''
  const libUrl = (opts && opts.lib) || ''
  const query  = (apiUrl || libUrl)
    ? '?api=' + encodeURIComponent(apiUrl) + '&lib=' + encodeURIComponent(libUrl)
    : ''

  const win = new BrowserWindow({
    width: 560,
    height: 720,
    minWidth: 380,
    minHeight: 420,
    resizable: true,
    frame: true,
    title: 'RadioFlow — Botoneira',
    // Sem parent — janela completamente independente.
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload-hotkeys.js'),
    },
  })
  win.loadFile(path.join(__dirname, 'hotkeys.html'), { search: query })
  // Bloqueia window.open() dentro da botoneira — novas janelas via IPC.
  win.webContents.setWindowOpenHandler(() => ({ action: 'deny' }))
  hotkeyWindows.add(win)
  win.on('closed', () => hotkeyWindows.delete(win))
}

ipcMain.on('open-hotkeys', (_event, opts) => createHotkeyWindow(opts))
ipcMain.on('app:quit', () => { _quitting = true; app.quit() })
ipcMain.on('app:cancel-close', () => { _quitting = false })

// ── Session persistence (safeStorage + userData) ────────────────────────────

function sessionFilePath() {
  return path.join(app.getPath('userData'), 'session.enc')
}

function saveSession(token) {
  if (!safeStorage.isEncryptionAvailable()) {
    fs.writeFileSync(sessionFilePath(), token, 'utf8')
    return
  }
  const enc = safeStorage.encryptString(token)
  fs.writeFileSync(sessionFilePath(), enc)
}

function loadSession() {
  const p = sessionFilePath()
  if (!fs.existsSync(p)) return null
  try {
    const raw = fs.readFileSync(p)
    if (!safeStorage.isEncryptionAvailable()) return raw.toString('utf8')
    return safeStorage.decryptString(raw)
  } catch {
    return null
  }
}

function clearSession() {
  const p = sessionFilePath()
  if (fs.existsSync(p)) fs.unlinkSync(p)
}

// ── HTTP helper (main process → library service) ─────────────────────────────

function apiPost(libUrl, endpoint, body, token) {
  return new Promise((resolve, reject) => {
    const payload = JSON.stringify(body)
    const parsed  = new URL(libUrl)
    const isHttps = parsed.protocol === 'https:'
    const lib     = isHttps ? https : http
    const options = {
      hostname: parsed.hostname,
      port:     parsed.port || (isHttps ? 443 : 80),
      path:     endpoint,
      method:   'POST',
      headers:  {
        'Content-Type':   'application/json',
        'Content-Length': Buffer.byteLength(payload),
        ...(token ? { Authorization: 'Bearer ' + token } : {}),
      },
    }
    const req = lib.request(options, (res) => {
      let data = ''
      res.on('data', (c) => { data += c })
      res.on('end', () => {
        try { resolve({ status: res.statusCode, body: JSON.parse(data) }) }
        catch { resolve({ status: res.statusCode, body: data }) }
      })
    })
    req.on('error', reject)
    req.write(payload)
    req.end()
  })
}

// ── Auth IPC handlers ────────────────────────────────────────────────────────

ipcMain.handle('auth:get-session', () => {
  return loadSession()
})

ipcMain.handle('auth:get-token', () => {
  return loadSession()
})

ipcMain.handle('auth:login', async (_e, { libUrl, email, password }) => {
  const res = await apiPost(libUrl, '/v1/auth/login', { email, password })
  if (res.status === 200 && res.body?.data?.token) {
    saveSession(res.body.data.token)
    scheduleExpiryWatch(res.body.data.token)
  }
  return res
})

ipcMain.handle('auth:logout', () => {
  if (_expiryTimer) { clearTimeout(_expiryTimer); _expiryTimer = null }
  clearSession()
  return { ok: true }
})

ipcMain.handle('auth:refresh', async (_e, { libUrl, token }) => {
  const res = await apiPost(libUrl, '/v1/auth/refresh', {}, token)
  if (res.status === 200 && res.body?.data?.token) {
    saveSession(res.body.data.token)
    scheduleExpiryWatch(res.body.data.token)
  }
  return res
})

// Renderer notifies that a new session is active (after login/refresh success).
// Re-arms the watchdog and tells all hotkey windows to unblock.
ipcMain.on('auth:session-renewed', () => {
  const token = loadSession()
  scheduleExpiryWatch(token)
  broadcastRenewed()
})

// Renderer detected a 401 — broadcast expired to all other windows immediately.
ipcMain.on('auth:notify-expired', () => {
  broadcastExpired()
})

ipcMain.handle('auth:reset-request', async (_e, { libUrl, email }) => {
  return apiPost(libUrl, '/v1/auth/reset-request', { email })
})

ipcMain.handle('auth:reset-verify', async (_e, { libUrl, email, code }) => {
  return apiPost(libUrl, '/v1/auth/reset-verify', { email, code })
})

ipcMain.handle('auth:reset-confirm', async (_e, { libUrl, resetToken, newPwd }) => {
  const res = await apiPost(libUrl, '/v1/auth/reset-confirm', {
    reset_token: resetToken, new_password: newPwd,
  })
  if (res.status === 200 && res.body?.data?.token) {
    saveSession(res.body.data.token)
  }
  return res
})

ipcMain.handle('auth:change-pwd', async (_e, { libUrl, token, curPwd, newPwd }) => {
  const res = await apiPost(libUrl, '/v1/auth/change-password', {
    current_password: curPwd, new_password: newPwd,
  }, token)
  if (res.status === 200 && res.body?.data?.token) {
    saveSession(res.body.data.token)
  }
  return res
})

app.whenReady().then(() => {
  createWindow()

  // Arm watchdog for any token already persisted from a previous session.
  const existingToken = loadSession()
  if (existingToken) scheduleExpiryWatch(existingToken)

  // Re-check on every window focus — catches expiry after system sleep/wake.
  app.on('browser-window-focus', () => {
    const token = loadSession()
    if (!token) return
    if (jwtExpiresAt(token) <= Date.now()) broadcastExpired()
  })

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})
