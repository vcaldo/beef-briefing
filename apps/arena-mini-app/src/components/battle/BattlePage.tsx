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
import { RPGPanel, GameButton } from '../ui'
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
// Index: 0=1x, 1=1.5x, 2=2x (controlled by parent via speedIndex prop)
const PLAYBACK_SPEEDS = [
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
  /** Playback state from parent (for TabBar sync) */
  isPlaying: boolean
  /** Callback to notify parent of playing state changes */
  onPlayingChange: (playing: boolean) => void
  /** Speed index from parent (0=1x, 1=1.5x, 2=2x) */
  speedIndex: number
  /** Callback to notify parent of battle completion state */
  onCompleteChange?: (complete: boolean) => void
  /** Counter that increments when replay is requested from TabBar */
  replayTrigger?: number
}

export function BattlePage({
  userId,
  activeMatch,
  gameConstants: _gameConstants,
  onNavigateToStats: _onNavigateToStats,
  onNavigateToLobby,
  onMatchChange,
  isPlaying: parentIsPlaying,
  onPlayingChange,
  speedIndex,
  onCompleteChange,
  replayTrigger,
}: BattlePageProps) {
  // Battle data state
  const [battleData, setBattleData] = useState<BattleResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Victory screen state (separate from animation completion for UI control)
  const [showVictory, setShowVictory] = useState(false)
  // Track if user has manually dismissed the victory modal (prevents auto-reopen)
  const [victoryDismissed, setVictoryDismissed] = useState(false)

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
    isPlaying: hookIsPlaying,
    isComplete,
    currentDamage,
    damageTargetKey,
    activeEffects,
    onEffectComplete,
    play,
    pause,
    reset,
  } = useBattleAnimation(battleData, {
    // Convert interval-based speed to multiplier (1000ms = 1x, 500ms = 2x)
    playbackSpeed: 1000 / PLAYBACK_SPEEDS[speedIndex].value,
    playerAId: battleData?.player_a_id ?? 0,
    playerBId: battleData?.player_b_id ?? 0,
    onPlaySound: playSound,
    getCardPosition,
  })

  // Sync parent isPlaying state with hook
  useEffect(() => {
    if (parentIsPlaying && !hookIsPlaying && !isComplete) {
      play()
    } else if (!parentIsPlaying && hookIsPlaying) {
      pause()
    }
  }, [parentIsPlaying, hookIsPlaying, isComplete, play, pause])

  // Notify parent when hook playing state changes
  useEffect(() => {
    onPlayingChange(hookIsPlaying)
  }, [hookIsPlaying, onPlayingChange])

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
    if (battleData && currentEventIndex === -1 && !hookIsPlaying && !isComplete) {
      const autoPlayTimer = setTimeout(() => {
        play()
        onPlayingChange(true)
        addPageAction('battle_autoplay_started', { match_id: matchId })
      }, 1500) // 1.5 second delay

      return () => clearTimeout(autoPlayTimer)
    }
  }, [battleData, currentEventIndex, hookIsPlaying, isComplete, matchId, play, onPlayingChange])

  // Show victory screen when animation completes and play win/lose/draw sound
  useEffect(() => {
    if (isComplete && !showVictory && !victoryDismissed) {
      setShowVictory(true)
      addPageAction('battle_playback_complete', { match_id: matchId })

      // Play win/lose/draw sound based on battle outcome
      if (battleData) {
        const isDraw = battleData.is_draw
        if (isDraw) {
          playSound('arena_battle_draw')
        } else {
          const isUserWinner = battleData.winner_id === userId
          playSound(isUserWinner ? 'arena_battle_win' : 'arena_battle_lose')
        }
      }
    }
  }, [isComplete, showVictory, victoryDismissed, matchId, battleData, userId, playSound])

  // Notify parent when battle completion state changes
  useEffect(() => {
    onCompleteChange?.(isComplete)
  }, [isComplete, onCompleteChange])

  // Dismiss victory modal (user manually closed it)
  const dismissVictory = useCallback(() => {
    setShowVictory(false)
    setVictoryDismissed(true)
  }, [])

  // Reset playback wrapper (hides victory screen and allows it to reopen)
  const resetPlayback = useCallback(() => {
    reset()
    setShowVictory(false)
    setVictoryDismissed(false)
    addPageAction('battle_reset', { match_id: matchId })
  }, [reset, matchId])

  // Handle replay trigger from parent (TabBar replay button)
  const prevReplayTriggerRef = useRef(replayTrigger)
  useEffect(() => {
    if (replayTrigger !== undefined && replayTrigger !== prevReplayTriggerRef.current) {
      prevReplayTriggerRef.current = replayTrigger
      if (replayTrigger > 0) {
        resetPlayback()
      }
    }
  }, [replayTrigger, resetPlayback])

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
            hp: state.hp,
            maxHp: state.max_hp,
          }}
          animationState={animState}
          damageNumber={damageToShow}
          isDead={!state.is_alive}
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
          '--hp-transition-duration': `${(PLAYBACK_SPEEDS[speedIndex].value / 1000) * 300}ms`,
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

        {/* Battle Effects - renders animated effects at card positions (supports multiple simultaneous effects) */}
        {activeEffects.map((effect) => (
          <BattleEffect
            key={`${effect.type}-${effect.cardKey}`}
            type={effect.type}
            position={effect.position}
            onComplete={() => onEffectComplete(effect.cardKey)}
            size={80}
          />
        ))}
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
        <div className="victory-screen" onClick={dismissVictory}>
          <RPGPanel variant="outer" className="rpg-victory-outer">
            <button
              className="victory-close-btn"
              onClick={(e) => {
                e.stopPropagation()
                dismissVictory()
              }}
              aria-label="Close"
            >
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
            <RPGPanel variant="inner-blue" className="rpg-victory-content">
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
            </RPGPanel>

            <RPGPanel variant="inner" className="rpg-victory-stats">
              <div className="victory-stats">
                <div className="victory-stat">
                  <span className="stat-label">Damage Dealt</span>
                  <span className="stat-value">
                    {isPlayerA ? battleData.team_a_damage : battleData.team_b_damage}
                  </span>
                </div>
                <div className="victory-stat">
                  <span className="stat-label">Damage Taken</span>
                  <span className="stat-value">
                    {isPlayerA ? battleData.team_b_damage : battleData.team_a_damage}
                  </span>
                </div>
                <div className="victory-stat">
                  <span className="stat-label">Rounds</span>
                  <span className="stat-value">{battleData.num_rounds}</span>
                </div>
              </div>
            </RPGPanel>

            <div className="rpg-victory-actions">
              <GameButton
                variant="primary"
                onClick={(e) => {
                  e.stopPropagation()
                  handleReturnToLobby()
                }}
              >
                Return to Lobby
              </GameButton>
            </div>
          </RPGPanel>
        </div>
      )}
    </div>
  )
}

export default BattlePage
