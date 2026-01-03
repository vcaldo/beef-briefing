/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string
  // New Relic Browser monitoring (optional)
  readonly VITE_NEW_RELIC_BROWSER_ACCOUNT_ID?: string
  readonly VITE_NEW_RELIC_BROWSER_APP_ID?: string
  readonly VITE_NEW_RELIC_BROWSER_LICENSE_KEY?: string
  readonly VITE_ENVIRONMENT?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
