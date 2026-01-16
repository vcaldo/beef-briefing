import React, { useMemo } from 'react'
import type { PlaceholderPositions } from '../../types'

/** Current stat values to display on the card */
interface CardStats {
  atk: number
  def: number
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
  /** Whether the card is alive (false = grayscale + reduced opacity) */
  isAlive?: boolean
  /** Whether to show the HP bar (default true) */
  showHpBar?: boolean
  /** Optional card name for alt text */
  cardName?: string
  /** Optional card ID for data attributes */
  cardId?: number
  /** Additional CSS classes */
  className?: string
  /** Whether this card is currently attacking (for battle animations) */
  isAttacking?: boolean
  /** Whether this card is currently defending (for battle animations) */
  isDefending?: boolean
  /** Click handler */
  onClick?: () => void
}

/**
 * Calculates HP bar color based on percentage
 * - Green: >66%
 * - Yellow: 33-66%
 * - Red: <33%
 */
function getHpBarColor(hp: number, maxHp: number): string {
  if (maxHp <= 0) return '#ef4444' // Red for invalid
  const percentage = (hp / maxHp) * 100
  if (percentage > 66) return '#22c55e' // Green
  if (percentage >= 33) return '#eab308' // Yellow
  return '#ef4444' // Red
}

/**
 * CompactCard - Displays a 300x450 compact card image with stat overlays
 *
 * This component renders a compact card image from the card-renderer API
 * and overlays dynamic stat values (ATK, DEF, HP) and an animated HP bar
 * using position metadata provided by the API.
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
 *   currentStats={{ atk: 15, def: 8, hp: 45, maxHp: 50 }}
 * />
 *
 * @example
 * // Battle card with alive state
 * <CompactCard
 *   imageUrl={card.image_url}
 *   positions={card.placeholder_positions}
 *   currentStats={{ atk: card.atk, def: card.def, hp: card.hp, maxHp: card.max_hp }}
 *   isAlive={card.hp > 0}
 *   isAttacking={card.is_attacking}
 *   isDefending={card.is_defending}
 *   cardId={card.card_id}
 *   cardName={card.name}
 * />
 */
export function CompactCard({
  imageUrl,
  positions,
  currentStats,
  isAlive = true,
  showHpBar = true,
  cardName = 'Card',
  cardId,
  className = '',
  isAttacking = false,
  isDefending = false,
  onClick,
}: CompactCardProps): React.JSX.Element {
  // Calculate HP bar width and color
  const hpPercentage = useMemo(() => {
    if (currentStats.maxHp <= 0) return 0
    return Math.max(0, Math.min(100, (currentStats.hp / currentStats.maxHp) * 100))
  }, [currentStats.hp, currentStats.maxHp])

  const hpBarColor = useMemo(
    () => getHpBarColor(currentStats.hp, currentStats.maxHp),
    [currentStats.hp, currentStats.maxHp]
  )

  // Extract position data with fallbacks
  const combatStats = positions?.placeholders?.combat_stats
  const hpBar = positions?.placeholders?.hp_bar

  // Build class names for card state
  const cardClasses = [
    'compact-card',
    !isAlive && 'compact-card-dead',
    isAttacking && 'compact-card-attacking',
    isDefending && 'compact-card-defending',
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
      data-is-alive={isAlive}
      data-is-attacking={isAttacking}
      data-is-defending={isDefending}
      style={{
        // Apply grayscale and reduced opacity when dead
        filter: !isAlive ? 'grayscale(100%)' : undefined,
        opacity: !isAlive ? 0.5 : 1,
        transition: 'filter 0.3s ease, opacity 0.3s ease',
      }}
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

          {/* DEF overlay */}
          <div
            className="compact-card-stat"
            style={{
              left: `${combatStats.def.x}px`,
              top: `${combatStats.def.y}px`,
              width: `${combatStats.def.width}px`,
              fontSize: `${combatStats.def.font_size}px`,
              fontWeight: combatStats.def.font_weight || 'bold',
              color: combatStats.def.color,
              textAlign: (combatStats.def.text_align as React.CSSProperties['textAlign']) || 'center',
              transform: combatStats.def.anchor === 'center' ? 'translateX(-50%)' : undefined,
              textShadow: combatStats.def.text_shadow || '0 1px 3px rgba(0, 0, 0, 0.8)',
            }}
            aria-label={`Defense: ${currentStats.def}`}
          >
            {currentStats.def}
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

      {/* HP bar - only render if positions are provided and showHpBar is true */}
      {showHpBar && hpBar && (
        <div
          className="compact-card-hp-bar"
          style={{
            left: `${hpBar.container.x}px`,
            top: `${hpBar.container.y}px`,
            width: `${hpBar.container.width}px`,
            height: `${hpBar.container.height}px`,
            borderRadius: `${hpBar.container.border_radius}px`,
            backgroundColor: hpBar.fill.background_color || 'rgba(0, 0, 0, 0.6)',
          }}
          role="progressbar"
          aria-valuenow={currentStats.hp}
          aria-valuemin={0}
          aria-valuemax={currentStats.maxHp}
          aria-label={`HP: ${currentStats.hp} of ${currentStats.maxHp}`}
        >
          <div
            className="compact-card-hp-bar-fill"
            style={{
              width: `${hpPercentage}%`,
              backgroundColor: hpBarColor,
              borderRadius: `${hpBar.fill.border_radius}px`,
              // CSS transition handles animation (defined in global.css)
            }}
          />
        </div>
      )}

      {/* Fallback stat display when positions not available */}
      {!combatStats && (
        <div className="compact-card-stats-fallback">
          <span className="stat-atk">ATK: {currentStats.atk}</span>
          <span className="stat-def">DEF: {currentStats.def}</span>
          <span className="stat-hp">HP: {currentStats.hp}/{currentStats.maxHp}</span>
        </div>
      )}

      {/* Fallback HP bar when positions not available but showHpBar is true */}
      {showHpBar && !hpBar && (
        <div
          className="compact-card-hp-bar-fallback"
          role="progressbar"
          aria-valuenow={currentStats.hp}
          aria-valuemin={0}
          aria-valuemax={currentStats.maxHp}
        >
          <div
            className="compact-card-hp-bar-fill-fallback"
            style={{
              width: `${hpPercentage}%`,
              backgroundColor: hpBarColor,
            }}
          />
        </div>
      )}
    </div>
  )
}
