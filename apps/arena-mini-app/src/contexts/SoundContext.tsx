/**
 * SoundContext - React context for game sound system
 *
 * Provides audio functionality throughout the app:
 * - Sound playback with audio pooling
 * - Volume and mute controls
 * - Preloading by category
 * - Mobile autoplay unlock
 *
 * Preloads lobby sounds automatically on mount.
 */

import { createContext, useContext, useEffect, useRef, type ReactNode } from 'react'
import { useSound, type UseSoundReturn } from '../hooks/useSound'

export interface SoundProviderProps {
  /** Base URL for sound files (e.g., "http://localhost:9000/beef-briefing/sounds/arena") */
  baseUrl: string
  children: ReactNode
}

// Create context with undefined default (will throw if used outside provider)
const SoundContext = createContext<UseSoundReturn | undefined>(undefined)

/**
 * Provider component that wraps the app and provides sound functionality.
 * Preloads lobby sounds on mount.
 *
 * Usage:
 * ```tsx
 * <SoundProvider baseUrl={import.meta.env.VITE_SOUND_BASE_URL}>
 *   <App />
 * </SoundProvider>
 * ```
 */
export function SoundProvider({ baseUrl, children }: SoundProviderProps) {
  const sound = useSound({ baseUrl })
  const hasPreloadedRef = useRef(false)
  const hasWarnedRef = useRef(false)

  // Warn if baseUrl is empty or invalid (only in development)
  useEffect(() => {
    if (hasWarnedRef.current) return
    hasWarnedRef.current = true

    if (!baseUrl) {
      console.warn(
        '[SoundProvider] baseUrl is empty. Sound URLs will be malformed. ' +
        'Set VITE_SOUND_BASE_URL environment variable.'
      )
    } else if (!baseUrl.startsWith('http://') && !baseUrl.startsWith('https://')) {
      console.warn(
        `[SoundProvider] baseUrl "${baseUrl}" does not look like a valid URL. ` +
        'Sound loading may fail.'
      )
    }
  }, [baseUrl])

  // Preload lobby sounds on mount (once only)
  useEffect(() => {
    if (hasPreloadedRef.current) return
    hasPreloadedRef.current = true

    sound.preloadCategory('lobby').catch((err) => {
      // Non-fatal: sounds will load on-demand if preload fails
      console.warn('Failed to preload lobby sounds:', err)
    })
  }, [sound])

  return <SoundContext.Provider value={sound}>{children}</SoundContext.Provider>
}

/**
 * Hook to access sound functionality from context.
 * Must be used within a SoundProvider.
 *
 * Returns the same interface as useSound:
 * - play(soundId): Play a sound
 * - preload(soundIds): Preload specific sounds
 * - preloadCategory(category): Preload all sounds in a category
 * - volume: Current volume (0-1)
 * - setVolume(volume): Set volume
 * - isMuted: Whether audio is muted
 * - toggleMute(): Toggle mute state
 * - isReady: Whether audio is unlocked on mobile
 * - unlockAudio(): Unlock audio (call on user interaction)
 *
 * Usage:
 * ```tsx
 * const { play, unlockAudio, isMuted, toggleMute } = useSoundContext()
 *
 * // On first user interaction
 * unlockAudio()
 *
 * // Play a sound
 * play('lobby_create')
 * ```
 */
export function useSoundContext(): UseSoundReturn {
  const context = useContext(SoundContext)
  if (context === undefined) {
    throw new Error('useSoundContext must be used within a SoundProvider')
  }
  return context
}

export default SoundProvider
