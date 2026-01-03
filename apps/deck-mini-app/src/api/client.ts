/**
 * API client for card-image-generator service.
 * Handles JWT authentication via Mini App init data.
 */

const API_BASE_URL = import.meta.env.VITE_API_URL || ''

export interface AuthResponse {
  token: string
  user_id: number
  chat_id: number | null
  first_name: string
  username: string | null
}

export interface CardImage {
  id: number
  user_id: number
  chat_id: number
  week_start: string
  storage_path: string
  theme: string
  generated_at: string
  first_name: string | null
  last_name: string | null
  username: string | null
}

export interface CardImageWithUrl extends CardImage {
  url: string
}

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
   * Get list of available weeks with card images.
   */
  async getWeeks(chatId?: number): Promise<string[]> {
    const targetChatId = chatId || this.chatId
    if (!targetChatId) {
      throw new Error('No chat ID available')
    }

    const response = await fetch(
      `${API_BASE_URL}/api/v1/weeks?chat_id=${targetChatId}`,
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
      `${API_BASE_URL}/api/v1/images?chat_id=${targetChatId}&week_start=${weekStart}`,
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
      `${API_BASE_URL}/api/v1/image/${imageId}?expires=${expiresIn}`,
      { headers: this.getHeaders() }
    )

    if (!response.ok) {
      throw new Error('Failed to fetch image URL')
    }

    const data = await response.json()
    return data.url
  }

  /**
   * Get cards with their presigned URLs (convenience method).
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
