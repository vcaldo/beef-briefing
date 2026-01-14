import { MatchCard } from './MatchCard'
import type { MatchResponse } from '../../types'

interface MatchListProps {
  matches: MatchResponse[]
  currentUserId?: number
  onJoin: (matchId: string) => void
  onLeave: (matchId: string) => void
  onStart: (matchId: string) => void
  onView: (matchId: string) => void
  loadingMatchId?: string | null
}

/**
 * Renders a list of match cards with empty state handling.
 */
export function MatchList({
  matches,
  currentUserId,
  onJoin,
  onLeave,
  onStart,
  onView,
  loadingMatchId,
}: MatchListProps) {
  if (matches.length === 0) {
    return (
      <div className="empty-state">
        <div className="empty-state-icon">🎯</div>
        <h3 className="empty-state-title">No Matches Yet</h3>
        <p className="empty-state-text">
          Be the first to create a match and challenge others!
        </p>
      </div>
    )
  }

  return (
    <div className="match-list">
      {matches.map((match) => (
        <MatchCard
          key={match.id}
          match={match}
          currentUserId={currentUserId}
          onJoin={onJoin}
          onLeave={onLeave}
          onStart={onStart}
          onView={onView}
          isLoading={loadingMatchId === match.id}
        />
      ))}
    </div>
  )
}
