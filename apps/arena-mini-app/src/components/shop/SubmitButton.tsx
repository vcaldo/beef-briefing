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
  return (
    <button
      className="btn btn-primary btn-lg submit-btn"
      onClick={onSubmit}
      disabled={!canSubmit || isLoading}
      title={disabledReason}
    >
      {isLoading ? (
        <>
          <div className="btn-spinner" />
          <span>Submitting...</span>
        </>
      ) : (
        <>
          <span>⚔️</span>
          <span>Ready for Battle!</span>
        </>
      )}
    </button>
  )
}
