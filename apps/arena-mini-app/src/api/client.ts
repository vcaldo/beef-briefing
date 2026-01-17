/**
 * API client for arena-mini-app.
 * Extends shared BaseApiClient with arena-specific game endpoints.
 */

import { BaseApiClient } from '@beef-briefing/shared-mini-app/api'
import type {
  Match,
  MatchesResponse,
  EnhancedShopResponse,
  BattleResult,
  LeaderboardResponse,
  HistoryResponse,
  H2HResponse,
  ProfileResponse,
  GameConstants,
  CardImageResponse,
} from '../types'

class ArenaApiClient extends BaseApiClient {
  constructor(baseUrl?: string) {
    super(baseUrl || import.meta.env.VITE_API_URL || '')
  }

  // =============================================================================
  // MATCH MANAGEMENT (6 endpoints)
  // =============================================================================

  /**
   * Get list of active matches for a chat.
   * GET /api/v1/mini-app/arena/matches?chat_id=X
   */
  async getMatches(chatId?: number): Promise<Match[]> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const data = await this.request<MatchesResponse>(
      `/api/v1/mini-app/arena/matches?chat_id=${targetChatId}`
    )
    return data.matches
  }

  /**
   * Create a new match.
   * POST /api/v1/mini-app/arena/match
   */
  async createMatch(chatId?: number): Promise<Match> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<Match>('/api/v1/mini-app/arena/match', {
      method: 'POST',
      body: JSON.stringify({ chat_id: targetChatId }),
    })
  }

  /**
   * Get match details by ID.
   * GET /api/v1/mini-app/arena/match/{id}
   */
  async getMatch(matchId: string): Promise<Match> {
    return this.request<Match>(`/api/v1/mini-app/arena/match/${matchId}`)
  }

  /**
   * Join a match.
   * POST /api/v1/mini-app/arena/match/{id}/join
   */
  async joinMatch(matchId: string): Promise<Match> {
    return this.request<Match>(`/api/v1/mini-app/arena/match/${matchId}/join`, {
      method: 'POST',
    })
  }

  /**
   * Leave a match.
   * POST /api/v1/mini-app/arena/match/{id}/leave
   */
  async leaveMatch(matchId: string): Promise<void> {
    await this.request<{ success: boolean }>(
      `/api/v1/mini-app/arena/match/${matchId}/leave`,
      {
        method: 'POST',
      }
    )
  }

  /**
   * Start a match early (creator only, requires 2+ players).
   * POST /api/v1/mini-app/arena/match/{id}/start
   */
  async startMatch(matchId: string): Promise<Match> {
    return this.request<Match>(`/api/v1/mini-app/arena/match/${matchId}/start`, {
      method: 'POST',
    })
  }

  // =============================================================================
  // SHOP PHASE (6 endpoints)
  // =============================================================================

  /**
   * Get shop state for a match.
   * GET /api/v1/mini-app/arena/match/{id}/shop
   */
  async getShop(matchId: string): Promise<EnhancedShopResponse> {
    return this.request<EnhancedShopResponse>(
      `/api/v1/mini-app/arena/match/${matchId}/shop`
    )
  }

  /**
   * Buy a card from the shop.
   * POST /api/v1/mini-app/arena/match/{id}/buy
   */
  async buyCard(matchId: string, cardIndex: number): Promise<EnhancedShopResponse> {
    return this.request<EnhancedShopResponse>(
      `/api/v1/mini-app/arena/match/${matchId}/buy`,
      {
        method: 'POST',
        body: JSON.stringify({ card_index: cardIndex }),
      }
    )
  }

  /**
   * Reroll the shop (only before first card purchase).
   * POST /api/v1/mini-app/arena/match/{id}/reroll
   */
  async rerollShop(matchId: string): Promise<EnhancedShopResponse> {
    return this.request<EnhancedShopResponse>(
      `/api/v1/mini-app/arena/match/${matchId}/reroll`,
      {
        method: 'POST',
      }
    )
  }

  /**
   * Upgrade a card in the team.
   * POST /api/v1/mini-app/arena/match/{id}/upgrade
   * @param teamSlot - The slot index (0-2) of the card to upgrade
   * @param upgradeType - Either "atk" or "hp"
   */
  async upgradeCard(
    matchId: string,
    teamSlot: number,
    upgradeType: 'atk' | 'hp'
  ): Promise<EnhancedShopResponse> {
    return this.request<EnhancedShopResponse>(
      `/api/v1/mini-app/arena/match/${matchId}/upgrade`,
      {
        method: 'POST',
        body: JSON.stringify({ team_slot: teamSlot, upgrade_type: upgradeType }),
      }
    )
  }

  /**
   * Set the battle order for the team.
   * POST /api/v1/mini-app/arena/match/{id}/order
   * @param order - Array of team slot indices in battle order
   */
  async setTeamOrder(matchId: string, order: number[]): Promise<EnhancedShopResponse> {
    return this.request<EnhancedShopResponse>(
      `/api/v1/mini-app/arena/match/${matchId}/order`,
      {
        method: 'POST',
        body: JSON.stringify({ order }),
      }
    )
  }

  /**
   * Submit the team for battle.
   * POST /api/v1/mini-app/arena/match/{id}/team
   */
  async submitTeam(matchId: string): Promise<EnhancedShopResponse> {
    return this.request<EnhancedShopResponse>(
      `/api/v1/mini-app/arena/match/${matchId}/team`,
      {
        method: 'POST',
      }
    )
  }

  // =============================================================================
  // BATTLE & STATS (6 endpoints)
  // =============================================================================

  /**
   * Get battle results for a match.
   * GET /api/v1/mini-app/arena/match/{id}/battle
   */
  async getBattle(matchId: string): Promise<BattleResult> {
    return this.request<BattleResult>(
      `/api/v1/mini-app/arena/match/${matchId}/battle`
    )
  }

  /**
   * Get leaderboard for a chat.
   * GET /api/v1/mini-app/arena/leaderboard?chat_id=X&type=ranked&limit=50&offset=0
   */
  async getLeaderboard(
    chatId?: number,
    type: 'ranked' | 'casual' = 'ranked',
    limit: number = 50,
    offset: number = 0
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
   * Get match history for the current user.
   * GET /api/v1/mini-app/arena/history?chat_id=X&limit=20&offset=0
   */
  async getHistory(
    chatId?: number,
    limit: number = 20,
    offset: number = 0
  ): Promise<HistoryResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<HistoryResponse>(
      `/api/v1/mini-app/arena/history?chat_id=${targetChatId}&limit=${limit}&offset=${offset}`
    )
  }

  /**
   * Get head-to-head record against a specific opponent.
   * GET /api/v1/mini-app/arena/h2h?chat_id=X&opponent_id=Y
   */
  async getH2H(opponentId: number, chatId?: number): Promise<H2HResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<H2HResponse>(
      `/api/v1/mini-app/arena/h2h?chat_id=${targetChatId}&opponent_id=${opponentId}`
    )
  }

  /**
   * Get the current user's profile with stats.
   * GET /api/v1/mini-app/arena/profile?chat_id=X
   */
  async getProfile(chatId?: number): Promise<ProfileResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    // Backend returns: { profile: {...}, recent_matches: [...] }
    const response = await this.request<{
      profile: any
      recent_matches: any[]
    }>(`/api/v1/mini-app/arena/profile?chat_id=${targetChatId}`)

    // If no profile (new user), return minimal structure without stats
    if (!response.profile) {
      return {
        user_id: 0,
        first_name: 'Unknown',
      }
    }

    const p = response.profile

    // Transform backend response to match frontend ProfileResponse structure
    return {
      user_id: p.user_id,
      first_name: p.first_name,
      username: p.username || undefined,
      photo_url: undefined, // Backend doesn't return photo_url yet
      stats: {
        ranked_wins: p.ranked_wins,
        ranked_losses: p.ranked_losses,
        ranked_draws: p.ranked_draws,
        ranked_matches_played: p.ranked_matches,
        ranked_win_rate: p.ranked_win_rate,
        ranked_current_streak: p.ranked_current_streak,
        ranked_best_streak: p.ranked_best_streak,
        ranked_tournaments_played: p.ranked_tournaments_played,
        ranked_tournaments_won: p.ranked_tournaments_won,
        regular_wins: p.regular_wins,
        regular_losses: p.regular_losses,
        regular_draws: p.regular_draws,
        regular_matches_played: p.regular_matches_played,
        regular_win_rate: p.regular_win_rate,
        regular_current_streak: p.regular_current_streak,
        regular_best_streak: p.regular_best_streak,
        total_matches: p.total_matches,
        total_wins: p.total_wins,
        total_damage_dealt: p.total_damage_dealt || 0,
        rank: p.ranked_rank || undefined,
        tier: undefined, // Backend doesn't calculate tier yet
      },
    }
  }

  /**
   * Get game configuration constants.
   * GET /api/v1/mini-app/arena/constants
   */
  async getConstants(): Promise<GameConstants> {
    return this.request<GameConstants>('/api/v1/mini-app/arena/constants')
  }

  // =============================================================================
  // CARD IMAGES (from Cards API)
  // =============================================================================

  /**
   * Get card image with presigned URL.
   * Fetches weekly stats card images from the card-renderer API.
   * @param userId - The user ID to get the card for
   * @param weekOffset - Week offset from current week (0 = current, -1 = last week)
   * @param cardType - Card type: "regular" (400x600) or "compact" (300x450)
   */
  async getCardImage(
    userId: number,
    weekOffset: number = 0,
    _cardType: 'regular' | 'compact' = 'regular'
  ): Promise<CardImageResponse | null> {
    const chatId = this.getChatId()
    if (!chatId) {
      throw new Error('No chat ID available')
    }

    // Calculate week start date based on offset
    const now = new Date()
    const dayOfWeek = now.getDay()
    const daysToMonday = dayOfWeek === 0 ? 6 : dayOfWeek - 1
    const weekStart = new Date(now)
    weekStart.setDate(now.getDate() - daysToMonday + weekOffset * 7)
    const weekStartStr = weekStart.toISOString().split('T')[0]

    try {
      const data = await this.request<{ images: CardImageResponse[] }>(
        `/api/v1/mini-app/gallery/images?chat_id=${chatId}&week_start=${weekStartStr}&user_id=${userId}`
      )

      if (data.images && data.images.length > 0) {
        // Find the image for this user
        const userImage = data.images.find((img) => img.user_id === userId)
        if (userImage) {
          // Get presigned URL for the image
          const urlData = await this.request<{ url: string }>(
            `/api/v1/mini-app/gallery/image/${userImage.id}?expires=3600`
          )
          return {
            ...userImage,
            url: urlData.url,
          }
        }
      }

      return null
    } catch (error) {
      console.error('Failed to get card image:', error)
      return null
    }
  }

  /**
   * Get presigned URL for a specific card image by ID.
   * @param imageId - The image ID from the card database
   * @param expiresIn - URL expiration time in seconds (default: 3600)
   */
  async getCardImageUrl(imageId: number, expiresIn: number = 3600): Promise<string> {
    const data = await this.request<{ url: string }>(
      `/api/v1/mini-app/gallery/image/${imageId}?expires=${expiresIn}`
    )
    return data.url
  }
}

// Singleton instance
export const apiClient = new ArenaApiClient()

// Export class for testing
export { ArenaApiClient }
