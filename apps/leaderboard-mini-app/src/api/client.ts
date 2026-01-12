/**
 * API client for leaderboard mini-app.
 * Extends shared BaseApiClient with timezone-aware endpoints.
 */

import { BaseApiClient } from '@beef-briefing/shared-mini-app/api'
import { getBrowserTimezone } from '@beef-briefing/shared-mini-app/utils'
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

class LeaderboardApiClient extends BaseApiClient {
  private browserTimezone: string = getBrowserTimezone()
  private storedTimezone: string | null = null

  constructor(baseUrl?: string) {
    super(baseUrl || import.meta.env.VITE_API_URL || '')
  }

  /**
   * Get the effective timezone to use for API requests.
   * Priority: Stored group timezone > Browser timezone
   */
  private getEffectiveTimezone(): string {
    return this.storedTimezone || this.browserTimezone
  }

  /**
   * Override authenticate to store timezone from response.
   */
  override async authenticate(initData: string): Promise<AuthResponse> {
    const data = await super.authenticate(initData)
    this.storedTimezone = data.chat_timezone || null
    return data
  }

  /**
   * Get overview statistics for the chat.
   */
  async getStats(period: Period = '30d', chatId?: number): Promise<StatsResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<StatsResponse>(
      `/api/v1/mini-app/stats?chat_id=${targetChatId}&period=${period}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`
    )
  }

  /**
   * Get activity timeline data for the chat.
   */
  async getActivity(period: Period = '30d', chatId?: number): Promise<ActivityResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<ActivityResponse>(
      `/api/v1/mini-app/activity?chat_id=${targetChatId}&period=${period}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`
    )
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
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<LeaderboardResponse>(
      `/api/v1/mini-app/leaderboard?chat_id=${targetChatId}&period=${period}&metric=${metric}&page=${page}&limit=${limit}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`
    )
  }

  /**
   * Get reactions overview (top reactions, givers, receivers).
   */
  async getReactionsOverview(
    period: Period = '30d',
    limit: number = 10,
    chatId?: number
  ): Promise<ReactionsOverviewResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<ReactionsOverviewResponse>(
      `/api/v1/mini-app/reactions-overview?chat_id=${targetChatId}&period=${period}&limit=${limit}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`
    )
  }

  /**
   * Get replies overview (top senders, receivers).
   */
  async getRepliesOverview(
    period: Period = '30d',
    limit: number = 10,
    chatId?: number
  ): Promise<RepliesOverviewResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<RepliesOverviewResponse>(
      `/api/v1/mini-app/replies-overview?chat_id=${targetChatId}&period=${period}&limit=${limit}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`
    )
  }

  /**
   * Get user profile data (stats, top interactors, personal heatmap).
   * If targetUserId is provided (admin only), fetches that user's profile.
   */
  async getProfile(period: Period = '30d', chatId?: number, targetUserId?: number): Promise<ProfileResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    let endpoint = `/api/v1/mini-app/profile?chat_id=${targetChatId}&period=${period}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`
    if (targetUserId) {
      endpoint += `&target_user_id=${targetUserId}`
    }

    return this.request<ProfileResponse>(endpoint)
  }

  /**
   * Get all users in a chat (admin only).
   */
  async getChatUsers(chatId?: number): Promise<ChatUsersResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<ChatUsersResponse>(
      `/api/v1/mini-app/users?chat_id=${targetChatId}`
    )
  }

  /**
   * Get activity heatmap data (group and optionally user).
   */
  async getHeatmap(
    period: Period = 'max',
    includeUser: boolean = true,
    chatId?: number
  ): Promise<HeatmapResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<HeatmapResponse>(
      `/api/v1/mini-app/heatmap?chat_id=${targetChatId}&period=${period}&include_user=${includeUser}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`
    )
  }

  /**
   * Get media overview (distribution, timeline, top senders).
   */
  async getMediaOverview(
    period: Period = '30d',
    limit: number = 10,
    chatId?: number
  ): Promise<MediaOverviewResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<MediaOverviewResponse>(
      `/api/v1/mini-app/media-overview?chat_id=${targetChatId}&period=${period}&limit=${limit}&tz=${encodeURIComponent(this.getEffectiveTimezone())}`
    )
  }

  /**
   * Get list of available weeks with card images.
   */
  async getWeeks(chatId?: number): Promise<string[]> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const data = await this.request<{ weeks: string[] }>(
      `/api/v1/mini-app/gallery/weeks?chat_id=${targetChatId}`
    )
    return data.weeks
  }

  /**
   * Get card images for a specific week.
   */
  async getCards(weekStart: string, chatId?: number): Promise<CardImage[]> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const data = await this.request<{ images: CardImage[] }>(
      `/api/v1/mini-app/gallery/images?chat_id=${targetChatId}&week_start=${weekStart}`
    )
    return data.images
  }

  /**
   * Get presigned URL for a specific card image.
   */
  async getImageUrl(imageId: number, expiresIn: number = 3600): Promise<string> {
    const data = await this.request<{ url: string }>(
      `/api/v1/mini-app/gallery/image/${imageId}?expires=${expiresIn}`
    )
    return data.url
  }

  /**
   * Get the current user's latest card with presigned URL.
   */
  async getUserLatestCard(userId: number, chatId?: number): Promise<CardImageWithUrl | null> {
    const targetChatId = chatId || this.getChatId()
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

  // ========== Timezone Getter Methods (App-specific) ==========

  /**
   * Get the effective timezone (stored or browser).
   */
  getTimezone(): string {
    return this.getEffectiveTimezone()
  }

  /**
   * Get the stored group timezone.
   */
  getStoredTimezone(): string | null {
    return this.storedTimezone
  }

  /**
   * Get the browser timezone.
   */
  getBrowserTimezone(): string {
    return this.browserTimezone
  }

  // ========== Timezone API Endpoints ==========

  /**
   * Get the stored group timezone from the API.
   */
  async getGroupTimezone(chatId?: number): Promise<ChatTimezoneResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    return this.request<ChatTimezoneResponse>(
      `/api/v1/mini-app/settings/timezone?chat_id=${targetChatId}`
    )
  }

  /**
   * Set the group timezone (admin only).
   */
  async setGroupTimezone(timezone: string, chatId?: number): Promise<ChatTimezoneResponse> {
    const targetChatId = chatId || this.getChatId()
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const result = await this.request<ChatTimezoneResponse>(
      `/api/v1/mini-app/settings/timezone?chat_id=${targetChatId}`,
      {
        method: 'PUT',
        body: JSON.stringify({ timezone }),
      }
    )

    this.storedTimezone = result.timezone // Update cached value
    return result
  }
}

// Singleton instance
export const apiClient = new LeaderboardApiClient()
