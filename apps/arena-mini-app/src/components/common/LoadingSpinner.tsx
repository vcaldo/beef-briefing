import React from 'react'

/**
 * LoadingSpinner - A configurable loading spinner component
 *
 * Displays an animated spinner with optional message text.
 * Supports full-screen and inline display modes, with size variants.
 *
 * @example
 * // Full-screen loading (default)
 * <LoadingSpinner message="Loading match data..." />
 *
 * @example
 * // Inline spinner (small, no message)
 * <LoadingSpinner inline size="sm" />
 *
 * @example
 * // Large spinner with custom class
 * <LoadingSpinner size="lg" message="Preparing battle..." className="my-8" />
 */

export type SpinnerSize = 'sm' | 'md' | 'lg'

export interface LoadingSpinnerProps {
  /** Optional message to display below the spinner */
  message?: string
  /** Size variant: 'sm' (24px), 'md' (40px, default), 'lg' (56px) */
  size?: SpinnerSize
  /** If true, renders inline without full-container styling */
  inline?: boolean
  /** If true, renders as full-screen overlay (takes precedence over inline) */
  fullScreen?: boolean
  /** Additional CSS class names */
  className?: string
  /** Test ID for testing */
  'data-testid'?: string
}

/**
 * Get spinner size class based on size prop
 */
function getSpinnerSizeClass(size: SpinnerSize): string {
  switch (size) {
    case 'sm':
      return 'spinner-sm'
    case 'lg':
      return 'spinner-lg'
    default:
      return '' // md is the default size, no extra class needed
  }
}

export const LoadingSpinner: React.FC<LoadingSpinnerProps> = ({
  message,
  size = 'md',
  inline = false,
  fullScreen = false,
  className = '',
  'data-testid': testId = 'loading-spinner',
}) => {
  const spinnerSizeClass = getSpinnerSizeClass(size)

  // Full-screen overlay mode
  if (fullScreen) {
    return (
      <div
        className={`fixed inset-0 z-50 flex flex-col items-center justify-center bg-[var(--arena-bg-dark)]/90 backdrop-blur-sm ${className}`}
        data-testid={testId}
        role="status"
        aria-live="polite"
        aria-label={message || 'Loading'}
      >
        <div className={`spinner ${spinnerSizeClass}`} aria-hidden="true" />
        {message && (
          <p className="mt-4 text-sm text-[var(--tg-theme-hint-color)] font-medium">
            {message}
          </p>
        )}
        <span className="sr-only">{message || 'Loading...'}</span>
      </div>
    )
  }

  // Inline mode (just the spinner, optionally with message)
  if (inline) {
    return (
      <span
        className={`inline-flex items-center gap-2 ${className}`}
        data-testid={testId}
        role="status"
        aria-live="polite"
        aria-label={message || 'Loading'}
      >
        <span className={`spinner ${spinnerSizeClass}`} aria-hidden="true" />
        {message && (
          <span className="text-sm text-[var(--tg-theme-hint-color)]">
            {message}
          </span>
        )}
        <span className="sr-only">{message || 'Loading...'}</span>
      </span>
    )
  }

  // Default: container mode (centered in available space)
  return (
    <div
      className={`loading-container ${className}`}
      data-testid={testId}
      role="status"
      aria-live="polite"
      aria-label={message || 'Loading'}
    >
      <div className={`spinner ${spinnerSizeClass}`} aria-hidden="true" />
      {message && <p>{message}</p>}
      <span className="sr-only">{message || 'Loading...'}</span>
    </div>
  )
}
