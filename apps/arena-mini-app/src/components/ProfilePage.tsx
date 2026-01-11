import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import { noticeError } from '../newrelic';
import type { ArenaProfile, MatchHistoryEntry, GameCard, BattleResult, BattleEvent } from '../types';
import { Avatar } from './Avatar';
import { ErrorDisplay } from './ErrorDisplay';
import './ProfilePage.css';

interface ProfilePageProps {
  onBack: () => void;
}

// Helper to get user photo from team
const getUserPhoto = (team: GameCard[]): string | null => {
  return team[0]?.photo_url || null;
};

// Helper to find a card by ID across both teams
const findCard = (cardId: number, yourTeam: GameCard[], opponentTeam: GameCard[]): GameCard | null => {
  return yourTeam.find(c => c.card_id === cardId) || opponentTeam.find(c => c.card_id === cardId) || null;
};

// Get event icon
const getEventIcon = (type: string): string => {
  switch (type) {
    case 'attack': return '\u2694\uFE0F';
    case 'death': return '\uD83D\uDC80';
    case 'victory': return '\uD83D\uDC51';
    case 'advance': return '\u27A1\uFE0F';
    default: return '';
  }
};

// Format battle event for compact display
const formatBattleEvent = (
  event: BattleEvent,
  yourTeam: GameCard[],
  opponentTeam: GameCard[],
  isWinner: boolean
): string | null => {
  const attacker = event.attacker_card_id ? findCard(event.attacker_card_id, yourTeam, opponentTeam) : null;
  const defender = event.defender_card_id ? findCard(event.defender_card_id, yourTeam, opponentTeam) : null;

  switch (event.type) {
    case 'attack':
      if (attacker && defender && event.damage) {
        return `${attacker.name} (${attacker.atk}) \u2192 ${defender.name} (${event.hp_before}) = ${event.damage} dmg`;
      }
      return event.message || 'Attack';
    case 'death':
      if (defender) {
        return `${defender.name} defeated`;
      }
      return event.message || 'Defeated';
    case 'victory':
      return isWinner ? 'Victory!' : 'Defeat';
    case 'advance':
      return null; // Skip advance events for compact view
    default:
      return event.message || null;
  }
};

export function ProfilePage({ onBack }: ProfilePageProps) {
  const [profile, setProfile] = useState<ArenaProfile | null>(null);
  const [recentMatches, setRecentMatches] = useState<MatchHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Expanded match state
  const [expandedMatchId, setExpandedMatchId] = useState<string | null>(null);
  const [battleData, setBattleData] = useState<BattleResult | null>(null);
  const [loadingBattle, setLoadingBattle] = useState(false);

  useEffect(() => {
    async function fetchProfile() {
      setLoading(true);
      setError(null);
      try {
        const result = await apiClient.getProfile();
        setProfile(result.profile);
        setRecentMatches(result.recent_matches || []);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch profile:', err);
        const errorMessage = err instanceof Error ? err.message : 'Failed to load profile';
        setError(errorMessage);
        noticeError(err instanceof Error ? err : new Error(errorMessage), {
          context: 'fetch_profile',
        });
      } finally {
        setLoading(false);
      }
    }

    fetchProfile();
  }, []);

  const handleMatchClick = async (match: MatchHistoryEntry) => {
    if (expandedMatchId === match.match_id) {
      setExpandedMatchId(null);
      setBattleData(null);
      return;
    }

    setExpandedMatchId(match.match_id);
    setLoadingBattle(true);
    setBattleData(null);

    try {
      const battle = await apiClient.getBattle(match.match_id);
      setBattleData(battle);
    } catch (err) {
      console.error('Failed to fetch battle:', err);
      noticeError(err instanceof Error ? err : new Error('Failed to fetch battle'), {
        context: 'fetch_battle',
        match_id: match.match_id,
      });
    } finally {
      setLoadingBattle(false);
    }
  };

  const getRankedWinRate = () => {
    if (!profile) return 0;
    const total = profile.ranked_wins + profile.ranked_losses;
    if (total === 0) return 0;
    return Math.round((profile.ranked_wins / total) * 100);
  };

  const getCasualWinRate = () => {
    if (!profile) return 0;
    const total = profile.regular_wins + profile.regular_losses;
    if (total === 0) return 0;
    return Math.round((profile.regular_wins / total) * 100);
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleDateString();
  };

  const formatRelativeDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffDays === 0) return 'Today';
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7) return `${diffDays}d ago`;
    return formatDate(dateStr);
  };

  const getCurrentStreak = () => {
    if (!profile) return 0;
    return Math.max(profile.ranked_current_streak, profile.regular_current_streak);
  };

  const getBestStreak = () => {
    if (!profile) return 0;
    return Math.max(profile.ranked_best_streak, profile.regular_best_streak);
  };

  const getResultClass = (result: string) => {
    switch (result) {
      case 'win': return 'result-win';
      case 'loss': return 'result-loss';
      default: return 'result-draw';
    }
  };

  const getResultText = (result: string) => {
    switch (result) {
      case 'win': return 'W';
      case 'loss': return 'L';
      default: return 'D';
    }
  };

  return (
    <div className="profile-page">
      <header className="profile-header">
        <button className="back-btn" onClick={onBack}>
          ← Back
        </button>
        <h1>My Profile</h1>
      </header>

      {error ? (
        <ErrorDisplay
          error={error}
          title="Failed to load profile"
          onRetry={() => window.location.reload()}
          onBack={onBack}
        />
      ) : loading ? (
        <div className="profile-loading">
          <div className="spinner" />
        </div>
      ) : !profile ? (
        <div className="profile-empty">
          <p>No matches played yet</p>
          <p className="profile-empty-hint">
            Play some matches to see your stats here!
          </p>
        </div>
      ) : (
        <>
          <div className="profile-stats-cards">
            <div className="stats-card ranked">
              <div className="stats-card-header">
                <span className="stats-card-label">Ranked</span>
                {profile.ranked_rank > 0 && (
                  <span className="stats-card-rank">#{profile.ranked_rank}</span>
                )}
              </div>
              <div className="stats-card-record">
                <span className="wins">{profile.ranked_wins}W</span>
                <span className="separator">-</span>
                <span className="losses">{profile.ranked_losses}L</span>
              </div>
              <div className="stats-card-winrate">
                <div className="winrate-bar">
                  <div
                    className="winrate-fill ranked"
                    style={{ width: `${getRankedWinRate()}%` }}
                  />
                </div>
                <span className="winrate-text">{getRankedWinRate()}%</span>
              </div>
              {profile.ranked_tournaments_played > 0 && (
                <div className="stats-card-tournaments">
                  {profile.ranked_tournaments_won}/{profile.ranked_tournaments_played} tournaments won
                </div>
              )}
            </div>

            <div className="stats-card casual">
              <div className="stats-card-header">
                <span className="stats-card-label">Casual</span>
                {profile.regular_rank > 0 && (
                  <span className="stats-card-rank">#{profile.regular_rank}</span>
                )}
              </div>
              <div className="stats-card-record">
                <span className="wins">{profile.regular_wins}W</span>
                <span className="separator">-</span>
                <span className="losses">{profile.regular_losses}L</span>
              </div>
              <div className="stats-card-winrate">
                <div className="winrate-bar">
                  <div
                    className="winrate-fill casual"
                    style={{ width: `${getCasualWinRate()}%` }}
                  />
                </div>
                <span className="winrate-text">{getCasualWinRate()}%</span>
              </div>
            </div>
          </div>

          <div className="profile-streaks">
            <div className="streak-item">
              <span className="streak-icon">🔥</span>
              <span className="streak-label">Current Streak</span>
              <span className="streak-value">{getCurrentStreak()}</span>
            </div>
            <div className="streak-item">
              <span className="streak-icon">⭐</span>
              <span className="streak-label">Best Streak</span>
              <span className="streak-value">{getBestStreak()}</span>
            </div>
            {profile.first_match_at && (
              <div className="streak-item">
                <span className="streak-icon">📅</span>
                <span className="streak-label">Playing since</span>
                <span className="streak-value">{formatDate(profile.first_match_at)}</span>
              </div>
            )}
          </div>

          {recentMatches.length > 0 && (
            <div className="profile-recent">
              <h2>Recent Matches</h2>
              <div className="recent-list">
                {recentMatches.map((match) => {
                  const isExpanded = expandedMatchId === match.match_id;
                  const yourPhoto = getUserPhoto(match.your_team);
                  const opponentPhoto = getUserPhoto(match.opponent_team);
                  const isWinner = match.result === 'win';

                  return (
                    <div
                      key={match.match_id}
                      className={`recent-match-entry ${isExpanded ? 'expanded' : ''}`}
                      onClick={() => handleMatchClick(match)}
                    >
                      <div className="match-compact">
                        <div className="match-players">
                          <div className="match-player you">
                            <Avatar src={yourPhoto} name="You" size="sm" />
                            <span className="match-player-name">You</span>
                          </div>
                          <span className="match-vs">vs</span>
                          <div className="match-player opponent">
                            <Avatar src={opponentPhoto} name={match.opponent.first_name} size="sm" />
                            <span className="match-player-name">{match.opponent.first_name}</span>
                          </div>
                        </div>

                        <div className="match-meta">
                          <span className={`match-result-badge ${getResultClass(match.result)}`}>
                            {getResultText(match.result)}
                          </span>
                          <span className={`match-type-badge ${match.match_type}`}>
                            {match.match_type === 'ranked' ? 'R' : 'C'}
                          </span>
                          <span className="match-date">{formatRelativeDate(match.completed_at)}</span>
                        </div>
                      </div>

                      {isExpanded && (
                        <div className="match-battle-log">
                          {loadingBattle ? (
                            <div className="battle-log-loading">
                              <div className="spinner spinner-sm" />
                            </div>
                          ) : battleData && battleData.rounds.length > 0 ? (
                            <div className="battle-log">
                              <div className="battle-log-header">Battle Log</div>
                              {battleData.rounds.map((round, roundIndex) => (
                                <div key={roundIndex} className="battle-round">
                                  {round.battle_log.map((event, eventIndex) => {
                                    const message = formatBattleEvent(
                                      event,
                                      match.your_team,
                                      match.opponent_team,
                                      isWinner
                                    );
                                    if (!message) return null;

                                    return (
                                      <div
                                        key={eventIndex}
                                        className={`log-entry log-${event.type}`}
                                      >
                                        <span className="log-icon">{getEventIcon(event.type)}</span>
                                        <span className="log-message">{message}</span>
                                      </div>
                                    );
                                  })}
                                </div>
                              ))}
                            </div>
                          ) : (
                            <div className="battle-log-empty">
                              No battle data available
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
