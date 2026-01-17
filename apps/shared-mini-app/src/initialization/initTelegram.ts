/**
 * Telegram Mini App SDK initialization.
 * Centralizes common initialization logic for all Mini Apps.
 *
 * IMPORTANT: This function is async and MUST be awaited before rendering React.
 * Failure to await can cause React error #310 in production builds.
 */

import {
  init,
  miniApp,
  viewport,
  backButton,
} from '@telegram-apps/sdk-react'

// Track SDK initialization state
let sdkInitialized = false

/**
 * Check if the Telegram SDK has been fully initialized.
 * Use this to guard code that depends on SDK features.
 */
export function isSDKInitialized(): boolean {
  return sdkInitialized
}

/**
 * Initialize Telegram Mini App SDK.
 * MUST be called and awaited before React renders.
 *
 * Initializes:
 * - SDK core components
 * - Mini App ready signal
 * - Theme parameters
 * - Viewport expansion
 * - Back button
 *
 * @returns Promise that resolves when SDK is fully initialized
 */
export async function initializeTelegramSDK(): Promise<void> {
  try {
    // Initialize SDK
    init()

    // Mount Telegram components - await async operations
    if (miniApp.mount.isAvailable()) {
      miniApp.mount()
      miniApp.ready()
    }

    // Disabled theme mounting to force dark theme only
    // if (themeParams.mount.isAvailable()) {
    //   themeParams.mount()
    // }

    if (viewport.mount.isAvailable()) {
      // viewport.mount() can be async in some SDK versions
      const mountResult = viewport.mount()
      if (mountResult instanceof Promise) {
        await mountResult
      }

      // Expand to full height
      if (viewport.expand.isAvailable()) {
        viewport.expand()
      }
    }

    if (backButton.mount.isAvailable()) {
      backButton.mount()
    }

    // Mark SDK as fully initialized
    sdkInitialized = true

    console.info('Telegram Mini App SDK initialized successfully')
  } catch (error) {
    console.error('Failed to initialize Telegram SDK:', error)
    // Even on failure, mark as initialized to prevent indefinite waiting
    // The app can still function with degraded Telegram integration
    sdkInitialized = true
    // SDK initialization failure is not critical - app can still work
    // if opened directly with query parameters
  }
}
