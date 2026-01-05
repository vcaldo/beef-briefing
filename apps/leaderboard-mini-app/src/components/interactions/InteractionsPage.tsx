import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { PeriodSelector } from '../PeriodSelector'

import type {
  Period,
  ReactionsOverviewResponse,
  TopReaction,
  ReactionUser,
  TopInteractor,
} from '../../types'

interface InteractionsPageProps {
  period: Period
  onPeriodChange: (period: Period) => void
}

export function InteractionsPage({ period, onPeriodChange }: InteractionsPageProps) {
  const [reactionsData, setReactionsData] = useState<ReactionsOverviewResponse | null>(null)
  const [topRepliers, setTopRepliers] = useState<TopInteractor[]>([])
  const [topRepliedTo, setTopRepliedTo] = useState<TopInteractor[]>([])
  const [loadingReactions, setLoadingReactions] = useState(false)
  const [loadingReplies, setLoadingReplies] = useState(false)

  const fetchData = useCallback(async () => {
    if (!apiClient.isAuthenticated()) return

    // Fetch reactions data
    setLoadingReactions(true)
    try {
      const response = await apiClient.getReactionsOverview(period, 10)
      setReactionsData(response)
    } catch (err) {
      console.error('Failed to fetch reactions overview:', err)
    } finally {
      setLoadingReactions(false)
    }

    // Fetch profile data for reply interactions
    setLoadingReplies(true)
    try {
      const profileResponse = await apiClient.getProfile(period)
      setTopRepliers(profileResponse.top_repliers)
      setTopRepliedTo(profileResponse.top_replied_to)
    } catch (err) {
      console.error('Failed to fetch reply data:', err)
    } finally {
      setLoadingReplies(false)
    }
  }, [period])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>Interactions</h1>
      </header>

      <PeriodSelector selectedPeriod={period} onPeriodChange={onPeriodChange} />

      {/* Top Reactions */}
      <section className="reactions-section">
        <h2 className="section-title">Top Reactions</h2>
        {loadingReactions ? (
          <div className="reactions-grid">
            {[...Array(10)].map((_, i) => (
              <div key={i} className="reaction-item skeleton" style={{ height: 72 }} />
            ))}
          </div>
        ) : reactionsData?.top_reactions.length ? (
          <div className="reactions-grid">
            {reactionsData.top_reactions.map((reaction) => (
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
        {loadingReactions ? (
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
        {loadingReactions ? (
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

      {/* Who You Reply To */}
      <section className="reactions-section">
        <h2 className="section-title">Who You Reply To</h2>
        <p className="section-subtitle">Users you reply to most</p>
        {loadingReplies ? (
          <div className="leaderboard-list">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : topRepliedTo.length ? (
          <InteractorList interactors={topRepliedTo} />
        ) : (
          <div className="empty-list">No replies yet</div>
        )}
      </section>

      {/* Who Replies to You */}
      <section className="reactions-section">
        <h2 className="section-title">Who Replies to You</h2>
        <p className="section-subtitle">Users who reply to your messages</p>
        {loadingReplies ? (
          <div className="leaderboard-list">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="skeleton skeleton-row" />
            ))}
          </div>
        ) : topRepliers.length ? (
          <InteractorList interactors={topRepliers} />
        ) : (
          <div className="empty-list">No replies yet</div>
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
          <span className="paid-label">PAID</span>
          <span className="paid-id">{reaction.emoji}</span>
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

function InteractorList({ interactors }: { interactors: TopInteractor[] }) {
  return (
    <div className="leaderboard-list">
      {interactors.map((interactor) => (
        <div key={interactor.user_id} className="leaderboard-item">
          <div className={`rank ${getRankClass(interactor.rank)}`}>{interactor.rank}</div>
          <div className="user-info">
            <div className="user-name">{interactor.first_name} {interactor.last_name || ''}</div>
            {interactor.username && <div className="user-username">@{interactor.username}</div>}
          </div>
          <div className="user-score">{interactor.score.toLocaleString()}</div>
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
