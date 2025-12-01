/**
 * Omniphi Local Validator - Electron Main Process
 * Manages window lifecycle, IPC communication, and system integration
 */

const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');
const { setupIpcHandlers } = require('./ipc-handlers');
const { startHttpBridge, stopHttpBridge } = require('./http-bridge');

let mainWindow = null;
let isDev = true; // Will be set properly in whenReady

/**
 * Create the main application window
 */
function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true,
      enableRemoteModule: false
    },
    icon: path.join(__dirname, '../public/icon.png'),
    title: 'Omniphi Local Validator'
  });

  // Load the app
  if (isDev) {
    mainWindow.loadURL('http://127.0.0.1:4200');
    mainWindow.webContents.openDevTools();
  } else {
    mainWindow.loadFile(path.join(__dirname, '../dist-react/index.html'));
  }

  // Handle window close
  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

/**
 * App lifecycle events
 */
app.whenReady().then(async () => {
  console.log('Omniphi Local Validator starting...');

  // Determine if running in development
  isDev = !app.isPackaged;

  // Create main window
  createWindow();

  // Setup IPC handlers
  setupIpcHandlers(ipcMain);

  // Start HTTP bridge server (port 15000)
  try {
    await startHttpBridge();
    console.log('HTTP bridge started on port 15000');
  } catch (error) {
    console.error('Failed to start HTTP bridge:', error);
  }

  // macOS: Re-create window when dock icon is clicked
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

/**
 * Quit when all windows are closed (except on macOS)
 */
app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

/**
 * Clean up before quit
 */
app.on('before-quit', async (event) => {
  event.preventDefault();

  console.log('Cleaning up before quit...');

  // Stop HTTP bridge
  await stopHttpBridge();

  // Stop validator node if running
  const { stopValidator } = require('./ipc-handlers');
  await stopValidator();

  // Now quit for real
  app.exit(0);
});

/**
 * Handle uncaught errors
 */
process.on('uncaughtException', (error) => {
  console.error('Uncaught exception:', error);
});

process.on('unhandledRejection', (error) => {
  console.error('Unhandled rejection:', error);
});
