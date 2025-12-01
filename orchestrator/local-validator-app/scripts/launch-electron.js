#!/usr/bin/env node
/**
 * Electron Launcher Script
 *
 * This script launches Electron with ELECTRON_RUN_AS_NODE properly unset.
 * This is necessary because IDEs like VSCode/Cursor set ELECTRON_RUN_AS_NODE=1
 * which causes Electron to run as a plain Node.js process instead of a GUI app.
 */

const { spawn } = require('child_process');
const path = require('path');

// Get the electron binary path
const electronPath = path.join(__dirname, '..', 'node_modules', 'electron', 'dist',
  process.platform === 'win32' ? 'electron.exe' : 'electron');

// Get the app directory (parent of scripts folder)
const appDir = path.join(__dirname, '..');

// Create a clean environment without ELECTRON_RUN_AS_NODE
const cleanEnv = { ...process.env };
delete cleanEnv.ELECTRON_RUN_AS_NODE;

console.log('Launching Electron...');
console.log('Electron path:', electronPath);
console.log('App directory:', appDir);

// Spawn Electron with the clean environment
const electron = spawn(electronPath, [appDir], {
  env: cleanEnv,
  stdio: 'inherit',
  cwd: appDir
});

electron.on('error', (err) => {
  console.error('Failed to launch Electron:', err.message);
  process.exit(1);
});

electron.on('close', (code) => {
  process.exit(code || 0);
});
