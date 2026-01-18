/**
 * useErrorBanner - Error state management with auto-dismiss
 *
 * Features:
 * - Manages error message state
 * - Auto-dismisses after configurable timeout (default: 5000ms)
 * - Properly cleans up timeout on unmount or error change
 * - Provides manual dismiss function
 * - Helper function to extract error message from unknown errors
 */

import { useState, useEffect, useCallback } from 'react'

export interface UseErrorBannerOptions {
  /** Auto-dismiss timeout in milliseconds (default: 5000) */
  dismissTimeout?: number
}

export interface UseErrorBannerReturn {
  /** Current error message, or null if no error */
  error: string | null
  /** Set an error message to display */
  setError: (message: string | null) => void
  /** Clear the current error */
  clearError: () => void
  /** Helper to set error from an unknown error type with a fallback message */
  showError: (err: unknown, fallbackMessage: string) => void
}

/**
 * Hook for managing error banner state with auto-dismiss.
 *
 * Usage:
 * ```tsx
 * const { error, showError, clearError } = useErrorBanner()
 *
 * // In an async handler
 * try {
 *   await apiCall()
 * } catch (err) {
 *   showError(err, 'Failed to load data')
 * }
 *
 * // In JSX
 * {error && (
 *   <div className="error-banner">
 *     <span>{error}</span>
 *     <button onClick={clearError}>×</button>
 *   </div>
 * )}
 * ```
 *
 * @param options Configuration options
 * @returns Error state and control functions
 */
export function useErrorBanner(options: UseErrorBannerOptions = {}): UseErrorBannerReturn {
  const { dismissTimeout = 5000 } = options

  const [error, setError] = useState<string | null>(null)

  // Auto-dismiss error after timeout
  useEffect(() => {
    if (error) {
      const timeout = setTimeout(() => setError(null), dismissTimeout)
      return () => clearTimeout(timeout)
    }
  }, [error, dismissTimeout])

  const clearError = useCallback(() => {
    setError(null)
  }, [])

  const showError = useCallback((err: unknown, fallbackMessage: string) => {
    setError(err instanceof Error ? err.message : fallbackMessage)
  }, [])

  return {
    error,
    setError,
    clearError,
    showError,
  }
}

export default useErrorBanner
