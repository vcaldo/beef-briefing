/**
 * Battle-Related Utilities
 * Helper functions for battle animations, event formatting, and card operations
 */

import type { GameCard, BattleEvent } from '../types';

/**
 * Finds a card by ID across both teams
 * @param cardId - Card ID to search for
 * @param teamA - First team array
 * @param teamB - Second team array
 * @returns The card if found, null otherwise
 */
export function findCard(
  cardId: number,
  teamA: GameCard[],
  teamB: GameCard[]
): GameCard | null {
  return (
    teamA.find((c) => c.card_id === cardId) ||
    teamB.find((c) => c.card_id === cardId) ||
    null
  );
}

/**
 * Gets emoji icon for battle event type
 * @param type - Event type ('attack', 'death', 'victory', 'advance', etc.)
 * @returns Unicode emoji representing the event type
 */
export function getEventIcon(type: string): string {
  switch (type) {
    case 'attack':
      return '⚔️';
    case 'death':
      return '💀';
    case 'victory':
      return '👑';
    case 'advance':
      return '➡️';
    default:
      return '';
  }
}

/**
 * Formats a battle event for compact display in logs
 * Handles event-specific formatting with card names and damage values
 *
 * @param event - Battle event to format
 * @param yourTeam - Your team array
 * @param opponentTeam - Opponent's team array
 * @param isWinner - Whether the current player won the battle
 * @returns Formatted event string, or null to skip in compact view
 */
export function formatBattleEvent(
  event: BattleEvent,
  yourTeam: GameCard[],
  opponentTeam: GameCard[],
  isWinner: boolean
): string | null {
  const attacker = event.attacker_card_id
    ? findCard(event.attacker_card_id, yourTeam, opponentTeam)
    : null;
  const defender = event.defender_card_id
    ? findCard(event.defender_card_id, yourTeam, opponentTeam)
    : null;

  switch (event.type) {
    case 'attack':
      if (attacker && defender && event.damage) {
        return `${attacker.name} (${attacker.atk}) → ${defender.name} (${event.hp_before}) = ${event.damage} dmg`;
      }
      return event.message || 'Attack';

    case 'death':
      if (defender) {
        return `${defender.name} defeated`;
      }
      return event.message || 'Defeated';

    case 'victory':
      return isWinner ? 'Victory!' : 'Defeat';

    case 'advance':
      // Skip advance events for compact view
      return null;

    default:
      return event.message || null;
  }
}

/**
 * Determines if a card is the attacker in a given event
 * @param event - Battle event (can be null)
 * @param card - Card to check (can be null)
 * @param teamOwnerId - Team owner ID of the card
 * @returns True if card is attacking in this event
 */
export function isCardAttacker(
  event: BattleEvent | null | undefined,
  card: GameCard | null | undefined,
  teamOwnerId: number
): boolean {
  return !!(
    event &&
    card &&
    event.attacker_card_id === card.card_id &&
    event.attacker_team_owner_id === teamOwnerId
  );
}

/**
 * Determines if a card is the defender in a given event
 * @param event - Battle event (can be null)
 * @param card - Card to check (can be null)
 * @param teamOwnerId - Team owner ID of the card
 * @returns True if card is defending in this event
 */
export function isCardDefender(
  event: BattleEvent | null | undefined,
  card: GameCard | null | undefined,
  teamOwnerId: number
): boolean {
  return !!(
    event &&
    card &&
    event.defender_card_id === card.card_id &&
    event.defender_team_owner_id === teamOwnerId
  );
}

/**
 * Gets a player's display name from match participants
 * @param userId - User ID to find
 * @param participants - Array of match participants
 * @returns Player's display name, or "Unknown" if not found
 */
export function getPlayerName(userId: number, participants: any[]): string {
  const participant = participants.find((p) => p.user_id === userId);
  return participant?.display_name || 'Unknown';
}

/**
 * Truncates a name to fit UI constraints
 * Adds ellipsis if name exceeds max length
 *
 * @param name - Name to truncate
 * @param maxLength - Maximum characters (default: 10)
 * @returns Truncated name with ellipsis if needed
 */
export function truncateName(name: string, maxLength: number = 10): string {
  if (name.length > maxLength) {
    return name.slice(0, maxLength - 1) + '...';
  }
  return name;
}
