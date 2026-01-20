/**
 * GameButton - Themed button using image assets
 *
 * A button component styled with game UI image assets as backgrounds.
 * Supports multiple variants (primary, secondary, danger, neutral) with
 * hover, active, and disabled states using CSS filters.
 *
 * Usage:
 * ```tsx
 * <GameButton variant="primary" onClick={handleClick}>
 *   Buy Card
 * </GameButton>
 *
 * <GameButton variant="danger" size="sm" disabled>
 *   Delete
 * </GameButton>
 * ```
 */

import { ButtonHTMLAttributes, ReactNode } from 'react'
import { useImages } from '../../hooks/useImages'
import { ButtonImageId } from '../../types/images'

export type GameButtonVariant = 'primary' | 'secondary' | 'danger' | 'neutral'
export type GameButtonSize = 'sm' | 'md' | 'lg'

export interface GameButtonProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  /** Button style variant */
  variant: GameButtonVariant
  /** Button size */
  size?: GameButtonSize
  /** Whether the button is disabled */
  disabled?: boolean
  /** Click handler */
  onClick?: () => void
  /** Button content */
  children: ReactNode
  /** Additional CSS classes */
  className?: string
}

// Size configurations for each button size
const SIZE_CONFIGS: Record<GameButtonSize, { height: number; padding: string; fontSize: string }> = {
  sm: { height: 32, padding: '0 12px', fontSize: '0.75rem' },
  md: { height: 44, padding: '0 20px', fontSize: '0.875rem' },
  lg: { height: 56, padding: '0 28px', fontSize: '1rem' },
}

/**
 * GameButton component with image-based styling.
 *
 * Uses button images from object storage as backgrounds with CSS
 * filters for hover, active, and disabled states.
 */
export function GameButton({
  variant,
  size = 'md',
  disabled = false,
  onClick,
  children,
  className = '',
  ...buttonProps
}: GameButtonProps) {
  const { getUrlById } = useImages()

  // Get the button image URL for the variant
  const buttonImageUrl = getUrlById(variant as ButtonImageId)

  // Get size configuration
  const sizeConfig = SIZE_CONFIGS[size]

  return (
    <button
      type="button"
      disabled={disabled}
      onClick={disabled ? undefined : onClick}
      className={`game-button game-button-${variant} game-button-${size} ${
        disabled ? 'game-button-disabled' : ''
      } ${className}`}
      style={{
        backgroundImage: `url(${buttonImageUrl})`,
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        backgroundRepeat: 'no-repeat',
        height: sizeConfig.height,
        padding: sizeConfig.padding,
        fontSize: sizeConfig.fontSize,
        minWidth: sizeConfig.height * 2.5,
      }}
      {...buttonProps}
    >
      <span className="game-button-content">{children}</span>
    </button>
  )
}

export default GameButton
