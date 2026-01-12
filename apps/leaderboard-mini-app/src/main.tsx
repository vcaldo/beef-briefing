// Initialize New Relic Browser monitoring (must be early, before React)
// This import triggers auto-initialization if configured
import './newrelic'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import {
  init,
  miniApp,
  themeParams,
  viewport,
  backButton,
} from '@telegram-apps/sdk-react'

import App from './App'
import { ErrorBoundary } from './components/ErrorBoundary'
import './styles/global.css'

// Initialize Telegram Mini App SDK
async function initApp() {
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
  } catch (error) {
    console.error('Failed to initialize Telegram SDK:', error)
  }

  // Render app
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <ErrorBoundary name="root" onReset={() => window.location.reload()}>
        <App />
      </ErrorBoundary>
    </StrictMode>
  )
}

initApp()
