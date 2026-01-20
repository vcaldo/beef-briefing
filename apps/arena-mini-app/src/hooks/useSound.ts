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
  | 'lobby_create'
  | 'lobby_join'
  | 'lobby_start'
  | 'shop_reroll'
  | 'shop_buy'
  | 'button_click'
  | 'team_place'
  | 'team_upgrade'
  | 'battle_attack'
  | 'battle_damage'
  | 'battle_death'
  | 'battle_win'
  | 'battle_lose'

// Sound categories for preloading
export type SoundCategory = 'lobby' | 'shop' | 'team' | 'battle'

// Map categories to their sound IDs
const CATEGORY_SOUNDS: Record<SoundCategory, SoundId[]> = {
  lobby: ['lobby_create', 'lobby_join', 'lobby_start'],
  shop: ['shop_reroll', 'shop_buy', 'button_click'],
  team: ['team_place', 'team_upgrade', 'button_click'],
  battle: ['battle_attack', 'battle_damage', 'battle_death', 'battle_win', 'battle_lose'],
}

// localStorage keys
const STORAGE_VOLUME_KEY = 'arena-sound-volume'
const STORAGE_MUTED_KEY = 'arena-sound-muted'

// Audio pool size for overlapping sounds
const POOL_SIZE = 4

// Default volume (0-1)
const DEFAULT_VOLUME = 0.7

export interface UseSoundOptions {
  /** Base URL for sound files (e.g., "http://localhost:9000/beef-briefing/sounds/arena") */
  baseUrl: string
}

export interface UseSoundReturn {
  /** Play a sound by ID */
  play: (soundId: SoundId) => void
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
 * play('lobby_create')
 * ```
 */
export function useSound({ baseUrl }: UseSoundOptions): UseSoundReturn {
  // Audio cache: soundId -> HTMLAudioElement[]
  const audioPoolRef = useRef<Map<SoundId, HTMLAudioElement[]>>(new Map())
  // Track next audio element index for round-robin pooling
  const poolIndexRef = useRef<Map<SoundId, number>>(new Map())
  // Track loaded sounds
  const loadedSoundsRef = useRef<Set<SoundId>>(new Set())
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
  const getSoundUrl = useCallback(
    (soundId: SoundId): string => {
      const version = import.meta.env.VITE_SOUND_VERSION || '1'
      return `${baseUrl}/${soundId}.ogg?v=${version}`
    },
    [baseUrl]
  )

  // Create audio pool for a sound
  const createAudioPool = useCallback(
    (soundId: SoundId): HTMLAudioElement[] => {
      const pool: HTMLAudioElement[] = []
      const url = getSoundUrl(soundId)

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

  // Preload a single sound
  const preloadSound = useCallback(
    async (soundId: SoundId): Promise<void> => {
      // Skip if already loaded
      if (loadedSoundsRef.current.has(soundId)) {
        return
      }

      // Create audio pool
      const pool = createAudioPool(soundId)
      audioPoolRef.current.set(soundId, pool)
      poolIndexRef.current.set(soundId, 0)

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

      loadedSoundsRef.current.add(soundId)
    },
    [createAudioPool]
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

  // Play a sound
  const play = useCallback(
    (soundId: SoundId): void => {
      // Don't play if muted
      if (isMuted) return

      // Get or create audio pool
      let pool = audioPoolRef.current.get(soundId)
      if (!pool) {
        pool = createAudioPool(soundId)
        audioPoolRef.current.set(soundId, pool)
        poolIndexRef.current.set(soundId, 0)
      }

      // Get next audio element (round-robin)
      const index = poolIndexRef.current.get(soundId) ?? 0
      const audio = pool[index]

      // Update pool index
      poolIndexRef.current.set(soundId, (index + 1) % POOL_SIZE)

      // Reset and play
      audio.currentTime = 0
      audio.volume = volume
      audio.play().catch(() => {
        // Ignore autoplay errors - user hasn't unlocked audio yet
      })
    },
    [isMuted, volume, createAudioPool]
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
