/**
 * LobbyPage - Main lobby view for arena matches
 *
 * Features:
 * - Displays list of active matches for the chat
 * - Shows active match card if user is in a match
 * - Polls for match updates every 2-3 seconds
 * - Auto-navigates to Shop when match transitions to shop_phase
 */

import { useState, useCallback, useRef } from 'react'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { LoadingSpinner, CountdownTimer, ErrorBanner } from '../common'
import { GameButton, RPGPanel } from '../ui'
import { usePolling, useErrorBanner, useImages } from '../../hooks'
import { useSoundContext } from '../../contexts'

import type { Match, GameConstants } from '../../types'

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
  gameConstants: GameConstants | null
}

export function LobbyPage({
  chatId,
  userId,
  firstName,
  onNavigateToShop,
  onNavigateToBattle,
  activeMatch,
  onMatchChange,
  gameConstants,
}: LobbyPageProps) {
  // Local state
  const [matches, setMatches] = useState<Match[]>([])
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState<string | null>(null) // 'create' | 'join' | 'leave' | 'start'

  // Error banner with auto-dismiss
  const { error, showError, clearError } = useErrorBanner()

  // Sound system
  const { play, unlockAudio } = useSoundContext()

  // Image assets
  const { getUrl } = useImages()
  const bannerModernUrl = getUrl('panels', 'banner_modern')

  // Track whether initial fetch is complete - used to distinguish page reload from real-time transitions
  const initialFetchDoneRef = useRef(false)

  // Track whether audio has been unlocked (for mobile autoplay)
  const audioUnlockedRef = useRef(false)

  // Track last second we played a countdown sound to prevent duplicate plays
  const lastTickSoundRef = useRef<number | null>(null)

  // Unlock audio on first user interaction (required for mobile)
  const ensureAudioUnlocked = useCallback(() => {
    if (!audioUnlockedRef.current) {
      audioUnlockedRef.current = true
      unlockAudio()
    }
  }, [unlockAudio])

  // Handle countdown timer tick - play sounds at thresholds
  // countdown_tick: plays when timer is 10s or less
  // countdown_warning: plays when timer is 3s or less
  const handleCountdownTick = useCallback(
    (remainingSeconds: number) => {
      // Prevent duplicate sounds for the same second
      if (lastTickSoundRef.current === remainingSeconds) {
        return
      }

      // Play warning sound at 3s, 2s, 1s
      if (remainingSeconds <= 3 && remainingSeconds > 0) {
        play('countdown_warning')
        lastTickSoundRef.current = remainingSeconds
      }
      // Play tick sound at 10s down to 4s (before warning kicks in)
      else if (remainingSeconds <= 10 && remainingSeconds > 3) {
        play('countdown_tick')
        lastTickSoundRef.current = remainingSeconds
      }
    },
    [play]
  )

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
   * Handle successful fetch of match list.
   * Detects phase transitions and triggers navigation accordingly.
   */
  const handleMatchListSuccess = useCallback(
    (fetchedMatches: Match[]) => {
      setMatches(fetchedMatches)

      // Find if user is participating in any active match
      const userMatch = findUserMatch(fetchedMatches)

      // CRITICAL: Detect phase transition to trigger navigation
      // This happens when match creator clicks "Start" or auto-start triggers
      // Only auto-navigate on REAL-TIME transitions, not on page reload
      if (userMatch && userMatch.status === 'shop_phase') {
        if (initialFetchDoneRef.current) {
          // Real-time transition - auto-navigate to shop
          play('lobby_start')
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
    },
    [findUserMatch, activeMatch, onMatchChange, onNavigateToShop, onNavigateToBattle, loading, play]
  )

  /**
   * Handle error when fetching match list.
   */
  const handleMatchListError = useCallback(
    (err: Error) => {
      console.error('Failed to fetch matches:', err)
      showError(err, 'Failed to load matches')
      setLoading(false)
      noticeError(err, { context: 'lobby_fetch_matches' })
    },
    [showError]
  )

  /**
   * Handle successful fetch of single match details.
   * Detects phase transitions for users already in a match.
   */
  const handleMatchDetailsSuccess = useCallback(
    (match: Match) => {
      // Check for phase transition to shop
      if (match.status === 'shop_phase') {
        play('lobby_start')
        addPageAction('match_phase_transition', {
          match_id: match.id,
          from_status: activeMatch?.status,
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
          from_status: activeMatch?.status,
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
        return
      }

      // Update match state
      onMatchChange(match)
    },
    [activeMatch, onMatchChange, onNavigateToShop, onNavigateToBattle, play]
  )

  /**
   * Handle error when fetching match details.
   */
  const handleMatchDetailsError = useCallback((err: Error) => {
    console.error('Failed to fetch match details:', err)
    // Errors are silently logged - polling will retry on next interval
  }, [])

  /**
   * Polling strategy:
   * - When NOT in a match: Poll /matches every 3s to show available matches
   * - When IN a match: Poll /match/{id} every 2s for faster status updates
   *
   * The faster polling when in a match ensures users see phase transitions
   * (open → shop_phase) quickly for responsive navigation.
   */

  // Poll match list when not in a match
  usePolling({
    fetchFn: useCallback(() => apiClient.getMatches(chatId), [chatId]),
    interval: POLL_INTERVAL_NO_MATCH,
    enabled: !activeMatch,
    onSuccess: handleMatchListSuccess,
    onError: handleMatchListError,
  })

  // Poll single match when in a match (faster interval for phase detection)
  usePolling({
    fetchFn: useCallback(() => apiClient.getMatch(activeMatch!.id), [activeMatch]),
    interval: POLL_INTERVAL_IN_MATCH,
    enabled: !!activeMatch,
    onSuccess: handleMatchDetailsSuccess,
    onError: handleMatchDetailsError,
  })

  // Create new match
  const handleCreateMatch = useCallback(async () => {
    ensureAudioUnlocked()
    setActionLoading('create')
    try {
      const match = await apiClient.createMatch(chatId)
      play('lobby_create')
      addPageAction('match_created', { match_id: match.id })
      onMatchChange(match)
      // usePolling will automatically refresh the list on next interval
    } catch (err) {
      console.error('Failed to create match:', err)
      showError(err, 'Failed to create match')
      if (err instanceof Error) {
        noticeError(err, { context: 'create_match' })
      }
    } finally {
      setActionLoading(null)
    }
  }, [chatId, onMatchChange, showError, ensureAudioUnlocked, play])

  // Join match
  const handleJoinMatch = useCallback(
    async (matchId: string) => {
      ensureAudioUnlocked()
      setActionLoading('join')
      try {
        const match = await apiClient.joinMatch(matchId)
        play('lobby_join')
        addPageAction('match_joined', { match_id: matchId })
        onMatchChange(match)
      } catch (err) {
        console.error('Failed to join match:', err)
        showError(err, 'Failed to join match')
        if (err instanceof Error) {
          noticeError(err, { context: 'join_match', match_id: matchId })
        }
      } finally {
        setActionLoading(null)
      }
    },
    [onMatchChange, showError, ensureAudioUnlocked, play]
  )

  // Leave match
  const handleLeaveMatch = useCallback(
    async (matchId: string) => {
      ensureAudioUnlocked()
      setActionLoading('leave')
      try {
        await apiClient.leaveMatch(matchId)
        addPageAction('match_left', { match_id: matchId })
        onMatchChange(null)
        // usePolling will automatically refresh the list on next interval
      } catch (err) {
        console.error('Failed to leave match:', err)
        showError(err, 'Failed to leave match')
        if (err instanceof Error) {
          noticeError(err, { context: 'leave_match', match_id: matchId })
        }
      } finally {
        setActionLoading(null)
      }
    },
    [onMatchChange, showError, ensureAudioUnlocked]
  )

  // Start match early (creator only)
  const handleStartMatch = useCallback(
    async (matchId: string) => {
      ensureAudioUnlocked()
      setActionLoading('start')
      try {
        const match = await apiClient.startMatch(matchId)
        addPageAction('match_started_early', { match_id: matchId })

        // If match transitioned to shop phase, navigate
        if (match.status === 'shop_phase') {
          play('lobby_start')
          onMatchChange(match)
          onNavigateToShop()
        } else {
          onMatchChange(match)
        }
      } catch (err) {
        console.error('Failed to start match:', err)
        showError(err, 'Failed to start match')
        if (err instanceof Error) {
          noticeError(err, { context: 'start_match', match_id: matchId })
        }
      } finally {
        setActionLoading(null)
      }
    },
    [onMatchChange, onNavigateToShop, showError, ensureAudioUnlocked, play]
  )

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
        <ErrorBanner
          message={error}
          onDismiss={clearError}
          className="lobby-error-banner"
        />
      )}

      {/* Active match card - RPG Panel style */}
      {activeMatch && (
        <section className="lobby-active-match">
          <RPGPanel variant="outer" className="rpg-match-card">
            {/* Inner beige panel with timer, participants, and status */}
            <RPGPanel variant="inner" className="rpg-match-card-content">
              {/* Timer at top - inside inner panel */}
              {activeMatch.status === 'open' && activeMatch.join_deadline && (
                <div className="rpg-match-card-timer">
                  <span className="timer-label">Starts in:</span>
                  <CountdownTimer
                    deadline={activeMatch.join_deadline}
                    onExpire={() => {
                      addPageAction('match_auto_start', { match_id: activeMatch.id })
                    }}
                    onTick={handleCountdownTick}
                    timerThresholds={gameConstants?.timer_thresholds}
                  />
                </div>
              )}

              {/* Participants */}
              <div className="rpg-match-card-participants">
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

              {/* Status at bottom - inside inner panel */}
              <div className={`rpg-match-card-status status-${activeMatch.status}`}>
                {activeMatch.status === 'open'
                  ? 'Waiting for player...'
                  : activeMatch.status === 'shop_phase'
                    ? 'Shopping Phase'
                    : activeMatch.status.replace('_', ' ')}
              </div>
            </RPGPanel>
          </RPGPanel>

          {/* Action buttons - outside both panels */}
          <div className="rpg-match-card-actions">
            {activeMatch.status === 'shop_phase' ? (
              <GameButton variant="primary" size="lg" onClick={onNavigateToShop}>
                Continue to Shop
              </GameButton>
            ) : (
              <>
                <GameButton
                  variant="neutral"
                  onClick={() => handleLeaveMatch(activeMatch.id)}
                  disabled={actionLoading !== null}
                >
                  {actionLoading === 'leave' ? <LoadingSpinner size="sm" inline /> : 'Leave Match'}
                </GameButton>
                {canStartEarly && (
                  <GameButton
                    variant="primary"
                    onClick={() => handleStartMatch(activeMatch.id)}
                    disabled={actionLoading !== null}
                  >
                    {actionLoading === 'start' ? <LoadingSpinner size="sm" inline /> : 'Start Now'}
                  </GameButton>
                )}
              </>
            )}
          </div>
        </section>
      )}

      {/* Match list (when not in a match) */}
      {!activeMatch && (
        <section className="lobby-matches">
          <div className="lobby-section-header">
            <h2 className="lobby-section-title">Available Matches</h2>
            <GameButton
              variant="primary"
              size="sm"
              onClick={handleCreateMatch}
              disabled={actionLoading !== null}
              className="create-match-btn"
            >
              {actionLoading === 'create' ? <LoadingSpinner size="sm" inline /> : '+ New Match'}
            </GameButton>
          </div>

          {matches.length === 0 ? (
            <div className="lobby-empty-state">
              <div className="empty-state-icon">⚔️</div>
              <h3 className="empty-state-title">No Active Matches</h3>
              <p className="empty-state-text">Create a new match to challenge your friends!</p>
              <GameButton
                variant="primary"
                size="lg"
                onClick={handleCreateMatch}
                disabled={actionLoading !== null}
              >
                {actionLoading === 'create' ? <LoadingSpinner size="sm" inline /> : 'Create Match'}
              </GameButton>
            </div>
          ) : (
            <div className="match-list">
              {matches
                .filter((m) => m.status === 'open')
                .map((match) => {
                  const isInMatch = match.participants?.some((p) => p.user_id === userId)
                  const participantCount = match.participants?.length || 0

                  return (
                    <div
                      key={match.id}
                      className="match-card lobby-match-card-themed"
                      style={{
                        backgroundImage: `url(${bannerModernUrl})`,
                      }}
                    >
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

                        {/* Timer with Sci-fi glassPanel styling */}
                        {match.join_deadline && (
                          <div className="match-card-timer compact lobby-timer-badge">
                            <CountdownTimer
                              deadline={match.join_deadline}
                              timerThresholds={gameConstants?.timer_thresholds}
                            />
                          </div>
                        )}
                      </div>

                      <div className="match-card-actions">
                        {!isInMatch && (
                          <GameButton
                            variant="primary"
                            size="sm"
                            onClick={() => handleJoinMatch(match.id)}
                            disabled={actionLoading !== null}
                          >
                            {actionLoading === 'join' ? <LoadingSpinner size="sm" inline /> : 'Join'}
                          </GameButton>
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
