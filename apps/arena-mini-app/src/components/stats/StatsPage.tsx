import { useState, useEffect, useCallback } from 'react'
import { apiClient } from '../../api/client'
import { LoadingSpinner } from '../common/LoadingSpinner'
import { ErrorDisplay } from '../common/ErrorDisplay'
import type { ProfileStats, LeaderboardEntry, MatchHistoryEntry, H2HRecord } from '../../types'

// Stats sub-tabs
type StatsSubTab = 'leaderboard' | 'profile' | 'history' | 'h2h'

interface StatsPageProps {
  currentUserId?: number
}

// Sub-tab configuration
const SUB_TABS: { id: StatsSubTab; label: string; icon: string }[] = [
  { id: 'leaderboard', label: 'Ranks', icon: '🏆' },
  { id: 'profile', label: 'Profile', icon: '👤' },
  { id: 'history', label: 'History', icon: '📜' },
  { id: 'h2h', label: 'H2H', icon: '⚔️' },
]

export function StatsPage({ currentUserId }: StatsPageProps) {
  const [activeSubTab, setActiveSubTab] = useState<StatsSubTab>('leaderboard')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Leaderboard state
  const [leaderboardType, setLeaderboardType] = useState<'ranked' | 'regular'>('ranked')
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([])

  // Profile state
  const [profile, setProfile] = useState<ProfileStats | null>(null)
  const [recentMatches, setRecentMatches] = useState<MatchHistoryEntry[]>([])

  // History state
  const [matchHistory, setMatchHistory] = useState<MatchHistoryEntry[]>([])
  const [historyHasMore, setHistoryHasMore] = useState(false)
  const [historyLoading, setHistoryLoading] = useState(false)

  // H2H state
  const [h2hSearchQuery, setH2hSearchQuery] = useState('')
  const [h2hRecord, setH2hRecord] = useState<H2HRecord | null>(null)
  const [h2hMatches, setH2hMatches] = useState<MatchHistoryEntry[]>([])
  const [h2hLoading, setH2hLoading] = useState(false)
  const [h2hSearched, setH2hSearched] = useState(false)

  // Load initial data
  const loadInitialData = useCallback(async () => {
    setLoading(true)
    setError(null)

    try {
      // Load leaderboard and profile in parallel
      const [leaderboardRes, profileRes] = await Promise.all([
        apiClient.getLeaderboard(leaderboardType, 50, 0),
        apiClient.getProfile(),
      ])

      setLeaderboard(leaderboardRes.entries || [])
      setProfile(profileRes.profile)
      setRecentMatches(profileRes.recent_matches || [])
    } catch (err) {
      console.error('Failed to load stats:', err)
      setError(err instanceof Error ? err.message : 'Failed to load stats')
    } finally {
      setLoading(false)
    }
  }, [leaderboardType])

  // Load leaderboard when type changes
  const loadLeaderboard = useCallback(async (type: 'ranked' | 'regular') => {
    try {
      const res = await apiClient.getLeaderboard(type, 50, 0)
      setLeaderboard(res.entries || [])
    } catch (err) {
      console.error('Failed to load leaderboard:', err)
    }
  }, [])

  // Load match history with pagination
  const loadMatchHistory = useCallback(async (reset: boolean = false) => {
    setHistoryLoading(true)
    try {
      const offset = reset ? 0 : matchHistory.length
      const res = await apiClient.getMatchHistory(20, offset)

      if (reset) {
        setMatchHistory(res.matches || [])
      } else {
        setMatchHistory(prev => [...prev, ...(res.matches || [])])
      }
      setHistoryHasMore(res.has_more)
    } catch (err) {
      console.error('Failed to load match history:', err)
    } finally {
      setHistoryLoading(false)
    }
  }, [matchHistory.length])

  // Search H2H record
  const searchH2H = useCallback(async () => {
    if (!h2hSearchQuery.trim()) return

    setH2hLoading(true)
    setH2hSearched(true)
    try {
      // Try to parse as user ID or search by username
      const opponentId = parseInt(h2hSearchQuery, 10)
      if (isNaN(opponentId)) {
        setH2hRecord(null)
        setH2hMatches([])
        return
      }

      const res = await apiClient.getH2HRecord(opponentId)
      setH2hRecord(res.record)
      setH2hMatches(res.recent_matches || [])
    } catch (err) {
      console.error('Failed to load H2H record:', err)
      setH2hRecord(null)
      setH2hMatches([])
    } finally {
      setH2hLoading(false)
    }
  }, [h2hSearchQuery])

  // Initial load
  useEffect(() => {
    loadInitialData()
  }, [])

  // Load leaderboard when type changes
  useEffect(() => {
    if (!loading) {
      loadLeaderboard(leaderboardType)
    }
  }, [leaderboardType])

  // Load history when switching to history tab
  useEffect(() => {
    if (activeSubTab === 'history' && matchHistory.length === 0) {
      loadMatchHistory(true)
    }
  }, [activeSubTab])

  if (loading) {
    return <LoadingSpinner message="Loading stats..." />
  }

  if (error) {
    return (
      <ErrorDisplay
        title="Failed to Load Stats"
        message={error}
        onRetry={loadInitialData}
      />
    )
  }

  // Render sub-tab navigation
  const renderSubTabs = () => (
    <div className="stats-subtabs">
      {SUB_TABS.map(tab => (
        <button
          key={tab.id}
          className={`stats-subtab ${activeSubTab === tab.id ? 'active' : ''}`}
          onClick={() => setActiveSubTab(tab.id)}
        >
          <span className="stats-subtab-icon">{tab.icon}</span>
          <span className="stats-subtab-label">{tab.label}</span>
        </button>
      ))}
    </div>
  )

  // Render leaderboard tab
  const renderLeaderboard = () => (
    <div className="stats-section">
      <div className="toggle-group">
        <button
          className={`toggle-btn ${leaderboardType === 'ranked' ? 'active' : ''}`}
          onClick={() => setLeaderboardType('ranked')}
        >
          Ranked
        </button>
        <button
          className={`toggle-btn ${leaderboardType === 'regular' ? 'active' : ''}`}
          onClick={() => setLeaderboardType('regular')}
        >
          Casual
        </button>
      </div>

      <div className="leaderboard-list">
        {leaderboard.length === 0 ? (
          <div className="empty-state">
            <div className="empty-state-icon">🏆</div>
            <h3 className="empty-state-title">No Rankings Yet</h3>
            <p className="empty-state-text">
              Play some matches to appear on the leaderboard!
            </p>
          </div>
        ) : (
          leaderboard.map((entry, index) => (
            <LeaderboardEntryRow
              key={entry.user_id}
              entry={entry}
              rank={index + 1}
              isCurrentUser={entry.user_id === currentUserId}
            />
          ))
        )}
      </div>
    </div>
  )

  // Render profile tab
  const renderProfile = () => (
    <div className="stats-section">
      {!profile ? (
        <div className="empty-state">
          <div className="empty-state-icon">👤</div>
          <h3 className="empty-state-title">No Stats Yet</h3>
          <p className="empty-state-text">
            Play your first match to start tracking your stats!
          </p>
        </div>
      ) : (
        <>
          <ProfileHeader profile={profile} />

          <div className="profile-section">
            <h3 className="section-title">Ranked Stats</h3>
            <div className="stats-grid">
              <StatCard icon="🏆" value={profile.ranked_wins} label="Wins" />
              <StatCard icon="💔" value={profile.ranked_losses} label="Losses" />
              <StatCard icon="🔥" value={profile.ranked_current_streak} label="Streak" />
              <StatCard icon="⭐" value={profile.ranked_best_streak} label="Best" />
            </div>
          </div>

          <div className="profile-section">
            <h3 className="section-title">Casual Stats</h3>
            <div className="stats-grid">
              <StatCard icon="🏆" value={profile.regular_wins} label="Wins" />
              <StatCard icon="💔" value={profile.regular_losses} label="Losses" />
              <StatCard icon="🔥" value={profile.regular_current_streak} label="Streak" />
              <StatCard icon="⭐" value={profile.regular_best_streak} label="Best" />
            </div>
          </div>

          {recentMatches.length > 0 && (
            <div className="profile-section">
              <h3 className="section-title">Recent Matches</h3>
              {recentMatches.slice(0, 3).map(match => (
                <MatchHistoryRow key={match.match_id} match={match} />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )

  // Render history tab
  const renderHistory = () => (
    <div className="stats-section">
      {matchHistory.length === 0 && !historyLoading ? (
        <div className="empty-state">
          <div className="empty-state-icon">📜</div>
          <h3 className="empty-state-title">No Match History</h3>
          <p className="empty-state-text">
            Your completed matches will appear here.
          </p>
        </div>
      ) : (
        <>
          <div className="history-list">
            {matchHistory.map(match => (
              <MatchHistoryRow key={match.match_id} match={match} />
            ))}
          </div>

          {historyHasMore && (
            <button
              className="load-more-btn"
              onClick={() => loadMatchHistory(false)}
              disabled={historyLoading}
            >
              {historyLoading ? (
                <>
                  <span className="btn-spinner" />
                  Loading...
                </>
              ) : (
                'Load More'
              )}
            </button>
          )}
        </>
      )}
    </div>
  )

  // Render H2H tab
  const renderH2H = () => (
    <div className="stats-section">
      <div className="h2h-search">
        <input
          type="text"
          className="h2h-input"
          placeholder="Enter opponent's user ID..."
          value={h2hSearchQuery}
          onChange={(e) => setH2hSearchQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && searchH2H()}
        />
        <button
          className="h2h-search-btn"
          onClick={searchH2H}
          disabled={h2hLoading || !h2hSearchQuery.trim()}
        >
          {h2hLoading ? <span className="btn-spinner" /> : '🔍'}
        </button>
      </div>

      {h2hSearched && !h2hLoading && (
        <>
          {!h2hRecord ? (
            <div className="empty-state">
              <div className="empty-state-icon">⚔️</div>
              <h3 className="empty-state-title">No Record Found</h3>
              <p className="empty-state-text">
                No matches found against this opponent.
              </p>
            </div>
          ) : (
            <>
              <H2HRecordDisplay record={h2hRecord} />

              {h2hMatches.length > 0 && (
                <div className="h2h-matches">
                  <h3 className="section-title">Recent Matches</h3>
                  {h2hMatches.map(match => (
                    <MatchHistoryRow key={match.match_id} match={match} />
                  ))}
                </div>
              )}
            </>
          )}
        </>
      )}

      {!h2hSearched && (
        <div className="empty-state">
          <div className="empty-state-icon">🔍</div>
          <h3 className="empty-state-title">Search Opponent</h3>
          <p className="empty-state-text">
            Enter an opponent's user ID to see your head-to-head record.
          </p>
        </div>
      )}
    </div>
  )

  // Render active sub-tab content
  const renderContent = () => {
    switch (activeSubTab) {
      case 'leaderboard':
        return renderLeaderboard()
      case 'profile':
        return renderProfile()
      case 'history':
        return renderHistory()
      case 'h2h':
        return renderH2H()
      default:
        return null
    }
  }

  return (
    <div className="stats-page">
      {renderSubTabs()}
      {renderContent()}
    </div>
  )
}

// Helper Components

interface LeaderboardEntryRowProps {
  entry: LeaderboardEntry
  rank: number
  isCurrentUser: boolean
}

function LeaderboardEntryRow({ entry, rank, isCurrentUser }: LeaderboardEntryRowProps) {
  const getRankClass = () => {
    if (rank === 1) return 'top-1'
    if (rank === 2) return 'top-2'
    if (rank === 3) return 'top-3'
    return ''
  }

  const getRankColor = () => {
    if (rank === 1) return 'gold'
    if (rank === 2) return 'silver'
    if (rank === 3) return 'bronze'
    return ''
  }

  const winRate = entry.matches_played > 0
    ? Math.round((entry.wins / entry.matches_played) * 100)
    : 0

  return (
    <div className={`leaderboard-entry ${getRankClass()} ${isCurrentUser ? 'current-user' : ''}`}>
      <span className={`leaderboard-rank ${getRankColor()}`}>
        {rank <= 3 ? ['🥇', '🥈', '🥉'][rank - 1] : `#${rank}`}
      </span>
      <div className="leaderboard-avatar">
        {entry.first_name.charAt(0).toUpperCase()}
      </div>
      <div className="leaderboard-info">
        <div className="leaderboard-name">
          {entry.first_name}
          {isCurrentUser && <span className="you-badge">You</span>}
        </div>
        <div className="leaderboard-stats">
          {entry.wins}W - {entry.losses}L ({winRate}%)
        </div>
      </div>
      <div className="leaderboard-points">
        🔥 {entry.current_streak}
      </div>
    </div>
  )
}

interface ProfileHeaderProps {
  profile: ProfileStats
}

function ProfileHeader({ profile }: ProfileHeaderProps) {
  const totalMatches = profile.ranked_tournaments_played + profile.regular_matches_played
  const totalWins = profile.ranked_wins + profile.regular_wins

  return (
    <div className="profile-header">
      <div className="profile-avatar">
        {profile.first_name.charAt(0).toUpperCase()}
      </div>
      <div className="profile-info">
        <h2 className="profile-name">{profile.first_name}</h2>
        {profile.username && (
          <p className="profile-username">@{profile.username}</p>
        )}
        <p className="profile-summary">
          {totalWins} wins in {totalMatches} matches
        </p>
      </div>
      <div className="profile-ranks">
        {profile.ranked_rank > 0 && (
          <div className="profile-rank-badge ranked">
            #{profile.ranked_rank} Ranked
          </div>
        )}
        {profile.regular_rank > 0 && (
          <div className="profile-rank-badge casual">
            #{profile.regular_rank} Casual
          </div>
        )}
      </div>
    </div>
  )
}

interface StatCardProps {
  icon: string
  value: number
  label: string
}

function StatCard({ icon, value, label }: StatCardProps) {
  return (
    <div className="stat-card">
      <div className="stat-card-icon">{icon}</div>
      <div className="stat-card-value">{value}</div>
      <div className="stat-card-label">{label}</div>
    </div>
  )
}

interface MatchHistoryRowProps {
  match: MatchHistoryEntry
}

function MatchHistoryRow({ match }: MatchHistoryRowProps) {
  const resultLabel = match.result === 'win' ? 'W' : match.result === 'loss' ? 'L' : 'D'
  const resultIcon = match.result === 'win' ? '🏆' : match.result === 'loss' ? '💔' : '🤝'

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    const now = new Date()
    const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24))

    if (diffDays === 0) return 'Today'
    if (diffDays === 1) return 'Yesterday'
    if (diffDays < 7) return `${diffDays}d ago`
    return date.toLocaleDateString()
  }

  return (
    <div className="history-card">
      <div className={`history-result ${match.result}`}>
        {resultLabel}
      </div>
      <div className="history-info">
        <div className="history-opponent">
          vs {match.opponent.first_name}
        </div>
        <div className="history-date">
          {resultIcon} {formatDate(match.completed_at)}
        </div>
      </div>
      <div className="history-details">
        <div className="history-type">
          {match.match_type === 'ranked' ? '🏅' : '⚔️'}
        </div>
      </div>
    </div>
  )
}

interface H2HRecordDisplayProps {
  record: H2HRecord
}

function H2HRecordDisplay({ record }: H2HRecordDisplayProps) {
  const total = record.wins + record.losses
  const winRate = total > 0 ? Math.round((record.wins / total) * 100) : 0

  return (
    <div className="h2h-record">
      <div className="h2h-opponent">
        <div className="h2h-opponent-avatar">
          {record.opponent.first_name.charAt(0).toUpperCase()}
        </div>
        <div className="h2h-opponent-info">
          <h3>{record.opponent.first_name}</h3>
          {record.opponent.username && (
            <p>@{record.opponent.username}</p>
          )}
        </div>
      </div>

      <div className="h2h-stats">
        <div className="h2h-stat wins">
          <span className="h2h-stat-value">{record.wins}</span>
          <span className="h2h-stat-label">Wins</span>
        </div>
        <div className="h2h-stat separator">
          <span className="h2h-stat-value">-</span>
        </div>
        <div className="h2h-stat losses">
          <span className="h2h-stat-value">{record.losses}</span>
          <span className="h2h-stat-label">Losses</span>
        </div>
      </div>

      <div className="h2h-winrate">
        {winRate}% Win Rate
      </div>
    </div>
  )
}

export default StatsPage
