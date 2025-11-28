import axios from 'axios';
import { ValidatorConfig } from '@/types/validator';

// Configure your backend URL
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8000';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

export const validatorApi = {
  // Health check
  healthCheck: async () => {
    const response = await api.get('/api/v1/health');
    return response.data;
  },

  // Create validator setup request (Cloud mode)
  createCloudValidator: async (config: ValidatorConfig, walletAddress: string) => {
    const response = await api.post('/api/v1/validators/setup-requests', {
      walletAddress: walletAddress,
      validatorName: config.moniker,
      website: config.website || '',
      description: config.description || '',
      commissionRate: config.commissionRate,
      runMode: 'cloud',
      provider: config.provider || 'docker', // docker, aws, digitalocean
    });
    return response.data;
  },

  // Setup local validator
  setupLocalValidator: async (config: ValidatorConfig, walletAddress: string) => {
    const response = await api.post('/api/v1/validators/setup-requests', {
      walletAddress: walletAddress,
      validatorName: config.moniker,
      website: config.website || '',
      description: config.description || '',
      commissionRate: config.commissionRate,
      runMode: 'local',
      provider: null,
    });
    return response.data;
  },

  // Get validator setup request status (for polling during provisioning)
  getValidatorStatus: async (setupRequestId: string) => {
    const response = await api.get(`/api/v1/validators/setup-requests/${setupRequestId}`);
    return response.data;
  },

  // Get all validators for a wallet address
  getValidatorsByWallet: async (walletAddress: string) => {
    const response = await api.get(`/api/v1/validators/by-wallet/${walletAddress}`);
    return response.data;
  },

  // Stop a cloud validator
  stopValidator: async (setupRequestId: string) => {
    const response = await api.post('/api/v1/validators/stop', {
      setupRequestId: setupRequestId,
    });
    return response.data;
  },

  // Redeploy a cloud validator
  redeployValidator: async (setupRequestId: string) => {
    const response = await api.post('/api/v1/validators/redeploy', {
      setupRequestId: setupRequestId,
    });
    return response.data;
  },

  // Submit local validator heartbeat (called by desktop app)
  submitHeartbeat: async (heartbeat: {
    walletAddress: string;
    consensusPubkey: string;
    blockHeight: number;
    uptimeSeconds: number;
    localRpcPort: number;
    localP2pPort: number;
  }) => {
    const response = await api.post('/api/v1/validators/heartbeat', heartbeat);
    return response.data;
  },
};

export default api;
