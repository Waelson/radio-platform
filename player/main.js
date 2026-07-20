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
  }
  return res
})

ipcMain.handle('auth:logout', () => {
  clearSession()
  return { ok: true }
})

ipcMain.handle('auth:refresh', async (_e, { libUrl, token }) => {
  const res = await apiPost(libUrl, '/v1/auth/refresh', {}, token)
  if (res.status === 200 && res.body?.data?.token) {
    saveSession(res.body.data.token)
  }
  return res
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
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})
