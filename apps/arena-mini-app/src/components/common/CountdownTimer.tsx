import { useState, useEffect, useRef } from 'react'

interface CountdownTimerProps {
  /**
   * Absolute deadline as ISO string (e.g., "2025-01-15T12:00:00Z").
   * Timer counts down to this time.
   * Either deadline or timeRemaining must be provided.
   */
  deadline?: string | null

  /**
   * Relative time remaining in seconds.
   * Timer counts down from this value.
   * Either deadline or timeRemaining must be provided.
   */
  timeRemaining?: number

  /**
   * Callback fired when timer reaches zero.
   * Only called once when transitioning to 0.
   */
  onExpire?: () => void

  /**
   * Additional CSS class names.
   */
  className?: string

  /**
   * Show clock icon before time.
   * @default true
   */
  showIcon?: boolean

  /**
   * Label to show before the time (e.g., "Deadline:").
   */
  label?: string
}

/**
 * Countdown timer component for arena match deadlines.
 *
 * CRITICAL: State is initialized to 0 and calculated in useEffect
 * to avoid React error #310 in production builds.
 *
 * Color states:
 * - Green (safe): >2 minutes remaining
 * - Yellow (warning): 30 seconds to 2 minutes
 * - Red with pulse (urgent): <30 seconds
 *
 * @example With absolute deadline
 * <CountdownTimer deadline="2025-01-15T12:00:00Z" onExpire={handleExpire} />
 *
 * @example With relative time remaining
 * <CountdownTimer timeRemaining={120} onExpire={handleExpire} />
 */
export function CountdownTimer({
  deadline,
  timeRemaining,
  onExpire,
  className = '',
  showIcon = true,
  label,
}: CountdownTimerProps) {
  // CRITICAL: Initialize to 0, NOT using lazy initializer with Date.now()
  // This prevents React error #310 caused by non-deterministic state initialization
  const [remainingSeconds, setRemainingSeconds] = useState(0)

  // Track if onExpire has been called to prevent multiple calls
  const hasExpiredRef = useRef(false)

  // Track if initial calculation has been done
  const isInitializedRef = useRef(false)

  // Calculate remaining seconds from deadline or timeRemaining prop
  useEffect(() => {
    // Calculate initial remaining time
    const calculateRemaining = (): number => {
      if (deadline) {
        const deadlineTime = new Date(deadline).getTime()
        const now = Date.now()
        const diff = Math.floor((deadlineTime - now) / 1000)
        return Math.max(0, diff)
      }
      if (typeof timeRemaining === 'number') {
        return Math.max(0, Math.floor(timeRemaining))
      }
      return 0
    }

    // Set initial value
    const initial = calculateRemaining()
    setRemainingSeconds(initial)
    isInitializedRef.current = true

    // Reset expired flag if timer is reset with new props
    if (initial > 0) {
      hasExpiredRef.current = false
    }

    // Set up interval to update every second
    const intervalId = setInterval(() => {
      setRemainingSeconds((prev) => {
        // For deadline-based timer, recalculate from deadline each tick
        // to stay in sync with actual time (handles tab switching, etc.)
        if (deadline) {
          const deadlineTime = new Date(deadline).getTime()
          const now = Date.now()
          const diff = Math.floor((deadlineTime - now) / 1000)
          return Math.max(0, diff)
        }

        // For relative timer, just decrement
        const next = prev - 1
        return Math.max(0, next)
      })
    }, 1000)

    // Cleanup interval on unmount or prop change
    return () => {
      clearInterval(intervalId)
    }
  }, [deadline, timeRemaining])

  // Handle expiration callback
  useEffect(() => {
    if (
      isInitializedRef.current &&
      remainingSeconds === 0 &&
      !hasExpiredRef.current &&
      onExpire
    ) {
      hasExpiredRef.current = true
      onExpire()
    }
  }, [remainingSeconds, onExpire])

  // Format seconds as MM:SS
  const formatTime = (seconds: number): string => {
    const mins = Math.floor(seconds / 60)
    const secs = seconds % 60
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  // Determine color state class
  const getStateClass = (): string => {
    if (remainingSeconds > 120) {
      // > 2 minutes: safe (green)
      return 'safe'
    }
    if (remainingSeconds > 30) {
      // 30s - 2m: warning (yellow)
      return 'warning'
    }
    // < 30s: urgent (red with pulse)
    return 'urgent'
  }

  const stateClass = getStateClass()

  return (
    <div
      className={`countdown-timer ${stateClass} ${className}`.trim()}
      role="timer"
      aria-live="polite"
      aria-label={`Time remaining: ${formatTime(remainingSeconds)}`}
    >
      {showIcon && (
        <span className="countdown-timer-icon" aria-hidden="true">
          <ClockIcon />
        </span>
      )}
      {label && <span className="countdown-timer-label">{label}</span>}
      <span className="countdown-timer-value font-mono">
        {formatTime(remainingSeconds)}
      </span>
    </div>
  )
}

/**
 * Clock SVG icon for countdown timer.
 */
function ClockIcon() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="10" />
      <polyline points="12,6 12,12 16,14" />
    </svg>
  )
}
