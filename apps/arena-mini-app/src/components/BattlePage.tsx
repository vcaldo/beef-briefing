import { useState, useEffect, useCallback, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { apiClient } from '../api/client';
import { useAudio } from '../hooks/useAudio';
import { BattleCard } from './BattleCard';
import type { Match, BattleResult, BattleEvent, GameCard } from '../types';
import './BattlePage.css';

interface BattlePageProps {
  match: Match;
  userId: number;
  onBack: () => void;
}

// Event timing in ms based on event type
const EVENT_TIMING: Record<string, number> = {
  attack: 1200,
  damage: 800,
  death: 1000,
  advance: 600,
  victory: 2500,
};

/**
 * Determines if a card is the attacker in the current event.
 */
function isCardAttacker(
  event: BattleEvent | null | undefined,
  card: GameCard,
  teamOwnerId: number
): boolean {
  if (!event) return false;
  return (
    event.attacker_card_id === card.card_id &&
    event.attacker_team_owner_id === teamOwnerId
  );
}

/**
 * Determines if a card is the defender in the current event.
 */
function isCardDefender(
  event: BattleEvent | null | undefined,
  card: GameCard,
  teamOwnerId: number
): boolean {
  if (!event) return false;
  return (
    event.defender_card_id === card.card_id &&
    event.defender_team_owner_id === teamOwnerId
  );
}

/**
 * Gets a player's display name from the participants list.
 */
function getPlayerName(userId: number, participants: any[]): string {
  const participant = participants.find(p => p.user_id === userId);
  if (!participant) return 'Unknown';
  return participant.username || participant.first_name;
}

/**
 * Gets an emoji icon for a battle event type.
 */
function getEventIcon(type: string): string {
  switch (type) {
    case 'attack':
      return '⚔️';
    case 'death':
      return '💀';
    case 'victory':
      return '👑';
    case 'advance':
      return '➡️';
    default:
      return '';
  }
}

export function BattlePage({ match, userId, onBack }: BattlePageProps) {
  const [battle, setBattle] = useState<BattleResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Replay state
  const [currentRoundIndex, setCurrentRoundIndex] = useState(0);
  const [currentEventIndex, setCurrentEventIndex] = useState(-1);
  const [isPlaying, setIsPlaying] = useState(false);

  // Result badge state - show only when animation reaches the end
  const [showResultBadge, setShowResultBadge] = useState(false);

  // Audio
  const { play, muted, toggleMuted } = useAudio();
  const lastEventRef = useRef<BattleEvent | null>(null);

  // Fetch battle results
  const fetchBattle = useCallback(async () => {
    try {
      const result = await apiClient.getBattle(match.id);
      setBattle(result);
      if (!result.is_complete) {
        // Keep polling until complete
        setTimeout(fetchBattle, 2000);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load battle');
    } finally {
      setLoading(false);
    }
  }, [match.id]);

  useEffect(() => {
    fetchBattle();
  }, [fetchBattle]);

  // Auto-play events with dynamic timing and audio
  useEffect(() => {
    if (!isPlaying || !battle || battle.rounds.length === 0) return;

    const round = battle.rounds[currentRoundIndex];
    if (!round) return;

    const events = round.battle_log;
    if (currentEventIndex >= events.length - 1) {
      // Move to next round or stop
      if (currentRoundIndex < battle.rounds.length - 1) {
        setCurrentRoundIndex(prev => prev + 1);
        setCurrentEventIndex(-1);
      } else {
        setIsPlaying(false);
      }
      return;
    }

    // Get timing for current event type
    const nextEventIndex = currentEventIndex + 1;
    const nextEvent = events[nextEventIndex];
    const timing = nextEvent ? (EVENT_TIMING[nextEvent.type] || 800) : 800;

    const timer = setTimeout(() => {
      setCurrentEventIndex(prev => prev + 1);
    }, timing);

    return () => clearTimeout(timer);
  }, [isPlaying, battle, currentRoundIndex, currentEventIndex]);

  // Play sounds on event change
  useEffect(() => {
    if (!battle || currentEventIndex < 0) return;

    const round = battle.rounds[currentRoundIndex];
    if (!round) return;

    const event = round.battle_log[currentEventIndex];
    if (!event || event === lastEventRef.current) return;

    lastEventRef.current = event;

    // Play appropriate sound
    switch (event.type) {
      case 'attack':
        play('attack');
        // Play hit sound after a delay
        setTimeout(() => play('hit'), 200);
        break;
      case 'death':
        play('ko');
        break;
      case 'victory':
        // Only play victory sound if current user won
        if (isWinner) {
          play('victory');
        }
        break;
    }
  }, [battle, currentRoundIndex, currentEventIndex, play]);

  // Show result badge only when animation reaches the end
  useEffect(() => {
    if (!battle || !battle.is_complete) return;

    const isAtEnd =
      currentRoundIndex === battle.rounds.length - 1 &&
      currentEventIndex === battle.rounds[battle.rounds.length - 1]?.battle_log.length - 1;

    if (isAtEnd && !showResultBadge) {
      setShowResultBadge(true);
    }
  }, [battle, currentRoundIndex, currentEventIndex, showResultBadge]);

  // Play/pause toggle
  const handlePlayPause = () => {
    if (!isPlaying) {
      // Start from beginning if at end
      if (battle && currentRoundIndex >= battle.rounds.length - 1) {
        const lastRound = battle.rounds[battle.rounds.length - 1];
        if (currentEventIndex >= lastRound.battle_log.length - 1) {
          setCurrentRoundIndex(0);
          setCurrentEventIndex(-1);
        }
      }
    }
    setIsPlaying(prev => !prev);
  };

  // Skip to end
  const handleSkipToEnd = () => {
    if (!battle || battle.rounds.length === 0) return;

    // Stop playback first
    setIsPlaying(false);

    // Use setTimeout to ensure state updates are processed
    // This avoids race conditions with the auto-play useEffect
    setTimeout(() => {
      const lastRoundIndex = battle.rounds.length - 1;
      const lastRound = battle.rounds[lastRoundIndex];
      const lastEventIndex = lastRound.battle_log.length - 1;

      setCurrentRoundIndex(lastRoundIndex);
      setCurrentEventIndex(lastEventIndex);
    }, 50); // Small delay to let isPlaying update propagate
  };

  if (loading) {
    return (
      <div className="battle-page">
        <div className="battle-loading">
          <div className="spinner" />
          <p>Loading battle...</p>
        </div>
      </div>
    );
  }

  if (error || !battle) {
    return (
      <div className="battle-page">
        <div className="battle-error">
          <p>{error || 'Failed to load battle'}</p>
          <button className="btn btn-secondary" onClick={onBack}>Back</button>
        </div>
      </div>
    );
  }

  if (!battle.is_complete) {
    return (
      <div className="battle-page">
        <div className="battle-waiting">
          <div className="spinner" />
          <h2>Battle in Progress</h2>
          <p>Waiting for all players to submit teams...</p>
        </div>
      </div>
    );
  }

  if (battle.rounds.length === 0) {
    return (
      <div className="battle-page">
        <div className="battle-error">
          <p>No battle rounds found</p>
          <button className="btn btn-secondary" onClick={onBack}>Back</button>
        </div>
      </div>
    );
  }

  const currentRound = battle.rounds[currentRoundIndex];
  const isUserPlayerA = currentRound.player_a_id === userId;
  const isWinner = battle.winner_id === userId;
  const currentEvent = currentEventIndex >= 0 ? currentRound.battle_log[currentEventIndex] : null;

  return (
    <div className="battle-page">
      {/* Header */}
      <header className="battle-header">
        <button className="btn btn-secondary back-btn" onClick={onBack}>
          Back
        </button>
        <div className="battle-title">
          Round {currentRoundIndex + 1}/{battle.rounds.length}
        </div>
        <div className="battle-status">
          {battle.is_complete && showResultBadge && (
            <span className={`result-badge ${isWinner ? 'win' : 'loss'}`}>
              {isWinner ? 'Victory!' : 'Defeat'}
            </span>
          )}
        </div>
      </header>

      {/* Battle Arena */}
      <div className="battle-arena">
        {/* Player A Team (Left) */}
        <div className="arena-team team-a">
          <div className="team-label">
            {getPlayerName(currentRound.player_a_id, match.participants)}
            {isUserPlayerA && ' (You)'}
          </div>
          <div className="team-cards">
            {currentRound.player_a_team.map((card, i) => (
              <BattleCard
                key={card.card_id}
                card={card}
                position={i}
                isActive={i === 0}
                currentEvent={currentEvent}
                isAttacker={isCardAttacker(currentEvent, card, currentRound.player_a_id)}
                isDefender={isCardDefender(currentEvent, card, currentRound.player_a_id)}
                side="left"
                teamOwnerId={currentRound.player_a_id}
              />
            ))}
          </div>
        </div>

        {/* VS / Event Display */}
        <div className="arena-vs">
          <AnimatePresence mode="wait">
            {currentEvent && (
              <motion.div
                key={`${currentRoundIndex}-${currentEventIndex}`}
                initial={{ opacity: 0, scale: 0.5, y: 10 }}
                animate={{ opacity: 1, scale: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.8 }}
                transition={{ duration: 0.2 }}
                className={`event-display event-${currentEvent.type}`}
              >
                {currentEvent.type === 'attack' && currentEvent.damage && (
                  <div className="damage-display">
                    <span className="damage-value">-{currentEvent.damage}</span>
                  </div>
                )}
                {currentEvent.type === 'death' && (
                  <div className="death-display">
                    <span className="ko-text">KO</span>
                  </div>
                )}
                {currentEvent.type === 'victory' && (
                  <div className="victory-display">
                    <span className={`victory-text ${isWinner ? 'win' : 'loss'}`}>
                      {isWinner ? 'Victory!' : 'Defeat!'}
                    </span>
                  </div>
                )}
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* Player B Team (Right) */}
        <div className="arena-team team-b">
          <div className="team-label">
            {getPlayerName(currentRound.player_b_id, match.participants)}
            {!isUserPlayerA && ' (You)'}
          </div>
          <div className="team-cards">
            {currentRound.player_b_team.map((card, i) => (
              <BattleCard
                key={card.card_id}
                card={card}
                position={i}
                isActive={i === 0}
                currentEvent={currentEvent}
                isAttacker={isCardAttacker(currentEvent, card, currentRound.player_b_id)}
                isDefender={isCardDefender(currentEvent, card, currentRound.player_b_id)}
                side="right"
                teamOwnerId={currentRound.player_b_id}
              />
            ))}
          </div>
        </div>
      </div>

      {/* Battle Log */}
      <div className="battle-log">
        {currentRound.battle_log.slice(0, currentEventIndex + 1).map((event, i) => (
          <div key={i} className={`log-entry log-entry-${event.type}`}>
            <span className="log-icon">{getEventIcon(event.type)}</span>
            <span className="log-message">{event.message}</span>
          </div>
        ))}
      </div>

      {/* Controls */}
      <div className="battle-controls">
        <button
          className="btn btn-icon mute-btn"
          onClick={toggleMuted}
          title={muted ? 'Unmute' : 'Mute'}
        >
          {muted ? (
            <span className="icon-muted">&#128264;</span>
          ) : (
            <span className="icon-sound">&#128266;</span>
          )}
        </button>
        <button
          className="btn btn-primary play-btn"
          onClick={handlePlayPause}
        >
          {isPlaying ? 'Pause' : 'Play'}
        </button>
        <button
          className="btn btn-secondary skip-btn"
          onClick={handleSkipToEnd}
        >
          Skip to End
        </button>
      </div>
    </div>
  );
}
