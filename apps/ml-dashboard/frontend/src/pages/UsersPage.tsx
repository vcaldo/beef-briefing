import { useState, useEffect } from 'react'
import { useChat } from '../App'
import { api } from '../api/client'
import type { User, UserProfile, UserCard } from '../types'

function UserAvatar({ user }: { user: User }) {
  const initial = (user.first_name?.[0] || user.username?.[0] || '?').toUpperCase()
  return <div className="user-avatar">{initial}</div>
}

function formatPercent(value: number | null): string {
  if (value === null) return '-'
  return `${(value * 100).toFixed(1)}%`
}

function formatSentiment(value: number | null): string {
  if (value === null) return '-'
  if (value > 0.1) return `+${value.toFixed(2)}`
  if (value < -0.1) return value.toFixed(2)
  return '~0'
}

function UserDetail({
  chatId,
  userId,
  onClose,
}: {
  chatId: number
  userId: number
  onClose: () => void
}) {
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [cards, setCards] = useState<UserCard[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([
      api.getUserProfile(chatId, userId),
      api.getUserCards(chatId, userId),
    ])
      .then(([profileRes, cardsRes]) => {
        setProfile(profileRes)
        setCards(cardsRes.cards)
      })
      .finally(() => setLoading(false))
  }, [chatId, userId])

  if (loading) {
    return (
      <div className="card" style={{ marginBottom: 'var(--spacing-lg)' }}>
        <div className="loading">Loading profile...</div>
      </div>
    )
  }

  return (
    <div className="card" style={{ marginBottom: 'var(--spacing-lg)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 'var(--spacing-md)' }}>
        <div style={{ fontWeight: 600, fontSize: '16px' }}>User Profile</div>
        <button
          onClick={onClose}
          style={{
            background: 'none',
            border: 'none',
            color: 'var(--text-muted)',
            fontSize: '20px',
            cursor: 'pointer',
          }}
        >
          &times;
        </button>
      </div>

      {profile && (
        <>
          {/* Sentiment Distribution */}
          <div style={{ marginBottom: 'var(--spacing-lg)' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
              SENTIMENT DISTRIBUTION
            </div>
            <div style={{ display: 'flex', gap: 'var(--spacing-md)' }}>
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: '18px', color: 'var(--sentiment-positive)' }}>
                  {profile.sentiment_distribution.percentages.positive || 0}%
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Positive</div>
              </div>
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: '18px', color: 'var(--sentiment-neutral)' }}>
                  {profile.sentiment_distribution.percentages.neutral || 0}%
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Neutral</div>
              </div>
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontFamily: 'var(--font-mono)', fontSize: '18px', color: 'var(--sentiment-negative)' }}>
                  {profile.sentiment_distribution.percentages.negative || 0}%
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Negative</div>
              </div>
            </div>
          </div>

          {/* Entity Mentions */}
          {profile.entity_mentions.length > 0 && (
            <div style={{ marginBottom: 'var(--spacing-lg)' }}>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
                TOP ENTITY MENTIONS
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                {profile.entity_mentions.slice(0, 10).map((e, i) => (
                  <span key={i} className="badge badge-entity">
                    {e.entity_text} ({e.count})
                  </span>
                ))}
              </div>
            </div>
          )}
        </>
      )}

      {/* Cards History */}
      {cards.length > 0 && (
        <div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
            CARD HISTORY ({cards.length} cards)
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '200px', overflowY: 'auto' }}>
            {cards.map((card) => (
              <div
                key={card.id}
                style={{
                  padding: '8px',
                  backgroundColor: 'var(--bg-tertiary)',
                  borderRadius: '4px',
                  fontSize: '13px',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span>Week of {new Date(card.week_start).toLocaleDateString()}</span>
                  {card.image_url && (
                    <span style={{ color: 'var(--accent-green)' }}>Has image</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

export default function UsersPage() {
  const { chatId } = useChat()
  const [users, setUsers] = useState<User[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null)

  const limit = 50

  useEffect(() => {
    if (!chatId) return
    setLoading(true)
    api
      .getUsers(chatId, limit, offset)
      .then((res) => {
        setUsers(res.users)
        setTotal(res.total)
      })
      .finally(() => setLoading(false))
  }, [chatId, offset])

  if (!chatId) {
    return <div className="loading">Select a chat to view users</div>
  }

  return (
    <div>
      <header className="page-header">
        <h1 className="page-title">User Analytics</h1>
        <p className="page-subtitle">Aggregated ML statistics by user</p>
      </header>

      {/* Selected user detail */}
      {selectedUserId && (
        <UserDetail
          chatId={chatId}
          userId={selectedUserId}
          onClose={() => setSelectedUserId(null)}
        />
      )}

      {/* Users table */}
      {loading ? (
        <div className="loading">Loading users...</div>
      ) : (
        <>
          <div style={{ marginBottom: 'var(--spacing-md)', color: 'var(--text-secondary)', fontSize: '13px' }}>
            Showing {offset + 1}-{Math.min(offset + users.length, total)} of {total} users
          </div>

          <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
            <table className="data-table">
              <thead>
                <tr>
                  <th>User</th>
                  <th>Messages</th>
                  <th>Sentiment</th>
                  <th>Toxicity</th>
                  <th>Humor</th>
                  <th>Questions</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => {
                  const name = user.first_name
                    ? `${user.first_name}${user.last_name ? ' ' + user.last_name : ''}`
                    : user.username || 'Unknown'

                  return (
                    <tr
                      key={user.user_id}
                      onClick={() => setSelectedUserId(user.user_id)}
                      style={{ cursor: 'pointer' }}
                    >
                      <td>
                        <div className="user-row">
                          <UserAvatar user={user} />
                          <div>
                            <div className="user-name">{name}</div>
                            {user.username && (
                              <div className="user-username">@{user.username}</div>
                            )}
                          </div>
                        </div>
                      </td>
                      <td className="mono">{user.message_count.toLocaleString()}</td>
                      <td className="mono" style={{
                        color: (user.avg_sentiment || 0) > 0.1
                          ? 'var(--sentiment-positive)'
                          : (user.avg_sentiment || 0) < -0.1
                            ? 'var(--sentiment-negative)'
                            : 'var(--text-secondary)'
                      }}>
                        {formatSentiment(user.avg_sentiment)}
                      </td>
                      <td className="mono" style={{
                        color: (user.toxicity_rate || 0) > 0.1
                          ? 'var(--accent-red)'
                          : 'var(--text-secondary)'
                      }}>
                        {formatPercent(user.toxicity_rate)}
                      </td>
                      <td className="mono" style={{ color: 'var(--accent-purple)' }}>
                        {formatPercent(user.humor_rate)}
                      </td>
                      <td className="mono" style={{ color: 'var(--accent-blue)' }}>
                        {formatPercent(user.question_rate)}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>

          <div className="pagination">
            <button
              className="pagination-btn"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - limit))}
            >
              Previous
            </button>
            <span className="pagination-info">
              Page {Math.floor(offset / limit) + 1} of {Math.ceil(total / limit)}
            </span>
            <button
              className="pagination-btn"
              disabled={offset + limit >= total}
              onClick={() => setOffset(offset + limit)}
            >
              Next
            </button>
          </div>
        </>
      )}
    </div>
  )
}
