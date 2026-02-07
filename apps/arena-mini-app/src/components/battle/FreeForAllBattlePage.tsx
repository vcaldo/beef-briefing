/**
 * Free-For-All Battle Page Component
 *
 * Displays round-robin tournament results for odd player counts (3, 5, 7...).
 * Features sequential round list with unlock rules (round N+1 unlocks after
 * round N watched), Watch/Rewatch buttons, final standings after all rounds
 * are viewed, and animated battle replay per round.
 */

import { useState, useCallback, useEffect, useRef } from 'react'
import { apiClient } from '../../api/client'
import { RPGPanel, GameButton } from '../ui'
import { LoadingSpinner, ErrorDisplay } from '../common'
import { CompactCard } from '../common/CompactCard'
import { BattleEffect } from './BattleEffect'
import type { EffectPosition } from './BattleEffect'
import { useBattleAnimation, getCardKey, usePageBackground } from '../../hooks'
import { useSoundContext } from '../../contexts'
import type { TournamentBattlePageProps } from './BracketBattlePage'
import type {
  BattleResult,
  FfaRoundSummary,
  EnhancedTeamCard,
  PlaceholderPositions,
} from '../../types'

/**
 * Determine if a round is unlocked for viewing.
 *
 * Rules:
 * - Round 1 (index 0): always unlocked
 * - Round N+1: unlocked only after round N has been watched
 */
function isRoundUnlocked(
  roundIndex: number,
  rounds: FfaRoundSummary[],
  watchedRounds: Set<number>,
): boolean {
  if (roundIndex === 0) return true
  return watchedRounds.has(rounds[roundIndex - 1].round_number)
}

/**
 * Check if all rounds have been watched.
 */
function allRoundsWatched(rounds: FfaRoundSummary[], watchedRounds: Set<number>): boolean {
  return rounds.every((r) => watchedRounds.has(r.round_number))
}

/**
 * Get the champion's name from standings.
 */
function getChampionName(standings?: { rank: number; name: string }[]): string | null {
  if (!standings?.length) return null
  const champion = standings.find((s) => s.rank === 1)
  return champion?.name ?? null
}

/**
 * Render a single round card in the round list.
 */
function FfaRoundCard({
  round,
  unlocked,
  watched,
  onWatch,
}: {
  round: FfaRoundSummary
  unlocked: boolean
  watched: boolean
  onWatch: (roundNumber: number) => void
}) {
  if (!unlocked) {
    return (
      <div className="ffa-round-card">
        <div className="ffa-round-card-locked">
          <span className="ffa-round-card-lock-icon">&#x1F512;</span>
          Round {round.round_number}
        </div>
        <div className="ffa-round-card-actions">
          <GameButton variant="neutral" size="sm" disabled>
            Locked
          </GameButton>
        </div>
      </div>
    )
  }

  const winnerName = round.winner_id === round.player_a_id
    ? round.player_a_name
    : round.winner_id === round.player_b_id
      ? round.player_b_name
      : null

  return (
    <div className="ffa-round-card">
      <div className="ffa-round-card-header">
        Round {round.round_number}
      </div>
      <div className="ffa-round-card-players">
        <span
          className={`ffa-round-card-player${
            watched && round.winner_id != null
              ? round.winner_id === round.player_a_id ? ' is-winner' : ' is-loser'
              : ''
          }`}
        >
          {round.player_a_name}
        </span>
        <span className="ffa-round-card-vs">vs</span>
        <span
          className={`ffa-round-card-player${
            watched && round.winner_id != null
              ? round.winner_id === round.player_b_id ? ' is-winner' : ' is-loser'
              : ''
          }`}
        >
          {round.player_b_name}
        </span>
      </div>

      {watched && winnerName && (
        <div className="ffa-round-card-result">
          Winner: {winnerName} &bull; {round.player_a_damage} - {round.player_b_damage} dmg
        </div>
      )}
      {watched && round.is_draw && (
        <div className="ffa-round-card-result">
          Draw &bull; {round.player_a_damage} - {round.player_b_damage} dmg
        </div>
      )}

      <div className="ffa-round-card-actions">
        <GameButton
          variant={watched ? 'secondary' : 'primary'}
          size="sm"
          onClick={() => onWatch(round.round_number)}
        >
          {watched ? 'Rewatch' : 'Watch Battle'}
        </GameButton>
      </div>
    </div>
  )
}

// ─── Battle Replay Sub-component ──────────────────────────────────────────────

interface FfaReplayViewProps {
  matchId: string
  roundNumber: number
  userId: number
  onComplete: () => void
  onBack: () => void
}

/**
 * Renders a full animated battle replay for a single FFA round.
 * Fetches round data, runs the battle animation, and shows result on completion.
 */
function FfaReplayView({
  matchId,
  roundNumber,
  userId,
  onComplete,
  onBack,
}: FfaReplayViewProps) {
  const [roundData, setRoundData] = useState<BattleResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const { play: playSound, preloadCategory } = useSoundContext()
  const soundsPreloadedRef = useRef(false)

  // Apply arena background
  usePageBackground({ backgroundId: 'arena' })

  // Refs for card position tracking (needed for battle effects)
  const battleArenaRef = useRef<HTMLDivElement>(null)
  const cardRefsMap = useRef<Map<string, HTMLDivElement>>(new Map())

  const getCardPosition = useCallback((cardKey: string): EffectPosition | null => {
    const cardElement = cardRefsMap.current.get(cardKey)
    const arenaElement = battleArenaRef.current
    if (!cardElement || !arenaElement) return null

    const cardRect = cardElement.getBoundingClientRect()
    const arenaRect = arenaElement.getBoundingClientRect()
    return {
      x: cardRect.left - arenaRect.left + cardRect.width / 2,
      y: cardRect.top - arenaRect.top + cardRect.height / 2,
    }
  }, [])

  // Animation hook
  const {
    cardStates,
    animationStates,
    currentEventIndex,
    isPlaying,
    isComplete,
    currentDamage,
    damageTargetKey,
    activeEffects,
    onEffectComplete,
    arenaCardA,
    arenaCardB,
    transitioningCards,
    play,
  } = useBattleAnimation(roundData, {
    playbackSpeed: 1,
    playerAId: roundData?.player_a_id ?? 0,
    playerBId: roundData?.player_b_id ?? 0,
    onPlaySound: playSound,
    getCardPosition,
  })

  // Fetch round battle data
  const fetchRound = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await apiClient.getRoundBattle(matchId, roundNumber)
      setRoundData(data)
    } catch (err) {
      console.error('Failed to fetch round battle:', err)
      setError(err instanceof Error ? err.message : 'Failed to load battle')
    } finally {
      setLoading(false)
    }
  }, [matchId, roundNumber])

  useEffect(() => {
    fetchRound()
  }, [fetchRound])

  // Preload battle sounds
  useEffect(() => {
    if (!soundsPreloadedRef.current) {
      soundsPreloadedRef.current = true
      preloadCategory('battle').catch((err) => {
        console.warn('Failed to preload battle sounds:', err)
      })
    }
  }, [preloadCategory])

  // Auto-play after data loads
  useEffect(() => {
    if (roundData && currentEventIndex === -1 && !isPlaying && !isComplete) {
      const timer = setTimeout(() => play(), 1500)
      return () => clearTimeout(timer)
    }
  }, [roundData, currentEventIndex, isPlaying, isComplete, play])

  // Play win/lose/draw sound on completion
  useEffect(() => {
    if (isComplete && roundData) {
      if (roundData.is_draw) {
        playSound('arena_battle_draw')
      } else {
        const isUserWinner = roundData.winner_id === userId
        playSound(isUserWinner ? 'arena_battle_win' : 'arena_battle_lose')
      }
    }
  }, [isComplete, roundData, userId, playSound])

  // Notify parent of completion (marks round as watched)
  const completeFired = useRef(false)
  useEffect(() => {
    if (isComplete && !completeFired.current) {
      completeFired.current = true
      onComplete()
    }
  }, [isComplete, onComplete])

  // Loading state
  if (loading) {
    return <LoadingSpinner message="Loading battle..." fullScreen />
  }

  // Error state
  if (error || !roundData) {
    return (
      <ErrorDisplay
        title="Battle Error"
        message={error || 'No battle data available'}
        onRetry={fetchRound}
      />
    )
  }

  // Check if a card is currently in the arena
  const isCardInArena = (cardId: number, teamOwnerId: number): boolean => {
    if (arenaCardA && arenaCardA.cardId === cardId && arenaCardA.teamOwnerId === teamOwnerId) return true
    if (arenaCardB && arenaCardB.cardId === cardId && arenaCardB.teamOwnerId === teamOwnerId) return true
    return false
  }

  // Render a deck card
  const renderDeckCard = (
    cardId: number,
    teamOwnerId: number,
    originalCard: {
      name: string
      photo_url?: string
      username?: string
      card_image_url?: string
      placeholder_positions?: PlaceholderPositions | null
      position: number
    },
  ) => {
    const cardKey = getCardKey(teamOwnerId, cardId)
    const state = cardStates.get(cardKey)
    if (!state) return null

    if (isCardInArena(cardId, teamOwnerId)) {
      return <div key={cardId} className="deck-card-placeholder" />
    }

    return (
      <div key={cardId} className={`deck-compact-card ${!state.is_alive ? 'dead' : ''}`}>
        <span className="deck-card-order">{originalCard.position + 1}</span>
        <CompactCard
          imageUrl={originalCard.card_image_url || ''}
          positions={originalCard.placeholder_positions}
          currentStats={{ atk: state.atk, hp: state.hp, maxHp: state.max_hp }}
          isDead={!state.is_alive}
          cardName={state.name}
          cardId={cardId}
        />
      </div>
    )
  }

  // Render an arena card (left or right)
  const renderArenaCard = (
    arenaCard: { cardId: number; teamOwnerId: number },
    side: 'left' | 'right',
    team: 'a' | 'b',
  ) => {
    const cardKey = getCardKey(arenaCard.teamOwnerId, arenaCard.cardId)
    const state = cardStates.get(cardKey)
    const animState = animationStates.get(cardKey)
    const cards = team === 'a' ? roundData.team_a_final?.cards : roundData.team_b_final?.cards
    const originalCard = cards?.find((c) => c.card_id === arenaCard.cardId) as EnhancedTeamCard | undefined
    if (!state || !originalCard) return null

    const isDamageTarget = damageTargetKey === cardKey
    const damageToShow = isDamageTarget && currentDamage !== null ? currentDamage : undefined
    const transitionClass = transitioningCards.exiting.has(cardKey)
      ? 'arena-card-exiting'
      : transitioningCards.entering.has(cardKey)
        ? 'arena-card-entering'
        : ''

    const setCardRef = (el: HTMLDivElement | null) => {
      if (el) cardRefsMap.current.set(cardKey, el)
      else cardRefsMap.current.delete(cardKey)
    }

    return (
      <div key={cardKey} className={`arena-card arena-card-${side} ${transitionClass}`} ref={setCardRef}>
        <CompactCard
          imageUrl={originalCard.card_image_url || ''}
          positions={originalCard.placeholder_positions}
          currentStats={{ atk: state.atk, hp: state.hp, maxHp: state.max_hp }}
          animationState={animState}
          damageNumber={damageToShow}
          isDead={!state.is_alive}
          cardName={state.name}
          cardId={arenaCard.cardId}
          className="arena-compact-card"
        />
      </div>
    )
  }

  const playerALabel = userId === roundData.player_a_id ? 'You' : roundData.player_a_name
  const playerBLabel = userId === roundData.player_b_id ? 'You' : roundData.player_b_name

  return (
    <div className="battle-page arena-layout page-bg page-bg--arena">
      {/* Team B Deck - Upper Left */}
      <div className="battle-deck battle-deck-b">
        <div className="deck-label">{playerBLabel}</div>
        <div className="deck-cards">
          {[...(roundData.team_b_final?.cards ?? [])].reverse().map((card) =>
            renderDeckCard(card.card_id, roundData.player_b_id, card),
          )}
        </div>
      </div>

      {/* Central Battle Arena */}
      <div
        ref={battleArenaRef}
        className={`battle-arena-center battle-effect-container${!isPlaying ? ' arena-paused' : ''}`}
      >
        {arenaCardB && renderArenaCard(arenaCardB, 'left', 'b')}
        {arenaCardA && arenaCardB && <div className="arena-vs">VS</div>}
        {arenaCardA && renderArenaCard(arenaCardA, 'right', 'a')}

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

      {/* Team A Deck - Bottom Right */}
      <div className="battle-deck battle-deck-a">
        <div className="deck-label">{playerALabel}</div>
        <div className="deck-cards">
          {[...(roundData.team_a_final?.cards ?? [])].map((card) =>
            renderDeckCard(card.card_id, roundData.player_a_id, card),
          )}
        </div>
      </div>

      {/* Result overlay on completion */}
      {isComplete && (
        <div className="ffa-replay-result">
          <RPGPanel variant="outer" className="ffa-replay-result-panel">
            <RPGPanel variant="inner">
              <div className="ffa-replay-result-content">
                <div className="ffa-replay-result-icon">
                  {roundData.is_draw ? '\u{1F91D}' : '\u{2694}\u{FE0F}'}
                </div>
                <h3>
                  {roundData.is_draw
                    ? 'Draw!'
                    : `${roundData.winner_id === roundData.player_a_id ? roundData.player_a_name : roundData.player_b_name} Wins!`}
                </h3>
                <p className="ffa-replay-result-stats">
                  {roundData.team_a_damage} - {roundData.team_b_damage} damage
                </p>
              </div>
            </RPGPanel>
            <GameButton variant="primary" onClick={onBack}>
              Back to Rounds
            </GameButton>
          </RPGPanel>
        </div>
      )}
    </div>
  )
}

// ─── Main Free-For-All Page ──────────────────────────────────────────────────

export function FreeForAllBattlePage({
  userId,
  activeMatch,
  battleData,
  onNavigateToLobby,
  onMatchChange,
}: TournamentBattlePageProps) {
  const [watchedRounds, setWatchedRounds] = useState<Set<number>>(new Set())
  const [viewingRound, setViewingRound] = useState<number | null>(null)

  const handleReturnToLobby = useCallback(() => {
    onMatchChange?.(null)
    onNavigateToLobby?.()
  }, [onMatchChange, onNavigateToLobby])

  const handleWatchRound = useCallback((roundNumber: number) => {
    setViewingRound(roundNumber)
  }, [])

  const handleReplayComplete = useCallback(() => {
    if (viewingRound !== null) {
      setWatchedRounds((prev) => {
        const next = new Set(prev)
        next.add(viewingRound)
        return next
      })
    }
  }, [viewingRound])

  const handleBackToRounds = useCallback(() => {
    setViewingRound(null)
  }, [])

  // When viewing a round, render the replay view
  if (viewingRound !== null && activeMatch) {
    return (
      <FfaReplayView
        matchId={activeMatch.id}
        roundNumber={viewingRound}
        userId={userId}
        onComplete={handleReplayComplete}
        onBack={handleBackToRounds}
      />
    )
  }

  if (!battleData.ffa_rounds) {
    return <LoadingSpinner message="Loading rounds..." fullScreen />
  }

  const rounds = battleData.ffa_rounds
  const allWatched = allRoundsWatched(rounds, watchedRounds)
  const championName = getChampionName(battleData.standings)

  return (
    <div className="battle-page ffa-page page-bg page-bg--arena">
      <RPGPanel variant="outer">
        {/* Header */}
        <RPGPanel variant="inner">
          <div className="ffa-page-header">
            <h2>Free-For-All</h2>
            <p>
              {rounds.length} round{rounds.length !== 1 ? 's' : ''}
              {' \u2022 '}
              {battleData.standings?.length ?? 0} players
            </p>
          </div>
        </RPGPanel>

        {/* Round list */}
        {rounds.map((round, roundIndex) => {
          const unlocked = isRoundUnlocked(roundIndex, rounds, watchedRounds)
          const watched = watchedRounds.has(round.round_number)

          return (
            <RPGPanel key={round.round_number} variant="inner" className="ffa-round">
              <FfaRoundCard
                round={round}
                unlocked={unlocked}
                watched={watched}
                onWatch={handleWatchRound}
              />
            </RPGPanel>
          )
        })}

        {/* Champion banner — shown after all rounds are watched */}
        {allWatched && championName && (
          <RPGPanel variant="inner-blue">
            <div className="ffa-champion">
              <div className="ffa-champion-icon">&#x1F3C6;</div>
              <h3 className="ffa-champion-title">Champion</h3>
              <p className="ffa-champion-name">{championName}</p>
            </div>
          </RPGPanel>
        )}

        {/* Final Standings — visible after all rounds watched */}
        {allWatched && battleData.standings && battleData.standings.length > 0 && (
          <RPGPanel variant="inner">
            <div className="ffa-standings">
              <h3 className="ffa-standings-title">Final Standings</h3>
              {battleData.standings.map((s) => (
                <div key={s.user_id} className="ffa-standing-row">
                  <span className={`ffa-standing-rank rank-${s.rank}`}>
                    #{s.rank}
                  </span>
                  <span className="ffa-standing-name">{s.name}</span>
                  <span className="ffa-standing-stats">
                    {s.wins}W/{s.losses}L &bull; {s.total_damage_dealt} dmg
                  </span>
                </div>
              ))}
            </div>
          </RPGPanel>
        )}

        {/* Actions */}
        <div className="ffa-actions">
          <GameButton variant="primary" onClick={handleReturnToLobby}>
            Return to Lobby
          </GameButton>
        </div>
      </RPGPanel>
    </div>
  )
}

export default FreeForAllBattlePage
