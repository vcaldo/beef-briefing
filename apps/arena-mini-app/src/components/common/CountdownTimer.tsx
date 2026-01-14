import { useState, useEffect } from 'react'

interface CountdownTimerProps {
  /** Deadline as ISO string or Date */
  deadline?: string | Date
  /** Time remaining in seconds (alternative to deadline) */
  timeRemainingSeconds?: number
  /** Callback when timer reaches zero */
  onComplete?: () => void
  /** Show compact format (mm:ss only) */
  compact?: boolean
}

/**
 * Countdown timer with urgency states.
 * Changes color based on time remaining:
 * - relaxed: > 30 seconds (green)
 * - warning: 10-30 seconds (yellow)
 * - urgent: < 10 seconds (red, pulsing)
 */
export function CountdownTimer({
  deadline,
  timeRemainingSeconds,
  onComplete,
  compact = false,
}: CountdownTimerProps) {
  const [secondsLeft, setSecondsLeft] = useState<number>(() => {
    if (timeRemainingSeconds !== undefined) {
      return Math.max(0, timeRemainingSeconds)
    }
    if (deadline) {
      const deadlineDate = typeof deadline === 'string' ? new Date(deadline) : deadline
      return Math.max(0, Math.floor((deadlineDate.getTime() - Date.now()) / 1000))
    }
    return 0
  })

  useEffect(() => {
    // Re-calculate when props change
    if (timeRemainingSeconds !== undefined) {
      setSecondsLeft(Math.max(0, timeRemainingSeconds))
    } else if (deadline) {
      const deadlineDate = typeof deadline === 'string' ? new Date(deadline) : deadline
      setSecondsLeft(Math.max(0, Math.floor((deadlineDate.getTime() - Date.now()) / 1000)))
    }
  }, [deadline, timeRemainingSeconds])

  useEffect(() => {
    if (secondsLeft <= 0) {
      onComplete?.()
      return
    }

    const interval = setInterval(() => {
      setSecondsLeft((prev) => {
        const next = prev - 1
        if (next <= 0) {
          onComplete?.()
          return 0
        }
        return next
      })
    }, 1000)

    return () => clearInterval(interval)
  }, [secondsLeft, onComplete])

  // Determine urgency state
  const getUrgencyClass = () => {
    if (secondsLeft > 30) return 'relaxed'
    if (secondsLeft > 10) return 'warning'
    return 'urgent'
  }

  // Format time display
  const formatTime = () => {
    const minutes = Math.floor(secondsLeft / 60)
    const seconds = secondsLeft % 60

    if (compact) {
      return `${minutes}:${seconds.toString().padStart(2, '0')}`
    }

    if (minutes > 0) {
      return `${minutes}:${seconds.toString().padStart(2, '0')}`
    }
    return `${seconds}s`
  }

  return (
    <div className={`countdown-timer ${getUrgencyClass()}`}>
      <span className="countdown-icon">⏱</span>
      <span className="countdown-value">{formatTime()}</span>
    </div>
  )
}
