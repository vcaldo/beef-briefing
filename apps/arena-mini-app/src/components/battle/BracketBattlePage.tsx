/**
 * Bracket Battle Page Component (Stub)
 *
 * Displays bracket tournament results for even player counts (4, 6, 8...).
 * Features bracket diagram with elimination rounds, match replay, and champion reveal.
 *
 * TODO: Full implementation in tasks 15-16.
 */

import { RPGPanel, GameButton } from '../ui'
import { LoadingSpinner } from '../common'
import type { BattleResult, Match, GameConstants } from '../../types'

export interface TournamentBattlePageProps {
  userId: number
  activeMatch: Match | null
  battleData: BattleResult
  gameConstants: GameConstants | null
  onNavigateToLobby?: () => void
  onMatchChange?: (match: Match | null) => void
}

export function BracketBattlePage({
  battleData,
  onNavigateToLobby,
  onMatchChange,
}: TournamentBattlePageProps) {
  const handleReturnToLobby = () => {
    onMatchChange?.(null)
    onNavigateToLobby?.()
  }

  if (!battleData.bracket_rounds) {
    return <LoadingSpinner message="Loading bracket..." fullScreen />
  }

  return (
    <div className="battle-page page-bg page-bg--arena">
      <RPGPanel variant="outer" className="tournament-placeholder">
        <RPGPanel variant="inner">
          <h2 style={{ textAlign: 'center', margin: '1rem 0 0.5rem' }}>Bracket Tournament</h2>
          <p style={{ textAlign: 'center', opacity: 0.7, margin: '0 0 1rem' }}>
            {battleData.bracket_rounds.length} round{battleData.bracket_rounds.length !== 1 ? 's' : ''}
            {' '}&bull;{' '}
            {battleData.standings?.length ?? 0} players
          </p>
        </RPGPanel>

        {battleData.bracket_rounds.map((round, i) => (
          <RPGPanel key={i} variant="inner" className="tournament-round-panel">
            <h3 style={{ textAlign: 'center', margin: '0.5rem 0' }}>{round.name}</h3>
            {round.matches.map((match) => (
              <div key={match.match_number} style={{ textAlign: 'center', padding: '0.25rem 0' }}>
                {match.player_a_name} vs {match.player_b_name}
                {match.winner_id != null && (
                  <span style={{ opacity: 0.6 }}>
                    {' '}&mdash; Winner: {match.winner_id === match.player_a_id ? match.player_a_name : match.player_b_name}
                  </span>
                )}
              </div>
            ))}
          </RPGPanel>
        ))}

        {battleData.standings && battleData.standings.length > 0 && (
          <RPGPanel variant="inner">
            <h3 style={{ textAlign: 'center', margin: '0.5rem 0' }}>Standings</h3>
            {battleData.standings.map((s) => (
              <div key={s.user_id} style={{ textAlign: 'center', padding: '0.25rem 0' }}>
                #{s.rank} {s.name} — {s.wins}W/{s.losses}L ({s.total_damage_dealt} dmg)
              </div>
            ))}
          </RPGPanel>
        )}

        <div style={{ padding: '1rem', textAlign: 'center' }}>
          <GameButton variant="primary" onClick={handleReturnToLobby}>
            Return to Lobby
          </GameButton>
        </div>
      </RPGPanel>
    </div>
  )
}

export default BracketBattlePage
