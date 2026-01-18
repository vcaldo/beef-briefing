/**
 * Battle Log Component
 *
 * Displays a scrollable log of battle events with animation synchronization.
 * Features:
 * - Auto-scroll to latest event
 * - Slide-in animation for new entries
 * - Progress indicator for currently animating events
 * - Visual distinction between current, playing, and animated events
 */

import { useRef, useEffect } from 'react'
import type { BattleEvent, EventAnimationPhase } from '../../types'

export interface BattleLogProps {
  /** List of all battle events */
  events: BattleEvent[]
  /** Current event index being displayed (0-based) */
  currentEventIndex: number
  /** Current animation phase (idle, highlight, attack, damage, death, complete) */
  currentPhase: EventAnimationPhase
  /** Function to format event message */
  getEventMessage: (event: BattleEvent) => string
}

export const BattleLog = ({
  events,
  currentEventIndex,
  currentPhase,
  getEventMessage,
}: BattleLogProps) => {
  const eventLogRef = useRef<HTMLDivElement>(null)

  /**
   * Derived state: Is the current event still animating?
   * Events are animating during highlight, attack, damage, and death phases.
   * Once phase reaches 'complete' or 'idle', the event is no longer animating.
   */
  const isCurrentEventAnimating =
    currentEventIndex >= 0 &&
    currentPhase !== 'idle' &&
    currentPhase !== 'complete'

  /**
   * Auto-scroll to bottom when new events appear or current event changes.
   * Smooth scrolling enhances the playback experience.
   */
  useEffect(() => {
    if (eventLogRef.current) {
      eventLogRef.current.scrollTop = eventLogRef.current.scrollHeight
    }
  }, [currentEventIndex])

  return (
    <div className="event-log" ref={eventLogRef}>
      <div className="event-log-title">Battle Log</div>
      {events.slice(0, currentEventIndex + 1).map((event, index) => {
        // Determine event item classes
        const isCurrent = index === currentEventIndex
        const isPlayed = index < currentEventIndex
        const isAnimating = isCurrent && isCurrentEventAnimating
        const shouldSlideIn = isCurrent && !isPlayed

        const classes = [
          'event-log-item',
          event.type,
          isCurrent ? 'current' : '',
          isPlayed ? 'played' : '',
          isAnimating ? 'animating' : '',
          shouldSlideIn ? 'slide-in' : '',
        ]
          .filter(Boolean)
          .join(' ')

        return (
          <div key={index} className={classes}>
            <span className="event-round">R{event.round}</span>
            <span className="event-message">{getEventMessage(event)}</span>
            {isAnimating && <div className="event-progress-dot" />}
          </div>
        )
      })}
    </div>
  )
}
