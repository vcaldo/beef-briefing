import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import { noticeError } from '../newrelic';
import type { ArenaProfile, MatchHistoryEntry } from '../types';
import { ErrorDisplay } from './ErrorDisplay';
import './ProfilePage.css';

interface ProfilePageProps {
  onBack: () => void;
}

export function ProfilePage({ onBack }: ProfilePageProps) {
  const [profile, setProfile] = useState<ArenaProfile | null>(null);
  const [recentMatches, setRecentMatches] = useState<MatchHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
    if (diffDays < 7) return `${diffDays} days ago`;
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
                {recentMatches.map((match) => (
                  <div
                    key={match.match_id}
                    className={`recent-match ${match.result}`}
                  >
                    <span className={`result-indicator ${match.result}`}>
                      {match.result === 'win' ? 'W' : match.result === 'loss' ? 'L' : 'D'}
                    </span>
                    <span className="match-opponent">vs {match.opponent.first_name}</span>
                    <span className="match-type-badge">{match.match_type}</span>
                    <span className="match-date">{formatRelativeDate(match.completed_at)}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
