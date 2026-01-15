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
  const ariaLabel = isLoading
    ? 'Rerolling shop cards...'
    : canReroll
      ? 'Reroll shop cards for 1 coin'
      : `Reroll unavailable: ${disabledReason || 'Not available'}`

  return (
    <button
      className="reroll-btn"
      onClick={onReroll}
      disabled={!canReroll || isLoading}
      title={disabledReason}
      aria-label={ariaLabel}
      aria-busy={isLoading}
    >
      {isLoading ? (
        <div className="btn-spinner" aria-hidden="true" />
      ) : (
        <>
          <span className="reroll-icon" aria-hidden="true">🔄</span>
          <span>Reroll</span>
          <span className="reroll-cost" aria-hidden="true">🪙 1</span>
        </>
      )}
    </button>
  )
}
