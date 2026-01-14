import { useEffect, useRef } from 'react'
import { EventMessage } from './EventMessage'
import type { BattleEvent } from '../../types'

interface EventLogProps {
  /** List of battle events to display */
  events: BattleEvent[]
  /** Current user ID for highlighting their events */
  currentUserId?: number
}

/**
 * Scrollable event log showing battle replay events.
 * Auto-scrolls to bottom as new events appear.
 */
export function EventLog({ events, currentUserId }: EventLogProps) {
  const scrollRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom when new events appear
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [events.length])

  if (events.length === 0) {
    return (
      <div className="event-log">
        <div className="event-log-empty">
          <span className="text-[var(--tg-theme-hint-color)]">
            Battle events will appear here...
          </span>
        </div>
      </div>
    )
  }

  return (
    <div className="event-log" ref={scrollRef}>
      {events.map((event, index) => (
        <EventMessage
          key={`${event.type}-${event.round}-${index}`}
          event={event}
          isYourTeam={
            event.attacker_team_owner_id === currentUserId ||
            event.defender_team_owner_id === currentUserId
          }
        />
      ))}
    </div>
  )
}
