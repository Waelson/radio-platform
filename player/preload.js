const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('electronAPI', {
  // ── Botoneira ────────────────────────────────────────────────────
  openHotkeys: (opts) => ipcRenderer.send('open-hotkeys', opts),

  // ── Auth ─────────────────────────────────────────────────────────
  authGetSession:   ()                              => ipcRenderer.invoke('auth:get-session'),
  authLogin:        (libUrl, email, password)       => ipcRenderer.invoke('auth:login',         { libUrl, email, password }),
  authLogout:       ()                              => ipcRenderer.invoke('auth:logout'),
  authRefresh:      (libUrl, token)                 => ipcRenderer.invoke('auth:refresh',        { libUrl, token }),
  authResetRequest: (libUrl, email)                 => ipcRenderer.invoke('auth:reset-request',  { libUrl, email }),
  authResetVerify:  (libUrl, email, code)           => ipcRenderer.invoke('auth:reset-verify',   { libUrl, email, code }),
  authResetConfirm: (libUrl, resetToken, newPwd)    => ipcRenderer.invoke('auth:reset-confirm',  { libUrl, resetToken, newPwd }),
  authChangePwd:    (libUrl, token, curPwd, newPwd) => ipcRenderer.invoke('auth:change-pwd',     { libUrl, token, curPwd, newPwd }),
  quitApp:          ()                              => ipcRenderer.send('app:quit'),
  cancelClose:      ()                              => ipcRenderer.send('app:cancel-close'),
  onClosing:        (cb)                            => ipcRenderer.on('app:closing', cb),
})
