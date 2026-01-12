/**
 * Telegram Mini App SDK initialization.
 * Centralizes common initialization logic for all Mini Apps.
 */

import {
  init,
  miniApp,
  themeParams,
  viewport,
  backButton,
} from '@telegram-apps/sdk-react'

/**
 * Initialize Telegram Mini App SDK.
 * Should be called early in the application (before React renders).
 *
 * Initializes:
 * - SDK core components
 * - Mini App ready signal
 * - Theme parameters
 * - Viewport expansion
 * - Back button
 */
export function initializeTelegramSDK(): void {
  try {
    // Initialize SDK
    init()

    // Mount Telegram components
    if (miniApp.mount.isAvailable()) {
      miniApp.mount()
      miniApp.ready()
    }

    if (themeParams.mount.isAvailable()) {
      themeParams.mount()
    }

    if (viewport.mount.isAvailable()) {
      viewport.mount()
      // Expand to full height
      if (viewport.expand.isAvailable()) {
        viewport.expand()
      }
    }

    if (backButton.mount.isAvailable()) {
      backButton.mount()
    }

    console.info('Telegram Mini App SDK initialized successfully')
  } catch (error) {
    console.error('Failed to initialize Telegram SDK:', error)
    // SDK initialization failure is not critical - app can still work
    // if opened directly with query parameters
  }
}
