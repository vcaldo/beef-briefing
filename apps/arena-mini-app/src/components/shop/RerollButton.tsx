interface RerollButtonProps {
  /** Whether reroll is available */
  canReroll: boolean
  /** Reason why reroll is disabled */
  disabledReason?: string
  /** Whether reroll is loading */
  isLoading: boolean
  /** Reroll callback */
  onReroll: () => void
}

/**
 * Reroll button for refreshing shop cards.
 * Only available before first card purchase (permanent lockout after).
 */
export function RerollButton({
  canReroll,
  disabledReason,
  isLoading,
  onReroll,
}: RerollButtonProps) {
  return (
    <button
      className="reroll-btn"
      onClick={onReroll}
      disabled={!canReroll || isLoading}
      title={disabledReason}
    >
      {isLoading ? (
        <div className="btn-spinner" />
      ) : (
        <>
          <span className="reroll-icon">🔄</span>
          <span>Reroll</span>
          <span className="reroll-cost">🪙 1</span>
        </>
      )}
    </button>
  )
}
