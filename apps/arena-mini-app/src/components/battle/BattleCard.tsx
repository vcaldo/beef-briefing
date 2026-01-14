import type { CardSnapshot } from '../../types'

interface BattleCardProps {
  /** Card snapshot with current battle state */
  card: CardSnapshot
  /** Whether this card is currently attacking */
  isAttacking?: boolean
  /** Whether this card is being attacked */
  isDefending?: boolean
}

/**
 * Card image with loading and error states for battle display.
 */
function CardImage({ name }: { name: string }) {
  // Generate initials for fallback (battle cards are smaller, so just use initials)
  const initials = name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()

  return (
    <div className="w-full h-full flex items-center justify-center bg-[var(--arena-purple)] text-white font-display font-bold text-sm">
      {initials || '?'}
    </div>
  )
}

/**
 * Battle card component showing card state during combat.
 * Displays HP bar, attack/defend animations, and death state.
 */
export function BattleCard({
  card,
  isAttacking = false,
  isDefending = false,
}: BattleCardProps) {
  // Calculate HP percentage for bar
  const hpPercent = Math.max(0, (card.hp / card.max_hp) * 100)
  const hpClass = hpPercent > 66 ? 'hp-high' : hpPercent > 33 ? 'hp-medium' : 'hp-low'

  return (
    <div
      className="battle-card"
      data-is-attacking={isAttacking}
      data-is-defending={isDefending}
      data-is-alive={card.is_alive}
    >
      {/* Card image */}
      <div className="battle-card-image">
        <CardImage name={card.name} />
      </div>

      {/* Card name overlay */}
      <div className="battle-card-name">
        {card.name.split(' ')[0]}
      </div>

      {/* Stats overlay */}
      <div className="battle-card-stats">
        <span className="battle-stat atk">⚔️{card.atk}</span>
        <span className="battle-stat hp">❤️{card.hp}</span>
      </div>

      {/* HP Bar */}
      <div className="hp-bar battle-hp">
        <div
          className={`hp-bar-fill ${hpClass}`}
          style={{ width: `${hpPercent}%` }}
        />
      </div>

      {/* Death overlay */}
      {!card.is_alive && (
        <div className="battle-card-death">
          <span className="death-icon">💀</span>
        </div>
      )}

      {/* Attack indicator */}
      {isAttacking && (
        <div className="battle-card-indicator attack">
          ⚔️
        </div>
      )}

      {/* Defend indicator */}
      {isDefending && (
        <div className="battle-card-indicator defend">
          🛡️
        </div>
      )}
    </div>
  )
}
