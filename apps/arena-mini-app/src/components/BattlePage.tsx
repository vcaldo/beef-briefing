import { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { apiClient } from '../api/client';
import type { Match, BattleResult, MatchRound, BattleEvent, GameCard } from '../types';
import './BattlePage.css';

interface BattlePageProps {
  match: Match;
  userId: number;
  onBack: () => void;
}

export function BattlePage({ match, userId, onBack }: BattlePageProps) {
  const [battle, setBattle] = useState<BattleResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Replay state
  const [currentRoundIndex, setCurrentRoundIndex] = useState(0);
  const [currentEventIndex, setCurrentEventIndex] = useState(-1);
  const [isPlaying, setIsPlaying] = useState(false);

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

  // Auto-play events
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

    const timer = setTimeout(() => {
      setCurrentEventIndex(prev => prev + 1);
    }, 800);

    return () => clearTimeout(timer);
  }, [isPlaying, battle, currentRoundIndex, currentEventIndex]);

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
    if (battle && battle.rounds.length > 0) {
      setCurrentRoundIndex(battle.rounds.length - 1);
      const lastRound = battle.rounds[battle.rounds.length - 1];
      setCurrentEventIndex(lastRound.battle_log.length - 1);
      setIsPlaying(false);
    }
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
          {battle.is_complete && (
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
            {isUserPlayerA ? 'You' : 'Opponent'}
          </div>
          <div className="team-cards">
            {currentRound.player_a_team.map((card, i) => (
              <BattleCard
                key={card.card_id}
                card={card}
                position={i}
                isActive={i === 0}
                currentEvent={currentEvent}
                isAttacker={currentEvent?.attacker_id === card.user_id}
                isDefender={currentEvent?.defender_id === card.user_id}
              />
            ))}
          </div>
        </div>

        {/* VS */}
        <div className="arena-vs">
          <AnimatePresence>
            {currentEvent && (
              <motion.div
                key={currentEventIndex}
                initial={{ opacity: 0, scale: 0.5 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0 }}
                className="event-display"
              >
                {currentEvent.type === 'attack' && (
                  <div className="damage-display">-{currentEvent.damage}</div>
                )}
                {currentEvent.type === 'death' && (
                  <div className="death-display">KO!</div>
                )}
                {currentEvent.type === 'victory' && (
                  <div className="victory-display">Victory!</div>
                )}
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* Player B Team (Right) */}
        <div className="arena-team team-b">
          <div className="team-label">
            {!isUserPlayerA ? 'You' : 'Opponent'}
          </div>
          <div className="team-cards">
            {currentRound.player_b_team.map((card, i) => (
              <BattleCard
                key={card.card_id}
                card={card}
                position={i}
                isActive={i === 0}
                currentEvent={currentEvent}
                isAttacker={currentEvent?.attacker_id === card.user_id}
                isDefender={currentEvent?.defender_id === card.user_id}
              />
            ))}
          </div>
        </div>
      </div>

      {/* Battle Log */}
      <div className="battle-log">
        {currentRound.battle_log.slice(0, currentEventIndex + 1).map((event, i) => (
          <div key={i} className={`log-entry ${event.type}`}>
            {event.message}
          </div>
        ))}
      </div>

      {/* Controls */}
      <div className="battle-controls">
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

// Battle Card Component
interface BattleCardProps {
  card: GameCard;
  position: number;
  isActive: boolean;
  currentEvent: BattleEvent | null;
  isAttacker: boolean;
  isDefender: boolean;
}

function BattleCard({ card, position, isActive, currentEvent, isAttacker, isDefender }: BattleCardProps) {
  const hp = currentEvent?.defender_id === card.user_id && currentEvent?.hp_after !== undefined
    ? currentEvent.hp_after
    : card.hp;
  const isDead = hp <= 0;

  return (
    <motion.div
      className={`battle-card ${isActive ? 'active' : ''} ${isDead ? 'dead' : ''}`}
      animate={{
        x: isAttacker ? 20 : 0,
        scale: isDefender ? 0.95 : 1,
        opacity: isDead ? 0.3 : 1,
      }}
      transition={{ duration: 0.2 }}
    >
      <div className="battle-card-photo">
        {card.photo_url ? (
          <img src={card.photo_url} alt={card.name} />
        ) : (
          <div className="no-photo">{card.name[0]}</div>
        )}
      </div>
      <div className="battle-card-name">{card.name}</div>
      <div className="battle-card-stats">
        <span className="stat-atk">ATK {card.atk}</span>
        <span className="stat-hp">HP {Math.max(0, hp)}/{card.max_hp}</span>
      </div>
      <div className="hp-bar">
        <motion.div
          className="hp-fill"
          animate={{ width: `${Math.max(0, (hp / card.max_hp) * 100)}%` }}
          transition={{ duration: 0.3 }}
        />
      </div>
      <AnimatePresence>
        {isDefender && currentEvent?.damage && (
          <motion.div
            className="damage-number"
            initial={{ opacity: 1, y: 0 }}
            animate={{ opacity: 0, y: -30 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.8 }}
          >
            -{currentEvent.damage}
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
