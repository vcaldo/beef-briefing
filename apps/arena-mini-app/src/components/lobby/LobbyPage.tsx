import { useState, useEffect, useCallback, useRef } from 'react'
import { apiClient } from '../../api/client'
import { ApiError } from '@beef-briefing/shared-mini-app/api'
import { LoadingSpinner } from '../common/LoadingSpinner'
import { ErrorDisplay } from '../common/ErrorDisplay'
import { MatchList } from './MatchList'
import { CreateMatchButton } from './CreateMatchButton'
import type { MatchResponse, GamePhase } from '../../types'

/** Polling interval in milliseconds (2.5 seconds as per plan) */
const POLL_INTERVAL = 2500

/** Maximum polling interval during backoff (30 seconds) */
const MAX_POLL_INTERVAL = 30000

/** Number of consecutive failures before showing persistent error */
const CONSECUTIVE_FAILURE_THRESHOLD = 3

interface LobbyPageProps {
  /** Current user's Telegram ID */
  currentUserId?: number
  /** Callback to navigate to a different tab */
  onTabChange?: (tab: GamePhase) => void
  /** Callback when user enters an active match (for shop/battle navigation) */
  onMatchEnter?: (matchId: string, phase: 'shop' | 'battle') => void
}

/**
 * Lobby page displaying available matches with real-time polling.
 * Handles match creation, joining, leaving, and starting.
 * Auto-navigates to shop/battle when a match the user is in transitions.
 */
export function LobbyPage({
  currentUserId,
  onTabChange,
  onMatchEnter,
}: LobbyPageProps) {
  const [matches, setMatches] = useState<MatchResponse[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const [creatingMatch, setCreatingMatch] = useState(false)

  // Track previous match states for auto-navigation
  const prevMatchStatesRef = useRef<Map<string, string>>(new Map())

  // Exponential backoff state for polling failures
  const consecutiveFailuresRef = useRef(0)
  const currentPollIntervalRef = useRef(POLL_INTERVAL)

  /**
   * Fetch matches from the API with exponential backoff on failures
   */
  const fetchMatches = useCallback(async (showLoading = false) => {
    try {
      if (showLoading) setLoading(true)
      const data = await apiClient.listMatches()
      setMatches(data)

      // Reset backoff on success
      consecutiveFailuresRef.current = 0
      currentPollIntervalRef.current = POLL_INTERVAL

      // Only clear error if we've recovered from consecutive failures
      setError(null)

      // Check for phase transitions to auto-navigate
      checkForPhaseTransitions(data)

      // Update previous states
      const newStates = new Map<string, string>()
      data.forEach(m => newStates.set(m.id, m.status))
      prevMatchStatesRef.current = newStates
    } catch (err) {
      console.error('Failed to fetch matches:', err)

      // Increment consecutive failures and apply backoff
      consecutiveFailuresRef.current += 1

      // Exponential backoff: double the interval up to max
      if (consecutiveFailuresRef.current > 1) {
        currentPollIntervalRef.current = Math.min(
          currentPollIntervalRef.current * 2,
          MAX_POLL_INTERVAL
        )
      }

      // Only show error after threshold consecutive failures OR if no data yet
      if (consecutiveFailuresRef.current >= CONSECUTIVE_FAILURE_THRESHOLD || matches.length === 0) {
        const errorMsg = err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : 'Failed to load matches'
        setError(errorMsg)
      }
    } finally {
      if (showLoading) setLoading(false)
    }
  }, [matches.length])

  /**
   * Check if any match the user is in has transitioned to shop/battle.
   * Also handles initial load when user already has an active match.
   */
  const checkForPhaseTransitions = useCallback((newMatches: MatchResponse[]) => {
    if (!currentUserId) return

    for (const match of newMatches) {
      const isParticipant = match.participants.some(p => p.user_id === currentUserId)
      if (!isParticipant) continue

      const prevStatus = prevMatchStatesRef.current.get(match.id)
      const isFirstLoad = prevMatchStatesRef.current.size === 0

      // Auto-navigate when match transitions to shop_phase
      // OR on first load if user already has an active shop_phase match
      if (
        (prevStatus === 'open' && match.status === 'shop_phase') ||
        (isFirstLoad && match.status === 'shop_phase')
      ) {
        onMatchEnter?.(match.id, 'shop')
        onTabChange?.('shop')
        return
      }

      // Auto-navigate when match transitions to battle_phase
      // OR on first load if user already has an active battle_phase match
      if (
        (prevStatus === 'shop_phase' && match.status === 'battle_phase') ||
        (isFirstLoad && (match.status === 'battle_phase' || match.status === 'completed'))
      ) {
        onMatchEnter?.(match.id, 'battle')
        onTabChange?.('battle')
        return
      }
    }
  }, [currentUserId, onMatchEnter, onTabChange])

  /**
   * Create a new match
   */
  const handleCreateMatch = async () => {
    try {
      setCreatingMatch(true)
      const newMatch = await apiClient.createMatch()

      // Add new match to the list
      setMatches(prev => [newMatch, ...prev])
    } catch (err) {
      console.error('Failed to create match:', err)
      setError(err instanceof Error ? err.message : 'Failed to create match')
    } finally {
      setCreatingMatch(false)
    }
  }

  /**
   * Join an existing match
   */
  const handleJoinMatch = async (matchId: string) => {
    try {
      setActionLoading(matchId)
      const updatedMatch = await apiClient.joinMatch(matchId)

      // Update the match in the list
      setMatches(prev =>
        prev.map(m => (m.id === matchId ? updatedMatch : m))
      )
    } catch (err) {
      console.error('Failed to join match:', err)
      setError(err instanceof Error ? err.message : 'Failed to join match')
    } finally {
      setActionLoading(null)
    }
  }

  /**
   * Leave a match
   */
  const handleLeaveMatch = async (matchId: string) => {
    try {
      setActionLoading(matchId)
      await apiClient.leaveMatch(matchId)

      // Refresh to get updated participant list
      await fetchMatches()
    } catch (err) {
      console.error('Failed to leave match:', err)
      setError(err instanceof Error ? err.message : 'Failed to leave match')
    } finally {
      setActionLoading(null)
    }
  }

  /**
   * Start a match early (creator only)
   */
  const handleStartMatch = async (matchId: string) => {
    try {
      setActionLoading(matchId)
      const updatedMatch = await apiClient.startMatch(matchId)

      // Update the match in the list
      setMatches(prev =>
        prev.map(m => (m.id === matchId ? updatedMatch : m))
      )

      // Navigate to shop if we're a participant
      if (updatedMatch.status === 'shop_phase') {
        onMatchEnter?.(matchId, 'shop')
        onTabChange?.('shop')
      }
    } catch (err) {
      console.error('Failed to start match:', err)
      setError(err instanceof Error ? err.message : 'Failed to start match')
    } finally {
      setActionLoading(null)
    }
  }

  /**
   * View a match (navigate to shop or battle based on status)
   */
  const handleViewMatch = (matchId: string) => {
    const match = matches.find(m => m.id === matchId)
    if (!match) return

    if (match.status === 'shop_phase') {
      onMatchEnter?.(matchId, 'shop')
      onTabChange?.('shop')
    } else if (match.status === 'battle_phase' || match.status === 'completed') {
      onMatchEnter?.(matchId, 'battle')
      onTabChange?.('battle')
    }
  }

  // Initial fetch
  useEffect(() => {
    fetchMatches(true)
  }, [fetchMatches])

  // Set up polling with dynamic interval (supports exponential backoff)
  useEffect(() => {
    let timeoutId: ReturnType<typeof setTimeout>

    const poll = () => {
      fetchMatches(false)
      // Schedule next poll with current interval (may be increased due to backoff)
      timeoutId = setTimeout(poll, currentPollIntervalRef.current)
    }

    // Start polling after initial interval
    timeoutId = setTimeout(poll, currentPollIntervalRef.current)

    return () => clearTimeout(timeoutId)
  }, [fetchMatches])

  // Loading state
  if (loading) {
    return (
      <div className="lobby-page">
        <LoadingSpinner message="Loading matches..." />
      </div>
    )
  }

  // Error state
  if (error && matches.length === 0) {
    return (
      <div className="lobby-page">
        <ErrorDisplay
          title="Failed to Load"
          message={error}
          onRetry={() => fetchMatches(true)}
        />
      </div>
    )
  }

  // Sort matches: open first, then by date
  const sortedMatches = [...matches].sort((a, b) => {
    // Open matches first
    if (a.status === 'open' && b.status !== 'open') return -1
    if (a.status !== 'open' && b.status === 'open') return 1

    // Active matches (shop/battle) second
    const aActive = a.status === 'shop_phase' || a.status === 'battle_phase'
    const bActive = b.status === 'shop_phase' || b.status === 'battle_phase'
    if (aActive && !bActive) return -1
    if (!aActive && bActive) return 1

    // Then by creation date (newest first)
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  })

  return (
    <div className="lobby-page">
      {/* Error banner (non-blocking) */}
      {error && (
        <div className="error-banner">
          <span>{error}</span>
          <button onClick={() => setError(null)}>✕</button>
        </div>
      )}

      {/* Create match button */}
      <div className="lobby-actions">
        <CreateMatchButton
          onCreate={handleCreateMatch}
          isLoading={creatingMatch}
        />
      </div>

      {/* Match list */}
      <MatchList
        matches={sortedMatches}
        currentUserId={currentUserId}
        onJoin={handleJoinMatch}
        onLeave={handleLeaveMatch}
        onStart={handleStartMatch}
        onView={handleViewMatch}
        loadingMatchId={actionLoading}
      />
    </div>
  )
}
