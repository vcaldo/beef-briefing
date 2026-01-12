import { useState, useEffect, useCallback } from 'react'
import { useLaunchParams } from '@telegram-apps/sdk-react'

import { apiClient } from './api/client'
import { setCustomAttribute, addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { TabBar, ErrorBoundary } from './components/common'
import { HomePage } from './components/home/HomePage'
import { LeaderboardPage } from './components/leaderboard/LeaderboardPage'
import { InteractionsPage } from './components/interactions/InteractionsPage'
import { MediaPage } from './components/media'
import { ProfilePage } from './components/profile/ProfilePage'
import { CardPage } from './components/card'
import { AdminPage } from './components/admin'

import type {
  TabId,
  Period,
  StatsResponse,
  ActivityDataPoint,
  HeatmapData,
  LeaderboardResponse,
  ReactionsOverviewResponse,
  RepliesOverviewResponse,
  ProfileResponse,
  CardImageWithUrl,
  MediaOverviewResponse,
} from './types'

// Prefetched data for all tabs (loaded during splash screen)
interface PrefetchedData {
  home: {
    stats: StatsResponse | null
    activity: ActivityDataPoint[]
    heatmap: HeatmapData | null
  } | null
  leaderboard: LeaderboardResponse | null
  interactions: {
    reactions: ReactionsOverviewResponse | null
    replies: RepliesOverviewResponse | null
  } | null
  media: MediaOverviewResponse | null
  profile: ProfileResponse | null
  card: CardImageWithUrl | null
  period: Period
}

type AppState = 'loading' | 'authenticated' | 'error'

function App() {
  // App state
  const [appState, setAppState] = useState<AppState>('loading')
  const [error, setError] = useState<string | null>(null)
  const [splashMinTimeElapsed, setSplashMinTimeElapsed] = useState(false)

  // User info (from auth response)
  const [userId, setUserId] = useState<number>(0)
  const [firstName, setFirstName] = useState<string>('')
  const [username, setUsername] = useState<string | null>(null)
  const [chatTitle, setChatTitle] = useState<string | null>(null)
  const [isAdmin, setIsAdmin] = useState<boolean>(false)

  // Navigation state
  const [activeTab, setActiveTab] = useState<TabId>('home')
  const [period, setPeriod] = useState<Period>('7d')

  // Admin impersonation state (persists across tab changes)
  const [impersonatedUserId, setImpersonatedUserId] = useState<number | null>(null)

  // Prefetched data for all tabs (loaded during splash screen)
  const [prefetched, setPrefetched] = useState<PrefetchedData | null>(null)

  // Get launch params from Telegram
  let launchParams: ReturnType<typeof useLaunchParams> | null = null
  try {
    launchParams = useLaunchParams()
  } catch {
    // Not in Telegram context
  }

  // Set timezone attribute on mount
  useEffect(() => {
    setCustomAttribute('timezone', apiClient.getTimezone())
  }, [])

  // Minimum splash screen display time (2s)
  useEffect(() => {
    const timer = setTimeout(() => setSplashMinTimeElapsed(true), 2000)
    return () => clearTimeout(timer)
  }, [])

  // Prefetch all tab data in parallel (called after auth, while splash still shows)
  const prefetchAllData = useCallback(async () => {
    const defaultPeriod: Period = '7d'

    // Fire all requests in parallel using Promise.allSettled (failures don't block others)
    // Note: Card prefetch disabled - it was causing issues
    const [home, leaderboard, interactions, media, profile] = await Promise.allSettled([
      // Home page data
      Promise.all([
        apiClient.getStats(defaultPeriod),
        apiClient.getActivity(defaultPeriod),
        apiClient.getHeatmap(defaultPeriod, false),
      ]).then(([stats, activity, heatmap]) => ({
        stats,
        activity: activity.data || [],
        heatmap: heatmap.group,
      })),

      // Leaderboard (first page, default metric)
      apiClient.getLeaderboard(defaultPeriod, 'message_count', 1, 20),

      // Interactions (reactions + replies in parallel)
      Promise.all([
        apiClient.getReactionsOverview(defaultPeriod, 10),
        apiClient.getRepliesOverview(defaultPeriod, 10),
      ]).then(([reactions, replies]) => ({ reactions, replies })),

      // Media
      apiClient.getMediaOverview(defaultPeriod, 10),

      // Profile
      apiClient.getProfile(defaultPeriod),
    ])

    setPrefetched({
      home: home.status === 'fulfilled' ? home.value : null,
      leaderboard: leaderboard.status === 'fulfilled' ? leaderboard.value : null,
      interactions: interactions.status === 'fulfilled' ? interactions.value : null,
      media: media.status === 'fulfilled' ? media.value : null,
      profile: profile.status === 'fulfilled' ? profile.value : null,
      card: null, // Card prefetch disabled
      period: defaultPeriod,
    })
  }, [])

  // Authenticate on mount
  useEffect(() => {
    async function authenticate() {
      try {
        const initDataRaw = launchParams?.initDataRaw
        if (!initDataRaw) {
          setError('This app must be opened from Telegram')
          setAppState('error')
          addPageAction('auth_failed', { reason: 'no_init_data' })
          return
        }

        const auth = await apiClient.authenticate(initDataRaw)
        if (!auth.chat_id) {
          setError('Please open this app from a group chat')
          setAppState('error')
          addPageAction('auth_failed', { reason: 'no_chat_id' })
          return
        }

        // Store user info
        setUserId(auth.user_id)
        setFirstName(auth.first_name)
        setUsername(auth.username)
        setChatTitle(auth.chat_title ?? null)
        setIsAdmin(auth.is_admin ?? false)

        // Set New Relic custom attributes for this session
        setCustomAttribute('user_id', auth.user_id)
        setCustomAttribute('chat_id', auth.chat_id)
        setCustomAttribute('username', auth.username || '')
        setCustomAttribute('first_name', auth.first_name)
        setCustomAttribute('is_admin', auth.is_admin ?? false)
        setCustomAttribute('is_authenticated', true)

        addPageAction('auth_success', {
          user_id: auth.user_id,
          chat_id: auth.chat_id,
          is_admin: auth.is_admin,
        })

        // Update document title to group name (shown in Telegram header)
        if (auth.chat_title) {
          document.title = auth.chat_title
        }

        setAppState('authenticated')

        // Start prefetching all tab data while splash screen is still showing
        prefetchAllData()
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

  // Handle period change with tracking
  const handlePeriodChange = useCallback(
    (newPeriod: Period) => {
      addPageAction('period_change', {
        period: newPeriod,
        previous_period: period,
        tab: activeTab,
      })
      setCustomAttribute('selected_period', newPeriod)
      setPeriod(newPeriod)
    },
    [period, activeTab]
  )

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

  // Render page based on active tab
  const renderPage = () => {
    switch (activeTab) {
      case 'home':
        return (
          <HomePage
            period={period}
            onPeriodChange={handlePeriodChange}
            chatTitle={chatTitle}
            prefetchedData={prefetched?.period === period ? prefetched.home : null}
            onPrefetchConsumed={() => setPrefetched(prev => prev ? { ...prev, home: null } : null)}
          />
        )
      case 'leaderboard':
        return (
          <LeaderboardPage
            period={period}
            onPeriodChange={handlePeriodChange}
            chatTitle={chatTitle}
            prefetchedData={prefetched?.period === period ? prefetched.leaderboard : null}
            onPrefetchConsumed={() => setPrefetched(prev => prev ? { ...prev, leaderboard: null } : null)}
          />
        )
      case 'interactions':
        return (
          <InteractionsPage
            period={period}
            onPeriodChange={handlePeriodChange}
            chatTitle={chatTitle}
            prefetchedData={prefetched?.period === period ? prefetched.interactions : null}
            onPrefetchConsumed={() => setPrefetched(prev => prev ? { ...prev, interactions: null } : null)}
          />
        )
      case 'media':
        return (
          <MediaPage
            period={period}
            onPeriodChange={handlePeriodChange}
            chatTitle={chatTitle}
            prefetchedData={prefetched?.period === period ? prefetched.media : null}
            onPrefetchConsumed={() => setPrefetched(prev => prev ? { ...prev, media: null } : null)}
          />
        )
      case 'card':
        return <CardPage userId={userId} chatTitle={chatTitle} />
      case 'profile':
        return (
          <ProfilePage
            period={period}
            onPeriodChange={handlePeriodChange}
            firstName={firstName}
            username={username}
            chatTitle={chatTitle}
            isAdmin={isAdmin}
            prefetchedData={prefetched?.period === period ? prefetched.profile : null}
            onPrefetchConsumed={() => setPrefetched(prev => prev ? { ...prev, profile: null } : null)}
            onTabChange={handleTabChange}
            impersonatedUserId={impersonatedUserId}
          />
        )
      case 'admin':
        return (
          <AdminPage
            chatTitle={chatTitle}
            onTabChange={handleTabChange}
            onTimezoneChange={() => {
              // Invalidate prefetched data since timezone changed
              setPrefetched(null)
            }}
            onImpersonate={setImpersonatedUserId}
            impersonatedUserId={impersonatedUserId}
          />
        )
      default:
        return (
          <HomePage
            period={period}
            onPeriodChange={handlePeriodChange}
            chatTitle={chatTitle}
            prefetchedData={prefetched?.period === period ? prefetched.home : null}
            onPrefetchConsumed={() => setPrefetched(prev => prev ? { ...prev, home: null } : null)}
          />
        )
    }
  }

  // Splash Screen - show while loading OR until minimum display time elapsed
  if (appState === 'loading' || (appState === 'authenticated' && !splashMinTimeElapsed)) {
    return (
      <div className="app">
        <div className="splash-screen">
          <div className="splash-emoji">🥩</div>
          <h1 className="splash-title">Beef Briefing</h1>
          <p className="splash-subtitle">Leaderboard</p>
        </div>
      </div>
    )
  }

  // Error state
  if (appState === 'error') {
    return (
      <div className="app">
        <div className="error-container">
          <div className="error-content">
            <div className="error-icon">!</div>
            <p>{error}</p>
          </div>
        </div>
      </div>
    )
  }

  // Main app with tab navigation
  return (
    <div className="app">
      <ErrorBoundary key={activeTab} onReset={handleErrorReset}>
        {renderPage()}
      </ErrorBoundary>
      {/* Hide tab bar on admin page (accessed via profile, not tab) */}
      {activeTab !== 'admin' && (
        <TabBar activeTab={activeTab} onTabChange={handleTabChange} />
      )}
    </div>
  )
}

export default App
