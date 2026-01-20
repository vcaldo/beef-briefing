/**
 * SoundSettings - Mute toggle button with volume slider popover
 *
 * Provides quick access to sound controls in the TabBar.
 * Clicking the speaker icon opens a popover with a volume slider.
 */

import { useState, useRef, useEffect, useCallback } from 'react'
import { useSoundContext } from '../../contexts'

/**
 * Speaker icon that changes based on mute/volume state
 */
const SpeakerIcon = ({ isMuted, volume }: { isMuted: boolean; volume: number }) => {
  // Determine which icon to show based on state
  if (isMuted || volume === 0) {
    // Muted / Volume off - speaker with X
    return (
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
        <line x1="23" y1="9" x2="17" y2="15" />
        <line x1="17" y1="9" x2="23" y2="15" />
      </svg>
    )
  }

  if (volume < 0.5) {
    // Low volume - speaker with one wave
    return (
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
        <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
      </svg>
    )
  }

  // High volume - speaker with two waves
  return (
    <svg
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
      <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
      <path d="M19.07 4.93a10 10 0 0 1 0 14.14" />
    </svg>
  )
}

/**
 * Sound settings component for TabBar
 * Shows a speaker icon that toggles a volume slider popover
 */
export function SoundSettings() {
  const { volume, setVolume, isMuted, toggleMute } = useSoundContext()
  const [isOpen, setIsOpen] = useState(false)
  const popoverRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  // Close popover when clicking outside
  useEffect(() => {
    if (!isOpen) return

    const handleClickOutside = (event: MouseEvent) => {
      if (
        popoverRef.current &&
        !popoverRef.current.contains(event.target as Node) &&
        buttonRef.current &&
        !buttonRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [isOpen])

  // Close popover on escape key
  useEffect(() => {
    if (!isOpen) return

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false)
        buttonRef.current?.focus()
      }
    }

    document.addEventListener('keydown', handleEscape)
    return () => document.removeEventListener('keydown', handleEscape)
  }, [isOpen])

  const handleVolumeChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const newVolume = parseFloat(event.target.value)
      setVolume(newVolume)
    },
    [setVolume]
  )

  const handleMuteClick = useCallback(() => {
    toggleMute()
  }, [toggleMute])

  const handleTogglePopover = useCallback(() => {
    setIsOpen((prev) => !prev)
  }, [])

  return (
    <div className="sound-settings">
      <button
        ref={buttonRef}
        className="sound-settings-btn"
        onClick={handleTogglePopover}
        aria-label={isMuted ? 'Sound muted, click to open settings' : 'Sound on, click to open settings'}
        aria-expanded={isOpen}
        aria-haspopup="true"
      >
        <SpeakerIcon isMuted={isMuted} volume={volume} />
      </button>

      {isOpen && (
        <div ref={popoverRef} className="sound-settings-popover" role="dialog" aria-label="Sound settings">
          <div className="sound-settings-header">
            <span className="sound-settings-title">Sound</span>
            <button
              className={`sound-mute-btn ${isMuted ? 'muted' : ''}`}
              onClick={handleMuteClick}
              aria-label={isMuted ? 'Unmute sound' : 'Mute sound'}
              aria-pressed={isMuted}
            >
              {isMuted ? 'OFF' : 'ON'}
            </button>
          </div>

          <div className="sound-volume-control">
            <label htmlFor="volume-slider" className="visually-hidden">
              Volume
            </label>
            <input
              id="volume-slider"
              type="range"
              min="0"
              max="1"
              step="0.05"
              value={volume}
              onChange={handleVolumeChange}
              className="sound-volume-slider"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={Math.round(volume * 100)}
              aria-valuetext={`${Math.round(volume * 100)}%`}
              disabled={isMuted}
            />
            <span className="sound-volume-value">{Math.round(volume * 100)}%</span>
          </div>
        </div>
      )}
    </div>
  )
}

export default SoundSettings
