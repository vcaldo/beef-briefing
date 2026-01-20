/**
 * CardSlot - Hexagon frame for cards
 *
 * A container component that provides a hexagonal frame around card content
 * using the hexagon_grey panel asset. Supports empty state, click interactions,
 * and visual variants for different states.
 *
 * Usage:
 * ```tsx
 * <CardSlot>
 *   <CardContent />
 * </CardSlot>
 *
 * <CardSlot empty onClick={handleClick}>
 *   Drop card here
 * </CardSlot>
 *
 * <CardSlot variant="selected">
 *   <SelectedCard />
 * </CardSlot>
 * ```
 */

import { ReactNode, MouseEvent } from 'react'
import { useImages } from '../../hooks/useImages'

export type CardSlotVariant = 'default' | 'hover' | 'selected'

export interface CardSlotProps {
  /** Content to render inside the slot */
  children?: ReactNode
  /** Whether the slot is empty (shows placeholder styling) */
  empty?: boolean
  /** Click handler for the slot */
  onClick?: (event: MouseEvent<HTMLDivElement>) => void
  /** Visual variant for the slot */
  variant?: CardSlotVariant
  /** Additional CSS classes */
  className?: string
}

/**
 * CardSlot component providing a hexagonal frame for cards.
 *
 * Uses the hexagon_grey panel asset as a decorative frame around
 * card content with support for empty states and interaction variants.
 */
export function CardSlot({
  children,
  empty = false,
  onClick,
  variant = 'default',
  className = '',
}: CardSlotProps) {
  const { getUrlById } = useImages()

  // Get the hexagon frame image URL
  const hexagonFrameUrl = getUrlById('hexagon_grey')

  // Determine if the slot is interactive
  const isInteractive = !!onClick

  // Build class names
  const classNames = [
    'card-slot',
    `card-slot-${variant}`,
    empty && 'card-slot-empty',
    isInteractive && 'card-slot-interactive',
    className,
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <div
      className={classNames}
      onClick={onClick}
      role={isInteractive ? 'button' : undefined}
      tabIndex={isInteractive ? 0 : undefined}
      onKeyDown={
        isInteractive
          ? (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                onClick?.(e as unknown as MouseEvent<HTMLDivElement>)
              }
            }
          : undefined
      }
    >
      {/* Hexagon frame overlay - positioned at corners */}
      <div
        className="card-slot-frame card-slot-frame-tl"
        style={{ backgroundImage: `url(${hexagonFrameUrl})` }}
        aria-hidden="true"
      />
      <div
        className="card-slot-frame card-slot-frame-tr"
        style={{ backgroundImage: `url(${hexagonFrameUrl})` }}
        aria-hidden="true"
      />
      <div
        className="card-slot-frame card-slot-frame-bl"
        style={{ backgroundImage: `url(${hexagonFrameUrl})` }}
        aria-hidden="true"
      />
      <div
        className="card-slot-frame card-slot-frame-br"
        style={{ backgroundImage: `url(${hexagonFrameUrl})` }}
        aria-hidden="true"
      />

      {/* Content area */}
      <div className="card-slot-content">
        {empty ? (
          <div className="card-slot-placeholder">
            {children || <span className="card-slot-placeholder-icon">+</span>}
          </div>
        ) : (
          children
        )}
      </div>
    </div>
  )
}

export default CardSlot
