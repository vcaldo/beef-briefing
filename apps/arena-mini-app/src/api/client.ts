import { BaseApiClient } from '@beef-briefing/shared-mini-app/api'

/**
 * Arena Mini App API Client
 *
 * Extends BaseApiClient with Arena-specific endpoints.
 * The full 18+ endpoints will be implemented in a subsequent task.
 */
class ArenaApiClient extends BaseApiClient {
  constructor(baseUrl?: string) {
    super(baseUrl || import.meta.env.VITE_API_URL || '')
  }

  // Arena API endpoints will be implemented in a subsequent task:
  // - Lobby: listMatches, createMatch, joinMatch, leaveMatch
  // - Shop: getShopState, buyCard, rerollShop, upgradeCard, setCardOrder, submitTeam
  // - Battle: getBattleState
  // - Stats: getLeaderboard, getProfile, getMatchHistory, getH2H
}

export const apiClient = new ArenaApiClient()
