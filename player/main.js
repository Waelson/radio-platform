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
    },
  })

  win.loadFile(path.join(__dirname, 'player.html'))
  win.webContents.openDevTools({ mode: 'detach' })

  const hotkeyWindowOptions = {
    action: 'allow',
    overrideBrowserWindowOptions: {
      width: 560,
      height: 720,
      minWidth: 380,
      minHeight: 420,
      fullscreen: false,
      resizable: true,
      frame: true,
      title: 'RadioFlow — Botoneira',
      webPreferences: {
        nodeIntegration: false,
        contextIsolation: true,
      },
    },
  }

  win.webContents.setWindowOpenHandler(() => hotkeyWindowOptions)

  win.webContents.on('did-create-window', (childWin) => {
    childWin.webContents.setWindowOpenHandler(() => hotkeyWindowOptions)
  })
}

ipcMain.on('open-hotkeys', (event, opts) => {
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
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
    },
  })
  win.loadFile(path.join(__dirname, 'hotkeys.html'), { search: query })
  hotkeyWindows.add(win)
  win.on('closed', () => hotkeyWindows.delete(win))
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
