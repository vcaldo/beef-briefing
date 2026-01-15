import React from 'react'

interface CardProps {
  /** Optional title displayed in the card header */
  title?: string
  /** Optional subtitle displayed below the title */
  subtitle?: string
  /** Main content of the card */
  children?: React.ReactNode
  /** Optional footer content */
  footer?: React.ReactNode
  /** Additional CSS classes */
  className?: string
  /** Optional click handler */
  onClick?: () => void
  /** Whether the card is clickable (adds hover effects) */
  clickable?: boolean
  /** Data attributes for testing or animations */
  'data-testid'?: string
  'data-card-id'?: string
}

/**
 * Card - A reusable card wrapper component
 *
 * Provides a consistent card styling with shadow, border-radius, and
 * optional header/footer sections. Used throughout the arena mini-app
 * for displaying various content types.
 *
 * @example
 * // Basic usage with content
 * <Card>
 *   <p>Card content here</p>
 * </Card>
 *
 * @example
 * // With title and footer
 * <Card
 *   title="Player Stats"
 *   subtitle="Week 12"
 *   footer={<button className="btn btn-primary">View Details</button>}
 * >
 *   <p>Stats content...</p>
 * </Card>
 *
 * @example
 * // Clickable card
 * <Card
 *   clickable
 *   onClick={() => handleCardClick()}
 *   title="Match #123"
 * >
 *   <p>Click to view match</p>
 * </Card>
 */
export function Card({
  title,
  subtitle,
  children,
  footer,
  className = '',
  onClick,
  clickable = false,
  'data-testid': testId,
  'data-card-id': cardId,
}: CardProps): React.JSX.Element {
  const isInteractive = clickable || !!onClick

  const handleClick = () => {
    if (onClick) {
      onClick()
    }
  }

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (isInteractive && (event.key === 'Enter' || event.key === ' ')) {
      event.preventDefault()
      handleClick()
    }
  }

  return (
    <div
      className={`card ${isInteractive ? 'cursor-pointer' : ''} ${className}`.trim()}
      onClick={isInteractive ? handleClick : undefined}
      onKeyDown={isInteractive ? handleKeyDown : undefined}
      role={isInteractive ? 'button' : undefined}
      tabIndex={isInteractive ? 0 : undefined}
      data-testid={testId}
      data-card-id={cardId}
    >
      {/* Card Header (optional) */}
      {(title || subtitle) && (
        <div className="card-header">
          {title && <h3 className="card-title">{title}</h3>}
          {subtitle && <p className="card-subtitle">{subtitle}</p>}
        </div>
      )}

      {/* Card Content */}
      {children && <div className="card-content">{children}</div>}

      {/* Card Footer (optional) */}
      {footer && <div className="card-footer">{footer}</div>}
    </div>
  )
}
