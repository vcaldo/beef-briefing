interface SubmitButtonProps {
  /** Whether submit is available */
  canSubmit: boolean
  /** Reason why submit is disabled */
  disabledReason?: string
  /** Whether submit is loading */
  isLoading: boolean
  /** Submit callback */
  onSubmit: () => void
}

/**
 * Submit button for locking in team for battle.
 * Requires a full team of 3 cards.
 */
export function SubmitButton({
  canSubmit,
  disabledReason,
  isLoading,
  onSubmit,
}: SubmitButtonProps) {
  const ariaLabel = isLoading
    ? 'Submitting team...'
    : canSubmit
      ? 'Submit team and start battle'
      : `Cannot submit: ${disabledReason || 'Team not complete'}`

  return (
    <button
      className="btn btn-primary btn-lg submit-btn"
      onClick={onSubmit}
      disabled={!canSubmit || isLoading}
      title={disabledReason}
      aria-label={ariaLabel}
      aria-busy={isLoading}
    >
      {isLoading ? (
        <>
          <div className="btn-spinner" aria-hidden="true" />
          <span>Submitting...</span>
        </>
      ) : (
        <>
          <span aria-hidden="true">⚔️</span>
          <span>Ready for Battle!</span>
        </>
      )}
    </button>
  )
}
