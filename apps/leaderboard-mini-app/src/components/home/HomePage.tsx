import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { PeriodSelector } from '../PeriodSelector'
import { OverviewStats } from '../OverviewStats'
import { ActivityChart } from '../ActivityChart'
import { LeaderboardTable } from '../LeaderboardTable'

import type {
  Period,
  LeaderboardMetric,
  StatsResponse,
  ActivityDataPoint,
  LeaderboardUser,
} from '../../types'

interface HomePageProps {
  period: Period
  onPeriodChange: (period: Period) => void
}

export function HomePage({ period, onPeriodChange }: HomePageProps) {
  // Data state
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
  }, [period, fetchLeaderboard])

  // Fetch data on mount and when period changes
  useEffect(() => {
    fetchData()
  }, [fetchData])

  // Refetch leaderboard when metric changes
  useEffect(() => {
    setLeaderboardPage(1)
    fetchLeaderboard(1)
  }, [leaderboardMetric, fetchLeaderboard])

  // Handle metric change
  const handleMetricChange = (newMetric: LeaderboardMetric) => {
    setLeaderboardMetric(newMetric)
  }

  // Handle page change
  const handlePageChange = (newPage: number) => {
    fetchLeaderboard(newPage)
  }

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Leaderboard</h1>
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

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
