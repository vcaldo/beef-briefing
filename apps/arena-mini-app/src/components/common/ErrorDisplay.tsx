import React from 'react'

/**
 * ErrorDisplay - A component for displaying error messages with optional retry functionality
 *
 * Displays an error icon, title, message, and optional retry button.
 * Styled with danger/red colors following the arena theme.
 *
 * @example
 * // Basic error display
 * <ErrorDisplay message="Failed to load matches" />
 *
 * @example
 * // With retry button
 * <ErrorDisplay
 *   title="Connection Error"
 *   message="Unable to connect to the server"
 *   onRetry={() => refetch()}
 * />
 *
 * @example
 * // Inline error (without full-screen styling)
 * <ErrorDisplay
 *   message="Failed to load card"
 *   inline
 *   size="sm"
 * />
 *
 * @example
 * // Custom icon
 * <ErrorDisplay
 *   icon="🔒"
 *   title="Access Denied"
 *   message="You don't have permission to view this match"
 * />
 */

export type ErrorSize = 'sm' | 'md' | 'lg'

export interface ErrorDisplayProps {
  /** Error title (optional, defaults to "Something went wrong") */
  title?: string
  /** Error message to display */
  message: string
  /** Icon to display (defaults to warning icon) */
  icon?: React.ReactNode
  /** Callback when retry button is clicked */
  onRetry?: () => void
  /** Custom retry button text */
  retryText?: string
  /** If true, renders inline without full-container styling */
  inline?: boolean
  /** Size variant: 'sm', 'md' (default), 'lg' */
  size?: ErrorSize
  /** Additional CSS class names */
  className?: string
  /** Test ID for testing */
  'data-testid'?: string
}

/**
 * Default error icon (warning triangle)
 */
const DefaultErrorIcon: React.FC<{ size: ErrorSize }> = ({ size }) => {
  const sizeClasses = {
    sm: 'w-6 h-6',
    md: 'w-9 h-9',
    lg: 'w-12 h-12',
  }

  return (
    <svg
      className={sizeClasses[size]}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <path
        d="M12 9V13M12 17H12.01M10.29 3.86L1.82 18C1.64 18.3 1.55 18.64 1.55 19C1.55 19.36 1.64 19.7 1.82 20C2 20.3 2.25 20.55 2.55 20.73C2.85 20.91 3.19 21 3.55 21H20.47C20.83 21 21.17 20.91 21.47 20.73C21.77 20.55 22.02 20.3 22.2 20C22.38 19.7 22.47 19.36 22.47 19C22.47 18.64 22.38 18.3 22.2 18L13.73 3.86C13.55 3.56 13.3 3.31 13 3.13C12.7 2.95 12.36 2.86 12.01 2.86C11.66 2.86 11.32 2.95 11.02 3.13C10.72 3.31 10.47 3.56 10.29 3.86Z"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

/**
 * Get icon container size styles
 */
function getIconContainerStyles(size: ErrorSize): string {
  switch (size) {
    case 'sm':
      return 'w-12 h-12 text-xl'
    case 'lg':
      return 'w-20 h-20 text-4xl'
    default:
      return 'w-[72px] h-[72px] text-4xl' // matches .error-icon in global.css
  }
}

/**
 * Get title size styles
 */
function getTitleStyles(size: ErrorSize): string {
  switch (size) {
    case 'sm':
      return 'text-base'
    case 'lg':
      return 'text-2xl'
    default:
      return 'text-xl' // matches .error-title
  }
}

/**
 * Get message size styles
 */
function getMessageStyles(size: ErrorSize): string {
  switch (size) {
    case 'sm':
      return 'text-xs'
    case 'lg':
      return 'text-base'
    default:
      return 'text-sm' // matches .error-message
  }
}

export const ErrorDisplay: React.FC<ErrorDisplayProps> = ({
  title = 'Something went wrong',
  message,
  icon,
  onRetry,
  retryText = 'Try Again',
  inline = false,
  size = 'md',
  className = '',
  'data-testid': testId = 'error-display',
}) => {
  const iconContainerStyles = getIconContainerStyles(size)
  const titleStyles = getTitleStyles(size)
  const messageStyles = getMessageStyles(size)

  // Render the icon (custom or default)
  const renderIcon = () => {
    if (typeof icon === 'string') {
      // Emoji or text icon
      return <span aria-hidden="true">{icon}</span>
    }
    if (icon) {
      // Custom React node
      return icon
    }
    // Default SVG icon
    return <DefaultErrorIcon size={size} />
  }

  // Inline mode (compact, no full-container styling)
  if (inline) {
    return (
      <div
        className={`flex items-center gap-3 p-3 rounded-lg bg-[rgba(239,68,68,0.1)] border border-[var(--hp-low)] ${className}`}
        data-testid={testId}
        role="alert"
        aria-live="polite"
      >
        <div className={`flex-shrink-0 text-[var(--hp-low)] ${size === 'sm' ? 'w-5 h-5' : 'w-6 h-6'}`}>
          {renderIcon()}
        </div>
        <div className="flex-1 min-w-0">
          <p className={`text-[var(--hp-low)] font-medium ${size === 'sm' ? 'text-xs' : 'text-sm'}`}>
            {message}
          </p>
        </div>
        {onRetry && (
          <button
            onClick={onRetry}
            className={`flex-shrink-0 btn btn-danger ${size === 'sm' ? 'btn-sm' : ''}`}
            type="button"
          >
            {retryText}
          </button>
        )}
      </div>
    )
  }

  // Full container mode (centered in available space)
  return (
    <div
      className={`error-container ${className}`}
      data-testid={testId}
      role="alert"
      aria-live="polite"
    >
      <div className="error-content">
        {/* Icon container */}
        <div
          className={`error-icon ${iconContainerStyles}`}
          style={{
            background: 'rgba(239, 68, 68, 0.1)',
            color: 'var(--hp-low)',
            borderRadius: '50%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            margin: '0 auto 20px',
          }}
        >
          {renderIcon()}
        </div>

        {/* Title */}
        <h2
          className={`error-title font-display font-bold text-[var(--tg-theme-text-color)] mb-2 ${titleStyles}`}
        >
          {title}
        </h2>

        {/* Message */}
        <p
          className={`error-message text-[var(--tg-theme-hint-color)] leading-relaxed mb-6 ${messageStyles}`}
        >
          {message}
        </p>

        {/* Retry button */}
        {onRetry && (
          <button
            onClick={onRetry}
            className={`btn btn-danger ${size === 'lg' ? 'btn-lg' : ''}`}
            type="button"
          >
            {retryText}
          </button>
        )}
      </div>
    </div>
  )
}
