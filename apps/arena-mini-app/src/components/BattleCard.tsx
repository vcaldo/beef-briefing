import { motion, AnimatePresence, Variants } from 'framer-motion';
import type { GameCard, BattleEvent } from '../types';
import './BattleCard.css';

export interface BattleCardProps {
  card: GameCard | null;
  position: number;
  isActive: boolean;
  currentEvent: BattleEvent | null;
  isAttacker: boolean;
  isDefender: boolean;
  side: 'left' | 'right';
  teamOwnerId: number;
}

// Animation variants
const cardVariants: Variants = {
  idle: {
    y: [0, -2, 0],
    transition: {
      y: { repeat: Infinity, duration: 2, ease: 'easeInOut' },
    },
  },
  attacking: {
    x: [0, 30, 0],
    transition: {
      duration: 0.35,
      times: [0, 0.5, 1],
      ease: 'easeOut',
    },
  },
  attackingRight: {
    x: [0, -30, 0],
    transition: {
      duration: 0.35,
      times: [0, 0.5, 1],
      ease: 'easeOut',
    },
  },
  hit: {
    x: [0, -6, 6, -4, 4, 0],
    transition: { duration: 0.3 },
  },
  dying: {
    scale: 0.85,
    rotate: -8,
    opacity: 0.4,
    filter: 'grayscale(100%) brightness(0.6)',
    transition: { duration: 0.5 },
  },
  entering: {
    x: [-80, 0],
    opacity: [0, 1],
    transition: { duration: 0.4, ease: 'backOut' },
  },
  enteringRight: {
    x: [80, 0],
    opacity: [0, 1],
    transition: { duration: 0.4, ease: 'backOut' },
  },
  victorious: {
    scale: [1, 1.12, 1.05, 1.08, 1],
    transition: { duration: 0.8, ease: 'easeOut' },
  },
};

export function BattleCard({
  card,
  isActive,
  currentEvent,
  isAttacker,
  isDefender,
  side,
  teamOwnerId,
}: BattleCardProps) {
  // Guard against null cards (defensive programming)
  if (!card) {
    return null;
  }

  // Calculate current HP based on event
  let hp = card.hp;
  if (
    currentEvent?.hp_after !== undefined &&
    currentEvent.defender_card_id === card.card_id &&
    currentEvent.defender_team_owner_id === teamOwnerId
  ) {
    hp = currentEvent.hp_after;
  }
  const isDead = hp <= 0;
  const hpPercent = Math.max(0, (hp / card.max_hp) * 100);

  // Determine animation state
  const getAnimationState = () => {
    if (isDead) return 'dying';
    if (isAttacker) return side === 'left' ? 'attacking' : 'attackingRight';
    if (isDefender) return 'hit';
    if (isActive) return 'idle';
    return undefined;
  };

  // Truncate name to fit
  const displayName = card.name.length > 10 ? card.name.slice(0, 9) + '...' : card.name;

  return (
    <motion.div
      className={`battle-card-v2 ${isActive ? 'active' : ''} ${isDead ? 'dead' : ''}`}
      variants={cardVariants}
      animate={getAnimationState()}
      initial={false}
    >
      {/* Glowing border for active card */}
      {isActive && !isDead && <div className="active-glow" />}

      {/* Avatar */}
      <div className="card-avatar">
        {card.photo_url ? (
          <img src={card.photo_url} alt={card.name} />
        ) : (
          <div className="avatar-placeholder">{card.name[0]?.toUpperCase()}</div>
        )}
      </div>

      {/* Name */}
      <div className="card-name-v2">{displayName}</div>

      {/* Stats Row */}
      <div className="card-stats-v2">
        <div className="stat-atk-v2">
          <span className="stat-icon">&#x2694;</span>
          <span className="stat-value">{card.atk}</span>
        </div>
      </div>

      {/* HP Bar */}
      <div className="hp-bar-v2">
        <motion.div
          className="hp-fill-v2"
          animate={{ width: `${hpPercent}%` }}
          transition={{ duration: 0.3, ease: 'easeOut' }}
          style={{
            background: hpPercent > 50
              ? 'linear-gradient(90deg, #22c55e, #51cf66)'
              : hpPercent > 25
              ? 'linear-gradient(90deg, #eab308, #facc15)'
              : 'linear-gradient(90deg, #dc2626, #ef4444)',
          }}
        />
      </div>

      {/* Damage Flash Overlay */}
      <AnimatePresence>
        {isDefender && currentEvent?.damage && (
          <motion.div
            className="damage-flash"
            initial={{ opacity: 0.6 }}
            animate={{ opacity: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
          />
        )}
      </AnimatePresence>

      {/* Floating Damage Number */}
      <AnimatePresence>
        {isDefender && currentEvent?.damage && (
          <motion.div
            className="damage-number-v2"
            initial={{ opacity: 1, y: 0, scale: 1.2 }}
            animate={{ opacity: 0, y: -35, scale: 0.8 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.7, ease: 'easeOut' }}
          >
            -{currentEvent.damage}
          </motion.div>
        )}
      </AnimatePresence>

      {/* KO Overlay */}
      {isDead && (
        <motion.div
          className="ko-overlay"
          initial={{ opacity: 0, scale: 0.5 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.3, ease: 'backOut' }}
        >
          KO
        </motion.div>
      )}
    </motion.div>
  );
}
