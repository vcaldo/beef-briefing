import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { PeriodSelector } from '../PeriodSelector'
import { Avatar } from '../common'

import type {
  Period,
  ReactionsOverviewResponse,
  TopReaction,
  ReactionUser,
} from '../../types'

// Group all paid/custom emoji reactions into a single combined item
function groupPaidReactions(reactions: TopReaction[]): TopReaction[] {
  const regularReactions: TopReaction[] = []
  let paidTotal = 0

  for (const reaction of reactions) {
    if (reaction.reaction_type === 'custom_emoji' || reaction.reaction_type === 'paid') {
      paidTotal += reaction.count
    } else {
      regularReactions.push(reaction)
    }
  }

  // Add combined paid reactions at the end if there are any
  if (paidTotal > 0) {
    regularReactions.push({
      emoji: 'paid',
      reaction_type: 'paid',
      count: paidTotal,
    })
  }

  return regularReactions
}

interface InteractionsPageProps {
  period: Period
  onPeriodChange: (period: Period) => void
}

export function InteractionsPage({ period, onPeriodChange }: InteractionsPageProps) {
  const [reactionsData, setReactionsData] = useState<ReactionsOverviewResponse | null>(null)
  const [loading, setLoading] = useState(false)

  const fetchData = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return

    setLoading(true)
    try {
      const response = await apiClient.getReactionsOverview(period, 10)
      setReactionsData(response)
    } catch (err) {
      console.error('Failed to fetch reactions overview:', err)
    } finally {
      setLoading(false)
    }
  }, [period])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  // Process reactions: group all paid/custom emojis into one combined item
  const processedReactions = reactionsData?.top_reactions
    ? groupPaidReactions(reactionsData.top_reactions)
    : []

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Interactions</h1>
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
        ) : processedReactions.length ? (
          <div className="reactions-grid">
            {processedReactions.map((reaction) => (
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
        ) : reactionsData?.top_givers.length ? (
          <UserList users={reactionsData.top_givers} />
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
        ) : reactionsData?.top_receivers.length ? (
          <UserList users={reactionsData.top_receivers} />
        ) : (
          <div className="empty-list">No data</div>
        )}
      </section>
    </div>
  )
}

function ReactionItem({ reaction }: { reaction: TopReaction }) {
  const isPaid = reaction.reaction_type === 'custom_emoji' || reaction.reaction_type === 'paid'

  return (
    <div className="reaction-item">
      {isPaid ? (
        <div className="reaction-emoji-paid">
          <span className="paid-label-line">PAID</span>
          <span className="paid-label-line">EMOJI</span>
        </div>
      ) : (
        <div className="reaction-emoji">{reaction.emoji}</div>
      )}
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
