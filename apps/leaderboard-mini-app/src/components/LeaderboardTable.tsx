import type { LeaderboardUser, LeaderboardMetric } from '../types'

interface LeaderboardTableProps {
  users: LeaderboardUser[]
  total: number
  page: number
  limit: number
  metric: LeaderboardMetric
  loading: boolean
  onMetricChange: (metric: LeaderboardMetric) => void
  onPageChange: (page: number) => void
}

const METRICS: { value: LeaderboardMetric; label: string }[] = [
  { value: 'message_count', label: 'Messages' },
  { value: 'reactions_sent', label: 'Sent' },
  { value: 'reactions_received', label: 'Received' },
  { value: 'active_days', label: 'Days' },
]

function getRankClass(rank: number): string {
  switch (rank) {
    case 1:
      return 'gold'
    case 2:
      return 'silver'
    case 3:
      return 'bronze'
    default:
      return 'default'
  }
}

function formatScore(score: number): string {
  if (score >= 1000) {
    return (score / 1000).toFixed(1) + 'K'
  }
  return score.toLocaleString()
}

function getUserDisplayName(user: LeaderboardUser): string {
  const parts = [user.first_name]
  if (user.last_name) {
    parts.push(user.last_name)
  }
  return parts.join(' ')
}

export function LeaderboardTable({
  users,
  total,
  page,
  limit,
  metric,
  loading,
  onMetricChange,
  onPageChange,
}: LeaderboardTableProps) {
  const totalPages = Math.ceil(total / limit)

  return (
    <div className="leaderboard-section">
      <h2 className="section-title">Leaderboard</h2>

      <div className="metric-tabs">
        {METRICS.map((m) => (
          <button
            key={m.value}
            className={`metric-tab ${metric === m.value ? 'active' : ''}`}
            onClick={() => onMetricChange(m.value)}
          >
            {m.label}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="leaderboard-list">
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} className="leaderboard-item">
              <div className="skeleton skeleton-row" style={{ width: '100%' }} />
            </div>
          ))}
        </div>
      ) : users.length === 0 ? (
        <div className="empty-state">No users found for this period</div>
      ) : (
        <>
          <div className="leaderboard-list">
            {users.map((user) => (
              <div key={user.user_id} className="leaderboard-item">
                <div className={`rank ${getRankClass(user.rank)}`}>{user.rank}</div>
                <div className="user-info">
                  <div className="user-name">{getUserDisplayName(user)}</div>
                  {user.username && (
                    <div className="user-username">@{user.username}</div>
                  )}
                </div>
                <div className="user-score">{formatScore(user.score)}</div>
              </div>
            ))}
          </div>

          {totalPages > 1 && (
            <div className="pagination">
              <button
                className="pagination-btn"
                disabled={page <= 1}
                onClick={() => onPageChange(page - 1)}
              >
                Prev
              </button>
              <span className="pagination-info">
                {page} / {totalPages}
              </span>
              <button
                className="pagination-btn"
                disabled={page >= totalPages}
                onClick={() => onPageChange(page + 1)}
              >
                Next
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
