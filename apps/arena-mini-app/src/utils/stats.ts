/**
 * Statistics and Results Utilities
 * Centralized functions for calculating stats and formatting results
 */

/**
 * Calculates win rate percentage from wins and losses
 * Returns 0 if no games played
 *
 * @param wins - Number of wins
 * @param losses - Number of losses
 * @returns Win rate as integer percentage (0-100)
 */
export function calculateWinRate(wins: number, losses: number): number {
  const total = wins + losses;
  if (total === 0) return 0;
  return Math.round((wins / total) * 100);
}

/**
 * Gets CSS class for result badge styling
 * Used to style win/loss/draw indicators in UI
 *
 * @param result - Match result ('win', 'loss', or any other value treated as draw)
 * @returns CSS class name for styling
 */
export function getResultClass(result: string): string {
  switch (result) {
    case 'win':
      return 'result-win';
    case 'loss':
      return 'result-loss';
    default:
      return 'result-draw';
  }
}

/**
 * Gets display text for result badge
 * Single character representation of match outcome
 *
 * @param result - Match result ('win', 'loss', or any other value treated as draw)
 * @returns Single character result ('W', 'L', or 'D')
 */
export function getResultText(result: string): string {
  switch (result) {
    case 'win':
      return 'W';
    case 'loss':
      return 'L';
    default:
      return 'D';
  }
}
