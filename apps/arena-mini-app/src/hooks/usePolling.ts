import { useEffect, useRef, useState } from 'react';

interface UsePollingOptions<T> {
  enabled?: boolean;
  stopWhen?: (data: T) => boolean;
}

interface UsePollingResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

/**
 * Custom hook for polling data at regular intervals.
 * Handles cleanup, error management, and optional early termination.
 *
 * @param fetchFn - Async function that returns the data
 * @param interval - Polling interval in milliseconds
 * @param options - Optional: enabled (default true), stopWhen (stops polling when condition met)
 * @returns Object with data, loading, error, and refetch function
 *
 * @example
 * const { data, loading, error } = usePolling(
 *   () => apiClient.getMatches(),
 *   5000,
 *   { stopWhen: (matches) => matches.length === 0 }
 * );
 */
export function usePolling<T>(
  fetchFn: () => Promise<T>,
  interval: number,
  options?: UsePollingOptions<T>,
): UsePollingResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const enabled = options?.enabled ?? true;
  const stopWhen = options?.stopWhen;
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const performFetch = async () => {
    try {
      setError(null);
      const result = await fetchFn();
      setData(result);

      // Check if we should stop polling
      if (stopWhen?.(result)) {
        if (intervalRef.current) {
          clearInterval(intervalRef.current);
          intervalRef.current = null;
        }
      }

      setLoading(false);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to fetch data';
      setError(errorMessage);
      setLoading(false);
    }
  };

  const refetch = () => {
    setLoading(true);
    performFetch();
  };

  useEffect(() => {
    if (!enabled) {
      return;
    }

    // Initial fetch
    performFetch();

    // Set up interval
    intervalRef.current = setInterval(performFetch, interval);

    // Cleanup
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [enabled, interval]);

  return { data, loading, error, refetch };
}
