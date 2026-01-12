/**
 * useMatchExpansion Hook
 * Manages expanded/collapsed state for match items in lists
 */

import { useState, useCallback } from 'react';

/**
 * Generic hook for managing which match is expanded in a list
 * Supports optional callback when a match is expanded
 *
 * @param onExpand - Optional callback function called when a match is expanded
 * @returns Expansion state management functions and queries
 */
export function useMatchExpansion<T extends { match_id: string }>(
  onExpand?: (match: T) => Promise<void>
) {
  const [expandedMatchId, setExpandedMatchId] = useState<string | null>(null);

  const toggleExpansion = useCallback(
    async (match: T) => {
      // If already expanded, collapse it
      if (expandedMatchId === match.match_id) {
        setExpandedMatchId(null);
        return;
      }

      // Expand the match
      setExpandedMatchId(match.match_id);

      // Call the optional callback (e.g., to fetch battle data)
      if (onExpand) {
        try {
          await onExpand(match);
        } catch (err) {
          console.error('Error during match expansion callback:', err);
          // Keep the match expanded even if callback fails
        }
      }
    },
    [expandedMatchId, onExpand]
  );

  const isExpanded = useCallback(
    (match: T) => expandedMatchId === match.match_id,
    [expandedMatchId]
  );

  return {
    expandedMatchId,
    isExpanded,
    toggleExpansion,
  };
}
