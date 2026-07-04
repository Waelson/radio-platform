const { app, BrowserWindow } = require('electron')
const path = require('path')

function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 800,
    title: 'Radio Player',
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
    },
  })

  // In production: player.html is bundled as an extra resource.
  // In development: reference the file directly from the playout project.
  const htmlPath = app.isPackaged
    ? path.join(process.resourcesPath, 'player.html')
    : path.join(__dirname, '..', 'playout', 'cmd', 'playout-engine', 'assets', 'player.html')

  win.loadFile(htmlPath)
}

app.whenReady().then(() => {
  createWindow()
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})
