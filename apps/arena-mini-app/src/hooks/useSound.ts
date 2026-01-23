/**
 * useSound - Audio playback hook for arena game sounds
 *
 * Features:
 * - Type-safe SoundId union for all game sounds
 * - Audio preloading by category (lobby, shop, team, battle)
 * - Volume and mute controls with localStorage persistence
 * - Mobile autoplay unlock via silent audio trick
 * - Audio element pooling for overlapping sounds
 */

import { useState, useCallback, useRef, useEffect } from 'react'

// All available sound IDs in the game
export type SoundId =
  | 'arena_lobby_create'
  | 'arena_lobby_join'
  | 'arena_lobby_start'
  | 'arena_button_click'
  | 'arena_team_place'
  | 'arena_team_upgrade'
  | 'arena_battle_attack'
  | 'arena_battle_damage'
  | 'arena_battle_death'
  | 'arena_battle_win'
  | 'arena_battle_lose'
  | 'arena_error'
  | 'arena_coin_spend'
  | 'arena_card_draw'
  | 'arena_card_shuffle'
  | 'arena_countdown_tick'
  | 'arena_countdown_warning'
  | 'arena_success'
  | 'arena_critical_hp'

// Sound categories for preloading
export type SoundCategory = 'lobby' | 'shop' | 'team' | 'battle'

/**
 * A sound item in a sequence.
 * Can be either:
 * - A SoundId (uses default delay after it)
 * - A tuple of [SoundId, delayMs] to specify custom delay after this sound
 */
export type SequenceItem = SoundId | [SoundId, number]

/**
 * Options for playSequence
 */
export interface PlaySequenceOptions {
  /** Default delay between sounds in milliseconds (default: 150) */
  defaultDelay?: number
}

// Map categories to their sound IDs
const CATEGORY_SOUNDS: Record<SoundCategory, SoundId[]> = {
  lobby: ['arena_lobby_create', 'arena_lobby_join', 'arena_lobby_start', 'arena_countdown_tick', 'arena_countdown_warning'],
  shop: ['arena_button_click', 'arena_card_draw', 'arena_card_shuffle', 'arena_coin_spend', 'arena_error', 'arena_success'],
  team: ['arena_team_place', 'arena_team_upgrade', 'arena_button_click', 'arena_error'],
  battle: ['arena_battle_attack', 'arena_battle_damage', 'arena_battle_death', 'arena_battle_win', 'arena_battle_lose', 'arena_critical_hp'],
}

// Sounds with multiple variants (randomly selected at play time)
// Maps soundId to number of variants available
const SOUND_VARIANTS: Partial<Record<SoundId, number>> = {
  arena_battle_attack: 10,
  arena_battle_damage: 10,
}

// localStorage keys
const STORAGE_VOLUME_KEY = 'arena-sound-volume'
const STORAGE_MUTED_KEY = 'arena-sound-muted'

// Audio pool size for overlapping sounds
const POOL_SIZE = 4

// Default volume (0-1) - fixed at 50%
const DEFAULT_VOLUME = 0.5

export interface UseSoundOptions {
  /** Base URL for sound files (e.g., "http://localhost:9000/beef-briefing/sounds/arena") */
  baseUrl: string
}

export interface UseSoundReturn {
  /** Play a sound by ID */
  play: (soundId: SoundId) => void
  /** Play multiple sounds in sequence with configurable delays */
  playSequence: (sounds: SequenceItem[], options?: PlaySequenceOptions) => void
  /** Preload specific sounds */
  preload: (soundIds: SoundId[]) => Promise<void>
  /** Preload all sounds in a category */
  preloadCategory: (category: SoundCategory) => Promise<void>
  /** Current volume (0-1) */
  volume: number
  /** Set volume (0-1) */
  setVolume: (volume: number) => void
  /** Whether audio is muted */
  isMuted: boolean
  /** Toggle mute state */
  toggleMute: () => void
  /** Whether audio system is ready (unlocked on mobile) */
  isReady: boolean
  /** Unlock audio on mobile (call on user interaction) */
  unlockAudio: () => void
}

/**
 * Hook for playing game sounds with preloading and mobile support.
 *
 * Usage:
 * ```tsx
 * const { play, preloadCategory, unlockAudio } = useSound({
 *   baseUrl: 'http://localhost:9000/beef-briefing/sounds/arena'
 * })
 *
 * // On first user interaction
 * unlockAudio()
 *
 * // Preload sounds for current phase
 * preloadCategory('lobby')
 *
 * // Play a sound
 * play('arena_lobby_create')
 * ```
 */
export function useSound({ baseUrl }: UseSoundOptions): UseSoundReturn {
  // Audio cache: cacheKey -> HTMLAudioElement[]
  // Cache keys are soundId for regular sounds, or "soundId:variant" for variant sounds
  const audioPoolRef = useRef<Map<string, HTMLAudioElement[]>>(new Map())
  // Track next audio element index for round-robin pooling
  const poolIndexRef = useRef<Map<string, number>>(new Map())
  // Track loaded sounds (by cache key)
  const loadedSoundsRef = useRef<Set<string>>(new Set())
  // Track if audio is unlocked on mobile
  const [isReady, setIsReady] = useState(false)
  // Silent audio element for mobile unlock
  const silentAudioRef = useRef<HTMLAudioElement | null>(null)

  // Volume state with localStorage persistence
  const [volume, setVolumeState] = useState(() => {
    if (typeof window === 'undefined') return DEFAULT_VOLUME
    try {
      const stored = localStorage.getItem(STORAGE_VOLUME_KEY)
      if (stored !== null) {
        const parsed = parseFloat(stored)
        if (!isNaN(parsed) && parsed >= 0 && parsed <= 1) {
          return parsed
        }
      }
    } catch {
      // localStorage may be unavailable in private browsing mode
    }
    return DEFAULT_VOLUME
  })

  // Mute state with localStorage persistence
  const [isMuted, setIsMuted] = useState(() => {
    if (typeof window === 'undefined') return false
    try {
      return localStorage.getItem(STORAGE_MUTED_KEY) === 'true'
    } catch {
      // localStorage may be unavailable in private browsing mode
      return false
    }
  })

  // Create silent audio for mobile unlock trick
  useEffect(() => {
    // Create a very short silent audio data URI
    // This is a minimal valid MP3 file (silence)
    const silentDataUri =
      'data:audio/mp3;base64,SUQzBAAAAAAAI1RTU0UAAAAPAAADTGF2ZjU4Ljc2LjEwMAAAAAAAAAAAAAAA//tQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWGluZwAAAA8AAAACAAABhgC7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7u7//////////////////////////////////////////////////////////////////8AAAAATGF2YzU4LjEzAAAAAAAAAAAAAAAAJAAAAAAAAAAAAYYoRwmHAAAAAAD/+9DEAAAIAANIAAAAEwAAaQAAAATEFNRTMuMTAwVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVX/+9DEMgAAADSAAAAAAAAANIAAAAAVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVU='

    const audio = new Audio(silentDataUri)
    audio.volume = 0.01
    silentAudioRef.current = audio

    return () => {
      if (silentAudioRef.current) {
        silentAudioRef.current.pause()
        silentAudioRef.current = null
      }
    }
  }, [])

  // Cleanup all audio elements on unmount
  useEffect(() => {
    return () => {
      audioPoolRef.current.forEach((pool) => {
        pool.forEach((audio) => {
          audio.pause()
          audio.src = ''
        })
      })
      audioPoolRef.current.clear()
      poolIndexRef.current.clear()
      loadedSoundsRef.current.clear()
    }
  }, [])

  // Get URL for a sound file (with cache-busting version param)
  // For sounds with variants, pass the variant number to get the specific file
  const getSoundUrl = useCallback(
    (soundId: SoundId, variant?: number): string => {
      const version = import.meta.env.VITE_SOUND_VERSION || '1'
      if (variant !== undefined) {
        // Variant sounds are stored in subdirectories: arena_battle_attack/arena_battle_attack_1.ogg
        return `${baseUrl}/${soundId}/${soundId}_${variant}.ogg?v=${version}`
      }
      return `${baseUrl}/${soundId}.ogg?v=${version}`
    },
    [baseUrl]
  )

  // Get internal cache key for a sound (includes variant number for variant sounds)
  const getCacheKey = (soundId: SoundId, variant?: number): string => {
    return variant !== undefined ? `${soundId}:${variant}` : soundId
  }

  // Create audio pool for a sound (optionally for a specific variant)
  const createAudioPool = useCallback(
    (soundId: SoundId, variant?: number): HTMLAudioElement[] => {
      const pool: HTMLAudioElement[] = []
      const url = getSoundUrl(soundId, variant)

      for (let i = 0; i < POOL_SIZE; i++) {
        const audio = new Audio(url)
        audio.preload = 'auto'
        audio.volume = isMuted ? 0 : volume
        pool.push(audio)
      }

      return pool
    },
    [getSoundUrl, volume, isMuted]
  )

  // Preload a single sound variant
  const preloadSoundVariant = useCallback(
    async (soundId: SoundId, variant?: number): Promise<void> => {
      const cacheKey = getCacheKey(soundId, variant)

      // Skip if already loaded
      if (loadedSoundsRef.current.has(cacheKey)) {
        return
      }

      // Create audio pool
      const pool = createAudioPool(soundId, variant)
      audioPoolRef.current.set(cacheKey, pool)
      poolIndexRef.current.set(cacheKey, 0)

      // Wait for all to be ready
      await Promise.all(
        pool.map(
          (audio) =>
            new Promise<void>((resolve) => {
              if (audio.readyState >= 2) {
                resolve()
              } else {
                audio.addEventListener('canplay', () => resolve(), { once: true })
                audio.addEventListener('error', () => resolve(), { once: true })
              }
            })
        )
      )

      loadedSoundsRef.current.add(cacheKey)
    },
    [createAudioPool]
  )

  // Preload a sound (including all variants if applicable)
  const preloadSound = useCallback(
    async (soundId: SoundId): Promise<void> => {
      const variantCount = SOUND_VARIANTS[soundId]

      if (variantCount) {
        // Preload all variants in parallel
        const variantPromises = []
        for (let i = 1; i <= variantCount; i++) {
          variantPromises.push(preloadSoundVariant(soundId, i))
        }
        await Promise.all(variantPromises)
      } else {
        // Preload single sound
        await preloadSoundVariant(soundId)
      }
    },
    [preloadSoundVariant]
  )

  // Preload multiple sounds
  const preload = useCallback(
    async (soundIds: SoundId[]): Promise<void> => {
      await Promise.all(soundIds.map(preloadSound))
    },
    [preloadSound]
  )

  // Preload all sounds in a category
  const preloadCategory = useCallback(
    async (category: SoundCategory): Promise<void> => {
      const sounds = CATEGORY_SOUNDS[category]
      await preload(sounds)
    },
    [preload]
  )

  // Play a sound (randomly selects variant for sounds with variants)
  const play = useCallback(
    (soundId: SoundId): void => {
      // Don't play if muted
      if (isMuted) return

      // Check if this sound has variants
      const variantCount = SOUND_VARIANTS[soundId]
      let cacheKey: string
      let variant: number | undefined

      if (variantCount) {
        // Randomly select a variant (1 to variantCount)
        variant = Math.floor(Math.random() * variantCount) + 1
        cacheKey = getCacheKey(soundId, variant)
      } else {
        cacheKey = soundId
      }

      // Get or create audio pool
      let pool = audioPoolRef.current.get(cacheKey)
      if (!pool) {
        pool = createAudioPool(soundId, variant)
        audioPoolRef.current.set(cacheKey, pool)
        poolIndexRef.current.set(cacheKey, 0)
      }

      // Get next audio element (round-robin)
      const index = poolIndexRef.current.get(cacheKey) ?? 0
      const audio = pool[index]

      // Update pool index
      poolIndexRef.current.set(cacheKey, (index + 1) % POOL_SIZE)

      // Reset and play
      audio.currentTime = 0
      audio.volume = volume
      audio.play().catch(() => {
        // Ignore autoplay errors - user hasn't unlocked audio yet
      })
    },
    [isMuted, volume, createAudioPool]
  )

  // Default delay between sounds in a sequence (ms)
  const DEFAULT_SEQUENCE_DELAY = 150

  // Play multiple sounds in sequence with configurable delays
  const playSequence = useCallback(
    (sounds: SequenceItem[], options?: PlaySequenceOptions): void => {
      if (sounds.length === 0) return

      const defaultDelay = options?.defaultDelay ?? DEFAULT_SEQUENCE_DELAY
      let cumulativeDelay = 0

      sounds.forEach((item, index) => {
        // Parse the item - either SoundId or [SoundId, delayMs]
        const soundId: SoundId = Array.isArray(item) ? item[0] : item
        const delayAfter: number = Array.isArray(item) ? item[1] : defaultDelay

        if (cumulativeDelay === 0) {
          // First sound plays immediately
          play(soundId)
        } else {
          // Subsequent sounds play after cumulative delay
          setTimeout(() => {
            play(soundId)
          }, cumulativeDelay)
        }

        // Add delay for next sound (skip for last item)
        if (index < sounds.length - 1) {
          cumulativeDelay += delayAfter
        }
      })
    },
    [play]
  )

  // Unlock audio on mobile (must be called from user interaction)
  const unlockAudio = useCallback((): void => {
    if (isReady) return

    // Play silent audio to unlock
    if (silentAudioRef.current) {
      silentAudioRef.current
        .play()
        .then(() => {
          setIsReady(true)
        })
        .catch(() => {
          // Still not unlocked, will try again on next interaction
        })
    }
  }, [isReady])

  // Set volume with localStorage persistence
  const setVolume = useCallback((newVolume: number): void => {
    const clampedVolume = Math.max(0, Math.min(1, newVolume))
    setVolumeState(clampedVolume)
    try {
      localStorage.setItem(STORAGE_VOLUME_KEY, clampedVolume.toString())
    } catch {
      // localStorage may be unavailable in private browsing mode
    }

    // Update all audio elements
    audioPoolRef.current.forEach((pool) => {
      pool.forEach((audio) => {
        audio.volume = clampedVolume
      })
    })
  }, [])

  // Toggle mute with localStorage persistence
  const toggleMute = useCallback((): void => {
    setIsMuted((prev) => {
      const newMuted = !prev
      try {
        localStorage.setItem(STORAGE_MUTED_KEY, newMuted.toString())
      } catch {
        // localStorage may be unavailable in private browsing mode
      }

      // Update all audio elements
      audioPoolRef.current.forEach((pool) => {
        pool.forEach((audio) => {
          audio.volume = newMuted ? 0 : volume
        })
      })

      return newMuted
    })
  }, [volume])

  return {
    play,
    playSequence,
    preload,
    preloadCategory,
    volume,
    setVolume,
    isMuted,
    toggleMute,
    isReady,
    unlockAudio,
  }
}

export default useSound
