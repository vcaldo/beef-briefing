/**
 * API client for leaderboard mini-app.
 * Handles JWT authentication via Mini App init data.
 */

import type {
  AuthResponse,
  StatsResponse,
  ActivityResponse,
  LeaderboardResponse,
  ReactionsOverviewResponse,
  RepliesOverviewResponse,
  ProfileResponse,
  HeatmapResponse,
  Period,
  LeaderboardMetric,
} from '../types'

const API_BASE_URL = import.meta.env.VITE_API_URL || ''

class ApiClient {
  private token: string | null = null
  private chatId: number | null = null

  /**
   * Authenticate with Mini App init data.
   * Returns user info and stores JWT token for subsequent requests.
   */
  async authenticate(initData: string): Promise<AuthResponse> {
    const response = await fetch(`${API_BASE_URL}/api/v1/mini-app/auth`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ init_data: initData }),
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ detail: 'Authentication failed' }))
      throw new Error(error.detail || 'Authentication failed')
    }

    const data: AuthResponse = await response.json()
    this.token = data.token
    this.chatId = data.chat_id
    return data
  }

  /**
   * Get overview statistics for the chat.
   */
  async getStats(period: Period = '30d', chatId?: number): Promise<StatsResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/stats?chat_id=${targetChatId}&period=${period}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch stats')
    }

    return response.json()
  }

  /**
   * Get activity timeline data for the chat.
   */
  async getActivity(period: Period = '30d', chatId?: number): Promise<ActivityResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/activity?chat_id=${targetChatId}&period=${period}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch activity')
    }

    return response.json()
  }

  /**
   * Get user leaderboard for the chat.
   */
  async getLeaderboard(
    period: Period = '30d',
    metric: LeaderboardMetric = 'message_count',
    page: number = 1,
    limit: number = 20,
    chatId?: number
  ): Promise<LeaderboardResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/leaderboard?chat_id=${targetChatId}&period=${period}&metric=${metric}&page=${page}&limit=${limit}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch leaderboard')
    }

    return response.json()
  }

  /**
   * Get reactions overview (top reactions, givers, receivers).
   */
  async getReactionsOverview(
    period: Period = '30d',
    limit: number = 10,
    chatId?: number
  ): Promise<ReactionsOverviewResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/reactions-overview?chat_id=${targetChatId}&period=${period}&limit=${limit}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch reactions overview')
    }

    return response.json()
  }

  /**
   * Get replies overview (top senders, receivers).
   */
  async getRepliesOverview(
    period: Period = '30d',
    limit: number = 10,
    chatId?: number
  ): Promise<RepliesOverviewResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/replies-overview?chat_id=${targetChatId}&period=${period}&limit=${limit}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch replies overview')
    }

    return response.json()
  }

  /**
   * Get user profile data (stats, top interactors, personal heatmap).
   */
  async getProfile(period: Period = '30d', chatId?: number): Promise<ProfileResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/profile?chat_id=${targetChatId}&period=${period}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch profile')
    }

    return response.json()
  }

  /**
   * Get activity heatmap data (group and optionally user).
   */
  async getHeatmap(
    period: Period = 'max',
    includeUser: boolean = true,
    chatId?: number
  ): Promise<HeatmapResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/heatmap?chat_id=${targetChatId}&period=${period}&include_user=${includeUser}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch heatmap')
    }

    return response.json()
  }

  private getHeaders(): HeadersInit {
    if (!this.token) {
      throw new Error('Not authenticated')
    }
    return {
      Authorization: `Bearer ${this.token}`,
      'Content-Type': 'application/json',
    }
  }

  getChatId(): number | null {
    return this.chatId
  }

  isAuthenticated(): boolean {
    return this.token !== null
  }
}

// Singleton instance
export const apiClient = new ApiClient()
