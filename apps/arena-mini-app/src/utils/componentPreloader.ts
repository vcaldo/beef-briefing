/**
 * Component preloader for splash screen
 *
 * Warms up lazy-loaded component chunks by triggering their dynamic imports
 * during the splash screen, so they're cached when Suspense renders them.
 */

/**
 * Preload all page components by triggering their dynamic imports.
 * This is fire-and-forget - failures are silently logged since
 * components will load on-demand if preload fails.
 */
export function preloadPageComponents(): void {
  const preloads = [
    import('../components/lobby/LobbyPage'),
    import('../components/shop/ShopPage'),
    import('../components/battle/BattlePage'),
    import('../components/stats/StatsPage'),
  ]

  Promise.all(preloads).catch((err) => {
    console.warn('[componentPreloader] Failed to preload components:', err)
  })
}
