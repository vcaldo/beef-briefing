import { useRef, useEffect, useState, useCallback } from 'react'

interface CardRevealProps {
  cardUrl: string
  cardName: string
  onComplete: () => void
  onShareStory?: () => void
  onDownload?: () => void
  showShareStory?: boolean
}

interface Particle {
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

const PARTICLE_COLORS = [
  '#FFD700', // Gold
  '#FFA500', // Orange
  '#FFFFFF', // White
  '#FFE4B5', // Moccasin
  '#F0E68C', // Khaki
  '#FFEC8B', // Light goldenrod
]

function useParticleSystem(canvasRef: React.RefObject<HTMLCanvasElement | null>) {
  const particles = useRef<Particle[]>([])
  const animationId = useRef<number>(0)
  const startTime = useRef<number>(0)

  const createBurstParticle = useCallback((cx: number, cy: number): Particle => {
    const angle = Math.random() * Math.PI * 2
    const speed = 2 + Math.random() * 4
    return {
      x: cx,
      y: cy,
      vx: Math.cos(angle) * speed,
      vy: Math.sin(angle) * speed - 2,
      size: 1 + Math.random() * 3,
      color: PARTICLE_COLORS[Math.floor(Math.random() * PARTICLE_COLORS.length)],
      alpha: 1,
      decay: 0.008 + Math.random() * 0.012,
      rotation: Math.random() * Math.PI * 2,
      rotationSpeed: (Math.random() - 0.5) * 0.2,
    }
  }, [])

  const animate = useCallback((timestamp: number) => {
    const canvas = canvasRef.current
    if (!canvas) return

    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const elapsed = timestamp - startTime.current

    ctx.clearRect(0, 0, canvas.width, canvas.height)

    // Add continuous sparkles for first 2 seconds
    if (elapsed < 2000) {
      for (let i = 0; i < 4; i++) {
        particles.current.push(
          createBurstParticle(
            canvas.width / 2 + (Math.random() - 0.5) * 200,
            canvas.height / 2 + (Math.random() - 0.5) * 300
          )
        )
      }
    }

    // Update and draw particles
    particles.current = particles.current.filter((p) => {
      p.x += p.vx
      p.y += p.vy
      p.vy += 0.05 // Gravity
      p.alpha -= p.decay
      p.rotation += p.rotationSpeed

      if (p.alpha <= 0) return false

      // Draw 4-point star
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

      return true
    })

    if (particles.current.length > 0 || elapsed < 2000) {
      animationId.current = requestAnimationFrame(animate)
    }
  }, [canvasRef, createBurstParticle])

  const startAnimation = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    // Initial burst
    const cx = canvas.width / 2
    const cy = canvas.height / 2
    for (let i = 0; i < 80; i++) {
      particles.current.push(createBurstParticle(cx, cy))
    }

    startTime.current = performance.now()
    animationId.current = requestAnimationFrame(animate)
  }, [canvasRef, createBurstParticle, animate])

  const stopAnimation = useCallback(() => {
    if (animationId.current) {
      cancelAnimationFrame(animationId.current)
    }
    particles.current = []
  }, [])

  return { startAnimation, stopAnimation }
}

export function CardReveal({
  cardUrl,
  cardName,
  onComplete,
  onShareStory,
  onDownload,
  showShareStory = false,
}: CardRevealProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [isFlipped, setIsFlipped] = useState(false)
  const [canDismiss, setCanDismiss] = useState(false)
  const [imageLoaded, setImageLoaded] = useState(false)
  const { startAnimation, stopAnimation } = useParticleSystem(canvasRef)

  // Preload the card image
  useEffect(() => {
    const img = new Image()
    img.src = cardUrl
    img.onload = () => {
      setImageLoaded(true)
      // Start flip animation after image is loaded
      setTimeout(() => setIsFlipped(true), 300)
    }
  }, [cardUrl])

  // Start particle animation when flip reaches midpoint
  useEffect(() => {
    if (isFlipped) {
      // Delay to sync with flip midpoint
      setTimeout(() => {
        startAnimation()
      }, 600)

      // Allow dismiss after animation completes
      setTimeout(() => {
        setCanDismiss(true)
      }, 2000)
    }

    return () => stopAnimation()
  }, [isFlipped, startAnimation, stopAnimation])

  // Resize canvas to match window
  useEffect(() => {
    const resizeCanvas = () => {
      const canvas = canvasRef.current
      if (!canvas) return
      canvas.width = window.innerWidth
      canvas.height = window.innerHeight
    }

    resizeCanvas()
    window.addEventListener('resize', resizeCanvas)
    return () => window.removeEventListener('resize', resizeCanvas)
  }, [])

  const handleDismiss = useCallback(() => {
    if (canDismiss) {
      onComplete()
    }
  }, [canDismiss, onComplete])

  return (
    <div className="card-reveal-overlay" onClick={handleDismiss}>
      <canvas ref={canvasRef} className="sparkle-canvas" />

      <div className="card-reveal-container">
        <div className={`card-reveal-inner ${isFlipped ? 'flipped' : ''}`}>
          <div className="card-reveal-glow" />

          {/* Back of card (shown first) */}
          <div className="card-reveal-back">
            <div className="card-reveal-back-pattern" />
            <div className="card-reveal-back-logo">
              <span>Beef</span>
              <span>Briefing</span>
            </div>
          </div>

          {/* Front of card (revealed after flip) */}
          <div className="card-reveal-front">
            {imageLoaded && <img src={cardUrl} alt={`${cardName}'s card`} />}
          </div>
        </div>
      </div>

      {canDismiss && (
        <div className="card-reveal-actions" onClick={(e) => e.stopPropagation()}>
          {showShareStory && onShareStory && (
            <button className="card-action-btn" onClick={onShareStory}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10"/>
                <circle cx="12" cy="12" r="3"/>
              </svg>
              Story
            </button>
          )}
          {onDownload && (
            <button className="card-action-btn" onClick={onDownload}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                <polyline points="7 10 12 15 17 10"/>
                <line x1="12" y1="15" x2="12" y2="3"/>
              </svg>
              Save
            </button>
          )}
          <button className="card-action-btn card-action-btn-secondary" onClick={onComplete}>
            Continue
          </button>
        </div>
      )}
    </div>
  )
}
