import { useState, useEffect } from 'react'
import type { Card as CardType, ShopCard, EnhancedShopCard, EnhancedTeamCard } from '../../types'

interface CardImageProps {
  /** Image URL */
  src?: string
  /** Alt text */
  alt: string
  /** Card name (for fallback display) */
  name: string
  /** Whether to show loading skeleton */
  showSkeleton?: boolean
  /** Size variant for fallback text */
  size?: 'sm' | 'md' | 'lg'
}

/**
 * Card image with loading and error states.
 * - Shows skeleton animation while loading
 * - Shows initials fallback on error or missing URL
 * - Resets loading state when src changes
 */
function CardImage({ src, alt, name, showSkeleton = true, size = 'lg' }: CardImageProps) {
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)

  // Reset loading state when src changes
  useEffect(() => {
    if (src) {
      setIsLoading(true)
      setHasError(false)
    }
  }, [src])

  // Generate initials for fallback
  const initials = name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()

  // Size-based text classes
  const sizeClasses = {
    sm: 'text-sm',
    md: 'text-lg',
    lg: 'text-2xl',
  }

  if (!src || hasError) {
    return (
      <div className={`w-full h-full flex items-center justify-center bg-[var(--arena-purple)] text-white font-display font-bold ${sizeClasses[size]}`}>
        {initials || '?'}
      </div>
    )
  }

  return (
    <>
      {showSkeleton && isLoading && <div className="card-image-skeleton absolute inset-0" />}
      <img
        src={src}
        alt={alt}
        className="w-full h-full object-cover"
        onLoad={() => setIsLoading(false)}
        onError={() => {
          setIsLoading(false)
          setHasError(true)
        }}
        style={{ display: isLoading ? 'none' : 'block' }}
      />
    </>
  )
}

// ============================================================================
// Shop Card (400x600 regular card for shop display)
// ============================================================================

interface ShopCardDisplayProps {
  /** Card data */
  card: ShopCard | EnhancedShopCard
  /** Click handler */
  onClick?: () => void
  /** Whether card is selected */
  isSelected?: boolean
}

/**
 * Shop card display (regular 400x600 format).
 * Shows card image with name and price overlay.
 */
export function ShopCardDisplay({ card, onClick, isSelected }: ShopCardDisplayProps) {
  const isEnhanced = 'can_buy' in card
  const canBuy = isEnhanced ? (card as EnhancedShopCard).can_buy : !card.is_purchased

  const affordabilityClass = card.is_purchased
    ? 'purchased'
    : canBuy
      ? 'affordable'
      : 'not-affordable'

  const ariaLabel = card.is_purchased
    ? `${card.name}'s card, already purchased`
    : canBuy
      ? `Buy ${card.name}'s card for 3 coins`
      : `${card.name}'s card, not affordable`

  return (
    <div
      className={`shop-card ${affordabilityClass} ${isSelected ? 'ring-2 ring-[var(--arena-orange)]' : ''}`}
      onClick={onClick}
      role="button"
      tabIndex={onClick ? 0 : -1}
      onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && onClick?.()}
      aria-label={ariaLabel}
      aria-disabled={!canBuy || card.is_purchased}
    >
      <div className="shop-card-image aspect-[2/3]">
        <CardImage
          src={card.card_image_url || card.photo_url}
          alt={`${card.name}'s card`}
          name={card.name}
        />
      </div>
      <div className="shop-card-overlay">
        <span className="shop-card-name">{card.name}</span>
        <span className="shop-card-price">
          <span>🪙</span>
          <span>3</span>
        </span>
      </div>
      {card.is_purchased && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/50">
          <span className="text-white font-display font-bold">SOLD</span>
        </div>
      )}
    </div>
  )
}

// ============================================================================
// Compact Card (300x450 with stat overlays for team/battle display)
// ============================================================================

interface CompactCardProps {
  /** Card data */
  card: CardType | EnhancedTeamCard
  /** Position in team (1-3) */
  position?: number
  /** Whether card is being attacked */
  isDefending?: boolean
  /** Whether card is attacking */
  isAttacking?: boolean
  /** Whether card is dead */
  isDead?: boolean
  /** Show HP bar */
  showHpBar?: boolean
  /** Click handler */
  onClick?: () => void
  /** Whether card is selected */
  isSelected?: boolean
}

/**
 * Compact card display (300x450 format) with stat overlays.
 * Used for team display and battle arena.
 */
export function CompactCard({
  card,
  position,
  isDefending = false,
  isAttacking = false,
  isDead = false,
  showHpBar = true,
  onClick,
  isSelected,
}: CompactCardProps) {
  // Calculate HP percentage for bar
  const hpPercent = (card.hp / card.max_hp) * 100
  const hpClass = hpPercent > 66 ? 'hp-high' : hpPercent > 33 ? 'hp-medium' : 'hp-low'

  return (
    <div
      className={`compact-card ${isDead ? 'dead' : ''} ${isSelected ? 'selected' : ''}`}
      data-is-attacking={isAttacking}
      data-is-defending={isDefending}
      data-is-alive={!isDead}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && onClick?.()}
      aria-label={`${card.name}'s card, Attack ${card.atk}, HP ${card.hp} of ${card.max_hp}${isDead ? ', defeated' : ''}${isSelected ? ', selected' : ''}`}
    >
      <CardImage
        src={card.photo_url}
        alt={`${card.name}'s card`}
        name={card.name}
        size="md"
      />

      {/* Position badge */}
      {position && (
        <div className="absolute -top-2 left-1/2 -translate-x-1/2 w-5 h-5 rounded-full bg-[var(--arena-purple)] text-white text-xs font-bold flex items-center justify-center z-10">
          {position}
        </div>
      )}

      {/* Stat overlays - ATK bottom-left, HP bottom-right */}
      <div className="compact-card-stat atk absolute bottom-8 left-1 text-sm">
        ⚔️ {card.atk}
        {card.atk_upgrades > 0 && (
          <span className="text-[var(--arena-gold)] text-xs">+{card.atk_upgrades * 3}</span>
        )}
      </div>
      <div className="compact-card-stat hp absolute bottom-8 right-1 text-sm">
        ❤️ {card.hp}/{card.max_hp}
        {card.hp_upgrades > 0 && (
          <span className="text-[var(--arena-gold)] text-xs">+{card.hp_upgrades * 3}</span>
        )}
      </div>

      {/* HP Bar */}
      {showHpBar && (
        <div className="hp-bar">
          <div
            className={`hp-bar-fill ${hpClass}`}
            style={{ width: `${hpPercent}%` }}
          />
        </div>
      )}

      {/* Death overlay */}
      {isDead && (
        <div className="absolute inset-0 flex items-center justify-center">
          <span className="text-4xl opacity-80">💀</span>
        </div>
      )}
    </div>
  )
}

// ============================================================================
// Team Card (Compact card with upgrade buttons)
// ============================================================================

interface TeamCardProps {
  /** Card data with upgrade info */
  card: EnhancedTeamCard
  /** Position in team (1-3) */
  position: number
  /** Upgrade ATK callback */
  onUpgradeAtk?: () => void
  /** Upgrade HP callback */
  onUpgradeHp?: () => void
  /** Click handler */
  onClick?: () => void
  /** Whether card is selected */
  isSelected?: boolean
  /** Whether in read-only mode (after submission) */
  readOnly?: boolean
}

/**
 * Team card with upgrade buttons.
 * Shows upgrade preview and handles upgrade interactions.
 */
export function TeamCard({
  card,
  position,
  onUpgradeAtk,
  onUpgradeHp,
  onClick,
  isSelected,
  readOnly = false,
}: TeamCardProps) {
  return (
    <div className="flex flex-col items-center gap-2">
      <div className={`team-slot filled ${isSelected ? 'selected' : ''}`}>
        <div className="team-slot-number">{position}</div>
        <CompactCard
          card={card}
          showHpBar={false}
          onClick={onClick}
          isSelected={isSelected}
        />
      </div>

      {/* Upgrade buttons */}
      {!readOnly && (
        <div className="upgrade-buttons" role="group" aria-label={`Upgrade options for ${card.name}`}>
          <button
            className="upgrade-btn atk"
            onClick={onUpgradeAtk}
            disabled={!card.can_upgrade_atk}
            title={card.upgrade_atk_disabled_reason}
            aria-label={`Upgrade attack from ${card.atk} to ${card.atk_if_upgraded}. Costs 1 coin${!card.can_upgrade_atk ? `. ${card.upgrade_atk_disabled_reason || 'Not available'}` : ''}`}
          >
            <span className="upgrade-label" aria-hidden="true">⚔️ +3</span>
            <span className="upgrade-preview" aria-hidden="true">
              <span className="upgrade-before">{card.atk}</span>
              <span className="upgrade-arrow">→</span>
              <span className="upgrade-after">{card.atk_if_upgraded}</span>
            </span>
          </button>
          <button
            className="upgrade-btn hp"
            onClick={onUpgradeHp}
            disabled={!card.can_upgrade_hp}
            title={card.upgrade_hp_disabled_reason}
            aria-label={`Upgrade HP from ${card.max_hp} to ${card.max_hp_if_upgraded}. Costs 1 coin${!card.can_upgrade_hp ? `. ${card.upgrade_hp_disabled_reason || 'Not available'}` : ''}`}
          >
            <span className="upgrade-label" aria-hidden="true">❤️ +3</span>
            <span className="upgrade-preview" aria-hidden="true">
              <span className="upgrade-before">{card.max_hp}</span>
              <span className="upgrade-arrow">→</span>
              <span className="upgrade-after">{card.max_hp_if_upgraded}</span>
            </span>
          </button>
        </div>
      )}
    </div>
  )
}

// ============================================================================
// Empty Team Slot
// ============================================================================

interface EmptySlotProps {
  /** Slot position (1-3) */
  position: number
  /** Click handler */
  onClick?: () => void
}

/**
 * Empty team slot placeholder.
 */
export function EmptySlot({ position, onClick }: EmptySlotProps) {
  return (
    <div
      className="team-slot"
      onClick={onClick}
      onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && onClick?.()}
      role="button"
      tabIndex={onClick ? 0 : -1}
      aria-label={`Empty team slot ${position}`}
    >
      <div className="team-slot-number" aria-hidden="true">{position}</div>
      <span className="team-slot-placeholder" aria-hidden="true">+</span>
    </div>
  )
}
