import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import { noticeError } from '@beef-briefing/shared-mini-app/monitoring';
import type { H2HRecord, MatchHistoryEntry } from '../types';
import { ErrorDisplay } from '@beef-briefing/shared-mini-app/components';
import './H2HPage.css';

interface H2HPageProps {
  opponentId: number;
  opponentName: string;
  onBack: () => void;
}

export function H2HPage({ opponentId, opponentName, onBack }: H2HPageProps) {
  const [record, setRecord] = useState<H2HRecord | null>(null);
  const [recentMatches, setRecentMatches] = useState<MatchHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function fetchH2H() {
      setLoading(true);
      setError(null);
      try {
        const result = await apiClient.getH2H(opponentId);
        setRecord(result.record);
        setRecentMatches(result.recent_matches || []);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch H2H:', err);
        const errorMessage = err instanceof Error ? err.message : 'Failed to load head-to-head data';
        setError(errorMessage);
        noticeError(err instanceof Error ? err : new Error(errorMessage), {
          context: 'fetch_h2h',
          opponent_id: opponentId,
        });
      } finally {
        setLoading(false);
      }
    }

    fetchH2H();
  }, [opponentId]);

  const getWinRate = () => {
    if (!record) return 0;
    const total = record.wins + record.losses;
    if (total === 0) return 0;
    return Math.round((record.wins / total) * 100);
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleDateString();
  };

  return (
    <div className="h2h-page">
      <header className="h2h-header">
        <button className="back-btn" onClick={onBack}>
          ← Back
        </button>
        <h1>vs {opponentName}</h1>
      </header>

      {error ? (
        <ErrorDisplay
          error={error}
          title="Failed to load head-to-head"
          onRetry={() => window.location.reload()}
          onBack={onBack}
        />
      ) : loading ? (
        <div className="h2h-loading">
          <div className="spinner" />
        </div>
      ) : !record ? (
        <div className="h2h-empty">
          <p>No matches against this player yet</p>
        </div>
      ) : (
        <>
          <div className="h2h-summary">
            <div className="h2h-record">
              <div className="record-stat wins">
                <span className="record-value">{record.wins}</span>
                <span className="record-label">Wins</span>
              </div>
              <div className="record-divider">-</div>
              <div className="record-stat losses">
                <span className="record-value">{record.losses}</span>
                <span className="record-label">Losses</span>
              </div>
            </div>

            <div className="h2h-winrate">
              <div className="winrate-bar">
                <div
                  className="winrate-fill"
                  style={{ width: `${getWinRate()}%` }}
                />
              </div>
              <span className="winrate-text">{getWinRate()}% win rate</span>
            </div>

            {record.last_match_at && (
              <div className="h2h-last-match">
                Last match: {formatDate(record.last_match_at)}
              </div>
            )}
          </div>

          {recentMatches.length > 0 && (
            <div className="h2h-recent">
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
                    <span className="match-type-badge">{match.match_type}</span>
                    <span className="match-date">{formatDate(match.completed_at)}</span>
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
