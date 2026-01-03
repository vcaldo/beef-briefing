import { useState, useEffect, useCallback } from 'react'
import { useLaunchParams } from '@telegram-apps/sdk-react'

import { apiClient } from './api/client'
import { PeriodSelector } from './components/PeriodSelector'
import { OverviewStats } from './components/OverviewStats'
import { ActivityChart } from './components/ActivityChart'
import { LeaderboardTable } from './components/LeaderboardTable'

import type {
  Period,
  LeaderboardMetric,
  StatsResponse,
  ActivityDataPoint,
  LeaderboardUser,
} from './types'

type AppState = 'loading' | 'authenticated' | 'error'

function App() {
  // App state
  const [appState, setAppState] = useState<AppState>('loading')
  const [error, setError] = useState<string | null>(null)

  // Data state
  const [period, setPeriod] = useState<Period>('30d')
  const [stats, setStats] = useState<StatsResponse | null>(null)
  const [activity, setActivity] = useState<ActivityDataPoint[]>([])
  const [leaderboard, setLeaderboard] = useState<LeaderboardUser[]>([])
  const [leaderboardTotal, setLeaderboardTotal] = useState(0)
  const [leaderboardPage, setLeaderboardPage] = useState(1)
  const [leaderboardMetric, setLeaderboardMetric] = useState<LeaderboardMetric>('message_count')

  // Loading states
  const [loadingStats, setLoadingStats] = useState(false)
  const [loadingActivity, setLoadingActivity] = useState(false)
  const [loadingLeaderboard, setLoadingLeaderboard] = useState(false)

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

        setAppState('authenticated')
      } catch (err) {
        console.error('Authentication failed:', err)
        setError(err instanceof Error ? err.message : 'Authentication failed')
        setAppState('error')
      }
    }

    authenticate()
  }, [launchParams?.initDataRaw])

  // Fetch all data when period changes
  const fetchData = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return

    // Fetch stats
    setLoadingStats(true)
    try {
      const statsData = await apiClient.getStats(period)
      setStats(statsData)
    } catch (err) {
      console.error('Failed to fetch stats:', err)
    } finally {
      setLoadingStats(false)
    }

    // Fetch activity
    setLoadingActivity(true)
    try {
      const activityData = await apiClient.getActivity(period)
      setActivity(activityData.data)
    } catch (err) {
      console.error('Failed to fetch activity:', err)
    } finally {
      setLoadingActivity(false)
    }

    // Fetch leaderboard (reset to page 1 when period changes)
    setLeaderboardPage(1)
    fetchLeaderboard(1)
  }, [period])

  // Fetch leaderboard (separate function for pagination)
  const fetchLeaderboard = useCallback(
    async (page: number) => {
      if (!apiClient.isAuthenticated()) return

      setLoadingLeaderboard(true)
      try {
        const data = await apiClient.getLeaderboard(period, leaderboardMetric, page, 20)
        setLeaderboard(data.users)
        setLeaderboardTotal(data.total)
        setLeaderboardPage(page)
      } catch (err) {
        console.error('Failed to fetch leaderboard:', err)
      } finally {
        setLoadingLeaderboard(false)
      }
    },
    [period, leaderboardMetric]
  )

  // Fetch data when authenticated
  useEffect(() => {
    if (appState === 'authenticated') {
      fetchData()
    }
  }, [appState, fetchData])

  // Refetch leaderboard when metric changes
  useEffect(() => {
    if (appState === 'authenticated') {
      setLeaderboardPage(1)
      fetchLeaderboard(1)
    }
  }, [leaderboardMetric, appState])

  // Handle period change
  const handlePeriodChange = (newPeriod: Period) => {
    setPeriod(newPeriod)
  }

  // Handle metric change
  const handleMetricChange = (newMetric: LeaderboardMetric) => {
    setLeaderboardMetric(newMetric)
  }

  // Handle page change
  const handlePageChange = (newPage: number) => {
    fetchLeaderboard(newPage)
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

  // Main app
  return (
    <div className="app">
      <header className="app-header">
        <h1>Leaderboard</h1>
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={handlePeriodChange} />

      <OverviewStats stats={stats} loading={loadingStats} />

      <ActivityChart data={activity} loading={loadingActivity} />

      <LeaderboardTable
        users={leaderboard}
        total={leaderboardTotal}
        page={leaderboardPage}
        limit={20}
        metric={leaderboardMetric}
        loading={loadingLeaderboard}
        onMetricChange={handleMetricChange}
        onPageChange={handlePageChange}
      />
    </div>
  )
}

export default App
