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

import { useState, useEffect, useCallback, useRef } from 'react'
import { apiClient } from '../../api/client'
import { addPageAction } from '@beef-briefing/shared-mini-app/monitoring'
import { LoadingSpinner, ErrorDisplay } from '../common'
import { CompactCard } from '../common/CompactCard'
import { BattleLog } from './BattleLog'
import { BattleEffect } from './BattleEffect'
import type { EffectPosition } from './BattleEffect'
import { useBattleAnimation, getCardKey } from '../../hooks'
import { useSoundContext } from '../../contexts'
import type {
  BattleResult,
  BattleEvent,
  Match,
  GameConstants,
  PlaceholderPositions,
} from '../../types'

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
  gameConstants: GameConstants | null
  onNavigateToStats?: () => void
  onNavigateToLobby?: () => void
  onMatchChange?: (match: Match | null) => void
}

export function BattlePage({
  userId,
  activeMatch,
  gameConstants,
  onNavigateToStats,
  onNavigateToLobby,
  onMatchChange,
}: BattlePageProps) {
  // Battle data state
  const [battleData, setBattleData] = useState<BattleResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Playback speed UI state (index into PLAYBACK_SPEEDS array)
  const [playbackSpeedIndex, setPlaybackSpeedIndex] = useState(1) // Default to 1x

  // Victory screen state (separate from animation completion for UI control)
  const [showVictory, setShowVictory] = useState(false)

  // Sound context for battle audio
  const { play: playSound, preloadCategory } = useSoundContext()
  const soundsPreloadedRef = useRef(false)

  // Refs for card positions (used to place battle effects)
  const battleArenaRef = useRef<HTMLDivElement>(null)
  const cardRefsMap = useRef<Map<string, HTMLDivElement>>(new Map())

  /**
   * Get the center position of a card element relative to the battle arena.
   * Used by useBattleAnimation to position battle effects.
   */
  const getCardPosition = useCallback((cardKey: string): EffectPosition | null => {
    const cardElement = cardRefsMap.current.get(cardKey)
    const arenaElement = battleArenaRef.current

    if (!cardElement || !arenaElement) {
      return null
    }

    const cardRect = cardElement.getBoundingClientRect()
    const arenaRect = arenaElement.getBoundingClientRect()

    // Calculate center position relative to arena
    const x = cardRect.left - arenaRect.left + cardRect.width / 2
    const y = cardRect.top - arenaRect.top + cardRect.height / 2

    return { x, y }
  }, [])

  // Animation state machine hook
  const {
    cardStates,
    animationStates,
    currentEventIndex,
    currentPhase,
    isPlaying,
    isComplete,
    currentDamage,
    damageTargetKey,
    activeEffect,
    onEffectComplete,
    play,
    pause,
    reset,
    skipToEnd: hookSkipToEnd,
  } = useBattleAnimation(battleData, {
    // Convert interval-based speed to multiplier (1000ms = 1x, 500ms = 2x)
    playbackSpeed: 1000 / PLAYBACK_SPEEDS[playbackSpeedIndex].value,
    playerAId: battleData?.player_a_id ?? 0,
    playerBId: battleData?.player_b_id ?? 0,
    onPlaySound: playSound,
    getCardPosition,
  })

  // Get match ID
  const matchId = activeMatch?.id

  // Fetch battle data (card state initialization is handled by the hook)
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

      // Validate required data is present
      if (!data.team_a_final?.cards || !data.team_b_final?.cards) {
        throw new Error('Battle data is incomplete - missing team cards')
      }

      // Set battle data - the useBattleAnimation hook will initialize card states
      setBattleData(data)

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

  // Preload battle sounds on mount
  useEffect(() => {
    if (!soundsPreloadedRef.current) {
      soundsPreloadedRef.current = true
      preloadCategory('battle').catch((err) => {
        console.warn('Failed to preload battle sounds:', err)
      })
    }
  }, [preloadCategory])

  // Auto-play battle after initial load
  useEffect(() => {
    if (battleData && currentEventIndex === -1 && !isPlaying && !isComplete) {
      const autoPlayTimer = setTimeout(() => {
        play()
        addPageAction('battle_autoplay_started', { match_id: matchId })
      }, 1500) // 1.5 second delay

      return () => clearTimeout(autoPlayTimer)
    }
  }, [battleData, currentEventIndex, isPlaying, isComplete, matchId, play])

  // Show victory screen when animation completes and play win/lose sound
  useEffect(() => {
    if (isComplete && !showVictory) {
      setShowVictory(true)
      addPageAction('battle_playback_complete', { match_id: matchId })

      // Play win/lose sound based on battle outcome
      if (battleData) {
        const isUserWinner = battleData.winner_id === userId
        const isDraw = battleData.is_draw
        if (!isDraw) {
          playSound(isUserWinner ? 'battle_win' : 'battle_lose')
        }
      }
    }
  }, [isComplete, showVictory, matchId, battleData, userId, playSound])

  // Handle play/pause toggle
  const togglePlayback = useCallback(() => {
    if (isPlaying) {
      pause()
    } else {
      play()
    }
  }, [isPlaying, play, pause])

  // Skip to end wrapper (shows victory screen)
  const skipToEnd = useCallback(() => {
    hookSkipToEnd()
    setShowVictory(true)
    addPageAction('battle_skipped_to_end', { match_id: matchId })
  }, [hookSkipToEnd, matchId])

  // Reset playback wrapper (hides victory screen)
  const resetPlayback = useCallback(() => {
    reset()
    setShowVictory(false)
    addPageAction('battle_reset', { match_id: matchId })
  }, [reset, matchId])

  // Handle return to lobby
  const handleReturnToLobby = useCallback(() => {
    if (onMatchChange) {
      onMatchChange(null)
    }
    if (onNavigateToLobby) {
      onNavigateToLobby()
    }
  }, [onMatchChange, onNavigateToLobby])

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

  // Render a battle card
  const renderBattleCard = (
    cardId: number,
    teamOwnerId: number,
    originalCard: {
      name: string
      photo_url?: string
      username?: string
      card_image_url?: string
      placeholder_positions?: PlaceholderPositions | null
    }
  ) => {
    // Use composite key to look up state (handles same card on both teams)
    const cardKey = getCardKey(teamOwnerId, cardId)
    const state = cardStates.get(cardKey)
    if (!state) return null

    // Get animation state from the animation state machine
    const animState = animationStates.get(cardKey)

    // Check if this card is currently taking damage and should show damage number
    const isDamageTarget = damageTargetKey === cardKey
    const damageToShow =
      isDamageTarget && currentDamage !== null ? currentDamage : undefined

    const isCurrentUser = teamOwnerId === userId

    // Ref callback to register card element for effect positioning
    const setCardRef = (el: HTMLDivElement | null) => {
      if (el) {
        cardRefsMap.current.set(cardKey, el)
      } else {
        cardRefsMap.current.delete(cardKey)
      }
    }

    return (
      <div key={cardId} className="battle-card-wrapper" ref={setCardRef}>
        <div className={`battle-card-owner-badge ${isCurrentUser ? 'you' : ''}`}>
          {isCurrentUser ? 'You' : ''}
        </div>
        <CompactCard
          imageUrl={originalCard.card_image_url || ''}
          positions={originalCard.placeholder_positions}
          currentStats={{
            atk: state.atk,
            def: 0,
            hp: state.hp,
            maxHp: state.max_hp,
          }}
          hpBarThresholds={gameConstants?.hp_bar_thresholds}
          animationState={animState}
          damageNumber={damageToShow}
          isDead={!state.is_alive}
          showHpBar={true}
          cardName={state.name}
          cardId={cardId}
          className="battle-compact-card"
        />
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
      <div
        ref={battleArenaRef}
        className="battle-arena battle-effect-container"
        style={{
          // Scale HP bar transition duration with playback speed
          // At 1x (value=1000), duration is 300ms; at 2x (value=500), duration is 150ms
          '--hp-transition-duration': `${(PLAYBACK_SPEEDS[playbackSpeedIndex].value / 1000) * 300}ms`,
        } as React.CSSProperties}
      >
        {/* Team A (opponent or self) */}
        <div className="battle-team team-a">
          <div className="team-label">{battleData.player_a_name}</div>
          <div className="team-cards">
            {(battleData.team_a_final?.cards ?? []).map((card) =>
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
            {(battleData.team_b_final?.cards ?? []).map((card) =>
              renderBattleCard(card.card_id, battleData.player_b_id, card)
            )}
          </div>
        </div>

        {/* Battle Effect - renders animated effect at card positions */}
        {activeEffect && (
          <BattleEffect
            type={activeEffect.type}
            position={activeEffect.position}
            onComplete={onEffectComplete}
            size={80}
          />
        )}
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
      <BattleLog
        events={battleData.events}
        currentEventIndex={currentEventIndex}
        currentPhase={currentPhase}
        getEventMessage={getEventMessage}
      />

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
