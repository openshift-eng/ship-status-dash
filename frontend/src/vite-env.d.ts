/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_PUBLIC_DOMAIN: string
  readonly VITE_PROTECTED_DOMAIN: string
  /** Optional poll interval in ms for live dashboard refreshes. Defaults to 300000 (5 minutes). */
  readonly VITE_DASHBOARD_REFRESH_INTERVAL_MS?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
