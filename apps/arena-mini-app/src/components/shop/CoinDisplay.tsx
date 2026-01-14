interface CoinDisplayProps {
  /** Number of coins */
  coins: number
  /** Coin cost to show affordability (optional) */
  cost?: number
}

/**
 * Coin display with optional affordability indicator.
 * Shows coin icon with animated shine effect.
 */
export function CoinDisplay({ coins, cost }: CoinDisplayProps) {
  const canAfford = cost === undefined || coins >= cost
  const affordabilityClass = cost !== undefined
    ? canAfford
      ? 'affordable'
      : 'not-affordable'
    : ''

  return (
    <div className={`coin-display ${affordabilityClass}`}>
      <span className="coin-icon">🪙</span>
      <span className="coin-value">{coins}</span>
      {cost !== undefined && (
        <span className="coin-cost">/ {cost}</span>
      )}
    </div>
  )
}
