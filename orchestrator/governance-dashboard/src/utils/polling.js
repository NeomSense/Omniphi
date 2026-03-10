/**
 * Lightweight polling hook for React.
 */
import { useEffect, useRef, useState, useCallback } from 'react';

const POLL_MS = Number(import.meta.env.VITE_POLL_MS) || 8000;

/**
 * usePolling – calls `fetchFn` immediately, then every `intervalMs`.
 *
 * Returns { data, error, loading, refresh }.
 *
 * - Automatically cleans up on unmount.
 * - Skips overlapping fetches.
 * - `refresh()` triggers an immediate re-fetch.
 */
export function usePolling(fetchFn, deps = [], intervalMs = POLL_MS) {
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(true);
  const inflightRef = useRef(false);
  const mountedRef = useRef(true);

  const doFetch = useCallback(async () => {
    if (inflightRef.current) return;
    inflightRef.current = true;
    try {
      const result = await fetchFn();
      if (mountedRef.current) {
        setData(result);
        setError(null);
      }
    } catch (err) {
      if (mountedRef.current) {
        setError(err);
      }
    } finally {
      inflightRef.current = false;
      if (mountedRef.current) {
        setLoading(false);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  useEffect(() => {
    mountedRef.current = true;
    setLoading(true);
    doFetch();

    const id = setInterval(doFetch, intervalMs);
    return () => {
      mountedRef.current = false;
      clearInterval(id);
    };
  }, [doFetch, intervalMs]);

  return { data, error, loading, refresh: doFetch };
}
