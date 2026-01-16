import React, { useEffect, useRef, useState } from 'react'

/**
 * SplashScreen - Animated splash screen for Arena Mini-App initialization
 *
 * Displays an animated logo and title with a loading spinner while the app
 * initializes. Enforces a minimum 2-second display time to ensure smooth
 * user experience and prevent jarring transitions.
 *
 * @example
 * // Basic usage with minimum time callback
 * const [minTimeElapsed, setMinTimeElapsed] = useState(false)
 *
 * if (isLoading || !minTimeElapsed) {
 *   return <SplashScreen onMinTimeElapsed={() => setMinTimeElapsed(true)} />
 * }
 *
 * @example
 * // Custom message during load
 * <SplashScreen
 *   message="Connecting to arena..."
 *   onMinTimeElapsed={() => setReady(true)}
 * />
 */

export interface SplashScreenProps {
  /** Optional message to display below the logo */
  message?: string
  /** Callback when minimum display time (2s) has elapsed */
  onMinTimeElapsed?: () => void
  /** Minimum display time in milliseconds (default: 2000) */
  minDisplayTime?: number
  /** Additional CSS class names */
  className?: string
  /** Test ID for testing */
  'data-testid'?: string
}

/**
 * Arena logo component - Crossed swords in a shield
 */
const ArenaLogo: React.FC<{ className?: string }> = ({ className = '' }) => (
  <svg
    className={className}
    width="80"
    height="80"
    viewBox="0 0 80 80"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    aria-hidden="true"
  >
    {/* Shield background */}
    <path
      d="M40 4L8 16V36C8 54.778 21.778 70.556 40 76C58.222 70.556 72 54.778 72 36V16L40 4Z"
      fill="url(#shield-gradient)"
      stroke="var(--arena-orange)"
      strokeWidth="2"
    />
    {/* Inner shield glow */}
    <path
      d="M40 10L14 20V36C14 51.778 25.778 65.556 40 70C54.222 65.556 66 51.778 66 36V20L40 10Z"
      fill="var(--arena-bg-dark)"
      fillOpacity="0.8"
    />
    {/* Left sword */}
    <path
      d="M28 52L24 56L32 64L36 60L28 52Z"
      fill="var(--arena-blue-light)"
    />
    <path
      d="M36 44L28 52L36 60L40 56L40 48L36 44Z"
      fill="var(--arena-blue)"
    />
    <path
      d="M38 20L36 44L40 48L44 44L42 20H38Z"
      fill="url(#blade-gradient)"
    />
    {/* Right sword (mirrored) */}
    <path
      d="M52 52L56 56L48 64L44 60L52 52Z"
      fill="var(--arena-orange-light)"
    />
    <path
      d="M44 44L52 52L44 60L40 56L40 48L44 44Z"
      fill="var(--arena-orange)"
    />
    {/* Center gem */}
    <circle
      cx="40"
      cy="36"
      r="6"
      fill="var(--arena-purple)"
      className="animate-pulse"
    />
    <circle
      cx="40"
      cy="36"
      r="3"
      fill="var(--arena-pink)"
      opacity="0.8"
    />
    {/* Gradients */}
    <defs>
      <linearGradient id="shield-gradient" x1="40" y1="4" x2="40" y2="76">
        <stop offset="0%" stopColor="var(--arena-bg-light)" />
        <stop offset="100%" stopColor="var(--arena-bg-medium)" />
      </linearGradient>
      <linearGradient id="blade-gradient" x1="40" y1="20" x2="40" y2="48">
        <stop offset="0%" stopColor="#e2e8f0" />
        <stop offset="50%" stopColor="#94a3b8" />
        <stop offset="100%" stopColor="#64748b" />
      </linearGradient>
    </defs>
  </svg>
)

export const SplashScreen: React.FC<SplashScreenProps> = ({
  message,
  onMinTimeElapsed,
  minDisplayTime = 2000,
  className = '',
  'data-testid': testId = 'splash-screen',
}) => {
  const [showSpinner, setShowSpinner] = useState(false)
  const minTimeElapsedRef = useRef(false)
  const callbackFiredRef = useRef(false)

  // Start spinner animation after brief delay (for smoother appearance)
  useEffect(() => {
    const spinnerTimer = setTimeout(() => setShowSpinner(true), 300)
    return () => clearTimeout(spinnerTimer)
  }, [])

  // Handle minimum display time
  useEffect(() => {
    const timer = setTimeout(() => {
      minTimeElapsedRef.current = true
      if (!callbackFiredRef.current && onMinTimeElapsed) {
        callbackFiredRef.current = true
        onMinTimeElapsed()
      }
    }, minDisplayTime)

    return () => clearTimeout(timer)
  }, [minDisplayTime, onMinTimeElapsed])

  return (
    <div
      className={`splash-screen ${className}`}
      data-testid={testId}
      role="status"
      aria-live="polite"
      aria-label="Loading Arena"
    >
      {/* Animated logo */}
      <div className="splash-logo">
        <ArenaLogo />
      </div>

      {/* Title */}
      <h1 className="splash-title">Arena</h1>

      {/* Subtitle */}
      <p className="splash-subtitle">
        {message || 'Prepare for battle'}
      </p>

      {/* Loading spinner (appears after brief delay) */}
      {showSpinner && (
        <div className="splash-spinner">
          <div className="spinner" aria-hidden="true" />
        </div>
      )}

      {/* Screen reader text */}
      <span className="sr-only">Loading Arena application...</span>
    </div>
  )
}
