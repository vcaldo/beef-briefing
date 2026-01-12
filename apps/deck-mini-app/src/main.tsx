// Initialize New Relic Browser monitoring (must be early, before React)
// This import triggers auto-initialization if configured
import '@beef-briefing/shared-mini-app/monitoring'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { initializeTelegramSDK } from '@beef-briefing/shared-mini-app/initialization'

import App from './App'
import { ErrorBoundary } from '@beef-briefing/shared-mini-app/components'
import './styles/global.css'

// Initialize app
async function initApp() {
  // Initialize Telegram Mini App SDK
  initializeTelegramSDK()

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
