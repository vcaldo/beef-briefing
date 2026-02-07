/**
 * Bracket Battle Page Component
 *
 * Displays bracket tournament results for even player counts (4, 6, 8...).
 * Features bracket diagram with elimination rounds, match watch/rewatch buttons,
 * unlock rules (semis open immediately, finals unlock after all prior matches watched),
 * champion reveal after all matches are viewed, and animated battle replay per match.
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
import type {
  BattleResult,
  Match,
  GameConstants,
  BracketRound,
  BracketMatch,
  EnhancedTeamCard,
  PlaceholderPositions,
} from '../../types'

export interface TournamentBattlePageProps {
  userId: number
  activeMatch: Match | null
  battleData: BattleResult
  gameConstants: GameConstants | null
  onNavigateToLobby?: () => void
  onMatchChange?: (match: Match | null) => void
}

/**
 * Determine if a match in a given round is unlocked for viewing.
 *
 * Rules:
 * - First round (index 0): all matches are unlocked immediately
 * - Subsequent rounds: a match is unlocked only when ALL matches in the previous round
 *   have been watched
 */
function isMatchUnlocked(
  roundIndex: number,
  _matchIndex: number,
  rounds: BracketRound[],
  watchedMatches: Set<number>,
): boolean {
  // First round is always unlocked
  if (roundIndex === 0) return true

  // Check that all matches in the previous round are watched
  const prevRound = rounds[roundIndex - 1]
  return prevRound.matches.every((m) => watchedMatches.has(m.match_number))
}

/**
 * Check if all matches across all rounds have been watched.
 */
function allMatchesWatched(rounds: BracketRound[], watchedMatches: Set<number>): boolean {
  return rounds.every((round) =>
    round.matches.every((m) => watchedMatches.has(m.match_number)),
  )
}

/**
 * Get the champion's name from standings or bracket data.
 */
function getChampionName(battleData: BattleResult): string | null {
  if (!battleData.winner_id) return null

  // Try standings first
  if (battleData.standings?.length) {
    const champion = battleData.standings.find((s) => s.rank === 1)
    if (champion) return champion.name
  }

  // Fall back to bracket rounds - winner of the final match
  if (battleData.bracket_rounds?.length) {
    const finalRound = battleData.bracket_rounds[battleData.bracket_rounds.length - 1]
    if (finalRound.matches.length === 1) {
      const finalMatch = finalRound.matches[0]
      if (finalMatch.winner_id === finalMatch.player_a_id) return finalMatch.player_a_name
      if (finalMatch.winner_id === finalMatch.player_b_id) return finalMatch.player_b_name
    }
  }

  return null
}

/**
 * Render a single bracket match card.
 */
function BracketMatchCard({
  match,
  unlocked,
  watched,
  onWatch,
}: {
  match: BracketMatch
  unlocked: boolean
  watched: boolean
  onWatch: (matchNumber: number) => void
}) {
  if (!unlocked) {
    return (
      <div className="bracket-match">
        <div className="bracket-match-locked">??? vs ???</div>
        <div className="bracket-match-actions">
          <GameButton variant="neutral" size="sm" disabled>
            Locked
          </GameButton>
        </div>
      </div>
    )
  }

  const winnerName = match.winner_id === match.player_a_id
    ? match.player_a_name
    : match.winner_id === match.player_b_id
      ? match.player_b_name
      : null

  return (
    <div className="bracket-match">
      <div className="bracket-match-players">
        <span
          className={`bracket-match-player${
            match.winner_id != null
              ? match.winner_id === match.player_a_id ? ' is-winner' : ' is-loser'
              : ''
          }`}
        >
          {match.player_a_name}
        </span>
        <span className="bracket-match-vs">vs</span>
        <span
          className={`bracket-match-player${
            match.winner_id != null
              ? match.winner_id === match.player_b_id ? ' is-winner' : ' is-loser'
              : ''
          }`}
        >
          {match.player_b_name}
        </span>
      </div>

      {watched && winnerName && (
        <div className="bracket-match-result">
          Winner: {winnerName} &bull; {match.player_a_damage} - {match.player_b_damage} dmg
        </div>
      )}

      <div className="bracket-match-actions">
        <GameButton
          variant={watched ? 'secondary' : 'primary'}
          size="sm"
          onClick={() => onWatch(match.match_number)}
        >
          {watched ? 'Rewatch' : 'Watch'}
        </GameButton>
      </div>
    </div>
  )
}

// ─── Battle Replay Sub-component ──────────────────────────────────────────────

interface BracketReplayViewProps {
  matchId: string
  matchNumber: number
  userId: number
  onComplete: () => void
  onBack: () => void
}

/**
 * Renders a full animated battle replay for a single bracket match.
 * Fetches round data, runs the battle animation, and shows result on completion.
 */
function BracketReplayView({
  matchId,
  matchNumber,
  userId,
  onComplete,
  onBack,
}: BracketReplayViewProps) {
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
      const data = await apiClient.getRoundBattle(matchId, matchNumber)
      setRoundData(data)
    } catch (err) {
      console.error('Failed to fetch round battle:', err)
      setError(err instanceof Error ? err.message : 'Failed to load battle')
    } finally {
      setLoading(false)
    }
  }, [matchId, matchNumber])

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

  // Notify parent of completion (marks match as watched)
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
        <div className="bracket-replay-result">
          <RPGPanel variant="outer" className="bracket-replay-result-panel">
            <RPGPanel variant="inner">
              <div className="bracket-replay-result-content">
                <div className="bracket-replay-result-icon">
                  {roundData.is_draw ? '\u{1F91D}' : '\u{2694}\u{FE0F}'}
                </div>
                <h3>
                  {roundData.is_draw
                    ? 'Draw!'
                    : `${roundData.winner_id === roundData.player_a_id ? roundData.player_a_name : roundData.player_b_name} Wins!`}
                </h3>
                <p className="bracket-replay-result-stats">
                  {roundData.team_a_damage} - {roundData.team_b_damage} damage
                </p>
              </div>
            </RPGPanel>
            <GameButton variant="primary" onClick={onBack}>
              Back to Bracket
            </GameButton>
          </RPGPanel>
        </div>
      )}
    </div>
  )
}

// ─── Main Bracket Page ────────────────────────────────────────────────────────

export function BracketBattlePage({
  userId,
  activeMatch,
  battleData,
  onNavigateToLobby,
  onMatchChange,
}: TournamentBattlePageProps) {
  const [watchedMatches, setWatchedMatches] = useState<Set<number>>(new Set())
  const [viewingMatch, setViewingMatch] = useState<number | null>(null)

  const handleReturnToLobby = useCallback(() => {
    onMatchChange?.(null)
    onNavigateToLobby?.()
  }, [onMatchChange, onNavigateToLobby])

  const handleWatchMatch = useCallback((matchNumber: number) => {
    setViewingMatch(matchNumber)
  }, [])

  const handleReplayComplete = useCallback(() => {
    if (viewingMatch !== null) {
      setWatchedMatches((prev) => {
        const next = new Set(prev)
        next.add(viewingMatch)
        return next
      })
    }
  }, [viewingMatch])

  const handleBackToBracket = useCallback(() => {
    setViewingMatch(null)
  }, [])

  // When viewing a match, render the replay view
  if (viewingMatch !== null && activeMatch) {
    return (
      <BracketReplayView
        matchId={activeMatch.id}
        matchNumber={viewingMatch}
        userId={userId}
        onComplete={handleReplayComplete}
        onBack={handleBackToBracket}
      />
    )
  }

  if (!battleData.bracket_rounds) {
    return <LoadingSpinner message="Loading bracket..." fullScreen />
  }

  const rounds = battleData.bracket_rounds
  const allWatched = allMatchesWatched(rounds, watchedMatches)
  const championName = getChampionName(battleData)

  return (
    <div className="battle-page bracket-page page-bg page-bg--arena">
      <RPGPanel variant="outer">
        {/* Header */}
        <RPGPanel variant="inner">
          <div className="bracket-page-header">
            <h2>Bracket Tournament</h2>
            <p>
              {rounds.length} round{rounds.length !== 1 ? 's' : ''}
              {' \u2022 '}
              {battleData.standings?.length ?? 0} players
            </p>
          </div>
        </RPGPanel>

        {/* Bracket rounds */}
        {rounds.map((round, roundIndex) => (
          <div key={roundIndex}>
            {/* Connector between rounds */}
            {roundIndex > 0 && (
              <div className="bracket-connector">&darr;</div>
            )}

            <RPGPanel variant="inner" className="bracket-round">
              <h3 className="bracket-round-title">{round.name}</h3>
              {round.matches.map((match, matchIndex) => {
                const unlocked = isMatchUnlocked(roundIndex, matchIndex, rounds, watchedMatches)
                const watched = watchedMatches.has(match.match_number)

                return (
                  <BracketMatchCard
                    key={match.match_number}
                    match={match}
                    unlocked={unlocked}
                    watched={watched}
                    onWatch={handleWatchMatch}
                  />
                )
              })}
            </RPGPanel>
          </div>
        ))}

        {/* Champion banner — shown after all matches are watched */}
        {allWatched && championName && (
          <RPGPanel variant="inner-blue">
            <div className="bracket-champion">
              <div className="bracket-champion-icon">&#x1F3C6;</div>
              <h3 className="bracket-champion-title">Champion</h3>
              <p className="bracket-champion-name">{championName}</p>
            </div>
          </RPGPanel>
        )}

        {/* Standings — visible after all matches watched */}
        {allWatched && battleData.standings && battleData.standings.length > 0 && (
          <RPGPanel variant="inner">
            <div className="bracket-standings">
              <h3 className="bracket-standings-title">Final Standings</h3>
              {battleData.standings.map((s) => (
                <div key={s.user_id} className="bracket-standing-row">
                  <span className={`bracket-standing-rank rank-${s.rank}`}>
                    #{s.rank}
                  </span>
                  <span className="bracket-standing-name">{s.name}</span>
                  <span className="bracket-standing-stats">
                    {s.wins}W/{s.losses}L &bull; {s.total_damage_dealt} dmg
                  </span>
                </div>
              ))}
            </div>
          </RPGPanel>
        )}

        {/* Actions */}
        <div className="bracket-actions">
          <GameButton variant="primary" onClick={handleReturnToLobby}>
            Return to Lobby
          </GameButton>
        </div>
      </RPGPanel>
    </div>
  )
}

export default BracketBattlePage
