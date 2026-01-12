import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import { noticeError } from '../newrelic';
import type { LeaderboardEntry } from '../types';
import { ErrorDisplay } from './ErrorDisplay';
import './LeaderboardPage.css';

interface LeaderboardPageProps {
  userId: number;
  onViewH2H: (opponentId: number, opponentName: string) => void;
  onViewProfile: () => void;
}

export function LeaderboardPage({ userId, onViewH2H, onViewProfile }: LeaderboardPageProps) {
  const [entries, setEntries] = useState<LeaderboardEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function fetchLeaderboard() {
      setLoading(true);
      setError(null);
      try {
        const { entries } = await apiClient.getLeaderboard('ranked');
        setEntries(entries);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch leaderboard:', err);
        const errorMessage = err instanceof Error ? err.message : 'Failed to load leaderboard';
        setError(errorMessage);
        noticeError(err instanceof Error ? err : new Error(errorMessage), {
          context: 'fetch_leaderboard',
        });
      } finally {
        setLoading(false);
      }
    }

    fetchLeaderboard();
  }, []);

  const getCasualWinRate = (entry: LeaderboardEntry) => {
    const total = entry.regular_wins + entry.regular_losses;
    if (total === 0) return 0;
    return Math.round((entry.regular_wins / total) * 100);
  };

  const getRankedWinRate = (entry: LeaderboardEntry) => {
    const total = entry.ranked_wins + entry.ranked_losses;
    if (total === 0) return 0;
    return Math.round((entry.ranked_wins / total) * 100);
  };

  return (
    <div className="leaderboard-page">
      <header className="leaderboard-header">
        <h1>Leaderboard</h1>
      </header>

      {error ? (
        <ErrorDisplay
          error={error}
          title="Failed to load leaderboard"
          onRetry={() => window.location.reload()}
        />
      ) : loading ? (
        <div className="leaderboard-loading">
          <div className="spinner" />
        </div>
      ) : entries.length === 0 ? (
        <div className="leaderboard-empty">
          <p>No matches played yet</p>
          <p className="leaderboard-empty-hint">
            Play some matches to appear here!
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
                onClick={() => {
                  if (isCurrentUser) {
                    onViewProfile();
                  } else {
                    onViewH2H(entry.user_id, entry.first_name);
                  }
                }}
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
                  <div className="entry-stats-container">
                    <div className="entry-stats-row casual">
                      <span className="stat-label">Casual:</span>
                      <span className="stat-wins">{entry.regular_wins}W</span>
                      <span className="stat-losses">{entry.regular_losses}L</span>
                      <span className="stat-winrate">{getCasualWinRate(entry)}%</span>
                    </div>
                    <div className="entry-stats-row ranked">
                      <span className="stat-label">Ranked:</span>
                      <span className="stat-wins">{entry.ranked_wins}W</span>
                      <span className="stat-losses">{entry.ranked_losses}L</span>
                      <span className="stat-winrate">{getRankedWinRate(entry)}%</span>
                    </div>
                  </div>
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
