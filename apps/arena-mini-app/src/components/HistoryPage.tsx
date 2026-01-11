import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';
import { noticeError } from '../newrelic';
import type { MatchHistoryEntry, GameCard, BattleResult, BattleEvent } from '../types';
import { Avatar } from './Avatar';
import { ErrorDisplay } from './ErrorDisplay';
import './HistoryPage.css';

interface HistoryPageProps {
  userId?: number;
}

// Helper to find a card by ID across both teams
const findCard = (cardId: number, yourTeam: GameCard[], opponentTeam: GameCard[]): GameCard | null => {
  return yourTeam.find(c => c.card_id === cardId) || opponentTeam.find(c => c.card_id === cardId) || null;
};

// Get event icon
const getEventIcon = (type: string): string => {
  switch (type) {
    case 'attack': return '\u2694\uFE0F';
    case 'death': return '\uD83D\uDC80';
    case 'victory': return '\uD83D\uDC51';
    case 'advance': return '\u27A1\uFE0F';
    default: return '';
  }
};

// Format battle event for compact display
const formatBattleEvent = (
  event: BattleEvent,
  yourTeam: GameCard[],
  opponentTeam: GameCard[],
  isWinner: boolean
): string | null => {
  const attacker = event.attacker_card_id ? findCard(event.attacker_card_id, yourTeam, opponentTeam) : null;
  const defender = event.defender_card_id ? findCard(event.defender_card_id, yourTeam, opponentTeam) : null;

  switch (event.type) {
    case 'attack':
      if (attacker && defender && event.damage) {
        return `${attacker.name} (${attacker.atk}) \u2192 ${defender.name} (${event.hp_before}) = ${event.damage} dmg`;
      }
      return event.message || 'Attack';
    case 'death':
      if (defender) {
        return `${defender.name} defeated`;
      }
      return event.message || 'Defeated';
    case 'victory':
      return isWinner ? 'Victory!' : 'Defeat';
    case 'advance':
      return null; // Skip advance events for compact view
    default:
      return event.message || null;
  }
};

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
              const isWinner = match.result === 'win';

              return (
                <div
                  key={match.match_id}
                  className={`history-entry ${isExpanded ? 'expanded' : ''}`}
                  onClick={() => handleMatchClick(match)}
                >
                  <div className="entry-compact">
                    <div className="players-section">
                      <div className="player you">
                        <Avatar src={yourPhoto} name="You" size="md" />
                        <span className="player-name">You</span>
                      </div>
                      <span className="vs-badge">vs</span>
                      <div className="player opponent">
                        <Avatar src={opponentPhoto} name={match.opponent.first_name} size="md" />
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
                              {round.battle_log.map((event, eventIndex) => {
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
