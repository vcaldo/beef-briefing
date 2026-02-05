/**
 * Free-For-All Battle Page Component (Stub)
 *
 * Displays round-robin tournament results for odd player counts (3, 5, 7...).
 * Features sequential round list, round replay, and final standings.
 *
 * TODO: Full implementation in tasks 17-18.
 */

import { RPGPanel, GameButton } from '../ui'
import { LoadingSpinner } from '../common'
import type { TournamentBattlePageProps } from './BracketBattlePage'

export function FreeForAllBattlePage({
  battleData,
  onNavigateToLobby,
  onMatchChange,
}: TournamentBattlePageProps) {
  const handleReturnToLobby = () => {
    onMatchChange?.(null)
    onNavigateToLobby?.()
  }

  if (!battleData.ffa_rounds) {
    return <LoadingSpinner message="Loading rounds..." fullScreen />
  }

  return (
    <div className="battle-page page-bg page-bg--arena">
      <RPGPanel variant="outer" className="tournament-placeholder">
        <RPGPanel variant="inner">
          <h2 style={{ textAlign: 'center', margin: '1rem 0 0.5rem' }}>Free-For-All</h2>
          <p style={{ textAlign: 'center', opacity: 0.7, margin: '0 0 1rem' }}>
            {battleData.ffa_rounds.length} round{battleData.ffa_rounds.length !== 1 ? 's' : ''}
            {' '}&bull;{' '}
            {battleData.standings?.length ?? 0} players
          </p>
        </RPGPanel>

        {battleData.ffa_rounds.map((round) => (
          <RPGPanel key={round.round_number} variant="inner" className="tournament-round-panel">
            <div style={{ textAlign: 'center', padding: '0.5rem 0' }}>
              <strong>Round {round.round_number}:</strong>{' '}
              {round.player_a_name} vs {round.player_b_name}
              {round.winner_id != null && (
                <span style={{ opacity: 0.6 }}>
                  {' '}&mdash; Winner: {round.winner_id === round.player_a_id ? round.player_a_name : round.player_b_name}
                </span>
              )}
            </div>
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

export default FreeForAllBattlePage
