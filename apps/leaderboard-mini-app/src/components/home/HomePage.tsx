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
}

export function HomePage({ period, onPeriodChange }: HomePageProps) {
  // Data state
  const [stats, setStats] = useState<StatsResponse | null>(null)
  const [activity, setActivity] = useState<ActivityDataPoint[]>([])
  const [heatmap, setHeatmap] = useState<HeatmapData | null>(null)

  // Loading states
  const [loadingStats, setLoadingStats] = useState(false)
  const [loadingActivity, setLoadingActivity] = useState(false)
  const [loadingHeatmap, setLoadingHeatmap] = useState(false)

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

    // Fetch group heatmap (always use max period for group heatmap)
    setLoadingHeatmap(true)
    try {
      const heatmapData = await apiClient.getHeatmap('max', false)
      setHeatmap(heatmapData.group)
    } catch (err) {
      console.error('Failed to fetch heatmap:', err)
    } finally {
      setLoadingHeatmap(false)
    }
  }, [period])

  // Fetch data on mount and when period changes
  useEffect(() => {
    fetchData()
  }, [fetchData])

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Overview</h1>
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      <OverviewStats stats={stats} loading={loadingStats} />

      <ActivityChart data={activity} loading={loadingActivity} />

      <section className="heatmap-section">
        <h2 className="section-title">Group Activity</h2>
        <p className="section-subtitle">When the group is most active</p>
        {loadingHeatmap ? (
          <HeatmapSkeleton />
        ) : heatmap ? (
          <HeatmapGrid data={heatmap} />
        ) : (
          <div className="empty-list">No activity data</div>
        )}
      </section>
    </div>
  )
}
