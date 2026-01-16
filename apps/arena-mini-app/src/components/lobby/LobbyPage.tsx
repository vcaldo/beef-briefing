/**
 * LobbyPage - Main lobby view for arena matches
 *
 * Features:
 * - Displays list of active matches for the chat
 * - Shows active match card if user is in a match
 * - Polls for match updates every 2-3 seconds
 * - Auto-navigates to Shop when match transitions to shop_phase
 */

import { useState, useEffect, useCallback, useRef } from 'react'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { LoadingSpinner, CountdownTimer } from '../common'

import type { Match } from '../../types'

// Polling intervals (in ms)
const POLL_INTERVAL_NO_MATCH = 3000 // 3 seconds when not in a match
const POLL_INTERVAL_IN_MATCH = 2000 // 2 seconds when in a match

interface LobbyPageProps {
  chatId: number
  userId: number
  firstName: string
  onNavigateToShop: () => void
  onNavigateToBattle: () => void
  activeMatch: Match | null
  onMatchChange: (match: Match | null) => void
}

export function LobbyPage({
  chatId,
  userId,
  firstName,
  onNavigateToShop,
  onNavigateToBattle,
  activeMatch,
  onMatchChange,
}: LobbyPageProps) {
  // Local state
  const [matches, setMatches] = useState<Match[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState<string | null>(null) // 'create' | 'join' | 'leave' | 'start'

  // Refs for cleanup and preventing stale closures
  const pollIntervalRef = useRef<number | null>(null)
  const isMountedRef = useRef(true)
  // Track whether initial fetch is complete - used to distinguish page reload from real-time transitions
  const initialFetchDoneRef = useRef(false)

  // Find user's active match from the list
  const findUserMatch = useCallback(
    (matchList: Match[]): Match | null => {
      for (const match of matchList) {
        const isParticipant = match.participants?.some((p) => p.user_id === userId)
        if (isParticipant && (match.status === 'open' || match.status === 'shop_phase' || match.status === 'battle_phase' || match.status === 'completed')) {
          return match
        }
      }
      return null
    },
    [userId]
  )

  /**
   * Fetch all active matches from the API and handle phase transitions.
   *
   * Key behaviors:
   * 1. Detects when user's match transitions to shop_phase → auto-navigate
   * 2. Updates local match list state for display
   * 3. Tracks user's active match across the app
   *
   * Phase transition detection is critical for seamless UX: when the match
   * creator starts a match or auto-start triggers, this callback detects
   * the status change and navigates to the shop screen automatically.
   */
  const fetchMatches = useCallback(async () => {
    // Skip if component unmounted (prevents React state update warnings)
    if (!isMountedRef.current) return

    try {
      const fetchedMatches = await apiClient.getMatches(chatId)

      // Double-check mounted state after async operation
      if (!isMountedRef.current) return

      setMatches(fetchedMatches)
      setError(null)

      // Find if user is participating in any active match
      const userMatch = findUserMatch(fetchedMatches)

      // CRITICAL: Detect phase transition to trigger navigation
      // This happens when match creator clicks "Start" or auto-start triggers
      // Only auto-navigate on REAL-TIME transitions, not on page reload
      if (userMatch && userMatch.status === 'shop_phase') {
        if (initialFetchDoneRef.current) {
          // Real-time transition - auto-navigate to shop
          addPageAction('match_phase_transition', {
            match_id: userMatch.id,
            from_status: activeMatch?.status || 'open',
            to_status: 'shop_phase',
          })
          onMatchChange(userMatch)
          onNavigateToShop()
          return
        }
        // Initial load during shop_phase - let user see lobby with "Continue to Shop" button
        // (handled by active match card display below)
      }

      // Handle battle_phase - user may have refreshed during battle
      if (userMatch && userMatch.status === 'battle_phase') {
        addPageAction('match_phase_recovery', {
          match_id: userMatch.id,
          status: 'battle_phase',
        })
        onMatchChange(userMatch)
        onNavigateToBattle()
        return
      }

      // Sync active match state with parent component
      if (userMatch !== activeMatch) {
        onMatchChange(userMatch)
      }

      // First successful load - update loading state and track analytics
      if (loading) {
        setLoading(false)
        addPageAction('lobby_loaded', {
          match_count: fetchedMatches.length,
          has_active_match: !!userMatch,
        })
      }

      // Mark initial fetch as complete after first successful load
      // This enables auto-navigation for subsequent real-time transitions
      if (!initialFetchDoneRef.current) {
        initialFetchDoneRef.current = true
      }
    } catch (err) {
      if (!isMountedRef.current) return

      console.error('Failed to fetch matches:', err)
      setError(err instanceof Error ? err.message : 'Failed to load matches')
      setLoading(false)

      if (err instanceof Error) {
        noticeError(err, { context: 'lobby_fetch_matches' })
      }
    }
  }, [chatId, userId, findUserMatch, activeMatch, onMatchChange, onNavigateToShop, onNavigateToBattle, loading])

  // Poll for match with specific match ID (when user is in a match)
  const fetchMatchDetails = useCallback(async () => {
    if (!isMountedRef.current || !activeMatch) return

    try {
      const match = await apiClient.getMatch(activeMatch.id)

      if (!isMountedRef.current) return

      // Check for phase transition to shop
      if (match.status === 'shop_phase') {
        addPageAction('match_phase_transition', {
          match_id: match.id,
          from_status: activeMatch.status,
          to_status: 'shop_phase',
        })
        onMatchChange(match)
        onNavigateToShop()
        return
      }

      // Check for phase transition to battle (or completed - battle executes instantly)
      if (match.status === 'battle_phase' || match.status === 'completed') {
        addPageAction('match_phase_transition', {
          match_id: match.id,
          from_status: activeMatch.status,
          to_status: match.status,
        })
        onMatchChange(match)
        onNavigateToBattle()
        return
      }

      // Check if match was cancelled
      if (match.status === 'cancelled') {
        addPageAction('match_ended', {
          match_id: match.id,
          status: match.status,
        })
        onMatchChange(null)
        // Refresh full list
        fetchMatches()
        return
      }

      // Update match state
      onMatchChange(match)
    } catch (err) {
      if (!isMountedRef.current) return

      console.error('Failed to fetch match details:', err)
      // On error, fall back to fetching all matches
      fetchMatches()
    }
  }, [activeMatch, onMatchChange, onNavigateToShop, onNavigateToBattle, fetchMatches])

  /**
   * Setup match polling with adaptive intervals.
   *
   * Polling strategy:
   * - When NOT in a match: Poll /matches every 3s to show available matches
   * - When IN a match: Poll /match/{id} every 2s for faster status updates
   *
   * This effect re-runs when activeMatch changes to switch between strategies.
   * The faster polling when in a match ensures users see phase transitions
   * (open → shop_phase) quickly for responsive navigation.
   */
  useEffect(() => {
    isMountedRef.current = true

    // Fetch immediately on mount or when activeMatch changes
    fetchMatches()

    // Configure polling based on match participation status
    const startPolling = () => {
      // Clear any existing interval before starting a new one
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current)
      }

      // Use faster polling (2s) when in a match for quicker phase detection
      const interval = activeMatch ? POLL_INTERVAL_IN_MATCH : POLL_INTERVAL_NO_MATCH
      // Poll specific match endpoint when in a match, otherwise poll match list
      const pollFn = activeMatch ? fetchMatchDetails : fetchMatches

      pollIntervalRef.current = window.setInterval(pollFn, interval)
    }

    startPolling()

    // Cleanup: mark unmounted and clear interval to prevent memory leaks
    return () => {
      isMountedRef.current = false
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current)
        pollIntervalRef.current = null
      }
    }
  }, [activeMatch, fetchMatches, fetchMatchDetails])

  // Create new match
  const handleCreateMatch = useCallback(async () => {
    setActionLoading('create')
    try {
      const match = await apiClient.createMatch(chatId)
      addPageAction('match_created', { match_id: match.id })
      onMatchChange(match)
      // Refresh list
      fetchMatches()
    } catch (err) {
      console.error('Failed to create match:', err)
      setError(err instanceof Error ? err.message : 'Failed to create match')
      if (err instanceof Error) {
        noticeError(err, { context: 'create_match' })
      }
    } finally {
      setActionLoading(null)
    }
  }, [chatId, onMatchChange, fetchMatches])

  // Join match
  const handleJoinMatch = useCallback(
    async (matchId: string) => {
      setActionLoading('join')
      try {
        const match = await apiClient.joinMatch(matchId)
        addPageAction('match_joined', { match_id: matchId })
        onMatchChange(match)
      } catch (err) {
        console.error('Failed to join match:', err)
        setError(err instanceof Error ? err.message : 'Failed to join match')
        if (err instanceof Error) {
          noticeError(err, { context: 'join_match', match_id: matchId })
        }
      } finally {
        setActionLoading(null)
      }
    },
    [onMatchChange]
  )

  // Leave match
  const handleLeaveMatch = useCallback(
    async (matchId: string) => {
      setActionLoading('leave')
      try {
        await apiClient.leaveMatch(matchId)
        addPageAction('match_left', { match_id: matchId })
        onMatchChange(null)
        // Refresh list
        fetchMatches()
      } catch (err) {
        console.error('Failed to leave match:', err)
        setError(err instanceof Error ? err.message : 'Failed to leave match')
        if (err instanceof Error) {
          noticeError(err, { context: 'leave_match', match_id: matchId })
        }
      } finally {
        setActionLoading(null)
      }
    },
    [onMatchChange, fetchMatches]
  )

  // Start match early (creator only)
  const handleStartMatch = useCallback(
    async (matchId: string) => {
      setActionLoading('start')
      try {
        const match = await apiClient.startMatch(matchId)
        addPageAction('match_started_early', { match_id: matchId })

        // If match transitioned to shop phase, navigate
        if (match.status === 'shop_phase') {
          onMatchChange(match)
          onNavigateToShop()
        } else {
          onMatchChange(match)
        }
      } catch (err) {
        console.error('Failed to start match:', err)
        setError(err instanceof Error ? err.message : 'Failed to start match')
        if (err instanceof Error) {
          noticeError(err, { context: 'start_match', match_id: matchId })
        }
      } finally {
        setActionLoading(null)
      }
    },
    [onMatchChange, onNavigateToShop]
  )

  // Clear error after a timeout
  useEffect(() => {
    if (error) {
      const timeout = setTimeout(() => setError(null), 5000)
      return () => clearTimeout(timeout)
    }
  }, [error])

  // Loading state
  if (loading) {
    return (
      <div className="lobby-page">
        <LoadingSpinner message="Loading matches..." />
      </div>
    )
  }

  // Check if user is the creator of their active match
  const isCreator = activeMatch?.creator_user_id === userId
  const canStartEarly = isCreator && activeMatch?.participants && activeMatch.participants.length >= 2

  return (
    <div className="lobby-page">
      <header className="lobby-header">
        <h1 className="lobby-title">Arena Lobby</h1>
        <p className="lobby-subtitle">Welcome, {firstName}</p>
      </header>

      {/* Error banner */}
      {error && (
        <div className="lobby-error-banner" role="alert">
          <span className="lobby-error-icon">⚠️</span>
          <span className="lobby-error-text">{error}</span>
          <button className="lobby-error-dismiss" onClick={() => setError(null)} aria-label="Dismiss error">
            ×
          </button>
        </div>
      )}

      {/* Active match card */}
      {activeMatch && (
        <section className="lobby-active-match">
          <div className="match-card active">
            <div className="match-card-header">
              <span className="match-card-badge">Your Match</span>
              <span className={`match-card-status status-${activeMatch.status}`}>
                {activeMatch.status === 'open' ? 'Waiting for players' : activeMatch.status.replace('_', ' ')}
              </span>
            </div>

            <div className="match-card-body">
              {/* Participants */}
              <div className="match-card-participants">
                <span className="participants-label">Players:</span>
                <div className="participants-list">
                  {activeMatch.participants?.map((p) => (
                    <div
                      key={p.user_id}
                      className={`participant-chip ${p.user_id === userId ? 'is-you' : ''}`}
                      title={p.username || p.first_name}
                    >
                      <span className="participant-name">
                        {p.first_name}
                        {p.user_id === activeMatch.creator_user_id && (
                          <span className="participant-creator-badge" title="Match creator">
                            👑
                          </span>
                        )}
                      </span>
                    </div>
                  ))}
                </div>
                <span className="participants-count">
                  {activeMatch.participants?.length || 0} / {activeMatch.format === '1v1' ? '2' : '∞'}
                </span>
              </div>

              {/* Countdown timer */}
              {activeMatch.status === 'open' && activeMatch.join_deadline && (
                <div className="match-card-timer">
                  <span className="timer-label">Match starts in:</span>
                  <CountdownTimer
                    deadline={activeMatch.join_deadline}
                    onExpire={() => {
                      addPageAction('match_auto_start', { match_id: activeMatch.id })
                    }}
                  />
                </div>
              )}
            </div>

            <div className="match-card-actions">
              {/* Leave button */}
              <button
                className="btn-secondary"
                onClick={() => handleLeaveMatch(activeMatch.id)}
                disabled={actionLoading !== null}
              >
                {actionLoading === 'leave' ? <LoadingSpinner size="sm" inline /> : 'Leave Match'}
              </button>

              {/* Start early button (creator only with 2+ players) */}
              {canStartEarly && (
                <button
                  className="btn-primary"
                  onClick={() => handleStartMatch(activeMatch.id)}
                  disabled={actionLoading !== null}
                >
                  {actionLoading === 'start' ? <LoadingSpinner size="sm" inline /> : 'Start Now'}
                </button>
              )}
            </div>
          </div>
        </section>
      )}

      {/* Match list (when not in a match) */}
      {!activeMatch && (
        <section className="lobby-matches">
          <div className="lobby-section-header">
            <h2 className="lobby-section-title">Available Matches</h2>
            <button
              className="btn-primary create-match-btn"
              onClick={handleCreateMatch}
              disabled={actionLoading !== null}
            >
              {actionLoading === 'create' ? <LoadingSpinner size="sm" inline /> : '+ New Match'}
            </button>
          </div>

          {matches.length === 0 ? (
            <div className="lobby-empty-state">
              <div className="empty-state-icon">⚔️</div>
              <h3 className="empty-state-title">No Active Matches</h3>
              <p className="empty-state-text">Create a new match to challenge your friends!</p>
              <button
                className="btn-primary btn-lg"
                onClick={handleCreateMatch}
                disabled={actionLoading !== null}
              >
                {actionLoading === 'create' ? <LoadingSpinner size="sm" inline /> : 'Create Match'}
              </button>
            </div>
          ) : (
            <div className="match-list">
              {matches
                .filter((m) => m.status === 'open')
                .map((match) => {
                  const isInMatch = match.participants?.some((p) => p.user_id === userId)
                  const participantCount = match.participants?.length || 0

                  return (
                    <div key={match.id} className="match-card">
                      <div className="match-card-header">
                        <span className="match-card-type">
                          {match.match_type === 'ranked' ? '🏆 Ranked' : '⚔️ Casual'}
                        </span>
                        <span className="match-card-format">{match.format || '1v1'}</span>
                      </div>

                      <div className="match-card-body">
                        {/* Participants preview */}
                        <div className="match-card-participants">
                          <div className="participants-avatars">
                            {match.participants?.slice(0, 4).map((p) => (
                              <div
                                key={p.user_id}
                                className="participant-avatar"
                                title={p.first_name}
                              >
                                {p.first_name.charAt(0).toUpperCase()}
                              </div>
                            ))}
                            {participantCount > 4 && (
                              <div className="participant-avatar more">+{participantCount - 4}</div>
                            )}
                          </div>
                          <span className="participants-count">{participantCount} players</span>
                        </div>

                        {/* Timer */}
                        {match.join_deadline && (
                          <div className="match-card-timer compact">
                            <CountdownTimer deadline={match.join_deadline} />
                          </div>
                        )}
                      </div>

                      <div className="match-card-actions">
                        {!isInMatch && (
                          <button
                            className="btn-primary"
                            onClick={() => handleJoinMatch(match.id)}
                            disabled={actionLoading !== null}
                          >
                            {actionLoading === 'join' ? <LoadingSpinner size="sm" inline /> : 'Join'}
                          </button>
                        )}
                      </div>
                    </div>
                  )
                })}
            </div>
          )}
        </section>
      )}
    </div>
  )
}

export default LobbyPage
