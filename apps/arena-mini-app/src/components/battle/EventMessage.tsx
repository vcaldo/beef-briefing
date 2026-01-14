import type { BattleEvent } from '../../types'

interface EventMessageProps {
  /** Battle event to display */
  event: BattleEvent
  /** Whether this event involves the current user's team */
  isYourTeam?: boolean
}

/**
 * Individual event message in the battle log.
 * Styled based on event type (attack, death, victory, etc).
 */
export function EventMessage({ event, isYourTeam }: EventMessageProps) {
  // Determine event class based on type
  const getEventClass = () => {
    switch (event.type) {
      case 'attack':
      case 'damage':
        return 'attack'
      case 'death':
        return 'death'
      case 'victory':
        return 'victory'
      case 'summary':
        return 'summary'
      case 'advance':
        return 'advance'
      default:
        return ''
    }
  }

  // Get event icon
  const getEventIcon = () => {
    switch (event.type) {
      case 'attack':
      case 'damage':
        return '⚔️'
      case 'death':
        return '💀'
      case 'victory':
        return '🏆'
      case 'summary':
        return '📊'
      case 'advance':
        return '➡️'
      default:
        return '📝'
    }
  }

  // Format the message
  const formatMessage = () => {
    if (event.message) {
      return event.message
    }

    switch (event.type) {
      case 'attack':
        return `Attack dealt ${event.damage || 0} damage!`
      case 'damage':
        return `Took ${event.damage || 0} damage (${event.hp_before} → ${event.hp_after} HP)`
      case 'death':
        return 'Card was eliminated!'
      case 'victory':
        return 'Victory!'
      case 'summary':
        return formatSummary()
      case 'advance':
        return 'Next card advances...'
      default:
        return 'Battle event'
    }
  }

  // Format summary message
  const formatSummary = () => {
    const parts: string[] = []
    if (event.total_damage_dealt !== undefined) {
      parts.push(`Dealt ${event.total_damage_dealt} total damage`)
    }
    if (event.kill_streak !== undefined && event.kill_streak > 0) {
      parts.push(`${event.kill_streak} kills`)
    }
    if (event.rounds_in_duel !== undefined) {
      parts.push(`${event.rounds_in_duel} rounds`)
    }
    return parts.join(' • ') || 'Battle summary'
  }

  return (
    <div className={`event-message ${getEventClass()} ${isYourTeam ? 'your-team' : ''}`}>
      <span className="event-icon">{getEventIcon()}</span>
      <span className="event-round">R{event.round}</span>
      <span className="event-text">{formatMessage()}</span>
      {event.damage !== undefined && event.type === 'attack' && (
        <span className="event-damage">-{event.damage}</span>
      )}
    </div>
  )
}
