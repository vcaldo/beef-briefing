/**
 * @fileoverview Battle animation state machine hook.
 *
 * This hook orchestrates the phase-based animation system for battle replays.
 * It separates data state (HP, is_alive) from visual animation state, ensuring
 * proper sequencing of attack and damage animations.
 *
 * Key concepts:
 * - Data state: Updated during damage phase (HP updates, is_alive for game logic)
 * - Animation state: Visual representation (attacking, taking_damage)
 * - Phase progression: Each event goes through highlight → attack → damage → complete
 *
 * @module hooks/useBattleAnimation
 * @see {@link ../types/animation.ts} Animation type definitions
 * @see {@link ../components/battle/BattlePage.tsx} Component that uses this hook
 */

import { useState, useCallback, useRef, useEffect } from 'react'
import type {
  BattleResult,
  BattleEvent,
  CardSnapshot,
  CardAnimationState,
  EventAnimationPhase,
} from '../types'
import type { SoundId } from './useSound'
import type { BattleEffectType, EffectPosition } from '../components/battle/BattleEffect'
import { ANIMATION_DURATIONS } from '../types'

// =============================================================================
// TYPES
// =============================================================================

/**
 * Active battle effect state - used to render BattleEffect components.
 */
export interface ActiveBattleEffect {
  /** Type of effect to display */
  type: BattleEffectType
  /** Position to display the effect (center coordinates) */
  position: EffectPosition
  /** Card key the effect is associated with (for tracking) */
  cardKey: string
}

/**
 * Arena card reference - identifies a card currently in the arena.
 */
export interface ArenaCard {
  /** The card's unique identifier */
  cardId: number
  /** The user ID of the team owner (player_a_id or player_b_id) */
  teamOwnerId: number
}

/**
 * Arena transition state - tracks the current state of arena card transitions.
 */
export type ArenaTransitionState = 'idle' | 'entering' | 'exiting'

/**
 * Return value from useBattleAnimation hook.
 */
export interface UseBattleAnimationReturn {
  /** Card data states (hp, is_alive, etc.) keyed by composite key */
  cardStates: Map<string, CardSnapshot>
  /** Visual animation states keyed by composite key */
  animationStates: Map<string, CardAnimationState>
  /** Index of the current event being processed (-1 = before first event) */
  currentEventIndex: number
  /** Current phase within the event animation sequence */
  currentPhase: EventAnimationPhase
  /** Whether playback is currently active */
  isPlaying: boolean
  /** Whether all events have been processed */
  isComplete: boolean
  /** Damage number to display (shown during damage phase) */
  currentDamage: number | null
  /** Card key that should show damage number */
  damageTargetKey: string | null
  /** Currently active battle effects (attack, damage, death, spark) - supports multiple simultaneous effects */
  activeEffects: ActiveBattleEffect[]
  /** Callback to call when an effect animation completes (pass cardKey to identify which effect) */
  onEffectComplete: (cardKey: string) => void
  /** Card currently in arena for Team A (player A's front card) */
  arenaCardA: ArenaCard | null
  /** Card currently in arena for Team B (player B's front card) */
  arenaCardB: ArenaCard | null
  /** Current state of arena card transitions */
  arenaTransitionState: ArenaTransitionState
  /** Which specific cards are currently transitioning (for per-card animation classes) */
  transitioningCards: { entering: Set<string>; exiting: Set<string> }
  /** Get the front (lowest position) alive card for each team. Optionally exclude cards that are pending death. */
  getFrontCards: (excludeDeaths?: Set<string>) => { cardA: ArenaCard | null; cardB: ArenaCard | null }
  /** Start or resume playback */
  play: () => void
  /** Pause playback */
  pause: () => void
  /** Reset to initial state */
  reset: () => void
  /** Skip to final state immediately */
  skipToEnd: () => void
}

/**
 * Options for the useBattleAnimation hook.
 */
export interface UseBattleAnimationOptions {
  /** Playback speed multiplier (0.5 = half speed, 2 = double speed) */
  playbackSpeed: number
  /** Player A's user ID (for composite keys) */
  playerAId: number
  /** Player B's user ID (for composite keys) */
  playerBId: number
  /** Optional callback to play sound effects during battle animation */
  onPlaySound?: (soundId: SoundId) => void
  /**
   * Optional callback to get card position for effect rendering.
   * Returns the center position {x, y} of the card element on screen.
   * If not provided, effects will not be displayed.
   */
  getCardPosition?: (cardKey: string) => EffectPosition | null
}

// =============================================================================
// HELPERS
// =============================================================================

/**
 * Generate a composite key for card state lookups.
 * Uses teamOwnerId + cardId to distinguish same card on different teams.
 *
 * This is needed because the same card_id can appear on both teams (e.g., when
 * both players purchase the same user's card). Using only card_id as the Map key
 * would cause both instances to share state, leading to animation bugs where
 * both cards animate together.
 *
 * @param teamOwnerId - The user ID of the team owner (player_a_id or player_b_id)
 * @param cardId - The card's unique identifier
 * @returns A composite key string in the format "teamOwnerId_cardId"
 */
export const getCardKey = (teamOwnerId: number, cardId: number): string =>
  `${teamOwnerId}_${cardId}`

/**
 * Initialize card states from battle data.
 */
const initializeCardStates = (
  battleData: BattleResult,
  playerAId: number,
  playerBId: number
): Map<string, CardSnapshot> => {
  const states = new Map<string, CardSnapshot>()

  // Team A cards
  battleData.team_a_final?.cards?.forEach((card, index) => {
    const key = getCardKey(playerAId, card.card_id)
    states.set(key, {
      card_id: card.card_id,
      user_id: card.user_id,
      name: card.name,
      hp: card.max_hp,
      max_hp: card.max_hp,
      atk: card.atk,
      position: index,
      is_alive: true,
      is_attacking: false,
      is_defending: false,
    })
  })

  // Team B cards
  battleData.team_b_final?.cards?.forEach((card, index) => {
    const key = getCardKey(playerBId, card.card_id)
    states.set(key, {
      card_id: card.card_id,
      user_id: card.user_id,
      name: card.name,
      hp: card.max_hp,
      max_hp: card.max_hp,
      atk: card.atk,
      position: index,
      is_alive: true,
      is_attacking: false,
      is_defending: false,
    })
  })

  return states
}

/**
 * Initialize animation states to idle for all cards.
 */
const initializeAnimationStates = (
  cardStates: Map<string, CardSnapshot>
): Map<string, CardAnimationState> => {
  const states = new Map<string, CardAnimationState>()
  cardStates.forEach((_, key) => {
    states.set(key, 'idle')
  })
  return states
}

// =============================================================================
// HOOK
// =============================================================================

/**
 * Battle animation state machine hook.
 *
 * Orchestrates card animations during battle replay by processing events
 * through a phase-based state machine. Separates data state from visual
 * animation state to ensure proper sequencing.
 *
 * Animation sequence per event:
 * 1. highlight (0ms): Attacker card gets 'attacking' state
 * 2. attack (400ms): Attack animation plays
 * 3. damage (700ms): HP bar updates, damage number shown
 * 4. complete (900ms): Event fully processed, gap before next
 *
 * @param battleData - Battle result data containing events and teams
 * @param options - Configuration options
 * @returns Animation state and control functions
 *
 * @example
 * const {
 *   cardStates,
 *   animationStates,
 *   currentPhase,
 *   play,
 *   pause,
 *   reset,
 * } = useBattleAnimation(battleData, {
 *   playbackSpeed: 1,
 *   playerAId: battleData.player_a_id,
 *   playerBId: battleData.player_b_id,
 * })
 */
export function useBattleAnimation(
  battleData: BattleResult | null,
  options: UseBattleAnimationOptions
): UseBattleAnimationReturn {
  const { playbackSpeed, playerAId, playerBId, onPlaySound, getCardPosition } = options

  // Core state
  const [cardStates, setCardStates] = useState<Map<string, CardSnapshot>>(
    () => new Map()
  )
  const [animationStates, setAnimationStates] = useState<
    Map<string, CardAnimationState>
  >(() => new Map())
  const [currentEventIndex, setCurrentEventIndex] = useState(-1)
  const [currentPhase, setCurrentPhase] = useState<EventAnimationPhase>('idle')
  const [isPlaying, setIsPlaying] = useState(false)
  const [isComplete, setIsComplete] = useState(false)

  // Damage display state
  const [currentDamage, setCurrentDamage] = useState<number | null>(null)
  const [damageTargetKey, setDamageTargetKey] = useState<string | null>(null)

  // Battle effect state (attack, damage, death animations) - supports multiple simultaneous effects
  const [activeEffects, setActiveEffects] = useState<ActiveBattleEffect[]>([])

  // Arena state - tracks which cards are currently in the central arena
  const [arenaCardA, setArenaCardA] = useState<ArenaCard | null>(null)
  const [arenaCardB, setArenaCardB] = useState<ArenaCard | null>(null)
  const [arenaTransitionState, setArenaTransitionState] = useState<ArenaTransitionState>('idle')
  // Track which specific cards are currently transitioning (for per-card animations)
  const [transitioningCards, setTransitioningCards] = useState<{
    entering: Set<string>
    exiting: Set<string>
  }>({ entering: new Set(), exiting: new Set() })

  // Deferred death state - cards that should die after the round completes
  // This prevents cards from greying out before their attack animation plays
  // Note: We only use the setter with callbacks, so we don't destructure the state value
  const [, setPendingDeaths] = useState<Set<string>>(() => new Set())
  // Also track pending deaths in a ref for synchronous access in arena transition checks
  const pendingDeathsRef = useRef<Set<string>>(new Set())
  const currentRoundRef = useRef<number>(0)

  // Refs for cleanup and state access in callbacks
  const phaseTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isPlayingRef = useRef(isPlaying)
  const currentEventIndexRef = useRef(currentEventIndex)
  const advanceToNextEventRef = useRef<(() => void) | null>(null)
  // Track if an arena transition is currently in progress
  const isArenaTransitionInProgressRef = useRef(false)

  // Keep currentEventIndex ref in sync
  useEffect(() => {
    currentEventIndexRef.current = currentEventIndex
  }, [currentEventIndex])

  // Initialize states when battle data changes
  useEffect(() => {
    if (battleData) {
      const initialCardStates = initializeCardStates(
        battleData,
        playerAId,
        playerBId
      )
      setCardStates(initialCardStates)
      setAnimationStates(initializeAnimationStates(initialCardStates))
      setCurrentEventIndex(-1)
      setCurrentPhase('idle')
      setIsComplete(false)
      setCurrentDamage(null)
      setDamageTargetKey(null)
      setActiveEffects([])
      setPendingDeaths(new Set())
      pendingDeathsRef.current = new Set()
      currentRoundRef.current = 0
      // Reset arena state
      setArenaCardA(null)
      setArenaCardB(null)
      setArenaTransitionState('idle')
      isArenaTransitionInProgressRef.current = false
    }
  }, [battleData, playerAId, playerBId])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (phaseTimeoutRef.current) {
        clearTimeout(phaseTimeoutRef.current)
      }
    }
  }, [])

  /**
   * Get scaled duration based on playback speed.
   * Higher speed = shorter duration.
   */
  const getScaledDuration = useCallback(
    (baseDuration: number): number => {
      return baseDuration / playbackSpeed
    },
    [playbackSpeed]
  )

  /**
   * Clear all card animation states to idle.
   */
  const clearAnimationStates = useCallback(() => {
    setAnimationStates((prev: Map<string, CardAnimationState>) => {
      const next = new Map(prev)
      next.forEach((_, key) => {
        next.set(key, 'idle')
      })
      return next
    })
    setCurrentDamage(null)
    setDamageTargetKey(null)
    setActiveEffects([])
  }, [])

  /**
   * Callback to signal that an effect animation has completed.
   * Removes the specific effect from the active effects array.
   */
  const onEffectComplete = useCallback((cardKey: string) => {
    setActiveEffects((prev: ActiveBattleEffect[]) =>
      prev.filter((e: ActiveBattleEffect) => e.cardKey !== cardKey)
    )
  }, [])

  /**
   * Process the current phase and schedule the next phase transition.
   */
  const processPhase = useCallback(
    (event: BattleEvent, phase: EventAnimationPhase) => {
      if (!isPlayingRef.current) return

      switch (phase) {
        case 'highlight': {
          // Start of event: set attacker to attacking state
          if (event.attacker_card_id && event.attacker_team_owner_id) {
            const attackerKey = getCardKey(
              event.attacker_team_owner_id,
              event.attacker_card_id
            )
            setAnimationStates((prev: Map<string, CardAnimationState>) => {
              const next = new Map(prev)
              next.set(attackerKey, 'attacking')
              return next
            })

            // Play attack sound when attacker starts attacking
            onPlaySound?.('arena_battle_attack')

            // Trigger 'attack' visual effect on the attacker card
            if (getCardPosition) {
              const position = getCardPosition(attackerKey)
              if (position) {
                setActiveEffects((prev: ActiveBattleEffect[]) => [
                  ...prev.filter((e: ActiveBattleEffect) => e.cardKey !== attackerKey),
                  { type: 'attack', position, cardKey: attackerKey },
                ])
              }
            }
          }

          // Schedule attack phase
          phaseTimeoutRef.current = setTimeout(() => {
            setCurrentPhase('attack')
            processPhase(event, 'attack')
          }, getScaledDuration(ANIMATION_DURATIONS.attack))
          break
        }

        case 'attack': {
          // Attack animation complete, show damage
          if (
            event.defender_card_id &&
            event.defender_team_owner_id &&
            event.damage
          ) {
            const defenderKey = getCardKey(
              event.defender_team_owner_id,
              event.defender_card_id
            )

            // Set defender to taking damage
            setAnimationStates((prev: Map<string, CardAnimationState>) => {
              const next = new Map(prev)
              next.set(defenderKey, 'taking_damage')
              return next
            })

            // Show damage number
            setCurrentDamage(event.damage)
            setDamageTargetKey(defenderKey)

            // Play damage sound when defender takes damage
            onPlaySound?.('arena_battle_damage')

            // Trigger 'damage' visual effect on the defender card
            if (getCardPosition) {
              const position = getCardPosition(defenderKey)
              if (position) {
                setActiveEffects((prev: ActiveBattleEffect[]) => [
                  ...prev.filter((e: ActiveBattleEffect) => e.cardKey !== defenderKey),
                  { type: 'damage', position, cardKey: defenderKey },
                ])
              }
            }
          }

          // Schedule damage phase
          phaseTimeoutRef.current = setTimeout(() => {
            setCurrentPhase('damage')
            processPhase(event, 'damage')
          }, getScaledDuration(ANIMATION_DURATIONS.damage))
          break
        }

        case 'damage': {
          // Update HP in card states (triggers HP bar animation via CSS transition)
          if (event.defender_card_id && event.defender_team_owner_id) {
            const defenderKey = getCardKey(
              event.defender_team_owner_id,
              event.defender_card_id
            )

            setCardStates((prev: Map<string, CardSnapshot>) => {
              const next = new Map(prev)
              const defender = next.get(defenderKey)

              // Calculate hp_after with fallback: prefer explicit value, else compute from hp_before - damage
              const hpAfter =
                event.hp_after ??
                (event.hp_before !== undefined && event.damage !== undefined
                  ? event.hp_before - event.damage
                  : undefined)

              if (defender && hpAfter !== undefined) {
                // Clamp HP to 0 minimum - backend may send negative values for overkill damage
                const clampedHp = Math.max(0, hpAfter)

                // Check if HP crossed the critical threshold (25%)
                // Play critical_hp sound when HP drops below 25% for the first time
                const criticalThreshold = defender.max_hp * 0.25
                const wasAboveCritical = defender.hp > criticalThreshold
                const isNowCritical = clampedHp <= criticalThreshold && clampedHp > 0
                if (wasAboveCritical && isNowCritical) {
                  onPlaySound?.('arena_critical_hp')
                }

                // Only update HP here - is_alive will be set in 'complete' phase
                // This ensures the card doesn't grey out while attack animation is still playing
                next.set(defenderKey, {
                  ...defender,
                  hp: clampedHp,
                  // Keep is_alive unchanged for now - will be updated in complete phase
                })
              }
              return next
            })

            // Clear damage number
            setCurrentDamage(null)
            setDamageTargetKey(null)
          }

          // Go to complete after HP transition
          phaseTimeoutRef.current = setTimeout(() => {
            setCurrentPhase('complete')
            processPhase(event, 'complete')
          }, getScaledDuration(ANIMATION_DURATIONS.hpTransition))
          break
        }

        case 'complete': {
          // Determine if defender died using event data (not state)
          // This avoids async state timing issues with setCardStates
          const hpAfter =
            event.hp_after ??
            (event.hp_before !== undefined && event.damage !== undefined
              ? event.hp_before - event.damage
              : undefined)
          const defenderDied = hpAfter !== undefined && hpAfter <= 0

          // Queue death for end of round instead of applying immediately.
          // This ensures cards don't grey out before their attack animation plays
          // in simultaneous combat (both cards attack in same round).
          if (
            defenderDied &&
            event.defender_card_id &&
            event.defender_team_owner_id
          ) {
            const defenderKey = getCardKey(
              event.defender_team_owner_id,
              event.defender_card_id
            )
            setPendingDeaths((prev: Set<string>) => new Set(prev).add(defenderKey))
            // Also update ref for synchronous access in arena transition checks
            pendingDeathsRef.current = new Set(pendingDeathsRef.current).add(defenderKey)
          }

          // Clear animation states back to idle
          clearAnimationStates()

          // Gap before next event
          // Use ref to always call the latest advanceToNextEvent (avoids stale closure)
          phaseTimeoutRef.current = setTimeout(() => {
            advanceToNextEventRef.current?.()
          }, getScaledDuration(ANIMATION_DURATIONS.eventGap))
          break
        }
      }
    },
    [getScaledDuration, clearAnimationStates, onPlaySound, getCardPosition]
  )

  /**
   * Apply all pending deaths - set is_alive to false, play death sounds, and show death effects.
   * Called at round boundaries (when transitioning to non-attack events).
   */
  const applyPendingDeaths = useCallback(
    (deaths: Set<string>) => {
      if (deaths.size === 0) return

      // Apply all deaths to card states
      setCardStates((prev: Map<string, CardSnapshot>) => {
        const next = new Map(prev)
        deaths.forEach((cardKey) => {
          const card = next.get(cardKey)
          if (card) {
            next.set(cardKey, { ...card, is_alive: false })
          }
        })
        return next
      })

      // Play death sound for each death
      deaths.forEach(() => onPlaySound?.('arena_battle_death'))

      // Trigger 'death' visual effect for ALL deaths simultaneously
      if (getCardPosition) {
        const deathEffects: ActiveBattleEffect[] = []
        deaths.forEach((cardKey) => {
          const position = getCardPosition(cardKey)
          if (position) {
            deathEffects.push({ type: 'death', position, cardKey })
          }
        })
        if (deathEffects.length > 0) {
          setActiveEffects((prev: ActiveBattleEffect[]) => [...prev, ...deathEffects])
        }
      }

      // Clear pending deaths (both state and ref)
      setPendingDeaths(new Set())
      pendingDeathsRef.current = new Set()
    },
    [onPlaySound, getCardPosition]
  )

  /**
   * Get the front (lowest position) alive card for each team.
   * Used to determine which cards should be displayed in the central arena.
   *
   * @param excludeDeaths - Optional set of card keys to treat as dead (for pending deaths that haven't been applied yet)
   * @returns Object with cardA and cardB - the front cards for each team, or null if no alive cards remain
   */
  const getFrontCards = useCallback((excludeDeaths?: Set<string>): { cardA: ArenaCard | null; cardB: ArenaCard | null } => {
    let frontA: ArenaCard | null = null
    let frontB: ArenaCard | null = null
    let minPosA = Infinity
    let minPosB = Infinity

    cardStates.forEach((state: CardSnapshot, key: string) => {
      // Skip dead cards
      if (!state.is_alive) return

      // Skip cards in the excludeDeaths set (pending deaths not yet applied to state)
      if (excludeDeaths && excludeDeaths.has(key)) return

      // Parse composite key to determine team ownership
      const parts = key.split('_')
      const teamOwnerId = parseInt(parts[0], 10)
      const cardId = state.card_id

      if (teamOwnerId === playerAId) {
        // Team A card - check if it has a lower position than current front
        if (state.position < minPosA) {
          minPosA = state.position
          frontA = { cardId, teamOwnerId }
        }
      } else if (teamOwnerId === playerBId) {
        // Team B card - check if it has a lower position than current front
        if (state.position < minPosB) {
          minPosB = state.position
          frontB = { cardId, teamOwnerId }
        }
      }
    })

    return { cardA: frontA, cardB: frontB }
  }, [cardStates, playerAId, playerBId])

  /**
   * Check if arena cards need to change based on current front cards.
   * Compares current arena cards with what the front cards should be.
   *
   * @param excludeDeaths - Optional set of card keys to treat as dead (for pending deaths not yet applied to state)
   * @returns Object with needsTransition flag and the new front cards
   */
  const checkArenaTransitionNeeded = useCallback((excludeDeaths?: Set<string>): {
    needsTransition: boolean
    newFrontA: ArenaCard | null
    newFrontB: ArenaCard | null
  } => {
    const { cardA: newFrontA, cardB: newFrontB } = getFrontCards(excludeDeaths)

    // Check if arena card A differs from front card A
    const arenaAChanged =
      (arenaCardA === null && newFrontA !== null) ||
      (arenaCardA !== null && newFrontA === null) ||
      (arenaCardA !== null && newFrontA !== null &&
        (arenaCardA.cardId !== newFrontA.cardId || arenaCardA.teamOwnerId !== newFrontA.teamOwnerId))

    // Check if arena card B differs from front card B
    const arenaBChanged =
      (arenaCardB === null && newFrontB !== null) ||
      (arenaCardB !== null && newFrontB === null) ||
      (arenaCardB !== null && newFrontB !== null &&
        (arenaCardB.cardId !== newFrontB.cardId || arenaCardB.teamOwnerId !== newFrontB.teamOwnerId))

    return {
      needsTransition: arenaAChanged || arenaBChanged,
      newFrontA,
      newFrontB,
    }
  }, [getFrontCards, arenaCardA, arenaCardB])

  /**
   * Perform arena card transition sequence.
   * Sequence: exit animation (if cards in arena) → update arena cards → enter animation → callback
   *
   * Only animates cards that actually changed (single card death = single card animates).
   *
   * @param newFrontA - New front card for Team A (or null if none)
   * @param newFrontB - New front card for Team B (or null if none)
   * @param onComplete - Callback to run after transition completes
   */
  const performArenaTransition = useCallback(
    (
      newFrontA: ArenaCard | null,
      newFrontB: ArenaCard | null,
      onComplete: () => void
    ) => {
      // Mark that we're starting a transition
      isArenaTransitionInProgressRef.current = true

      // Determine which cards are actually changing
      const aChanged =
        (arenaCardA === null && newFrontA !== null) ||
        (arenaCardA !== null && newFrontA === null) ||
        (arenaCardA !== null && newFrontA !== null &&
          (arenaCardA.cardId !== newFrontA.cardId || arenaCardA.teamOwnerId !== newFrontA.teamOwnerId))
      const bChanged =
        (arenaCardB === null && newFrontB !== null) ||
        (arenaCardB !== null && newFrontB === null) ||
        (arenaCardB !== null && newFrontB !== null &&
          (arenaCardB.cardId !== newFrontB.cardId || arenaCardB.teamOwnerId !== newFrontB.teamOwnerId))

      // Build exit set with only cards that are leaving
      const exitingKeys = new Set<string>()
      if (aChanged && arenaCardA) {
        exitingKeys.add(getCardKey(arenaCardA.teamOwnerId, arenaCardA.cardId))
      }
      if (bChanged && arenaCardB) {
        exitingKeys.add(getCardKey(arenaCardB.teamOwnerId, arenaCardB.cardId))
      }

      // Build enter set with only new cards coming in
      const enteringKeys = new Set<string>()
      if (aChanged && newFrontA) {
        enteringKeys.add(getCardKey(newFrontA.teamOwnerId, newFrontA.cardId))
      }
      if (bChanged && newFrontB) {
        enteringKeys.add(getCardKey(newFrontB.teamOwnerId, newFrontB.cardId))
      }

      // Check if we have cards to exit
      const hasCardsToExit = exitingKeys.size > 0

      if (hasCardsToExit) {
        // Phase 1: Exit animation for cards that are leaving
        setArenaTransitionState('exiting')
        setTransitioningCards({ entering: new Set(), exiting: exitingKeys })
        setCurrentPhase('arena_transition')

        phaseTimeoutRef.current = setTimeout(() => {
          // Phase 2: Update arena cards after exit animation
          setArenaCardA(newFrontA)
          setArenaCardB(newFrontB)

          // Check if we have new cards to enter
          const hasCardsToEnter = enteringKeys.size > 0

          if (hasCardsToEnter) {
            // Phase 3: Enter animation for new cards
            setArenaTransitionState('entering')
            setTransitioningCards({ entering: enteringKeys, exiting: new Set() })

            phaseTimeoutRef.current = setTimeout(() => {
              // Transition complete
              setArenaTransitionState('idle')
              setTransitioningCards({ entering: new Set(), exiting: new Set() })
              isArenaTransitionInProgressRef.current = false
              onComplete()
            }, getScaledDuration(ANIMATION_DURATIONS.arenaEnter))
          } else {
            // No new cards, transition complete
            setArenaTransitionState('idle')
            setTransitioningCards({ entering: new Set(), exiting: new Set() })
            isArenaTransitionInProgressRef.current = false
            onComplete()
          }
        }, getScaledDuration(ANIMATION_DURATIONS.arenaExit))
      } else {
        // No cards to exit - skip exit, go directly to enter
        setArenaCardA(newFrontA)
        setArenaCardB(newFrontB)

        const hasCardsToEnter = enteringKeys.size > 0

        if (hasCardsToEnter) {
          // Enter animation for new cards
          setArenaTransitionState('entering')
          setTransitioningCards({ entering: enteringKeys, exiting: new Set() })
          setCurrentPhase('arena_transition')

          phaseTimeoutRef.current = setTimeout(() => {
            setArenaTransitionState('idle')
            setTransitioningCards({ entering: new Set(), exiting: new Set() })
            isArenaTransitionInProgressRef.current = false
            onComplete()
          }, getScaledDuration(ANIMATION_DURATIONS.arenaEnter))
        } else {
          // No cards at all, just complete
          setArenaTransitionState('idle')
          setTransitioningCards({ entering: new Set(), exiting: new Set() })
          isArenaTransitionInProgressRef.current = false
          onComplete()
        }
      }
    },
    [arenaCardA, arenaCardB, getScaledDuration]
  )

  /**
   * Handle completion of death animation phase.
   * Called after death effects have finished playing, before arena transition.
   *
   * @param nextEvent - The next event to process after transition
   * @param nextIndex - Index of the next event
   * @param deathsToExclude - Card keys that died (for arena transition check)
   */
  const handleDeathAnimationComplete = useCallback((
    nextEvent: BattleEvent,
    nextIndex: number,
    deathsToExclude: Set<string>
  ) => {
    const { needsTransition, newFrontA, newFrontB } = checkArenaTransitionNeeded(deathsToExclude)

    if (nextEvent.type !== 'attack') {
      // Non-attack event - skip animation, just advance
      setCurrentEventIndex(nextIndex)
      setCurrentPhase('idle')
      phaseTimeoutRef.current = setTimeout(() => {
        advanceToNextEventRef.current?.()
      }, getScaledDuration(ANIMATION_DURATIONS.eventGap))
      return
    }

    // Attack event - handle arena transition if needed
    if (needsTransition) {
      setCurrentEventIndex(nextIndex)
      performArenaTransition(newFrontA, newFrontB, () => {
        if (isPlayingRef.current) {
          setCurrentPhase('highlight')
          processPhase(nextEvent, 'highlight')
        }
      })
    } else {
      setCurrentEventIndex(nextIndex)
      setCurrentPhase('highlight')
      processPhase(nextEvent, 'highlight')
    }
  }, [checkArenaTransitionNeeded, performArenaTransition, processPhase, getScaledDuration])

  /**
   * Complete the battle after all events have been processed.
   * Handles final arena transition for dead cards exiting before showing victory.
   */
  const completeBattle = useCallback(() => {
    // Capture pending deaths to pass to arena transition check
    const deathsToExclude = new Set(pendingDeathsRef.current)
    const hasDeaths = deathsToExclude.size > 0

    // Apply any remaining pending deaths
    setPendingDeaths((currentPendingDeaths: Set<string>) => {
      if (currentPendingDeaths.size > 0) {
        applyPendingDeaths(currentPendingDeaths)
      }
      return new Set()
    })

    // Helper to finalize the battle after any animations complete
    const finalizeBattle = () => {
      // Check if final arena transition is needed (dead cards should exit)
      const { needsTransition, newFrontA, newFrontB } = checkArenaTransitionNeeded(deathsToExclude)

      if (needsTransition) {
        // Perform final arena transition before marking complete
        performArenaTransition(newFrontA, newFrontB, () => {
          // Now we can mark battle as complete
          setIsPlaying(false)
          isPlayingRef.current = false
          setIsComplete(true)
          setCurrentPhase('idle')
        })
      } else {
        // No transition needed, mark complete immediately
        setIsPlaying(false)
        isPlayingRef.current = false
        setIsComplete(true)
        setCurrentPhase('idle')
      }
    }

    // If there were deaths, wait for death animation before final transition
    if (hasDeaths) {
      setCurrentPhase('death_animation')
      phaseTimeoutRef.current = setTimeout(finalizeBattle,
        getScaledDuration(ANIMATION_DURATIONS.deathEffect))
    } else {
      finalizeBattle()
    }
  }, [applyPendingDeaths, checkArenaTransitionNeeded, performArenaTransition, getScaledDuration])

  /**
   * Advance to the next event in the sequence.
   */
  const advanceToNextEvent = useCallback(() => {
    if (!battleData || !isPlayingRef.current) return

    const nextIndex = currentEventIndexRef.current + 1

    if (nextIndex >= battleData.events.length) {
      // All events processed - handle final transition and completion
      completeBattle()
      return
    }

    const nextEvent = battleData.events[nextIndex]
    const prevRound = currentRoundRef.current
    const isNonAttackEvent = nextEvent.type !== 'attack'
    const isRoundTransition = nextEvent.round !== prevRound && prevRound !== 0

    // Capture pending deaths BEFORE applying them, so we can pass them to arena transition check.
    // This handles simultaneous card deaths: both cards die in the same round, and we need to
    // transition to the next front cards for both teams atomically.
    const deathsToExclude = new Set(pendingDeathsRef.current)
    const hasDeaths = deathsToExclude.size > 0

    // Update current round tracking
    currentRoundRef.current = nextEvent.round

    // Check if arena transition is needed (do this early to determine if death animation is required)
    const { needsTransition, newFrontA, newFrontB } = checkArenaTransitionNeeded(deathsToExclude)

    // Determine if any pending deaths are currently in the arena and need to exit.
    // This ensures death animation plays WHILE the card is visible in the arena,
    // not after it has already started exiting.
    const arenaTransitionInvolvesDeath = needsTransition && (
      (arenaCardA && deathsToExclude.has(getCardKey(arenaCardA.teamOwnerId, arenaCardA.cardId))) ||
      (arenaCardB && deathsToExclude.has(getCardKey(arenaCardB.teamOwnerId, arenaCardB.cardId)))
    )

    // Check if the dead card is the attacker in the next event (simultaneous combat).
    // If so, let them complete their attack first before applying death animation.
    // This ensures the dead card's "last attack" plays while still in the arena.
    const deadCardIsAttacker = nextEvent.type === 'attack' &&
      nextEvent.attacker_card_id && nextEvent.attacker_team_owner_id &&
      deathsToExclude.has(getCardKey(nextEvent.attacker_team_owner_id, nextEvent.attacker_card_id))

    // If there are deaths that are in the arena, play death animation before transitioning.
    // BUT: if the dead card is about to attack, let them attack first (simultaneous combat).
    if (hasDeaths && arenaTransitionInvolvesDeath && !deadCardIsAttacker) {
      // Apply pending deaths (card greys out, death effect starts)
      setPendingDeaths((currentPendingDeaths: Set<string>) => {
        if (currentPendingDeaths.size > 0) {
          applyPendingDeaths(currentPendingDeaths)
        }
        return new Set()
      })

      // Wait for death animation to complete before arena transition
      setCurrentPhase('death_animation')
      phaseTimeoutRef.current = setTimeout(() => {
        handleDeathAnimationComplete(nextEvent, nextIndex, deathsToExclude)
      }, getScaledDuration(ANIMATION_DURATIONS.deathEffect))

      return  // handleDeathAnimationComplete will continue the flow
    }

    // If there are deaths at round boundary (but not involving arena transition), wait for death animation
    if (hasDeaths && (isNonAttackEvent || isRoundTransition)) {
      // Apply pending deaths (card greys out, death effect starts)
      setPendingDeaths((currentPendingDeaths: Set<string>) => {
        if (currentPendingDeaths.size > 0) {
          applyPendingDeaths(currentPendingDeaths)
        }
        return new Set()
      })

      // Wait for death animation to complete before arena transition
      setCurrentPhase('death_animation')
      phaseTimeoutRef.current = setTimeout(() => {
        handleDeathAnimationComplete(nextEvent, nextIndex, deathsToExclude)
      }, getScaledDuration(ANIMATION_DURATIONS.deathEffect))

      return  // handleDeathAnimationComplete will continue the flow
    }

    // No deaths to handle - proceed with normal flow

    // Skip animation for non-attack events (death, advance, summary, victory)
    // These are informational - the visual state is handled via pending deaths
    if (isNonAttackEvent) {
      setCurrentEventIndex(nextIndex)
      setCurrentPhase('idle')
      // Small gap then advance to next event
      phaseTimeoutRef.current = setTimeout(() => {
        advanceToNextEventRef.current?.()
      }, getScaledDuration(ANIMATION_DURATIONS.eventGap))
      return
    }

    // Process attack event with full animation sequence
    // Arena transition check was already done earlier, reuse those values
    if (needsTransition) {
      // Arena cards need to change - perform transition before attack
      setCurrentEventIndex(nextIndex)
      performArenaTransition(newFrontA, newFrontB, () => {
        // After arena transition completes, proceed to highlight phase
        if (isPlayingRef.current) {
          setCurrentPhase('highlight')
          processPhase(nextEvent, 'highlight')
        }
      })
    } else {
      // No arena transition needed - proceed directly to highlight
      setCurrentEventIndex(nextIndex)
      setCurrentPhase('highlight')
      processPhase(nextEvent, 'highlight')
    }
  }, [battleData, processPhase, applyPendingDeaths, getScaledDuration, checkArenaTransitionNeeded, performArenaTransition, completeBattle, handleDeathAnimationComplete, arenaCardA, arenaCardB])

  // Keep advanceToNextEvent ref in sync for use in processPhase
  // (avoids circular dependency: processPhase -> advanceToNextEvent -> processPhase)
  useEffect(() => {
    advanceToNextEventRef.current = advanceToNextEvent
  }, [advanceToNextEvent])

  /**
   * Initialize arena cards at battle start.
   * Gets the front cards and performs an enter animation before the first event.
   *
   * @param onComplete - Callback to run after initialization completes
   */
  const initializeArenaCards = useCallback(
    (onComplete: () => void) => {
      const { cardA, cardB } = getFrontCards()

      // Set initial arena cards
      setArenaCardA(cardA)
      setArenaCardB(cardB)

      // Build entering set with all initial cards
      const enteringKeys = new Set<string>()
      if (cardA) {
        enteringKeys.add(getCardKey(cardA.teamOwnerId, cardA.cardId))
      }
      if (cardB) {
        enteringKeys.add(getCardKey(cardB.teamOwnerId, cardB.cardId))
      }

      // If we have cards, play enter animation
      const hasCards = cardA !== null || cardB !== null

      if (hasCards) {
        setArenaTransitionState('entering')
        setTransitioningCards({ entering: enteringKeys, exiting: new Set() })
        setCurrentPhase('arena_transition')

        phaseTimeoutRef.current = setTimeout(() => {
          setArenaTransitionState('idle')
          setTransitioningCards({ entering: new Set(), exiting: new Set() })
          onComplete()
        }, getScaledDuration(ANIMATION_DURATIONS.arenaEnter))
      } else {
        // No cards to display, just complete
        onComplete()
      }
    },
    [getFrontCards, getScaledDuration]
  )

  /**
   * Start or resume playback.
   */
  const play = useCallback(() => {
    if (!battleData || isComplete) return

    setIsPlaying(true)
    isPlayingRef.current = true

    // If we haven't started yet, initialize arena and advance to first event
    if (currentEventIndexRef.current === -1) {
      // Small delay before starting
      phaseTimeoutRef.current = setTimeout(() => {
        // Initialize arena cards with enter animation, then proceed to first event
        initializeArenaCards(() => {
          if (isPlayingRef.current) {
            advanceToNextEvent()
          }
        })
      }, ANIMATION_DURATIONS.playStart)
    } else if (currentPhase === 'arena_transition') {
      // Resuming during arena transition - CSS animation will resume from paused state
      // We need to reschedule the transition completion timer
      // Since CSS animation uses 'forwards' fill mode and continues from where it paused,
      // we use a short delay to allow the animation to complete
      const remainingDuration = getScaledDuration(
        arenaTransitionState === 'entering' ? ANIMATION_DURATIONS.arenaEnter : ANIMATION_DURATIONS.arenaExit
      )
      phaseTimeoutRef.current = setTimeout(() => {
        if (!isPlayingRef.current) return

        if (arenaTransitionState === 'exiting') {
          // After exit completes, check if we need to enter new cards
          const { cardA, cardB } = getFrontCards()
          setArenaCardA(cardA)
          setArenaCardB(cardB)

          const hasNewCards = cardA !== null || cardB !== null
          if (hasNewCards) {
            setArenaTransitionState('entering')
            phaseTimeoutRef.current = setTimeout(() => {
              setArenaTransitionState('idle')
              isArenaTransitionInProgressRef.current = false
              if (isPlayingRef.current) {
                advanceToNextEvent()
              }
            }, getScaledDuration(ANIMATION_DURATIONS.arenaEnter))
          } else {
            setArenaTransitionState('idle')
            isArenaTransitionInProgressRef.current = false
            if (isPlayingRef.current) {
              advanceToNextEvent()
            }
          }
        } else if (arenaTransitionState === 'entering') {
          // After enter completes, proceed to next event
          setArenaTransitionState('idle')
          isArenaTransitionInProgressRef.current = false
          if (isPlayingRef.current) {
            advanceToNextEvent()
          }
        }
      }, remainingDuration)
    } else if (currentPhase === 'death_animation') {
      // Resuming during death animation - reschedule the completion
      // Use full duration as we don't track partial progress
      phaseTimeoutRef.current = setTimeout(() => {
        if (!isPlayingRef.current) return
        advanceToNextEvent()
      }, getScaledDuration(ANIMATION_DURATIONS.deathEffect))
    } else if (currentPhase === 'idle') {
      // Resume from current position
      advanceToNextEvent()
    } else {
      // If in middle of a phase (highlight, attack, damage, complete), it will continue automatically
      // after the CSS transitions complete - we need to reschedule the processPhase timeout
      // Resuming during an attack phase - reschedule the phase progression
      const currentEvent = battleData.events[currentEventIndexRef.current]
      if (currentEvent) {
        // Resume the current phase processing
        processPhase(currentEvent, currentPhase)
      }
    }
  }, [battleData, isComplete, currentPhase, advanceToNextEvent, initializeArenaCards, arenaTransitionState, getFrontCards, getScaledDuration, processPhase])

  /**
   * Pause playback.
   */
  const pause = useCallback(() => {
    setIsPlaying(false)
    isPlayingRef.current = false
    if (phaseTimeoutRef.current) {
      clearTimeout(phaseTimeoutRef.current)
      phaseTimeoutRef.current = null
    }
  }, [])

  /**
   * Reset to initial state.
   */
  const reset = useCallback(() => {
    // Stop playback
    setIsPlaying(false)
    isPlayingRef.current = false
    if (phaseTimeoutRef.current) {
      clearTimeout(phaseTimeoutRef.current)
      phaseTimeoutRef.current = null
    }

    // Reset all state
    if (battleData) {
      const initialCardStates = initializeCardStates(
        battleData,
        playerAId,
        playerBId
      )
      setCardStates(initialCardStates)
      setAnimationStates(initializeAnimationStates(initialCardStates))
    }
    setCurrentEventIndex(-1)
    setCurrentPhase('idle')
    setIsComplete(false)
    setCurrentDamage(null)
    setDamageTargetKey(null)
    setActiveEffects([])
    setPendingDeaths(new Set())
    pendingDeathsRef.current = new Set()
    currentRoundRef.current = 0
    // Reset arena state
    setArenaCardA(null)
    setArenaCardB(null)
    setArenaTransitionState('idle')
    isArenaTransitionInProgressRef.current = false
  }, [battleData, playerAId, playerBId])

  /**
   * Skip to final state immediately.
   */
  const skipToEnd = useCallback(() => {
    if (!battleData) return

    // Stop playback
    setIsPlaying(false)
    isPlayingRef.current = false
    if (phaseTimeoutRef.current) {
      clearTimeout(phaseTimeoutRef.current)
      phaseTimeoutRef.current = null
    }

    // Apply all events to get final state
    const finalCardStates = initializeCardStates(
      battleData,
      playerAId,
      playerBId
    )
    const finalAnimationStates = initializeAnimationStates(finalCardStates)

    // Process all events to compute final state
    for (const event of battleData.events) {
      if (event.defender_card_id && event.defender_team_owner_id) {
        const defenderKey = getCardKey(
          event.defender_team_owner_id,
          event.defender_card_id
        )
        const defender = finalCardStates.get(defenderKey)

        if (defender) {
          // Calculate hp_after with fallback: prefer explicit value, else compute from hp_before - damage
          const hpAfter =
            event.hp_after ??
            (event.hp_before !== undefined && event.damage !== undefined
              ? event.hp_before - event.damage
              : undefined)

          // Update HP and is_alive
          if (hpAfter !== undefined) {
            // Clamp HP to 0 minimum - backend may send negative values for overkill damage
            const clampedHp = Math.max(0, hpAfter)
            finalCardStates.set(defenderKey, {
              ...defender,
              hp: clampedHp,
              is_alive: clampedHp > 0,
            })
          }
        }
      }
    }

    setCardStates(finalCardStates)
    setAnimationStates(finalAnimationStates)
    setCurrentEventIndex(battleData.events.length - 1)
    setCurrentPhase('idle')
    setIsComplete(true)
    setCurrentDamage(null)
    setDamageTargetKey(null)
    setActiveEffects([])
    setPendingDeaths(new Set())
    pendingDeathsRef.current = new Set()
    currentRoundRef.current = 0

    // Set final arena state based on surviving cards
    // Find front cards from the final card states
    let finalFrontA: ArenaCard | null = null
    let finalFrontB: ArenaCard | null = null
    let minPosA = Infinity
    let minPosB = Infinity

    finalCardStates.forEach((state: CardSnapshot, key: string) => {
      if (!state.is_alive) return

      const parts = key.split('_')
      const teamOwnerId = parseInt(parts[0], 10)
      const cardId = state.card_id

      if (teamOwnerId === playerAId) {
        if (state.position < minPosA) {
          minPosA = state.position
          finalFrontA = { cardId, teamOwnerId }
        }
      } else if (teamOwnerId === playerBId) {
        if (state.position < minPosB) {
          minPosB = state.position
          finalFrontB = { cardId, teamOwnerId }
        }
      }
    })

    setArenaCardA(finalFrontA)
    setArenaCardB(finalFrontB)
    setArenaTransitionState('idle')
    isArenaTransitionInProgressRef.current = false
  }, [battleData, playerAId, playerBId])

  return {
    cardStates,
    animationStates,
    currentEventIndex,
    currentPhase,
    isPlaying,
    isComplete,
    currentDamage,
    damageTargetKey,
    activeEffects,
    onEffectComplete,
    arenaCardA,
    arenaCardB,
    arenaTransitionState,
    transitioningCards,
    getFrontCards,
    play,
    pause,
    reset,
    skipToEnd,
  }
}

export default useBattleAnimation
