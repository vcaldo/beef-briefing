import { createContext, useContext, useState, useCallback, ReactNode } from 'react';
import { apiClient } from '../api/client';
import { addPageAction } from '@beef-briefing/shared-mini-app/monitoring';
import type { AppPage, Match } from '../types';

interface NavigationContextType {
  // Navigation state
  page: AppPage;
  activeMatch: Match | null;
  h2hOpponentId: number | null;
  h2hOpponentName: string;

  // Navigation handlers
  handleMatchSelect: (match: Match) => void;
  handleBackToLobby: () => void;
  handleGameLaunch: (chatId: number) => Promise<void>;
  handleNavigate: (newPage: AppPage) => void;
  handleViewH2H: (opponentId: number, opponentName: string) => void;
  handleBackFromH2H: () => void;
  handleViewProfile: () => void;
  handleBackFromProfile: () => void;

  // State validation
  isValidShopState: boolean;
  isValidBattleState: boolean;
  isValidH2HState: boolean;
  showNavigation: boolean;
}

const NavigationContext = createContext<NavigationContextType | undefined>(undefined);

interface NavigationProviderProps {
  children: ReactNode;
  userId: number | null;
}

export function NavigationProvider({ children, userId }: NavigationProviderProps) {
  const [page, setPage] = useState<AppPage>('lobby');
  const [activeMatch, setActiveMatch] = useState<Match | null>(null);
  const [h2hOpponentId, setH2hOpponentId] = useState<number | null>(null);
  const [h2hOpponentName, setH2hOpponentName] = useState<string>('');

  // Handle match selection
  const handleMatchSelect = useCallback((match: Match) => {
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
  }, []);

  // Handle back to lobby
  const handleBackToLobby = useCallback(() => {
    addPageAction('back_to_lobby', {
      from_page: page,
      match_id: activeMatch?.id,
    });
    setActiveMatch(null);
    setPage('lobby');
  }, [page, activeMatch?.id]);

  // Handle game launch - auto-join or show lobby
  const handleGameLaunch = useCallback(async (chatId: number) => {
    try {
      // Check for active matches in this chat
      const { matches } = await apiClient.getActiveMatches();
      const match = matches.find((m) =>
        m.chat_id === chatId && m.status === 'open'
      );

      if (match) {
        // Auto-join the active match
        const joinedMatch = await apiClient.joinMatch(match.id);
        handleMatchSelect(joinedMatch);
        addPageAction('game_launch_joined', {
          chat_id: chatId,
          match_id: joinedMatch.id,
        });
      } else {
        // No active match - show lobby
        setPage('lobby');
        addPageAction('game_launch_no_match', {
          chat_id: chatId,
        });
      }
    } catch (err) {
      // Fail gracefully - show lobby
      setPage('lobby');
      addPageAction('game_launch_error', {
        error: err instanceof Error ? err.message : 'unknown',
      });
    }
  }, [handleMatchSelect]);

  // Handle navigation
  const handleNavigate = useCallback((newPage: AppPage) => {
    addPageAction('tab_change', {
      tab: newPage,
      previous_tab: page,
    });
    setActiveMatch(null);
    setH2hOpponentId(null);
    setPage(newPage);
  }, [page]);

  // Handle H2H view
  const handleViewH2H = useCallback((opponentId: number, opponentName: string) => {
    addPageAction('h2h_view', {
      opponent_id: opponentId,
      opponent_name: opponentName,
    });
    setH2hOpponentId(opponentId);
    setH2hOpponentName(opponentName);
    setPage('h2h');
  }, []);

  // Handle back from H2H
  const handleBackFromH2H = useCallback(() => {
    addPageAction('back_from_h2h', {
      opponent_id: h2hOpponentId,
    });
    setH2hOpponentId(null);
    setPage('leaderboard');
  }, [h2hOpponentId]);

  // Handle view profile
  const handleViewProfile = useCallback(() => {
    addPageAction('profile_view', {
      user_id: userId,
    });
    setPage('profile');
  }, [userId]);

  // Handle back from profile
  const handleBackFromProfile = useCallback(() => {
    addPageAction('back_from_profile', {});
    setPage('leaderboard');
  }, []);

  // State validation
  const isValidShopState = page === 'shop' && activeMatch !== null;
  const isValidBattleState = page === 'battle' && activeMatch !== null;
  const isValidH2HState = page === 'h2h' && h2hOpponentId !== null;
  const showNavigation = !activeMatch && (page === 'lobby' || page === 'leaderboard' || page === 'history');

  const value: NavigationContextType = {
    page,
    activeMatch,
    h2hOpponentId,
    h2hOpponentName,
    handleMatchSelect,
    handleBackToLobby,
    handleGameLaunch,
    handleNavigate,
    handleViewH2H,
    handleBackFromH2H,
    handleViewProfile,
    handleBackFromProfile,
    isValidShopState,
    isValidBattleState,
    isValidH2HState,
    showNavigation,
  };

  return (
    <NavigationContext.Provider value={value}>
      {children}
    </NavigationContext.Provider>
  );
}

export function useNavigation(): NavigationContextType {
  const context = useContext(NavigationContext);
  if (context === undefined) {
    throw new Error('useNavigation must be used within NavigationProvider');
  }
  return context;
}
