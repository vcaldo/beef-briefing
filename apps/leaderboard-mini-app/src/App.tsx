import { useState, useEffect } from 'react'
import { useLaunchParams } from '@telegram-apps/sdk-react'

import { apiClient } from './api/client'
import { TabBar } from './components/common/TabBar'
import { HomePage } from './components/home/HomePage'
import { ReactionsPage } from './components/reactions/ReactionsPage'
import { ProfilePage } from './components/profile/ProfilePage'
import { ActivityPage } from './components/activity/ActivityPage'

import type { TabId, Period } from './types'

type AppState = 'loading' | 'authenticated' | 'error'

function App() {
  // App state
  const [appState, setAppState] = useState<AppState>('loading')
  const [error, setError] = useState<string | null>(null)

  // User info (from auth response)
  const [firstName, setFirstName] = useState<string>('')
  const [username, setUsername] = useState<string | null>(null)

  // Navigation state
  const [activeTab, setActiveTab] = useState<TabId>('home')
  const [period, setPeriod] = useState<Period>('30d')

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
        setFirstName(auth.first_name)
        setUsername(auth.username)

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

  // Render page based on active tab
  const renderPage = () => {
    switch (activeTab) {
      case 'home':
        return <HomePage period={period} onPeriodChange={handlePeriodChange} />
      case 'reactions':
        return <ReactionsPage period={period} onPeriodChange={handlePeriodChange} />
      case 'profile':
        return (
          <ProfilePage
            period={period}
            onPeriodChange={handlePeriodChange}
            firstName={firstName}
            username={username}
          />
        )
      case 'activity':
        return <ActivityPage period={period} onPeriodChange={handlePeriodChange} />
      default:
        return <HomePage period={period} onPeriodChange={handlePeriodChange} />
    }
  }

  // Loading state
  if (appState === 'loading') {
    return (
      <div className="app">
        <div className="loading-container">
          <div className="spinner" />
          <p>Loading...</p>
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
      {renderPage()}
      <TabBar activeTab={activeTab} onTabChange={handleTabChange} />
    </div>
  )
}

export default App
