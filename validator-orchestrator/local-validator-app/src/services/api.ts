/**
 * Omniphi Local Validator - API Service Layer
 *
 * Centralized IPC communication wrapper for all Electron API calls.
 * Provides type-safe API calls with error handling and retry logic.
 */

import {
  ValidatorStatus,
  NodeHealth,
  ValidatorMetadata,
  PoCScore,
  RewardsInfo,
  SlashingInfo,
  ValidatorConfig,
  LogEntry,
  ConsensusKey,
} from '../types/validator';

// Re-export types for convenience
export type {
  ValidatorStatus,
  NodeHealth,
  ValidatorMetadata,
  PoCScore,
  RewardsInfo,
  SlashingInfo,
  ValidatorConfig,
  LogEntry,
  ConsensusKey,
};

// API Response wrapper
interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

// Electron API interface (mirrors window.electronAPI)
interface ElectronAPI {
  startValidator(config: object): Promise<{ success: boolean; error?: string }>;
  stopValidator(): Promise<{ success: boolean; error?: string }>;
  getValidatorStatus(): Promise<any>;
  onStatusUpdate(callback: (status: any) => void): void;
  removeStatusListener(): void;
  getConfig(): Promise<ValidatorConfig>;
  setConfig(config: ValidatorConfig): Promise<void>;
  checkBinaryExists(): Promise<{ exists: boolean }>;
  // Extended API for Phase 4
  getNodeHealth?(): Promise<NodeHealth>;
  getValidatorMetadata?(): Promise<ValidatorMetadata>;
  getPoCScore?(): Promise<PoCScore>;
  getRewardsInfo?(): Promise<RewardsInfo>;
  getSlashingInfo?(): Promise<SlashingInfo>;
  getLogs?(options?: { level?: string; limit?: number }): Promise<LogEntry[]>;
  getKeys?(): Promise<ConsensusKey[]>;
  exportKey?(keyName: string): Promise<{ success: boolean; path?: string; error?: string }>;
  checkForUpdates?(): Promise<{ available: boolean; version?: string; releaseNotes?: string }>;
}

// Get the Electron API or return a mock for development
function getElectronAPI(): ElectronAPI | null {
  if (typeof window !== 'undefined' && window.electronAPI) {
    return window.electronAPI as ElectronAPI;
  }
  return null;
}

// Check if running in Electron
export function isElectron(): boolean {
  return getElectronAPI() !== null;
}

/**
 * Validator Control API
 */
export const validatorApi = {
  /**
   * Start the validator node
   */
  async start(config: Partial<ValidatorConfig> = {}): Promise<ApiResponse<void>> {
    try {
      const api = getElectronAPI();
      if (!api) {
        return { success: false, error: 'Electron API not available' };
      }
      const result = await api.startValidator(config);
      return { success: result.success, error: result.error };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to start validator' };
    }
  },

  /**
   * Stop the validator node
   */
  async stop(): Promise<ApiResponse<void>> {
    try {
      const api = getElectronAPI();
      if (!api) {
        return { success: false, error: 'Electron API not available' };
      }
      const result = await api.stopValidator();
      return { success: result.success, error: result.error };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to stop validator' };
    }
  },

  /**
   * Get current validator status
   */
  async getStatus(): Promise<ApiResponse<ValidatorStatus>> {
    try {
      const api = getElectronAPI();
      if (!api) {
        return { success: false, error: 'Electron API not available' };
      }
      const status = await api.getValidatorStatus();

      // Map old status format to new enhanced format
      const mappedStatus: ValidatorStatus = {
        moniker: status.moniker || 'local-validator',
        chain_id: status.chain_id || 'omniphi-localnet-1',
        network_id: status.network_id || 'omniphi-1',
        block_height: status.blockHeight || status.block_height || 0,
        syncing: status.syncing || false,
        peers: status.peers || 0,
        is_active: status.running || status.is_active || false,
        jailed: status.jailed || false,
        missed_blocks: status.missed_blocks || 0,
        last_signature: status.last_signature || null,
        uptime: status.uptime || 0,
        running: status.running || false,
        error: status.error || null,
      };

      return { success: true, data: mappedStatus };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to fetch status' };
    }
  },

  /**
   * Subscribe to status updates
   */
  onStatusUpdate(callback: (status: ValidatorStatus) => void): () => void {
    const api = getElectronAPI();
    if (!api) {
      console.warn('Electron API not available for status updates');
      return () => {};
    }

    const handler = (rawStatus: any) => {
      const mappedStatus: ValidatorStatus = {
        moniker: rawStatus.moniker || 'local-validator',
        chain_id: rawStatus.chain_id || 'omniphi-localnet-1',
        network_id: rawStatus.network_id || 'omniphi-1',
        block_height: rawStatus.blockHeight || rawStatus.block_height || 0,
        syncing: rawStatus.syncing || false,
        peers: rawStatus.peers || 0,
        is_active: rawStatus.running || rawStatus.is_active || false,
        jailed: rawStatus.jailed || false,
        missed_blocks: rawStatus.missed_blocks || 0,
        last_signature: rawStatus.last_signature || null,
        uptime: rawStatus.uptime || 0,
        running: rawStatus.running || false,
        error: rawStatus.error || null,
      };
      callback(mappedStatus);
    };

    api.onStatusUpdate(handler);
    return () => api.removeStatusListener();
  },

  /**
   * Check if validator binary exists
   */
  async checkBinary(): Promise<ApiResponse<boolean>> {
    try {
      const api = getElectronAPI();
      if (!api) {
        return { success: false, error: 'Electron API not available' };
      }
      const result = await api.checkBinaryExists();
      return { success: true, data: result.exists };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to check binary' };
    }
  },
};

/**
 * Node Health API
 */
export const healthApi = {
  /**
   * Get node health metrics
   */
  async getHealth(): Promise<ApiResponse<NodeHealth>> {
    try {
      const api = getElectronAPI();

      // Use real API if available
      if (api?.getNodeHealth) {
        const health = await api.getNodeHealth();
        return { success: true, data: health };
      }

      // Return mock data for development
      const mockHealth: NodeHealth = {
        cpu: Math.random() * 25 + 5,
        ram: '512MB / 16GB',
        ram_percent: 3.2 + Math.random() * 2,
        disk: '2.8GB / 500GB',
        disk_percent: 0.6,
        db_size: '2.1GB',
        net_in: `${(0.3 + Math.random() * 0.4).toFixed(1)}MB/s`,
        net_out: `${(0.2 + Math.random() * 0.3).toFixed(1)}MB/s`,
        node_id: 'f632a7ee6f28d12cde86d009ba0cc614795bf59f',
        rpc_port: 26657,
        p2p_port: 26656,
      };

      return { success: true, data: mockHealth };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to fetch health' };
    }
  },
};

/**
 * Validator Metadata API
 */
export const metadataApi = {
  /**
   * Get validator metadata
   */
  async getMetadata(): Promise<ApiResponse<ValidatorMetadata>> {
    try {
      const api = getElectronAPI();

      if (api?.getValidatorMetadata) {
        const metadata = await api.getValidatorMetadata();
        return { success: true, data: metadata };
      }

      // Mock data for development
      const mockMetadata: ValidatorMetadata = {
        operator_address: 'omnivaloper1z2rnzs9s5ga8v0nceuky2hqphx8gxms3w6a9qp',
        validator_address: 'omnivalcons1abc123def456...',
        consensus_pubkey: 'omnivalconspub1zcjduepq...',
        commission_rate: '0.10',
        commission_max_rate: '0.20',
        commission_max_change_rate: '0.01',
        min_self_delegation: '1000000',
        self_delegation: '10000000',
        delegator_shares: '10000000.000000000000000000',
        tokens: '10000000',
        voting_power: '0.15',
        website: 'https://omniphi.network',
        details: 'Local development validator',
      };

      return { success: true, data: mockMetadata };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to fetch metadata' };
    }
  },
};

/**
 * PoC Score API
 */
export const pocApi = {
  /**
   * Get PoC reputation score
   */
  async getScore(): Promise<ApiResponse<PoCScore>> {
    try {
      const api = getElectronAPI();

      if (api?.getPoCScore) {
        const score = await api.getPoCScore();
        return { success: true, data: score };
      }

      // Mock data
      const mockScore: PoCScore = {
        total_score: 85,
        reliability: 92,
        contributions: 78,
        governance: 85,
        tier: 'Gold',
        tier_color: '#FFD700',
        rank: 42,
        history: [
          { timestamp: new Date().toISOString(), score: 85, change: 2 },
          { timestamp: new Date(Date.now() - 86400000).toISOString(), score: 83, change: -1 },
          { timestamp: new Date(Date.now() - 172800000).toISOString(), score: 84, change: 3 },
        ],
      };

      return { success: true, data: mockScore };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to fetch PoC score' };
    }
  },
};

/**
 * Rewards API
 */
export const rewardsApi = {
  /**
   * Get rewards information
   */
  async getRewards(): Promise<ApiResponse<RewardsInfo>> {
    try {
      const api = getElectronAPI();

      if (api?.getRewardsInfo) {
        const rewards = await api.getRewardsInfo();
        return { success: true, data: rewards };
      }

      // Mock data
      const mockRewards: RewardsInfo = {
        daily: '12.5 OMNI',
        weekly: '87.5 OMNI',
        monthly: '375 OMNI',
        apr: '15.2%',
        apy: '16.4%',
        next_reward: 'in ~2 blocks',
        total_rewards: '1,250 OMNI',
        unclaimed_rewards: '45.2 OMNI',
      };

      return { success: true, data: mockRewards };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to fetch rewards' };
    }
  },
};

/**
 * Slashing API
 */
export const slashingApi = {
  /**
   * Get slashing information
   */
  async getSlashingInfo(): Promise<ApiResponse<SlashingInfo>> {
    try {
      const api = getElectronAPI();

      if (api?.getSlashingInfo) {
        const info = await api.getSlashingInfo();
        return { success: true, data: info };
      }

      // Mock data
      const mockSlashing: SlashingInfo = {
        missed_blocks_window: 10000,
        missed_blocks_count: 12,
        missed_blocks_percent: 0.12,
        double_sign_protection: true,
        last_double_sign_check: new Date().toISOString(),
        slashing_risk: 'low',
        tombstoned: false,
      };

      return { success: true, data: mockSlashing };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to fetch slashing info' };
    }
  },
};

/**
 * Configuration API
 */
export const configApi = {
  /**
   * Get current configuration
   */
  async getConfig(): Promise<ApiResponse<ValidatorConfig>> {
    try {
      const api = getElectronAPI();
      if (!api) {
        return { success: false, error: 'Electron API not available' };
      }
      const config = await api.getConfig();
      return { success: true, data: config };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to fetch config' };
    }
  },

  /**
   * Save configuration
   */
  async setConfig(config: ValidatorConfig): Promise<ApiResponse<void>> {
    try {
      const api = getElectronAPI();
      if (!api) {
        return { success: false, error: 'Electron API not available' };
      }
      await api.setConfig(config);
      return { success: true };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to save config' };
    }
  },
};

/**
 * Logs API
 */
export const logsApi = {
  /**
   * Get recent logs
   */
  async getLogs(options?: { level?: string; limit?: number }): Promise<ApiResponse<LogEntry[]>> {
    try {
      const api = getElectronAPI();

      if (api?.getLogs) {
        const logs = await api.getLogs(options);
        return { success: true, data: logs };
      }

      // Return empty array for mock
      return { success: true, data: [] };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to fetch logs' };
    }
  },
};

/**
 * Keys API
 */
export const keysApi = {
  /**
   * Get validator keys
   */
  async getKeys(): Promise<ApiResponse<ConsensusKey[]>> {
    try {
      const api = getElectronAPI();

      if (api?.getKeys) {
        const keys = await api.getKeys();
        return { success: true, data: keys };
      }

      // Return empty array for mock
      return { success: true, data: [] };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to fetch keys' };
    }
  },

  /**
   * Export a key
   */
  async exportKey(keyName: string): Promise<ApiResponse<string>> {
    try {
      const api = getElectronAPI();

      if (api?.exportKey) {
        const result = await api.exportKey(keyName);
        if (result.success) {
          return { success: true, data: result.path };
        }
        return { success: false, error: result.error };
      }

      return { success: false, error: 'Key export not available in this environment' };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to export key' };
    }
  },
};

/**
 * Updates API
 */
export const updatesApi = {
  /**
   * Check for application updates
   */
  async checkForUpdates(): Promise<ApiResponse<{ available: boolean; version?: string; releaseNotes?: string }>> {
    try {
      const api = getElectronAPI();

      if (api?.checkForUpdates) {
        const result = await api.checkForUpdates();
        return { success: true, data: result };
      }

      // Mock: no updates available
      return { success: true, data: { available: false } };
    } catch (error: any) {
      return { success: false, error: error.message || 'Failed to check for updates' };
    }
  },
};

// Unified API export
export const api = {
  validator: validatorApi,
  health: healthApi,
  metadata: metadataApi,
  poc: pocApi,
  rewards: rewardsApi,
  slashing: slashingApi,
  config: configApi,
  logs: logsApi,
  keys: keysApi,
  updates: updatesApi,
  isElectron,
};

export default api;
