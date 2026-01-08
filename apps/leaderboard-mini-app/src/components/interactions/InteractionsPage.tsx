import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '../../newrelic'
import { PeriodSelector } from '../PeriodSelector'
import { Avatar } from '../common'

import type {
  Period,
  ReactionsOverviewResponse,
  RepliesOverviewResponse,
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

// Prefetched data passed from App (loaded during splash screen)
interface PrefetchedInteractionsData {
  reactions: ReactionsOverviewResponse | null
  replies: RepliesOverviewResponse | null
}

interface InteractionsPageProps {
  period: Period
  onPeriodChange: (period: Period) => void
  chatTitle: string | null
  prefetchedData?: PrefetchedInteractionsData | null
  onPrefetchConsumed?: () => void
}

export function InteractionsPage({
  period,
  onPeriodChange,
  chatTitle,
  prefetchedData,
  onPrefetchConsumed,
}: InteractionsPageProps) {
  const [reactionsData, setReactionsData] = useState<ReactionsOverviewResponse | null>(null)
  const [repliesData, setRepliesData] = useState<RepliesOverviewResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchData = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return

    setLoading(true)
    setError(null)
    try {
      const [reactionsResponse, repliesResponse] = await Promise.all([
        apiClient.getReactionsOverview(period, 10),
        apiClient.getRepliesOverview(period, 10),
      ])
      setReactionsData(reactionsResponse)
      setRepliesData(repliesResponse)
      addPageAction('interactions_loaded', {
        period,
        top_reactions_count: reactionsResponse.top_reactions?.length || 0,
      })
    } catch (err) {
      console.error('Failed to fetch interactions data:', err)
      setError('Failed to load interactions')
      if (err instanceof Error) {
        noticeError(err, { context: 'interactions_fetch', period })
      }
    } finally {
      setLoading(false)
    }
  }, [period])

  // Use prefetched data if available, otherwise fetch
  useEffect(() => {
    if (prefetchedData) {
      setReactionsData(prefetchedData.reactions)
      setRepliesData(prefetchedData.replies)
      onPrefetchConsumed?.()
      return
    }
    fetchData()
  }, [period, prefetchedData, onPrefetchConsumed, fetchData])

  // Process reactions: group all paid/custom emojis into one combined item
  const processedReactions = reactionsData?.top_reactions
    ? groupPaidReactions(reactionsData.top_reactions)
    : []

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Interactions</h1>
        {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      {/* Top Reactions */}
      <section className="reactions-section">
        <h2 className="section-title">Top Reactions</h2>
        <p className="section-subtitle">Most used reactions in the group</p>
        {loading ? (
          <div className="reactions-grid">
            {[...Array(10)].map((_, i) => (
              <div key={i} className="reaction-item skeleton" style={{ height: 72 }} />
            ))}
          </div>
        ) : error ? (
          <div className="section-error">
            <p className="section-error-message">{error}</p>
            <button className="section-error-btn" onClick={fetchData}>
              Retry
            </button>
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

      {/* Top Reaction Givers */}
      <section className="reactions-section">
        <h2 className="section-title">Top Reaction Givers</h2>
        <p className="section-subtitle">Users who give the most reactions</p>
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

      {/* Top Reaction Receivers */}
      <section className="reactions-section">
        <h2 className="section-title">Top Reaction Receivers</h2>
        <p className="section-subtitle">Users who receive the most reactions</p>
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

      {/* Top Replies Sent */}
      <section className="reactions-section">
        <h2 className="section-title">Top Replies Sent</h2>
        <p className="section-subtitle">Users who reply the most to others</p>
        {loading ? (
          <div className="leaderboard-list">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : repliesData?.top_senders.length ? (
          <UserList users={repliesData.top_senders} />
        ) : (
          <div className="empty-list">No data</div>
        )}
      </section>

      {/* Top Replies Received */}
      <section className="reactions-section">
        <h2 className="section-title">Top Replies Received</h2>
        <p className="section-subtitle">Users whose messages get the most replies</p>
        {loading ? (
          <div className="leaderboard-list">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : repliesData?.top_receivers.length ? (
          <UserList users={repliesData.top_receivers} />
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
