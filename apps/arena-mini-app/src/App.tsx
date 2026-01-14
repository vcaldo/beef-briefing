import { useState, useEffect } from 'react'
import { useLaunchParams } from '@telegram-apps/sdk-react'
import { apiClient } from './api/client'
import type { AuthResponse } from '@beef-briefing/shared-mini-app/types'

// Game phases
type GamePhase = 'lobby' | 'shop' | 'battle' | 'stats'

// App states
type AppState = 'loading' | 'authenticated' | 'error'

function App() {
  const [appState, setAppState] = useState<AppState>('loading')
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<GamePhase>('lobby')
  const [_authData, setAuthData] = useState<AuthResponse | null>(null)

  const launchParams = useLaunchParams()

  useEffect(() => {
    async function initialize() {
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

    initialize()
  }, [launchParams.initDataRaw])

  // Splash screen during loading
  if (appState === 'loading') {
    return (
      <div className="splash-screen">
        <div className="splash-emoji">&#x2694;&#xFE0F;</div>
        <h1 className="splash-title">Arena</h1>
        <p className="splash-subtitle">Card Battle</p>
      </div>
    )
  }

  // Error state
  if (appState === 'error') {
    return (
      <div className="error-container">
        <div className="error-content">
          <div className="error-icon">!</div>
          <p>{error || 'Something went wrong'}</p>
        </div>
      </div>
    )
  }

  // Main app with tab navigation
  return (
    <div className="app">
      <div className="page-container">
        {activeTab === 'lobby' && (
          <div className="app-header">
            <h1>Lobby</h1>
            <p className="app-header-subtitle">Find or create a match</p>
          </div>
        )}
        {activeTab === 'shop' && (
          <div className="app-header">
            <h1>Shop</h1>
            <p className="app-header-subtitle">Build your team</p>
          </div>
        )}
        {activeTab === 'battle' && (
          <div className="app-header">
            <h1>Battle</h1>
            <p className="app-header-subtitle">Watch the fight</p>
          </div>
        )}
        {activeTab === 'stats' && (
          <div className="app-header">
            <h1>Stats</h1>
            <p className="app-header-subtitle">Your performance</p>
          </div>
        )}
      </div>

      {/* Tab Bar */}
      <nav className="tab-bar">
        <button
          className={`tab-item ${activeTab === 'lobby' ? 'active' : ''}`}
          onClick={() => setActiveTab('lobby')}
        >
          <span className="tab-icon">
            <span className="tab-emoji">&#x1F3E0;</span>
          </span>
          <span className="tab-label">Lobby</span>
        </button>
        <button
          className={`tab-item ${activeTab === 'shop' ? 'active' : ''}`}
          onClick={() => setActiveTab('shop')}
        >
          <span className="tab-icon">
            <span className="tab-emoji">&#x1F6D2;</span>
          </span>
          <span className="tab-label">Shop</span>
        </button>
        <button
          className={`tab-item ${activeTab === 'battle' ? 'active' : ''}`}
          onClick={() => setActiveTab('battle')}
        >
          <span className="tab-icon">
            <span className="tab-emoji">&#x2694;&#xFE0F;</span>
          </span>
          <span className="tab-label">Battle</span>
        </button>
        <button
          className={`tab-item ${activeTab === 'stats' ? 'active' : ''}`}
          onClick={() => setActiveTab('stats')}
        >
          <span className="tab-icon">
            <span className="tab-emoji">&#x1F4CA;</span>
          </span>
          <span className="tab-label">Stats</span>
        </button>
      </nav>
    </div>
  )
}

export default App
