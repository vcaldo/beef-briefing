import type {
  AuthResponse,
  Match,
  ShopState,
  BattleResult,
  LeaderboardEntry,
} from '../types';

const API_BASE_URL = import.meta.env.VITE_API_URL || '';

class ApiClient {
  private token: string | null = null;
  private chatId: number | null = null;

  async authenticate(initData: string): Promise<AuthResponse> {
    const response = await fetch(`${API_BASE_URL}/api/v1/mini-app/auth`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ init_data: initData }),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Authentication failed');
    }

    const data: AuthResponse = await response.json();
    this.token = data.token;
    this.chatId = data.chat_id || null;
    return data;
  }

  private getHeaders(): HeadersInit {
    return {
      Authorization: `Bearer ${this.token}`,
      'Content-Type': 'application/json',
    };
  }

  private getChatIdParam(): string {
    return this.chatId ? `chat_id=${this.chatId}` : '';
  }

  // Match endpoints
  async getActiveMatches(): Promise<{ matches: Match[] }> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/matches?${this.getChatIdParam()}`,
      { headers: this.getHeaders() }
    );
    if (!response.ok) throw new Error('Failed to fetch matches');
    return response.json();
  }

  async createMatch(): Promise<Match> {
    const response = await fetch(`${API_BASE_URL}/api/v1/mini-app/arena/match`, {
      method: 'POST',
      headers: this.getHeaders(),
      body: JSON.stringify({ chat_id: this.chatId }),
    });
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to create match');
    }
    return response.json();
  }

  async getMatch(matchId: string): Promise<Match> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}`,
      { headers: this.getHeaders() }
    );
    if (!response.ok) throw new Error('Failed to fetch match');
    return response.json();
  }

  async joinMatch(matchId: string): Promise<Match> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/join`,
      { method: 'POST', headers: this.getHeaders() }
    );
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to join match');
    }
    return response.json();
  }

  async leaveMatch(matchId: string): Promise<void> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/leave`,
      { method: 'POST', headers: this.getHeaders() }
    );
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to leave match');
    }
  }

  async startMatch(matchId: string): Promise<Match> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/start`,
      { method: 'POST', headers: this.getHeaders() }
    );
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to start match');
    }
    return response.json();
  }

  // Shop endpoints
  async getShop(matchId: string): Promise<ShopState> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/shop`,
      { headers: this.getHeaders() }
    );
    if (!response.ok) throw new Error('Failed to fetch shop');
    return response.json();
  }

  async buyCard(matchId: string, cardIndex: number): Promise<ShopState> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/buy`,
      {
        method: 'POST',
        headers: this.getHeaders(),
        body: JSON.stringify({ card_index: cardIndex }),
      }
    );
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to buy card');
    }
    return response.json();
  }

  async reroll(matchId: string): Promise<ShopState> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/reroll`,
      { method: 'POST', headers: this.getHeaders() }
    );
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to reroll');
    }
    return response.json();
  }

  async upgradeCard(
    matchId: string,
    teamSlot: number,
    upgradeType: 'atk' | 'hp'
  ): Promise<ShopState> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/upgrade`,
      {
        method: 'POST',
        headers: this.getHeaders(),
        body: JSON.stringify({ team_slot: teamSlot, upgrade_type: upgradeType }),
      }
    );
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to upgrade card');
    }
    return response.json();
  }

  async setTeamOrder(matchId: string, order: number[]): Promise<ShopState> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/order`,
      {
        method: 'POST',
        headers: this.getHeaders(),
        body: JSON.stringify({ order }),
      }
    );
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to set order');
    }
    return response.json();
  }

  async submitTeam(matchId: string): Promise<ShopState> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/team`,
      { method: 'POST', headers: this.getHeaders() }
    );
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to submit team');
    }
    return response.json();
  }

  // Battle endpoints
  async getBattle(matchId: string): Promise<BattleResult> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/match/${matchId}/battle`,
      { headers: this.getHeaders() }
    );
    if (!response.ok) throw new Error('Failed to fetch battle');
    return response.json();
  }

  // Leaderboard endpoints
  async getLeaderboard(
    type: 'ranked' | 'regular' = 'ranked'
  ): Promise<{ entries: LeaderboardEntry[]; type: string }> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/arena/leaderboard?${this.getChatIdParam()}&type=${type}`,
      { headers: this.getHeaders() }
    );
    if (!response.ok) throw new Error('Failed to fetch leaderboard');
    return response.json();
  }
}

export const apiClient = new ApiClient();
