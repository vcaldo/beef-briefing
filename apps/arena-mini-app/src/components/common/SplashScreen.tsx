import React, { useEffect, useRef, useState } from 'react'
import logoUrl from '../../../assets/images/logo/logo.webp'
import splashBgUrl from '../../../assets/images/bg/splash.webp'

/**
 * SplashScreen - Animated splash screen for Arena Mini-App initialization
 *
 * Displays the Beef Arena logo with a pulsating animation over a dark forest
 * background while the app initializes. Enforces a minimum 2-second display
 * time to ensure smooth user experience and prevent jarring transitions.
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

  const backgroundStyle = {
    backgroundImage: `linear-gradient(to bottom, rgba(15, 15, 26, 0.4) 0%, rgba(15, 15, 26, 0.6) 100%), url(${splashBgUrl})`,
  }

  return (
    <div
      className={`splash-screen ${className}`}
      style={backgroundStyle}
      data-testid={testId}
      role="status"
      aria-live="polite"
      aria-label="Loading Beef Arena"
    >
      {/* Animated logo */}
      <img
        src={logoUrl}
        alt="Beef Arena"
        className="splash-logo"
      />

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
      <span className="sr-only">Loading Beef Arena application...</span>
    </div>
  )
}
