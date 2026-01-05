import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { PeriodSelector } from '../PeriodSelector'
import { OverviewStats } from '../OverviewStats'
import { ActivityChart } from '../ActivityChart'
import { HeatmapGrid, HeatmapSkeleton } from '../common/HeatmapGrid'

import type { Period, StatsResponse, ActivityDataPoint, HeatmapData } from '../../types'

interface HomePageProps {
  period: Period
  onPeriodChange: (period: Period) => void
  chatTitle: string | null
}

export function HomePage({ period, onPeriodChange, chatTitle }: HomePageProps) {
  // Data state
  const [stats, setStats] = useState<StatsResponse | null>(null)
  const [activity, setActivity] = useState<ActivityDataPoint[]>([])
  const [heatmap, setHeatmap] = useState<HeatmapData | null>(null)

  // Loading states
  const [loadingStats, setLoadingStats] = useState(false)
  const [loadingActivity, setLoadingActivity] = useState(false)
  const [loadingHeatmap, setLoadingHeatmap] = useState(false)

  // Error states
  const [statsError, setStatsError] = useState<string | null>(null)
  const [activityError, setActivityError] = useState<string | null>(null)
  const [heatmapError, setHeatmapError] = useState<string | null>(null)

  // Fetch stats
  const fetchStats = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return
    setLoadingStats(true)
    setStatsError(null)
    try {
      const statsData = await apiClient.getStats(period)
      setStats(statsData)
    } catch (err) {
      console.error('Failed to fetch stats:', err)
      setStatsError('Failed to load stats')
    } finally {
      setLoadingStats(false)
    }
  }, [period])

  // Fetch activity
  const fetchActivity = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return
    setLoadingActivity(true)
    setActivityError(null)
    try {
      const activityData = await apiClient.getActivity(period)
      setActivity(activityData.data || [])
    } catch (err) {
      console.error('Failed to fetch activity:', err)
      setActivityError('Failed to load activity')
    } finally {
      setLoadingActivity(false)
    }
  }, [period])

  // Fetch heatmap
  const fetchHeatmap = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return
    setLoadingHeatmap(true)
    setHeatmapError(null)
    try {
      const heatmapData = await apiClient.getHeatmap('max', false)
      setHeatmap(heatmapData.group)
    } catch (err) {
      console.error('Failed to fetch heatmap:', err)
      setHeatmapError('Failed to load heatmap')
    } finally {
      setLoadingHeatmap(false)
    }
  }, [])

  // Fetch all data when period changes
  const fetchData = useCallback(async () => {
    fetchStats()
    fetchActivity()
    fetchHeatmap()
  }, [fetchStats, fetchActivity, fetchHeatmap])

  // Fetch data on mount and when period changes
  useEffect(() => {
    fetchData()
  }, [fetchData])

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Overview</h1>
        {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      <OverviewStats stats={stats} loading={loadingStats} error={statsError} onRetry={fetchStats} />

      <ActivityChart data={activity} loading={loadingActivity} error={activityError} onRetry={fetchActivity} />

      <section className="heatmap-section">
        <h2 className="section-title">Group Activity</h2>
        {loadingHeatmap ? (
          <HeatmapSkeleton />
        ) : heatmapError ? (
          <div className="section-error">
            <p className="section-error-message">{heatmapError}</p>
            <button className="section-error-btn" onClick={fetchHeatmap}>
              Retry
            </button>
          </div>
        ) : heatmap ? (
          <HeatmapGrid data={heatmap} />
        ) : (
          <div className="empty-list">No activity data</div>
        )}
      </section>
    </div>
  )
}
