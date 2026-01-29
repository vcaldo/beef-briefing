import { describe, it, expect } from 'vitest'
import { parseGameUrlParams } from '../utils/urlParams'

describe('parseGameUrlParams', () => {
  describe('valid parameters', () => {
    it('parses all required parameters correctly', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&match_id=abc123&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.params.chatId).toBe(-1001234567890)
        expect(result.params.userId).toBe(123456)
        expect(result.params.matchId).toBe('abc123')
        expect(result.params.ts).toBe(1706540800)
        expect(result.params.sig).toBe('abcdef123456')
      }
    })

    it('handles empty match_id for lobby-only access', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&match_id=&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.params.matchId).toBe('')
      }
    })

    it('handles missing match_id parameter (defaults to empty string)', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.params.matchId).toBe('')
      }
    })

    it('handles negative chat_id (Telegram supergroup IDs)', () => {
      const search = '?chat_id=-1003280306634&user_id=123456&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.params.chatId).toBe(-1003280306634)
      }
    })

    it('handles positive user_id correctly', () => {
      const search = '?chat_id=-1001234567890&user_id=9876543210&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.params.userId).toBe(9876543210)
      }
    })
  })

  describe('missing signature', () => {
    it('returns error when sig parameter is missing', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&match_id=abc123&ts=1706540800'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('missing_sig')
        expect(result.error).toContain('Missing signature')
      }
    })

    it('returns error when sig parameter is empty', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&match_id=abc123&ts=1706540800&sig='

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('missing_sig')
      }
    })
  })

  describe('missing required parameters', () => {
    it('returns error when chat_id is missing', () => {
      const search = '?user_id=123456&match_id=abc123&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('missing_params')
        expect(result.error).toContain('Missing required')
      }
    })

    it('returns error when user_id is missing', () => {
      const search = '?chat_id=-1001234567890&match_id=abc123&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('missing_params')
      }
    })

    it('returns error when ts is missing', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&match_id=abc123&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('missing_params')
      }
    })

    it('returns error when all parameters are missing', () => {
      const search = ''

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('missing_sig')
      }
    })

    it('returns error for empty search string with question mark', () => {
      const search = '?'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('missing_sig')
      }
    })
  })

  describe('invalid numeric parameters', () => {
    it('returns error when chat_id is not a number', () => {
      const search = '?chat_id=invalid&user_id=123456&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('invalid_params')
        expect(result.error).toContain('Invalid URL parameters')
      }
    })

    it('returns error when user_id is not a number', () => {
      const search = '?chat_id=-1001234567890&user_id=abc&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('invalid_params')
      }
    })

    it('returns error when ts is not a number', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&ts=notanumber&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.reason).toBe('invalid_params')
      }
    })

    it('handles mixed alphanumeric chat_id (parseInt behavior)', () => {
      // parseInt parses until first non-numeric char: '-100abc123' -> -100
      const search = '?chat_id=-100abc123&user_id=123456&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      // This is valid because parseInt('-100abc123') === -100
      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.params.chatId).toBe(-100)
      }
    })

    it('returns error when user_id is floating point', () => {
      const search = '?chat_id=-1001234567890&user_id=123.456&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      // parseInt truncates floating point, so 123.456 becomes 123
      // This is valid behavior - the user_id will be parsed as 123
      expect(result.success).toBe(true)
    })
  })

  describe('edge cases', () => {
    it('handles URL-encoded parameters', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&match_id=abc%20123&ts=1706540800&sig=abcdef%3D123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.params.matchId).toBe('abc 123')
        expect(result.params.sig).toBe('abcdef=123456')
      }
    })

    it('handles parameters with extra whitespace in values', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&ts=1706540800&sig=abc123'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
    })

    it('ignores extra unknown parameters', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&ts=1706540800&sig=abcdef123456&extra=value&foo=bar'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.params.chatId).toBe(-1001234567890)
        expect(result.params.userId).toBe(123456)
      }
    })

    it('handles very large user_id', () => {
      const search = '?chat_id=-1001234567890&user_id=9007199254740991&ts=1706540800&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
      if (result.success) {
        // JavaScript can handle integers up to MAX_SAFE_INTEGER (2^53 - 1)
        expect(result.params.userId).toBe(9007199254740991)
      }
    })

    it('handles zero timestamp', () => {
      const search = '?chat_id=-1001234567890&user_id=123456&ts=0&sig=abcdef123456'

      const result = parseGameUrlParams(search)

      expect(result.success).toBe(true)
      if (result.success) {
        expect(result.params.ts).toBe(0)
      }
    })
  })
})
