/**
 * Centralized configuration and constants for the Arena Mini App
 */

// Polling intervals (in milliseconds)
export const POLLING_INTERVALS = {
  LOBBY: 5000, // Poll active matches every 5 seconds
  SHOP: 3000, // Poll shop state every 3 seconds
  BATTLE: 2000, // Poll battle progress every 2 seconds
} as const;

// Shop economy costs
export const SHOP_COSTS = {
  CARD: 2, // Cost to purchase a card
  REROLL: 1, // Cost to reroll shop
  UPGRADE: 2, // Cost to upgrade a card's stats
} as const;

// Timer values (in milliseconds)
export const TIMERS = {
  SPLASH_SCREEN_MIN: 1500, // Minimum time to show splash screen
  AUTO_PLAY_DELAY: 500, // Delay before auto-playing battle
  STATE_UPDATE_DELAY: 50, // Delay for state propagation workaround
} as const;

// Battle event animation durations (in milliseconds)
export const EVENT_TIMING = {
  attack: 1200,
  damage: 800,
  death: 1000,
  advance: 600,
  victory: 2500,
} as const;

// Team configuration
export const TEAM = {
  MAX_SIZE: 3, // Maximum cards per team
  POSITIONS: {
    FRONT: 'front',
    MID: 'mid',
    BACK: 'back',
  },
  POSITION_LABELS: {
    0: 'Front',
    1: 'Mid',
    2: 'Back',
  },
} as const;

// Haptic feedback durations (in milliseconds)
export const HAPTIC_FEEDBACK = {
  DRAG_START: 50, // Vibration duration when starting to drag
} as const;

// Match statuses
export const MATCH_STATUS = {
  OPEN: 'open',
  SHOP_PHASE: 'shop_phase',
  BATTLE_PHASE: 'battle_phase',
  COMPLETED: 'completed',
  CANCELLED: 'cancelled',
} as const;

// App pages
export const APP_PAGES = {
  LOBBY: 'lobby',
  SHOP: 'shop',
  BATTLE: 'battle',
  LEADERBOARD: 'leaderboard',
  HISTORY: 'history',
  H2H: 'h2h',
  PROFILE: 'profile',
} as const;

// App states
export const APP_STATES = {
  LOADING: 'loading',
  AUTHENTICATED: 'authenticated',
  ERROR: 'error',
} as const;
