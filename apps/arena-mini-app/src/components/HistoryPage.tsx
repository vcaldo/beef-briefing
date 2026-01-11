import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import { noticeError } from '../newrelic';
import type { MatchHistoryEntry } from '../types';
import { ErrorDisplay } from './ErrorDisplay';
import './HistoryPage.css';

interface HistoryPageProps {
  userId?: number;
}

export function HistoryPage(_props: HistoryPageProps) {
  const [matches, setMatches] = useState<MatchHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [loadingMore, setLoadingMore] = useState(false);

  const limit = 20;

  useEffect(() => {
    fetchHistory(0, true);
  }, []);

  async function fetchHistory(newOffset: number, reset: boolean) {
    if (reset) {
      setLoading(true);
      setError(null);
    } else {
      setLoadingMore(true);
    }

    try {
      const result = await apiClient.getHistory(limit, newOffset);
      if (reset) {
        setMatches(result.matches);
      } else {
        setMatches(prev => [...prev, ...result.matches]);
      }
      setHasMore(result.has_more);
      setOffset(newOffset + result.matches.length);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch history:', err);
      const errorMessage = err instanceof Error ? err.message : 'Failed to load match history';
      setError(errorMessage);
      noticeError(err instanceof Error ? err : new Error(errorMessage), {
        context: 'fetch_history',
      });
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  }

  const handleLoadMore = () => {
    if (!loadingMore && hasMore) {
      fetchHistory(offset, false);
    }
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24));

    if (diffDays === 0) {
      return 'Today';
    } else if (diffDays === 1) {
      return 'Yesterday';
    } else if (diffDays < 7) {
      return `${diffDays} days ago`;
    } else {
      return date.toLocaleDateString();
    }
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
      case 'win': return 'Victory';
      case 'loss': return 'Defeat';
      default: return 'Draw';
    }
  };

  return (
    <div className="history-page">
      <header className="history-header">
        <h1>Match History</h1>
      </header>

      {error ? (
        <ErrorDisplay
          error={error}
          title="Failed to load history"
          onRetry={() => fetchHistory(0, true)}
        />
      ) : loading ? (
        <div className="history-loading">
          <div className="spinner" />
        </div>
      ) : matches.length === 0 ? (
        <div className="history-empty">
          <p>No matches played yet</p>
          <p className="history-empty-hint">
            Play some matches to see your history!
          </p>
        </div>
      ) : (
        <>
          <div className="history-list">
            {matches.map((match) => (
              <div key={match.match_id} className="history-entry">
                <div className="entry-header">
                  <span className={`match-type ${match.match_type}`}>
                    {match.match_type}
                  </span>
                  <span className="match-date">
                    {formatDate(match.completed_at)}
                  </span>
                </div>

                <div className="entry-body">
                  <div className="opponent-info">
                    <span className="vs-label">vs</span>
                    <span className="opponent-name">{match.opponent.first_name}</span>
                  </div>
                  <div className={`match-result ${getResultClass(match.result)}`}>
                    {getResultText(match.result)}
                  </div>
                </div>

                <div className="entry-teams">
                  <div className="team-preview your-team">
                    {match.your_team.slice(0, 3).map((card, i) => (
                      <div key={i} className="team-card-mini">
                        <span className="card-atk">{card.atk}</span>
                      </div>
                    ))}
                  </div>
                  <span className="vs-divider">vs</span>
                  <div className="team-preview opponent-team">
                    {match.opponent_team.slice(0, 3).map((card, i) => (
                      <div key={i} className="team-card-mini">
                        <span className="card-atk">{card.atk}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            ))}
          </div>

          {hasMore && (
            <button
              className="btn btn-secondary load-more-btn"
              onClick={handleLoadMore}
              disabled={loadingMore}
            >
              {loadingMore ? 'Loading...' : 'Load More'}
            </button>
          )}
        </>
      )}

      {/* Bottom padding for navigation */}
      <div className="nav-spacer" />
    </div>
  );
}
