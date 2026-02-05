/**
 * Free-For-All Battle Page Component
 *
 * Displays round-robin tournament results for odd player counts (3, 5, 7...).
 * Features sequential round list with unlock rules (round N+1 unlocks after
 * round N watched), Watch/Rewatch buttons, and final standings after all
 * rounds are viewed.
 */

import { useState, useCallback } from 'react'
import { RPGPanel, GameButton } from '../ui'
import { LoadingSpinner } from '../common'
import type { TournamentBattlePageProps } from './BracketBattlePage'
import type { FfaRoundSummary } from '../../types'

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

// ─── Main Free-For-All Page ──────────────────────────────────────────────────

export function FreeForAllBattlePage({
  battleData,
  onNavigateToLobby,
  onMatchChange,
}: TournamentBattlePageProps) {
  const [watchedRounds, setWatchedRounds] = useState<Set<number>>(new Set())

  const handleReturnToLobby = useCallback(() => {
    onMatchChange?.(null)
    onNavigateToLobby?.()
  }, [onMatchChange, onNavigateToLobby])

  // Mark round as watched immediately (task 18 will replace this with animated replay)
  const handleWatchRound = useCallback((roundNumber: number) => {
    setWatchedRounds((prev) => {
      const next = new Set(prev)
      next.add(roundNumber)
      return next
    })
  }, [])

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
