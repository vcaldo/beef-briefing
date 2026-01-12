import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import { noticeError } from '@beef-briefing/shared-mini-app/monitoring';
import type { ArenaProfile, MatchHistoryEntry, MatchRound, BattleEvent } from '../types';
import { Avatar } from '@beef-briefing/shared-mini-app/components';
import { ErrorDisplay } from '@beef-briefing/shared-mini-app/components';
import { formatDate, formatRelativeDate } from '../utils/date';
import { calculateWinRate, getResultClass, getResultText } from '../utils/stats';
import { formatBattleEvent, getEventIcon } from '../utils/battle';
import './ProfilePage.css';

interface ProfilePageProps {
  onBack: () => void;
}

export function ProfilePage({ onBack }: ProfilePageProps) {
  const [profile, setProfile] = useState<ArenaProfile | null>(null);
  const [recentMatches, setRecentMatches] = useState<MatchHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Battle data state
  const [expandedMatchId, setExpandedMatchId] = useState<string | null>(null);
  const [battleData, setBattleData] = useState<any>(null);
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
    return calculateWinRate(profile.ranked_wins, profile.ranked_losses);
  };

  const getCasualWinRate = () => {
    if (!profile) return 0;
    return calculateWinRate(profile.regular_wins, profile.regular_losses);
  };

  const getCurrentStreak = () => {
    if (!profile) return 0;
    return Math.max(profile.ranked_current_streak, profile.regular_current_streak);
  };

  const getBestStreak = () => {
    if (!profile) return 0;
    return Math.max(profile.ranked_best_streak, profile.regular_best_streak);
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
                  const yourPhoto = match.your_photo_url || null;
                  const opponentPhoto = match.opponent.photo_url || null;
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
                            <Avatar photoUrl={yourPhoto} firstName="You" size="small" />
                            <span className="match-player-name">You</span>
                          </div>
                          <span className="match-vs">vs</span>
                          <div className="match-player opponent">
                            <Avatar photoUrl={opponentPhoto} firstName={match.opponent.first_name} size="small" />
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
                              {battleData.rounds.map((round: MatchRound, roundIndex: number) => (
                                <div key={roundIndex} className="battle-round">
                                  {round.battle_log.map((event: BattleEvent, eventIndex: number) => {
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
