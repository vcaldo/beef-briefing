import { useEffect, useRef } from 'react'

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
  const dialogRef = useRef<HTMLDivElement>(null)
  const firstButtonRef = useRef<HTMLButtonElement>(null)

  // Focus management: trap focus within dialog and focus first button on mount
  useEffect(() => {
    firstButtonRef.current?.focus()

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  return (
    <div
      className="victory-overlay"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="victory-title"
      aria-describedby="victory-subtitle"
    >
      <div
        ref={dialogRef}
        className="victory-content"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Result emoji */}
        <div className="victory-emoji" aria-hidden="true">
          {isWinner ? '🏆' : '😢'}
        </div>

        {/* Result title */}
        <h1
          id="victory-title"
          className={`victory-title ${isWinner ? '' : 'defeat'}`}
        >
          {isWinner ? 'Victory!' : 'Defeat'}
        </h1>

        {/* Winner subtitle */}
        {winnerName && (
          <p id="victory-subtitle" className="victory-subtitle">
            {isWinner
              ? 'You have won the battle!'
              : `${winnerName} wins this round`}
          </p>
        )}

        {/* Action buttons */}
        <div className="victory-actions" role="group" aria-label="Post-battle actions">
          <button
            ref={firstButtonRef}
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
        <p className="victory-hint" aria-hidden="true">
          Tap anywhere to close
        </p>
      </div>
    </div>
  )
}
