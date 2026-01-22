/**
 * CardSlot - Container for cards
 *
 * A container component for card content. Supports empty state, click interactions,
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
 * CardSlot component providing a container for cards.
 *
 * Supports empty states and interaction variants.
 */
export function CardSlot({
  children,
  empty = false,
  onClick,
  variant = 'default',
  className = '',
}: CardSlotProps) {
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
