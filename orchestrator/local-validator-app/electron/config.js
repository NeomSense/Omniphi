/**
 * Omniphi Local Validator - Centralized Configuration
 * Single source of truth for all configuration values
 */

const path = require('path');
const os = require('os');

/**
 * Application constants
 */
const APP_NAME = 'omniphi-local-validator';
const APP_VERSION = '1.0.0';

/**
 * Network configuration
 */
const CHAIN_ID = 'omniphi-testnet-1';
const DEFAULT_MONIKER = 'local-validator';

/**
 * Port configuration
 */
const PORTS = {
  HTTP_BRIDGE: 15000,
  RPC: 26657,
  P2P: 26656,
  GRPC: 9090,
  REST: 1317
};

/**
 * Path configuration
 */
function getPaths() {
  const homeDir = os.homedir();
  const validatorHome = path.join(homeDir, '.pos-validator2');
  const binaryPath = path.join(__dirname, '../bin', process.platform === 'win32' ? 'posd.exe' : 'posd');

  return {
    homeDir,
    validatorHome,
    binaryPath,
    configPath: path.join(validatorHome, 'config'),
    dataPath: path.join(validatorHome, 'data'),
    logsPath: path.join(validatorHome, 'logs'),
    privKeyPath: path.join(validatorHome, 'config', 'priv_validator_key.json'),
    privStatePath: path.join(validatorHome, 'data', 'priv_validator_state.json'),
    genesisPath: path.join(validatorHome, 'config', 'genesis.json'),
    configTomlPath: path.join(validatorHome, 'config', 'config.toml'),
    appTomlPath: path.join(validatorHome, 'config', 'app.toml')
  };
}

/**
 * Default configuration for electron-store
 */
const DEFAULT_CONFIG = {
  chainId: CHAIN_ID,
  moniker: DEFAULT_MONIKER,
  orchestratorUrl: 'http://localhost:8000',
  heartbeatInterval: 60000, // 60 seconds
  autoStart: false,
  minimumGasPrices: '0.001omniphi'
};

/**
 * CORS configuration
 */
const CORS_CONFIG = {
  // Allowed origins for HTTP bridge
  // In production, this should be restricted to known domains
  allowedOrigins: [
    'http://localhost:4200',
    'http://127.0.0.1:4200',
    'http://localhost:3000',
    'http://127.0.0.1:3000',
    'app://.'  // Electron app
  ],
  allowedMethods: ['GET', 'POST', 'OPTIONS'],
  allowedHeaders: ['Content-Type', 'Authorization']
};

/**
 * Keychain service configuration
 */
const KEYCHAIN_CONFIG = {
  service: 'omniphi-validator',
  consensusKeyAccount: 'consensus-key',
  encryptionKeyAccount: 'encryption-key'
};

/**
 * RPC endpoints
 */
const RPC_ENDPOINTS = {
  local: 'http://127.0.0.1:26657',
  testnet: 'http://46.202.179.182:26657'
};

/**
 * Genesis validator setup configuration
 */
const GENESIS_CONFIG = {
  keyringBackend: 'test',
  validatorKeyName: 'validator',
  initialTokens: '1000000000000omniphi',
  stakeAmount: '100000000000omniphi'
};

/**
 * Logging configuration
 */
const LOG_CONFIG = {
  maxLogLines: 1000,
  logLevel: process.env.NODE_ENV === 'production' ? 'info' : 'debug'
};

/**
 * Status polling configuration
 */
const POLLING_CONFIG = {
  statusPollInterval: 3000, // 3 seconds
  rpcTimeout: 2000 // 2 seconds
};

module.exports = {
  APP_NAME,
  APP_VERSION,
  CHAIN_ID,
  DEFAULT_MONIKER,
  PORTS,
  getPaths,
  DEFAULT_CONFIG,
  CORS_CONFIG,
  KEYCHAIN_CONFIG,
  RPC_ENDPOINTS,
  GENESIS_CONFIG,
  LOG_CONFIG,
  POLLING_CONFIG
};
