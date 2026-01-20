/**
 * CoinDisplay - Coin icon with animated counter
 *
 * Displays a coin icon alongside a numeric value with optional
 * pulse animation when the amount changes.
 *
 * Usage:
 * ```tsx
 * <CoinDisplay amount={10} />
 *
 * <CoinDisplay amount={coins} size="lg" animated />
 * ```
 */

import { useEffect, useRef, useState } from 'react'
import { useImages } from '../../hooks/useImages'

export type CoinDisplaySize = 'sm' | 'md' | 'lg'

export interface CoinDisplayProps {
  /** Number of coins to display */
  amount: number
  /** Size variant */
  size?: CoinDisplaySize
  /** Whether to animate on value change */
  animated?: boolean
  /** Additional CSS classes */
  className?: string
}

// Size configurations for icon and text
const SIZE_CONFIGS: Record<CoinDisplaySize, { iconSize: number; fontSize: string; gap: string }> = {
  sm: { iconSize: 16, fontSize: '0.875rem', gap: '4px' },
  md: { iconSize: 24, fontSize: '1.125rem', gap: '6px' },
  lg: { iconSize: 32, fontSize: '1.5rem', gap: '8px' },
}

/**
 * CoinDisplay component showing coin icon with counter.
 *
 * Features:
 * - Responsive sizing (sm, md, lg)
 * - Optional pulse animation on value change
 * - Uses coin icon from object storage
 */
export function CoinDisplay({
  amount,
  size = 'md',
  animated = false,
  className = '',
}: CoinDisplayProps) {
  const { getUrlById } = useImages()
  const [isPulsing, setIsPulsing] = useState(false)
  const prevAmountRef = useRef(amount)

  // Get the coin icon URL
  const coinIconUrl = getUrlById('coin')

  // Get size configuration
  const sizeConfig = SIZE_CONFIGS[size]

  // Trigger pulse animation when amount changes (if animated is true)
  useEffect(() => {
    if (animated && prevAmountRef.current !== amount) {
      setIsPulsing(true)
      const timer = setTimeout(() => setIsPulsing(false), 300)
      prevAmountRef.current = amount
      return () => clearTimeout(timer)
    }
    prevAmountRef.current = amount
  }, [amount, animated])

  return (
    <div
      className={`coin-display coin-display-${size} ${isPulsing ? 'coin-display-pulse' : ''} ${className}`}
      style={{
        gap: sizeConfig.gap,
        fontSize: sizeConfig.fontSize,
      }}
    >
      <img
        src={coinIconUrl}
        alt="Coins"
        className="coin-display-icon"
        style={{
          width: sizeConfig.iconSize,
          height: sizeConfig.iconSize,
        }}
      />
      <span className="coin-display-amount">{amount}</span>
    </div>
  )
}

export default CoinDisplay
