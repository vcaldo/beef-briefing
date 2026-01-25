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
}

/**
 * Single round row in the battle log
 */
function BattleLogRound({
  roundGroup,
  isCurrentRound,
  isAnimating,
  isPlayed,
  getIconUrl,
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

  return (
    <div className={classes}>
      <span className="round-indicator">R{roundGroup.round}</span>
      <div className="round-icons">
        {roundGroup.events.map((event, idx) => (
          <img
            key={idx}
            src={getIconUrl(event.type)}
            alt={event.type}
            className={`event-icon event-icon-${event.type}`}
            title={event.type}
          />
        ))}
      </div>
    </div>
  )
}

export const BattleLog = ({
  events,
  currentEventIndex = -1,
  currentPhase = 'idle',
  animated = true,
  className = '',
}: BattleLogProps) => {
  const contentRef = useRef<HTMLDivElement>(null)
  const { getUrlById } = useImages()

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
   */
  const visibleEvents = useMemo(() => {
    if (!animated || currentEventIndex < 0) {
      return events
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
   * Auto-scroll to bottom when new rounds appear
   */
  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight
    }
  }, [roundGroups.length])

  /**
   * Get icon URL for event type
   */
  const getIconUrl = (type: EventType): string => {
    return getUrlById(getIconForEventType(type))
  }

  return (
    <RPGPanel variant="outer" className={`battle-log-outer ${className}`}>
      <RPGPanel variant="inner" className="battle-log-inner">
        <div className="battle-log-title">Battle Log</div>
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
              />
            )
          })}
        </div>
      </RPGPanel>
    </RPGPanel>
  )
}
