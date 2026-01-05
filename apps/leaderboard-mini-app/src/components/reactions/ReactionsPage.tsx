import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { PeriodSelector } from '../PeriodSelector'

import type {
  Period,
  ReactionsOverviewResponse,
  TopReaction,
  ReactionUser,
} from '../../types'

interface ReactionsPageProps {
  period: Period
  onPeriodChange: (period: Period) => void
}

export function ReactionsPage({ period, onPeriodChange }: ReactionsPageProps) {
  const [data, setData] = useState<ReactionsOverviewResponse | null>(null)
  const [loading, setLoading] = useState(false)

  const fetchData = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return

    setLoading(true)
    try {
      const response = await apiClient.getReactionsOverview(period, 10)
      setData(response)
    } catch (err) {
      console.error('Failed to fetch reactions overview:', err)
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
        <h1>Reactions</h1>
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      {/* Top Reactions */}
      <section className="reactions-section">
        <h2 className="section-title">Top Reactions</h2>
        {loading ? (
          <div className="reactions-grid">
            {[...Array(10)].map((_, i) => (
              <div key={i} className="reaction-item skeleton" style={{ height: 72 }} />
            ))}
          </div>
        ) : data?.top_reactions.length ? (
          <div className="reactions-grid">
            {data.top_reactions.map((reaction) => (
              <ReactionItem key={reaction.emoji} reaction={reaction} />
            ))}
          </div>
        ) : (
          <div className="empty-list">No reactions yet</div>
        )}
      </section>

      {/* Top Givers */}
      <section className="reactions-section">
        <h2 className="section-title">Top Givers</h2>
        {loading ? (
          <div className="leaderboard-list">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : data?.top_givers.length ? (
          <UserList users={data.top_givers} />
        ) : (
          <div className="empty-list">No data</div>
        )}
      </section>

      {/* Top Receivers */}
      <section className="reactions-section">
        <h2 className="section-title">Top Receivers</h2>
        {loading ? (
          <div className="leaderboard-list">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : data?.top_receivers.length ? (
          <UserList users={data.top_receivers} />
        ) : (
          <div className="empty-list">No data</div>
        )}
      </section>
    </div>
  )
}

function ReactionItem({ reaction }: { reaction: TopReaction }) {
  return (
    <div className="reaction-item">
      <div className="reaction-emoji">{reaction.emoji}</div>
      <div className="reaction-count">{reaction.count.toLocaleString()}</div>
    </div>
  )
}

function UserList({ users }: { users: ReactionUser[] }) {
  return (
    <div className="leaderboard-list">
      {users.map((user) => (
        <div key={user.user_id} className="leaderboard-item">
          <div className={`rank ${getRankClass(user.rank)}`}>{user.rank}</div>
          <div className="user-info">
            <div className="user-name">{user.first_name} {user.last_name || ''}</div>
            {user.username && <div className="user-username">@{user.username}</div>}
          </div>
          <div className="user-score">{user.score.toLocaleString()}</div>
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
