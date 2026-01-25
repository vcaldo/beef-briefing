/**
 * Asset preloader for splash screen
 *
 * Preloads critical UI assets during the splash screen to eliminate
 * visual delays when the main app renders.
 *
 * Called from App.tsx immediately on mount (before authentication).
 */

import { preloadCategories } from './images'
import { preloadPageComponents } from './componentPreloader'
import type { ImageCategory } from '../types/images'

/**
 * Categories to preload during splash screen.
 * These are used immediately on LobbyPage and throughout the app.
 *
 * NOT preloaded:
 * - 'bars' - Only needed in shop/battle UI
 * - 'effects' - Only needed in battle, too heavy (15 images @ 512x512)
 */
const CRITICAL_IMAGE_CATEGORIES: ImageCategory[] = ['buttons', 'panels', 'icons']

/**
 * Preload critical UI assets during splash screen.
 *
 * This function is fire-and-forget (non-blocking).
 * If preloading fails, the app continues normally and images
 * will load on-demand.
 *
 * @example
 * // In App.tsx, after authentication
 * prefetchGameConstants()
 * preloadCriticalAssets() // Fire and forget
 */
export function preloadCriticalAssets(): void {
  preloadCategories(CRITICAL_IMAGE_CATEGORIES).catch((err) => {
    // Non-fatal: images will load on-demand if preload fails
    console.warn('[assetPreloader] Failed to preload critical assets:', err)
  })
}

/**
 * Categories to preload during shop phase.
 * These are needed for battle animations.
 */
const BATTLE_IMAGE_CATEGORIES: ImageCategory[] = ['effects', 'bars']

/**
 * Preload battle assets during shop phase.
 *
 * Called after shop UI is ready, while user is selecting cards.
 * This ensures battle animations play smoothly without on-demand loading.
 *
 * @example
 * // In ShopPage, after loading = false
 * preloadBattleAssets()
 */
export function preloadBattleAssets(): void {
  preloadCategories(BATTLE_IMAGE_CATEGORIES).catch((err) => {
    // Non-fatal: images will load on-demand if preload fails
    console.warn('[assetPreloader] Failed to preload battle assets:', err)
  })
}

/**
 * Preload all critical assets during splash screen.
 * Called immediately on app mount (before authentication).
 *
 * This runs in parallel with authentication to maximize use of the
 * 2-second splash screen window.
 *
 * @example
 * // In App.tsx, on mount
 * useEffect(() => {
 *   preloadAllDuringSplash()
 * }, [])
 */
export function preloadAllDuringSplash(): void {
  // 1. Preload page component chunks
  preloadPageComponents()

  // 2. Preload critical images (buttons, panels, icons)
  preloadCriticalAssets()

  // 3. Preload battle assets with slight delay (lower priority)
  setTimeout(() => {
    preloadBattleAssets()
  }, 500)
}
