import type { ParticipantWithUser } from '../../types'

interface ParticipantsListProps {
  participants: ParticipantWithUser[]
  currentUserId?: number
  creatorUserId?: number
}

/**
 * Full list of participants with status indicators.
 * Used in expanded match view or match details modal.
 */
export function ParticipantsList({
  participants,
  currentUserId,
  creatorUserId,
}: ParticipantsListProps) {
  if (participants.length === 0) {
    return (
      <div className="participants-list-empty">
        <span className="participants-list-empty-text">
          No players yet. Be the first to join!
        </span>
      </div>
    )
  }

  return (
    <div className="participants-list">
      {participants.map((participant) => (
        <ParticipantItem
          key={participant.user_id}
          participant={participant}
          isCurrentUser={participant.user_id === currentUserId}
          isCreator={participant.user_id === creatorUserId}
        />
      ))}
    </div>
  )
}

interface ParticipantItemProps {
  participant: ParticipantWithUser
  isCurrentUser: boolean
  isCreator: boolean
}

function ParticipantItem({ participant, isCurrentUser, isCreator }: ParticipantItemProps) {
  const statusIcon = getStatusIcon(participant.status)

  return (
    <div className={`participant-item ${isCurrentUser ? 'current-user' : ''}`}>
      <div className="participant-avatar-lg">
        {getInitials(participant.first_name)}
      </div>
      <div className="participant-info">
        <span className="participant-name">
          {participant.first_name}
          {isCurrentUser && <span className="participant-tag you">You</span>}
          {isCreator && <span className="participant-tag creator">Host</span>}
        </span>
        {participant.username && (
          <span className="participant-username">@{participant.username}</span>
        )}
      </div>
      <div className="participant-status">
        <span className="participant-status-icon">{statusIcon}</span>
        <span className="participant-status-text">{formatStatus(participant.status)}</span>
      </div>
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

function getStatusIcon(status: string): string {
  switch (status) {
    case 'ready':
      return '✓'
    case 'winner':
      return '👑'
    case 'eliminated':
      return '💀'
    default:
      return '⏳'
  }
}

function formatStatus(status: string): string {
  switch (status) {
    case 'joined':
      return 'Waiting'
    case 'ready':
      return 'Ready'
    case 'winner':
      return 'Winner'
    case 'eliminated':
      return 'Eliminated'
    default:
      return status
  }
}
