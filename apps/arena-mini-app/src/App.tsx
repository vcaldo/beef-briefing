import { useState, useEffect, useCallback, lazy, Suspense } from 'react'
import { useLaunchParams } from '@telegram-apps/sdk-react'
import { apiClient } from './api/client'
import { TabBar, LoadingSpinner, ErrorDisplay } from './components'
import type { AuthResponse } from '@beef-briefing/shared-mini-app/types'
import type { GamePhase } from './types'

// Lazy load page components for code splitting
const LobbyPage = lazy(() => import('./components/lobby/LobbyPage').then(m => ({ default: m.LobbyPage })))
const ShopPage = lazy(() => import('./components/shop/ShopPage').then(m => ({ default: m.ShopPage })))
const BattlePage = lazy(() => import('./components/battle/BattlePage').then(m => ({ default: m.BattlePage })))
const StatsPage = lazy(() => import('./components/stats/StatsPage').then(m => ({ default: m.StatsPage })))

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
          <StatsPage currentUserId={authData?.user_id} isActive={activeTab === 'stats'} />
        )
      default:
        return null
    }
  }

  // Main app with tab navigation
  return (
    <div className="app">
      {/* Skip link for keyboard users */}
      <a href="#main-content" className="skip-link">
        Skip to main content
      </a>

      <main id="main-content" className="page-container" role="main">
        <header className="app-header">
          <h1>{currentPage.title}</h1>
          <p className="app-header-subtitle">{currentPage.subtitle}</p>
        </header>

        <div
          id={`${activeTab}-panel`}
          role="tabpanel"
          aria-labelledby={activeTab}
          tabIndex={0}
        >
          <Suspense fallback={<LoadingSpinner size="lg" message="Loading..." />}>
            {renderPageContent()}
          </Suspense>
        </div>
      </main>

      <TabBar activeTab={activeTab} onTabChange={setActiveTab} />
    </div>
  )
}

export default App
