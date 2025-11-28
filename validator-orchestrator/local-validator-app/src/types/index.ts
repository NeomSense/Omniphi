/**
 * TypeScript type definitions for Omniphi Local Validator
 */

export interface ValidatorStatus {
  running: boolean;
  syncing: boolean;
  blockHeight: number;
  peers: number;
  uptime: number;
}

export interface ConsensusPubkey {
  '@type': string;
  key: string;
}

export interface ValidatorConfig {
  chainId: string;
  moniker: string;
  orchestratorUrl: string;
  heartbeatInterval: number;
  autoStart: boolean;
}

export interface LogEntry {
  timestamp: string;
  level: 'info' | 'error' | 'warn';
  message: string;
}

export interface ElectronAPI {
  // Validator node management
  startValidator: (config: Partial<ValidatorConfig>) => Promise<{
    success: boolean;
    message?: string;
    error?: string;
  }>;
  stopValidator: () => Promise<{
    success: boolean;
    message?: string;
    error?: string;
  }>;
  getValidatorStatus: () => Promise<ValidatorStatus>;
  getValidatorLogs: (lines: number) => Promise<LogEntry[]>;

  // Consensus key management
  generateConsensusKey: () => Promise<{
    success: boolean;
    pubkey?: string;
    error?: string;
  }>;
  getConsensusPubkey: () => Promise<{
    success: boolean;
    pubkey?: ConsensusPubkey;
    error?: string;
  }>;
  exportPrivateKey: (password: string) => Promise<{
    success: boolean;
    encryptedKey?: string;
    error?: string;
  }>;
  importPrivateKey: (keyData: string, password: string) => Promise<{
    success: boolean;
    error?: string;
  }>;

  // Configuration
  getConfig: () => Promise<ValidatorConfig>;
  setConfig: (config: Partial<ValidatorConfig>) => Promise<{
    success: boolean;
  }>;

  // Binary management
  checkBinaryExists: () => Promise<{ exists: boolean }>;
  downloadBinary: () => Promise<{
    success: boolean;
    error?: string;
  }>;

  // Heartbeat
  sendHeartbeat: (data: { walletAddress: string }) => Promise<{
    success: boolean;
    response?: any;
    error?: string;
  }>;

  // Event listeners
  onStatusUpdate: (callback: (status: ValidatorStatus) => void) => void;
  onLogUpdate: (callback: (log: string) => void) => void;
  removeStatusListener: () => void;
  removeLogListener: () => void;
}

// Extend Window interface
declare global {
  interface Window {
    electronAPI: ElectronAPI;
  }
}

export {};
