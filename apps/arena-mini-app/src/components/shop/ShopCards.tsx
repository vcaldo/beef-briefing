import { ShopCardDisplay } from '../common/Card'
import type { EnhancedShopCard } from '../../types'

interface ShopCardsProps {
  /** Available cards in the shop */
  cards: EnhancedShopCard[]
  /** Buy card callback */
  onBuy: (cardIndex: number) => void
  /** Card index currently being purchased */
  buyingCardIndex?: number
}

/**
 * Grid of shop cards available for purchase.
 * Displays regular 400x600 card images with price overlays.
 */
export function ShopCards({ cards, onBuy, buyingCardIndex }: ShopCardsProps) {
  if (cards.length === 0) {
    return (
      <div className="shop-cards-empty">
        <p className="text-[var(--tg-theme-hint-color)] text-center py-8">
          No cards available in the shop.
        </p>
      </div>
    )
  }

  return (
    <div className="shop-grid stagger-grid">
      {cards.map((card) => {
        const isLoading = buyingCardIndex === card.index
        const isDisabled = !card.can_buy || card.is_purchased || isLoading

        return (
          <div
            key={card.index}
            className={`shop-card-wrapper ${isLoading ? 'loading' : ''}`}
          >
            <ShopCardDisplay
              card={card}
              onClick={() => !isDisabled && onBuy(card.index)}
              isSelected={isLoading}
            />
            {/* Loading overlay */}
            {isLoading && (
              <div className="shop-card-loading">
                <div className="btn-spinner" />
              </div>
            )}
            {/* Disabled reason tooltip */}
            {!card.can_buy && !card.is_purchased && card.buy_disabled_reason && (
              <div className="shop-card-tooltip">
                {card.buy_disabled_reason}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
