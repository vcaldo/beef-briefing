/**
 * useBattleData Hook
 * Manages fetching and caching battle data for a specific match
 */

import { useState, useCallback } from 'react';
import { apiClient } from '../api/client';
import { noticeError } from '@beef-briefing/shared-mini-app/monitoring';
import type { BattleResult } from '../types';

/**
 * Hook for managing battle data fetching and state
 * Handles loading, error states, and caches battle data
 *
 * @param matchId - Match ID to fetch battle data for, or null
 * @returns Battle data, loading state, fetch function, and clear function
 */
export function useBattleData(matchId: string | null) {
  const [battleData, setBattleData] = useState<BattleResult | null>(null);
  const [loadingBattle, setLoadingBattle] = useState(false);

  const fetchBattle = useCallback(async () => {
    if (!matchId) return;

    setLoadingBattle(true);
    setBattleData(null);

    try {
      const battle = await apiClient.getBattle(matchId);
      setBattleData(battle);
    } catch (err) {
      console.error('Failed to fetch battle:', err);
      noticeError(err instanceof Error ? err : new Error('Failed to fetch battle'), {
        context: 'fetch_battle',
        match_id: matchId,
      });
    } finally {
      setLoadingBattle(false);
    }
  }, [matchId]);

  const clearBattle = useCallback(() => {
    setBattleData(null);
    setLoadingBattle(false);
  }, []);

  return {
    battleData,
    loadingBattle,
    fetchBattle,
    clearBattle,
  };
}
