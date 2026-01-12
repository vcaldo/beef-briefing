import { useState, useEffect } from 'react';
import { useLaunchParams } from '@telegram-apps/sdk-react';
import { apiClient } from './api/client';
import { setCustomAttribute, addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring';
import { TIMERS } from './config/constants';
import type { AppState } from './types';
import { LobbyPage } from './components/LobbyPage';
import { ShopPage } from './components/ShopPage';
import { BattlePage } from './components/BattlePage';
import { LeaderboardPage } from './components/LeaderboardPage';
import { HistoryPage } from './components/HistoryPage';
import { H2HPage } from './components/H2HPage';
import { ProfilePage } from './components/ProfilePage';
import { Navigation } from './components/Navigation';
import { ErrorBoundary } from '@beef-briefing/shared-mini-app/components';
import { NavigationProvider, useNavigation } from './contexts/NavigationContext';

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

// Main content component that consumes NavigationContext
function AppContent({ userId, firstName }: { userId: number; firstName: string }) {
  const {
    page,
    activeMatch,
    h2hOpponentId,
    h2hOpponentName,
    handleMatchSelect,
    handleBackToLobby,
    handleNavigate,
    handleViewH2H,
    handleBackFromH2H,
    handleViewProfile,
    handleBackFromProfile,
    isValidShopState,
    isValidBattleState,
    isValidH2HState,
    showNavigation,
  } = useNavigation();

  // Track invalid state combinations for monitoring
  useEffect(() => {
    if (page === 'shop' && !activeMatch) {
      if (import.meta.env.DEV) {
        console.error('Invalid state: shop page without activeMatch');
      }
      addPageAction('invalid_state_shop', {
        page,
        hasActiveMatch: !!activeMatch,
      });
    }
    if (page === 'battle' && !activeMatch) {
      if (import.meta.env.DEV) {
        console.error('Invalid state: battle page without activeMatch');
      }
      addPageAction('invalid_state_battle', {
        page,
        hasActiveMatch: !!activeMatch,
      });
    }
    if (page === 'h2h' && !h2hOpponentId) {
      if (import.meta.env.DEV) {
        console.error('Invalid state: h2h page without opponentId');
      }
      addPageAction('invalid_state_h2h', {
        page,
        hasOpponentId: !!h2hOpponentId,
      });
    }
  }, [page, activeMatch, h2hOpponentId]);

  return (
    <div className="app">
      <ErrorBoundary name="lobby" onReset={() => handleNavigate('lobby')}>
        {page === 'lobby' && (
          <LobbyPage
            userId={userId}
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
              userId={userId}
              onBack={handleBackToLobby}
              onBattleStart={() => handleNavigate('battle')}
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
              userId={userId}
              onBack={handleBackToLobby}
            />
          ) : (
            <InvalidStateFallback onReset={handleBackToLobby} />
          )
        )}
      </ErrorBoundary>

      <ErrorBoundary name="leaderboard" onReset={() => handleNavigate('leaderboard')}>
        {page === 'leaderboard' && (
          <LeaderboardPage
            userId={userId}
            onViewH2H={handleViewH2H}
            onViewProfile={handleViewProfile}
          />
        )}
      </ErrorBoundary>

      <ErrorBoundary name="history" onReset={() => handleNavigate('history')}>
        {page === 'history' && (
          <HistoryPage userId={userId} />
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

// Main App component handling authentication
function App() {
  const launchParams = useLaunchParams();
  const [appState, setAppState] = useState<AppState>('loading');
  const [error, setError] = useState<string | null>(null);
  const [userId, setUserId] = useState<number | null>(null);
  const [firstName, setFirstName] = useState<string>('');
  const [splashMinTimeElapsed, setSplashMinTimeElapsed] = useState(false);

  // Minimum splash time for smooth UX
  useEffect(() => {
    const timer = setTimeout(() => setSplashMinTimeElapsed(true), TIMERS.SPLASH_SCREEN_MIN);
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

        const initDataRaw = launchParams?.initDataRaw;

        if (!initDataRaw) {
          // If we have query parameters from a direct link (game callback), show helpful message
          if (urlChatId) {
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
      } catch (err) {
        if (import.meta.env.DEV) {
          console.error('Auth failed:', err);
        }
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

  // Render authenticated state with navigation context
  return (
    <NavigationProvider userId={userId}>
      <AppContent userId={userId!} firstName={firstName} />
    </NavigationProvider>
  );
}

export default App;
