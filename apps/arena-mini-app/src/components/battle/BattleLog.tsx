/**
 * Battle Log Component
 *
 * Displays a compact, scrollable log of battle events grouped by round.
 * Features:
 * - Events grouped by round with icons showing what happened
 * - RPG-themed styling with brown/beige borders
 * - Auto-scroll to latest round
 * - Animation support for live battle playback
 * - Reusable for battle history (static display)
 */

import { useRef, useEffect, useMemo } from 'react'
import type { BattleEvent, EventAnimationPhase, EventType } from '../../types'
import type { IconImageId } from '../../types/images'
import { useImages } from '../../hooks/useImages'
import { RPGPanel } from '../ui/RPGPanel'

/**
 * Group of events for a single round
 */
interface RoundGroup {
  round: number
  events: BattleEvent[]
}

/**
 * Groups battle events by their round number
 */
function groupEventsByRound(events: BattleEvent[]): RoundGroup[] {
  const groups = new Map<number, BattleEvent[]>()

  for (const event of events) {
    const existing = groups.get(event.round)
    if (existing) {
      existing.push(event)
    } else {
      groups.set(event.round, [event])
    }
  }

  // Convert to array and sort by round number
  return Array.from(groups.entries())
    .sort((a, b) => a[0] - b[0])
    .map(([round, events]) => ({ round, events }))
}

/**
 * Maps event type to icon ID
 */
function getIconForEventType(type: EventType): IconImageId {
  switch (type) {
    case 'attack':
      return 'sword'
    case 'damage':
      return 'heart_broken'
    case 'death':
      return 'skull'
    case 'advance':
      return 'arrow_right_board'
    case 'victory':
      return 'crown'
    case 'summary':
      return 'book_open'
    default:
      return 'book_open'
  }
}

/**
 * Determines which side (left/right) an event should align to.
 * Left = Team A (top player), Right = Team B (bottom player)
 */
function getEventAlignment(
  event: BattleEvent,
  playerAId: number | undefined
): 'left' | 'right' {
  if (!playerAId) return 'left'

  // Attack/advance events belong to the attacker's side
  if (event.attacker_team_owner_id) {
    return event.attacker_team_owner_id === playerAId ? 'left' : 'right'
  }
  // Damage/death events belong to the defender's side
  if (event.defender_team_owner_id) {
    return event.defender_team_owner_id === playerAId ? 'left' : 'right'
  }
  // Default to left
  return 'left'
}

/**
 * Gets the front card stats for a player at the start of a round.
 * Returns { atk, hp } for the front-most alive card.
 */
function getFrontCardStats(
  allEvents: BattleEvent[],
  roundNumber: number,
  playerId: number
): { atk: number; hp: number } | null {
  // Find the card states to use:
  // - For round 1: use the first event's card_states (approximate)
  // - For round N: use the last event of round N-1
  let cardStates: BattleEvent['card_states'] | undefined

  if (roundNumber === 1) {
    // Use first event's card_states as approximation
    const firstEvent = allEvents[0]
    cardStates = firstEvent?.card_states
  } else {
    // Find last event of previous round
    const prevRoundEvents = allEvents.filter((e) => e.round === roundNumber - 1)
    const lastPrevEvent = prevRoundEvents[prevRoundEvents.length - 1]
    cardStates = lastPrevEvent?.card_states
  }

  if (!cardStates) return null

  // Find the front-most alive card for this player
  const playerCards = cardStates
    .filter((c) => c.user_id === playerId && c.is_alive)
    .sort((a, b) => a.position - b.position)

  const frontCard = playerCards[0]
  if (!frontCard) return null

  return { atk: frontCard.atk, hp: frontCard.hp }
}

export interface BattleLogProps {
  /** List of all battle events */
  events: BattleEvent[]
  /** Current event index being displayed (0-based). Optional for static display. */
  currentEventIndex?: number
  /** Current animation phase. Optional for static display (battle history). */
  currentPhase?: EventAnimationPhase
  /** Function to format event message (optional, for tooltip display) */
  getEventMessage?: (event: BattleEvent) => string
  /** Whether to show animation indicators (default: true) */
  animated?: boolean
  /** Custom className for additional styling */
  className?: string
  /** Player A (top team) user ID */
  playerAId?: number
  /** Player B (bottom team) user ID */
  playerBId?: number
  /** Player A display name */
  playerAName?: string
  /** Player B display name */
  playerBName?: string
  /** Current user's ID (to show "You" instead of name) */
  currentUserId?: number
}

/**
 * Props for the internal BattleLogRound component
 */
interface BattleLogRoundProps {
  roundGroup: RoundGroup
  isCurrentRound: boolean
  isAnimating: boolean
  isPlayed: boolean
  getIconUrl: (type: EventType) => string
  getEventMessage: (event: BattleEvent) => string
  /** Player A (left side) user ID */
  playerAId?: number
  /** Player B (right side) user ID */
  playerBId?: number
  /** Player A display name (or "You" if current user) */
  playerALabel: string
  /** Player B display name (or "You" if current user) */
  playerBLabel: string
  /** Whether current user is Player A */
  isCurrentUserPlayerA: boolean
  /** All events (for looking up card stats) */
  allEvents: BattleEvent[]
}

/**
 * Single round frame in the battle log with all events listed vertically
 */
function BattleLogRound({
  roundGroup,
  isCurrentRound,
  isAnimating,
  isPlayed,
  getIconUrl,
  getEventMessage,
  playerAId,
  playerBId,
  playerALabel,
  playerBLabel,
  isCurrentUserPlayerA,
  allEvents,
}: BattleLogRoundProps) {
  const classes = [
    'battle-log-round',
    isCurrentRound ? 'current' : '',
    isAnimating ? 'animating' : '',
    isPlayed ? 'played' : '',
    isCurrentRound && !isPlayed ? 'slide-in' : '',
  ]
    .filter(Boolean)
    .join(' ')

  // Get front card stats for each player at the start of this round
  const playerAStats = playerAId
    ? getFrontCardStats(allEvents, roundGroup.round, playerAId)
    : null
  const playerBStats = playerBId
    ? getFrontCardStats(allEvents, roundGroup.round, playerBId)
    : null

  // Format player label with stats: "Name (ATK/HP)"
  const formatPlayerWithStats = (
    label: string,
    stats: { atk: number; hp: number } | null
  ) => {
    if (!stats) return label
    return `${label} (${stats.atk}/${stats.hp})`
  }

  return (
    <div className={classes}>
      <div className="round-header">
        <span className={`player-name ${isCurrentUserPlayerA ? 'is-you' : ''}`}>
          {formatPlayerWithStats(playerALabel, playerAStats)}
        </span>
        <span className="round-number">R{roundGroup.round}</span>
        <span className={`player-name ${!isCurrentUserPlayerA ? 'is-you' : ''}`}>
          {formatPlayerWithStats(playerBLabel, playerBStats)}
        </span>
      </div>
      <div className="round-events">
        {roundGroup.events.map((event, idx) => {
          const alignment = getEventAlignment(event, playerAId)
          return (
            <div
              key={idx}
              className={`event-row event-row-${event.type} align-${alignment}`}
            >
              <img
                src={getIconUrl(event.type)}
                alt={event.type}
                className={`event-icon event-icon-${event.type}`}
              />
              <span className="event-message">{getEventMessage(event)}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

export const BattleLog = ({
  events,
  currentEventIndex = -1,
  currentPhase = 'idle',
  getEventMessage = (event) => event.message || event.type,
  animated = true,
  className = '',
  playerAId,
  playerBId,
  playerAName = 'Player A',
  playerBName = 'Player B',
  currentUserId,
}: BattleLogProps) => {
  const contentRef = useRef<HTMLDivElement>(null)
  const { getUrlById } = useImages()

  /**
   * Compute player labels - show "You" for current user
   */
  const isCurrentUserPlayerA = currentUserId !== undefined && currentUserId === playerAId
  const playerALabel = isCurrentUserPlayerA ? 'You' : playerAName
  const playerBLabel = !isCurrentUserPlayerA && currentUserId !== undefined ? 'You' : playerBName

  /**
   * Derived state: Is the current event still animating?
   */
  const isCurrentEventAnimating =
    animated &&
    currentEventIndex >= 0 &&
    currentPhase !== 'idle' &&
    currentPhase !== 'complete'

  /**
   * Get visible events based on currentEventIndex
   * - When not animated: show all events (static display for battle history)
   * - When animated and index < 0: show nothing (waiting for animation to start)
   * - When animated and index >= 0: show events up to current index
   */
  const visibleEvents = useMemo(() => {
    if (!animated) {
      return events
    }
    if (currentEventIndex < 0) {
      return []
    }
    return events.slice(0, currentEventIndex + 1)
  }, [events, currentEventIndex, animated])

  /**
   * Group visible events by round
   */
  const roundGroups = useMemo(
    () => groupEventsByRound(visibleEvents),
    [visibleEvents]
  )

  /**
   * Determine which round the current event belongs to
   */
  const currentRound = useMemo(() => {
    if (currentEventIndex < 0 || currentEventIndex >= events.length) {
      return -1
    }
    return events[currentEventIndex]?.round ?? -1
  }, [events, currentEventIndex])

  /**
   * Auto-scroll to bottom when new events appear
   */
  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight
    }
  }, [visibleEvents.length])

  /**
   * Get icon URL for event type
   */
  const getIconUrl = (type: EventType): string => {
    return getUrlById(getIconForEventType(type))
  }

  return (
    <RPGPanel variant="outer" className={`battle-log-outer ${className}`}>
      <RPGPanel variant="inner" className="battle-log-inner">
        <div className="battle-log-content" ref={contentRef}>
          {roundGroups.map((roundGroup) => {
            const isCurrentRound = roundGroup.round === currentRound
            const isPlayed = roundGroup.round < currentRound

            return (
              <BattleLogRound
                key={roundGroup.round}
                roundGroup={roundGroup}
                isCurrentRound={isCurrentRound}
                isAnimating={isCurrentRound && isCurrentEventAnimating}
                isPlayed={isPlayed}
                getIconUrl={getIconUrl}
                getEventMessage={getEventMessage}
                playerAId={playerAId}
                playerBId={playerBId}
                playerALabel={playerALabel}
                playerBLabel={playerBLabel}
                isCurrentUserPlayerA={isCurrentUserPlayerA}
                allEvents={events}
              />
            )
          })}
        </div>
      </RPGPanel>
    </RPGPanel>
  )
}
