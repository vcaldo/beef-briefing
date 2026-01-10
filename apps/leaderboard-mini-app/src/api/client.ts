/**
 * API client for leaderboard mini-app.
 * Handles JWT authentication via Mini App init data.
 */

import { getBrowserTimezone } from '../utils/timezone'
import type {
  AuthResponse,
  StatsResponse,
  ActivityResponse,
  LeaderboardResponse,
  ReactionsOverviewResponse,
  RepliesOverviewResponse,
  ProfileResponse,
  HeatmapResponse,
  ChatUsersResponse,
  MediaOverviewResponse,
  ChatTimezoneResponse,
  Period,
  LeaderboardMetric,
  CardImage,
  CardImageWithUrl,
} from '../types'

const API_BASE_URL = import.meta.env.VITE_API_URL || ''

class ApiClient {
  private token: string | null = null
  private chatId: number | null = null
  private isAdmin: boolean = false
  private browserTimezone: string = getBrowserTimezone()
  private storedTimezone: string | null = null

  /**
   * Get the effective timezone to use for API requests.
   * Priority: Stored group timezone > Browser timezone
   */
  private getEffectiveTimezone(): string {
    return this.storedTimezone || this.browserTimezone
  }

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
    this.isAdmin = data.is_admin
    this.storedTimezone = data.chat_timezone || null
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
      `${API_BASE_URL}/api/v1/mini-app/stats?chat_id=${targetChatId}&period=${period}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`,
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
      `${API_BASE_URL}/api/v1/mini-app/activity?chat_id=${targetChatId}&period=${period}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`,
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
      `${API_BASE_URL}/api/v1/mini-app/leaderboard?chat_id=${targetChatId}&period=${period}&metric=${metric}&page=${page}&limit=${limit}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`,
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
      `${API_BASE_URL}/api/v1/mini-app/reactions-overview?chat_id=${targetChatId}&period=${period}&limit=${limit}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`,
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
      `${API_BASE_URL}/api/v1/mini-app/replies-overview?chat_id=${targetChatId}&period=${period}&limit=${limit}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch replies overview')
    }

    return response.json()
  }

  /**
   * Get user profile data (stats, top interactors, personal heatmap).
   * If targetUserId is provided (admin only), fetches that user's profile.
   */
  async getProfile(period: Period = '30d', chatId?: number, targetUserId?: number): Promise<ProfileResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    let url = `${API_BASE_URL}/api/v1/mini-app/profile?chat_id=${targetChatId}&period=${period}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`
    if (targetUserId) {
      url += `&target_user_id=${targetUserId}`
    }

    const response = await fetch(url, { headers: this.getHeaders() })

    if (!response.ok) {
      throw new Error('Failed to fetch profile')
    }

    return response.json()
  }

  /**
   * Get all users in a chat (admin only).
   */
  async getChatUsers(chatId?: number): Promise<ChatUsersResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/users?chat_id=${targetChatId}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch users')
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
      `${API_BASE_URL}/api/v1/mini-app/heatmap?chat_id=${targetChatId}&period=${period}&include_user=${includeUser}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch heatmap')
    }

    return response.json()
  }

  /**
   * Get media overview (distribution, timeline, top senders).
   */
  async getMediaOverview(
    period: Period = '30d',
    limit: number = 10,
    chatId?: number
  ): Promise<MediaOverviewResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/media-overview?chat_id=${targetChatId}&period=${period}&limit=${limit}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch media overview')
    }

    return response.json()
  }

  /**
   * Get list of available weeks with card images.
   */
  async getWeeks(chatId?: number): Promise<string[]> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/gallery/weeks?chat_id=${targetChatId}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch weeks')
    }

    const data = await response.json()
    return data.weeks
  }

  /**
   * Get card images for a specific week.
   */
  async getCards(weekStart: string, chatId?: number): Promise<CardImage[]> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/gallery/images?chat_id=${targetChatId}&week_start=${weekStart}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch cards')
    }

    const data = await response.json()
    return data.images
  }

  /**
   * Get presigned URL for a specific card image.
   */
  async getImageUrl(imageId: number, expiresIn: number = 3600): Promise<string> {
    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/gallery/image/${imageId}?expires=${expiresIn}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch image URL')
    }

    const data = await response.json()
    return data.url
  }

  /**
   * Get the current user's latest card with presigned URL.
   */
  async getUserLatestCard(userId: number, chatId?: number): Promise<CardImageWithUrl | null> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    // Get latest week
    const weeks = await this.getWeeks(targetChatId)
    if (weeks.length === 0) {
      return null
    }

    // Get cards for the latest week
    const cards = await this.getCards(weeks[0], targetChatId)
    const userCard = cards.find((card) => card.user_id === userId)

    if (!userCard) {
      return null
    }

    // Get presigned URL
    const url = await this.getImageUrl(userCard.id)
    return { ...userCard, url }
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

  getIsAdmin(): boolean {
    return this.isAdmin
  }

  isAuthenticated(): boolean {
    return this.token !== null
  }

  getTimezone(): string {
    return this.getEffectiveTimezone()
  }

  getStoredTimezone(): string | null {
    return this.storedTimezone
  }

  getBrowserTimezone(): string {
    return this.browserTimezone
  }

  /**
   * Get the stored group timezone from the API.
   */
  async getGroupTimezone(chatId?: number): Promise<ChatTimezoneResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/settings/timezone?chat_id=${targetChatId}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch timezone')
    }

    return response.json()
  }

  /**
   * Set the group timezone (admin only).
   */
  async setGroupTimezone(timezone: string, chatId?: number): Promise<ChatTimezoneResponse> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/mini-app/settings/timezone?chat_id=${targetChatId}`,
      {
        method: 'PUT',
        headers: this.getHeaders(),
        body: JSON.stringify({ timezone }),
      }
    )

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Failed to set timezone' }))
      throw new Error(error.error || 'Failed to set timezone')
    }

    const result: ChatTimezoneResponse = await response.json()
    this.storedTimezone = result.timezone // Update cached value
    return result
  }
}

// Singleton instance
export const apiClient = new ApiClient()
