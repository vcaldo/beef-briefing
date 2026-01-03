import { CardImageWithUrl } from '../api/client'
import { CardImage } from './CardImage'

interface CardGalleryProps {
  cards: CardImageWithUrl[]
  onCardClick: (card: CardImageWithUrl) => void
}

/**
 * Grid gallery of card images.
 */
export function CardGallery({ cards, onCardClick }: CardGalleryProps) {
  if (cards.length === 0) {
    return (
      <div className="empty-state">
        <p>No cards available for this week</p>
      </div>
    )
  }

  return (
    <div className="card-gallery">
      {cards.map((card) => (
        <CardImage key={card.id} card={card} onClick={() => onCardClick(card)} />
      ))}
    </div>
  )
}
