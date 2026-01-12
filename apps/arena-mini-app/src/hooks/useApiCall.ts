import { useState } from 'react';
import { noticeError } from '@beef-briefing/shared-mini-app';

interface UseApiCallOptions<T> {
  onSuccess?: (data: T) => void;
  onError?: (error: string) => void;
  context?: string;
}

interface UseApiCallResult<T> {
  execute: (apiFn: () => Promise<T>) => Promise<T | null>;
  loading: boolean;
  error: string | null;
  clearError: () => void;
}

/**
 * Custom hook for managing API calls with consistent error handling.
 * Reduces duplication of try-catch patterns across components.
 *
 * @param options - Optional callbacks and context for error tracking
 * @returns Object with execute function, loading/error states, and clearError function
 *
 * @example
 * const { execute, loading, error } = useApiCall({
 *   context: 'fetching matches',
 *   onError: (err) => showToast(err)
 * });
 *
 * const handleFetch = async () => {
 *   const result = await execute(() => apiClient.getMatches());
 *   if (result) {
 *     setMatches(result);
 *   }
 * };
 */
export function useApiCall<T>(
  options?: UseApiCallOptions<T>,
): UseApiCallResult<T> {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const execute = async (apiFn: () => Promise<T>): Promise<T | null> => {
    try {
      setLoading(true);
      setError(null);

      const result = await apiFn();

      setLoading(false);

      if (options?.onSuccess) {
        options.onSuccess(result);
      }

      return result;
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'An error occurred';
      setError(errorMessage);
      setLoading(false);

      // Log to New Relic if we have an Error instance
      if (err instanceof Error && options?.context) {
        noticeError(err, { context: options.context });
      }

      if (options?.onError) {
        options.onError(errorMessage);
      }

      return null;
    }
  };

  const clearError = () => setError(null);

  return { execute, loading, error, clearError };
}
