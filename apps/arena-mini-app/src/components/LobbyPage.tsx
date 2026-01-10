import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import type { Match } from '../types';
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
      } catch (err) {
        console.error('Failed to fetch matches:', err);
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
      onMatchSelect(match);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create match');
    } finally {
      setCreating(false);
    }
  };

  // Join match
  const handleJoinMatch = async (matchId: string) => {
    setError(null);
    try {
      const match = await apiClient.joinMatch(matchId);
      onMatchSelect(match);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to join match');
    }
  };

  // Start match (creator only)
  const handleStartMatch = async (matchId: string) => {
    setError(null);
    try {
      const match = await apiClient.startMatch(matchId);
      onMatchSelect(match);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start match');
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
        <h1>BEEF ARENA</h1>
        <p>Welcome, {firstName}!</p>
      </header>

      {error && (
        <div className="lobby-error">
          {error}
        </div>
      )}

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
    </div>
  );
}
