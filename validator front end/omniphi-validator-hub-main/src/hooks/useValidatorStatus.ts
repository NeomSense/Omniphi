import { useState, useEffect, useCallback } from 'react';
import { chainService, ValidatorInfo, ValidatorStatus } from '@/services/chain';

export const useValidatorStatus = (operatorAddress?: string, rpcEndpoint?: string) => {
  const [validator, setValidator] = useState<ValidatorInfo | null>(null);
  const [status, setStatus] = useState<ValidatorStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchValidatorInfo = useCallback(async () => {
    if (!operatorAddress) return;

    setLoading(true);
    setError(null);

    try {
      const validatorData = await chainService.getValidator(operatorAddress);
      setValidator(validatorData);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch validator info');
    } finally {
      setLoading(false);
    }
  }, [operatorAddress]);

  const fetchStatus = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const statusData = await chainService.getValidatorStatus(rpcEndpoint);
      setStatus(statusData);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch validator status');
    } finally {
      setLoading(false);
    }
  }, [rpcEndpoint]);

  const refresh = useCallback(() => {
    fetchValidatorInfo();
    fetchStatus();
  }, [fetchValidatorInfo, fetchStatus]);

  // Poll every 5 seconds
  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, 5000);
    return () => clearInterval(interval);
  }, [refresh]);

  return {
    validator,
    status,
    loading,
    error,
    refresh,
  };
};
