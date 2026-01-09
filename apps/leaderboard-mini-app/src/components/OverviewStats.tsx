import type { StatsResponse } from '../types'
import { TrendIndicator } from './common'

interface OverviewStatsProps {
  stats: StatsResponse | null
  loading: boolean
  error?: string | null
  onRetry?: () => void
}

function formatNumber(num: number): string {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M'
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'K'
  }
  return num.toLocaleString()
}

export function OverviewStats({ stats, loading, error, onRetry }: OverviewStatsProps) {
  if (loading) {
    return (
      <div className="stats-grid">
        {[1, 2, 3, 4, 5].map((i) => (
          <div key={i} className={`stat-card ${i === 5 ? 'full-width' : ''}`}>
            <div className="skeleton skeleton-stat" />
          </div>
        ))}
      </div>
    )
  }

  if (error) {
    return (
      <div className="stats-grid">
        <div className="stat-card full-width section-error">
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

  if (!stats) {
    return null
  }

  const statCards = [
    { value: stats.total_messages, label: 'Messages', trend: stats.total_messages_trend },
    { value: stats.total_users, label: 'Active Users', trend: stats.total_users_trend },
    { value: stats.total_reactions, label: 'Reactions', trend: stats.total_reactions_trend },
    { value: stats.total_media, label: 'Media Files', trend: stats.total_media_trend },
  ]

  return (
    <div className="stats-grid">
      {statCards.map((stat, index) => (
        <div key={index} className="stat-card stat-card-with-trend">
          <TrendIndicator trend={stat.trend} />
          <div className="stat-value">{formatNumber(stat.value)}</div>
          <div className="stat-label">{stat.label}</div>
        </div>
      ))}
      <div className="stat-card full-width stat-card-with-trend">
        <TrendIndicator trend={stats.messages_per_day_trend} />
        <div className="stat-value">{stats.messages_per_day.toFixed(1)}</div>
        <div className="stat-label">Messages per Day</div>
      </div>
    </div>
  )
}
