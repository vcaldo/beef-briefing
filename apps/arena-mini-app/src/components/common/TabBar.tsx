import type { TabId } from '../../types'
import { SoundSettings } from './SoundSettings'
import { PlaybackControls } from './PlaybackControls'
import { GameButton } from '../ui/GameButton'

interface Tab {
  id: TabId
  label: string
  icon: React.ReactNode
}

/**
 * SVG Icons for tab bar navigation
 * Using custom SVG icons instead of emojis for better consistency and styling
 */

const LobbyIcon = () => (
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
    {/* Gamepad icon */}
    <rect x="2" y="6" width="20" height="12" rx="2" />
    <path d="M6 12h4" />
    <path d="M8 10v4" />
    <circle cx="17" cy="10" r="1" />
    <circle cx="15" cy="12" r="1" />
  </svg>
)

// Exported for use in LobbyPage "Continue to Shop" button
export const ShopIcon = () => (
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
    {/* Shopping cart icon */}
    <circle cx="9" cy="21" r="1" />
    <circle cx="20" cy="21" r="1" />
    <path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6" />
  </svg>
)

// Exported for potential future use (Battle screen accessible via auto-navigation only)
export const BattleIcon = () => (
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
    {/* Crossed swords icon */}
    <path d="M14.5 17.5 3 6V3h3l11.5 11.5" />
    <path d="m13 19 6-6" />
    <path d="m16 16 4 4" />
    <path d="m19 21 2-2" />
    <path d="M9.5 6.5 21 18v3h-3L6.5 9.5" />
    <path d="m11 5 6 6" />
    <path d="m8 8-4-4" />
    <path d="m5 3-2 2" />
  </svg>
)

const StatsIcon = () => (
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
    {/* Bar chart icon */}
    <line x1="12" y1="20" x2="12" y2="10" />
    <line x1="18" y1="20" x2="18" y2="4" />
    <line x1="6" y1="20" x2="6" y2="14" />
  </svg>
)

/**
 * Tab configuration for arena mini-app navigation
 * 2 tabs: Lobby, Stats
 * Shop and Battle screens are only accessible via automatic transition during active match
 */
const TABS: Tab[] = [
  { id: 'lobby', label: 'Lobby', icon: <LobbyIcon /> },
  { id: 'stats', label: 'Stats', icon: <StatsIcon /> },
]

/** Battle playback state passed from App when in battle tab */
export interface BattlePlaybackProps {
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
  /** Whether user can navigate away (true after battle has played through once) */
  canNavigateAway?: boolean
}

interface TabBarProps {
  /** Currently active tab */
  activeTab: TabId
  /** Callback when user clicks a tab */
  onTabChange: (tab: TabId) => void
  /** Battle playback controls (optional - only shown during battle) */
  battlePlayback?: BattlePlaybackProps
}

/**
 * Bottom navigation bar for arena mini-app.
 * Fixed position at bottom of screen with 2 tabs + optional playback controls + sound settings.
 * Uses CSS classes from global.css for styling.
 */
export function TabBar({ activeTab, onTabChange, battlePlayback }: TabBarProps) {
  const handleTabClick = (tabId: TabId) => {
    onTabChange(tabId)
  }

  // Disable Lobby tab during battle until battle has played through once
  // Use explicit check for `!== true` to safely handle undefined values
  const isLobbyDisabled = Boolean(battlePlayback && battlePlayback.canNavigateAway !== true)

  return (
    <nav className="tab-bar" role="navigation" aria-label="Main navigation">
      {TABS.map((tab) => {
        const isActive = activeTab === tab.id
        const isDisabled = tab.id === 'lobby' && isLobbyDisabled

        return (
          <GameButton
            key={tab.id}
            variant={isActive ? 'primary' : 'neutral'}
            shape="square"
            size="lg"
            className={`tab-item-game ${isActive ? 'active' : ''}`}
            onClick={() => handleTabClick(tab.id)}
            disabled={isDisabled}
            aria-current={isActive ? 'page' : undefined}
            aria-label={tab.label}
          >
            <span className="tab-icon">{tab.icon}</span>
            <span className="tab-label">{tab.label}</span>
          </GameButton>
        )
      })}
      {battlePlayback && (
        <PlaybackControls
          isPlaying={battlePlayback.isPlaying}
          onPlayPause={battlePlayback.onPlayPause}
          speedIndex={battlePlayback.speedIndex}
          onCycleSpeed={battlePlayback.onCycleSpeed}
          isComplete={battlePlayback.isComplete}
          onReplay={battlePlayback.onReplay}
        />
      )}
      <SoundSettings />
    </nav>
  )
}
