import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import { noticeError } from '@beef-briefing/shared-mini-app/monitoring';
import type { MatchHistoryEntry, BattleResult } from '../types';
import { Avatar } from '@beef-briefing/shared-mini-app/components';
import { ErrorDisplay } from '@beef-briefing/shared-mini-app/components';
import { BattleLog } from './BattleLog';
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

  // Expanded match state
  const [expandedMatchId, setExpandedMatchId] = useState<string | null>(null);
  const [battleData, setBattleData] = useState<BattleResult | null>(null);
  const [loadingBattle, setLoadingBattle] = useState(false);

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

  const handleMatchClick = async (match: MatchHistoryEntry) => {
    if (expandedMatchId === match.match_id) {
      // Collapse if already expanded
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

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24));

    if (diffDays === 0) {
      return 'Today';
    } else if (diffDays === 1) {
      return 'Yesterday';
    } else if (diffDays < 7) {
      return `${diffDays}d ago`;
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
      case 'win': return 'W';
      case 'loss': return 'L';
      default: return 'D';
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
            {matches.map((match) => {
              const isExpanded = expandedMatchId === match.match_id;
              const yourPhoto = match.your_photo_url || null;
              const opponentPhoto = match.opponent.photo_url || null;

              return (
                <div
                  key={match.match_id}
                  className={`history-entry ${isExpanded ? 'expanded' : ''}`}
                  onClick={() => handleMatchClick(match)}
                >
                  <div className="entry-compact">
                    <div className="players-section">
                      <div className="player you">
                        <Avatar photoUrl={yourPhoto} firstName="You" size="medium" />
                        <span className="player-name">You</span>
                      </div>
                      <span className="vs-badge">vs</span>
                      <div className="player opponent">
                        <Avatar photoUrl={opponentPhoto} firstName={match.opponent.first_name} size="medium" />
                        <span className="player-name">{match.opponent.first_name}</span>
                      </div>
                    </div>

                    <div className="meta-section">
                      <span className={`result-badge ${getResultClass(match.result)}`}>
                        {getResultText(match.result)}
                      </span>
                      <span className={`match-type ${match.match_type}`}>
                        {match.match_type === 'ranked' ? 'R' : 'C'}
                      </span>
                      <span className="match-date">{formatDate(match.completed_at)}</span>
                    </div>
                  </div>

                  {isExpanded && (
                    <div className="battle-log-section">
                      {loadingBattle ? (
                        <div className="battle-log-loading">
                          <div className="spinner spinner-sm" />
                        </div>
                      ) : battleData && battleData.rounds.length > 0 ? (
                        <div className="battle-log">
                          <div className="battle-log-header">Battle Log</div>
                          {battleData.rounds.map((round, roundIndex) => (
                            <div key={roundIndex} className="battle-round">
                              <div className="round-header">Round {roundIndex + 1}</div>
                              <BattleLog
                                events={round.battle_log}
                                isLive={false}
                                autoScroll={false}
                              />
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

          {hasMore && (
            <button
              className="btn btn-secondary load-more-btn"
              onClick={(e) => {
                e.stopPropagation();
                handleLoadMore();
              }}
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
