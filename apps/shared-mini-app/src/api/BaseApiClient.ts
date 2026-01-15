/**
 * Base API client class with common JWT authentication logic.
 * Mini app-specific clients should extend this class and add their endpoints.
 */

import type { AuthResponse } from '../types'

/** Default timeout for API requests (15 seconds) */
const DEFAULT_TIMEOUT_MS = 15000

/** Error types for better error handling */
export type ApiErrorType = 'network' | 'timeout' | 'auth' | 'server' | 'validation' | 'unknown'

/** Custom API error with type categorization */
export class ApiError extends Error {
  readonly type: ApiErrorType
  readonly statusCode?: number
  readonly retryable: boolean

  constructor(message: string, type: ApiErrorType, statusCode?: number) {
    super(message)
    this.name = 'ApiError'
    this.type = type
    this.statusCode = statusCode
    // Network and timeout errors are retryable, most server errors are too
    this.retryable = type === 'network' || type === 'timeout' ||
                     (type === 'server' && statusCode !== undefined && statusCode >= 500)
  }
}

/**
 * Wraps a fetch promise with a timeout.
 * Aborts the request if it takes longer than the specified timeout.
 */
async function fetchWithTimeout(
  url: string,
  options: RequestInit & { timeout?: number }
): Promise<Response> {
  const { timeout = DEFAULT_TIMEOUT_MS, ...fetchOptions } = options
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeout)

  try {
    const response = await fetch(url, {
      ...fetchOptions,
      signal: controller.signal,
    })
    return response
  } catch (error) {
    if (error instanceof Error && error.name === 'AbortError') {
      throw new ApiError(
        'Request timed out. Please check your connection and try again.',
        'timeout'
      )
    }
    // Handle network errors (offline, DNS failure, etc.)
    if (error instanceof TypeError && error.message.includes('fetch')) {
      throw new ApiError(
        'Network error. Please check your internet connection.',
        'network'
      )
    }
    throw error
  } finally {
    clearTimeout(timeoutId)
  }
}

/**
 * Categorize HTTP status codes into error types
 */
function getErrorTypeFromStatus(status: number): ApiErrorType {
  if (status === 401 || status === 403) return 'auth'
  if (status === 400 || status === 422) return 'validation'
  if (status >= 500) return 'server'
  return 'unknown'
}

export class BaseApiClient {
  protected baseUrl: string
  protected token: string | null = null
  protected chatId: number | null = null
  protected isAdmin: boolean = false
  protected timeoutMs: number = DEFAULT_TIMEOUT_MS

  constructor(baseUrl?: string, timeoutMs?: number) {
    this.baseUrl = baseUrl || window.location.origin
    if (timeoutMs) this.timeoutMs = timeoutMs
  }

  /**
   * Set the request timeout in milliseconds.
   */
  setTimeout(ms: number): void {
    this.timeoutMs = ms
  }

  /**
   * Check if the device appears to be online.
   * Note: This is a hint, not a guarantee - the actual request may still fail.
   */
  isOnline(): boolean {
    return typeof navigator !== 'undefined' ? navigator.onLine : true
  }

  /**
   * Authenticate with Mini App init data.
   * Returns user info and stores JWT token for subsequent requests.
   *
   * This method must be called before making any authenticated requests.
   */
  async authenticate(initData: string): Promise<AuthResponse> {
    if (!this.isOnline()) {
      throw new ApiError(
        'You appear to be offline. Please check your connection.',
        'network'
      )
    }

    let response: Response
    try {
      response = await fetchWithTimeout(`${this.baseUrl}/api/v1/mini-app/auth`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ init_data: initData }),
        timeout: this.timeoutMs,
      })
    } catch (error) {
      if (error instanceof ApiError) throw error
      throw new ApiError(
        'Failed to connect to server. Please try again.',
        'network'
      )
    }

    if (!response.ok) {
      const errorBody = await response.json().catch(() => ({ detail: 'Authentication failed' }))
      const errorType = getErrorTypeFromStatus(response.status)
      throw new ApiError(
        errorBody.detail || 'Authentication failed',
        errorType,
        response.status
      )
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
   * @throws ApiError with type categorization for better error handling
   */
  protected async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    if (!this.token) {
      throw new ApiError(
        'Not authenticated. Please reload the app.',
        'auth'
      )
    }

    if (!this.isOnline()) {
      throw new ApiError(
        'You appear to be offline. Please check your connection.',
        'network'
      )
    }

    let response: Response
    try {
      response = await fetchWithTimeout(`${this.baseUrl}${endpoint}`, {
        ...options,
        headers: {
          ...options?.headers,
          'Authorization': `Bearer ${this.token}`,
          'Content-Type': 'application/json',
        },
        timeout: this.timeoutMs,
      })
    } catch (error) {
      if (error instanceof ApiError) throw error
      throw new ApiError(
        'Failed to connect to server. Please try again.',
        'network'
      )
    }

    if (!response.ok) {
      const errorBody = await response.json().catch(() => ({ detail: 'Request failed' }))
      const errorType = getErrorTypeFromStatus(response.status)
      throw new ApiError(
        errorBody.detail || `Request failed: ${response.statusText}`,
        errorType,
        response.status
      )
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
