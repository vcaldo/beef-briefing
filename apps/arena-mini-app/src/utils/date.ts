/**
 * Date and Time Utilities
 * Centralized date formatting functions used across multiple components
 */

/**
 * Formats a date string to localized date string
 * @param dateStr - ISO date string (e.g., "2023-12-25T10:30:00Z")
 * @returns Formatted date using browser locale (e.g., "12/25/2023")
 */
export function formatDate(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    return date.toLocaleDateString();
  } catch {
    return 'Invalid date';
  }
}

/**
 * Formats a date string with relative time if recent
 * Returns "Today" for same day, "Yesterday" for previous day,
 * "Xd ago" for recent days, or formatted date for older dates
 *
 * @param dateStr - ISO date string
 * @returns Relative date string (e.g., "Today", "Yesterday", "3d ago", "12/25/2023")
 */
export function formatRelativeDate(dateStr: string): string {
  try {
    const date = new Date(dateStr);

    // Get start of today and target date in local time
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    const targetDate = new Date(date);
    targetDate.setHours(0, 0, 0, 0);

    const diffMs = today.getTime() - targetDate.getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffDays === 0) return 'Today';
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7) return `${diffDays}d ago`;

    return formatDate(dateStr);
  } catch {
    return 'Invalid date';
  }
}

/**
 * Formats time remaining until a deadline
 * Shows countdown in MM:SS format, or "Expired" if deadline passed
 *
 * @param deadline - ISO date string or seconds remaining as number
 * @returns Formatted time string (e.g., "3:45", "0:15", "Expired")
 */
export function formatTimeRemaining(deadline: string | number): string {
  try {
    let secondsRemaining: number;

    if (typeof deadline === 'string') {
      const deadlineDate = new Date(deadline);
      const now = new Date();
      secondsRemaining = Math.max(
        0,
        Math.floor((deadlineDate.getTime() - now.getTime()) / 1000)
      );
    } else {
      secondsRemaining = Math.max(0, Math.floor(deadline));
    }

    if (secondsRemaining <= 0) {
      return 'Expired';
    }

    const minutes = Math.floor(secondsRemaining / 60);
    const seconds = secondsRemaining % 60;

    return `${minutes}:${seconds.toString().padStart(2, '0')}`;
  } catch {
    return 'Expired';
  }
}

/**
 * Formats seconds to MM:SS time format
 * Commonly used for countdowns and timers
 *
 * @param seconds - Number of seconds to format
 * @returns Formatted time string (e.g., "1:30", "0:05")
 */
export function formatTime(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  return `${minutes}:${secs.toString().padStart(2, '0')}`;
}
