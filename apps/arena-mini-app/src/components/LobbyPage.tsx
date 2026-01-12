import { useState, useCallback } from 'react';
import { apiClient } from '../api/client';
import { addPageAction } from '@beef-briefing/shared-mini-app/monitoring';
import type { Match } from '../types';
import { ErrorDisplay } from '@beef-briefing/shared-mini-app/components';
import { usePolling } from '../hooks/usePolling';
import { useApiCall } from '../hooks/useApiCall';
import { POLLING_INTERVALS } from '../config/constants';
import './LobbyPage.css';

interface LobbyPageProps {
  userId: number;
  firstName: string;
  onMatchSelect: (match: Match) => void;
}

export function LobbyPage({ userId, firstName, onMatchSelect }: LobbyPageProps) {
  const [error, setError] = useState<string | null>(null);

  // Poll active matches
  const { data: matchesResponse, loading } = usePolling(
    async () => {
      const response = await apiClient.getActiveMatches();
      return response;
    },
    POLLING_INTERVALS.LOBBY,
  );

  const matches = matchesResponse?.matches ?? [];

  // Handle create match
  const createMatchApi = useApiCall({
    context: 'create_match',
    onError: (err) => {
      setError(err);
      addPageAction('match_create_error', { error: err });
    },
  });

  const handleCreateMatch = useCallback(async () => {
    const match = await createMatchApi.execute(() => apiClient.createMatch());
    if (match) {
      addPageAction('match_created', { match_id: match.id });
      onMatchSelect(match);
    }
  }, [createMatchApi, onMatchSelect]);

  // Handle join match
  const joinMatchApi = useApiCall({
    context: 'join_match',
    onError: (err) => {
      setError(err);
      addPageAction('match_join_error', { error: err });
    },
  });

  const handleJoinMatch = useCallback(
    async (matchId: string) => {
      const match = await joinMatchApi.execute(() => apiClient.joinMatch(matchId));
      if (match) {
        addPageAction('match_joined', { match_id: matchId });
        onMatchSelect(match);
      }
    },
    [joinMatchApi, onMatchSelect],
  );

  // Handle start match
  const startMatchApi = useApiCall({
    context: 'start_match',
    onError: (err) => {
      setError(err);
      addPageAction('match_start_error', { error: err });
    },
  });

  const handleStartMatch = useCallback(
    async (matchId: string) => {
      const match = await startMatchApi.execute(() => apiClient.startMatch(matchId));
      if (match) {
        addPageAction('match_started', {
          match_id: matchId,
          participant_count: match.participants.length,
        });
        onMatchSelect(match);
      }
    },
    [startMatchApi, onMatchSelect],
  );

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
              disabled={createMatchApi.loading}
            >
              {createMatchApi.loading ? 'Creating...' : 'Create Match'}
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
