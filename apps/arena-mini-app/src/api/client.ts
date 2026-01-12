import { BaseApiClient } from '@beef-briefing/shared-mini-app/api'
import type {
  AuthResponse,
  Match,
  ShopState,
  BattleResult,
  LeaderboardEntry,
  MatchHistoryEntry,
  H2HRecord,
  ArenaProfile,
} from '../types'

/**
 * Arena Mini App API client.
 * Extends BaseApiClient with Arena-specific endpoints for battle gameplay.
 */
class ArenaApiClient extends BaseApiClient {
  constructor(baseUrl?: string) {
    super(baseUrl || import.meta.env.VITE_API_URL || '')
  }

  // Match endpoints
  async getActiveMatches(): Promise<{ matches: Match[] }> {
    const chatIdParam = this.getChatId() ? `chat_id=${this.getChatId()}` : ''
    return this.request(`/api/v1/mini-app/arena/matches?${chatIdParam}`)
  }

  async createMatch(): Promise<Match> {
    return this.request('/api/v1/mini-app/arena/match', {
      method: 'POST',
      body: JSON.stringify({ chat_id: this.getChatId() }),
    })
  }

  async getMatch(matchId: string): Promise<Match> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}`)
  }

  async joinMatch(matchId: string): Promise<Match> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/join`, {
      method: 'POST',
    })
  }

  async leaveMatch(matchId: string): Promise<void> {
    await this.request(`/api/v1/mini-app/arena/match/${matchId}/leave`, {
      method: 'POST',
    })
  }

  async startMatch(matchId: string): Promise<Match> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/start`, {
      method: 'POST',
    })
  }

  // Shop endpoints
  async getShop(matchId: string): Promise<ShopState> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/shop`)
  }

  async buyCard(matchId: string, cardIndex: number): Promise<ShopState> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/buy`, {
      method: 'POST',
      body: JSON.stringify({ card_index: cardIndex }),
    })
  }

  async reroll(matchId: string): Promise<ShopState> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/reroll`, {
      method: 'POST',
    })
  }

  async upgradeCard(
    matchId: string,
    teamSlot: number,
    upgradeType: 'atk' | 'hp'
  ): Promise<ShopState> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/upgrade`, {
      method: 'POST',
      body: JSON.stringify({ team_slot: teamSlot, upgrade_type: upgradeType }),
    })
  }

  async setTeamOrder(matchId: string, order: number[]): Promise<ShopState> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/order`, {
      method: 'POST',
      body: JSON.stringify({ order }),
    })
  }

  async submitTeam(matchId: string): Promise<ShopState> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/team`, {
      method: 'POST',
    })
  }

  // Battle endpoints
  async getBattle(matchId: string): Promise<BattleResult> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/battle`)
  }

  // Leaderboard endpoints
  async getLeaderboard(
    type: 'ranked' | 'regular' = 'ranked'
  ): Promise<{ entries: LeaderboardEntry[]; type: string }> {
    const chatIdParam = this.getChatId() ? `chat_id=${this.getChatId()}` : ''
    return this.request(`/api/v1/mini-app/arena/leaderboard?${chatIdParam}&type=${type}`)
  }

  // History endpoints
  async getHistory(
    limit: number = 20,
    offset: number = 0
  ): Promise<{ matches: MatchHistoryEntry[]; total: number; has_more: boolean }> {
    const chatIdParam = this.getChatId() ? `chat_id=${this.getChatId()}` : ''
    return this.request(
      `/api/v1/mini-app/arena/history?${chatIdParam}&limit=${limit}&offset=${offset}`
    )
  }

  // Head-to-head endpoints
  async getH2H(
    opponentId?: number
  ): Promise<{ record: H2HRecord | null; recent_matches: MatchHistoryEntry[] }> {
    const chatIdParam = this.getChatId() ? `chat_id=${this.getChatId()}` : ''
    let path = `/api/v1/mini-app/arena/h2h?${chatIdParam}`
    if (opponentId) {
      path += `&opponent_id=${opponentId}`
    }
    return this.request(path)
  }

  // Profile endpoint
  async getProfile(): Promise<{ profile: ArenaProfile | null; recent_matches: MatchHistoryEntry[] }> {
    const chatIdParam = this.getChatId() ? `chat_id=${this.getChatId()}` : ''
    return this.request(`/api/v1/mini-app/arena/profile?${chatIdParam}`)
  }

  // Share result
  async shareResult(matchId: string): Promise<{ success: boolean; message?: string }> {
    return this.request(`/api/v1/mini-app/arena/match/${matchId}/share`, {
      method: 'POST',
    })
  }
}

export const apiClient = new ArenaApiClient()
// Re-export AuthResponse for backward compatibility
export type { AuthResponse }
