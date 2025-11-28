/**
 * Omniphi Local Validator - Preload Script
 * Security bridge between main process and renderer
 * Exposes safe API to renderer via contextBridge
 */

const { contextBridge, ipcRenderer } = require('electron');

/**
 * Expose protected API to renderer process
 */
contextBridge.exposeInMainWorld('electronAPI', {
  // Validator node management
  startValidator: (config) => ipcRenderer.invoke('start-validator', config),
  stopValidator: () => ipcRenderer.invoke('stop-validator'),
  getValidatorStatus: () => ipcRenderer.invoke('get-validator-status'),
  getValidatorLogs: (lines) => ipcRenderer.invoke('get-validator-logs', lines),

  // Consensus key management
  generateConsensusKey: () => ipcRenderer.invoke('generate-consensus-key'),
  getConsensusPubkey: () => ipcRenderer.invoke('get-consensus-pubkey'),
  exportPrivateKey: (password) => ipcRenderer.invoke('export-private-key', password),
  importPrivateKey: (keyData, password) => ipcRenderer.invoke('import-private-key', { keyData, password }),

  // Configuration
  getConfig: () => ipcRenderer.invoke('get-config'),
  setConfig: (config) => ipcRenderer.invoke('set-config', config),

  // Binary management
  checkBinaryExists: () => ipcRenderer.invoke('check-binary-exists'),
  downloadBinary: () => ipcRenderer.invoke('download-binary'),

  // Heartbeat to orchestrator
  sendHeartbeat: (data) => ipcRenderer.invoke('send-heartbeat', data),

  // Status listeners
  onStatusUpdate: (callback) => {
    ipcRenderer.on('status-update', (event, data) => callback(data));
  },
  onLogUpdate: (callback) => {
    ipcRenderer.on('log-update', (event, data) => callback(data));
  },

  // Remove listeners
  removeStatusListener: () => {
    ipcRenderer.removeAllListeners('status-update');
  },
  removeLogListener: () => {
    ipcRenderer.removeAllListeners('log-update');
  }
});

console.log('Preload script initialized');
