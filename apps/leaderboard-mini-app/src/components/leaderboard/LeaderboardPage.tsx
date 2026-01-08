import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { setCustomAttribute, addPageAction, noticeError } from '../../newrelic'
import { PeriodSelector } from '../PeriodSelector'
import { LeaderboardTable } from '../LeaderboardTable'

import type { Period, LeaderboardMetric, LeaderboardUser } from '../../types'

interface LeaderboardPageProps {
  period: Period
  onPeriodChange: (period: Period) => void
  chatTitle: string | null
}

export function LeaderboardPage({ period, onPeriodChange, chatTitle }: LeaderboardPageProps) {
  const [leaderboard, setLeaderboard] = useState<LeaderboardUser[]>([])
  const [leaderboardTotal, setLeaderboardTotal] = useState(0)
  const [leaderboardPage, setLeaderboardPage] = useState(1)
  const [leaderboardMetric, setLeaderboardMetric] = useState<LeaderboardMetric>('message_count')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchLeaderboard = useCallback(
    async (page: number) => {
      if (!apiClient.isAuthenticated()) return

      setLoading(true)
      setError(null)
      addPageAction('leaderboard_fetch_start', {
        period,
        metric: leaderboardMetric,
        page,
      })
      try {
        const data = await apiClient.getLeaderboard(period, leaderboardMetric, page, 20)
        setLeaderboard(data.users)
        setLeaderboardTotal(data.total)
        setLeaderboardPage(page)
        addPageAction('leaderboard_fetch_success', {
          period,
          metric: leaderboardMetric,
          page,
          total_users: data.total,
          returned_users: data.users.length,
        })
      } catch (err) {
        console.error('Failed to fetch leaderboard:', err)
        setError('Failed to load leaderboard')
        if (err instanceof Error) {
          noticeError(err, {
            context: 'leaderboard_fetch',
            period,
            metric: leaderboardMetric,
            page,
          })
        }
        addPageAction('leaderboard_fetch_error', {
          period,
          metric: leaderboardMetric,
          page,
        })
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
    addPageAction('metric_change', {
      metric: newMetric,
      previous_metric: leaderboardMetric,
    })
    setCustomAttribute('selected_metric', newMetric)
    setLeaderboardMetric(newMetric)
  }

  const handlePageChange = (newPage: number) => {
    fetchLeaderboard(newPage)
  }

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Leaderboard</h1>
        {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      <LeaderboardTable
        users={leaderboard}
        total={leaderboardTotal}
        page={leaderboardPage}
        limit={20}
        metric={leaderboardMetric}
        loading={loading}
        error={error}
        onMetricChange={handleMetricChange}
        onPageChange={handlePageChange}
        onRetry={() => fetchLeaderboard(leaderboardPage)}
      />
    </div>
  )
}
