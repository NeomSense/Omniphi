/**
 * useNodeHealth Hook
 *
 * Manages node health metrics with polling.
 * Monitors CPU, RAM, disk, and network usage.
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import { healthApi, NodeHealth } from '../services/api';

interface UseNodeHealthOptions {
  /** Polling interval in milliseconds (default: 3000) */
  pollingInterval?: number;
  /** Whether to enable polling (default: true) */
  enablePolling?: boolean;
}

interface UseNodeHealthResult {
  /** Current node health metrics */
  health: NodeHealth | null;
  /** Whether the initial load is in progress */
  loading: boolean;
  /** Error message if any */
  error: string | null;
  /** Manually refresh the health metrics */
  refresh: () => Promise<void>;
  /** Health status level */
  healthStatus: 'healthy' | 'warning' | 'critical' | 'unknown';
  /** CPU usage percentage */
  cpuUsage: number;
  /** RAM usage percentage */
  ramUsage: number;
  /** Disk usage percentage */
  diskUsage: number;
}

export function useNodeHealth(options: UseNodeHealthOptions = {}): UseNodeHealthResult {
  const {
    pollingInterval = 3000,
    enablePolling = true,
  } = options;

  const [health, setHealth] = useState<NodeHealth | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const mountedRef = useRef(true);

  // Fetch health metrics
  const fetchHealth = useCallback(async () => {
    try {
      const result = await healthApi.getHealth();
      if (!mountedRef.current) return;

      if (result.success && result.data) {
        setHealth(result.data);
        setError(null);
      } else {
        setError(result.error || 'Failed to fetch health metrics');
      }
    } catch (err: any) {
      if (!mountedRef.current) return;
      setError(err.message || 'Failed to fetch health metrics');
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, []);

  // Manual refresh
  const refresh = useCallback(async () => {
    setLoading(true);
    await fetchHealth();
  }, [fetchHealth]);

  // Initial fetch and polling
  useEffect(() => {
    mountedRef.current = true;

    fetchHealth();

    let pollInterval: NodeJS.Timeout | null = null;
    if (enablePolling) {
      pollInterval = setInterval(fetchHealth, pollingInterval);
    }

    return () => {
      mountedRef.current = false;
      if (pollInterval) {
        clearInterval(pollInterval);
      }
    };
  }, [fetchHealth, pollingInterval, enablePolling]);

  // Computed values
  const cpuUsage = health?.cpu ?? 0;
  const ramUsage = health?.ram_percent ?? 0;
  const diskUsage = health?.disk_percent ?? 0;

  // Determine overall health status
  const healthStatus = (() => {
    if (!health) return 'unknown';

    const maxUsage = Math.max(cpuUsage, ramUsage, diskUsage);
    if (maxUsage > 90) return 'critical';
    if (maxUsage > 75) return 'warning';
    return 'healthy';
  })();

  return {
    health,
    loading,
    error,
    refresh,
    healthStatus,
    cpuUsage,
    ramUsage,
    diskUsage,
  };
}

export default useNodeHealth;
