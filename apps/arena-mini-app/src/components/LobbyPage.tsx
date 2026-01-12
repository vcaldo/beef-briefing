import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import { addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring';
import type { Match } from '../types';
import { ErrorDisplay } from '@beef-briefing/shared-mini-app/components';
import './LobbyPage.css';

interface LobbyPageProps {
  userId: number;
  firstName: string;
  onMatchSelect: (match: Match) => void;
}

export function LobbyPage({ userId, firstName, onMatchSelect }: LobbyPageProps) {
  const [matches, setMatches] = useState<Match[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fetch active matches
  useEffect(() => {
    async function fetchMatches() {
      try {
        const { matches } = await apiClient.getActiveMatches();
        setMatches(matches);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch matches:', err);
        const errorMessage = err instanceof Error ? err.message : 'Failed to load active matches';
        setError(errorMessage);
        noticeError(err instanceof Error ? err : new Error(errorMessage), {
          context: 'fetch_active_matches',
        });
      } finally {
        setLoading(false);
      }
    }

    fetchMatches();
    const interval = setInterval(fetchMatches, 5000); // Poll every 5s
    return () => clearInterval(interval);
  }, []);

  // Create new match
  const handleCreateMatch = async () => {
    setCreating(true);
    setError(null);
    try {
      const match = await apiClient.createMatch();
      setMatches(prev => [match, ...prev]);
      addPageAction('match_created', {
        match_id: match.id,
      });
      onMatchSelect(match);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create match');
      if (err instanceof Error) {
        noticeError(err, { context: 'create_match' });
      }
      addPageAction('match_create_error', {
        error: err instanceof Error ? err.message : 'unknown',
      });
    } finally {
      setCreating(false);
    }
  };

  // Join match
  const handleJoinMatch = async (matchId: string) => {
    setError(null);
    try {
      const match = await apiClient.joinMatch(matchId);
      addPageAction('match_joined', {
        match_id: matchId,
      });
      onMatchSelect(match);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to join match');
      if (err instanceof Error) {
        noticeError(err, { context: 'join_match', match_id: matchId });
      }
      addPageAction('match_join_error', {
        match_id: matchId,
        error: err instanceof Error ? err.message : 'unknown',
      });
    }
  };

  // Start match (creator only)
  const handleStartMatch = async (matchId: string) => {
    setError(null);
    try {
      const match = await apiClient.startMatch(matchId);
      addPageAction('match_started', {
        match_id: matchId,
        participant_count: match.participants.length,
      });
      onMatchSelect(match);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start match');
      if (err instanceof Error) {
        noticeError(err, { context: 'start_match', match_id: matchId });
      }
      addPageAction('match_start_error', {
        match_id: matchId,
        error: err instanceof Error ? err.message : 'unknown',
      });
    }
  };

  // Check if user is in match
  const isInMatch = (match: Match) =>
    match.participants.some(p => p.user_id === userId);

  // Check if user is creator
  const isCreator = (match: Match) => match.creator_user_id === userId;

  // Format time remaining
  const formatTimeRemaining = (deadline: string) => {
    const remaining = new Date(deadline).getTime() - Date.now();
    if (remaining <= 0) return 'Expired';
    const minutes = Math.floor(remaining / 60000);
    const seconds = Math.floor((remaining % 60000) / 1000);
    return `${minutes}:${seconds.toString().padStart(2, '0')}`;
  };

  return (
    <div className="lobby-page">
      <header className="lobby-header">
        <h1>⚔️ BEEF ARENA 🥩</h1>
        <p>Welcome, {firstName}!</p>
      </header>

      {error && (
        <ErrorDisplay
          error={error}
          title="Failed to load matches"
          onRetry={() => window.location.reload()}
        />
      )}

      {!error && (
        <>
          <section className="lobby-section">
            <button
              className="btn btn-primary create-match-btn"
              onClick={handleCreateMatch}
              disabled={creating}
            >
              {creating ? 'Creating...' : 'Create Match'}
            </button>
          </section>

          <section className="lobby-section">
            <h2>Active Matches</h2>
            {loading ? (
          <div className="lobby-loading">
            <div className="spinner" />
          </div>
        ) : matches.length === 0 ? (
          <div className="lobby-empty">
            <p>No active matches</p>
            <p className="lobby-empty-hint">Create one to get started!</p>
          </div>
        ) : (
          <div className="match-list">
            {matches.map(match => (
              <div key={match.id} className="match-card">
                <div className="match-header">
                  <span className={`match-status ${match.status}`}>
                    {match.status.replace('_', ' ')}
                  </span>
                  {match.join_deadline && match.status === 'open' && (
                    <span className="match-timer">
                      {formatTimeRemaining(match.join_deadline)}
                    </span>
                  )}
                </div>

                <div className="match-participants">
                  {match.participants.map(p => (
                    <div key={p.user_id} className="participant">
                      <span className="participant-name">{p.first_name}</span>
                      {p.status === 'ready' && <span className="ready-badge">Ready</span>}
                    </div>
                  ))}
                  {match.participants.length < 2 && (
                    <div className="participant waiting">
                      Waiting for opponent...
                    </div>
                  )}
                </div>

                <div className="match-actions">
                  {match.status === 'open' && !isInMatch(match) && (
                    <button
                      className="btn btn-primary"
                      onClick={() => handleJoinMatch(match.id)}
                    >
                      Join
                    </button>
                  )}
                  {match.status === 'open' && isCreator(match) && match.participants.length >= 2 && (
                    <button
                      className="btn btn-success"
                      onClick={() => handleStartMatch(match.id)}
                    >
                      Start Match
                    </button>
                  )}
                  {match.status === 'shop_phase' && isInMatch(match) && (
                    <button
                      className="btn btn-primary"
                      onClick={() => onMatchSelect(match)}
                    >
                      Go to Shop
                    </button>
                  )}
                  {(match.status === 'battle_phase' || match.status === 'completed') && isInMatch(match) && (
                    <button
                      className="btn btn-secondary"
                      onClick={() => onMatchSelect(match)}
                    >
                      View Battle
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
          </section>
        </>
      )}
    </div>
  );
}
