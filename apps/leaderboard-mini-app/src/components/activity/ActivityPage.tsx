import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { PeriodSelector } from '../PeriodSelector'
import { HeatmapGrid, HeatmapSkeleton } from '../common/HeatmapGrid'

import type { Period, HeatmapResponse } from '../../types'

interface ActivityPageProps {
  period: Period
  onPeriodChange: (period: Period) => void
}

export function ActivityPage({ period, onPeriodChange }: ActivityPageProps) {
  const [data, setData] = useState<HeatmapResponse | null>(null)
  const [loading, setLoading] = useState(false)

  const fetchData = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return

    setLoading(true)
    try {
      const response = await apiClient.getHeatmap(period, true)
      setData(response)
    } catch (err) {
      console.error('Failed to fetch heatmap:', err)
    } finally {
      setLoading(false)
    }
  }, [period])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Activity</h1>
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      {/* Group Heatmap */}
      <section className="activity-heatmap-section">
        <h2 className="section-title">Group Activity</h2>
        <p className="section-subtitle">When the group is most active (all time)</p>
        {loading ? (
          <HeatmapSkeleton />
        ) : data?.group ? (
          <HeatmapGrid data={data.group} />
        ) : (
          <div className="empty-list">No activity data</div>
        )}
      </section>

      {/* User Heatmap */}
      <section className="activity-heatmap-section">
        <h2 className="section-title">Your Activity</h2>
        <p className="section-subtitle">When you're most active</p>
        {loading ? (
          <HeatmapSkeleton />
        ) : data?.user ? (
          <HeatmapGrid data={data.user} />
        ) : (
          <div className="empty-list">No personal activity data</div>
        )}
      </section>
    </div>
  )
}
