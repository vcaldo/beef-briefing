/**
 * URL parameter parsing utilities for Games API authentication.
 */

export interface GameAuthParams {
  chatId: number
  userId: number
  matchId: string
  ts: number
  sig: string
}

export interface ParseResult {
  success: true
  params: GameAuthParams
}

export interface ParseError {
  success: false
  error: string
  reason: 'missing_sig' | 'missing_params' | 'invalid_params'
}

export type ParseUrlParamsResult = ParseResult | ParseError

/**
 * Parse URL query parameters for Games API authentication.
 * Validates that all required parameters are present and correctly formatted.
 *
 * @param search - The URL search string (window.location.search)
 * @returns Parsed parameters or an error object
 */
export function parseGameUrlParams(search: string): ParseUrlParamsResult {
  const urlParams = new URLSearchParams(search)
  const chatIdStr = urlParams.get('chat_id')
  const userIdStr = urlParams.get('user_id')
  const matchId = urlParams.get('match_id') || ''
  const tsStr = urlParams.get('ts')
  const sig = urlParams.get('sig')

  // Validate required signature parameter
  if (!sig) {
    return {
      success: false,
      error: 'Missing signature parameter. Please launch the game from Telegram.',
      reason: 'missing_sig',
    }
  }

  // Validate other required parameters
  if (!chatIdStr || !userIdStr || !tsStr) {
    return {
      success: false,
      error: 'Missing required URL parameters',
      reason: 'missing_params',
    }
  }

  // Parse numeric parameters
  const chatId = parseInt(chatIdStr, 10)
  const userId = parseInt(userIdStr, 10)
  const ts = parseInt(tsStr, 10)

  if (isNaN(chatId) || isNaN(userId) || isNaN(ts)) {
    return {
      success: false,
      error: 'Invalid URL parameters',
      reason: 'invalid_params',
    }
  }

  return {
    success: true,
    params: {
      chatId,
      userId,
      matchId,
      ts,
      sig,
    },
  }
}
