import { useState } from 'react'
import { CardImageWithUrl } from '../api/client'

interface CardImageProps {
  card: CardImageWithUrl
  onClick: () => void
}

/**
 * Single card image with loading state.
 */
export function CardImage({ card, onClick }: CardImageProps) {
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)

  const displayName =
    [card.first_name, card.last_name].filter(Boolean).join(' ') || 'Unknown'

  return (
    <div className="card-item" onClick={onClick}>
      <div className="card-header">
        <span className="card-name">{displayName}</span>
        {card.username && <span className="card-username">@{card.username}</span>}
      </div>

      <div className="card-image-container">
        {isLoading && <div className="card-skeleton" />}
        {hasError ? (
          <div className="card-error">Failed to load</div>
        ) : (
          <img
            src={card.url}
            alt={`${displayName}'s card`}
            onLoad={() => setIsLoading(false)}
            onError={() => {
              setIsLoading(false)
              setHasError(true)
            }}
            style={{ display: isLoading ? 'none' : 'block' }}
          />
        )}
      </div>
    </div>
  )
}
