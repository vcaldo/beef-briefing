import { useState, useEffect, useRef } from 'react';
import { useLaunchParams } from '@telegram-apps/sdk-react';
import { apiClient } from './api/client';
import { setCustomAttribute, addPageAction, noticeError } from './newrelic';
import type { AppState, AppPage, Match } from './types';
import { LobbyPage } from './components/LobbyPage';
import { ShopPage } from './components/ShopPage';
import { BattlePage } from './components/BattlePage';
import { LeaderboardPage } from './components/LeaderboardPage';
import { HistoryPage } from './components/HistoryPage';
import { H2HPage } from './components/H2HPage';
import { ProfilePage } from './components/ProfilePage';
import { Navigation } from './components/Navigation';
import { ErrorBoundary } from './components/ErrorBoundary';

// Fallback component for invalid state combinations
function InvalidStateFallback({ onReset }: { onReset: () => void }) {
  return (
    <div className="invalid-state-fallback">
      <div className="fallback-icon">⚠️</div>
      <div className="fallback-title">Page not available</div>
      <div className="fallback-message">
        We encountered an issue loading this page. Let's get you back on track.
      </div>
      <button className="btn btn-primary" onClick={onReset}>
        Return to Lobby
      </button>
    </div>
  );
}

function App() {
  const launchParams = useLaunchParams();
  const [appState, setAppState] = useState<AppState>('loading');
  const [error, setError] = useState<string | null>(null);
  const [userId, setUserId] = useState<number | null>(null);
  const [firstName, setFirstName] = useState<string>('');
  const [splashMinTimeElapsed, setSplashMinTimeElapsed] = useState(false);

  // Current page and active match
  const [page, setPage] = useState<AppPage>('lobby');
  const [activeMatch, setActiveMatch] = useState<Match | null>(null);

  // H2H state
  const [h2hOpponentId, setH2hOpponentId] = useState<number | null>(null);
  const [h2hOpponentName, setH2hOpponentName] = useState<string>('');

  // Minimum splash time for smooth UX
  useEffect(() => {
    const timer = setTimeout(() => setSplashMinTimeElapsed(true), 1500);
    return () => clearTimeout(timer);
  }, []);

  // Set timezone attribute on mount
  useEffect(() => {
    try {
      const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
      setCustomAttribute('timezone', timezone);
    } catch {
      setCustomAttribute('timezone', 'unknown');
    }
  }, []);

  // Authenticate on mount
  useEffect(() => {
    async function authenticate() {
      try {
        // Parse URL query parameters (for game launches and fallback auth)
        const urlParams = new URLSearchParams(window.location.search);
        const urlChatId = urlParams.get('chat_id');
        const urlUserId = urlParams.get('user_id');

        const initDataRaw = launchParams?.initDataRaw;

        // Log authentication context
        console.debug('Auth context:', {
          hasInitData: !!initDataRaw,
          hasChatId: !!urlChatId,
          hasUserId: !!urlUserId,
          launchParamsExists: !!launchParams,
        });

        if (!initDataRaw) {
          // If we have query parameters from a direct link (game callback), show helpful message
          if (urlChatId || urlUserId) {
            setError(
              'Please open this link by clicking the button in Telegram. ' +
              'The app must be opened from within Telegram to work properly.'
            );
          } else {
            setError('This app must be opened from Telegram');
          }
          setAppState('error');
          addPageAction('auth_failed', {
            reason: 'no_init_data',
            hasChatId: !!urlChatId,
            hasUserId: !!urlUserId,
          });
          return;
        }

        const auth = await apiClient.authenticate(initDataRaw);
        setUserId(auth.user_id);
        setFirstName(auth.first_name);
        setAppState('authenticated');

        // Set New Relic custom attributes for this session
        setCustomAttribute('user_id', auth.user_id);
        if (auth.chat_id) {
          setCustomAttribute('chat_id', auth.chat_id);
        }
        setCustomAttribute('username', auth.username || '');
        setCustomAttribute('first_name', auth.first_name);
        setCustomAttribute('is_authenticated', true);

        addPageAction('auth_success', {
          user_id: auth.user_id,
          chat_id: auth.chat_id,
        });

        // If launched from game, handle match joining
        if (urlChatId) {
          await handleGameLaunch(parseInt(urlChatId));
        }
      } catch (err) {
        console.error('Auth failed:', err);
        setError(err instanceof Error ? err.message : 'Authentication failed');
        setAppState('error');

        if (err instanceof Error) {
          noticeError(err, { context: 'authentication' });
        }
        addPageAction('auth_error', {
          error: err instanceof Error ? err.message : 'unknown',
        });
      }
    }

    authenticate();
  }, [launchParams?.initDataRaw]);

  // Handle match selection
  const handleMatchSelect = (match: Match) => {
    setActiveMatch(match);
    const targetPage = match.status === 'shop_phase' ? 'shop' : 'battle';
    addPageAction('match_selected', {
      match_id: match.id,
      match_status: match.status,
      target_page: targetPage,
    });
    if (match.status === 'shop_phase') {
      setPage('shop');
    } else if (match.status === 'battle_phase' || match.status === 'completed') {
      setPage('battle');
    }
  };

  // Handle back to lobby
  const handleBackToLobby = () => {
    addPageAction('back_to_lobby', {
      from_page: page,
      match_id: activeMatch?.id,
    });
    setActiveMatch(null);
    setPage('lobby');
  };

  // Handle game launch - auto-join or show lobby
  const handleGameLaunch = async (chatId: number) => {
    try {
      // Check for active matches in this chat
      const { matches } = await apiClient.getActiveMatches();
      const activeMatch = matches.find((m) =>
        m.chat_id === chatId && m.status === 'open'
      );

      if (activeMatch) {
        // Auto-join the active match
        const match = await apiClient.joinMatch(activeMatch.id);
        handleMatchSelect(match);
        addPageAction('game_launch_joined', {
          chat_id: chatId,
          match_id: match.id,
        });
      } else {
        // No active match - show lobby
        setPage('lobby');
        addPageAction('game_launch_no_match', {
          chat_id: chatId,
        });
      }
    } catch (err) {
      console.error('Game launch error:', err);
      // Fail gracefully - show lobby
      setPage('lobby');
      addPageAction('game_launch_error', {
        error: err instanceof Error ? err.message : 'unknown',
      });
    }
  };

  // Handle navigation
  const handleNavigate = (newPage: AppPage) => {
    addPageAction('tab_change', {
      tab: newPage,
      previous_tab: page,
    });
    setActiveMatch(null);
    setH2hOpponentId(null);
    setPage(newPage);
  };

  // Handle H2H view
  const handleViewH2H = (opponentId: number, opponentName: string) => {
    addPageAction('h2h_view', {
      opponent_id: opponentId,
      opponent_name: opponentName,
    });
    setH2hOpponentId(opponentId);
    setH2hOpponentName(opponentName);
    setPage('h2h');
  };

  // Handle back from H2H
  const handleBackFromH2H = () => {
    addPageAction('back_from_h2h', {
      opponent_id: h2hOpponentId,
    });
    setH2hOpponentId(null);
    setPage('leaderboard');
  };

  // Handle view profile
  const handleViewProfile = () => {
    addPageAction('profile_view', {
      user_id: userId,
    });
    setPage('profile');
  };

  // Handle back from profile
  const handleBackFromProfile = () => {
    addPageAction('back_from_profile', {});
    setPage('leaderboard');
  };

  // State validation - check if page and dependencies are consistent
  const isValidShopState = page === 'shop' && activeMatch;
  const isValidBattleState = page === 'battle' && activeMatch;
  const isValidH2HState = page === 'h2h' && h2hOpponentId;

  // Show navigation only on main pages (not in active match flow)
  const showNavigation = !activeMatch && (page === 'lobby' || page === 'leaderboard' || page === 'history');

  // Track page transitions for debugging
  const prevPageRef = useRef<AppPage>(page);
  useEffect(() => {
    if (prevPageRef.current !== page) {
      addPageAction('page_transition', {
        from: prevPageRef.current,
        to: page,
        hasActiveMatch: !!activeMatch,
        hasH2hOpponentId: !!h2hOpponentId,
      });
      prevPageRef.current = page;
    }
  }, [page, activeMatch, h2hOpponentId]);

  // Track invalid state combinations for debugging
  useEffect(() => {
    if (page === 'shop' && !activeMatch) {
      console.error('Invalid state: shop page without activeMatch', { page, activeMatch });
      addPageAction('invalid_state_shop', {
        page,
        hasActiveMatch: !!activeMatch,
      });
    }
    if (page === 'battle' && !activeMatch) {
      console.error('Invalid state: battle page without activeMatch', { page, activeMatch });
      addPageAction('invalid_state_battle', {
        page,
        hasActiveMatch: !!activeMatch,
      });
    }
    if (page === 'h2h' && !h2hOpponentId) {
      console.error('Invalid state: h2h page without opponentId', { page, h2hOpponentId });
      addPageAction('invalid_state_h2h', {
        page,
        hasOpponentId: !!h2hOpponentId,
      });
    }
  }, [page, activeMatch, h2hOpponentId]);

  // Loading state
  if (appState === 'loading' || (appState === 'authenticated' && !splashMinTimeElapsed)) {
    return (
      <div className="splash">
        <div className="splash-title">⚔️ BEEF ARENA 🥩</div>
        <div className="spinner" />
      </div>
    );
  }

  // Error state
  if (appState === 'error') {
    return (
      <div className="error-container">
        <div className="error-icon">!</div>
        <div className="error-message">{error}</div>
      </div>
    );
  }

  // Render current page
  return (
    <div className="app">
      <ErrorBoundary name="lobby" onReset={() => setPage('lobby')}>
        {page === 'lobby' && (
          <LobbyPage
            userId={userId!}
            firstName={firstName}
            onMatchSelect={handleMatchSelect}
          />
        )}
      </ErrorBoundary>

      <ErrorBoundary name="shop" onReset={handleBackToLobby}>
        {page === 'shop' && (
          isValidShopState ? (
            <ShopPage
              match={activeMatch!}
              userId={userId!}
              onBack={handleBackToLobby}
              onBattleStart={() => setPage('battle')}
            />
          ) : (
            <InvalidStateFallback onReset={handleBackToLobby} />
          )
        )}
      </ErrorBoundary>

      <ErrorBoundary name="battle" onReset={handleBackToLobby}>
        {page === 'battle' && (
          isValidBattleState ? (
            <BattlePage
              match={activeMatch!}
              userId={userId!}
              onBack={handleBackToLobby}
            />
          ) : (
            <InvalidStateFallback onReset={handleBackToLobby} />
          )
        )}
      </ErrorBoundary>

      <ErrorBoundary name="leaderboard" onReset={() => setPage('leaderboard')}>
        {page === 'leaderboard' && (
          <LeaderboardPage
            userId={userId!}
            onViewH2H={handleViewH2H}
            onViewProfile={handleViewProfile}
          />
        )}
      </ErrorBoundary>

      <ErrorBoundary name="history" onReset={() => setPage('history')}>
        {page === 'history' && (
          <HistoryPage
            userId={userId!}
          />
        )}
      </ErrorBoundary>

      <ErrorBoundary name="h2h" onReset={handleBackFromH2H}>
        {page === 'h2h' && (
          isValidH2HState ? (
            <H2HPage
              opponentId={h2hOpponentId!}
              opponentName={h2hOpponentName}
              onBack={handleBackFromH2H}
            />
          ) : (
            <InvalidStateFallback onReset={handleBackFromH2H} />
          )
        )}
      </ErrorBoundary>

      <ErrorBoundary name="profile" onReset={handleBackFromProfile}>
        {page === 'profile' && (
          <ProfilePage onBack={handleBackFromProfile} />
        )}
      </ErrorBoundary>

      {showNavigation && (
        <Navigation
          currentPage={page}
          onNavigate={handleNavigate}
        />
      )}
    </div>
  );
}

export default App;
