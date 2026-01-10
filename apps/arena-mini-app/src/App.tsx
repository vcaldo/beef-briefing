import { useState, useEffect } from 'react';
import { useLaunchParams } from '@telegram-apps/sdk-react';
import { apiClient } from './api/client';
import type { AppState, AppPage, Match } from './types';
import { LobbyPage } from './components/LobbyPage';
import { ShopPage } from './components/ShopPage';
import { BattlePage } from './components/BattlePage';
import { LeaderboardPage } from './components/LeaderboardPage';
import { HistoryPage } from './components/HistoryPage';
import { H2HPage } from './components/H2HPage';
import { Navigation } from './components/Navigation';

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

  // Authenticate on mount
  useEffect(() => {
    async function authenticate() {
      try {
        const initDataRaw = launchParams?.initDataRaw;
        if (!initDataRaw) {
          setError('This app must be opened from Telegram');
          setAppState('error');
          return;
        }

        const auth = await apiClient.authenticate(initDataRaw);
        setUserId(auth.user_id);
        setFirstName(auth.first_name);
        setAppState('authenticated');
      } catch (err) {
        console.error('Auth failed:', err);
        setError(err instanceof Error ? err.message : 'Authentication failed');
        setAppState('error');
      }
    }

    authenticate();
  }, [launchParams?.initDataRaw]);

  // Handle match selection
  const handleMatchSelect = (match: Match) => {
    setActiveMatch(match);
    if (match.status === 'shop_phase') {
      setPage('shop');
    } else if (match.status === 'battle_phase' || match.status === 'completed') {
      setPage('battle');
    }
  };

  // Handle back to lobby
  const handleBackToLobby = () => {
    setActiveMatch(null);
    setPage('lobby');
  };

  // Handle navigation
  const handleNavigate = (newPage: AppPage) => {
    setActiveMatch(null);
    setH2hOpponentId(null);
    setPage(newPage);
  };

  // Handle H2H view
  const handleViewH2H = (opponentId: number, opponentName: string) => {
    setH2hOpponentId(opponentId);
    setH2hOpponentName(opponentName);
    setPage('h2h');
  };

  // Handle back from H2H
  const handleBackFromH2H = () => {
    setH2hOpponentId(null);
    setPage('leaderboard');
  };

  // Show navigation only on main pages (not in active match flow)
  const showNavigation = !activeMatch && (page === 'lobby' || page === 'leaderboard' || page === 'history');

  // Loading state
  if (appState === 'loading' || (appState === 'authenticated' && !splashMinTimeElapsed)) {
    return (
      <div className="splash">
        <div className="splash-title">BEEF ARENA</div>
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
      {page === 'lobby' && (
        <LobbyPage
          userId={userId!}
          firstName={firstName}
          onMatchSelect={handleMatchSelect}
        />
      )}
      {page === 'shop' && activeMatch && (
        <ShopPage
          match={activeMatch}
          userId={userId!}
          onBack={handleBackToLobby}
          onBattleStart={() => setPage('battle')}
        />
      )}
      {page === 'battle' && activeMatch && (
        <BattlePage
          match={activeMatch}
          userId={userId!}
          onBack={handleBackToLobby}
        />
      )}
      {page === 'leaderboard' && (
        <LeaderboardPage
          userId={userId!}
          onViewH2H={handleViewH2H}
        />
      )}
      {page === 'history' && (
        <HistoryPage
          userId={userId!}
        />
      )}
      {page === 'h2h' && h2hOpponentId && (
        <H2HPage
          opponentId={h2hOpponentId}
          opponentName={h2hOpponentName}
          onBack={handleBackFromH2H}
        />
      )}

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
