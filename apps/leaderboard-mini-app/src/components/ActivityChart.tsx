import { useMemo } from 'react'
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import type { ActivityDataPoint } from '../types'
import { apiClient } from '../api/client'

interface ActivityChartProps {
  data: ActivityDataPoint[]
  loading: boolean
  error?: string | null
  onRetry?: () => void
  title?: string
}

function formatDate(dateStr: string, timezone: string): string {
  const date = new Date(dateStr)
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', timeZone: timezone })
}

export function ActivityChart({ data, loading, error, onRetry, title = 'Activity' }: ActivityChartProps) {
  const timezone = apiClient.getTimezone()
  const chartData = useMemo(() => {
    return data.map((point) => ({
      ...point,
      dateLabel: formatDate(point.date, timezone),
    }))
  }, [data, timezone])

  if (loading) {
    return (
      <div className="activity-section">
        <h2 className="section-title">{title}</h2>
        <div className="chart-container">
          <div className="skeleton skeleton-chart" />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="activity-section">
        <h2 className="section-title">{title}</h2>
        <div className="section-error">
          <p className="section-error-message">{error}</p>
          {onRetry && (
            <button className="section-error-btn" onClick={onRetry}>
              Retry
            </button>
          )}
        </div>
      </div>
    )
  }

  if (data.length === 0) {
    return (
      <div className="activity-section">
        <h2 className="section-title">{title}</h2>
        <div className="chart-container">
          <div className="empty-state">No activity data available</div>
        </div>
      </div>
    )
  }

  return (
    <div className="activity-section">
      <h2 className="section-title">{title}</h2>
      <div className="chart-container">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={chartData} margin={{ top: 10, right: 10, left: -4, bottom: 0 }}>
            <defs>
              <linearGradient id="colorMessages" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--tg-theme-button-color)" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--tg-theme-button-color)" stopOpacity={0} />
              </linearGradient>
            </defs>
            <XAxis
              dataKey="dateLabel"
              tick={{ fontSize: 10, fill: 'var(--tg-theme-hint-color)' }}
              tickLine={false}
              axisLine={false}
              interval="preserveStartEnd"
            />
            <YAxis
              tick={{ fontSize: 10, fill: 'var(--tg-theme-hint-color)' }}
              tickLine={false}
              axisLine={false}
              width={40}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: 'var(--tg-theme-bg-color)',
                border: '1px solid var(--tg-theme-secondary-bg-color)',
                borderRadius: '8px',
                fontSize: '12px',
              }}
              labelStyle={{ color: 'var(--tg-theme-text-color)' }}
            />
            <Area
              type="monotone"
              dataKey="messages"
              stroke="var(--tg-theme-button-color)"
              strokeWidth={2}
              fillOpacity={1}
              fill="url(#colorMessages)"
              name="Messages"
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
