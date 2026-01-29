/**
 * @fileoverview Particle system for victory celebration animations.
 *
 * Adapted from deck-mini-app's CardReveal particle system.
 * Creates gold/orange star particles with physics-based animation.
 *
 * @module hooks/useVictoryParticles
 */

import { useRef, useCallback } from 'react'

// =============================================================================
// TYPES
// =============================================================================

export interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  size: number
  color: string
  alpha: number
  decay: number
  rotation: number
  rotationSpeed: number
}

export interface UseVictoryParticlesOptions {
  /** Animation duration in ms (default: 2000) */
  duration?: number
  /** Initial burst particle count (default: 60) */
  burstCount?: number
  /** Particles per frame during continuous phase (default: 3) */
  continuousRate?: number
  /** Horizontal spread for continuous particles (default: 150) */
  spreadX?: number
  /** Vertical spread for continuous particles (default: 100) */
  spreadY?: number
}

export interface UseVictoryParticlesReturn {
  /** Start the particle animation at given position */
  startAnimation: (centerX: number, centerY: number) => void
  /** Stop the animation and clear particles */
  stopAnimation: () => void
  /** Ref to attach to canvas element */
  canvasRef: React.RefObject<HTMLCanvasElement>
}

// =============================================================================
// CONSTANTS
// =============================================================================

/** Gold/warm color palette for victory particles */
const PARTICLE_COLORS = [
  '#FFD700', // Gold
  '#FFA500', // Orange
  '#FFFFFF', // White
  '#FFE4B5', // Moccasin
  '#F0E68C', // Khaki
  '#FFEC8B', // Light goldenrod
]

// =============================================================================
// HOOK
// =============================================================================

/**
 * Hook for managing victory celebration particle animations.
 *
 * Uses Canvas 2D API with requestAnimationFrame for smooth 60fps rendering.
 * Particles are 4-point stars with physics-based movement (velocity + gravity).
 *
 * @example
 * ```tsx
 * const { canvasRef, startAnimation, stopAnimation } = useVictoryParticles()
 *
 * // Start particles at deck position
 * startAnimation(deckCenterX, deckCenterY)
 *
 * // Cleanup on unmount
 * useEffect(() => () => stopAnimation(), [stopAnimation])
 *
 * return <canvas ref={canvasRef} className="victory-particles" />
 * ```
 */
export function useVictoryParticles(
  options: UseVictoryParticlesOptions = {}
): UseVictoryParticlesReturn {
  const {
    duration = 2000,
    burstCount = 60,
    continuousRate = 3,
    spreadX = 150,
    spreadY = 100,
  } = options

  const canvasRef = useRef<HTMLCanvasElement>(null)
  const particles = useRef<Particle[]>([])
  const animationId = useRef<number>(0)
  const startTime = useRef<number>(0)
  const centerPosition = useRef<{ x: number; y: number }>({ x: 0, y: 0 })

  /**
   * Create a single particle with randomized properties.
   */
  const createBurstParticle = useCallback((cx: number, cy: number): Particle => {
    const angle = Math.random() * Math.PI * 2
    const speed = 2 + Math.random() * 4
    return {
      x: cx,
      y: cy,
      vx: Math.cos(angle) * speed,
      vy: Math.sin(angle) * speed - 2, // Initial upward bias
      size: 1 + Math.random() * 3,
      color: PARTICLE_COLORS[Math.floor(Math.random() * PARTICLE_COLORS.length)],
      alpha: 1,
      decay: 0.008 + Math.random() * 0.012, // Variable fade rate
      rotation: Math.random() * Math.PI * 2,
      rotationSpeed: (Math.random() - 0.5) * 0.2,
    }
  }, [])

  /**
   * Draw a 4-point star at the particle's position.
   */
  const drawStar = useCallback((ctx: CanvasRenderingContext2D, p: Particle) => {
    ctx.save()
    ctx.translate(p.x, p.y)
    ctx.rotate(p.rotation)
    ctx.globalAlpha = p.alpha
    ctx.fillStyle = p.color

    ctx.beginPath()
    for (let i = 0; i < 4; i++) {
      const outerAngle = (i * Math.PI) / 2
      const outerX = Math.cos(outerAngle) * p.size * 2
      const outerY = Math.sin(outerAngle) * p.size * 2
      const innerAngle = outerAngle + Math.PI / 4
      const innerX = Math.cos(innerAngle) * p.size * 0.5
      const innerY = Math.sin(innerAngle) * p.size * 0.5

      if (i === 0) ctx.moveTo(outerX, outerY)
      else ctx.lineTo(outerX, outerY)
      ctx.lineTo(innerX, innerY)
    }
    ctx.closePath()
    ctx.fill()
    ctx.restore()
  }, [])

  /**
   * Animation loop - updates and renders all particles.
   */
  const animate = useCallback(
    (timestamp: number) => {
      const canvas = canvasRef.current
      if (!canvas) return

      const ctx = canvas.getContext('2d')
      if (!ctx) return

      const elapsed = timestamp - startTime.current
      const { x: cx, y: cy } = centerPosition.current

      // Clear canvas
      ctx.clearRect(0, 0, canvas.width, canvas.height)

      // Add continuous sparkles during animation duration
      if (elapsed < duration) {
        for (let i = 0; i < continuousRate; i++) {
          particles.current.push(
            createBurstParticle(
              cx + (Math.random() - 0.5) * spreadX * 2,
              cy + (Math.random() - 0.5) * spreadY * 2
            )
          )
        }
      }

      // Update and draw particles
      particles.current = particles.current.filter((p) => {
        // Physics update
        p.x += p.vx
        p.y += p.vy
        p.vy += 0.05 // Gravity
        p.alpha -= p.decay
        p.rotation += p.rotationSpeed

        if (p.alpha <= 0) return false

        // Draw the particle
        drawStar(ctx, p)

        return true
      })

      // Continue animation if particles exist or still within duration
      if (particles.current.length > 0 || elapsed < duration) {
        animationId.current = requestAnimationFrame(animate)
      }
    },
    [createBurstParticle, drawStar, duration, continuousRate, spreadX, spreadY]
  )

  /**
   * Start the particle animation at the specified center position.
   */
  const startAnimation = useCallback(
    (centerX: number, centerY: number) => {
      const canvas = canvasRef.current
      if (!canvas) return

      // Store center position for continuous particles
      centerPosition.current = { x: centerX, y: centerY }

      // Clear any existing animation
      if (animationId.current) {
        cancelAnimationFrame(animationId.current)
      }
      particles.current = []

      // Initial burst at center
      for (let i = 0; i < burstCount; i++) {
        particles.current.push(createBurstParticle(centerX, centerY))
      }

      startTime.current = performance.now()
      animationId.current = requestAnimationFrame(animate)
    },
    [animate, burstCount, createBurstParticle]
  )

  /**
   * Stop the animation and clear all particles.
   */
  const stopAnimation = useCallback(() => {
    if (animationId.current) {
      cancelAnimationFrame(animationId.current)
      animationId.current = 0
    }
    particles.current = []

    // Clear the canvas
    const canvas = canvasRef.current
    if (canvas) {
      const ctx = canvas.getContext('2d')
      if (ctx) {
        ctx.clearRect(0, 0, canvas.width, canvas.height)
      }
    }
  }, [])

  return {
    canvasRef,
    startAnimation,
    stopAnimation,
  }
}
