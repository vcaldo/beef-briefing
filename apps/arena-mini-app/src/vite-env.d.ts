/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string
  readonly VITE_CARD_API_URL?: string
  // Sound system base URL (object storage)
  readonly VITE_SOUND_BASE_URL?: string
  // Sound version for cache busting (bump when sounds change)
  readonly VITE_SOUND_VERSION?: string
  // New Relic Browser monitoring (optional)
  readonly VITE_NEW_RELIC_BROWSER_ACCOUNT_ID?: string
  readonly VITE_NEW_RELIC_BROWSER_APP_ID?: string
  readonly VITE_NEW_RELIC_BROWSER_LICENSE_KEY?: string
  readonly VITE_ENVIRONMENT?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

// CSS module declarations
declare module '*.css' {
  const classes: { readonly [key: string]: string }
  export default classes
}
