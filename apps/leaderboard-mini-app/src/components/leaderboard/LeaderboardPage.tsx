import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { PeriodSelector } from '../PeriodSelector'
import { LeaderboardTable } from '../LeaderboardTable'

import type { Period, LeaderboardMetric, LeaderboardUser } from '../../types'

interface LeaderboardPageProps {
  period: Period
  onPeriodChange: (period: Period) => void
}

export function LeaderboardPage({ period, onPeriodChange }: LeaderboardPageProps) {
  const [leaderboard, setLeaderboard] = useState<LeaderboardUser[]>([])
  const [leaderboardTotal, setLeaderboardTotal] = useState(0)
  const [leaderboardPage, setLeaderboardPage] = useState(1)
  const [leaderboardMetric, setLeaderboardMetric] = useState<LeaderboardMetric>('message_count')
  const [loading, setLoading] = useState(false)

  const fetchLeaderboard = useCallback(
    async (page: number) => {
      if (!apiClient.isAuthenticated()) return

      setLoading(true)
      try {
        const data = await apiClient.getLeaderboard(period, leaderboardMetric, page, 20)
        setLeaderboard(data.users)
        setLeaderboardTotal(data.total)
        setLeaderboardPage(page)
      } catch (err) {
        console.error('Failed to fetch leaderboard:', err)
      } finally {
        setLoading(false)
      }
    },
    [period, leaderboardMetric]
  )

  // Fetch data on mount and when period changes
  useEffect(() => {
    setLeaderboardPage(1)
    fetchLeaderboard(1)
  }, [period, fetchLeaderboard])

  // Refetch leaderboard when metric changes
  useEffect(() => {
    setLeaderboardPage(1)
    fetchLeaderboard(1)
  }, [leaderboardMetric, fetchLeaderboard])

  const handleMetricChange = (newMetric: LeaderboardMetric) => {
    setLeaderboardMetric(newMetric)
  }

  const handlePageChange = (newPage: number) => {
    fetchLeaderboard(newPage)
  }

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Leaderboard</h1>
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      <LeaderboardTable
        users={leaderboard}
        total={leaderboardTotal}
        page={leaderboardPage}
        limit={20}
        metric={leaderboardMetric}
        loading={loading}
        onMetricChange={handleMetricChange}
        onPageChange={handlePageChange}
      />
    </div>
  )
}
