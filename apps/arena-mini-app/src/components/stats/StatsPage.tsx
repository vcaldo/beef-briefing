/**
 * StatsPage - Main stats view for arena matches
 *
 * Features:
 * - 2 sub-tabs: Leaderboard, History
 * - Profile shown as modal overlay when clicking your own leaderboard entry
 * - H2H stats shown as modal overlay when clicking opponents
 * - Tab data caching to avoid refetching
 * - Manual refresh only (no automatic polling)
 * - Pagination for leaderboard and history
 */

import { useState, useEffect, useCallback, useRef } from 'react'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { Avatar } from '@beef-briefing/shared-mini-app/components'
import { LoadingSpinner } from '../common'
import { RPGPanel, GameButton } from '../ui'

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
  const [leaderboardType, setLeaderboardType] = useState<'ranked' | 'casual'>('casual')
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

  // H2H state (now for modal)
  const [h2hData, setH2HData] = useState<H2HResponse | null>(null)
  const [h2hLoading, setH2HLoading] = useState(false)
  const [showH2HModal, setShowH2HModal] = useState(false)

  // Profile modal state
  const [showProfileModal, setShowProfileModal] = useState(false)

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
   */
  useEffect(() => {
    switch (activeSubTab) {
      case 'leaderboard':
        // Only fetch if not already cached
        if (!leaderboardData) {
          fetchLeaderboard()
        }
        break
      case 'history':
        if (!historyData) {
          fetchHistory()
        }
        break
    }
  }, [activeSubTab, leaderboardData, historyData, fetchLeaderboard, fetchHistory])

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

  // Handle opponent selection from leaderboard or history
  const handleSelectOpponent = useCallback(
    (opponentId: number) => {
      if (opponentId === userId) {
        // Clicking own entry opens profile modal
        setShowProfileModal(true)
        if (!profileData) {
          fetchProfile()
        }
        return
      }
      setShowH2HModal(true)
      fetchH2H(opponentId)
    },
    [userId, fetchH2H, profileData, fetchProfile]
  )

  // Handle closing H2H modal
  const handleCloseH2HModal = useCallback(() => {
    setShowH2HModal(false)
    setH2HData(null)
  }, [])

  // Handle closing Profile modal
  const handleCloseProfileModal = useCallback(() => {
    setShowProfileModal(false)
  }, [])

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
      <>
        {/* Type toggle - outside the panel */}
        <div className="rpg-type-toggle">
          <GameButton
            variant={leaderboardType === 'casual' ? 'primary' : 'neutral'}
            size="sm"
            onClick={() => handleLeaderboardTypeChange('casual')}
          >
            ⚔️ Casual
          </GameButton>
          <GameButton
            variant={leaderboardType === 'ranked' ? 'primary' : 'neutral'}
            size="sm"
            onClick={() => handleLeaderboardTypeChange('ranked')}
          >
            🏆 Ranked
          </GameButton>
        </div>

        <RPGPanel variant="outer" className="stats-outer-panel">
          {/* Leaderboard list */}
          {leaderboardData && leaderboardData.entries.length > 0 ? (
            <RPGPanel variant="inner" className="leaderboard-inner-panel">
              <div className="rpg-leaderboard-list">
                {leaderboardData.entries.map((entry: LeaderboardEntry) => (
                  <div
                    key={entry.user_id}
                    className={`rpg-leaderboard-item ${entry.user_id === userId ? 'current-user' : ''} ${getRankClass(entry.rank)}`}
                    onClick={() => handleSelectOpponent(entry.user_id)}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), handleSelectOpponent(entry.user_id))}
                  >
                    <Avatar
                      firstName={entry.first_name}
                      photoUrl={entry.photo_url}
                      size="small"
                    />
                    <div className="rpg-leaderboard-user">
                      <div className="rpg-leaderboard-name">
                        <span className={`rpg-rank-text ${getRankClass(entry.rank)}`}>#{entry.rank}</span>
                        {entry.first_name}
                        {entry.user_id === userId && <span className="rpg-you-badge">You</span>}
                      </div>
                      {entry.tier && <div className="rpg-leaderboard-tier">{entry.tier}</div>}
                    </div>
                    <div className="rpg-leaderboard-stats">
                      <span className="wins">{leaderboardType === 'ranked' ? entry.ranked_wins : entry.regular_wins}W</span>
                      <span className="losses">{leaderboardType === 'ranked' ? entry.ranked_losses : entry.regular_losses}L</span>
                    </div>
                    <div className="rpg-leaderboard-score">{entry.score}</div>
                  </div>
                ))}
              </div>

              {/* Pagination */}
              {leaderboardData.total > PAGE_SIZE && (
                <div className="rpg-pagination">
                  <GameButton
                    variant="neutral"
                    size="sm"
                    disabled={leaderboardPage === 0 || leaderboardLoading}
                    onClick={() => handleLeaderboardPageChange(leaderboardPage - 1)}
                  >
                    ← Prev
                  </GameButton>
                  <span className="rpg-pagination-info">
                    Page {leaderboardPage + 1} of {Math.ceil(leaderboardData.total / PAGE_SIZE)}
                  </span>
                  <GameButton
                    variant="neutral"
                    size="sm"
                    disabled={(leaderboardPage + 1) * PAGE_SIZE >= leaderboardData.total || leaderboardLoading}
                    onClick={() => handleLeaderboardPageChange(leaderboardPage + 1)}
                  >
                    Next →
                  </GameButton>
                </div>
              )}
            </RPGPanel>
          ) : (
            <div className="rpg-empty-state">
              <span className="rpg-empty-icon">📊</span>
              <p className="rpg-empty-title">No leaderboard data yet</p>
              <p className="rpg-empty-hint">Play some matches to appear on the leaderboard!</p>
            </div>
          )}
        </RPGPanel>
      </>
    )
  }

  // Render profile tab content
  const renderProfile = () => {
    if (profileLoading && !profileData) {
      return <LoadingSpinner message="Loading profile..." />
    }

    if (!profileData) {
      return (
        <RPGPanel variant="outer" className="stats-outer-panel">
          <div className="rpg-empty-state">
            <span className="rpg-empty-icon">👤</span>
            <p className="rpg-empty-title">No profile data</p>
            <GameButton variant="primary" size="sm" onClick={fetchProfile}>
              Load Profile
            </GameButton>
          </div>
        </RPGPanel>
      )
    }

    const { stats } = profileData

    // Check if stats data is available
    if (!stats) {
      return (
        <RPGPanel variant="outer" className="stats-outer-panel">
          <div className="rpg-empty-state">
            <span className="rpg-empty-icon">📊</span>
            <p className="rpg-empty-title">No statistics available</p>
            <p className="rpg-empty-hint">Play some matches to build your stats!</p>
            <GameButton variant="primary" size="sm" onClick={fetchProfile}>
              Refresh
            </GameButton>
          </div>
        </RPGPanel>
      )
    }

    return (
      <RPGPanel variant="outer" className="profile-outer-panel">
        {/* Profile header */}
        <RPGPanel variant="inner" className="rpg-profile-header-panel">
          <div className="rpg-profile-header">
            <Avatar
              firstName={profileData.first_name}
              photoUrl={profileData.photo_url}
              size="large"
            />
            <div className="rpg-profile-info">
              <h2 className="rpg-profile-name">{profileData.first_name}</h2>
              {profileData.username && <p className="rpg-profile-username">@{profileData.username}</p>}
              {stats.tier && (
                <span className={`rpg-profile-tier tier-${stats.tier.toLowerCase().replace(/\s+/g, '-')}`}>
                  {stats.tier}
                </span>
              )}
              {stats.rank && <p className="rpg-profile-rank">Rank #{stats.rank}</p>}
            </div>
          </div>
        </RPGPanel>

        {/* Overall stats - blue panel for emphasis */}
        <RPGPanel variant="inner-blue" className="rpg-section-panel">
          <h3 className="rpg-section-title">Overall</h3>
          <div className="rpg-stats-grid">
            <div className="rpg-stat-card">
              <span className="rpg-stat-value">{stats.total_matches}</span>
              <span className="rpg-stat-label">Matches</span>
            </div>
            <div className="rpg-stat-card">
              <span className="rpg-stat-value">{stats.total_wins}</span>
              <span className="rpg-stat-label">Wins</span>
            </div>
            <div className="rpg-stat-card">
              <span className="rpg-stat-value">{stats.total_damage_dealt.toLocaleString()}</span>
              <span className="rpg-stat-label">Damage</span>
            </div>
          </div>
        </RPGPanel>

        {/* Ranked stats */}
        <RPGPanel variant="inner" className="rpg-section-panel">
          <h3 className="rpg-section-title">🏆 Ranked</h3>
          <div className="rpg-stats-grid">
            <div className="rpg-stat-card">
              <span className="rpg-stat-value stat-wins">{stats.ranked_wins}</span>
              <span className="rpg-stat-label">Wins</span>
            </div>
            <div className="rpg-stat-card">
              <span className="rpg-stat-value stat-losses">{stats.ranked_losses}</span>
              <span className="rpg-stat-label">Losses</span>
            </div>
            <div className="rpg-stat-card">
              <span className="rpg-stat-value">{stats.ranked_draws}</span>
              <span className="rpg-stat-label">Draws</span>
            </div>
            <div className="rpg-stat-card">
              <span className="rpg-stat-value">{(stats.ranked_win_rate * 100).toFixed(1)}%</span>
              <span className="rpg-stat-label">Win Rate</span>
            </div>
          </div>
          <div className="rpg-stats-row">
            <div className="rpg-stat-mini">
              <span className="rpg-stat-mini-value">🔥 {stats.ranked_current_streak}</span>
              <span className="rpg-stat-mini-label">Current Streak</span>
            </div>
            <div className="rpg-stat-mini">
              <span className="rpg-stat-mini-value">⭐ {stats.ranked_best_streak}</span>
              <span className="rpg-stat-mini-label">Best Streak</span>
            </div>
            <div className="rpg-stat-mini">
              <span className="rpg-stat-mini-value">🏅 {stats.ranked_tournaments_won}/{stats.ranked_tournaments_played}</span>
              <span className="rpg-stat-mini-label">Tournaments</span>
            </div>
          </div>
        </RPGPanel>

        {/* Casual stats */}
        <RPGPanel variant="inner" className="rpg-section-panel">
          <h3 className="rpg-section-title">⚔️ Casual</h3>
          <div className="rpg-stats-grid">
            <div className="rpg-stat-card">
              <span className="rpg-stat-value stat-wins">{stats.regular_wins}</span>
              <span className="rpg-stat-label">Wins</span>
            </div>
            <div className="rpg-stat-card">
              <span className="rpg-stat-value stat-losses">{stats.regular_losses}</span>
              <span className="rpg-stat-label">Losses</span>
            </div>
            <div className="rpg-stat-card">
              <span className="rpg-stat-value">{stats.regular_draws}</span>
              <span className="rpg-stat-label">Draws</span>
            </div>
            <div className="rpg-stat-card">
              <span className="rpg-stat-value">{(stats.regular_win_rate * 100).toFixed(1)}%</span>
              <span className="rpg-stat-label">Win Rate</span>
            </div>
          </div>
          <div className="rpg-stats-row">
            <div className="rpg-stat-mini">
              <span className="rpg-stat-mini-value">🔥 {stats.regular_current_streak}</span>
              <span className="rpg-stat-mini-label">Current Streak</span>
            </div>
            <div className="rpg-stat-mini">
              <span className="rpg-stat-mini-value">⭐ {stats.regular_best_streak}</span>
              <span className="rpg-stat-mini-label">Best Streak</span>
            </div>
          </div>
        </RPGPanel>
      </RPGPanel>
    )
  }

  // Render Profile modal overlay
  const renderProfileModal = () => {
    if (!showProfileModal) return null

    return (
      <div className="rpg-h2h-modal-backdrop" onClick={handleCloseProfileModal}>
        <div className="rpg-h2h-modal-wrapper" onClick={(e) => e.stopPropagation()}>
          <RPGPanel variant="outer" className="rpg-h2h-modal-outer">
            {/* Close button */}
            <div className="rpg-h2h-close-wrapper">
              <GameButton
                variant="danger"
                size="sm"
                shape="square"
                onClick={handleCloseProfileModal}
                aria-label="Close"
              >
                ×
              </GameButton>
            </div>

            {/* Loading state */}
            {profileLoading && !profileData && (
              <div className="rpg-h2h-loading">
                <LoadingSpinner message="Loading profile..." />
              </div>
            )}

            {/* Profile content */}
            {(!profileLoading || profileData) && renderProfile()}
          </RPGPanel>
        </div>
      </div>
    )
  }

  // Render history tab content
  const renderHistory = () => {
    if (historyLoading && !historyData) {
      return <LoadingSpinner message="Loading history..." />
    }

    return (
      <RPGPanel variant="outer" className="stats-outer-panel">
        {historyData && historyData.matches.length > 0 ? (
          <RPGPanel variant="inner" className="history-inner-panel">
            <div className="rpg-history-list">
              {historyData.matches.map((match: MatchHistoryEntry) => (
                <div
                  key={match.match_id}
                  className={`rpg-history-item ${getResultClass(match.result)}`}
                  onClick={() => handleSelectOpponent(match.opponent.user_id)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), handleSelectOpponent(match.opponent.user_id))}
                >
                  <div className="rpg-history-result">
                    <span className={`rpg-result-badge ${match.result}`}>
                      {match.result === 'win' ? 'W' : match.result === 'loss' ? 'L' : 'D'}
                    </span>
                  </div>
                  <Avatar
                    firstName={match.opponent.first_name}
                    photoUrl={match.opponent.photo_url}
                    size="small"
                  />
                  <div className="rpg-history-details">
                    <div className="rpg-history-opponent">vs {match.opponent.first_name}</div>
                    <div className="rpg-history-meta">
                      <span className="rpg-history-type">
                        {match.match_type === 'ranked' ? '🏆' : '⚔️'}
                      </span>
                      <span className="rpg-history-date">{formatDate(match.completed_at)}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>

            {/* Pagination */}
            {historyData.total > PAGE_SIZE && (
              <div className="rpg-pagination">
                <GameButton
                  variant="neutral"
                  size="sm"
                  disabled={historyPage === 0 || historyLoading}
                  onClick={() => handleHistoryPageChange(historyPage - 1)}
                >
                  ← Prev
                </GameButton>
                <span className="rpg-pagination-info">
                  Page {historyPage + 1} of {Math.ceil(historyData.total / PAGE_SIZE)}
                </span>
                <GameButton
                  variant="neutral"
                  size="sm"
                  disabled={(historyPage + 1) * PAGE_SIZE >= historyData.total || historyLoading}
                  onClick={() => handleHistoryPageChange(historyPage + 1)}
                >
                  Next →
                </GameButton>
              </div>
            )}
          </RPGPanel>
        ) : (
          <div className="rpg-empty-state">
            <span className="rpg-empty-icon">📜</span>
            <p className="rpg-empty-title">No match history yet</p>
            <p className="rpg-empty-hint">Play some matches to see your history!</p>
          </div>
        )}
      </RPGPanel>
    )
  }

  // Render H2H modal overlay
  const renderH2HModal = () => {
    if (!showH2HModal) return null

    return (
      <div className="rpg-h2h-modal-backdrop" onClick={handleCloseH2HModal}>
        <div className="rpg-h2h-modal-wrapper" onClick={(e) => e.stopPropagation()}>
          <RPGPanel variant="outer" className="rpg-h2h-modal-outer">
            {/* Close button */}
            <div className="rpg-h2h-close-wrapper">
              <GameButton
                variant="danger"
                size="sm"
                shape="square"
                onClick={handleCloseH2HModal}
                aria-label="Close"
              >
                ×
              </GameButton>
            </div>

            {/* Loading state */}
            {h2hLoading && (
              <div className="rpg-h2h-loading">
                <LoadingSpinner message="Loading head-to-head..." />
              </div>
            )}

            {/* H2H data display */}
            {!h2hLoading && h2hData && (
              <RPGPanel variant="inner" className="rpg-h2h-content">
                <div className="rpg-h2h-header">
                  <div className="rpg-h2h-player you">You</div>
                  <div className="rpg-h2h-vs">VS</div>
                  <div className="rpg-h2h-player opponent">
                    {h2hData.record.opponent.first_name}
                    {h2hData.record.opponent.username && (
                      <span className="rpg-h2h-username">@{h2hData.record.opponent.username}</span>
                    )}
                  </div>
                </div>

                <div className="rpg-h2h-record">
                  <div className="rpg-h2h-stat wins">
                    <span className="rpg-h2h-stat-value">{h2hData.record.wins}</span>
                    <span className="rpg-h2h-stat-label">Wins</span>
                  </div>
                  <div className="rpg-h2h-stat draws">
                    <span className="rpg-h2h-stat-value">{h2hData.record.draws}</span>
                    <span className="rpg-h2h-stat-label">Draws</span>
                  </div>
                  <div className="rpg-h2h-stat losses">
                    <span className="rpg-h2h-stat-value">{h2hData.record.losses}</span>
                    <span className="rpg-h2h-stat-label">Losses</span>
                  </div>
                </div>

                <div className="rpg-h2h-summary">
                  <div className="rpg-h2h-summary-item">
                    <span className="rpg-h2h-summary-label">Total Matches</span>
                    <span className="rpg-h2h-summary-value">{h2hData.record.total_matches}</span>
                  </div>
                  <div className="rpg-h2h-summary-item">
                    <span className="rpg-h2h-summary-label">Win Rate</span>
                    <span className="rpg-h2h-summary-value">{(h2hData.record.win_rate * 100).toFixed(1)}%</span>
                  </div>
                  {h2hData.record.last_played && (
                    <div className="rpg-h2h-summary-item">
                      <span className="rpg-h2h-summary-label">Last Played</span>
                      <span className="rpg-h2h-summary-value">{formatDate(h2hData.record.last_played)}</span>
                    </div>
                  )}
                </div>
              </RPGPanel>
            )}
          </RPGPanel>
        </div>
      </div>
    )
  }

  // Render active sub-tab content
  const renderContent = () => {
    switch (activeSubTab) {
      case 'leaderboard':
        return renderLeaderboard()
      case 'history':
        return renderHistory()
      default:
        return renderLeaderboard()
    }
  }

  return (
    <div className="stats-page rpg-stats-page">
      {/* Sub-tabs */}
      <nav className="stats-tabs-rpg" role="tablist">
        <GameButton
          variant={activeSubTab === 'leaderboard' ? 'primary' : 'neutral'}
          size="sm"
          onClick={() => handleSubTabChange('leaderboard')}
          aria-selected={activeSubTab === 'leaderboard'}
        >
          📊 Leaderboard
        </GameButton>
        <GameButton
          variant={activeSubTab === 'history' ? 'primary' : 'neutral'}
          size="sm"
          onClick={() => handleSubTabChange('history')}
          aria-selected={activeSubTab === 'history'}
        >
          📜 History
        </GameButton>
      </nav>

      {/* Error banner */}
      {error && (
        <div className="rpg-error-banner" role="alert">
          <span className="rpg-error-icon">⚠️</span>
          <span className="rpg-error-text">{error}</span>
          <GameButton
            variant="danger"
            size="sm"
            shape="square"
            onClick={() => setError(null)}
            aria-label="Dismiss error"
          >
            ×
          </GameButton>
        </div>
      )}

      {/* Content */}
      <div className="rpg-stats-tab-content" role="tabpanel">
        {renderContent()}
      </div>

      {/* H2H Modal */}
      {renderH2HModal()}

      {/* Profile Modal */}
      {renderProfileModal()}
    </div>
  )
}

export default StatsPage
