/**
 * Omniphi Local Validator - IPC Handlers
 * Handles communication between renderer and main process
 */

const { spawn } = require('child_process');
const fs = require('fs').promises;
const path = require('path');
const os = require('os');
const keytar = require('keytar');
const axios = require('axios');
const cryptoUtils = require('./crypto-utils');
const config = require('./config');

// Lazy-load electron-store to avoid initializing before Electron is ready
let store = null;
function getStore() {
  if (!store) {
    const Store = require('electron-store');
    store = new Store();
  }
  return store;
}

// State
let validatorProcess = null;
let validatorStatus = {
  running: false,
  syncing: false,
  blockHeight: 0,
  peers: 0,
  uptime: 0
};
let startTime = null;
let logBuffer = [];
const MAX_LOG_LINES = 1000;
let setupInProgress = false; // Lock to prevent concurrent setup

/**
 * Get paths for validator data
 */
function getPaths() {
  const homeDir = os.homedir();
  const validatorHome = path.join(homeDir, '.pos-validator2');
  const binaryPath = path.join(__dirname, '../bin', process.platform === 'win32' ? 'posd.exe' : 'posd');

  return {
    validatorHome,
    binaryPath,
    configPath: path.join(validatorHome, 'config'),
    dataPath: path.join(validatorHome, 'data'),
    privKeyPath: path.join(validatorHome, 'config', 'priv_validator_key.json')
  };
}

/**
 * Helper: Run posd command and capture output
 */
async function runPosdCommand(binaryPath, args, options = {}) {
  return new Promise((resolve, reject) => {
    const proc = spawn(binaryPath, args, options);

    let stdout = '';
    let stderr = '';

    proc.stdout?.on('data', (data) => {
      stdout += data.toString();
    });

    proc.stderr?.on('data', (data) => {
      stderr += data.toString();
    });

    proc.on('close', (code) => {
      if (code === 0) {
        resolve({ stdout: stdout.trim(), stderr: stderr.trim() });
      } else {
        reject(new Error(`Command failed with code ${code}. stderr: ${stderr}`));
      }
    });

    proc.on('error', (err) => {
      reject(err);
    });
  });
}

/**
 * Check if genesis has validators
 */
async function validateGenesisSetup(validatorHome) {
  try {
    const genesisPath = path.join(validatorHome, 'config', 'genesis.json');
    const genesisContent = await fs.readFile(genesisPath, 'utf8');
    const genesis = JSON.parse(genesisContent);

    // Check if validators array exists and has at least one validator
    const validators = genesis?.app_state?.staking?.validators || [];
    // Also check for gen_txs (genesis transactions that create validators at chain start)
    const genTxs = genesis?.app_state?.genutil?.gen_txs || [];
    return validators.length > 0 || genTxs.length > 0;
  } catch (error) {
    console.error('Error validating genesis:', error);
    return false;
  }
}

/**
 * Complete genesis setup for single-node validator
 */
async function setupGenesisForValidator(binaryPath, validatorHome, config = {}) {
  const chainId = config.chainId || 'omniphi-testnet-1';
  const moniker = config.moniker || 'local-validator';
  const keyringBackend = 'test'; // Unencrypted keyring for local dev
  const validatorKeyName = 'validator';
  const initialTokens = '1000000000000omniphi';
  const stakeAmount = '100000000000omniphi';

  console.log('Starting complete genesis setup...');

  try {
    // Step 1: Create validator key (if it doesn't exist)
    console.log('Step 1: Creating validator key...');
    try {
      await runPosdCommand(binaryPath, [
        'keys', 'add', validatorKeyName,
        '--keyring-backend', keyringBackend,
        '--home', validatorHome,
        '--output', 'json',
        '--overwrite'  // Skip confirmation prompts
      ]);
      console.log('✓ Validator key created');
    } catch (error) {
      // Key might already exist, try to continue
      if (error.message.includes('already exists') || error.message.includes('overwrite')) {
        console.log('✓ Validator key already exists');
      } else {
        throw error;
      }
    }

    // Step 2: Get validator address
    console.log('Step 2: Getting validator address...');
    const { stdout: validatorAddr } = await runPosdCommand(binaryPath, [
      'keys', 'show', validatorKeyName, '-a',
      '--keyring-backend', keyringBackend,
      '--home', validatorHome
    ]);
    console.log(`✓ Validator address: ${validatorAddr}`);

    // Step 3: Add genesis account
    console.log('Step 3: Adding genesis account...');
    await runPosdCommand(binaryPath, [
      'genesis', 'add-genesis-account',
      validatorAddr, initialTokens,
      '--home', validatorHome,
      '--append'  // Allow appending if account already exists
    ]);
    console.log('✓ Genesis account added');

    // Step 4: Create genesis transaction (gentx)
    console.log('Step 4: Creating genesis transaction...');

    // Clear existing gentx files to avoid conflicts
    const gentxDir = path.join(validatorHome, 'config', 'gentx');
    try {
      await fs.rm(gentxDir, { recursive: true, force: true });
      console.log('✓ Cleared existing gentx directory');
    } catch (error) {
      // Directory might not exist, that's ok
      console.log('ℹ️  No existing gentx directory to clear');
    }

    // Create gentx
    await runPosdCommand(binaryPath, [
      'genesis', 'gentx', validatorKeyName, stakeAmount,
      '--chain-id', chainId,
      '--keyring-backend', keyringBackend,
      '--home', validatorHome
    ]);
    console.log('✓ Genesis transaction created');

    // Step 5: Collect genesis transactions (THIS IS CRITICAL!)
    console.log('Step 5: Collecting genesis transactions...');
    await runPosdCommand(binaryPath, [
      'genesis', 'collect-gentxs',
      '--home', validatorHome
    ]);
    console.log('✓ Genesis transactions collected');

    // Step 6: Validate genesis
    console.log('Step 6: Validating genesis...');
    await runPosdCommand(binaryPath, [
      'genesis', 'validate',
      '--home', validatorHome
    ]);
    console.log('✓ Genesis validated');

    console.log('✅ Complete genesis setup successful!');
    return { success: true, address: validatorAddr };

  } catch (error) {
    console.error('❌ Genesis setup failed:', error);
    throw error;
  }
}

/**
 * Start validator node
 */
async function startValidator(event, config = {}) {
  if (validatorProcess) {
    return { success: false, error: 'Validator already running' };
  }

  try {
    const paths = getPaths();
    const { binaryPath, validatorHome } = paths;

    // Check if binary exists
    try {
      await fs.access(binaryPath);
    } catch {
      return { success: false, error: 'Binary not found. Please download it first.' };
    }

    // Ensure validator is initialized
    const configPath = path.join(validatorHome, 'config', 'config.toml');
    const privKeyPath = path.join(validatorHome, 'config', 'priv_validator_key.json');
    const dataPath = path.join(validatorHome, 'data');

    let needsInit = false;

    try {
      // Check if already initialized by verifying critical files
      await fs.access(configPath);
      await fs.access(privKeyPath);
    } catch {
      needsInit = true;
    }

    if (needsInit) {
      // Initialize node
      const moniker = config.moniker || 'local-validator';
      const chainId = config.chainId || 'omniphi-testnet-1';

      await new Promise((resolve, reject) => {
        const initProcess = spawn(binaryPath, [
          'init',
          moniker,
          '--chain-id',
          chainId,
          '--home',
          validatorHome
        ]);

        let initOutput = '';
        let initError = '';

        initProcess.stdout.on('data', (data) => {
          initOutput += data.toString();
        });

        initProcess.stderr.on('data', (data) => {
          initError += data.toString();
        });

        initProcess.on('close', (code) => {
          if (code === 0) {
            // Verify initialization files were created
            fs.access(privKeyPath)
              .then(() => resolve())
              .catch(() => reject(new Error(`Init succeeded but priv_validator_key.json not created. Output: ${initOutput}`)));
          } else {
            reject(new Error(`Init failed with code ${code}. Error: ${initError}`));
          }
        });
      });
    }

    // Ensure data directory exists and is writable (critical for priv_validator_state.json)
    try {
      await fs.access(dataPath);
    } catch {
      // Create data directory if missing
      await fs.mkdir(dataPath, { recursive: true });
    }

    // Create initial priv_validator_state.json if it doesn't exist
    // This file tracks the last block signed by the validator to prevent double-signing
    const privStatePath = path.join(dataPath, 'priv_validator_state.json');
    try {
      await fs.access(privStatePath);
    } catch {
      // File doesn't exist, create it with initial state
      const initialState = {
        height: "0",
        round: 0,
        step: 0
      };
      await fs.writeFile(privStatePath, JSON.stringify(initialState, null, 2), 'utf8');
      console.log('Created initial priv_validator_state.json');
    }

    // Check if genesis has validators - if not, run complete setup
    const hasValidators = await validateGenesisSetup(validatorHome);
    if (!hasValidators) {
      // Check if setup is already in progress
      if (setupInProgress) {
        console.log('⚠️  Genesis setup already in progress, waiting...');
        return {
          success: false,
          error: 'Genesis setup already in progress. Please wait for it to complete.'
        };
      }

      setupInProgress = true; // Set lock
      console.log('Genesis has no validators - running complete setup...');

      if (event.sender) {
        event.sender.send('log-update', '🔧 Setting up genesis with validator...');
      }

      try {
        const setupResult = await setupGenesisForValidator(binaryPath, validatorHome, config);
        console.log('Genesis setup complete:', setupResult);

        if (event.sender) {
          event.sender.send('log-update', `✅ Genesis setup complete! Validator address: ${setupResult.address}`);
        }
      } catch (setupError) {
        console.error('Genesis setup failed:', setupError);
        return {
          success: false,
          error: `Genesis setup failed: ${setupError.message}. Please check logs for details.`
        };
      } finally {
        setupInProgress = false; // Release lock
      }
    } else {
      console.log('✓ Genesis already has validators');
    }

    // Start validator process
    validatorProcess = spawn(binaryPath, [
      'start',
      '--home',
      validatorHome
    ]);

    startTime = Date.now();
    validatorStatus.running = true;

    // Capture stdout
    validatorProcess.stdout.on('data', (data) => {
      const lines = data.toString().split('\n');
      lines.forEach(line => {
        if (line.trim()) {
          logBuffer.push({
            timestamp: new Date().toISOString(),
            level: 'info',
            message: line.trim()
          });

          // Trim log buffer
          if (logBuffer.length > MAX_LOG_LINES) {
            logBuffer.shift();
          }

          // Send to renderer
          if (event.sender) {
            event.sender.send('log-update', line.trim());
          }

          // Parse status from logs
          parseLogForStatus(line);
        }
      });
    });

    // Capture stderr
    validatorProcess.stderr.on('data', (data) => {
      const lines = data.toString().split('\n');
      lines.forEach(line => {
        if (line.trim()) {
          logBuffer.push({
            timestamp: new Date().toISOString(),
            level: 'error',
            message: line.trim()
          });

          if (logBuffer.length > MAX_LOG_LINES) {
            logBuffer.shift();
          }

          if (event.sender) {
            event.sender.send('log-update', line.trim());
          }
        }
      });
    });

    // Handle process exit
    validatorProcess.on('close', (code) => {
      console.log(`Validator process exited with code ${code}`);

      // Log exit reason
      if (code !== 0 && code !== null) {
        const errorMsg = `Validator exited unexpectedly with code ${code}`;
        console.error(errorMsg);

        logBuffer.push({
          timestamp: new Date().toISOString(),
          level: 'error',
          message: errorMsg
        });

        if (event.sender) {
          event.sender.send('log-update', errorMsg);
        }
      }

      validatorProcess = null;
      validatorStatus.running = false;
      validatorStatus.error = code !== 0 && code !== null ? `Process exited with code ${code}` : null;
      startTime = null;

      if (event.sender) {
        event.sender.send('status-update', validatorStatus);
      }
    });

    // Handle process errors
    validatorProcess.on('error', (err) => {
      console.error('Validator process error:', err);

      logBuffer.push({
        timestamp: new Date().toISOString(),
        level: 'error',
        message: `Process error: ${err.message}`
      });

      if (event.sender) {
        event.sender.send('log-update', `Error: ${err.message}`);
      }
    });

    // Start status polling
    startStatusPolling(event);

    return { success: true, message: 'Validator started successfully' };

  } catch (error) {
    console.error('Error starting validator:', error);
    return { success: false, error: error.message };
  }
}

/**
 * Stop validator node
 */
async function stopValidator() {
  if (!validatorProcess) {
    return { success: false, error: 'Validator not running' };
  }

  return new Promise((resolve) => {
    validatorProcess.on('close', () => {
      validatorProcess = null;
      validatorStatus.running = false;
      startTime = null;
      resolve({ success: true, message: 'Validator stopped' });
    });

    // Send SIGTERM for graceful shutdown
    validatorProcess.kill('SIGTERM');

    // Force kill after 10 seconds
    setTimeout(() => {
      if (validatorProcess) {
        validatorProcess.kill('SIGKILL');
      }
    }, 10000);
  });
}

/**
 * Get validator status
 */
async function getValidatorStatus() {
  if (!validatorProcess) {
    return {
      ...validatorStatus,
      running: false,
      uptime: 0
    };
  }

  // Calculate uptime
  if (startTime) {
    validatorStatus.uptime = Math.floor((Date.now() - startTime) / 1000);
  }

  // Query local RPC for status
  try {
    // Use 127.0.0.1 instead of localhost to force IPv4 (blockchain only listens on IPv4)
    const [statusResponse, netInfoResponse] = await Promise.all([
      axios.get('http://127.0.0.1:26657/status', { timeout: 2000 }),
      axios.get('http://127.0.0.1:26657/net_info', { timeout: 2000 })
    ]);

    const statusData = statusResponse.data;
    const netInfoData = netInfoResponse.data;

    if (statusData.result) {
      validatorStatus.blockHeight = parseInt(statusData.result.sync_info.latest_block_height);
      validatorStatus.syncing = statusData.result.sync_info.catching_up;
    }

    if (netInfoData.result) {
      validatorStatus.peers = parseInt(netInfoData.result.n_peers || 0);
    }

    console.log(`✓ RPC Status: Block ${validatorStatus.blockHeight}, Syncing: ${validatorStatus.syncing}, Peers: ${validatorStatus.peers}`);
  } catch (error) {
    // RPC not ready yet
    console.log(`⚠️  RPC not responding: ${error.message}`);
  }

  return validatorStatus;
}

/**
 * Get validator logs
 */
async function getValidatorLogs(event, lines = 100) {
  const requestedLines = Math.min(lines, logBuffer.length);
  return logBuffer.slice(-requestedLines);
}

/**
 * Generate consensus key
 */
async function generateConsensusKey() {
  const paths = getPaths();

  try {
    // Key is generated during init
    // Read the generated key
    const keyData = await fs.readFile(paths.privKeyPath, 'utf8');
    const key = JSON.parse(keyData);

    // Store in keychain using centralized config
    const { KEYCHAIN_CONFIG } = config;
    await keytar.setPassword(
      KEYCHAIN_CONFIG.service,
      KEYCHAIN_CONFIG.consensusKeyAccount,
      keyData
    );

    return {
      success: true,
      pubkey: key.pub_key.value
    };
  } catch (error) {
    return {
      success: false,
      error: error.message
    };
  }
}

/**
 * Get consensus public key
 */
async function getConsensusPubkey() {
  const paths = getPaths();
  const { KEYCHAIN_CONFIG } = config;

  try {
    // First try from validator home
    const keyData = await fs.readFile(paths.privKeyPath, 'utf8');
    const key = JSON.parse(keyData);

    return {
      success: true,
      pubkey: key.pub_key
    };
  } catch (error) {
    // Try keychain as fallback
    try {
      const stored = await keytar.getPassword(
        KEYCHAIN_CONFIG.service,
        KEYCHAIN_CONFIG.consensusKeyAccount
      );
      if (stored) {
        const key = JSON.parse(stored);
        return {
          success: true,
          pubkey: key.pub_key
        };
      }
    } catch {}

    return {
      success: false,
      error: 'Consensus key not found. Initialize validator first.'
    };
  }
}

/**
 * Export private key (encrypted with password)
 */
async function exportPrivateKey(event, password) {
  const paths = getPaths();

  try {
    if (!password || password.length < 8) {
      return {
        success: false,
        error: 'Password must be at least 8 characters'
      };
    }

    const keyData = await fs.readFile(paths.privKeyPath, 'utf8');

    // Encrypt with AES-256-GCM using PBKDF2 key derivation
    const encrypted = cryptoUtils.encrypt(keyData, password);

    return {
      success: true,
      encryptedKey: encrypted
    };
  } catch (error) {
    return {
      success: false,
      error: error.message
    };
  }
}

/**
 * Import private key
 */
async function importPrivateKey(event, { keyData, password }) {
  const paths = getPaths();

  try {
    if (!password || password.length < 8) {
      return {
        success: false,
        error: 'Password must be at least 8 characters'
      };
    }

    // Decrypt with AES-256-GCM
    let decrypted;
    try {
      decrypted = cryptoUtils.decrypt(keyData, password);
    } catch (decryptError) {
      return {
        success: false,
        error: 'Invalid password or corrupted key data'
      };
    }

    // Validate decrypted data is valid JSON with expected structure
    let keyJson;
    try {
      keyJson = JSON.parse(decrypted);
      if (!keyJson.pub_key || !keyJson.priv_key) {
        throw new Error('Invalid key structure');
      }
    } catch (parseError) {
      return {
        success: false,
        error: 'Invalid key format: expected priv_validator_key.json structure'
      };
    }

    // Ensure config directory exists
    await fs.mkdir(paths.configPath, { recursive: true });

    // Write key file
    await fs.writeFile(paths.privKeyPath, decrypted);

    // Store in keychain
    const { KEYCHAIN_CONFIG } = config;
    await keytar.setPassword(
      KEYCHAIN_CONFIG.service,
      KEYCHAIN_CONFIG.consensusKeyAccount,
      decrypted
    );

    return { success: true };
  } catch (error) {
    return {
      success: false,
      error: error.message
    };
  }
}

/**
 * Get configuration
 */
async function getConfig() {
  return getStore().get('config', config.DEFAULT_CONFIG);
}

/**
 * Set configuration
 */
async function setConfig(event, config) {
  getStore().set('config', config);
  return { success: true };
}

/**
 * Check if binary exists
 */
async function checkBinaryExists() {
  const paths = getPaths();

  try {
    await fs.access(paths.binaryPath);
    return { exists: true };
  } catch {
    return { exists: false };
  }
}

/**
 * Download binary (placeholder - implement actual download)
 */
async function downloadBinary() {
  // TODO: Implement binary download from GitHub releases
  return {
    success: false,
    error: 'Binary download not implemented. Please manually place posd binary in bin/ directory.'
  };
}

/**
 * Send heartbeat to orchestrator
 */
async function sendHeartbeat(event, data) {
  const config = await getConfig();

  try {
    const pubkey = await getConsensusPubkey();

    if (!pubkey.success) {
      return { success: false, error: 'No consensus key available' };
    }

    const status = await getValidatorStatus();

    const heartbeatData = {
      walletAddress: data.walletAddress,
      consensusPubkey: pubkey.pubkey.value,
      blockHeight: status.blockHeight,
      uptimeSeconds: status.uptime,
      localRpcPort: 26657,
      localP2pPort: 26656
    };

    const response = await axios.post(
      `${config.orchestratorUrl}/api/v1/validators/heartbeat`,
      heartbeatData,
      { timeout: 5000 }
    );

    return {
      success: true,
      response: response.data
    };
  } catch (error) {
    return {
      success: false,
      error: error.message
    };
  }
}

/**
 * Parse logs for status updates
 */
function parseLogForStatus(line) {
  // Example: Extract block height from logs
  const heightMatch = line.match(/committed state.*height=(\d+)/);
  if (heightMatch) {
    validatorStatus.blockHeight = parseInt(heightMatch[1]);
  }

  // Example: Detect syncing
  if (line.includes('Executed block') || line.includes('committed state')) {
    validatorStatus.syncing = true;
  }
}

/**
 * Start periodic status polling
 */
function startStatusPolling(event) {
  const interval = setInterval(async () => {
    if (!validatorProcess) {
      clearInterval(interval);
      return;
    }

    const status = await getValidatorStatus();

    if (event.sender) {
      event.sender.send('status-update', status);
    }
  }, 3000); // Poll every 3 seconds
}

/**
 * Setup all IPC handlers
 */
function setupIpcHandlers(ipcMain) {
  ipcMain.handle('start-validator', startValidator);
  ipcMain.handle('stop-validator', stopValidator);
  ipcMain.handle('get-validator-status', getValidatorStatus);
  ipcMain.handle('get-validator-logs', getValidatorLogs);
  ipcMain.handle('generate-consensus-key', generateConsensusKey);
  ipcMain.handle('get-consensus-pubkey', getConsensusPubkey);
  ipcMain.handle('export-private-key', exportPrivateKey);
  ipcMain.handle('import-private-key', importPrivateKey);
  ipcMain.handle('get-config', getConfig);
  ipcMain.handle('set-config', setConfig);
  ipcMain.handle('check-binary-exists', checkBinaryExists);
  ipcMain.handle('download-binary', downloadBinary);
  ipcMain.handle('send-heartbeat', sendHeartbeat);
}

module.exports = {
  setupIpcHandlers,
  stopValidator
};
