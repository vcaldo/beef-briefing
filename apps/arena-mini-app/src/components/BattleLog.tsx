import { useEffect, useRef, useMemo } from 'react';
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
  playerAId?: number;
  playerBId?: number;
  playerATeam?: any[];
  playerBTeam?: any[];
  currentEventIndex?: number;
  isLive?: boolean;
  autoScroll?: boolean;
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

// Determine text alignment based on which team initiated the event
function getEventTeamAlignment(
  event: BattleEvent,
  playerAId?: number,
  playerBId?: number,
  playerATeam?: any[],
  playerBTeam?: any[],
): 'left' | 'right' | 'center' {
  // Require team info to be available
  if (!playerAId || !playerBId || !playerATeam || !playerBTeam) {
    return 'left';
  }

  switch (event.type) {
    case 'attack':
      // Align based on attacker
      if (event.attacker_team_owner_id === playerAId) return 'left';
      if (event.attacker_team_owner_id === playerBId) return 'right';
      return 'center';

    case 'death':
      // Death message format: "X defeats Y" where Y is the defender
      // Align based on the killer (opposite of defender's team)
      if (event.defender_team_owner_id === playerAId) return 'right'; // B killed A
      if (event.defender_team_owner_id === playerBId) return 'left';  // A killed B
      return 'center';

    case 'summary':
      // Find which team the killer belongs to
      if (event.killer_card_id) {
        const isPlayerA = playerATeam.some((card: any) => card.card_id === event.killer_card_id);
        const isPlayerB = playerBTeam.some((card: any) => card.card_id === event.killer_card_id);
        if (isPlayerA) return 'left';
        if (isPlayerB) return 'right';
      }
      return 'center';

    case 'victory':
    case 'advance':
    default:
      // Neutral events stay centered
      return 'center';
  }
}

export function BattleLog({
  events,
  playerAId,
  playerBId,
  playerATeam,
  playerBTeam,
  currentEventIndex = undefined,
  isLive = false,
  autoScroll = isLive,
}: BattleLogProps) {
  const logEndRef = useRef<HTMLDivElement>(null);

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

    const alignment = getEventTeamAlignment(event, playerAId, playerBId, playerATeam, playerBTeam);
    const alignmentClass = `align-${alignment}`;

    return (
      <div key={index} className={`log-entry log-entry-${event.type} ${healthBarClass} ${alignmentClass}`}>
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

  // Render for history (show only summaries, no expansion)
  return (
    <div className="battle-log battle-log-history">
      {displayEvents
        .filter((event) => event.is_summary)
        .map((event, index) => {
          const alignment = getEventTeamAlignment(event, playerAId, playerBId, playerATeam, playerBTeam);
          const alignmentClass = `align-${alignment}`;

          return (
            <div key={index} className={`log-entry log-entry-summary ${alignmentClass}`}>
              <span className="log-message">{event.message}</span>
            </div>
          );
        })}
    </div>
  );
}

export default BattleLog;
