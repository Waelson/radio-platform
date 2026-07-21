const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('electronAPI', {
  openHotkeys: (opts) => ipcRenderer.send('open-hotkeys', opts),
  getToken:    ()     => ipcRenderer.invoke('auth:get-token'),

  // ── Session expiry notifications ──────────────────────────────────
  // Fired by the main process when the JWT expires — hotkey window must block.
  onSessionExpired: (cb) => ipcRenderer.on('auth:session-expired', cb),
  // Fired by the main process after a successful re-login — unblock.
  onSessionRenewed: (cb) => ipcRenderer.on('auth:session-renewed', cb),
})
