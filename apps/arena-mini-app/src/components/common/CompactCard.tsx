import React from 'react'
import type { PlaceholderPositions, CardAnimationState } from '../../types'

/** Current stat values to display on the card */
interface CardStats {
  atk: number
  hp: number
  maxHp: number
}

interface CompactCardProps {
  /** Presigned URL to the compact card image (300x450) */
  imageUrl: string
  /** Placeholder positions metadata from card-renderer API */
  positions?: PlaceholderPositions | null
  /** Current card stats to overlay on the image */
  currentStats: CardStats
  /** Optional card name for alt text */
  cardName?: string
  /** Optional card ID for data attributes */
  cardId?: number
  /** Additional CSS classes */
  className?: string
  /**
   * Animation state for battle replay animations.
   * Maps to CSS class: `compact-card-anim-${animationState}`
   * @see CardAnimationState
   */
  animationState?: CardAnimationState
  /**
   * Damage number to display as an overlay during damage phase.
   * Only rendered when provided and > 0.
   */
  damageNumber?: number
  /**
   * Whether the card is dead. If true, applies `.card-dead` class.
   * Defaults to checking if currentStats.hp <= 0.
   */
  isDead?: boolean
  /** Click handler */
  onClick?: () => void
}

/**
 * CompactCard - Displays a 300x450 compact card image with stat overlays
 *
 * This component renders a compact card image from the card-renderer API
 * and overlays dynamic stat values (ATK, HP) using position metadata
 * provided by the API.
 *
 * The positions are provided in logical pixels (300x450) which match the
 * card dimensions. If displaying at a different size, the component scales
 * the positions accordingly.
 *
 * @example
 * // Basic usage with stats
 * <CompactCard
 *   imageUrl="https://example.com/card.png"
 *   positions={placeholderPositions}
 *   currentStats={{ atk: 15, hp: 45, maxHp: 50 }}
 * />
 *
 * @example
 * // Battle card with animation state
 * <CompactCard
 *   imageUrl={card.image_url}
 *   positions={card.placeholder_positions}
 *   currentStats={{ atk: card.atk, hp: card.hp, maxHp: card.max_hp }}
 *   animationState="attacking"
 *   cardId={card.card_id}
 *   cardName={card.name}
 * />
 */
export function CompactCard({
  imageUrl,
  positions,
  currentStats,
  cardName = 'Card',
  cardId,
  className = '',
  animationState,
  damageNumber,
  isDead,
  onClick,
}: CompactCardProps): React.JSX.Element {
  // Extract position data with fallbacks
  const combatStats = positions?.placeholders?.combat_stats

  // Determine if card is dead (explicit prop or derived from HP)
  const isCardDead = isDead ?? currentStats.hp <= 0

  // Build class names for card state
  const cardClasses = [
    'compact-card',
    // Animation state class (e.g., compact-card-anim-attacking, compact-card-anim-taking_damage)
    animationState && `compact-card-anim-${animationState}`,
    // Dead card class forces HP bar to 0% immediately
    isCardDead && 'card-dead',
    onClick && 'cursor-pointer',
    className,
  ]
    .filter(Boolean)
    .join(' ')

  // Handle keyboard interaction for accessible cards
  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (onClick && (event.key === 'Enter' || event.key === ' ')) {
      event.preventDefault()
      onClick()
    }
  }

  return (
    <div
      className={cardClasses}
      onClick={onClick}
      onKeyDown={onClick ? handleKeyDown : undefined}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      data-card-id={cardId}
    >
      {/* Base card image */}
      <img
        src={imageUrl}
        alt={`${cardName} card`}
        className="compact-card-image"
        loading="lazy"
      />

      {/* Stat overlays - only render if positions are provided */}
      {combatStats && (
        <>
          {/* ATK overlay */}
          <div
            className="compact-card-stat"
            style={{
              left: `${combatStats.atk.x}px`,
              top: `${combatStats.atk.y}px`,
              width: `${combatStats.atk.width}px`,
              fontSize: `${combatStats.atk.font_size}px`,
              fontWeight: combatStats.atk.font_weight || 'bold',
              color: combatStats.atk.color,
              textAlign: (combatStats.atk.text_align as React.CSSProperties['textAlign']) || 'center',
              transform: combatStats.atk.anchor === 'center' ? 'translateX(-50%)' : undefined,
              textShadow: combatStats.atk.text_shadow || '0 1px 3px rgba(0, 0, 0, 0.8)',
            }}
            aria-label={`Attack: ${currentStats.atk}`}
          >
            {currentStats.atk}
          </div>

          {/* HP overlay */}
          <div
            className="compact-card-stat"
            style={{
              left: `${combatStats.hp.x}px`,
              top: `${combatStats.hp.y}px`,
              width: `${combatStats.hp.width}px`,
              fontSize: `${combatStats.hp.font_size}px`,
              fontWeight: combatStats.hp.font_weight || 'bold',
              color: combatStats.hp.color,
              textAlign: (combatStats.hp.text_align as React.CSSProperties['textAlign']) || 'center',
              transform: combatStats.hp.anchor === 'center' ? 'translateX(-50%)' : undefined,
              textShadow: combatStats.hp.text_shadow || '0 1px 3px rgba(0, 0, 0, 0.8)',
            }}
            aria-label={`HP: ${currentStats.hp} / ${currentStats.maxHp}`}
          >
            {currentStats.hp}
          </div>
        </>
      )}

      {/* Damage number overlay - displayed during damage phase */}
      {damageNumber !== undefined && damageNumber > 0 && (
        <div
          className="damage-number"
          aria-label={`Damage: ${damageNumber}`}
        >
          -{damageNumber}
        </div>
      )}

      {/* Fallback stat display when positions not available */}
      {!combatStats && (
        <div className="compact-card-stats-fallback">
          <span className="stat-atk">ATK: {currentStats.atk}</span>
          <span className="stat-hp">HP: {currentStats.hp}/{currentStats.maxHp}</span>
        </div>
      )}
    </div>
  )
}
