import { CountdownTimer } from '../common/CountdownTimer'
import type { MatchResponse, ParticipantWithUser } from '../../types'

interface MatchCardProps {
  match: MatchResponse
  currentUserId?: number
  onJoin: (matchId: string) => void
  onLeave: (matchId: string) => void
  onStart: (matchId: string) => void
  onView: (matchId: string) => void
  isLoading?: boolean
}

/**
 * Individual match card displaying match info, participants, and action buttons.
 */
export function MatchCard({
  match,
  currentUserId,
  onJoin,
  onLeave,
  onStart,
  onView,
  isLoading = false,
}: MatchCardProps) {
  const isCreator = match.creator_user_id === currentUserId
  const isParticipant = match.participants.some(p => p.user_id === currentUserId)
  const canStart = isCreator && match.participants.length >= 2 && match.status === 'open'
  const canJoin = !isParticipant && match.status === 'open'
  const canLeave = isParticipant && !isCreator && match.status === 'open'

  const isActive = match.status === 'shop_phase' || match.status === 'battle_phase'

  return (
    <div className={`match-card ${isActive ? 'active' : ''}`}>
      <div className="match-card-header">
        <span className="match-card-title">
          {match.match_type === 'ranked' ? '⚔️ Ranked Match' : '🎮 Casual Match'}
        </span>
        <span className={`match-card-badge ${match.match_type === 'ranked' ? 'ranked' : 'casual'}`}>
          {match.match_type === 'ranked' ? 'Ranked' : 'Casual'}
        </span>
      </div>

      {/* Join deadline countdown for open matches */}
      {match.status === 'open' && match.join_deadline && (
        <div className="match-card-countdown">
          <CountdownTimer deadline={match.join_deadline} compact />
          <span className="match-card-countdown-label">to join</span>
        </div>
      )}

      {/* Participants list */}
      <ParticipantAvatars participants={match.participants} />

      {/* Action buttons */}
      <div className="match-card-actions">
        {canJoin && (
          <button
            className="btn btn-primary btn-sm"
            onClick={() => onJoin(match.id)}
            disabled={isLoading}
          >
            {isLoading ? <><span className="btn-spinner" />Joining...</> : 'Join'}
          </button>
        )}
        {canLeave && (
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => onLeave(match.id)}
            disabled={isLoading}
          >
            {isLoading ? <><span className="btn-spinner" />Leaving...</> : 'Leave'}
          </button>
        )}
        {canStart && (
          <button
            className="btn btn-action btn-sm"
            onClick={() => onStart(match.id)}
            disabled={isLoading}
          >
            {isLoading ? <><span className="btn-spinner" />Starting...</> : 'Start Now'}
          </button>
        )}
        {isActive && isParticipant && (
          <button
            className="btn btn-primary btn-sm"
            onClick={() => onView(match.id)}
          >
            {match.status === 'shop_phase' ? 'Go to Shop' : 'Watch Battle'}
          </button>
        )}
        {match.status === 'completed' && (
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => onView(match.id)}
          >
            View Results
          </button>
        )}
      </div>
    </div>
  )
}

interface ParticipantAvatarsProps {
  participants: ParticipantWithUser[]
  maxDisplay?: number
}

/**
 * Overlapping avatar display for match participants.
 */
function ParticipantAvatars({ participants, maxDisplay = 5 }: ParticipantAvatarsProps) {
  const displayParticipants = participants.slice(0, maxDisplay)
  const extraCount = participants.length - maxDisplay

  return (
    <div className="match-card-participants">
      {displayParticipants.map((participant) => (
        <div
          key={participant.user_id}
          className="participant-avatar"
          title={participant.first_name}
        >
          {getInitials(participant.first_name)}
        </div>
      ))}
      {extraCount > 0 && (
        <div className="participant-avatar">+{extraCount}</div>
      )}
      <span className="participant-count">
        {participants.length} player{participants.length !== 1 ? 's' : ''}
      </span>
    </div>
  )
}

function getInitials(name: string): string {
  return name
    .split(' ')
    .map(n => n.charAt(0))
    .join('')
    .toUpperCase()
    .slice(0, 2)
}
