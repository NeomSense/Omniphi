/**
 * Omniphi Local Validator - HTTP Bridge Server
 * Exposes HTTP API on port 15000 for external access to validator status
 */

const express = require('express');
const fs = require('fs').promises;
const config = require('./config');

let server = null;

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

  // CORS with proper security
  app.use((req, res, next) => {
    const origin = req.headers.origin;
    const { CORS_CONFIG } = config;

    // Check if origin is allowed
    if (origin && CORS_CONFIG.allowedOrigins.includes(origin)) {
      res.header('Access-Control-Allow-Origin', origin);
    } else if (!origin) {
      // Allow requests without origin (same-origin, curl, etc.)
      res.header('Access-Control-Allow-Origin', '*');
    }

    res.header('Access-Control-Allow-Methods', CORS_CONFIG.allowedMethods.join(', '));
    res.header('Access-Control-Allow-Headers', CORS_CONFIG.allowedHeaders.join(', '));
    res.header('Access-Control-Max-Age', '86400'); // 24 hours

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
      const paths = config.getPaths();
      const privKeyPath = paths.privKeyPath;

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
      // Query local RPC using IPv4 explicitly
      const [statusResponse, netInfoResponse] = await Promise.all([
        fetch(config.RPC_ENDPOINTS.local + '/status'),
        fetch(config.RPC_ENDPOINTS.local + '/net_info')
      ]);

      const statusData = await statusResponse.json();
      const netInfoData = await netInfoResponse.json();

      if (statusData.result) {
        res.json({
          success: true,
          status: {
            running: true,
            blockHeight: parseInt(statusData.result.sync_info.latest_block_height),
            syncing: statusData.result.sync_info.catching_up,
            peers: parseInt(netInfoData.result?.n_peers || 0),
            moniker: statusData.result.node_info.moniker,
            network: statusData.result.node_info.network
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
      const paths = config.getPaths();
      const logPath = paths.logsPath + '/node.log';

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
      service: config.APP_NAME + '-bridge',
      version: config.APP_VERSION
    });
  });

  // Start server
  const PORT = config.PORTS.HTTP_BRIDGE;
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
