/**
 * GameButton - Animated pressing button component
 *
 * A button component with press animation using normal/pressed image states.
 * Supports multiple variants (primary, secondary, danger, neutral) and shapes
 * (long, square) with visual feedback on interaction.
 *
 * Usage:
 * ```tsx
 * <GameButton variant="primary" onClick={handleClick}>
 *   Buy Card
 * </GameButton>
 *
 * <GameButton variant="danger" size="sm" shape="square" disabled>
 *   X
 * </GameButton>
 * ```
 */

import { ButtonHTMLAttributes, ReactNode, useState, useCallback } from 'react'
import {
  getVariantButtonImages,
  GameButtonVariant,
  ButtonShape,
} from '../../assets/pressingButtons'

export type { GameButtonVariant }
export type GameButtonSize = 'sm' | 'md' | 'lg'

export interface GameButtonProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  /** Button style variant */
  variant: GameButtonVariant
  /** Button shape - long (default) or square */
  shape?: ButtonShape
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

// Size configurations for long buttons
const LONG_SIZE_CONFIGS: Record<
  GameButtonSize,
  { height: number; minWidth: number; padding: string; fontSize: string }
> = {
  sm: { height: 32, minWidth: 80, padding: '0 12px', fontSize: '0.75rem' },
  md: { height: 44, minWidth: 110, padding: '0 20px', fontSize: '0.875rem' },
  lg: { height: 56, minWidth: 140, padding: '0 28px', fontSize: '1rem' },
}

// Size configurations for square buttons
const SQUARE_SIZE_CONFIGS: Record<
  GameButtonSize,
  { size: number; padding: string; fontSize: string }
> = {
  sm: { size: 32, padding: '0', fontSize: '0.75rem' },
  md: { size: 44, padding: '0', fontSize: '0.875rem' },
  lg: { size: 56, padding: '0', fontSize: '1rem' },
}

/**
 * GameButton component with press animation.
 *
 * Uses pressing button images that swap between normal and pressed states
 * on mousedown/touchstart for tactile visual feedback.
 */
export function GameButton({
  variant,
  shape = 'long',
  size = 'md',
  disabled = false,
  onClick,
  children,
  className = '',
  ...buttonProps
}: GameButtonProps) {
  const [isPressed, setIsPressed] = useState(false)

  // Get button images for this variant and shape
  const buttonImages = getVariantButtonImages(variant, shape)
  const currentImage = isPressed && !disabled ? buttonImages.pressed : buttonImages.normal

  // Press handlers
  const handlePressStart = useCallback(() => {
    if (!disabled) {
      setIsPressed(true)
    }
  }, [disabled])

  const handlePressEnd = useCallback(() => {
    setIsPressed(false)
  }, [])

  // Get size configuration based on shape
  const isSquare = shape === 'square'
  const longConfig = LONG_SIZE_CONFIGS[size]
  const squareConfig = SQUARE_SIZE_CONFIGS[size]

  const style = isSquare
    ? {
        backgroundImage: `url(${currentImage})`,
        backgroundSize: '100% 100%',
        backgroundPosition: 'center',
        backgroundRepeat: 'no-repeat',
        width: squareConfig.size,
        height: squareConfig.size,
        minWidth: squareConfig.size,
        padding: squareConfig.padding,
        fontSize: squareConfig.fontSize,
      }
    : {
        backgroundImage: `url(${currentImage})`,
        backgroundSize: '100% 100%',
        backgroundPosition: 'center',
        backgroundRepeat: 'no-repeat',
        height: longConfig.height,
        minWidth: longConfig.minWidth,
        padding: longConfig.padding,
        fontSize: longConfig.fontSize,
      }

  return (
    <button
      type="button"
      disabled={disabled}
      onClick={disabled ? undefined : onClick}
      onMouseDown={handlePressStart}
      onMouseUp={handlePressEnd}
      onMouseLeave={handlePressEnd}
      onTouchStart={handlePressStart}
      onTouchEnd={handlePressEnd}
      onTouchCancel={handlePressEnd}
      className={`game-button game-button-${variant} game-button-${size} ${
        isSquare ? 'game-button-square' : 'game-button-long'
      } ${isPressed && !disabled ? 'game-button-pressed' : ''} ${
        disabled ? 'game-button-disabled' : ''
      } ${className}`}
      style={style}
      {...buttonProps}
    >
      <span className="game-button-content">{children}</span>
    </button>
  )
}

export default GameButton
