interface ErrorDisplayProps {
  /** Error title */
  title?: string
  /** Error message */
  message: string
  /** Retry callback */
  onRetry?: () => void
  /** Full screen mode */
  fullScreen?: boolean
}

/**
 * Error display component for API errors and failed states.
 */
export function ErrorDisplay({
  title = 'Something went wrong',
  message,
  onRetry,
  fullScreen = false,
}: ErrorDisplayProps) {
  const content = (
    <div className="error-content">
      <div className="error-icon">!</div>
      <h2>{title}</h2>
      <p>{message}</p>
      {onRetry && (
        <button className="btn btn-primary mt-4" onClick={onRetry}>
          Try Again
        </button>
      )}
    </div>
  )

  if (fullScreen) {
    return <div className="error-container">{content}</div>
  }

  return (
    <div className="error-content py-8 text-center">
      <div className="error-icon mx-auto">!</div>
      <h2 className="mt-4 font-display font-bold">{title}</h2>
      <p className="mt-2 text-[var(--tg-theme-hint-color)]">{message}</p>
      {onRetry && (
        <button className="btn btn-primary mt-4" onClick={onRetry}>
          Try Again
        </button>
      )}
    </div>
  )
}
