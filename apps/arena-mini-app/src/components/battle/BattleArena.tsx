import { BattleCard } from './BattleCard'
import type { CardSnapshot, BattleEvent } from '../../types'

interface BattleArenaProps {
  /** Your team's cards */
  yourCards: CardSnapshot[]
  /** Opponent's team cards */
  opponentCards: CardSnapshot[]
  /** Current battle event (for highlighting attacker/defender) */
  currentEvent?: BattleEvent
}

/**
 * Battle arena displaying both teams facing each other.
 * Shows 3 cards per team with live HP and attack/defend states.
 */
export function BattleArena({
  yourCards,
  opponentCards,
  currentEvent,
}: BattleArenaProps) {
  // Sort cards by position
  const sortedYourCards = [...yourCards].sort((a, b) => a.position - b.position)
  const sortedOpponentCards = [...opponentCards].sort((a, b) => a.position - b.position)

  return (
    <div className="battle-arena">
      {/* Opponent team (top, rotated) */}
      <div className="battle-team opponent">
        {sortedOpponentCards.map((card) => (
          <BattleCard
            key={card.card_id}
            card={card}
            isAttacking={currentEvent?.attacker_card_id === card.card_id}
            isDefending={currentEvent?.defender_card_id === card.card_id}
          />
        ))}
        {/* Fill empty slots if less than 3 cards */}
        {Array.from({ length: Math.max(0, 3 - sortedOpponentCards.length) }).map((_, i) => (
          <div key={`empty-opp-${i}`} className="battle-card-empty" />
        ))}
      </div>

      {/* VS indicator */}
      <div className="battle-vs">
        <span className="battle-vs-text">VS</span>
      </div>

      {/* Your team (bottom) */}
      <div className="battle-team yours">
        {sortedYourCards.map((card) => (
          <BattleCard
            key={card.card_id}
            card={card}
            isAttacking={currentEvent?.attacker_card_id === card.card_id}
            isDefending={currentEvent?.defender_card_id === card.card_id}
          />
        ))}
        {/* Fill empty slots if less than 3 cards */}
        {Array.from({ length: Math.max(0, 3 - sortedYourCards.length) }).map((_, i) => (
          <div key={`empty-you-${i}`} className="battle-card-empty" />
        ))}
      </div>
    </div>
  )
}
