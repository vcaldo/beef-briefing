/**
 * Base API client class with common JWT authentication logic.
 * Mini app-specific clients should extend this class and add their endpoints.
 */

import type { AuthResponse } from '../types'

export class BaseApiClient {
  protected baseUrl: string
  protected token: string | null = null
  protected chatId: number | null = null
  protected isAdmin: boolean = false

  constructor(baseUrl?: string) {
    this.baseUrl = baseUrl || window.location.origin
  }

  /**
   * Authenticate with Mini App init data.
   * Returns user info and stores JWT token for subsequent requests.
   *
   * This method must be called before making any authenticated requests.
   */
  async authenticate(initData: string): Promise<AuthResponse> {
    const response = await fetch(`${this.baseUrl}/api/v1/mini-app/auth`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ init_data: initData }),
    })

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({ error: 'Authentication failed' }))
      throw new Error(errorData.error || errorData.detail || 'Authentication failed')
    }

    const data: AuthResponse = await response.json()
    this.token = data.token
    this.chatId = data.chat_id ?? null
    this.isAdmin = data.is_admin ?? false
    return data
  }

  /**
   * Make an authenticated request to the API.
   * Automatically includes the Authorization header with JWT token.
   *
   * @param endpoint - The API endpoint path (e.g., '/api/v1/mini-app/stats')
   * @param options - Fetch options (method, body, etc.)
   * @returns The parsed JSON response
   * @throws Error if not authenticated or request fails
   */
  protected async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    if (!this.token) {
      throw new Error('Not authenticated. Call authenticate() first.')
    }

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...options,
      headers: {
        ...options?.headers,
        'Authorization': `Bearer ${this.token}`,
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({ error: 'Request failed' }))
      throw new Error(errorData.error || errorData.detail || `Request failed: ${response.statusText}`)
    }

    return response.json()
  }

  /**
   * Get the current chat ID.
   * Returns null if not authenticated.
   */
  getChatId(): number | null {
    return this.chatId
  }

  /**
   * Check if the current user is an admin.
   */
  isUserAdmin(): boolean {
    return this.isAdmin
  }

  /**
   * Check if the client is authenticated.
   */
  isAuthenticated(): boolean {
    return this.token !== null
  }

  /**
   * Clear authentication state (useful for logout).
   */
  clearAuth(): void {
    this.token = null
    this.chatId = null
    this.isAdmin = false
  }
}
