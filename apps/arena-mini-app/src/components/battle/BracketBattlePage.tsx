/**
 * Bracket Battle Page Component
 *
 * Displays bracket tournament results for even player counts (4, 6, 8...).
 * Features bracket diagram with elimination rounds, match watch/rewatch buttons,
 * unlock rules (semis open immediately, finals unlock after all prior matches watched),
 * and champion reveal after all matches are viewed.
 */

import { useState, useCallback } from 'react'
import { RPGPanel, GameButton } from '../ui'
import { LoadingSpinner } from '../common'
import type { BattleResult, Match, GameConstants, BracketRound, BracketMatch } from '../../types'

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

export function BracketBattlePage({
  battleData,
  onNavigateToLobby,
  onMatchChange,
}: TournamentBattlePageProps) {
  const [watchedMatches, setWatchedMatches] = useState<Set<number>>(new Set())
  const [viewingMatch, setViewingMatch] = useState<number | null>(null)

  // viewingMatch is tracked for task 16 (replay integration) — for now,
  // clicking Watch/Rewatch marks the match as watched immediately
  // and will be extended with actual battle replay in task 16.
  void viewingMatch

  const handleReturnToLobby = useCallback(() => {
    onMatchChange?.(null)
    onNavigateToLobby?.()
  }, [onMatchChange, onNavigateToLobby])

  const handleWatchMatch = useCallback((matchNumber: number) => {
    // Mark as watched (task 16 will add actual replay before marking)
    setWatchedMatches((prev) => {
      const next = new Set(prev)
      next.add(matchNumber)
      return next
    })
    setViewingMatch(matchNumber)
    // Task 16: will fetch round data and show replay here.
    // For now, return to bracket view immediately after marking watched.
    setViewingMatch(null)
  }, [])

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
