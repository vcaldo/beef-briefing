import { useState, useEffect, useCallback, lazy, Suspense } from 'react'
import { useLaunchParams } from '@telegram-apps/sdk-react'

import { apiClient } from './api/client'
import { setCustomAttribute, addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { ErrorBoundary } from '@beef-briefing/shared-mini-app/components'
import { TabBar, SplashScreen, ErrorDisplay, LoadingSpinner } from './components/common'

import type { TabId, AppState, GameConstants, Match } from './types'

// Lazy load page components for code splitting
// Each page will be loaded in a separate chunk when first accessed
const LobbyPage = lazy(() => import('./components/lobby/LobbyPage'))
const ShopPage = lazy(() => import('./components/shop/ShopPage'))
const BattlePage = lazy(() => import('./components/battle/BattlePage'))
const StatsPage = lazy(() => import('./components/stats/StatsPage'))

function App() {
  // App state
  const [appState, setAppState] = useState<AppState>('loading')
  const [error, setError] = useState<string | null>(null)
  const [splashMinTimeElapsed, setSplashMinTimeElapsed] = useState(false)

  // Navigation state
  const [activeTab, setActiveTab] = useState<TabId>('lobby')

  // User state
  const [userId, setUserId] = useState<number>(0)
  const [chatId, setChatId] = useState<number>(0)
  const [firstName, setFirstName] = useState<string>('')

  // Game state
  const [activeMatch, setActiveMatch] = useState<Match | null>(null)
  const [gameConstants, setGameConstants] = useState<GameConstants | null>(null)

  // Get launch params from Telegram
  let launchParams: ReturnType<typeof useLaunchParams> | null = null
  try {
    launchParams = useLaunchParams()
  } catch {
    // Not in Telegram context
  }

  // Set timezone attribute on mount
  useEffect(() => {
    try {
      const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
      setCustomAttribute('timezone', timezone)
    } catch {
      setCustomAttribute('timezone', 'unknown')
    }
  }, [])

  // Handle splash screen minimum time elapsed
  const handleSplashMinTimeElapsed = useCallback(() => {
    setSplashMinTimeElapsed(true)
  }, [])

  // Prefetch game constants after auth
  const prefetchGameConstants = useCallback(async () => {
    try {
      const constants = await apiClient.getConstants()
      setGameConstants(constants)
    } catch (err) {
      console.error('Failed to prefetch game constants:', err)
      // Non-fatal error, continue without constants
    }
  }, [])

  // Authenticate on mount
  useEffect(() => {
    async function authenticate() {
      try {
        const initDataRaw = launchParams?.initDataRaw
        if (!initDataRaw) {
          // Development fallback
          if (import.meta.env.DEV) {
            setError('No init data available. Running in development mode without Telegram context.')
          } else {
            setError('This app must be opened from Telegram')
          }
          setAppState('error')
          addPageAction('auth_failed', { reason: 'no_init_data' })
          return
        }

        const auth = await apiClient.authenticate(initDataRaw)

        if (!auth.chat_id) {
          setError('Please open this app from a group chat using the /arena command')
          setAppState('error')
          addPageAction('auth_failed', { reason: 'no_chat_id' })
          return
        }

        // Store user info for page components
        setUserId(auth.user_id)
        setChatId(auth.chat_id)
        setFirstName(auth.first_name)

        // Set New Relic custom attributes for this session
        setCustomAttribute('user_id', auth.user_id)
        setCustomAttribute('chat_id', auth.chat_id)
        setCustomAttribute('username', auth.username || '')
        setCustomAttribute('first_name', auth.first_name)
        setCustomAttribute('is_authenticated', true)
        setCustomAttribute('app', 'arena')

        addPageAction('auth_success', {
          user_id: auth.user_id,
          chat_id: auth.chat_id,
        })

        setAppState('authenticated')

        // Prefetch game constants while splash screen is still showing
        prefetchGameConstants()
      } catch (err) {
        console.error('Authentication failed:', err)
        setError(err instanceof Error ? err.message : 'Authentication failed')
        setAppState('error')

        if (err instanceof Error) {
          noticeError(err, { context: 'authentication' })
        }
        addPageAction('auth_error', {
          error: err instanceof Error ? err.message : 'unknown',
        })
      }
    }

    authenticate()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [launchParams?.initDataRaw])

  // Handle tab change with tracking
  const handleTabChange = useCallback(
    (tab: TabId) => {
      addPageAction('tab_change', {
        tab,
        previous_tab: activeTab,
      })
      setCustomAttribute('active_tab', tab)
      setActiveTab(tab)
    },
    [activeTab]
  )

  // Reset error boundary on tab change
  const handleErrorReset = useCallback(() => {
    // Re-render will happen automatically due to key change
  }, [])

  // Handle match change with tracking
  const handleMatchChange = useCallback((match: Match | null) => {
    if (match) {
      addPageAction('match_changed', { match_id: match.id, status: match.status })
    } else {
      addPageAction('match_cleared', {})
    }
    setActiveMatch(match)
  }, [])

  // Navigate to lobby tab
  const navigateToLobby = useCallback(() => {
    handleTabChange('lobby')
  }, [handleTabChange])

  // Navigate to shop tab
  const navigateToShop = useCallback(() => {
    handleTabChange('shop')
  }, [handleTabChange])

  // Navigate to battle tab
  const navigateToBattle = useCallback(() => {
    handleTabChange('battle')
  }, [handleTabChange])

  // Render page based on active tab
  const renderPage = () => {
    switch (activeTab) {
      case 'lobby':
        return (
          <LobbyPage
            userId={userId}
            chatId={chatId}
            firstName={firstName}
            activeMatch={activeMatch}
            onMatchChange={handleMatchChange}
            onNavigateToShop={navigateToShop}
            onNavigateToBattle={navigateToBattle}
          />
        )
      case 'shop':
        return (
          <ShopPage
            userId={userId}
            chatId={chatId}
            activeMatch={activeMatch}
            onMatchChange={handleMatchChange}
            onNavigateToBattle={navigateToBattle}
            gameConstants={gameConstants}
          />
        )
      case 'battle':
        return (
          <BattlePage
            userId={userId}
            activeMatch={activeMatch}
            gameConstants={gameConstants}
            onMatchChange={handleMatchChange}
            onNavigateToStats={() => handleTabChange('stats')}
            onNavigateToLobby={navigateToLobby}
          />
        )
      case 'stats':
        return (
          <StatsPage
            chatId={chatId}
            userId={userId}
          />
        )
      default:
        return (
          <LobbyPage
            userId={userId}
            chatId={chatId}
            firstName={firstName}
            activeMatch={activeMatch}
            onMatchChange={handleMatchChange}
            onNavigateToShop={navigateToShop}
            onNavigateToBattle={navigateToBattle}
          />
        )
    }
  }

  // Splash Screen - show while loading OR until minimum display time elapsed
  if (appState === 'loading' || (appState === 'authenticated' && !splashMinTimeElapsed)) {
    return (
      <div className="app">
        <SplashScreen
          message="Prepare for battle"
          onMinTimeElapsed={handleSplashMinTimeElapsed}
          minDisplayTime={2000}
        />
      </div>
    )
  }

  // Error state
  if (appState === 'error') {
    return (
      <div className="app">
        <ErrorDisplay
          title="Unable to Start"
          message={error || 'An unknown error occurred'}
          onRetry={() => window.location.reload()}
          retryText="Reload"
        />
      </div>
    )
  }

  // Main app with tab navigation
  return (
    <div className="app">
      {/* Skip link for keyboard users to bypass navigation */}
      <a href="#main-content" className="skip-link">
        Skip to main content
      </a>
      <main id="main-content" role="main" aria-label={`${activeTab} page`}>
        <ErrorBoundary key={activeTab} onReset={handleErrorReset} name={`arena-${activeTab}`}>
          <Suspense fallback={<LoadingSpinner message="Loading..." />}>
            {renderPage()}
          </Suspense>
        </ErrorBoundary>
      </main>
      <TabBar activeTab={activeTab} onTabChange={handleTabChange} />
    </div>
  )
}

export default App
