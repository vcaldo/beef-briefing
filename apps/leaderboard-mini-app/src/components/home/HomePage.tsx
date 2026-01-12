import { useState, useEffect, useCallback, useRef } from 'react'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { PeriodSelector } from '../PeriodSelector'
import { OverviewStats } from '../OverviewStats'
import { ActivityChart } from '../ActivityChart'
import { HeatmapGrid, HeatmapSkeleton } from '../common/HeatmapGrid'

import type { Period, StatsResponse, ActivityDataPoint, HeatmapData } from '../../types'

// Prefetched data passed from App (loaded during splash screen)
interface PrefetchedHomeData {
  stats: StatsResponse | null
  activity: ActivityDataPoint[]
  heatmap: HeatmapData | null
}

interface HomePageProps {
  period: Period
  onPeriodChange: (period: Period) => void
  chatTitle: string | null
  prefetchedData?: PrefetchedHomeData | null
  onPrefetchConsumed?: () => void
}

export function HomePage({ period, onPeriodChange, chatTitle, prefetchedData, onPrefetchConsumed }: HomePageProps) {
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

  // Track if prefetch data has been consumed (to avoid double-fetching on mount)
  const prefetchUsedRef = useRef(false)

  // Fetch stats
  const fetchStats = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return
    setLoadingStats(true)
    setStatsError(null)
    try {
      const statsData = await apiClient.getStats(period)
      setStats(statsData)
      addPageAction('stats_loaded', {
        period,
        total_messages: statsData.total_messages,
        total_users: statsData.total_users,
      })
    } catch (err) {
      console.error('Failed to fetch stats:', err)
      setStatsError('Failed to load stats')
      if (err instanceof Error) {
        noticeError(err, { context: 'stats_fetch', period })
      }
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
      addPageAction('activity_loaded', {
        period,
        data_points: activityData.data?.length || 0,
      })
    } catch (err) {
      console.error('Failed to fetch activity:', err)
      setActivityError('Failed to load activity')
      if (err instanceof Error) {
        noticeError(err, { context: 'activity_fetch', period })
      }
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
      const heatmapData = await apiClient.getHeatmap(period, false)
      setHeatmap(heatmapData.group)
      addPageAction('heatmap_loaded', { period })
    } catch (err) {
      console.error('Failed to fetch heatmap:', err)
      setHeatmapError('Failed to load heatmap')
      if (err instanceof Error) {
        noticeError(err, { context: 'heatmap_fetch', period })
      }
    } finally {
      setLoadingHeatmap(false)
    }
  }, [period])

  // Fetch all data when period changes
  const fetchData = useCallback(async () => {
    fetchStats()
    fetchActivity()
    fetchHeatmap()
  }, [fetchStats, fetchActivity, fetchHeatmap])

  // Use prefetched data if available (from splash screen), otherwise fetch
  useEffect(() => {
    // Use prefetch only once on initial mount
    if (prefetchedData && !prefetchUsedRef.current) {
      prefetchUsedRef.current = true
      setStats(prefetchedData.stats)
      setActivity(prefetchedData.activity)
      setHeatmap(prefetchedData.heatmap)
      onPrefetchConsumed?.()
      return
    }
    // Always fetch on period change (after initial prefetch consumed)
    fetchData()
  }, [period, prefetchedData, onPrefetchConsumed, fetchData])

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Overview</h1>
        {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      <OverviewStats stats={stats} loading={loadingStats} error={statsError} onRetry={fetchStats} />

      <ActivityChart data={activity} loading={loadingActivity} error={activityError} onRetry={fetchActivity} title="Group Activity" />

      <section className="heatmap-section">
        <h2 className="section-title">Group Heatmap</h2>
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
