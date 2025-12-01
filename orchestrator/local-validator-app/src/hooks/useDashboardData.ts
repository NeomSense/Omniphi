/**
 * useDashboardData Hook
 *
 * Unified data orchestration hook that combines all dashboard data sources.
 * Provides a single interface for fetching all validator-related data.
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import {
  validatorApi,
  healthApi,
  metadataApi,
  pocApi,
  rewardsApi,
  slashingApi,
  ValidatorStatus,
  NodeHealth,
  ValidatorMetadata,
  PoCScore,
  RewardsInfo,
  SlashingInfo,
} from '../services/api';

interface UseDashboardDataOptions {
  /** Polling interval for status/health in milliseconds (default: 3000) */
  fastPollingInterval?: number;
  /** Polling interval for metadata/rewards in milliseconds (default: 30000) */
  slowPollingInterval?: number;
  /** Whether to enable polling (default: true) */
  enablePolling?: boolean;
}

interface DashboardData {
  status: ValidatorStatus | null;
  health: NodeHealth | null;
  metadata: ValidatorMetadata | null;
  poc: PoCScore | null;
  rewards: RewardsInfo | null;
  slashing: SlashingInfo | null;
}

interface UseDashboardDataResult extends DashboardData {
  /** Whether initial data is loading */
  loading: boolean;
  /** Global error message */
  error: string | null;
  /** Individual loading states */
  loadingStates: {
    status: boolean;
    health: boolean;
    metadata: boolean;
    poc: boolean;
    rewards: boolean;
    slashing: boolean;
  };
  /** Individual error states */
  errors: {
    status: string | null;
    health: string | null;
    metadata: string | null;
    poc: string | null;
    rewards: string | null;
    slashing: string | null;
  };
  /** Refresh all data */
  refreshAll: () => Promise<void>;
  /** Refresh specific data type */
  refresh: (type: keyof DashboardData) => Promise<void>;
  /** Start validator */
  startValidator: (config?: object) => Promise<{ success: boolean; error?: string }>;
  /** Stop validator */
  stopValidator: () => Promise<{ success: boolean; error?: string }>;
  /** Whether validator is running */
  isRunning: boolean;
  /** Whether any data is still loading */
  isLoading: boolean;
  /** Whether there are any errors */
  hasErrors: boolean;
}

export function useDashboardData(options: UseDashboardDataOptions = {}): UseDashboardDataResult {
  const {
    fastPollingInterval = 3000,
    slowPollingInterval = 30000,
    enablePolling = true,
  } = options;

  // Data states
  const [status, setStatus] = useState<ValidatorStatus | null>(null);
  const [health, setHealth] = useState<NodeHealth | null>(null);
  const [metadata, setMetadata] = useState<ValidatorMetadata | null>(null);
  const [poc, setPoc] = useState<PoCScore | null>(null);
  const [rewards, setRewards] = useState<RewardsInfo | null>(null);
  const [slashing, setSlashing] = useState<SlashingInfo | null>(null);

  // Loading states
  const [loadingStates, setLoadingStates] = useState({
    status: true,
    health: true,
    metadata: true,
    poc: true,
    rewards: true,
    slashing: true,
  });

  // Error states
  const [errors, setErrors] = useState<UseDashboardDataResult['errors']>({
    status: null,
    health: null,
    metadata: null,
    poc: null,
    rewards: null,
    slashing: null,
  });

  const mountedRef = useRef(true);

  // Update loading state helper
  const setLoading = (key: keyof DashboardData, value: boolean) => {
    setLoadingStates(prev => ({ ...prev, [key]: value }));
  };

  // Update error state helper
  const setError = (key: keyof DashboardData, value: string | null) => {
    setErrors(prev => ({ ...prev, [key]: value }));
  };

  // Fetch functions
  const fetchStatus = useCallback(async () => {
    try {
      const result = await validatorApi.getStatus();
      if (!mountedRef.current) return;

      if (result.success && result.data) {
        setStatus(result.data);
        setError('status', null);
      } else {
        setError('status', result.error || 'Failed to fetch status');
      }
    } catch (err: any) {
      if (!mountedRef.current) return;
      setError('status', err.message);
    } finally {
      if (mountedRef.current) {
        setLoading('status', false);
      }
    }
  }, []);

  const fetchHealth = useCallback(async () => {
    try {
      const result = await healthApi.getHealth();
      if (!mountedRef.current) return;

      if (result.success && result.data) {
        setHealth(result.data);
        setError('health', null);
      } else {
        setError('health', result.error || 'Failed to fetch health');
      }
    } catch (err: any) {
      if (!mountedRef.current) return;
      setError('health', err.message);
    } finally {
      if (mountedRef.current) {
        setLoading('health', false);
      }
    }
  }, []);

  const fetchMetadata = useCallback(async () => {
    try {
      const result = await metadataApi.getMetadata();
      if (!mountedRef.current) return;

      if (result.success && result.data) {
        setMetadata(result.data);
        setError('metadata', null);
      } else {
        setError('metadata', result.error || 'Failed to fetch metadata');
      }
    } catch (err: any) {
      if (!mountedRef.current) return;
      setError('metadata', err.message);
    } finally {
      if (mountedRef.current) {
        setLoading('metadata', false);
      }
    }
  }, []);

  const fetchPoc = useCallback(async () => {
    try {
      const result = await pocApi.getScore();
      if (!mountedRef.current) return;

      if (result.success && result.data) {
        setPoc(result.data);
        setError('poc', null);
      } else {
        setError('poc', result.error || 'Failed to fetch PoC score');
      }
    } catch (err: any) {
      if (!mountedRef.current) return;
      setError('poc', err.message);
    } finally {
      if (mountedRef.current) {
        setLoading('poc', false);
      }
    }
  }, []);

  const fetchRewards = useCallback(async () => {
    try {
      const result = await rewardsApi.getRewards();
      if (!mountedRef.current) return;

      if (result.success && result.data) {
        setRewards(result.data);
        setError('rewards', null);
      } else {
        setError('rewards', result.error || 'Failed to fetch rewards');
      }
    } catch (err: any) {
      if (!mountedRef.current) return;
      setError('rewards', err.message);
    } finally {
      if (mountedRef.current) {
        setLoading('rewards', false);
      }
    }
  }, []);

  const fetchSlashing = useCallback(async () => {
    try {
      const result = await slashingApi.getSlashingInfo();
      if (!mountedRef.current) return;

      if (result.success && result.data) {
        setSlashing(result.data);
        setError('slashing', null);
      } else {
        setError('slashing', result.error || 'Failed to fetch slashing info');
      }
    } catch (err: any) {
      if (!mountedRef.current) return;
      setError('slashing', err.message);
    } finally {
      if (mountedRef.current) {
        setLoading('slashing', false);
      }
    }
  }, []);

  // Refresh individual data type
  const refresh = useCallback(async (type: keyof DashboardData) => {
    setLoading(type, true);
    switch (type) {
      case 'status':
        await fetchStatus();
        break;
      case 'health':
        await fetchHealth();
        break;
      case 'metadata':
        await fetchMetadata();
        break;
      case 'poc':
        await fetchPoc();
        break;
      case 'rewards':
        await fetchRewards();
        break;
      case 'slashing':
        await fetchSlashing();
        break;
    }
  }, [fetchStatus, fetchHealth, fetchMetadata, fetchPoc, fetchRewards, fetchSlashing]);

  // Refresh all data
  const refreshAll = useCallback(async () => {
    await Promise.all([
      refresh('status'),
      refresh('health'),
      refresh('metadata'),
      refresh('poc'),
      refresh('rewards'),
      refresh('slashing'),
    ]);
  }, [refresh]);

  // Start validator
  const startValidator = useCallback(async (config: object = {}) => {
    const result = await validatorApi.start(config);
    if (result.success) {
      await fetchStatus();
    }
    return result;
  }, [fetchStatus]);

  // Stop validator
  const stopValidator = useCallback(async () => {
    const result = await validatorApi.stop();
    if (result.success) {
      await fetchStatus();
    }
    return result;
  }, [fetchStatus]);

  // Initial fetch and polling
  useEffect(() => {
    mountedRef.current = true;

    // Initial fetch - all data
    fetchStatus();
    fetchHealth();
    fetchMetadata();
    fetchPoc();
    fetchRewards();
    fetchSlashing();

    // Set up fast polling (status and health)
    let fastPollInterval: NodeJS.Timeout | null = null;
    if (enablePolling) {
      fastPollInterval = setInterval(() => {
        fetchStatus();
        fetchHealth();
      }, fastPollingInterval);
    }

    // Set up slow polling (metadata, poc, rewards, slashing)
    let slowPollInterval: NodeJS.Timeout | null = null;
    if (enablePolling) {
      slowPollInterval = setInterval(() => {
        fetchMetadata();
        fetchPoc();
        fetchRewards();
        fetchSlashing();
      }, slowPollingInterval);
    }

    // Set up real-time status updates
    const unsubscribe = validatorApi.onStatusUpdate((newStatus) => {
      if (mountedRef.current) {
        setStatus(newStatus);
        setError('status', null);
      }
    });

    return () => {
      mountedRef.current = false;
      if (fastPollInterval) clearInterval(fastPollInterval);
      if (slowPollInterval) clearInterval(slowPollInterval);
      unsubscribe();
    };
  }, [
    fetchStatus,
    fetchHealth,
    fetchMetadata,
    fetchPoc,
    fetchRewards,
    fetchSlashing,
    fastPollingInterval,
    slowPollingInterval,
    enablePolling,
  ]);

  // Computed values
  const loading = Object.values(loadingStates).some(Boolean);
  const isLoading = loading;
  const hasErrors = Object.values(errors).some(Boolean);
  const isRunning = status?.running ?? false;

  // First error encountered (for global error display)
  const error = Object.values(errors).find(Boolean) || null;

  return {
    // Data
    status,
    health,
    metadata,
    poc,
    rewards,
    slashing,
    // States
    loading,
    error,
    loadingStates,
    errors,
    // Actions
    refreshAll,
    refresh,
    startValidator,
    stopValidator,
    // Computed
    isRunning,
    isLoading,
    hasErrors,
  };
}

export default useDashboardData;
