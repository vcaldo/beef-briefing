import { BaseApiClient } from '@beef-briefing/shared-mini-app/api'
import type {
  MatchResponse,
  MatchesListResponse,
  ShopResponse,
  BattleResponse,
  LeaderboardResponse,
  ProfileResponse,
  MatchHistoryResponse,
  H2HResponse,
  GameConstants,
  ShareResultResponse,
} from '../types'

/**
 * Arena Mini App API Client
 *
 * Extends BaseApiClient with 18+ Arena-specific endpoints for:
 * - Lobby: match creation, joining, and management
 * - Shop: card purchasing, rerolling, upgrading, team submission
 * - Battle: battle results and replay data
 * - Stats: leaderboards, profile, history, head-to-head records
 */
class ArenaApiClient extends BaseApiClient {
  constructor(baseUrl?: string) {
    super(baseUrl || import.meta.env.VITE_API_URL || '')
  }

  // ==========================================================================
  // Lobby Endpoints
  // ==========================================================================

  /**
   * List active matches for the current chat.
   * GET /api/v1/mini-app/arena/matches?chat_id=X
   */
  async listMatches(chatId?: number): Promise<MatchResponse[]> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }
    const response = await this.request<MatchesListResponse>(
      `/api/v1/mini-app/arena/matches?chat_id=${targetChatId}`
    )
    return response.matches
  }

  /**
   * Create a new match in the current chat.
   * POST /api/v1/mini-app/arena/match
   */
  async createMatch(chatId?: number): Promise<MatchResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }
    return this.request<MatchResponse>('/api/v1/mini-app/arena/match', {
      method: 'POST',
      body: JSON.stringify({ chat_id: targetChatId }),
    })
  }

  /**
   * Get a specific match by ID.
   * GET /api/v1/mini-app/arena/match/{id}
   */
  async getMatch(matchId: string): Promise<MatchResponse> {
    return this.request<MatchResponse>(`/api/v1/mini-app/arena/match/${matchId}`)
  }

  /**
   * Join an existing match.
   * POST /api/v1/mini-app/arena/match/{id}/join
   */
  async joinMatch(matchId: string): Promise<MatchResponse> {
    return this.request<MatchResponse>(`/api/v1/mini-app/arena/match/${matchId}/join`, {
      method: 'POST',
    })
  }

  /**
   * Leave a match (only during open phase).
   * POST /api/v1/mini-app/arena/match/{id}/leave
   */
  async leaveMatch(matchId: string): Promise<void> {
    await this.request<{ ok: boolean }>(`/api/v1/mini-app/arena/match/${matchId}/leave`, {
      method: 'POST',
    })
  }

  /**
   * Start a match early (creator only, requires 2+ participants).
   * POST /api/v1/mini-app/arena/match/{id}/start
   */
  async startMatch(matchId: string): Promise<MatchResponse> {
    return this.request<MatchResponse>(`/api/v1/mini-app/arena/match/${matchId}/start`, {
      method: 'POST',
    })
  }

  // ==========================================================================
  // Shop Endpoints
  // ==========================================================================

  /**
   * Get current shop state for a match.
   * Returns cards available, player's team, coins, affordability, and upgrade previews.
   * GET /api/v1/mini-app/arena/match/{id}/shop
   */
  async getShop(matchId: string): Promise<ShopResponse> {
    return this.request<ShopResponse>(`/api/v1/mini-app/arena/match/${matchId}/shop`)
  }

  /**
   * Buy a card from the shop.
   * POST /api/v1/mini-app/arena/match/{id}/buy
   * @param matchId - The match ID
   * @param cardIndex - Index of the card in the shop (0-based)
   */
  async buyCard(matchId: string, cardIndex: number): Promise<ShopResponse> {
    return this.request<ShopResponse>(`/api/v1/mini-app/arena/match/${matchId}/buy`, {
      method: 'POST',
      body: JSON.stringify({ card_index: cardIndex }),
    })
  }

  /**
   * Reroll unpurchased shop cards (costs coins).
   * Only available before first card purchase.
   * POST /api/v1/mini-app/arena/match/{id}/reroll
   */
  async rerollCards(matchId: string): Promise<ShopResponse> {
    return this.request<ShopResponse>(`/api/v1/mini-app/arena/match/${matchId}/reroll`, {
      method: 'POST',
    })
  }

  /**
   * Upgrade a card in the team.
   * POST /api/v1/mini-app/arena/match/{id}/upgrade
   * @param matchId - The match ID
   * @param teamSlot - Index of the card in the team (0-2)
   * @param upgradeType - Type of upgrade: 'atk' or 'hp'
   */
  async upgradeCard(
    matchId: string,
    teamSlot: number,
    upgradeType: 'atk' | 'hp'
  ): Promise<ShopResponse> {
    return this.request<ShopResponse>(`/api/v1/mini-app/arena/match/${matchId}/upgrade`, {
      method: 'POST',
      body: JSON.stringify({ team_slot: teamSlot, upgrade_type: upgradeType }),
    })
  }

  /**
   * Set the battle order of cards in the team.
   * POST /api/v1/mini-app/arena/match/{id}/order
   * @param matchId - The match ID
   * @param order - Array of team slot indices in desired order [0,1,2]
   */
  async setTeamOrder(matchId: string, order: number[]): Promise<ShopResponse> {
    return this.request<ShopResponse>(`/api/v1/mini-app/arena/match/${matchId}/order`, {
      method: 'POST',
      body: JSON.stringify({ order }),
    })
  }

  /**
   * Submit the team for battle (locks in choices).
   * POST /api/v1/mini-app/arena/match/{id}/team
   */
  async submitTeam(matchId: string): Promise<ShopResponse> {
    return this.request<ShopResponse>(`/api/v1/mini-app/arena/match/${matchId}/team`, {
      method: 'POST',
    })
  }

  // ==========================================================================
  // Battle Endpoints
  // ==========================================================================

  /**
   * Get battle results and replay events.
   * GET /api/v1/mini-app/arena/match/{id}/battle
   */
  async getBattle(matchId: string): Promise<BattleResponse> {
    return this.request<BattleResponse>(`/api/v1/mini-app/arena/match/${matchId}/battle`)
  }

  /**
   * Share match result to the group.
   * POST /api/v1/mini-app/arena/match/{id}/share
   */
  async shareResult(matchId: string): Promise<ShareResultResponse> {
    return this.request<ShareResultResponse>(`/api/v1/mini-app/arena/match/${matchId}/share`, {
      method: 'POST',
    })
  }

  // ==========================================================================
  // Stats Endpoints
  // ==========================================================================

  /**
   * Get leaderboard for the chat.
   * GET /api/v1/mini-app/arena/leaderboard?chat_id=X&type=ranked&limit=50&offset=0
   * @param type - 'ranked' or 'regular'
   * @param limit - Max entries to return (default 50)
   * @param offset - Pagination offset (default 0)
   */
  async getLeaderboard(
    type: 'ranked' | 'regular' = 'ranked',
    limit = 50,
    offset = 0,
    chatId?: number
  ): Promise<LeaderboardResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }
    return this.request<LeaderboardResponse>(
      `/api/v1/mini-app/arena/leaderboard?chat_id=${targetChatId}&type=${type}&limit=${limit}&offset=${offset}`
    )
  }

  /**
   * Get current user's profile and stats.
   * GET /api/v1/mini-app/arena/profile?chat_id=X
   */
  async getProfile(chatId?: number): Promise<ProfileResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }
    return this.request<ProfileResponse>(
      `/api/v1/mini-app/arena/profile?chat_id=${targetChatId}`
    )
  }

  /**
   * Get user's match history.
   * GET /api/v1/mini-app/arena/history?chat_id=X&limit=20&offset=0
   * @param limit - Max entries to return (default 20)
   * @param offset - Pagination offset (default 0)
   */
  async getMatchHistory(
    limit = 20,
    offset = 0,
    chatId?: number
  ): Promise<MatchHistoryResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }
    return this.request<MatchHistoryResponse>(
      `/api/v1/mini-app/arena/history?chat_id=${targetChatId}&limit=${limit}&offset=${offset}`
    )
  }

  /**
   * Get head-to-head record against a specific opponent.
   * GET /api/v1/mini-app/arena/h2h?chat_id=X&opponent_id=Y
   * @param opponentId - User ID of the opponent
   */
  async getH2HRecord(opponentId: number, chatId?: number): Promise<H2HResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }
    return this.request<H2HResponse>(
      `/api/v1/mini-app/arena/h2h?chat_id=${targetChatId}&opponent_id=${opponentId}`
    )
  }

  // ==========================================================================
  // Configuration Endpoints
  // ==========================================================================

  /**
   * Get game constants (costs, sizes, timings).
   * Should be called once on app load and cached.
   * GET /api/v1/mini-app/arena/constants
   */
  async getConstants(): Promise<GameConstants> {
    return this.request<GameConstants>('/api/v1/mini-app/arena/constants')
  }
}

// Export singleton instance
export const apiClient = new ArenaApiClient()
