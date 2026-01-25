/**
 * usePageBackground - Hook for managing page background images
 *
 * Features:
 * - Sets CSS custom properties for background URLs with cache-busting
 * - Preloads background images before applying
 * - Cleans up CSS properties on unmount
 *
 * Usage:
 * ```tsx
 * function BattlePage() {
 *   usePageBackground({ backgroundId: 'arena' })
 *
 *   return (
 *     <div className="battle-arena page-bg page-bg--arena">
 *       ...
 *     </div>
 *   )
 * }
 * ```
 */

import { useEffect, useCallback, useState } from 'react'
import { BackgroundImageId } from '../types/images'
import { getBackgroundUrl, preloadImage } from '../utils/images'

export interface UsePageBackgroundOptions {
  /** Background ID to apply */
  backgroundId: BackgroundImageId
  /** Whether to preload before applying (default: true) */
  preload?: boolean
}

export interface UsePageBackgroundReturn {
  /** Whether the background image is ready (preloaded) */
  isReady: boolean
  /** Error if preloading failed */
  error: Error | null
}

/**
 * Hook to manage page background images via CSS custom properties.
 *
 * Sets a CSS variable `--bg-{backgroundId}-url` on the document root
 * with the background image URL. This is consumed by CSS classes like
 * `.page-bg--arena` which reference `var(--bg-arena-url)`.
 *
 * @param options - Background options
 * @returns Object with isReady state and any error
 *
 * @example
 * ```tsx
 * function BattlePage() {
 *   const { isReady } = usePageBackground({ backgroundId: 'arena' })
 *
 *   return (
 *     <div className="page-bg page-bg--arena">
 *       {isReady ? 'Background loaded!' : 'Loading...'}
 *     </div>
 *   )
 * }
 * ```
 */
export function usePageBackground({
  backgroundId,
  preload = true,
}: UsePageBackgroundOptions): UsePageBackgroundReturn {
  const [isReady, setIsReady] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const applyBackground = useCallback(() => {
    const url = getBackgroundUrl(backgroundId)
    document.documentElement.style.setProperty(
      `--bg-${backgroundId}-url`,
      `url(${url})`
    )
    setIsReady(true)
  }, [backgroundId])

  useEffect(() => {
    const url = getBackgroundUrl(backgroundId)

    if (preload) {
      preloadImage(url)
        .then(() => {
          applyBackground()
        })
        .catch((err) => {
          console.warn(`[usePageBackground] Failed to preload ${backgroundId}:`, err)
          setError(err instanceof Error ? err : new Error(String(err)))
          // Apply anyway - browser will load it when needed
          applyBackground()
        })
    } else {
      applyBackground()
    }

    // Cleanup: reset CSS property when unmounting
    return () => {
      document.documentElement.style.setProperty(`--bg-${backgroundId}-url`, 'none')
    }
  }, [backgroundId, preload, applyBackground])

  return { isReady, error }
}

export default usePageBackground
