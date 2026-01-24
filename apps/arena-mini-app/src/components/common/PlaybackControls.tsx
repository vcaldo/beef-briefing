/**
 * Playback Controls Component
 *
 * Compact playback controls for the tab bar during battle:
 * - Play/Pause button with SVG icons
 * - Speed button cycling 1x → 1.5x → 2x
 */

import { GameButton } from '../ui/GameButton'

/** SVG Play icon - triangle pointing right */
const PlayIcon = () => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 24 24"
    fill="currentColor"
    stroke="none"
  >
    <polygon points="5,3 19,12 5,21" />
  </svg>
)

/** SVG Pause icon - two vertical bars */
const PauseIcon = () => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 24 24"
    fill="currentColor"
    stroke="none"
  >
    <rect x="5" y="4" width="4" height="16" rx="1" />
    <rect x="15" y="4" width="4" height="16" rx="1" />
  </svg>
)

/** Speed labels for each index */
const SPEED_LABELS = ['1x', '1.5x', '2x']

export interface PlaybackControlsProps {
  /** Whether playback is currently playing */
  isPlaying: boolean
  /** Callback when play/pause is toggled */
  onPlayPause: () => void
  /** Current speed index (0=1x, 1=1.5x, 2=2x) */
  speedIndex: number
  /** Callback to cycle to next speed */
  onCycleSpeed: () => void
}

/**
 * Compact playback controls for the tab bar.
 * Displayed during battle phase only.
 */
export function PlaybackControls({
  isPlaying,
  onPlayPause,
  speedIndex,
  onCycleSpeed,
}: PlaybackControlsProps) {
  return (
    <div className="playback-controls">
      <GameButton
        variant={isPlaying ? 'secondary' : 'primary'}
        shape="square"
        size="lg"
        className="tab-item-game playback-btn"
        onClick={onPlayPause}
        aria-label={isPlaying ? 'Pause' : 'Play'}
      >
        <span className="tab-icon">
          {isPlaying ? <PauseIcon /> : <PlayIcon />}
        </span>
        <span className="tab-label">{isPlaying ? 'Pause' : 'Play'}</span>
      </GameButton>

      <GameButton
        variant="neutral"
        shape="square"
        size="lg"
        className="tab-item-game playback-btn"
        onClick={onCycleSpeed}
        aria-label={`Speed: ${SPEED_LABELS[speedIndex]}`}
      >
        <span className="tab-icon speed-icon">{SPEED_LABELS[speedIndex]}</span>
        <span className="tab-label">Speed</span>
      </GameButton>
    </div>
  )
}

export default PlaybackControls
