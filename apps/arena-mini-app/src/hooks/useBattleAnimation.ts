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
import { ANIMATION_DURATIONS } from '../types'

// =============================================================================
// TYPES
// =============================================================================

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
}

// =============================================================================
// HELPERS
// =============================================================================

/**
 * Generate a composite key for card state lookups.
 * Uses teamOwnerId + cardId to distinguish same card on different teams.
 */
const getCardKey = (teamOwnerId: number, cardId: number): string =>
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
  const { playbackSpeed, playerAId, playerBId, onPlaySound } = options

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
            onPlaySound?.('battle_attack')
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
            onPlaySound?.('battle_damage')
          }

          // Schedule damage phase
          phaseTimeoutRef.current = setTimeout(() => {
            setCurrentPhase('damage')
            processPhase(event, 'damage')
          }, getScaledDuration(ANIMATION_DURATIONS.damage))
          break
        }

        case 'damage': {
          // Debug logging to trace HP updates
          console.log('[Battle Debug] Damage phase:', {
            eventType: event.type,
            defender_card_id: event.defender_card_id,
            defender_team_owner_id: event.defender_team_owner_id,
            hp_before: event.hp_before,
            hp_after: event.hp_after,
            damage: event.damage,
          })

          // Update HP in card states (triggers HP bar animation via CSS transition)
          if (event.defender_card_id && event.defender_team_owner_id) {
            const defenderKey = getCardKey(
              event.defender_team_owner_id,
              event.defender_card_id
            )

            setCardStates((prev: Map<string, CardSnapshot>) => {
              const next = new Map(prev)
              const defender = next.get(defenderKey)

              // Debug: log key lookup result
              console.log('[Battle Debug] Key lookup:', {
                defenderKey,
                found: !!defender,
                availableKeys: Array.from(prev.keys()),
              })

              // Calculate hp_after with fallback: prefer explicit value, else compute from hp_before - damage
              const hpAfter =
                event.hp_after ??
                (event.hp_before !== undefined && event.damage !== undefined
                  ? event.hp_before - event.damage
                  : undefined)

              console.log('[Battle Debug] HP calculation:', {
                hp_after_from_event: event.hp_after,
                hp_after_computed: hpAfter,
                will_update: defender && hpAfter !== undefined,
              })

              if (defender && hpAfter !== undefined) {
                // Clamp HP to 0 minimum - backend may send negative values for overkill damage
                const clampedHp = Math.max(0, hpAfter)
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
          // Update is_alive state for any cards that died during this event
          // This is done AFTER animations complete so the card doesn't grey out mid-attack
          if (event.defender_card_id && event.defender_team_owner_id) {
            const defenderKey = getCardKey(
              event.defender_team_owner_id,
              event.defender_card_id
            )
            setCardStates((prev: Map<string, CardSnapshot>) => {
              const next = new Map(prev)
              const defender = next.get(defenderKey)
              if (defender) {
                const defenderDied = defender.hp <= 0

                // Now set is_alive based on current HP
                next.set(defenderKey, {
                  ...defender,
                  is_alive: defender.hp > 0,
                })

                // Play death sound if defender died
                if (defenderDied) {
                  onPlaySound?.('battle_death')
                }
              }
              return next
            })
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
    [getScaledDuration, clearAnimationStates, onPlaySound]
  )

  /**
   * Advance to the next event in the sequence.
   */
  const advanceToNextEvent = useCallback(() => {
    if (!battleData || !isPlayingRef.current) return

    const nextIndex = currentEventIndexRef.current + 1

    if (nextIndex >= battleData.events.length) {
      // All events processed
      setIsPlaying(false)
      isPlayingRef.current = false
      setIsComplete(true)
      setCurrentPhase('idle')
      return
    }

    const nextEvent = battleData.events[nextIndex]
    setCurrentEventIndex(nextIndex)
    setCurrentPhase('highlight')
    processPhase(nextEvent, 'highlight')
  }, [battleData, processPhase])

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
      }, 100)
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
    play,
    pause,
    reset,
    skipToEnd,
  }
}

export default useBattleAnimation
