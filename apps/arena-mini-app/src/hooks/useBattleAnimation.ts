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
  /** Currently active battle effect (attack, damage, death, spark) */
  activeEffect: ActiveBattleEffect | null
  /** Callback to call when an effect animation completes */
  onEffectComplete: () => void
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

  // Battle effect state (attack, damage, death animations)
  const [activeEffect, setActiveEffect] = useState<ActiveBattleEffect | null>(null)

  // Deferred death state - cards that should die after the round completes
  // This prevents cards from greying out before their attack animation plays
  // Note: We only use the setter with callbacks, so we don't destructure the state value
  const [, setPendingDeaths] = useState<Set<string>>(() => new Set())
  const currentRoundRef = useRef<number>(0)

  // Refs for cleanup and state access in callbacks
  const phaseTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isPlayingRef = useRef(isPlaying)
  const currentEventIndexRef = useRef(currentEventIndex)
  const advanceToNextEventRef = useRef<(() => void) | null>(null)

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
      setActiveEffect(null)
      setPendingDeaths(new Set())
      currentRoundRef.current = 0
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
    setActiveEffect(null)
  }, [])

  /**
   * Callback to signal that an effect animation has completed.
   * Clears the active effect state.
   */
  const onEffectComplete = useCallback(() => {
    setActiveEffect(null)
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
                setActiveEffect({
                  type: 'attack',
                  position,
                  cardKey: attackerKey,
                })
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
                setActiveEffect({
                  type: 'damage',
                  position,
                  cardKey: defenderKey,
                })
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
            setPendingDeaths((prev) => new Set(prev).add(defenderKey))
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

      // Trigger 'death' visual effect for the first death
      // Note: We only show one death effect at a time to avoid visual clutter
      // If multiple cards die simultaneously, they will still all be marked as dead
      if (getCardPosition) {
        const firstDeathKey = deaths.values().next().value
        if (firstDeathKey) {
          const position = getCardPosition(firstDeathKey)
          if (position) {
            setActiveEffect({
              type: 'death',
              position,
              cardKey: firstDeathKey,
            })
          }
        }
      }

      // Clear pending deaths
      setPendingDeaths(new Set())
    },
    [onPlaySound, getCardPosition]
  )

  /**
   * Advance to the next event in the sequence.
   */
  const advanceToNextEvent = useCallback(() => {
    if (!battleData || !isPlayingRef.current) return

    const nextIndex = currentEventIndexRef.current + 1

    if (nextIndex >= battleData.events.length) {
      // All events processed - apply any remaining pending deaths
      setPendingDeaths((currentPendingDeaths: Set<string>) => {
        if (currentPendingDeaths.size > 0) {
          applyPendingDeaths(currentPendingDeaths)
        }
        return new Set()
      })
      setIsPlaying(false)
      isPlayingRef.current = false
      setIsComplete(true)
      setCurrentPhase('idle')
      return
    }

    const nextEvent = battleData.events[nextIndex]
    const prevRound = currentRoundRef.current
    const isNonAttackEvent = nextEvent.type !== 'attack'
    const isRoundTransition = nextEvent.round !== prevRound && prevRound !== 0

    // Apply pending deaths at round boundaries:
    // - When transitioning to a different round
    // - When processing non-attack events (death, advance, summary, victory)
    if (isNonAttackEvent || isRoundTransition) {
      setPendingDeaths((currentPendingDeaths: Set<string>) => {
        if (currentPendingDeaths.size > 0) {
          applyPendingDeaths(currentPendingDeaths)
        }
        return new Set()
      })
    }

    // Update current round tracking
    currentRoundRef.current = nextEvent.round

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
    setCurrentEventIndex(nextIndex)
    setCurrentPhase('highlight')
    processPhase(nextEvent, 'highlight')
  }, [battleData, processPhase, applyPendingDeaths, getScaledDuration])

  // Keep advanceToNextEvent ref in sync for use in processPhase
  // (avoids circular dependency: processPhase -> advanceToNextEvent -> processPhase)
  useEffect(() => {
    advanceToNextEventRef.current = advanceToNextEvent
  }, [advanceToNextEvent])

  /**
   * Start or resume playback.
   */
  const play = useCallback(() => {
    if (!battleData || isComplete) return

    setIsPlaying(true)
    isPlayingRef.current = true

    // If we haven't started yet, advance to first event
    if (currentEventIndexRef.current === -1) {
      // Small delay before starting
      phaseTimeoutRef.current = setTimeout(() => {
        advanceToNextEvent()
      }, ANIMATION_DURATIONS.playStart)
    } else if (currentPhase === 'idle') {
      // Resume from current position
      advanceToNextEvent()
    }
    // If in middle of a phase, it will continue automatically
  }, [battleData, isComplete, currentPhase, advanceToNextEvent])

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
    setActiveEffect(null)
    setPendingDeaths(new Set())
    currentRoundRef.current = 0
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
    setActiveEffect(null)
    setPendingDeaths(new Set())
    currentRoundRef.current = 0
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
    activeEffect,
    onEffectComplete,
    play,
    pause,
    reset,
    skipToEnd,
  }
}

export default useBattleAnimation
