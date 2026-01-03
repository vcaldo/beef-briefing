/**
 * New Relic Browser Agent initialization.
 * Follows the pattern from other services: only initializes if configured.
 */
import { BrowserAgent } from '@newrelic/browser-agent/loaders/browser-agent'

// Browser-specific configuration from Vite build-time environment variables
const config = {
  accountId: import.meta.env.VITE_NEW_RELIC_BROWSER_ACCOUNT_ID,
  applicationId: import.meta.env.VITE_NEW_RELIC_BROWSER_APP_ID,
  licenseKey: import.meta.env.VITE_NEW_RELIC_BROWSER_LICENSE_KEY,
  environment: import.meta.env.VITE_ENVIRONMENT || 'development',
}

/**
 * Check if New Relic Browser monitoring is configured.
 * Returns true only if all required configuration values are present.
 */
export function isNewRelicEnabled(): boolean {
  return !!(config.accountId && config.applicationId && config.licenseKey)
}

// Store the agent instance for later use
let browserAgent: BrowserAgent | null = null

/**
 * Initialize New Relic Browser agent.
 * Should be called early in the application lifecycle (before React renders).
 * Returns the agent instance or null if not configured.
 */
function initNewRelic(): BrowserAgent | null {
  if (!isNewRelicEnabled()) {
    if (import.meta.env.DEV) {
      console.debug(
        'New Relic Browser monitoring not configured, skipping initialization'
      )
    }
    return null
  }

  try {
    const agent = new BrowserAgent({
      init: {
        distributed_tracing: { enabled: true },
        privacy: { cookies_enabled: true },
        ajax: { deny_list: [] },
        spa: { enabled: true },
      },
      info: {
        beacon: 'bam.eu01.nr-data.net',
        errorBeacon: 'bam.eu01.nr-data.net',
        licenseKey: config.licenseKey!,
        applicationID: config.applicationId!,
        sa: 1,
      },
      loader_config: {
        accountID: config.accountId!,
        trustKey: config.accountId!,
        agentID: config.applicationId!,
        licenseKey: config.licenseKey!,
        applicationID: config.applicationId!,
      },
    })

    console.info(
      `New Relic Browser monitoring initialized (app: ${config.applicationId}, env: ${config.environment})`
    )

    return agent
  } catch (error) {
    console.error('Failed to initialize New Relic Browser agent:', error)
    return null
  }
}

/**
 * Get the New Relic Browser agent instance.
 * Returns null if not initialized.
 */
export function getNewRelicAgent(): BrowserAgent | null {
  return browserAgent
}

/**
 * Set a custom attribute on the current page view.
 * No-op if New Relic is not enabled.
 */
export function setCustomAttribute(
  name: string,
  value: string | number | boolean
): void {
  if (browserAgent) {
    browserAgent.setCustomAttribute(name, value)
  }
}

/**
 * Record a custom page action.
 * No-op if New Relic is not enabled.
 */
export function addPageAction(
  name: string,
  attributes?: Record<string, unknown>
): void {
  if (browserAgent) {
    browserAgent.addPageAction(name, attributes)
  }
}

/**
 * Notify New Relic of a JavaScript error.
 * No-op if New Relic is not enabled.
 */
export function noticeError(
  error: Error,
  customAttributes?: Record<string, unknown>
): void {
  if (browserAgent) {
    browserAgent.noticeError(error, customAttributes)
  }
}

// Auto-initialize on module load
browserAgent = initNewRelic()
