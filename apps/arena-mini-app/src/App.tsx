import { useState, useEffect, useCallback } from 'react'
import { useLaunchParams } from '@telegram-apps/sdk-react'
import { apiClient } from './api/client'
import { TabBar, LoadingSpinner, ErrorDisplay, LobbyPage, ShopPage, BattlePage } from './components'
import type { AuthResponse } from '@beef-briefing/shared-mini-app/types'
import type { GamePhase } from './types'

// App states
type AppState = 'loading' | 'authenticated' | 'error'

// Page headers configuration
const PAGE_HEADERS: Record<GamePhase, { title: string; subtitle: string }> = {
  lobby: { title: 'Lobby', subtitle: 'Find or create a match' },
  shop: { title: 'Shop', subtitle: 'Build your team' },
  battle: { title: 'Battle', subtitle: 'Watch the fight' },
  stats: { title: 'Stats', subtitle: 'Your performance' },
}

function App() {
  const [appState, setAppState] = useState<AppState>('loading')
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<GamePhase>('lobby')
  const [authData, setAuthData] = useState<AuthResponse | null>(null)
  // Track active match ID for shop/battle navigation
  const [activeMatchId, setActiveMatchId] = useState<string | null>(null)

  const launchParams = useLaunchParams()

  const initialize = async () => {
    const minSplashTime = new Promise(resolve => setTimeout(resolve, 2000))

    try {
      // Authenticate with the API
      const initData = launchParams.initDataRaw
      if (!initData) {
        throw new Error('No init data available')
      }

      const authResult = await apiClient.authenticate(initData)
      setAuthData(authResult)

      // Wait for minimum splash time
      await minSplashTime

      setAppState('authenticated')
    } catch (err) {
      console.error('Initialization error:', err)
      await minSplashTime
      setError(err instanceof Error ? err.message : 'Failed to initialize')
      setAppState('error')
    }
  }

  useEffect(() => {
    initialize()
  }, [launchParams.initDataRaw])

  // Splash screen during loading
  if (appState === 'loading') {
    return (
      <div className="splash-screen">
        <div className="splash-emoji">⚔️</div>
        <h1 className="splash-title">Arena</h1>
        <p className="splash-subtitle">Card Battle</p>
        <LoadingSpinner size="sm" />
      </div>
    )
  }

  // Error state
  if (appState === 'error') {
    return (
      <ErrorDisplay
        title="Failed to Initialize"
        message={error || 'Something went wrong'}
        onRetry={initialize}
        fullScreen
      />
    )
  }

  const currentPage = PAGE_HEADERS[activeTab]

  // Handle match enter (from LobbyPage auto-navigation)
  const handleMatchEnter = useCallback((matchId: string, _phase: 'shop' | 'battle') => {
    setActiveMatchId(matchId)
  }, [])

  // Render page content based on active tab
  const renderPageContent = () => {
    switch (activeTab) {
      case 'lobby':
        return (
          <LobbyPage
            currentUserId={authData?.user_id}
            onTabChange={setActiveTab}
            onMatchEnter={handleMatchEnter}
          />
        )
      case 'shop':
        return (
          <ShopPage
            matchId={activeMatchId}
            currentUserId={authData?.user_id}
            onTabChange={setActiveTab}
            onBattleStart={(matchId) => {
              setActiveMatchId(matchId)
            }}
          />
        )
      case 'battle':
        return (
          <BattlePage
            matchId={activeMatchId}
            currentUserId={authData?.user_id}
            onTabChange={setActiveTab}
          />
        )
      case 'stats':
        return (
          <div className="empty-state">
            <div className="empty-state-icon">📊</div>
            <h3 className="empty-state-title">Stats Coming Soon</h3>
            <p className="empty-state-text">
              View leaderboards, your profile, and match history.
            </p>
          </div>
        )
      default:
        return null
    }
  }

  // Main app with tab navigation
  return (
    <div className="app">
      <div className="page-container">
        <div className="app-header">
          <h1>{currentPage.title}</h1>
          <p className="app-header-subtitle">{currentPage.subtitle}</p>
        </div>

        {renderPageContent()}
      </div>

      <TabBar activeTab={activeTab} onTabChange={setActiveTab} />
    </div>
  )
}

export default App
