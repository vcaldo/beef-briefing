/**
 * Battle Log Component
 *
 * Displays a compact, scrollable log of battle events grouped by combat.
 * Features:
 * - Events grouped by combat (card duels) with centered matchup headers
 * - RPG-themed styling with brown/beige borders
 * - Auto-scroll to latest combat
 * - Animation support for live battle playback
 * - Reusable for battle history (static display)
 */

import { useRef, useEffect, useMemo } from 'react'
import type {
  Combat,
  CombatEvent,
  BattleEvent,
  EventAnimationPhase,
  EventType,
} from '../../types'
import type { IconImageId } from '../../types/images'
import { useImages } from '../../hooks/useImages'
import { RPGPanel } from '../ui/RPGPanel'

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
 * Determines which side (left/right/center) an event should align to.
 * Left = Team A (top player), Right = Team B (bottom player), Center = neutral
 */
function getCombatEventAlignment(
  event: CombatEvent,
  playerAId: number | undefined
): 'left' | 'right' | 'center' {
  // Summary events are always centered
  if (event.type === 'summary') return 'center'

  // Advance events without owner IDs are matchup displays - center them
  if (event.type === 'advance' && !event.attacker_team_owner_id) return 'center'

  // Victory events are centered
  if (event.type === 'victory') return 'center'

  if (!playerAId) return 'left'

  // Attack/advance events belong to the attacker's side
  if (event.attacker_team_owner_id) {
    return event.attacker_team_owner_id === playerAId ? 'left' : 'right'
  }
  // Damage/death events belong to the defender's side
  if (event.defender_team_owner_id) {
    return event.defender_team_owner_id === playerAId ? 'left' : 'right'
  }

  return 'center' // Unknown events default to center
}

/**
 * Extracts the matchup display text from a combat's events.
 * Returns "Card A vs Card B" based on the first attack event.
 */
function getCombatMatchup(
  combat: Combat,
  playerAId: number | undefined,
  playerAName: string,
  playerBName: string
): { leftName: string; rightName: string } | null {
  // Find the first attack event to determine who's fighting
  const firstAttack = combat.events.find((e) => e.type === 'attack')
  if (!firstAttack) return null

  // Determine card names from the message if available
  // Format is usually "CardA attacks CardB for X damage"
  const message = firstAttack.message || ''
  const attackMatch = message.match(/^(.+?) attacks (.+?) for/)

  if (attackMatch) {
    const [, attackerName, defenderName] = attackMatch
    // Determine which side is which based on attacker_team_owner_id
    if (firstAttack.attacker_team_owner_id === playerAId) {
      return { leftName: attackerName, rightName: defenderName }
    } else {
      return { leftName: defenderName, rightName: attackerName }
    }
  }

  // Fallback to player names
  return { leftName: playerAName, rightName: playerBName }
}

export interface BattleLogProps {
  /** List of combats (primary data source) */
  combats: Combat[]
  /** List of all battle events (for animation tracking) */
  events: BattleEvent[]
  /** Current event index being displayed (0-based). Optional for static display. */
  currentEventIndex?: number
  /** Current animation phase. Optional for static display (battle history). */
  currentPhase?: EventAnimationPhase
  /** Function to format event message (optional, for tooltip display) */
  getEventMessage?: (event: CombatEvent) => string
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
 * Props for the internal BattleLogCombat component
 */
interface BattleLogCombatProps {
  combat: Combat
  visibleEventCount: number
  isCurrentCombat: boolean
  isAnimating: boolean
  isPlayed: boolean
  getIconUrl: (type: EventType) => string
  getEventMessage: (event: CombatEvent) => string
  /** Player A (left side) user ID */
  playerAId?: number
  /** Player A display name (or "You" if current user) */
  playerALabel: string
  /** Player B display name (or "You" if current user) */
  playerBLabel: string
  /** Whether current user is Player A */
  isCurrentUserPlayerA: boolean
}

/**
 * Single combat frame in the battle log with all events listed vertically
 */
function BattleLogCombat({
  combat,
  visibleEventCount,
  isCurrentCombat,
  isAnimating,
  isPlayed,
  getIconUrl,
  getEventMessage,
  playerAId,
  playerALabel,
  playerBLabel,
  isCurrentUserPlayerA,
}: BattleLogCombatProps) {
  const classes = [
    'battle-log-combat',
    isCurrentCombat ? 'current' : '',
    isAnimating ? 'animating' : '',
    isPlayed ? 'played' : '',
    isCurrentCombat && !isPlayed ? 'slide-in' : '',
  ]
    .filter(Boolean)
    .join(' ')

  // Get matchup info from combat events
  const matchup = getCombatMatchup(combat, playerAId, playerALabel, playerBLabel)

  // Determine visible events within this combat
  const visibleEvents = combat.events.slice(0, visibleEventCount)

  return (
    <div className={classes}>
      <div className="combat-header">
        {combat.outcome === 'victory' ? (
          // Victory summary - no combat number, just player names
          <div className="combat-matchup">
            <span className={`card-name ${isCurrentUserPlayerA ? 'is-you' : ''}`}>
              {playerALabel}
            </span>
            <span className="vs">vs</span>
            <span className={`card-name ${!isCurrentUserPlayerA ? 'is-you' : ''}`}>
              {playerBLabel}
            </span>
          </div>
        ) : (
          // Regular combat - show "Combat N - CardA vs CardB"
          <div className="combat-matchup">
            <span className="combat-number">Combat {combat.combat_number}</span>
            {matchup && (
              <>
                <span className="combat-separator">-</span>
                <span className={`card-name ${isCurrentUserPlayerA ? 'is-you' : ''}`}>
                  {matchup.leftName}
                </span>
                <span className="vs">vs</span>
                <span className={`card-name ${!isCurrentUserPlayerA ? 'is-you' : ''}`}>
                  {matchup.rightName}
                </span>
              </>
            )}
          </div>
        )}
      </div>
      <div className="combat-events">
        {visibleEvents.map((event, idx) => {
          const alignment = getCombatEventAlignment(event, playerAId)
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
  combats,
  events: _events,
  currentEventIndex = -1,
  currentPhase = 'idle',
  getEventMessage = (event) => event.message || event.type,
  animated = true,
  className = '',
  playerAId,
  playerBId: _playerBId,
  playerAName = 'Player A',
  playerBName = 'Player B',
  currentUserId,
}: BattleLogProps) => {
  const contentRef = useRef<HTMLDivElement>(null)
  const { getUrlById } = useImages()

  /**
   * Compute player labels - show "You" for current user
   */
  const isCurrentUserPlayerA =
    currentUserId !== undefined && currentUserId === playerAId
  const playerALabel = isCurrentUserPlayerA ? 'You' : playerAName
  const playerBLabel =
    !isCurrentUserPlayerA && currentUserId !== undefined ? 'You' : playerBName

  /**
   * Derived state: Is the current event still animating?
   */
  const isCurrentEventAnimating =
    animated &&
    currentEventIndex >= 0 &&
    currentPhase !== 'idle' &&
    currentPhase !== 'complete'

  /**
   * Map currentEventIndex to combat index and event offset within combat.
   * This allows us to determine which combat is current and how many events
   * within that combat should be visible.
   */
  const { currentCombatIndex, eventOffsets } = useMemo(() => {
    const offsets: number[] = []
    let cumulativeCount = 0

    for (const combat of combats) {
      offsets.push(cumulativeCount)
      cumulativeCount += combat.events.length
    }

    // Find which combat contains the current event
    let combatIdx = -1
    if (currentEventIndex >= 0) {
      for (let i = 0; i < combats.length; i++) {
        const combatStart = offsets[i]
        const combatEnd = combatStart + combats[i].events.length
        if (currentEventIndex < combatEnd) {
          combatIdx = i
          break
        }
      }
      // If past all events, default to last combat
      if (combatIdx === -1 && combats.length > 0) {
        combatIdx = combats.length - 1
      }
    }

    return { currentCombatIndex: combatIdx, eventOffsets: offsets }
  }, [combats, currentEventIndex])

  /**
   * Determine visible combats and event counts within each
   */
  const visibleCombats = useMemo(() => {
    if (!animated) {
      // Static display: show all combats with all events
      return combats.map((combat) => ({
        combat,
        visibleEventCount: combat.events.length,
      }))
    }

    if (currentEventIndex < 0) {
      return []
    }

    // Show combats up to and including the current one
    const result: { combat: Combat; visibleEventCount: number }[] = []

    for (let i = 0; i < combats.length; i++) {
      const combatStart = eventOffsets[i]
      const combatEnd = combatStart + combats[i].events.length

      if (currentEventIndex >= combatStart) {
        // This combat has at least some visible events
        const visibleCount = Math.min(
          combats[i].events.length,
          currentEventIndex - combatStart + 1
        )
        result.push({
          combat: combats[i],
          visibleEventCount: visibleCount,
        })
      }

      if (currentEventIndex < combatEnd) {
        // Current event is in this combat, stop here
        break
      }
    }

    return result
  }, [combats, eventOffsets, currentEventIndex, animated])

  /**
   * Auto-scroll to bottom when new events appear
   */
  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight
    }
  }, [visibleCombats.length, currentEventIndex])

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
          {visibleCombats.map(
            (
              item: { combat: Combat; visibleEventCount: number },
              idx: number
            ) => {
              const isCurrentCombat = idx === currentCombatIndex
              const isPlayed = idx < currentCombatIndex

              return (
                <BattleLogCombat
                  key={item.combat.combat_number}
                  combat={item.combat}
                  visibleEventCount={item.visibleEventCount}
                  isCurrentCombat={isCurrentCombat}
                  isAnimating={isCurrentCombat && isCurrentEventAnimating}
                  isPlayed={isPlayed}
                  getIconUrl={getIconUrl}
                  getEventMessage={getEventMessage}
                  playerAId={playerAId}
                  playerALabel={playerALabel}
                  playerBLabel={playerBLabel}
                  isCurrentUserPlayerA={isCurrentUserPlayerA}
                />
              )
            }
          )}
        </div>
      </RPGPanel>
    </RPGPanel>
  )
}
