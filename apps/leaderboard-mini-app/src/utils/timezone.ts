/**
 * Timezone utilities for the leaderboard mini-app.
 */

/**
 * Detect browser timezone using Intl API.
 * Falls back to UTC if detection fails.
 */
export function getBrowserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}
