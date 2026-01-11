// Initialize New Relic Browser monitoring FIRST (must be early, before React)
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

    console.info('Telegram Mini App SDK initialized successfully')
  } catch (error) {
    console.error('Failed to initialize Telegram SDK:', error)
    // SDK initialization failure is not critical - app can still work
    // if opened directly with query parameters
  }

  // Render app
  try {
    const rootElement = document.getElementById('root')
    if (!rootElement) {
      console.error('Root element not found - cannot render app')
      throw new Error('Root element not found')
    }

    createRoot(rootElement).render(
      <StrictMode>
        <ErrorBoundary name="root" onReset={() => window.location.reload()}>
          <App />
        </ErrorBoundary>
      </StrictMode>
    )
  } catch (error) {
    console.error('Failed to render app:', error)
    // Fallback: show error message directly
    const root = document.getElementById('root')
    if (root) {
      root.innerHTML =
        '<div style="display:flex;align-items:center;justify-content:center;height:100vh;background:#0a0a0f;color:#ff4757;font-family:system-ui;text-align:center;padding:20px">' +
        '<div><div style="font-size:48px;margin-bottom:20px">!</div>' +
        '<div style="font-size:18px">Failed to load Beef Arena</div>' +
        '<div style="font-size:14px;color:#a0a0b0;margin-top:10px">Please refresh the page</div>' +
        '</div></div>'
    }
  }
}

initApp()
