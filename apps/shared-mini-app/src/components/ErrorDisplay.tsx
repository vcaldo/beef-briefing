interface ErrorDisplayProps {
  error: string
  onRetry?: () => void
  onBack?: () => void
  title?: string
}

export function ErrorDisplay({
  error,
  onRetry,
  onBack,
  title = 'Something went wrong',
}: ErrorDisplayProps) {
  return (
    <div className="error-display">
      <div className="error-display-icon">⚠️</div>
      <div className="error-display-title">{title}</div>
      <div className="error-display-message">{error}</div>
      <div className="error-display-actions">
        {onRetry && (
          <button className="btn btn-primary" onClick={onRetry}>
            Try Again
          </button>
        )}
        {onBack && (
          <button className="btn btn-secondary" onClick={onBack}>
            Go Back
          </button>
        )}
      </div>
    </div>
  )
}
