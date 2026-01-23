/**
 * SoundSettings - Simple mute/unmute toggle button
 *
 * Provides quick sound toggle in the TabBar.
 * Clicking the speaker icon toggles mute state directly (no popover).
 * Volume is fixed at 25%.
 */

import { useSoundContext } from '../../contexts'
import { GameButton } from '../ui/GameButton'

/**
 * Speaker icon that shows muted or unmuted state
 * - Muted: speaker with X
 * - Unmuted: speaker with 2 sound waves (fixed 25% volume)
 */
const SpeakerIcon = ({ isMuted }: { isMuted: boolean }) => {
  if (isMuted) {
    // Muted - speaker with X
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

  // Unmuted - speaker with two sound waves
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
 * Simple toggle button - click to mute/unmute
 * Styled consistently with tab items (icon + label)
 */
export function SoundSettings() {
  const { isMuted, toggleMute } = useSoundContext()

  return (
    <div className="sound-settings">
      <GameButton
        variant="neutral"
        shape="square"
        size="lg"
        className="sound-settings-game-btn"
        onClick={toggleMute}
        aria-label={isMuted ? 'Sound muted, click to unmute' : 'Sound on, click to mute'}
        aria-pressed={isMuted}
      >
        <span className="tab-icon">
          <SpeakerIcon isMuted={isMuted} />
        </span>
        <span className="tab-label">{isMuted ? 'Muted' : 'Sound'}</span>
      </GameButton>
    </div>
  )
}

export default SoundSettings
