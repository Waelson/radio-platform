const { app, BrowserWindow } = require('electron')
const path = require('path')

function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 800,
    show: false,
    title: 'Radio Player',
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
    },
  })

  win.loadFile(path.join(__dirname, 'player.html'))
  win.once('ready-to-show', () => {
    win.setFullScreen(true)
    win.show()
  })
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
