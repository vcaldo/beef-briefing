/**
 * BattleEffect Component - Visual effects during battle
 *
 * Displays animated effects at specified positions during battle phases.
 * Supports multiple effect types with different frame sequences:
 * - attack: flame_01/02 frames (fire projectile)
 * - damage: magic_01/02 frames (impact effect)
 * - death: explosion00-08 frames (9-frame explosion sequence)
 * - spark: spark_01/02 frames (sparkle effect)
 *
 * @example
 * ```tsx
 * // Attack effect at card position
 * <BattleEffect
 *   type="attack"
 *   position={{ x: 100, y: 200 }}
 *   onComplete={() => console.log('Attack effect done')}
 * />
 *
 * // Death explosion at center
 * <BattleEffect
 *   type="death"
 *   position={{ x: 150, y: 150 }}
 *   size={96}
 * />
 * ```
 */

import React, { useState, useEffect, useCallback, useRef } from 'react'
import { useImages } from '../../hooks/useImages'
import type { EffectImageId } from '../../types/images'

/** Effect type determines which image frames to use */
export type BattleEffectType = 'attack' | 'damage' | 'death' | 'spark'

/** Position for the effect (center point) */
export interface EffectPosition {
  x: number
  y: number
}

interface BattleEffectProps {
  /** Type of effect to display */
  type: BattleEffectType
  /** Position to display the effect (center coordinates) */
  position: EffectPosition
  /** Callback when the effect animation completes */
  onComplete?: () => void
  /** Size of the effect in pixels (default: 64) */
  size?: number
  /** Additional CSS class name */
  className?: string
}

/** Frame definitions for each effect type */
interface EffectFrameConfig {
  frames: EffectImageId[]
  /** Duration per frame in milliseconds */
  frameDuration: number
  /** Whether to loop the animation */
  loop: boolean
  /** Number of loops before completing (only if loop is true) */
  loopCount?: number
}

/** Effect configurations mapping type to frames and timing */
const EFFECT_CONFIGS: Record<BattleEffectType, EffectFrameConfig> = {
  attack: {
    frames: ['flame_01', 'flame_02'],
    frameDuration: 100,
    loop: true,
    loopCount: 3,
  },
  damage: {
    frames: ['magic_01', 'magic_02'],
    frameDuration: 80,
    loop: true,
    loopCount: 2,
  },
  death: {
    frames: [
      'explosion/explosion00',
      'explosion/explosion01',
      'explosion/explosion02',
      'explosion/explosion03',
      'explosion/explosion04',
      'explosion/explosion05',
      'explosion/explosion06',
      'explosion/explosion07',
      'explosion/explosion08',
    ],
    frameDuration: 70,
    loop: false,
  },
  spark: {
    frames: ['spark_01', 'spark_02'],
    frameDuration: 60,
    loop: true,
    loopCount: 4,
  },
}

/**
 * BattleEffect - Animated visual effect component
 *
 * Renders sprite-based animations at specified positions.
 * Automatically cleans up and calls onComplete when animation finishes.
 */
export function BattleEffect({
  type,
  position,
  onComplete,
  size = 64,
  className = '',
}: BattleEffectProps): React.JSX.Element | null {
  const { getUrlById } = useImages()
  const [currentFrame, setCurrentFrame] = useState(0)
  const [isVisible, setIsVisible] = useState(true)
  const loopCountRef = useRef(0)
  const animationRef = useRef<number | null>(null)
  const lastFrameTimeRef = useRef<number>(0)

  const config = EFFECT_CONFIGS[type]

  // Animation loop using requestAnimationFrame for smooth performance
  const animate = useCallback(
    (timestamp: number) => {
      if (!lastFrameTimeRef.current) {
        lastFrameTimeRef.current = timestamp
      }

      const elapsed = timestamp - lastFrameTimeRef.current

      if (elapsed >= config.frameDuration) {
        lastFrameTimeRef.current = timestamp

        setCurrentFrame((prevFrame: number) => {
          const nextFrame = prevFrame + 1

          // Check if we've completed a full animation cycle
          if (nextFrame >= config.frames.length) {
            if (config.loop && config.loopCount) {
              loopCountRef.current++

              // Check if we've completed all loops
              if (loopCountRef.current >= config.loopCount) {
                setIsVisible(false)
                return prevFrame
              }
            } else if (!config.loop) {
              // Non-looping animation completed
              setIsVisible(false)
              return prevFrame
            }

            return 0 // Reset to first frame for looping
          }

          return nextFrame
        })
      }

      // Continue animation if still visible
      if (isVisible) {
        animationRef.current = requestAnimationFrame(animate)
      }
    },
    [config, isVisible]
  )

  // Start animation on mount
  useEffect(() => {
    animationRef.current = requestAnimationFrame(animate)

    return () => {
      if (animationRef.current !== null) {
        cancelAnimationFrame(animationRef.current)
      }
    }
  }, [animate])

  // Handle completion callback
  useEffect(() => {
    if (!isVisible && onComplete) {
      onComplete()
    }
  }, [isVisible, onComplete])

  // Don't render if animation is complete
  if (!isVisible) {
    return null
  }

  const currentFrameId = config.frames[currentFrame]
  const imageUrl = getUrlById(currentFrameId)

  // Calculate position offset to center the effect
  const offsetX = position.x - size / 2
  const offsetY = position.y - size / 2

  return (
    <div
      className={`battle-effect battle-effect-${type} ${className}`}
      style={{
        position: 'absolute',
        left: `${offsetX}px`,
        top: `${offsetY}px`,
        width: `${size}px`,
        height: `${size}px`,
        pointerEvents: 'none',
        zIndex: 100,
      }}
      aria-hidden="true"
    >
      <img
        src={imageUrl}
        alt=""
        style={{
          width: '100%',
          height: '100%',
          objectFit: 'contain',
        }}
      />
    </div>
  )
}

export default BattleEffect
