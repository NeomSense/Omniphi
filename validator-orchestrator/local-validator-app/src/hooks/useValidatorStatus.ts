/**
 * useValidatorStatus Hook
 *
 * Manages validator status state with polling and real-time updates.
 * Provides automatic reconnection and error handling.
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import { validatorApi, ValidatorStatus } from '../services/api';

interface UseValidatorStatusOptions {
  /** Polling interval in milliseconds (default: 3000) */
  pollingInterval?: number;
  /** Whether to enable polling (default: true) */
  enablePolling?: boolean;
  /** Whether to enable real-time updates via IPC (default: true) */
  enableRealtime?: boolean;
}

interface UseValidatorStatusResult {
  /** Current validator status */
  status: ValidatorStatus | null;
  /** Whether the initial load is in progress */
  loading: boolean;
  /** Error message if any */
  error: string | null;
  /** Manually refresh the status */
  refresh: () => Promise<void>;
  /** Start the validator */
  start: (config?: object) => Promise<{ success: boolean; error?: string }>;
  /** Stop the validator */
  stop: () => Promise<{ success: boolean; error?: string }>;
  /** Whether the validator is running */
  isRunning: boolean;
  /** Whether the node is syncing */
  isSyncing: boolean;
  /** Whether the validator is jailed */
  isJailed: boolean;
  /** Whether the validator is active */
  isActive: boolean;
}

export function useValidatorStatus(options: UseValidatorStatusOptions = {}): UseValidatorStatusResult {
  const {
    pollingInterval = 3000,
    enablePolling = true,
    enableRealtime = true,
  } = options;

  const [status, setStatus] = useState<ValidatorStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Track mounted state to prevent state updates after unmount
  const mountedRef = useRef(true);

  // Fetch status
  const fetchStatus = useCallback(async () => {
    try {
      const result = await validatorApi.getStatus();
      if (!mountedRef.current) return;

      if (result.success && result.data) {
        setStatus(result.data);
        setError(null);
      } else {
        setError(result.error || 'Failed to fetch status');
      }
    } catch (err: any) {
      if (!mountedRef.current) return;
      setError(err.message || 'Failed to fetch status');
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, []);

  // Refresh function for manual refresh
  const refresh = useCallback(async () => {
    setLoading(true);
    await fetchStatus();
  }, [fetchStatus]);

  // Start validator
  const start = useCallback(async (config: object = {}) => {
    setError(null);
    const result = await validatorApi.start(config);
    if (!result.success) {
      setError(result.error || 'Failed to start validator');
    }
    // Refresh status after action
    await fetchStatus();
    return result;
  }, [fetchStatus]);

  // Stop validator
  const stop = useCallback(async () => {
    setError(null);
    const result = await validatorApi.stop();
    if (!result.success) {
      setError(result.error || 'Failed to stop validator');
    }
    // Refresh status after action
    await fetchStatus();
    return result;
  }, [fetchStatus]);

  // Initial fetch and polling setup
  useEffect(() => {
    mountedRef.current = true;

    // Initial fetch
    fetchStatus();

    // Set up polling
    let pollInterval: NodeJS.Timeout | null = null;
    if (enablePolling) {
      pollInterval = setInterval(fetchStatus, pollingInterval);
    }

    // Set up real-time updates
    let unsubscribe: (() => void) | null = null;
    if (enableRealtime) {
      unsubscribe = validatorApi.onStatusUpdate((newStatus) => {
        if (mountedRef.current) {
          setStatus(newStatus);
          setError(null);
        }
      });
    }

    return () => {
      mountedRef.current = false;
      if (pollInterval) {
        clearInterval(pollInterval);
      }
      if (unsubscribe) {
        unsubscribe();
      }
    };
  }, [fetchStatus, pollingInterval, enablePolling, enableRealtime]);

  // Computed properties
  const isRunning = status?.running ?? false;
  const isSyncing = status?.syncing ?? false;
  const isJailed = status?.jailed ?? false;
  const isActive = status?.is_active ?? false;

  return {
    status,
    loading,
    error,
    refresh,
    start,
    stop,
    isRunning,
    isSyncing,
    isJailed,
    isActive,
  };
}

export default useValidatorStatus;
