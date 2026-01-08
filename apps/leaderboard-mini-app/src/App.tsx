import { useState, useEffect, useCallback } from 'react'
import { useLaunchParams } from '@telegram-apps/sdk-react'

import { apiClient } from './api/client'
import { TabBar, ErrorBoundary } from './components/common'
import { HomePage } from './components/home/HomePage'
import { LeaderboardPage } from './components/leaderboard/LeaderboardPage'
import { InteractionsPage } from './components/interactions/InteractionsPage'
import { ProfilePage } from './components/profile/ProfilePage'
import { CardPage } from './components/card'

import type { TabId, Period } from './types'

type AppState = 'loading' | 'authenticated' | 'error'

function App() {
  // App state
  const [appState, setAppState] = useState<AppState>('loading')
  const [error, setError] = useState<string | null>(null)

  // User info (from auth response)
  const [userId, setUserId] = useState<number>(0)
  const [firstName, setFirstName] = useState<string>('')
  const [username, setUsername] = useState<string | null>(null)
  const [chatTitle, setChatTitle] = useState<string | null>(null)
  const [isAdmin, setIsAdmin] = useState<boolean>(false)

  // Navigation state
  const [activeTab, setActiveTab] = useState<TabId>('home')
  const [period, setPeriod] = useState<Period>('7d')

  // Get launch params from Telegram
  let launchParams: ReturnType<typeof useLaunchParams> | null = null
  try {
    launchParams = useLaunchParams()
  } catch {
    // Not in Telegram context
  }

  // Authenticate on mount
  useEffect(() => {
    async function authenticate() {
      try {
        const initDataRaw = launchParams?.initDataRaw
        if (!initDataRaw) {
          setError('This app must be opened from Telegram')
          setAppState('error')
          return
        }

        const auth = await apiClient.authenticate(initDataRaw)
        if (!auth.chat_id) {
          setError('Please open this app from a group chat')
          setAppState('error')
          return
        }

        // Store user info
        setUserId(auth.user_id)
        setFirstName(auth.first_name)
        setUsername(auth.username)
        setChatTitle(auth.chat_title)
        setIsAdmin(auth.is_admin)

        // Update document title to group name (shown in Telegram header)
        if (auth.chat_title) {
          document.title = auth.chat_title
        }

        setAppState('authenticated')
      } catch (err) {
        console.error('Authentication failed:', err)
        setError(err instanceof Error ? err.message : 'Authentication failed')
        setAppState('error')
      }
    }

    authenticate()
  }, [launchParams?.initDataRaw])

  // Handle period change
  const handlePeriodChange = (newPeriod: Period) => {
    setPeriod(newPeriod)
  }

  // Handle tab change
  const handleTabChange = (tab: TabId) => {
    setActiveTab(tab)
  }

  // Reset error boundary on tab change
  const handleErrorReset = useCallback(() => {
    // Re-render will happen automatically due to key change
  }, [])

  // Render page based on active tab
  const renderPage = () => {
    switch (activeTab) {
      case 'home':
        return <HomePage period={period} onPeriodChange={handlePeriodChange} chatTitle={chatTitle} />
      case 'leaderboard':
        return <LeaderboardPage period={period} onPeriodChange={handlePeriodChange} chatTitle={chatTitle} />
      case 'interactions':
        return <InteractionsPage period={period} onPeriodChange={handlePeriodChange} chatTitle={chatTitle} />
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
          />
        )
      default:
        return <HomePage period={period} onPeriodChange={handlePeriodChange} chatTitle={chatTitle} />
    }
  }

  // Loading state - Splash Screen
  if (appState === 'loading') {
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
      <TabBar activeTab={activeTab} onTabChange={handleTabChange} />
    </div>
  )
}

export default App
