import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { PeriodSelector } from '../PeriodSelector'
import { Avatar, HeatmapGrid, HeatmapSkeleton } from '../common'

import type {
  Period,
  ProfileResponse,
  TopInteractor,
} from '../../types'

interface ProfilePageProps {
  period: Period
  onPeriodChange: (period: Period) => void
  firstName: string
  username: string | null
}

export function ProfilePage({ period, onPeriodChange, firstName, username }: ProfilePageProps) {
  const [data, setData] = useState<ProfileResponse | null>(null)
  const [loading, setLoading] = useState(false)

  const fetchData = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return

    setLoading(true)
    try {
      const response = await apiClient.getProfile(period)
      setData(response)
    } catch (err) {
      console.error('Failed to fetch profile:', err)
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
        <h1>Profile</h1>
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      {/* Profile Header */}
      <div className="profile-header">
        <Avatar
          photoUrl={data?.photo_url}
          firstName={firstName}
          size="large"
        />
        <h2 className="profile-name">{firstName}</h2>
        {username && <div className="profile-username">@{username}</div>}
      </div>

      {/* Stats Grid */}
      <div className="profile-stats-grid">
        {loading ? (
          <>
            {[...Array(4)].map((_, i) => (
              <div key={i} className="skeleton skeleton-stat" />
            ))}
          </>
        ) : (
          <>
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.message_count.toLocaleString() || 0}</div>
              <div className="profile-stat-label">Messages</div>
              {data?.stats.rank_by_messages && (
                <div className="profile-rank-badge">#{data.stats.rank_by_messages}</div>
              )}
            </div>
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.reactions_received.toLocaleString() || 0}</div>
              <div className="profile-stat-label">Reactions</div>
              {data?.stats.rank_by_reactions_received && (
                <div className="profile-rank-badge">#{data.stats.rank_by_reactions_received}</div>
              )}
            </div>
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.reactions_sent.toLocaleString() || 0}</div>
              <div className="profile-stat-label">Given</div>
            </div>
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.active_days || 0}</div>
              <div className="profile-stat-label">Active Days</div>
            </div>
          </>
        )}
      </div>

      {/* Top Reactors */}
      <section className="interactors-section">
        <h2 className="section-title">Top Fans</h2>
        <p className="section-subtitle">Users who react most to your messages</p>
        {loading ? (
          <div className="interactor-list">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : data?.top_reactors.length ? (
          <InteractorList interactors={data.top_reactors} showEmoji />
        ) : (
          <div className="empty-list">No reactions yet</div>
        )}
      </section>

      {/* Who You Reply To */}
      <section className="interactors-section">
        <h2 className="section-title">Who You Reply To</h2>
        <p className="section-subtitle">Users you reply to most</p>
        {loading ? (
          <div className="interactor-list">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : data?.top_replied_to.length ? (
          <InteractorList interactors={data.top_replied_to} />
        ) : (
          <div className="empty-list">No replies yet</div>
        )}
      </section>

      {/* Who Replies to You */}
      <section className="interactors-section">
        <h2 className="section-title">Who Replies to You</h2>
        <p className="section-subtitle">Users who reply to your messages</p>
        {loading ? (
          <div className="interactor-list">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : data?.top_repliers.length ? (
          <InteractorList interactors={data.top_repliers} />
        ) : (
          <div className="empty-list">No replies yet</div>
        )}
      </section>

      {/* Personal Heatmap */}
      <section className="activity-heatmap-section">
        <h2 className="section-title">Your Activity</h2>
        {loading ? (
          <HeatmapSkeleton />
        ) : data?.heatmap ? (
          <HeatmapGrid data={data.heatmap} />
        ) : (
          <div className="empty-list">No activity data</div>
        )}
      </section>
    </div>
  )
}

// Check if emoji is a paid emoji (numeric ID)
function isPaidEmoji(emoji: string): boolean {
  return /^\d+$/.test(emoji)
}

function InteractorList({ interactors, showEmoji = false }: { interactors: TopInteractor[]; showEmoji?: boolean }) {
  return (
    <div className="interactor-list">
      {interactors.map((interactor) => (
        <div key={interactor.user_id} className="interactor-item">
          <div className="interactor-rank">{interactor.rank}</div>
          <Avatar
            photoUrl={interactor.photo_url}
            firstName={interactor.first_name}
            lastName={interactor.last_name}
            size="small"
          />
          <div className="interactor-info">
            <div className="interactor-name">
              {interactor.first_name} {interactor.last_name || ''}
            </div>
          </div>
          {showEmoji && interactor.top_emoji && (
            isPaidEmoji(interactor.top_emoji) ? (
              <div className="interactor-emoji-paid">
                <span className="paid-label-small">PAID</span>
              </div>
            ) : (
              <div className="interactor-emoji">{interactor.top_emoji}</div>
            )
          )}
          <div className="interactor-score">{interactor.score}</div>
        </div>
      ))}
    </div>
  )
}
