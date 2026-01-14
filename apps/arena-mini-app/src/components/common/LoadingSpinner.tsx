interface LoadingSpinnerProps {
  /** Optional loading message */
  message?: string
  /** Size variant */
  size?: 'sm' | 'md' | 'lg'
  /** Full screen overlay mode */
  fullScreen?: boolean
}

/**
 * Loading spinner with optional message.
 * Used during API calls and page transitions.
 */
export function LoadingSpinner({
  message,
  size = 'md',
  fullScreen = false,
}: LoadingSpinnerProps) {
  const sizeClasses = {
    sm: 'w-6 h-6',
    md: 'w-10 h-10',
    lg: 'w-12 h-12',
  }

  const content = (
    <div className="loading-container">
      <div className={`spinner ${sizeClasses[size]}`} />
      {message && <p className="loading-text">{message}</p>}
    </div>
  )

  if (fullScreen) {
    return (
      <div className="fixed inset-0 flex items-center justify-center bg-[var(--tg-theme-bg-color)] z-50">
        {content}
      </div>
    )
  }

  return content
}
