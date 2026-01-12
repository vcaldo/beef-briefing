import React, { useEffect, useRef, useState, useMemo } from 'react';
import './BattleLog.css';

interface BattleEvent {
  type: string;
  round: number;
  message: string;
  attacker_card_id?: number;
  defender_card_id?: number;
  attacker_team_owner_id?: number;
  defender_team_owner_id?: number;
  damage?: number;
  hp_before?: number;
  hp_after?: number;
  is_summary?: boolean;
  killer_card_id?: number;
  total_damage_dealt?: number;
  total_damage_taken?: number;
  rounds_in_duel?: number;
  kill_streak?: number;
}

interface BattleLogProps {
  events: BattleEvent[];
  currentEventIndex?: number;
  isLive?: boolean;
  autoScroll?: boolean;
}

function getEventIcon(type: string): string {
  switch (type) {
    case 'attack':
      return '🗡️';
    case 'death':
      return '💀';
    case 'victory':
      return '🏆';
    case 'advance':
      return '➡️';
    case 'summary':
      return '⚔️';
    default:
      return '';
  }
}

// Parse health bar from message and extract HP percentage
function getHealthBarInfo(message: string): { bar: string; percentage: number } | null {
  const match = message.match(/\[([█░]{10})\]/);
  if (!match) return null;

  const bar = match[1];
  const filledBlocks = (bar.match(/█/g) || []).length;
  const percentage = (filledBlocks / 10) * 100;

  return { bar, percentage };
}

// Group events by summary (for history mode, group attack/death events between summaries)
function groupEventsBySummary(events: BattleEvent[]): (BattleEvent | BattleEvent[])[] {
  const grouped: (BattleEvent | BattleEvent[])[] = [];
  let currentGroup: BattleEvent[] = [];

  for (const event of events) {
    if (event.is_summary) {
      if (currentGroup.length > 0) {
        grouped.push(...currentGroup);
        currentGroup = [];
      }
      grouped.push(event);
    } else {
      currentGroup.push(event);
    }
  }

  if (currentGroup.length > 0) {
    grouped.push(...currentGroup);
  }

  return grouped;
}

export function BattleLog({
  events,
  currentEventIndex = undefined,
  isLive = false,
  autoScroll = isLive,
}: BattleLogProps) {
  const logEndRef = useRef<HTMLDivElement>(null);
  const [expandedSummaries, setExpandedSummaries] = useState<Set<number>>(new Set());

  // Auto-scroll when new events appear
  useEffect(() => {
    if (isLive && autoScroll && logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [currentEventIndex, isLive, autoScroll]);

  // Determine which events to display
  const displayEvents = useMemo(() => {
    if (isLive && currentEventIndex !== undefined) {
      return events.slice(0, currentEventIndex + 1);
    }
    return events;
  }, [events, currentEventIndex, isLive]);

  // Toggle summary expansion
  const toggleSummaryExpansion = (eventIndex: number) => {
    const newExpanded = new Set(expandedSummaries);
    if (newExpanded.has(eventIndex)) {
      newExpanded.delete(eventIndex);
    } else {
      newExpanded.add(eventIndex);
    }
    setExpandedSummaries(newExpanded);
  };

  // Render a single event
  const renderEvent = (event: BattleEvent, index: number) => {
    const healthBarInfo = getHealthBarInfo(event.message);
    const healthBarClass = healthBarInfo
      ? healthBarInfo.percentage > 60
        ? 'health-bar-high'
        : healthBarInfo.percentage > 30
          ? 'health-bar-medium'
          : 'health-bar-low'
      : '';

    return (
      <div key={index} className={`log-entry log-entry-${event.type} ${healthBarClass}`}>
        <span className="log-icon">{getEventIcon(event.type)}</span>
        <span className="log-message">{event.message}</span>
      </div>
    );
  };

  // Render for live battle (show all events in sequence)
  if (isLive) {
    return (
      <div className="battle-log battle-log-live">
        {displayEvents.map((event, index) => renderEvent(event, index))}
        <div ref={logEndRef} />
      </div>
    );
  }

  // Render for history (with expandable summaries, collapsed by default)
  // Find all summary events and their surrounding attack/death events
  const summaryIndices = displayEvents
    .map((event, index) => (event.is_summary ? index : -1))
    .filter((index) => index !== -1);

  return (
    <div className="battle-log battle-log-history">
      {displayEvents.map((event, index) => {
        if (event.is_summary) {
          // Find the next summary or end of events
          const nextSummaryIndex = summaryIndices.find((si) => si > index) ?? displayEvents.length;

          // Get events between this summary and the next
          const endIndex = nextSummaryIndex - 1;
          const groupedEvents = displayEvents.slice(index + 1, endIndex + 1);

          const isExpanded = expandedSummaries.has(index);

          return (
            <div key={index} className="summary-group">
              <button
                className={`log-entry log-entry-summary ${isExpanded ? 'expanded' : 'collapsed'}`}
                onClick={() => toggleSummaryExpansion(index)}
              >
                <span className="log-icon">{getEventIcon(event.type)}</span>
                <span className="log-message">{event.message}</span>
                <span className="expand-icon">{isExpanded ? '▲' : '▼'}</span>
              </button>

              {/* Expandable details */}
              {isExpanded && groupedEvents.length > 0 && (
                <div className="summary-details">
                  {groupedEvents.map((detailEvent, detailIndex) =>
                    renderEvent(detailEvent, detailIndex),
                  )}
                </div>
              )}
            </div>
          );
        }

        // Skip non-summary events in history mode, they're rendered under summaries
        return null;
      })}
    </div>
  );
}

export default BattleLog;
