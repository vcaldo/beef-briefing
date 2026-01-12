import { useState } from 'react';
import '../styles/shop-card-image.css';

interface ShopCardImageProps {
  imageUrl: string | undefined;
  name: string;
  fallbackPhotoUrl?: string;
}

/**
 * Display a card image with 2:3 aspect ratio (like deck mini app).
 * Falls back to circular avatar if no card image is available.
 */
export function ShopCardImage({ imageUrl, name, fallbackPhotoUrl }: ShopCardImageProps) {
  const [isLoading, setIsLoading] = useState(true);
  const [hasError, setHasError] = useState(false);

  // If no card image, show circular avatar fallback
  if (!imageUrl || hasError) {
    return (
      <div className="shop-card-avatar">
        {fallbackPhotoUrl && !hasError ? (
          <img src={fallbackPhotoUrl} alt={name} />
        ) : (
          <div className="shop-card-avatar-fallback">{name[0]?.toUpperCase() || '?'}</div>
        )}
      </div>
    );
  }

  return (
    <div className="shop-card-image-container">
      {isLoading && <div className="shop-card-skeleton" />}
      <img
        src={imageUrl}
        alt={`${name}'s card`}
        onLoad={() => setIsLoading(false)}
        onError={() => {
          setIsLoading(false);
          setHasError(true);
        }}
        style={{ display: isLoading ? 'none' : 'block' }}
      />
    </div>
  );
}
