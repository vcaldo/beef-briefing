import React from 'react'

/**
 * ErrorBanner - A dismissible error notification banner
 *
 * Displays a transient error message with an icon and dismiss button.
 * Auto-dismisses after 5 seconds when used with the `useErrorBanner` hook.
 *
 * @example
 * const { error, showError, clearError } = useErrorBanner()
 *
 * {error && (
 *   <ErrorBanner
 *     message={error}
 *     onDismiss={clearError}
 *     className="shop-error-banner"
 *   />
 * )}
 */

export interface ErrorBannerProps {
  /** Error message to display */
  message: string
  /** Callback when dismiss button is clicked */
  onDismiss: () => void
  /** CSS class name for styling (e.g., "lobby-error-banner", "shop-error-banner") */
  className?: string
  /** Icon to display (defaults to ⚠️) */
  icon?: string
  /** Test ID for testing */
  'data-testid'?: string
}

export const ErrorBanner: React.FC<ErrorBannerProps> = ({
  message,
  onDismiss,
  className = 'error-banner',
  icon = '⚠️',
  'data-testid': testId = 'error-banner',
}) => {
  // Derive child class names from the parent className
  // e.g., "shop-error-banner" -> "shop-error-icon", "shop-error-text", "shop-error-dismiss"
  const baseClass = className.replace(/-banner$/, '')
  const iconClass = `${baseClass}-icon`
  const textClass = `${baseClass}-text`
  const dismissClass = `${baseClass}-dismiss`

  return (
    <div className={className} role="alert" data-testid={testId}>
      <span className={iconClass}>{icon}</span>
      <span className={textClass}>{message}</span>
      <button
        className={dismissClass}
        onClick={onDismiss}
        aria-label="Dismiss error"
        type="button"
      >
        ×
      </button>
    </div>
  )
}
