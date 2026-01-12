/**
 * Team Management Utilities
 * Helper functions for team ordering and positioning
 */

import type { GameCard } from '../types';

/**
 * Applies team order to get cards in battle position order
 * Reorders cards based on the provided team_order array of card IDs
 *
 * @param team - Current team array
 * @param teamOrder - Array of card IDs in desired battle order
 * @returns Reordered team array
 */
export function applyTeamOrder(team: GameCard[], teamOrder: number[]): GameCard[] {
  return teamOrder
    .map((cardId) => team.find((c) => c.card_id === cardId))
    .filter((card): card is GameCard => card !== undefined);
}

/**
 * Gets position label for team slot
 * Used to display Front/Mid/Back positioning in UI
 *
 * @param index - Position index (0, 1, 2)
 * @returns Position label string
 */
export function getPositionLabel(index: number): string {
  switch (index) {
    case 0:
      return 'Front';
    case 1:
      return 'Mid';
    case 2:
      return 'Back';
    default:
      return '';
  }
}
