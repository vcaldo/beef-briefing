import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { PeriodSelector } from '../PeriodSelector'
import { ActivityChart } from '../ActivityChart'
import { Avatar, HeatmapGrid, HeatmapSkeleton } from '../common'

import type {
  Period,
  ProfileResponse,
  TopInteractor,
  ChatUser,
} from '../../types'

interface ProfilePageProps {
  period: Period
  onPeriodChange: (period: Period) => void
  firstName: string
  username: string | null
  chatTitle: string | null
  isAdmin: boolean
}

export function ProfilePage({ period, onPeriodChange, firstName, username, chatTitle, isAdmin }: ProfilePageProps) {
  const [data, setData] = useState<ProfileResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Admin impersonation state
  const [users, setUsers] = useState<ChatUser[]>([])
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null)
  const [loadingUsers, setLoadingUsers] = useState(false)

  // Determine displayed user info (impersonated user or self)
  const displayFirstName = selectedUserId && data?.first_name ? data.first_name : firstName
  const displayUsername = selectedUserId && data?.username !== undefined ? data.username : username

  // Fetch users list for admin dropdown
  useEffect(() => {
    async function fetchUsers() {
      if (!isAdmin || !apiClient.isAuthenticated()) return

      setLoadingUsers(true)
      try {
        const response = await apiClient.getChatUsers()
        setUsers(response.users)
      } catch (err) {
        console.error('Failed to fetch users:', err)
      } finally {
        setLoadingUsers(false)
      }
    }
    fetchUsers()
  }, [isAdmin])

  const fetchData = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return

    setLoading(true)
    setError(null)
    try {
      const response = await apiClient.getProfile(period, undefined, selectedUserId || undefined)
      setData(response)
    } catch (err) {
      console.error('Failed to fetch profile:', err)
      setError('Failed to load profile')
    } finally {
      setLoading(false)
    }
  }, [period, selectedUserId])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  // Handle user selection change
  const handleUserChange = (userId: number | null) => {
    setSelectedUserId(userId)
  }

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Profile</h1>
        {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
      </header>

      {/* Admin User Selector */}
      {isAdmin && (
        <div className="admin-selector">
          <label className="admin-selector-label">View as user:</label>
          {loadingUsers ? (
            <div className="skeleton skeleton-row" style={{ height: '44px' }} />
          ) : (
            <select
              className="admin-selector-dropdown"
              value={selectedUserId || ''}
              onChange={(e) => handleUserChange(e.target.value ? Number(e.target.value) : null)}
            >
              <option value="">-- My Profile --</option>
              {users.map((user) => (
                <option key={user.user_id} value={user.user_id}>
                  {user.first_name} {user.last_name || ''} {user.username ? `(@${user.username})` : ''}
                </option>
              ))}
            </select>
          )}
        </div>
      )}

      {/* Impersonation Banner */}
      {selectedUserId && (
        <div className="impersonation-banner">
          Viewing {displayFirstName}'s profile
        </div>
      )}

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      {/* Profile Header */}
      <div className="profile-header">
        <Avatar
          photoUrl={data?.photo_url}
          firstName={displayFirstName}
          size="large"
        />
        <h2 className="profile-name">{displayFirstName}</h2>
        {displayUsername && <div className="profile-username">@{displayUsername}</div>}
      </div>

      {/* Stats Grid */}
      <div className="profile-stats-grid">
        {loading ? (
          <>
            {[...Array(6)].map((_, i) => (
              <div key={i} className="skeleton skeleton-stat" />
            ))}
          </>
        ) : error ? (
          <div className="profile-stat-card section-error" style={{ gridColumn: '1 / -1' }}>
            <p className="section-error-message">{error}</p>
            <button className="section-error-btn" onClick={fetchData}>
              Retry
            </button>
          </div>
        ) : (
          <>
            {/* Row 1: Messages | Avg */}
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.message_count.toLocaleString() || 0}</div>
              <div className="profile-stat-label">Messages</div>
              {data?.stats.rank_by_messages && (
                <div className="profile-rank-badge">#{data.stats.rank_by_messages}</div>
              )}
            </div>
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.avg_messages_per_day?.toFixed(1) || '0'}</div>
              <div className="profile-stat-label">Avg</div>
            </div>
            {/* Row 2: Reactions Received | Reactions Given */}
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.reactions_received.toLocaleString() || 0}</div>
              <div className="profile-stat-label">Reactions Received</div>
              {data?.stats.rank_by_reactions_received && (
                <div className="profile-rank-badge">#{data.stats.rank_by_reactions_received}</div>
              )}
            </div>
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.reactions_sent.toLocaleString() || 0}</div>
              <div className="profile-stat-label">Reactions Given</div>
            </div>
            {/* Row 3: Replies Received | Replies Sent */}
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.replies_received?.toLocaleString() || 0}</div>
              <div className="profile-stat-label">Replies Received</div>
            </div>
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.replies_sent?.toLocaleString() || 0}</div>
              <div className="profile-stat-label">Replies Sent</div>
            </div>
            {/* Row 4: Days Streak | Active Days */}
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.current_streak || 0}</div>
              <div className="profile-stat-label">Days Streak</div>
            </div>
            <div className="profile-stat-card">
              <div className="profile-stat-value">{data?.stats.active_days || 0}</div>
              <div className="profile-stat-label">Active Days</div>
            </div>
          </>
        )}
      </div>

      {/* Who Reacts to You */}
      <section className="interactors-section">
        <h2 className="section-title">Who Reacts to You</h2>
        <p className="section-subtitle">Users who react most to your messages</p>
        {loading ? (
          <div className="interactor-list">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : data?.top_reactors.length ? (
          <InteractorList interactors={data.top_reactors} />
        ) : (
          <div className="empty-list">No reactions yet</div>
        )}
      </section>

      {/* Who You React To */}
      <section className="interactors-section">
        <h2 className="section-title">Who You React To</h2>
        <p className="section-subtitle">Users whose messages you react to most</p>
        {loading ? (
          <div className="interactor-list">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : data?.top_reacted_to.length ? (
          <InteractorList interactors={data.top_reacted_to} />
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

      {/* Personal Activity Chart */}
      <ActivityChart
        data={data?.activity || []}
        loading={loading}
        error={error}
        onRetry={fetchData}
        title="Your Activity"
      />

      {/* Personal Heatmap */}
      <section className="activity-heatmap-section">
        <h2 className="section-title">Your Heatmap</h2>
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
