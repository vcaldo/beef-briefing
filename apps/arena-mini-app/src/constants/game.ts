/**
 * Game Constants
 * Centralized configuration for all magic numbers in the arena mini app
 */

// ============================================================================
// POLLING INTERVALS (milliseconds)
// ============================================================================

/** Interval for polling active matches in lobby */
export const LOBBY_POLL_INTERVAL = 5000;

/** Interval for polling shop state during shop phase */
export const SHOP_POLL_INTERVAL = 3000;

/** Interval for polling battle results during battle phase */
export const BATTLE_POLL_INTERVAL = 2000;

// ============================================================================
// ANIMATION TIMING (milliseconds)
// ============================================================================

/** Minimum duration to display splash screen */
export const SPLASH_DURATION = 1500;

/** Delay before auto-playing battle animation */
export const AUTOPLAY_DELAY = 500;

/** Delay when skipping to end of battle */
export const SKIP_DELAY = 50;

/** Delay before playing hit sound effect */
export const HIT_SOUND_DELAY = 200;

/** Duration of haptic feedback vibration */
export const HAPTIC_DURATION = 50;

// ============================================================================
// BATTLE EVENT TIMING (milliseconds)
// Timing for different event types in battle animation
// ============================================================================

export const EVENT_TIMING = {
  attack: 1200,    // Card attacks opponent
  damage: 800,     // Damage is applied to card
  death: 1000,     // Card dies/faints
  advance: 600,    // Next card advances to battle
  victory: 2500,   // Battle victory/result shown
} as const;

// ============================================================================
// GAME COSTS (coins)
// Economic constants for shop mechanics
// ============================================================================

/** Cost to buy a card from the shop */
export const CARD_COST = 2;

/** Cost to reroll the available shop cards */
export const REROLL_COST = 1;

/** Cost to upgrade a card's stat (ATK, DEF, or HP) */
export const UPGRADE_COST = 2;

// ============================================================================
// TEAM CONFIGURATION
// ============================================================================

/** Maximum number of cards in a player's team */
export const MAX_TEAM_SIZE = 3;

// ============================================================================
// UI THRESHOLDS (percentages)
// Health bar color transitions
// ============================================================================

/** Health above this percentage shows as "high" (green) */
export const HEALTH_HIGH_THRESHOLD = 60;

/** Health between MEDIUM_THRESHOLD and HIGH_THRESHOLD shows as "medium" (yellow) */
export const HEALTH_MEDIUM_THRESHOLD = 30;

/** HP bar gradient transition at 50% */
export const HP_HIGH_PERCENT = 50;

/** HP bar gradient transition at 25% */
export const HP_MEDIUM_PERCENT = 25;

// ============================================================================
// CHARACTER LIMITS
// ============================================================================

/** Maximum characters to display for card names before truncation */
export const MAX_NAME_LENGTH = 10;

/** Seconds until timer shows warning state */
export const LOW_TIME_WARNING = 30;
