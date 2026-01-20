/**
 * HPBar Component - Image-based HP bar using Sci-fi bar assets
 *
 * Renders an HP bar using image assets for left/mid/right segments.
 * The middle segment repeats to fill the available width based on HP percentage.
 * Supports green (>33% HP) and red (<=33% HP) color variants with smooth transitions.
 *
 * @example
 * ```tsx
 * // Basic usage
 * <HPBar current={75} max={100} />
 *
 * // With animation disabled
 * <HPBar current={30} max={100} animated={false} />
 *
 * // Custom width
 * <HPBar current={50} max={100} width={120} />
 * ```
 */

import React, { useMemo } from 'react'
import { useImages } from '../../hooks/useImages'
import type { BarImageId } from '../../types/images'

interface HPBarProps {
  /** Current HP value */
  current: number
  /** Maximum HP value */
  max: number
  /** Whether to animate HP changes (default: true) */
  animated?: boolean
  /** Optional width in pixels (default: 100) */
  width?: number
  /** Optional height in pixels (default: 16, matches asset height) */
  height?: number
  /** Additional CSS class name */
  className?: string
}

/** Default threshold for switching from green to red (percentage) */
const LOW_HP_THRESHOLD = 33

/** Width of the left/right cap images in pixels */
const CAP_WIDTH = 8

/**
 * HPBar - Image-based HP bar component
 *
 * Uses Sci-fi bar assets from the image store:
 * - Left cap: barHorizontal_{color}_left.png (8x16)
 * - Middle: barHorizontal_{color}_mid.png (1x16, repeating)
 * - Right cap: barHorizontal_{color}_right.png (8x16)
 *
 * The bar smoothly transitions between green and red based on HP percentage.
 */
export function HPBar({
  current,
  max,
  animated = true,
  width = 100,
  height = 16,
  className = '',
}: HPBarProps): React.JSX.Element {
  const { getUrlById } = useImages()

  // Calculate HP percentage
  const percentage = useMemo(() => {
    if (max <= 0) return 0
    return Math.max(0, Math.min(100, (current / max) * 100))
  }, [current, max])

  // Determine color variant based on HP percentage
  const isLowHp = percentage <= LOW_HP_THRESHOLD
  const colorVariant = isLowHp ? 'red' : 'green'

  // Get image URLs for the current color variant
  const leftCapUrl = getUrlById(`barHorizontal_${colorVariant}_left` as BarImageId)
  const midUrl = getUrlById(`barHorizontal_${colorVariant}_mid` as BarImageId)
  const rightCapUrl = getUrlById(`barHorizontal_${colorVariant}_right` as BarImageId)

  // Calculate the fill width (total bar width minus both caps, then apply percentage)
  // The fill section includes: left cap + middle + right cap
  const fillableWidth = width - (CAP_WIDTH * 2) // Space for middle section only
  const fillWidth = (fillableWidth * percentage) / 100

  // Calculate total fill section width (caps + middle fill)
  const totalFillWidth = percentage > 0 ? CAP_WIDTH + fillWidth + CAP_WIDTH : 0

  // Build class names
  const barClasses = [
    'hp-bar-image',
    animated && 'hp-bar-animated',
    isLowHp && 'hp-bar-critical',
    className,
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <div
      className={barClasses}
      style={{
        width: `${width}px`,
        height: `${height}px`,
        position: 'relative',
        backgroundColor: 'rgba(0, 0, 0, 0.6)',
        borderRadius: '2px',
        overflow: 'hidden',
      }}
      role="progressbar"
      aria-valuenow={current}
      aria-valuemin={0}
      aria-valuemax={max}
      aria-label={`HP: ${current} of ${max}`}
    >
      {/* HP fill container - animates width changes */}
      <div
        className="hp-bar-image-fill"
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          height: '100%',
          width: `${totalFillWidth}px`,
          display: 'flex',
          alignItems: 'center',
          transition: animated
            ? 'width var(--hp-transition-duration, 0.3s) ease'
            : 'none',
          overflow: 'hidden',
        }}
      >
        {/* Left cap */}
        {percentage > 0 && (
          <img
            src={leftCapUrl}
            alt=""
            aria-hidden="true"
            style={{
              width: `${CAP_WIDTH}px`,
              height: `${height}px`,
              flexShrink: 0,
            }}
          />
        )}

        {/* Middle repeating section */}
        {percentage > 0 && fillWidth > 0 && (
          <div
            style={{
              width: `${fillWidth}px`,
              height: `${height}px`,
              backgroundImage: `url(${midUrl})`,
              backgroundRepeat: 'repeat-x',
              backgroundSize: `1px ${height}px`,
              flexShrink: 0,
            }}
            aria-hidden="true"
          />
        )}

        {/* Right cap */}
        {percentage > 0 && (
          <img
            src={rightCapUrl}
            alt=""
            aria-hidden="true"
            style={{
              width: `${CAP_WIDTH}px`,
              height: `${height}px`,
              flexShrink: 0,
            }}
          />
        )}
      </div>
    </div>
  )
}

export default HPBar
