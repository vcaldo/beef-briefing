import { useState, useEffect, useCallback, useMemo } from 'react'
import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  Tooltip,
  Legend,
  AreaChart,
  Area,
  XAxis,
  YAxis,
} from 'recharts'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '../../newrelic'
import { PeriodSelector } from '../PeriodSelector'
import { Avatar, TrendIndicator } from '../common'

import type {
  Period,
  MediaOverviewResponse,
  MediaTypeDistribution,
  MediaUser,
} from '../../types'

// Media type colors for pie chart
const MEDIA_COLORS: Record<string, string> = {
  photo: '#3390ec',      // Telegram blue
  video: '#ef4444',      // Red
  video_note: '#f97316', // Orange
  animation: '#8b5cf6',  // Purple
  audio: '#22c55e',      // Green
  voice: '#14b8a6',      // Teal
  document: '#6b7280',   // Gray
  sticker: '#ec4899',    // Pink
}

// Human-readable labels
const MEDIA_LABELS: Record<string, string> = {
  photo: 'Photos',
  video: 'Videos',
  video_note: 'Video Notes',
  animation: 'GIFs',
  audio: 'Audio',
  voice: 'Voice',
  document: 'Documents',
  sticker: 'Stickers',
}

function formatDate(dateStr: string, timezone: string): string {
  const date = new Date(dateStr)
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', timeZone: timezone })
}

interface MediaPageProps {
  period: Period
  onPeriodChange: (period: Period) => void
  chatTitle: string | null
  prefetchedData?: MediaOverviewResponse | null
  onPrefetchConsumed?: () => void
}

export function MediaPage({
  period,
  onPeriodChange,
  chatTitle,
  prefetchedData,
  onPrefetchConsumed,
}: MediaPageProps) {
  const [data, setData] = useState<MediaOverviewResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchData = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return

    setLoading(true)
    setError(null)
    try {
      const response = await apiClient.getMediaOverview(period, 10)
      setData(response)
      addPageAction('media_loaded', {
        period,
        total_media: response.stats.total_media,
        distribution_count: response.distribution.length,
      })
    } catch (err) {
      console.error('Failed to fetch media data:', err)
      setError('Failed to load media statistics')
      if (err instanceof Error) {
        noticeError(err, { context: 'media_fetch', period })
      }
    } finally {
      setLoading(false)
    }
  }, [period])

  useEffect(() => {
    if (prefetchedData) {
      setData(prefetchedData)
      onPrefetchConsumed?.()
      return
    }
    fetchData()
  }, [period, prefetchedData, onPrefetchConsumed, fetchData])

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Media</h1>
        {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      {/* Stats Cards */}
      <MediaStats stats={data?.stats} loading={loading} error={error} />

      {/* Pie Chart - Media Distribution */}
      <section className="reactions-section">
        <h2 className="section-title">Media Distribution</h2>
        <p className="section-subtitle">Breakdown by type</p>
        {loading ? (
          <div className="chart-container skeleton skeleton-chart" />
        ) : error ? (
          <div className="section-error">
            <p className="section-error-message">{error}</p>
            <button className="section-error-btn" onClick={fetchData}>
              Retry
            </button>
          </div>
        ) : data?.distribution.length ? (
          <MediaPieChart distribution={data.distribution} />
        ) : (
          <div className="empty-list">No media data</div>
        )}
      </section>

      {/* Timeline */}
      <MediaTimeline
        activity={data?.activity || []}
        loading={loading}
        error={error}
        onRetry={fetchData}
      />

      {/* Top Senders */}
      <section className="reactions-section">
        <h2 className="section-title">Top Media Senders</h2>
        <p className="section-subtitle">Users who share the most media</p>
        {loading ? (
          <div className="leaderboard-list">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : data?.top_senders.length ? (
          <MediaUserList users={data.top_senders} />
        ) : (
          <div className="empty-list">No data</div>
        )}
      </section>
    </div>
  )
}

// Stats component
function MediaStats({ stats, loading, error }: {
  stats: MediaOverviewResponse['stats'] | undefined
  loading: boolean
  error: string | null
}) {
  if (loading) {
    return (
      <div className="stats-grid">
        {[1, 2, 3, 4, 5, 6, 7, 8].map((i) => (
          <div key={i} className="stat-card skeleton skeleton-stat" />
        ))}
      </div>
    )
  }

  if (error || !stats) {
    return null // Error shown in other sections
  }

  return (
    <div className="stats-grid">
      <div className="stat-card stat-card-with-trend">
        <TrendIndicator trend={stats.total_media_trend} />
        <div className="stat-value">{stats.total_media.toLocaleString()}</div>
        <div className="stat-label">Total Media</div>
      </div>
      <div className="stat-card stat-card-with-trend">
        <TrendIndicator trend={stats.total_photos_trend} />
        <div className="stat-value">{stats.total_photos.toLocaleString()}</div>
        <div className="stat-label">Photos</div>
      </div>
      <div className="stat-card stat-card-with-trend">
        <TrendIndicator trend={stats.total_videos_trend} />
        <div className="stat-value">{stats.total_videos.toLocaleString()}</div>
        <div className="stat-label">Videos</div>
      </div>
      <div className="stat-card stat-card-with-trend">
        <TrendIndicator trend={stats.total_gifs_trend} />
        <div className="stat-value">{stats.total_gifs.toLocaleString()}</div>
        <div className="stat-label">GIFs</div>
      </div>
      <div className="stat-card stat-card-with-trend">
        <TrendIndicator trend={stats.total_voice_trend} />
        <div className="stat-value">{stats.total_voice.toLocaleString()}</div>
        <div className="stat-label">Voice</div>
      </div>
      <div className="stat-card stat-card-with-trend">
        <TrendIndicator trend={stats.total_documents_trend} />
        <div className="stat-value">{stats.total_documents.toLocaleString()}</div>
        <div className="stat-label">Documents</div>
      </div>
      <div className="stat-card stat-card-with-trend">
        <TrendIndicator trend={stats.total_stickers_trend} />
        <div className="stat-value">{stats.total_stickers.toLocaleString()}</div>
        <div className="stat-label">Stickers</div>
      </div>
      <div className="stat-card stat-card-with-trend">
        <TrendIndicator trend={stats.media_per_day_trend} />
        <div className="stat-value">{stats.media_per_day.toFixed(1)}</div>
        <div className="stat-label">Media/Day</div>
      </div>
    </div>
  )
}

// Pie chart component
function MediaPieChart({ distribution }: { distribution: MediaTypeDistribution[] }) {
  const chartData = useMemo(() =>
    distribution.map(d => ({
      name: MEDIA_LABELS[d.media_type] || d.media_type,
      value: d.count,
      color: MEDIA_COLORS[d.media_type] || '#6b7280',
    })),
  [distribution])

  const total = useMemo(() =>
    chartData.reduce((sum, d) => sum + d.value, 0),
  [chartData])

  const renderLabel = ({ name, percent }: { name: string; percent: number }) => {
    if (percent < 0.05) return null // Hide labels for small slices
    return `${name} ${(percent * 100).toFixed(0)}%`
  }

  return (
    <div className="chart-container" style={{ height: 280 }}>
      <ResponsiveContainer width="100%" height="100%">
        <PieChart>
          <Pie
            data={chartData}
            dataKey="value"
            nameKey="name"
            cx="50%"
            cy="50%"
            outerRadius={85}
            innerRadius={45}
            label={renderLabel}
            labelLine={false}
            paddingAngle={2}
          >
            {chartData.map((entry, index) => (
              <Cell key={`cell-${index}`} fill={entry.color} />
            ))}
          </Pie>
          <Tooltip
            formatter={(value: number) => [value.toLocaleString(), 'Count']}
            contentStyle={{
              backgroundColor: 'var(--tg-theme-bg-color)',
              border: '1px solid var(--tg-theme-secondary-bg-color)',
              borderRadius: '8px',
              fontSize: '12px',
              color: 'var(--tg-theme-text-color)',
            }}
            itemStyle={{ color: 'var(--tg-theme-text-color)' }}
            labelStyle={{ color: 'var(--tg-theme-text-color)' }}
          />
          <Legend
            wrapperStyle={{ fontSize: '11px' }}
            formatter={(value) => <span style={{ color: 'var(--tg-theme-text-color)' }}>{value}</span>}
          />
          <text
            x="50%"
            y="50%"
            textAnchor="middle"
            dominantBaseline="middle"
            style={{ fontSize: '18px', fontWeight: 600, fill: 'var(--tg-theme-text-color)' }}
          >
            {total.toLocaleString()}
          </text>
        </PieChart>
      </ResponsiveContainer>
    </div>
  )
}

// Timeline chart component
function MediaTimeline({ activity, loading, error, onRetry }: {
  activity: MediaOverviewResponse['activity']
  loading: boolean
  error: string | null
  onRetry: () => void
}) {
  const timezone = apiClient.getTimezone()

  const chartData = useMemo(() =>
    activity.map((point) => ({
      date: point.date,
      dateLabel: formatDate(point.date, timezone),
      media: point.count,
    })),
  [activity, timezone])

  if (loading) {
    return (
      <div className="activity-section">
        <h2 className="section-title">Media Timeline</h2>
        <div className="chart-container">
          <div className="skeleton skeleton-chart" />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="activity-section">
        <h2 className="section-title">Media Timeline</h2>
        <div className="section-error">
          <p className="section-error-message">{error}</p>
          <button className="section-error-btn" onClick={onRetry}>
            Retry
          </button>
        </div>
      </div>
    )
  }

  if (activity.length === 0) {
    return (
      <div className="activity-section">
        <h2 className="section-title">Media Timeline</h2>
        <div className="chart-container">
          <div className="empty-state">No activity data available</div>
        </div>
      </div>
    )
  }

  return (
    <div className="activity-section">
      <h2 className="section-title">Media Timeline</h2>
      <div className="chart-container">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={chartData} margin={{ top: 10, right: 10, left: -4, bottom: 0 }}>
            <defs>
              <linearGradient id="colorMedia" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.3} />
                <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
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
              dataKey="media"
              stroke="#8b5cf6"
              strokeWidth={2}
              fillOpacity={1}
              fill="url(#colorMedia)"
              name="Media"
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

// User list component
function MediaUserList({ users }: { users: MediaUser[] }) {
  return (
    <div className="leaderboard-list">
      {users.map((user) => (
        <div key={user.user_id} className="leaderboard-item">
          <div className={`rank ${getRankClass(user.rank)}`}>{user.rank}</div>
          <Avatar
            photoUrl={user.photo_url}
            firstName={user.first_name}
            lastName={user.last_name}
            size="small"
          />
          <div className="user-info">
            <div className="user-name">{user.first_name} {user.last_name || ''}</div>
            {user.username && <div className="user-username">@{user.username}</div>}
          </div>
          <div className="user-score">{user.media_count.toLocaleString()}</div>
        </div>
      ))}
    </div>
  )
}

function getRankClass(rank: number): string {
  if (rank === 1) return 'gold'
  if (rank === 2) return 'silver'
  if (rank === 3) return 'bronze'
  return 'default'
}
