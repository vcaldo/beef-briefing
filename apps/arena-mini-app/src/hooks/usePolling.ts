/**
 * usePolling - Generic polling hook with cleanup
 *
 * Features:
 * - Polls an async function at specified intervals
 * - Automatically cleans up on unmount
 * - Supports enable/disable via `enabled` flag
 * - Prevents stale closure issues via refs
 * - Calls onSuccess/onError callbacks with results
 */

import { useEffect, useRef, useCallback } from 'react'

export interface UsePollingOptions<T> {
  /** Async function to call on each poll */
  fetchFn: () => Promise<T>
  /** Interval in milliseconds between polls */
  interval: number
  /** Whether polling is enabled (default: true) */
  enabled?: boolean
  /** Callback when fetchFn succeeds */
  onSuccess?: (data: T) => void
  /** Callback when fetchFn throws */
  onError?: (error: Error) => void
}

/**
 * Generic polling hook that handles cleanup and stale closures.
 *
 * Usage:
 * ```tsx
 * usePolling({
 *   fetchFn: () => apiClient.getMatches(chatId),
 *   interval: 3000,
 *   enabled: !loading,
 *   onSuccess: (matches) => setMatches(matches),
 *   onError: (err) => console.error(err),
 * })
 * ```
 *
 * @param options Polling configuration
 */
export function usePolling<T>({
  fetchFn,
  interval,
  enabled = true,
  onSuccess,
  onError,
}: UsePollingOptions<T>): void {
  // Track mounted state to prevent state updates after unmount
  const isMountedRef = useRef(true)
  // Store interval ID for cleanup
  const intervalRef = useRef<number | null>(null)
  // Store latest callbacks in refs to avoid stale closures
  const fetchFnRef = useRef(fetchFn)
  const onSuccessRef = useRef(onSuccess)
  const onErrorRef = useRef(onError)

  // Keep refs up to date with latest values
  useEffect(() => {
    fetchFnRef.current = fetchFn
  }, [fetchFn])

  useEffect(() => {
    onSuccessRef.current = onSuccess
  }, [onSuccess])

  useEffect(() => {
    onErrorRef.current = onError
  }, [onError])

  // The actual polling function that uses refs
  const poll = useCallback(async () => {
    if (!isMountedRef.current) return

    try {
      const data = await fetchFnRef.current()

      // Double-check mounted state after async operation
      if (!isMountedRef.current) return

      onSuccessRef.current?.(data)
    } catch (err) {
      if (!isMountedRef.current) return

      const error = err instanceof Error ? err : new Error(String(err))
      onErrorRef.current?.(error)
    }
  }, [])

  // Setup and cleanup polling
  useEffect(() => {
    isMountedRef.current = true

    if (!enabled) {
      // Clear any existing interval when disabled
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
      return
    }

    // Fetch immediately on mount/enable
    poll()

    // Setup interval for continuous polling
    intervalRef.current = window.setInterval(poll, interval)

    // Cleanup on unmount or when dependencies change
    return () => {
      isMountedRef.current = false
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
  }, [enabled, interval, poll])
}

export default usePolling
