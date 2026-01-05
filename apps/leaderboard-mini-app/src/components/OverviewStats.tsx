import type { StatsResponse } from '../types'

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
    { value: stats.total_messages, label: 'Messages' },
    { value: stats.total_users, label: 'Active Users' },
    { value: stats.total_reactions, label: 'Reactions' },
    { value: stats.total_media, label: 'Media Files' },
  ]

  return (
    <div className="stats-grid">
      {statCards.map((stat, index) => (
        <div key={index} className="stat-card">
          <div className="stat-value">{formatNumber(stat.value)}</div>
          <div className="stat-label">{stat.label}</div>
        </div>
      ))}
      <div className="stat-card full-width">
        <div className="stat-value">{stats.messages_per_day.toFixed(1)}</div>
        <div className="stat-label">Messages per Day</div>
      </div>
    </div>
  )
}
