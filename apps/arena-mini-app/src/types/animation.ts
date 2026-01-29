/**
 * @fileoverview Animation types for the battle replay system.
 *
 * This module contains types and constants for the phase-based animation
 * system that orchestrates card animations and battle log synchronization.
 *
 * Key concepts:
 * - CardAnimationState: Visual state of individual cards (idle → attacking → taking_damage)
 * - EventAnimationPhase: Current phase of event processing (idle → highlight → attack → damage → complete)
 * - ANIMATION_DURATIONS: Timing constants for each animation phase (scaled by playback speed)
 *
 * @module types/animation
 * @see {@link ../hooks/useBattleAnimation.ts} Animation hook that uses these types
 * @see {@link ../components/battle/BattlePage.tsx} Battle page that integrates animations
 */

// =============================================================================
// CARD ANIMATION STATE
// =============================================================================

/**
 * Visual animation state for a single card during battle replay.
 *
 * Cards transition through these states as events are processed:
 * - `idle`: Default state, no animation active
 * - `attacking`: Card is performing an attack (orange glow, scale up)
 * - `taking_damage`: Card is receiving damage (shake effect, red flash)
 *
 * @example
 * // Map animation state to CSS class
 * const animationClass = `compact-card-anim-${animationState}`;
 *
 * @example
 * // Check if card should show damage overlay
 * const showDamage = animationState === 'taking_damage';
 */
export type CardAnimationState = 'idle' | 'attacking' | 'taking_damage'

// =============================================================================
// EVENT ANIMATION PHASE
// =============================================================================

/**
 * Current phase of event animation processing.
 *
 * Each battle event goes through these phases in sequence:
 * - `idle`: Waiting for playback to start or between events
 * - `highlight`: Event starts, attacker/defender are highlighted
 * - `attack`: Attack animation plays on attacker card
 * - `damage`: Damage number appears, HP bar updates
 * - `complete`: Event fully processed, ready for next event
 *
 * The BattleLog component uses this to show in-progress indicators
 * and transition entry styling when complete.
 *
 * @example
 * // Check if current event is still animating
 * const isAnimating = currentPhase !== 'complete' && currentPhase !== 'idle';
 *
 * @example
 * // Render log entry styling based on phase
 * const entryClass = isAnimating ? 'animating' : 'played';
 */
export type EventAnimationPhase =
  | 'idle'
  | 'arena_transition'
  | 'highlight'
  | 'attack'
  | 'damage'
  | 'complete'
  | 'death_animation'
  | 'victory_celebration'

// =============================================================================
// ANIMATION DURATIONS
// =============================================================================

/**
 * Timing constants for animation phases in milliseconds.
 *
 * These durations define how long each animation phase lasts.
 * All values are scaled by the playback speed multiplier:
 * - 0.5x speed: durations × 2
 * - 1x speed: durations × 1
 * - 2x speed: durations × 0.5
 *
 * Animation sequence timeline (at 1x speed):
 * ```
 * 0ms     → highlight phase starts
 * 400ms   → attack animation completes, damage phase starts
 * 700ms   → HP transition completes, event complete
 * 900ms   → gap before next event
 * ```
 *
 * @example
 * // Scale durations by playback speed
 * const scaledDuration = ANIMATION_DURATIONS.attack / playbackSpeed;
 *
 * @example
 * // Use in setTimeout for phase transitions
 * setTimeout(() => {
 *   setPhase('damage');
 * }, ANIMATION_DURATIONS.attack / speed);
 */
export const ANIMATION_DURATIONS = {
  /** Attack animation duration (orange glow, scale up) */
  attack: 400,

  /** Damage display duration (shake effect, damage number visible) */
  damage: 300,

  /** HP bar transition duration (CSS transition for smooth bar movement) */
  hpTransition: 300,

  /** Gap between events (pause before next event starts) */
  eventGap: 200,

  /** Delay before starting the first event on play */
  playStart: 100,

  /** Arena card enter animation duration (scale up from deck to arena) */
  arenaEnter: 400,

  /** Arena card exit animation duration (scale down from arena back to deck) */
  arenaExit: 400,

  /** Death effect animation duration (9 frames × 70ms per frame) */
  deathEffect: 630,

  /** Victory return animation duration (winner card exits arena) */
  victoryReturn: 400,

  /** Victory celebration duration (deck shake + particles) */
  victoryCelebration: 2000,
} as const

/**
 * Type for animation duration keys.
 *
 * Used when dynamically accessing duration values.
 */
export type AnimationDurationKey = keyof typeof ANIMATION_DURATIONS
