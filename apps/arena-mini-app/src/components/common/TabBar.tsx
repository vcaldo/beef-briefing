import type { GamePhase } from '../../types'

interface Tab {
  id: GamePhase
  label: string
  icon: React.ReactNode
}

const LobbyIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
    <polyline points="9 22 9 12 15 12 15 22" />
  </svg>
)

const ShopIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="9" cy="21" r="1" />
    <circle cx="20" cy="21" r="1" />
    <path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6" />
  </svg>
)

const BattleIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M14.5 17.5L3 6V3h3l11.5 11.5" />
    <path d="M13 19l6-6" />
    <path d="M16 16l4 4" />
    <path d="M19 21l2-2" />
    <path d="M14.5 6.5L18 3h3v3l-3.5 3.5" />
    <path d="M5 14l4 4" />
    <path d="M7 17l-3 3" />
  </svg>
)

const StatsIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <line x1="18" y1="20" x2="18" y2="10" />
    <line x1="12" y1="20" x2="12" y2="4" />
    <line x1="6" y1="20" x2="6" y2="14" />
  </svg>
)

const TABS: Tab[] = [
  { id: 'lobby', label: 'Lobby', icon: <LobbyIcon /> },
  { id: 'shop', label: 'Shop', icon: <ShopIcon /> },
  { id: 'battle', label: 'Battle', icon: <BattleIcon /> },
  { id: 'stats', label: 'Stats', icon: <StatsIcon /> },
]

interface TabBarProps {
  activeTab: GamePhase
  onTabChange: (tab: GamePhase) => void
  disabledTabs?: GamePhase[]
}

export function TabBar({ activeTab, onTabChange, disabledTabs = [] }: TabBarProps) {
  return (
    <nav className="tab-bar">
      {TABS.map((tab) => {
        const isDisabled = disabledTabs.includes(tab.id)
        return (
          <button
            key={tab.id}
            className={`tab-item ${activeTab === tab.id ? 'active' : ''} ${isDisabled ? 'disabled' : ''}`}
            onClick={() => !isDisabled && onTabChange(tab.id)}
            aria-current={activeTab === tab.id ? 'page' : undefined}
            disabled={isDisabled}
          >
            <span className="tab-icon">{tab.icon}</span>
            <span className="tab-label">{tab.label}</span>
          </button>
        )
      })}
    </nav>
  )
}
