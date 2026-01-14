interface VictoryScreenProps {
  /** Whether current user is the winner */
  isWinner: boolean
  /** Name of the winner */
  winnerName?: string
  /** Close callback */
  onClose: () => void
  /** Navigate to stats */
  onViewStats?: () => void
  /** Navigate back to lobby */
  onBackToLobby?: () => void
}

/**
 * Victory/Defeat overlay shown at the end of battle.
 * Displays animated result with action buttons.
 */
export function VictoryScreen({
  isWinner,
  winnerName,
  onClose,
  onViewStats,
  onBackToLobby,
}: VictoryScreenProps) {
  return (
    <div className="victory-overlay" onClick={onClose}>
      <div
        className="victory-content"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Result emoji */}
        <div className="victory-emoji">
          {isWinner ? '🏆' : '😢'}
        </div>

        {/* Result title */}
        <h1 className={`victory-title ${isWinner ? '' : 'defeat'}`}>
          {isWinner ? 'Victory!' : 'Defeat'}
        </h1>

        {/* Winner subtitle */}
        {winnerName && (
          <p className="victory-subtitle">
            {isWinner
              ? 'You have won the battle!'
              : `${winnerName} wins this round`}
          </p>
        )}

        {/* Action buttons */}
        <div className="victory-actions">
          <button
            className="btn btn-secondary victory-btn"
            onClick={onBackToLobby}
          >
            Back to Lobby
          </button>
          <button
            className="btn btn-primary victory-btn"
            onClick={onViewStats}
          >
            View Stats
          </button>
        </div>

        {/* Tap to close hint */}
        <p className="victory-hint">
          Tap anywhere to close
        </p>
      </div>
    </div>
  )
}
