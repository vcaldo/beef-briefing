import { useState, useEffect, useCallback, useRef } from 'react'
import { apiClient } from '../../api/client'
import { LoadingSpinner } from '../common/LoadingSpinner'
import { ErrorDisplay } from '../common/ErrorDisplay'
import { BattleArena } from './BattleArena'
import { EventLog } from './EventLog'
import { VictoryScreen } from './VictoryScreen'
import type { BattleResponse, BattleEvent, CardSnapshot, GamePhase } from '../../types'

/** Playback speed options in milliseconds */
const PLAYBACK_SPEEDS = {
  slow: 2000,
  normal: 1000,
  fast: 500,
}

interface BattlePageProps {
  /** Match ID to load battle for */
  matchId: string | null
  /** Current user's Telegram ID */
  currentUserId?: number
  /** Callback to navigate to a different tab */
  onTabChange?: (tab: GamePhase) => void
}

/**
 * Battle page for watching battle replay and viewing results.
 * Handles event playback controls and displays battle state.
 */
export function BattlePage({
  matchId,
  currentUserId,
  onTabChange,
}: BattlePageProps) {
  const [battle, setBattle] = useState<BattleResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Playback state
  const [isPlaying, setIsPlaying] = useState(false)
  const [playbackSpeed, setPlaybackSpeed] = useState<keyof typeof PLAYBACK_SPEEDS>('normal')
  const [currentEventIndex, setCurrentEventIndex] = useState(0)
  const [displayedEvents, setDisplayedEvents] = useState<BattleEvent[]>([])
  const [showVictory, setShowVictory] = useState(false)

  // For tracking playback
  const playbackTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Get all events from all rounds
  const allEvents = battle?.rounds?.flatMap(round => round.battle_log) || []

  /**
   * Fetch battle state from the API
   */
  const fetchBattle = useCallback(async () => {
    if (!matchId) return

    try {
      setLoading(true)
      const data = await apiClient.getBattle(matchId)
      setBattle(data)
      setError(null)

      // If battle is complete and no events yet, auto-start playback
      if (data.is_complete && displayedEvents.length === 0) {
        setIsPlaying(true)
      }
    } catch (err) {
      console.error('Failed to fetch battle:', err)
      setError(err instanceof Error ? err.message : 'Failed to load battle')
    } finally {
      setLoading(false)
    }
  }, [matchId, displayedEvents.length])

  /**
   * Get current card states from the latest displayed event
   */
  const getCurrentCardStates = useCallback((): CardSnapshot[] => {
    // Find the last event with card_states
    for (let i = displayedEvents.length - 1; i >= 0; i--) {
      if (displayedEvents[i].card_states) {
        return displayedEvents[i].card_states!
      }
    }
    // Return initial state from first round
    if (battle?.rounds?.[0]) {
      const round = battle.rounds[0]
      const playerACards: CardSnapshot[] = round.player_a_team.map((card, idx) => ({
        card_id: card.card_id,
        user_id: card.user_id,
        team_owner_id: round.player_a_id,
        name: card.name,
        hp: card.hp,
        max_hp: card.max_hp,
        atk: card.atk,
        position: idx + 1,
        is_alive: true,
        is_attacking: false,
        is_defending: false,
      }))
      const playerBCards: CardSnapshot[] = round.player_b_team.map((card, idx) => ({
        card_id: card.card_id,
        user_id: card.user_id,
        team_owner_id: round.player_b_id,
        name: card.name,
        hp: card.hp,
        max_hp: card.max_hp,
        atk: card.atk,
        position: idx + 1,
        is_alive: true,
        is_attacking: false,
        is_defending: false,
      }))
      return [...playerACards, ...playerBCards]
    }
    return []
  }, [displayedEvents, battle])

  /**
   * Advance to next event
   */
  const advanceEvent = useCallback(() => {
    if (currentEventIndex >= allEvents.length) {
      setIsPlaying(false)
      // Show victory screen after last event
      if (battle?.is_complete) {
        setShowVictory(true)
      }
      return
    }

    const nextEvent = allEvents[currentEventIndex]
    setDisplayedEvents(prev => [...prev, nextEvent])
    setCurrentEventIndex(prev => prev + 1)
  }, [currentEventIndex, allEvents, battle?.is_complete])

  /**
   * Handle play/pause
   */
  const togglePlayback = () => {
    if (currentEventIndex >= allEvents.length) {
      // Reset playback if finished
      setCurrentEventIndex(0)
      setDisplayedEvents([])
      setShowVictory(false)
    }
    setIsPlaying(prev => !prev)
  }

  /**
   * Skip to end
   */
  const skipToEnd = () => {
    setIsPlaying(false)
    setDisplayedEvents(allEvents)
    setCurrentEventIndex(allEvents.length)
    if (battle?.is_complete) {
      setShowVictory(true)
    }
  }

  /**
   * Restart playback
   */
  const restartPlayback = () => {
    setCurrentEventIndex(0)
    setDisplayedEvents([])
    setShowVictory(false)
    setIsPlaying(true)
  }

  // Initial fetch
  useEffect(() => {
    if (matchId) {
      fetchBattle()
    }
  }, [matchId, fetchBattle])

  // Playback timer
  useEffect(() => {
    if (isPlaying && currentEventIndex < allEvents.length) {
      playbackTimerRef.current = setTimeout(() => {
        advanceEvent()
      }, PLAYBACK_SPEEDS[playbackSpeed])
    }

    return () => {
      if (playbackTimerRef.current) {
        clearTimeout(playbackTimerRef.current)
      }
    }
  }, [isPlaying, currentEventIndex, playbackSpeed, advanceEvent, allEvents.length])

  // No match selected
  if (!matchId) {
    return (
      <div className="battle-page">
        <div className="empty-state">
          <div className="empty-state-icon">⚔️</div>
          <h3 className="empty-state-title">No Battle Selected</h3>
          <p className="empty-state-text">
            Join a match and complete the shop phase to watch battles.
          </p>
          <button
            className="btn btn-primary mt-4"
            onClick={() => onTabChange?.('lobby')}
          >
            Go to Lobby
          </button>
        </div>
      </div>
    )
  }

  // Loading state
  if (loading) {
    return (
      <div className="battle-page">
        <LoadingSpinner message="Loading battle..." />
      </div>
    )
  }

  // Error state
  if (error && !battle) {
    return (
      <div className="battle-page">
        <ErrorDisplay
          title="Failed to Load Battle"
          message={error}
          onRetry={fetchBattle}
        />
      </div>
    )
  }

  // No battle data
  if (!battle) {
    return (
      <div className="battle-page">
        <div className="empty-state">
          <div className="empty-state-icon">⚔️</div>
          <h3 className="empty-state-title">Battle Not Available</h3>
          <p className="empty-state-text">
            The battle hasn't started yet or is not available.
          </p>
        </div>
      </div>
    )
  }

  // No rounds yet
  if (!battle.rounds || battle.rounds.length === 0) {
    return (
      <div className="battle-page">
        <div className="empty-state">
          <div className="empty-state-icon">⏳</div>
          <h3 className="empty-state-title">Battle Starting...</h3>
          <p className="empty-state-text">
            Waiting for battle to begin.
          </p>
          <LoadingSpinner size="sm" />
        </div>
      </div>
    )
  }

  const round = battle.rounds[0]
  const cardStates = getCurrentCardStates()

  // Separate cards by team owner (use team_owner_id, NOT user_id)
  const playerACards = cardStates.filter(c => c.team_owner_id === round.player_a_id)
  const playerBCards = cardStates.filter(c => c.team_owner_id === round.player_b_id)

  // Determine which team is "yours" vs opponent
  const isPlayerA = currentUserId === round.player_a_id
  const yourCards = isPlayerA ? playerACards : playerBCards
  const opponentCards = isPlayerA ? playerBCards : playerACards

  // Get winner info
  const isWinner = battle.winner_id === currentUserId
  const winnerName = battle.winner_id === round.player_a_id
    ? round.player_a_team[0]?.name
    : round.player_b_team[0]?.name

  return (
    <div className="battle-page">
      {/* Victory overlay */}
      {showVictory && (
        <VictoryScreen
          isWinner={isWinner}
          winnerName={winnerName}
          onClose={() => setShowVictory(false)}
          onViewStats={() => onTabChange?.('stats')}
          onBackToLobby={() => onTabChange?.('lobby')}
        />
      )}

      {/* Battle header */}
      <div className="battle-header">
        <span className="battle-round-badge">
          Round {currentEventIndex > 0 ? displayedEvents[displayedEvents.length - 1]?.round || 1 : 1}
        </span>
        <span className="battle-progress">
          {currentEventIndex}/{allEvents.length} events
        </span>
      </div>

      {/* Battle arena */}
      <BattleArena
        yourCards={yourCards}
        opponentCards={opponentCards}
        currentEvent={displayedEvents[displayedEvents.length - 1]}
      />

      {/* Playback controls */}
      <div className="playback-controls">
        <button
          className="playback-btn"
          onClick={restartPlayback}
          title="Restart"
        >
          ⏮️
        </button>
        <button
          className="playback-btn play-pause"
          onClick={togglePlayback}
          title={isPlaying ? 'Pause' : 'Play'}
        >
          {isPlaying ? '⏸️' : '▶️'}
        </button>
        <button
          className="playback-btn"
          onClick={skipToEnd}
          title="Skip to end"
        >
          ⏭️
        </button>
        <select
          className="speed-select"
          value={playbackSpeed}
          onChange={(e) => setPlaybackSpeed(e.target.value as keyof typeof PLAYBACK_SPEEDS)}
        >
          <option value="slow">0.5x</option>
          <option value="normal">1x</option>
          <option value="fast">2x</option>
        </select>
      </div>

      {/* Event log */}
      <div className="battle-section">
        <h3 className="battle-section-title">Battle Log</h3>
        <EventLog events={displayedEvents} currentUserId={currentUserId} />
      </div>

      {/* Results section (when complete) */}
      {battle.is_complete && currentEventIndex >= allEvents.length && (
        <div className="battle-results">
          <h3 className="battle-results-title">
            {isWinner ? '🎉 Victory!' : '😢 Defeat'}
          </h3>
          <div className="battle-results-stats">
            <div className="stat-item">
              <span className="stat-label">Your Damage</span>
              <span className="stat-value">
                {isPlayerA ? round.player_a_damage : round.player_b_damage}
              </span>
            </div>
            <div className="stat-item">
              <span className="stat-label">Opponent Damage</span>
              <span className="stat-value">
                {isPlayerA ? round.player_b_damage : round.player_a_damage}
              </span>
            </div>
            <div className="stat-item">
              <span className="stat-label">Total Rounds</span>
              <span className="stat-value">{round.total_rounds}</span>
            </div>
          </div>
          <div className="battle-results-actions">
            <button
              className="btn btn-secondary"
              onClick={() => onTabChange?.('lobby')}
            >
              Back to Lobby
            </button>
            <button
              className="btn btn-primary"
              onClick={() => onTabChange?.('stats')}
            >
              View Stats
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
