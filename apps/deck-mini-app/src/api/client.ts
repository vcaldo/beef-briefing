/**
 * API client for deck-mini-app.
 * Extends shared BaseApiClient with deck-specific gallery endpoints.
 */

import { BaseApiClient } from '@beef-briefing/shared-mini-app/api'
import type { CardImage, CardImageWithUrl } from '../types'

// Re-export types for convenience
export type { CardImage, CardImageWithUrl }

class DeckApiClient extends BaseApiClient {
  constructor(baseUrl?: string) {
    super(baseUrl || import.meta.env.VITE_API_URL || '')
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
   * Get cards with their presigned URLs (convenience method).
   * Fetches URLs in parallel for efficiency.
   */
  async getCardsWithUrls(weekStart: string, chatId?: number): Promise<CardImageWithUrl[]> {
    const cards = await this.getCards(weekStart, chatId)

    // Fetch URLs in parallel
    const cardsWithUrls = await Promise.all(
      cards.map(async (card) => {
        try {
          const url = await this.getImageUrl(card.id)
          return { ...card, url }
        } catch (error) {
          console.error(`Failed to get URL for card ${card.id}:`, error)
          return { ...card, url: '' }
        }
      })
    )

    return cardsWithUrls.filter((card) => card.url !== '')
  }
}

// Singleton instance
export const apiClient = new DeckApiClient()
