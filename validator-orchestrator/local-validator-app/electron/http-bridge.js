/**
 * Omniphi Local Validator - HTTP Bridge Server
 * Exposes HTTP API on port 15000 for external access to validator status
 */

const express = require('express');
const { exec } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs').promises;

let server = null;
const PORT = 15000;

/**
 * Get paths
 */
function getPaths() {
  const homeDir = os.homedir();
  const validatorHome = path.join(homeDir, '.omniphi');
  const binaryPath = path.join(__dirname, '../bin', process.platform === 'win32' ? 'posd.exe' : 'posd');

  return { validatorHome, binaryPath };
}

/**
 * Start HTTP bridge server
 */
async function startHttpBridge() {
  if (server) {
    console.log('HTTP bridge already running');
    return;
  }

  const app = express();
  app.use(express.json());

  // CORS
  app.use((req, res, next) => {
    res.header('Access-Control-Allow-Origin', '*');
    res.header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    res.header('Access-Control-Allow-Headers', 'Content-Type');
    if (req.method === 'OPTIONS') {
      return res.sendStatus(200);
    }
    next();
  });

  /**
   * GET /consensus-pubkey
   * Return consensus public key
   */
  app.get('/consensus-pubkey', async (req, res) => {
    try {
      const paths = getPaths();
      const privKeyPath = path.join(paths.validatorHome, 'config', 'priv_validator_key.json');

      const keyData = await fs.readFile(privKeyPath, 'utf8');
      const key = JSON.parse(keyData);

      res.json({
        success: true,
        pubkey: key.pub_key
      });
    } catch (error) {
      res.status(500).json({
        success: false,
        error: error.message
      });
    }
  });

  /**
   * GET /status
   * Return validator node status
   */
  app.get('/status', async (req, res) => {
    try {
      const paths = getPaths();

      // Query local RPC
      const response = await fetch('http://localhost:26657/status');
      const data = await response.json();

      if (data.result) {
        res.json({
          success: true,
          status: {
            running: true,
            blockHeight: parseInt(data.result.sync_info.latest_block_height),
            syncing: data.result.sync_info.catching_up,
            peers: parseInt(data.result.sync_info.num_peers || 0),
            moniker: data.result.node_info.moniker,
            network: data.result.node_info.network
          }
        });
      } else {
        throw new Error('Invalid RPC response');
      }
    } catch (error) {
      res.json({
        success: true,
        status: {
          running: false,
          error: 'Node not responding'
        }
      });
    }
  });

  /**
   * GET /logs
   * Return recent logs
   */
  app.get('/logs', async (req, res) => {
    try {
      const lines = parseInt(req.query.lines) || 100;
      const paths = getPaths();
      const logPath = path.join(paths.validatorHome, 'logs', 'node.log');

      try {
        const logData = await fs.readFile(logPath, 'utf8');
        const logLines = logData.split('\n').slice(-lines);

        res.json({
          success: true,
          logs: logLines
        });
      } catch {
        res.json({
          success: true,
          logs: []
        });
      }
    } catch (error) {
      res.status(500).json({
        success: false,
        error: error.message
      });
    }
  });

  /**
   * POST /start
   * Start validator node
   */
  app.post('/start', async (req, res) => {
    try {
      // This would need to communicate with the main process
      // For now, return not implemented
      res.status(501).json({
        success: false,
        error: 'Use desktop app UI to start validator'
      });
    } catch (error) {
      res.status(500).json({
        success: false,
        error: error.message
      });
    }
  });

  /**
   * POST /stop
   * Stop validator node
   */
  app.post('/stop', async (req, res) => {
    try {
      // This would need to communicate with the main process
      res.status(501).json({
        success: false,
        error: 'Use desktop app UI to stop validator'
      });
    } catch (error) {
      res.status(500).json({
        success: false,
        error: error.message
      });
    }
  });

  /**
   * GET /health
   * Health check
   */
  app.get('/health', (req, res) => {
    res.json({
      success: true,
      service: 'omniphi-local-validator-bridge',
      version: '1.0.0'
    });
  });

  // Start server
  return new Promise((resolve, reject) => {
    server = app.listen(PORT, () => {
      console.log(`HTTP bridge listening on http://localhost:${PORT}`);
      resolve();
    });

    server.on('error', (error) => {
      if (error.code === 'EADDRINUSE') {
        console.error(`Port ${PORT} already in use`);
      }
      reject(error);
    });
  });
}

/**
 * Stop HTTP bridge server
 */
async function stopHttpBridge() {
  if (!server) {
    return;
  }

  return new Promise((resolve) => {
    server.close(() => {
      console.log('HTTP bridge stopped');
      server = null;
      resolve();
    });
  });
}

module.exports = {
  startHttpBridge,
  stopHttpBridge
};
