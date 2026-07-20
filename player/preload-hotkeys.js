const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('electronAPI', {
  openHotkeys: (opts) => ipcRenderer.send('open-hotkeys', opts),
  getToken:    ()     => ipcRenderer.invoke('auth:get-token'),
})
