const { app, BrowserWindow, ipcMain } = require('electron')
const path = require('path')

// Set of all open hotkey windows — allows multiple simultaneous instances.
const hotkeyWindows = new Set()

function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 800,
    fullscreen: true,
    title: 'Radio Player',
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload-hotkeys.js'),
    },
  })

  win.loadFile(path.join(__dirname, 'player.html'))
  win.webContents.openDevTools({ mode: 'detach' })

  // Bloqueia window.open() no player principal — janelas de botoneira
  // são sempre abertas via IPC pelo processo principal, sem parent-child.
  win.webContents.setWindowOpenHandler(() => ({ action: 'deny' }))
}

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

app.whenReady().then(() => {
  createWindow()
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})
