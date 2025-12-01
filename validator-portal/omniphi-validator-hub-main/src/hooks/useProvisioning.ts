import { useState, useEffect, useCallback } from 'react';
import { validatorApi } from '@/lib/api';

export interface ProvisioningStatus {
  status: 'pending' | 'creating_vm' | 'installing' | 'syncing' | 'generating_keys' | 'completed' | 'failed';
  progress: number;
  message: string;
  consensusPubkey?: string;
  nodeId?: string;
  rpcEndpoint?: string;
  error?: string;
}

export const useProvisioning = (setupId: string | null) => {
  const [status, setStatus] = useState<ProvisioningStatus>({
    status: 'pending',
    progress: 0,
    message: 'Initializing...',
  });
  const [isComplete, setIsComplete] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchStatus = useCallback(async () => {
    if (!setupId) return;

    try {
      const response = await validatorApi.getValidatorStatus(setupId);

      // Backend returns: { setupRequest: {...}, node: {...} }
      const setupRequest = response.setupRequest;
      const node = response.node;

      // Map backend status to frontend status
      const statusMap: Record<string, ProvisioningStatus['status']> = {
        'pending': 'pending',
        'provisioning': 'creating_vm',
        'initializing': 'installing',
        'syncing': 'syncing',
        'ready_for_chain_tx': 'completed',
        'failed': 'failed',
      };

      const frontendStatus = statusMap[setupRequest.status] || 'pending';
      const progress = frontendStatus === 'completed' ? 100 :
                      frontendStatus === 'syncing' ? 75 :
                      frontendStatus === 'installing' ? 50 :
                      frontendStatus === 'creating_vm' ? 25 : 10;

      const messageMap: Record<string, string> = {
        'pending': 'Initializing provisioning...',
        'creating_vm': 'Creating validator instance...',
        'installing': 'Installing validator software...',
        'syncing': 'Syncing blockchain state...',
        'completed': 'Node is ready! Consensus key generated.',
        'failed': 'Provisioning failed',
      };

      const newStatus: ProvisioningStatus = {
        status: frontendStatus,
        progress: progress,
        message: messageMap[frontendStatus] || 'Processing...',
        consensusPubkey: setupRequest.consensusPubkey,
        nodeId: node?.id,
        rpcEndpoint: node?.rpcEndpoint,
      };

      setStatus(newStatus);

      if (frontendStatus === 'completed' && setupRequest.consensusPubkey) {
        setIsComplete(true);
      } else if (frontendStatus === 'failed') {
        setError('Provisioning failed. Please try again or contact support.');
      }
    } catch (err: any) {
      console.error('Failed to fetch provisioning status:', err);
      // Don't set error for network issues, keep polling
    }
  }, [setupId]);

  // Poll every 3 seconds
  useEffect(() => {
    if (!setupId || isComplete) return;

    fetchStatus();
    const interval = setInterval(fetchStatus, 3000);
    
    return () => clearInterval(interval);
  }, [setupId, isComplete, fetchStatus]);

  return {
    status,
    isComplete,
    error,
    refresh: fetchStatus,
  };
};
