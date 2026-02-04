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

/** SVG Replay icon - circular arrow */
const ReplayIcon = () => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 24 24"
    fill="currentColor"
    stroke="none"
  >
    <path d="M12 5V1L7 6l5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z" />
  </svg>
)

/** Speed labels for each index */
const SPEED_LABELS = ['1x', '2x']

export interface PlaybackControlsProps {
  /** Whether playback is currently playing */
  isPlaying: boolean
  /** Callback when play/pause is toggled */
  onPlayPause: () => void
  /** Current speed index (0=1x, 1=2x) */
  speedIndex: number
  /** Callback to cycle to next speed */
  onCycleSpeed: () => void
  /** Whether battle playback has completed */
  isComplete?: boolean
  /** Callback to replay battle from start */
  onReplay?: () => void
}

/**
 * Compact playback controls for the tab bar.
 * Displayed during battle phase only.
 * Shows Replay button when battle is complete.
 */
export function PlaybackControls({
  isPlaying,
  onPlayPause,
  speedIndex,
  onCycleSpeed,
  isComplete,
  onReplay,
}: PlaybackControlsProps) {
  // When battle is complete, show replay button instead of play/pause
  const showReplay = isComplete && onReplay

  return (
    <div className="playback-controls">
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

      {showReplay ? (
        <GameButton
          variant="primary"
          shape="square"
          size="lg"
          className="tab-item-game playback-btn"
          onClick={onReplay}
          aria-label="Replay"
        >
          <span className="tab-icon">
            <ReplayIcon />
          </span>
          <span className="tab-label">Replay</span>
        </GameButton>
      ) : (
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
      )}
    </div>
  )
}

export default PlaybackControls
