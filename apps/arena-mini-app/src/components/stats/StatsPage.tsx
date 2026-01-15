/**
 * StatsPage - Main stats view for arena matches
 *
 * Features:
 * - 4 sub-tabs: Leaderboard, Profile, History, H2H
 * - Tab data caching to avoid refetching
 * - Manual refresh only (no automatic polling)
 * - Pagination for leaderboard and history
 */

import { useState, useEffect, useCallback, useRef } from 'react'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { LoadingSpinner } from '../common'

import type {
  StatsSubTab,
  LeaderboardResponse,
  LeaderboardEntry,
  ProfileResponse,
  HistoryResponse,
  MatchHistoryEntry,
  H2HResponse,
} from '../../types'

// Page size for pagination
const PAGE_SIZE = 20

interface StatsPageProps {
  chatId: number
  userId: number
}

export function StatsPage({ chatId, userId }: StatsPageProps) {
  // Sub-tab state
  const [activeSubTab, setActiveSubTab] = useState<StatsSubTab>('leaderboard')

  // Leaderboard state
  const [leaderboardType, setLeaderboardType] = useState<'ranked' | 'casual'>('ranked')
  const [leaderboardData, setLeaderboardData] = useState<LeaderboardResponse | null>(null)
  const [leaderboardPage, setLeaderboardPage] = useState(0)
  const [leaderboardLoading, setLeaderboardLoading] = useState(false)

  // Profile state
  const [profileData, setProfileData] = useState<ProfileResponse | null>(null)
  const [profileLoading, setProfileLoading] = useState(false)

  // History state
  const [historyData, setHistoryData] = useState<HistoryResponse | null>(null)
  const [historyPage, setHistoryPage] = useState(0)
  const [historyLoading, setHistoryLoading] = useState(false)

  // H2H state
  const [h2hData, setH2HData] = useState<H2HResponse | null>(null)
  const [h2hOpponentId, setH2HOpponentId] = useState<number | null>(null)
  const [h2hLoading, setH2HLoading] = useState(false)
  const [h2hSearchQuery, setH2HSearchQuery] = useState('')

  // Error state
  const [error, setError] = useState<string | null>(null)

  // Track mounted state for async cleanup
  const isMountedRef = useRef(true)

  // Clear error after timeout
  useEffect(() => {
    if (error) {
      const timeout = setTimeout(() => setError(null), 5000)
      return () => clearTimeout(timeout)
    }
  }, [error])

  // Cleanup on unmount
  useEffect(() => {
    isMountedRef.current = true
    return () => {
      isMountedRef.current = false
    }
  }, [])

  // Fetch leaderboard data
  const fetchLeaderboard = useCallback(
    async (type: 'ranked' | 'casual' = leaderboardType, page: number = leaderboardPage) => {
      if (!isMountedRef.current) return

      setLeaderboardLoading(true)
      try {
        const data = await apiClient.getLeaderboard(chatId, type, PAGE_SIZE, page * PAGE_SIZE)
        if (!isMountedRef.current) return

        setLeaderboardData(data)
        addPageAction('leaderboard_loaded', {
          type,
          page,
          entries: data.entries.length,
          total: data.total,
        })
      } catch (err) {
        if (!isMountedRef.current) return

        console.error('Failed to fetch leaderboard:', err)
        setError(err instanceof Error ? err.message : 'Failed to load leaderboard')
        if (err instanceof Error) {
          noticeError(err, { context: 'fetch_leaderboard' })
        }
      } finally {
        if (isMountedRef.current) {
          setLeaderboardLoading(false)
        }
      }
    },
    [chatId, leaderboardType, leaderboardPage]
  )

  // Fetch profile data
  const fetchProfile = useCallback(async () => {
    if (!isMountedRef.current) return

    setProfileLoading(true)
    try {
      const data = await apiClient.getProfile(chatId)
      if (!isMountedRef.current) return

      setProfileData(data)
      addPageAction('profile_loaded', {
        user_id: data.user_id,
        tier: data.stats?.tier,
      })
    } catch (err) {
      if (!isMountedRef.current) return

      console.error('Failed to fetch profile:', err)
      setError(err instanceof Error ? err.message : 'Failed to load profile')
      if (err instanceof Error) {
        noticeError(err, { context: 'fetch_profile' })
      }
    } finally {
      if (isMountedRef.current) {
        setProfileLoading(false)
      }
    }
  }, [chatId])

  // Fetch history data
  const fetchHistory = useCallback(
    async (page: number = historyPage) => {
      if (!isMountedRef.current) return

      setHistoryLoading(true)
      try {
        const data = await apiClient.getHistory(chatId, PAGE_SIZE, page * PAGE_SIZE)
        if (!isMountedRef.current) return

        setHistoryData(data)
        addPageAction('history_loaded', {
          page,
          matches: data.matches.length,
          total: data.total,
        })
      } catch (err) {
        if (!isMountedRef.current) return

        console.error('Failed to fetch history:', err)
        setError(err instanceof Error ? err.message : 'Failed to load history')
        if (err instanceof Error) {
          noticeError(err, { context: 'fetch_history' })
        }
      } finally {
        if (isMountedRef.current) {
          setHistoryLoading(false)
        }
      }
    },
    [chatId, historyPage]
  )

  // Fetch H2H data
  const fetchH2H = useCallback(
    async (opponentId: number) => {
      if (!isMountedRef.current) return

      setH2HLoading(true)
      try {
        const data = await apiClient.getH2H(opponentId, chatId)
        if (!isMountedRef.current) return

        setH2HData(data)
        setH2HOpponentId(opponentId)
        addPageAction('h2h_loaded', {
          opponent_id: opponentId,
          total_matches: data.record.total_matches,
        })
      } catch (err) {
        if (!isMountedRef.current) return

        console.error('Failed to fetch H2H:', err)
        setError(err instanceof Error ? err.message : 'Failed to load head-to-head')
        if (err instanceof Error) {
          noticeError(err, { context: 'fetch_h2h' })
        }
      } finally {
        if (isMountedRef.current) {
          setH2HLoading(false)
        }
      }
    },
    [chatId]
  )

  /**
   * Load data when switching to a tab that hasn't been loaded yet.
   *
   * Caching strategy:
   * - Data is cached in component state (leaderboardData, profileData, etc.)
   * - Only fetch if data is null (not yet loaded)
   * - Use the Refresh button for manual updates
   *
   * This avoids unnecessary API calls when switching between tabs,
   * providing a snappier UX. The trade-off is slightly stale data
   * until the user explicitly refreshes.
   *
   * H2H is special: it requires selecting an opponent first,
   * so no auto-fetch occurs when navigating to that tab.
   */
  useEffect(() => {
    switch (activeSubTab) {
      case 'leaderboard':
        // Only fetch if not already cached
        if (!leaderboardData) {
          fetchLeaderboard()
        }
        break
      case 'profile':
        if (!profileData) {
          fetchProfile()
        }
        break
      case 'history':
        if (!historyData) {
          fetchHistory()
        }
        break
      case 'h2h':
        // H2H requires user to select an opponent from Leaderboard/History
        // No auto-fetch - user must click on a player first
        break
    }
  }, [activeSubTab, leaderboardData, profileData, historyData, fetchLeaderboard, fetchProfile, fetchHistory])

  // Handle tab change with tracking
  const handleSubTabChange = useCallback(
    (tab: StatsSubTab) => {
      addPageAction('stats_subtab_change', {
        tab,
        previous_tab: activeSubTab,
      })
      setActiveSubTab(tab)
    },
    [activeSubTab]
  )

  // Handle leaderboard type change
  const handleLeaderboardTypeChange = useCallback(
    (type: 'ranked' | 'casual') => {
      setLeaderboardType(type)
      setLeaderboardPage(0)
      fetchLeaderboard(type, 0)
    },
    [fetchLeaderboard]
  )

  // Handle leaderboard pagination
  const handleLeaderboardPageChange = useCallback(
    (newPage: number) => {
      setLeaderboardPage(newPage)
      fetchLeaderboard(leaderboardType, newPage)
    },
    [fetchLeaderboard, leaderboardType]
  )

  // Handle history pagination
  const handleHistoryPageChange = useCallback(
    (newPage: number) => {
      setHistoryPage(newPage)
      fetchHistory(newPage)
    },
    [fetchHistory]
  )

  // Handle H2H opponent selection from leaderboard
  const handleSelectOpponent = useCallback(
    (opponentId: number) => {
      if (opponentId === userId) return // Can't view H2H with yourself
      setActiveSubTab('h2h')
      fetchH2H(opponentId)
    },
    [userId, fetchH2H]
  )

  // Render rank badge class
  const getRankClass = (rank: number): string => {
    if (rank === 1) return 'gold'
    if (rank === 2) return 'silver'
    if (rank === 3) return 'bronze'
    return 'default'
  }

  // Render match result color
  const getResultClass = (result: 'win' | 'loss' | 'draw'): string => {
    if (result === 'win') return 'result-win'
    if (result === 'loss') return 'result-loss'
    return 'result-draw'
  }

  // Format date
  const formatDate = (dateString: string): string => {
    const date = new Date(dateString)
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  // Render leaderboard tab content
  const renderLeaderboard = () => {
    if (leaderboardLoading && !leaderboardData) {
      return <LoadingSpinner message="Loading leaderboard..." />
    }

    return (
      <div className="stats-content">
        {/* Type toggle */}
        <div className="leaderboard-type-toggle">
          <button
            className={`toggle-btn ${leaderboardType === 'ranked' ? 'active' : ''}`}
            onClick={() => handleLeaderboardTypeChange('ranked')}
          >
            🏆 Ranked
          </button>
          <button
            className={`toggle-btn ${leaderboardType === 'casual' ? 'active' : ''}`}
            onClick={() => handleLeaderboardTypeChange('casual')}
          >
            ⚔️ Casual
          </button>
        </div>

        {/* Leaderboard list */}
        {leaderboardData && leaderboardData.entries.length > 0 ? (
          <>
            <div className="leaderboard-list">
              {leaderboardData.entries.map((entry: LeaderboardEntry) => (
                <div
                  key={entry.user_id}
                  className={`leaderboard-item ${entry.user_id === userId ? 'current-user' : ''}`}
                  onClick={() => handleSelectOpponent(entry.user_id)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), handleSelectOpponent(entry.user_id))}
                >
                  <div className={`leaderboard-rank ${getRankClass(entry.rank)}`}>
                    {entry.rank}
                  </div>
                  <div className="leaderboard-user">
                    <div className="leaderboard-name">
                      {entry.first_name}
                      {entry.user_id === userId && <span className="you-badge">You</span>}
                    </div>
                    {entry.tier && <div className="leaderboard-tier">{entry.tier}</div>}
                  </div>
                  <div className="leaderboard-stats-mini">
                    <span className="wins">{leaderboardType === 'ranked' ? entry.ranked_wins : entry.regular_wins}W</span>
                    <span className="losses">{leaderboardType === 'ranked' ? entry.ranked_losses : entry.regular_losses}L</span>
                  </div>
                  <div className="leaderboard-score">{entry.score}</div>
                </div>
              ))}
            </div>

            {/* Pagination */}
            {leaderboardData.total > PAGE_SIZE && (
              <div className="pagination">
                <button
                  className="pagination-btn"
                  disabled={leaderboardPage === 0 || leaderboardLoading}
                  onClick={() => handleLeaderboardPageChange(leaderboardPage - 1)}
                >
                  ← Prev
                </button>
                <span className="pagination-info">
                  Page {leaderboardPage + 1} of {Math.ceil(leaderboardData.total / PAGE_SIZE)}
                </span>
                <button
                  className="pagination-btn"
                  disabled={(leaderboardPage + 1) * PAGE_SIZE >= leaderboardData.total || leaderboardLoading}
                  onClick={() => handleLeaderboardPageChange(leaderboardPage + 1)}
                >
                  Next →
                </button>
              </div>
            )}
          </>
        ) : (
          <div className="empty-state">
            <span className="empty-icon">📊</span>
            <p>No leaderboard data yet</p>
            <p className="empty-hint">Play some matches to appear on the leaderboard!</p>
          </div>
        )}

        {/* Refresh button */}
        <button
          className="refresh-btn"
          onClick={() => fetchLeaderboard()}
          disabled={leaderboardLoading}
        >
          {leaderboardLoading ? <LoadingSpinner size="sm" inline /> : '🔄'} Refresh
        </button>
      </div>
    )
  }

  // Render profile tab content
  const renderProfile = () => {
    if (profileLoading && !profileData) {
      return <LoadingSpinner message="Loading profile..." />
    }

    if (!profileData) {
      return (
        <div className="empty-state">
          <span className="empty-icon">👤</span>
          <p>No profile data</p>
          <button className="btn-primary" onClick={fetchProfile}>
            Load Profile
          </button>
        </div>
      )
    }

    const { stats } = profileData

    return (
      <div className="stats-content">
        {/* Profile header */}
        <div className="profile-header">
          <div className="profile-avatar">
            {profileData.photo_url ? (
              <img src={profileData.photo_url} alt={profileData.first_name} />
            ) : (
              <span className="avatar-placeholder">👤</span>
            )}
          </div>
          <div className="profile-info">
            <h2 className="profile-name">{profileData.first_name}</h2>
            {profileData.username && <p className="profile-username">@{profileData.username}</p>}
            {stats.tier && (
              <span className={`profile-tier tier-${stats.tier.toLowerCase().replace(/\s+/g, '-')}`}>
                {stats.tier}
              </span>
            )}
            {stats.rank && <p className="profile-rank">Rank #{stats.rank}</p>}
          </div>
        </div>

        {/* Overall stats */}
        <div className="profile-section">
          <h3 className="profile-section-title">Overall</h3>
          <div className="stats-grid">
            <div className="stat-card">
              <span className="stat-value">{stats.total_matches}</span>
              <span className="stat-label">Matches</span>
            </div>
            <div className="stat-card">
              <span className="stat-value">{stats.total_wins}</span>
              <span className="stat-label">Wins</span>
            </div>
            <div className="stat-card">
              <span className="stat-value">{stats.total_damage_dealt.toLocaleString()}</span>
              <span className="stat-label">Damage</span>
            </div>
          </div>
        </div>

        {/* Ranked stats */}
        <div className="profile-section">
          <h3 className="profile-section-title">🏆 Ranked</h3>
          <div className="stats-grid">
            <div className="stat-card">
              <span className="stat-value stat-wins">{stats.ranked_wins}</span>
              <span className="stat-label">Wins</span>
            </div>
            <div className="stat-card">
              <span className="stat-value stat-losses">{stats.ranked_losses}</span>
              <span className="stat-label">Losses</span>
            </div>
            <div className="stat-card">
              <span className="stat-value">{stats.ranked_draws}</span>
              <span className="stat-label">Draws</span>
            </div>
            <div className="stat-card">
              <span className="stat-value">{(stats.ranked_win_rate * 100).toFixed(1)}%</span>
              <span className="stat-label">Win Rate</span>
            </div>
          </div>
          <div className="stats-row">
            <div className="stat-mini">
              <span className="stat-mini-value">🔥 {stats.ranked_current_streak}</span>
              <span className="stat-mini-label">Current Streak</span>
            </div>
            <div className="stat-mini">
              <span className="stat-mini-value">⭐ {stats.ranked_best_streak}</span>
              <span className="stat-mini-label">Best Streak</span>
            </div>
            <div className="stat-mini">
              <span className="stat-mini-value">🏅 {stats.ranked_tournaments_won}/{stats.ranked_tournaments_played}</span>
              <span className="stat-mini-label">Tournaments</span>
            </div>
          </div>
        </div>

        {/* Casual stats */}
        <div className="profile-section">
          <h3 className="profile-section-title">⚔️ Casual</h3>
          <div className="stats-grid">
            <div className="stat-card">
              <span className="stat-value stat-wins">{stats.regular_wins}</span>
              <span className="stat-label">Wins</span>
            </div>
            <div className="stat-card">
              <span className="stat-value stat-losses">{stats.regular_losses}</span>
              <span className="stat-label">Losses</span>
            </div>
            <div className="stat-card">
              <span className="stat-value">{stats.regular_draws}</span>
              <span className="stat-label">Draws</span>
            </div>
            <div className="stat-card">
              <span className="stat-value">{(stats.regular_win_rate * 100).toFixed(1)}%</span>
              <span className="stat-label">Win Rate</span>
            </div>
          </div>
          <div className="stats-row">
            <div className="stat-mini">
              <span className="stat-mini-value">🔥 {stats.regular_current_streak}</span>
              <span className="stat-mini-label">Current Streak</span>
            </div>
            <div className="stat-mini">
              <span className="stat-mini-value">⭐ {stats.regular_best_streak}</span>
              <span className="stat-mini-label">Best Streak</span>
            </div>
          </div>
        </div>

        {/* Refresh button */}
        <button className="refresh-btn" onClick={fetchProfile} disabled={profileLoading}>
          {profileLoading ? <LoadingSpinner size="sm" inline /> : '🔄'} Refresh
        </button>
      </div>
    )
  }

  // Render history tab content
  const renderHistory = () => {
    if (historyLoading && !historyData) {
      return <LoadingSpinner message="Loading history..." />
    }

    return (
      <div className="stats-content">
        {historyData && historyData.matches.length > 0 ? (
          <>
            <div className="history-list">
              {historyData.matches.map((match: MatchHistoryEntry) => (
                <div
                  key={match.match_id}
                  className={`history-item ${getResultClass(match.result)}`}
                  onClick={() => handleSelectOpponent(match.opponent_id)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), handleSelectOpponent(match.opponent_id))}
                >
                  <div className="history-result">
                    <span className={`result-badge ${match.result}`}>
                      {match.result === 'win' ? 'W' : match.result === 'loss' ? 'L' : 'D'}
                    </span>
                  </div>
                  <div className="history-details">
                    <div className="history-opponent">vs {match.opponent_name}</div>
                    <div className="history-meta">
                      <span className="history-type">
                        {match.match_type === 'ranked' ? '🏆' : '⚔️'}
                      </span>
                      <span className="history-date">{formatDate(match.played_at)}</span>
                    </div>
                  </div>
                  <div className="history-stats">
                    <span className="damage-dealt" title="Damage dealt">
                      ⚔️ {match.damage_dealt}
                    </span>
                    <span className="damage-received" title="Damage received">
                      💔 {match.damage_received}
                    </span>
                    {match.rating_change !== undefined && match.rating_change !== 0 && (
                      <span className={`rating-change ${match.rating_change > 0 ? 'positive' : 'negative'}`}>
                        {match.rating_change > 0 ? '+' : ''}{match.rating_change}
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>

            {/* Pagination */}
            {historyData.total > PAGE_SIZE && (
              <div className="pagination">
                <button
                  className="pagination-btn"
                  disabled={historyPage === 0 || historyLoading}
                  onClick={() => handleHistoryPageChange(historyPage - 1)}
                >
                  ← Prev
                </button>
                <span className="pagination-info">
                  Page {historyPage + 1} of {Math.ceil(historyData.total / PAGE_SIZE)}
                </span>
                <button
                  className="pagination-btn"
                  disabled={(historyPage + 1) * PAGE_SIZE >= historyData.total || historyLoading}
                  onClick={() => handleHistoryPageChange(historyPage + 1)}
                >
                  Next →
                </button>
              </div>
            )}
          </>
        ) : (
          <div className="empty-state">
            <span className="empty-icon">📜</span>
            <p>No match history yet</p>
            <p className="empty-hint">Play some matches to see your history!</p>
          </div>
        )}

        {/* Refresh button */}
        <button className="refresh-btn" onClick={() => fetchHistory()} disabled={historyLoading}>
          {historyLoading ? <LoadingSpinner size="sm" inline /> : '🔄'} Refresh
        </button>
      </div>
    )
  }

  // Render H2H tab content
  const renderH2H = () => {
    return (
      <div className="stats-content">
        {/* Search/Select opponent */}
        <div className="h2h-search">
          <p className="h2h-hint">
            Tap a player on the Leaderboard or History to view your head-to-head record
          </p>
          {h2hSearchQuery && (
            <input
              type="text"
              className="h2h-search-input"
              placeholder="Search by name or ID..."
              value={h2hSearchQuery}
              onChange={(e) => setH2HSearchQuery(e.target.value)}
            />
          )}
        </div>

        {/* Loading state */}
        {h2hLoading && <LoadingSpinner message="Loading head-to-head..." />}

        {/* H2H data display */}
        {!h2hLoading && h2hData && (
          <div className="h2h-result">
            <div className="h2h-header">
              <div className="h2h-player you">You</div>
              <div className="h2h-vs">VS</div>
              <div className="h2h-player opponent">
                {h2hData.opponent_name}
                {h2hData.opponent_username && (
                  <span className="h2h-username">@{h2hData.opponent_username}</span>
                )}
              </div>
            </div>

            <div className="h2h-record">
              <div className="h2h-stat wins">
                <span className="h2h-stat-value">{h2hData.record.wins}</span>
                <span className="h2h-stat-label">Wins</span>
              </div>
              <div className="h2h-stat draws">
                <span className="h2h-stat-value">{h2hData.record.draws}</span>
                <span className="h2h-stat-label">Draws</span>
              </div>
              <div className="h2h-stat losses">
                <span className="h2h-stat-value">{h2hData.record.losses}</span>
                <span className="h2h-stat-label">Losses</span>
              </div>
            </div>

            <div className="h2h-summary">
              <div className="h2h-summary-item">
                <span className="summary-label">Total Matches</span>
                <span className="summary-value">{h2hData.record.total_matches}</span>
              </div>
              <div className="h2h-summary-item">
                <span className="summary-label">Win Rate</span>
                <span className="summary-value">{(h2hData.record.win_rate * 100).toFixed(1)}%</span>
              </div>
              {h2hData.record.last_played && (
                <div className="h2h-summary-item">
                  <span className="summary-label">Last Played</span>
                  <span className="summary-value">{formatDate(h2hData.record.last_played)}</span>
                </div>
              )}
            </div>
          </div>
        )}

        {/* No opponent selected */}
        {!h2hLoading && !h2hData && !h2hOpponentId && (
          <div className="empty-state">
            <span className="empty-icon">🤝</span>
            <p>Select an opponent</p>
            <p className="empty-hint">Choose a player from Leaderboard or History</p>
          </div>
        )}
      </div>
    )
  }

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
        return renderLeaderboard()
    }
  }

  return (
    <div className="stats-page">
      {/* Sub-tabs */}
      <nav className="stats-tabs" role="tablist">
        <button
          className={`stats-tab ${activeSubTab === 'leaderboard' ? 'active' : ''}`}
          onClick={() => handleSubTabChange('leaderboard')}
          role="tab"
          aria-selected={activeSubTab === 'leaderboard'}
        >
          📊 Leaderboard
        </button>
        <button
          className={`stats-tab ${activeSubTab === 'profile' ? 'active' : ''}`}
          onClick={() => handleSubTabChange('profile')}
          role="tab"
          aria-selected={activeSubTab === 'profile'}
        >
          👤 Profile
        </button>
        <button
          className={`stats-tab ${activeSubTab === 'history' ? 'active' : ''}`}
          onClick={() => handleSubTabChange('history')}
          role="tab"
          aria-selected={activeSubTab === 'history'}
        >
          📜 History
        </button>
        <button
          className={`stats-tab ${activeSubTab === 'h2h' ? 'active' : ''}`}
          onClick={() => handleSubTabChange('h2h')}
          role="tab"
          aria-selected={activeSubTab === 'h2h'}
        >
          🤝 H2H
        </button>
      </nav>

      {/* Error banner */}
      {error && (
        <div className="stats-error-banner" role="alert">
          <span className="stats-error-icon">⚠️</span>
          <span className="stats-error-text">{error}</span>
          <button className="stats-error-dismiss" onClick={() => setError(null)} aria-label="Dismiss error">
            ×
          </button>
        </div>
      )}

      {/* Content */}
      <div className="stats-tab-content" role="tabpanel">
        {renderContent()}
      </div>
    </div>
  )
}

export default StatsPage
