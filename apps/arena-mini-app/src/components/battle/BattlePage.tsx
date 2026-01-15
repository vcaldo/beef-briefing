/**
 * Battle Page Component
 *
 * Displays battle results with event playback, card animations, and HP bar updates.
 * Features:
 * - Fetch battle results on mount
 * - Sequential event playback with configurable speed
 * - HP bar animations with color transitions
 * - Attack/defense highlighting
 * - Victory screen overlay
 */

import { useState, useEffect, useRef, useCallback } from 'react'
import { apiClient } from '../../api/client'
import { addPageAction } from '@beef-briefing/shared-mini-app/monitoring'
import { LoadingSpinner, ErrorDisplay } from '../common'
import type { BattleResult, BattleEvent, CardSnapshot, Match } from '../../types'

// Playback speed options (events per second)
const PLAYBACK_SPEEDS = [
  { label: '0.5x', value: 2000 },
  { label: '1x', value: 1000 },
  { label: '1.5x', value: 667 },
  { label: '2x', value: 500 },
]

interface BattlePageProps {
  userId: number
  activeMatch: Match | null
  onNavigateToStats?: () => void
  onMatchChange?: (match: Match | null) => void
}

export function BattlePage({
  userId,
  activeMatch,
  onNavigateToStats,
  onMatchChange,
}: BattlePageProps) {
  // Battle data state
  const [battleData, setBattleData] = useState<BattleResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Playback state
  const [currentEventIndex, setCurrentEventIndex] = useState(-1)
  const [isPlaying, setIsPlaying] = useState(false)
  const [playbackSpeedIndex, setPlaybackSpeedIndex] = useState(1) // Default to 1x

  // Card states for animation
  const [cardStates, setCardStates] = useState<Map<number, CardSnapshot>>(new Map())

  // Victory screen state
  const [showVictory, setShowVictory] = useState(false)

  // Refs for cleanup
  const playbackIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const eventLogRef = useRef<HTMLDivElement>(null)

  // Get match ID
  const matchId = activeMatch?.id

  // Fetch battle data
  const fetchBattle = useCallback(async () => {
    if (!matchId) {
      setError('No active match')
      setLoading(false)
      return
    }

    try {
      setLoading(true)
      setError(null)

      const data = await apiClient.getBattle(matchId)
      setBattleData(data)

      // Initialize card states from initial team data
      const initialStates = new Map<number, CardSnapshot>()

      // Team A cards
      data.team_a_final.cards.forEach((card, index) => {
        initialStates.set(card.card_id, {
          card_id: card.card_id,
          user_id: card.user_id,
          name: card.name,
          hp: card.max_hp,
          max_hp: card.max_hp,
          atk: card.atk,
          position: index,
          is_alive: true,
          is_attacking: false,
          is_defending: false,
        })
      })

      // Team B cards
      data.team_b_final.cards.forEach((card, index) => {
        initialStates.set(card.card_id, {
          card_id: card.card_id,
          user_id: card.user_id,
          name: card.name,
          hp: card.max_hp,
          max_hp: card.max_hp,
          atk: card.atk,
          position: index,
          is_alive: true,
          is_attacking: false,
          is_defending: false,
        })
      })

      setCardStates(initialStates)

      addPageAction('battle_loaded', {
        match_id: matchId,
        num_events: data.events.length,
        winner_id: data.winner_id,
      })
    } catch (err) {
      console.error('Failed to fetch battle:', err)
      setError(err instanceof Error ? err.message : 'Failed to load battle')
      addPageAction('battle_load_error', {
        match_id: matchId,
        error: err instanceof Error ? err.message : 'unknown',
      })
    } finally {
      setLoading(false)
    }
  }, [matchId])

  // Fetch on mount
  useEffect(() => {
    fetchBattle()
  }, [fetchBattle])

  // Apply event to card states
  const applyEvent = useCallback(
    (event: BattleEvent, eventIndex: number) => {
      // If event has card_states, use them directly
      if (event.card_states && event.card_states.length > 0) {
        const newStates = new Map<number, CardSnapshot>()
        event.card_states.forEach((state) => {
          newStates.set(state.card_id, state)
        })
        setCardStates(newStates)
        return
      }

      // Otherwise, update based on event type
      setCardStates((prevStates) => {
        const newStates = new Map(prevStates)

        // Clear attack/defend flags first
        newStates.forEach((state, cardId) => {
          newStates.set(cardId, {
            ...state,
            is_attacking: false,
            is_defending: false,
          })
        })

        if (event.type === 'attack' || event.type === 'damage') {
          // Mark attacker
          if (event.attacker_card_id) {
            const attacker = newStates.get(event.attacker_card_id)
            if (attacker) {
              newStates.set(event.attacker_card_id, {
                ...attacker,
                is_attacking: true,
              })
            }
          }

          // Mark defender and update HP
          if (event.defender_card_id) {
            const defender = newStates.get(event.defender_card_id)
            if (defender) {
              newStates.set(event.defender_card_id, {
                ...defender,
                is_defending: true,
                hp: event.hp_after ?? defender.hp,
              })
            }
          }
        } else if (event.type === 'death') {
          // Mark card as dead
          if (event.defender_card_id) {
            const deadCard = newStates.get(event.defender_card_id)
            if (deadCard) {
              newStates.set(event.defender_card_id, {
                ...deadCard,
                is_alive: false,
                hp: 0,
              })
            }
          }
        }

        return newStates
      })

      // Track event in analytics
      addPageAction('battle_event_played', {
        event_index: eventIndex,
        event_type: event.type,
      })
    },
    []
  )

  // Playback control - advance to next event
  const advanceEvent = useCallback(() => {
    if (!battleData) return

    setCurrentEventIndex((prevIndex) => {
      const nextIndex = prevIndex + 1

      if (nextIndex >= battleData.events.length) {
        // Playback complete
        setIsPlaying(false)
        setShowVictory(true)
        addPageAction('battle_playback_complete', {
          match_id: matchId,
        })
        return prevIndex
      }

      // Apply the event
      applyEvent(battleData.events[nextIndex], nextIndex)
      return nextIndex
    })
  }, [battleData, matchId, applyEvent])

  // Handle play/pause
  const togglePlayback = useCallback(() => {
    setIsPlaying((prev) => !prev)
  }, [])

  // Start/stop playback interval
  useEffect(() => {
    if (isPlaying && battleData) {
      const speed = PLAYBACK_SPEEDS[playbackSpeedIndex].value
      playbackIntervalRef.current = setInterval(advanceEvent, speed)
    } else {
      if (playbackIntervalRef.current) {
        clearInterval(playbackIntervalRef.current)
        playbackIntervalRef.current = null
      }
    }

    return () => {
      if (playbackIntervalRef.current) {
        clearInterval(playbackIntervalRef.current)
      }
    }
  }, [isPlaying, battleData, playbackSpeedIndex, advanceEvent])

  // Auto-scroll event log
  useEffect(() => {
    if (eventLogRef.current && currentEventIndex >= 0) {
      const activeItem = eventLogRef.current.querySelector('.current')
      if (activeItem) {
        activeItem.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
      }
    }
  }, [currentEventIndex])

  // Skip to end
  const skipToEnd = useCallback(() => {
    if (!battleData) return

    setIsPlaying(false)
    setCurrentEventIndex(battleData.events.length - 1)

    // Apply final states
    const lastEvent = battleData.events[battleData.events.length - 1]
    if (lastEvent) {
      applyEvent(lastEvent, battleData.events.length - 1)
    }

    setShowVictory(true)
    addPageAction('battle_skipped_to_end', { match_id: matchId })
  }, [battleData, matchId, applyEvent])

  // Reset playback
  const resetPlayback = useCallback(() => {
    if (!battleData) return

    setCurrentEventIndex(-1)
    setIsPlaying(false)
    setShowVictory(false)

    // Reset card states to initial
    const initialStates = new Map<number, CardSnapshot>()
    battleData.team_a_final.cards.forEach((card, index) => {
      initialStates.set(card.card_id, {
        card_id: card.card_id,
        user_id: card.user_id,
        name: card.name,
        hp: card.max_hp,
        max_hp: card.max_hp,
        atk: card.atk,
        position: index,
        is_alive: true,
        is_attacking: false,
        is_defending: false,
      })
    })
    battleData.team_b_final.cards.forEach((card, index) => {
      initialStates.set(card.card_id, {
        card_id: card.card_id,
        user_id: card.user_id,
        name: card.name,
        hp: card.max_hp,
        max_hp: card.max_hp,
        atk: card.atk,
        position: index,
        is_alive: true,
        is_attacking: false,
        is_defending: false,
      })
    })
    setCardStates(initialStates)

    addPageAction('battle_reset', { match_id: matchId })
  }, [battleData, matchId])

  // Handle return to lobby
  const handleReturnToLobby = useCallback(() => {
    if (onMatchChange) {
      onMatchChange(null)
    }
  }, [onMatchChange])

  // Loading state
  if (loading) {
    return (
      <div className="battle-page">
        <LoadingSpinner message="Loading battle..." fullScreen />
      </div>
    )
  }

  // Error state
  if (error || !battleData) {
    return (
      <div className="battle-page">
        <ErrorDisplay
          title="Battle Error"
          message={error || 'No battle data available'}
          onRetry={fetchBattle}
        />
      </div>
    )
  }

  // Determine if current user is player A or B
  const isPlayerA = userId === battleData.player_a_id
  const isWinner =
    battleData.winner_id === userId ||
    (battleData.is_draw && false) // No winner on draw
  const isDraw = battleData.is_draw

  // Get HP bar color based on percentage
  const getHpColor = (hp: number, maxHp: number): string => {
    const pct = (hp / maxHp) * 100
    if (pct > 66) return 'var(--hp-full)'
    if (pct > 33) return 'var(--hp-medium)'
    return 'var(--hp-low)'
  }

  // Render a battle card
  const renderBattleCard = (
    cardId: number,
    teamOwnerId: number,
    originalCard: { name: string; photo_url?: string; username?: string }
  ) => {
    const state = cardStates.get(cardId)
    if (!state) return null

    const hpPercent = Math.max(0, (state.hp / state.max_hp) * 100)
    const hpColor = getHpColor(state.hp, state.max_hp)
    const isCurrentUser = teamOwnerId === userId

    return (
      <div
        key={cardId}
        className={`battle-card ${!state.is_alive ? 'dead' : ''} ${state.is_attacking ? 'attacking' : ''} ${state.is_defending ? 'defending' : ''}`}
        data-card-id={cardId}
        data-is-alive={state.is_alive}
        data-is-attacking={state.is_attacking}
        data-is-defending={state.is_defending}
      >
        <div className={`battle-card-owner ${isCurrentUser ? 'you' : ''}`}>
          {isCurrentUser ? 'You' : ''}
        </div>
        <div className="battle-card-avatar">
          {originalCard.photo_url ? (
            <img src={originalCard.photo_url} alt={state.name} />
          ) : (
            <div className="battle-card-avatar-placeholder">
              {state.name.charAt(0).toUpperCase()}
            </div>
          )}
        </div>
        <div className="battle-card-name">{state.name}</div>
        <div className="battle-card-stats">
          <span className="stat atk">⚔️ {state.atk}</span>
          <span className="stat hp">❤️ {state.hp}/{state.max_hp}</span>
        </div>
        <div className="battle-card-hp-bar">
          <div
            className="battle-card-hp-fill"
            style={{
              width: `${hpPercent}%`,
              backgroundColor: hpColor,
              transition: 'width 0.3s ease, background-color 0.3s ease',
            }}
          />
        </div>
      </div>
    )
  }

  // Get event message text
  const getEventMessage = (event: BattleEvent): string => {
    if (event.message) return event.message

    switch (event.type) {
      case 'attack':
        return `Attack! ${event.damage} damage dealt`
      case 'damage':
        return `${event.damage} damage (${event.hp_before} → ${event.hp_after} HP)`
      case 'death':
        return `Card defeated!`
      case 'advance':
        return `Next card advances`
      case 'victory':
        return `Victory!`
      case 'summary':
        return `Round ${event.round} summary`
      default:
        return `Event: ${event.type}`
    }
  }

  return (
    <div className="battle-page">
      {/* Header */}
      <div className="battle-header">
        <h1 className="battle-title">Battle Arena</h1>
        <div className="battle-info">
          <span className="battle-round">Round {battleData.num_rounds}</span>
          <span className="battle-score">
            {battleData.team_a_damage} - {battleData.team_b_damage}
          </span>
        </div>
      </div>

      {/* Battle Arena */}
      <div className="battle-arena">
        {/* Team A (opponent or self) */}
        <div className="battle-team team-a">
          <div className="team-label">{battleData.player_a_name}</div>
          <div className="team-cards">
            {battleData.team_a_final.cards.map((card) =>
              renderBattleCard(card.card_id, battleData.player_a_id, card)
            )}
          </div>
        </div>

        {/* VS Indicator */}
        <div className="battle-vs">VS</div>

        {/* Team B (opponent or self) */}
        <div className="battle-team team-b">
          <div className="team-label">{battleData.player_b_name}</div>
          <div className="team-cards">
            {battleData.team_b_final.cards.map((card) =>
              renderBattleCard(card.card_id, battleData.player_b_id, card)
            )}
          </div>
        </div>
      </div>

      {/* Playback Controls */}
      <div className="battle-controls">
        <button
          className="btn-ghost control-btn"
          onClick={resetPlayback}
          disabled={currentEventIndex < 0}
          aria-label="Reset"
        >
          ⏮️
        </button>

        <button
          className={`btn-primary control-btn play-btn ${isPlaying ? 'playing' : ''}`}
          onClick={togglePlayback}
          aria-label={isPlaying ? 'Pause' : 'Play'}
        >
          {isPlaying ? '⏸️' : '▶️'}
        </button>

        <button
          className="btn-ghost control-btn"
          onClick={skipToEnd}
          disabled={currentEventIndex >= battleData.events.length - 1}
          aria-label="Skip to end"
        >
          ⏭️
        </button>

        {/* Speed selector */}
        <div className="speed-selector">
          {PLAYBACK_SPEEDS.map((speed, index) => (
            <button
              key={speed.label}
              className={`speed-btn ${index === playbackSpeedIndex ? 'active' : ''}`}
              onClick={() => setPlaybackSpeedIndex(index)}
            >
              {speed.label}
            </button>
          ))}
        </div>

        {/* Event counter */}
        <div className="event-counter">
          {currentEventIndex + 1} / {battleData.events.length}
        </div>
      </div>

      {/* Event Log */}
      <div className="event-log" ref={eventLogRef}>
        <div className="event-log-title">Battle Log</div>
        {battleData.events.map((event, index) => (
          <div
            key={index}
            className={`event-log-item ${event.type} ${index === currentEventIndex ? 'current' : ''} ${index < currentEventIndex ? 'played' : ''}`}
          >
            <span className="event-round">R{event.round}</span>
            <span className="event-message">{getEventMessage(event)}</span>
          </div>
        ))}
      </div>

      {/* Victory Screen Overlay */}
      {showVictory && (
        <div className="victory-screen" onClick={() => setShowVictory(false)}>
          <div className="victory-content">
            <div className="victory-icon">
              {isDraw ? '🤝' : isWinner ? '🏆' : '💀'}
            </div>
            <h2 className="victory-title">
              {isDraw
                ? 'Draw!'
                : isWinner
                  ? 'Victory!'
                  : 'Defeat'}
            </h2>
            <p className="victory-subtitle">
              {isDraw
                ? 'Both players fought to a standstill'
                : isWinner
                  ? 'You won the battle!'
                  : `${battleData.winner_id === battleData.player_a_id ? battleData.player_a_name : battleData.player_b_name} wins!`}
            </p>

            <div className="victory-stats">
              <div className="victory-stat">
                <span className="stat-label">Total Damage</span>
                <span className="stat-value">
                  {isPlayerA ? battleData.team_a_damage : battleData.team_b_damage}
                </span>
              </div>
              <div className="victory-stat">
                <span className="stat-label">Rounds</span>
                <span className="stat-value">{battleData.num_rounds}</span>
              </div>
            </div>

            <div className="victory-actions">
              <button
                className="btn-primary"
                onClick={(e) => {
                  e.stopPropagation()
                  setShowVictory(false)
                  resetPlayback()
                }}
              >
                Watch Again
              </button>
              {onNavigateToStats && (
                <button
                  className="btn-secondary"
                  onClick={(e) => {
                    e.stopPropagation()
                    onNavigateToStats()
                  }}
                >
                  View Stats
                </button>
              )}
              <button
                className="btn-ghost"
                onClick={(e) => {
                  e.stopPropagation()
                  handleReturnToLobby()
                }}
              >
                Return to Lobby
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default BattlePage
