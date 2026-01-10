import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import type { LeaderboardEntry } from '../types';
import './LeaderboardPage.css';

interface LeaderboardPageProps {
  userId: number;
  onViewH2H: (opponentId: number, opponentName: string) => void;
}

export function LeaderboardPage({ userId, onViewH2H }: LeaderboardPageProps) {
  const [entries, setEntries] = useState<LeaderboardEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [type, setType] = useState<'ranked' | 'regular'>('ranked');

  useEffect(() => {
    async function fetchLeaderboard() {
      setLoading(true);
      try {
        const { entries } = await apiClient.getLeaderboard(type);
        setEntries(entries);
      } catch (err) {
        console.error('Failed to fetch leaderboard:', err);
      } finally {
        setLoading(false);
      }
    }

    fetchLeaderboard();
  }, [type]);

  const getWins = (entry: LeaderboardEntry) =>
    type === 'ranked' ? entry.ranked_wins : entry.regular_wins;

  const getLosses = (entry: LeaderboardEntry) =>
    type === 'ranked' ? entry.ranked_losses : entry.regular_losses;

  const getStreak = (entry: LeaderboardEntry) =>
    type === 'ranked' ? entry.ranked_current_streak : entry.regular_current_streak;

  const getWinRate = (entry: LeaderboardEntry) => {
    const wins = getWins(entry);
    const losses = getLosses(entry);
    const total = wins + losses;
    if (total === 0) return 0;
    return Math.round((wins / total) * 100);
  };

  return (
    <div className="leaderboard-page">
      <header className="leaderboard-header">
        <h1>Leaderboard</h1>
        <div className="leaderboard-tabs">
          <button
            className={`tab-btn ${type === 'ranked' ? 'active' : ''}`}
            onClick={() => setType('ranked')}
          >
            Ranked
          </button>
          <button
            className={`tab-btn ${type === 'regular' ? 'active' : ''}`}
            onClick={() => setType('regular')}
          >
            Casual
          </button>
        </div>
      </header>

      {loading ? (
        <div className="leaderboard-loading">
          <div className="spinner" />
        </div>
      ) : entries.length === 0 ? (
        <div className="leaderboard-empty">
          <p>No matches played yet</p>
          <p className="leaderboard-empty-hint">
            Play some {type} matches to appear here!
          </p>
        </div>
      ) : (
        <div className="leaderboard-list">
          {entries.map((entry, index) => {
            const isCurrentUser = entry.user_id === userId;
            const rank = index + 1;

            return (
              <div
                key={entry.user_id}
                className={`leaderboard-entry ${isCurrentUser ? 'current-user' : ''}`}
                onClick={() => onViewH2H(entry.user_id, entry.first_name)}
              >
                <div className="entry-rank">
                  {rank <= 3 ? (
                    <span className={`rank-medal rank-${rank}`}>
                      {rank === 1 ? '🥇' : rank === 2 ? '🥈' : '🥉'}
                    </span>
                  ) : (
                    <span className="rank-number">#{rank}</span>
                  )}
                </div>

                <div className="entry-info">
                  <div className="entry-name">
                    {entry.first_name}
                    {entry.username && (
                      <span className="entry-username">@{entry.username}</span>
                    )}
                    {isCurrentUser && <span className="you-badge">You</span>}
                  </div>
                  <div className="entry-stats-row">
                    <span className="stat-wins">{getWins(entry)}W</span>
                    <span className="stat-losses">{getLosses(entry)}L</span>
                    <span className="stat-winrate">{getWinRate(entry)}%</span>
                  </div>
                </div>

                <div className="entry-streak">
                  {getStreak(entry) > 0 && (
                    <div className="streak-current">
                      🔥 {getStreak(entry)}
                    </div>
                  )}
                  {type === 'ranked' && entry.ranked_tournaments_won > 0 && (
                    <div className="tournaments-won">
                      🏆 {entry.ranked_tournaments_won}
                    </div>
                  )}
                </div>

                <div className="entry-arrow">›</div>
              </div>
            );
          })}
        </div>
      )}

      {/* Bottom padding for navigation */}
      <div className="nav-spacer" />
    </div>
  );
}
