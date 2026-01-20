/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_PUBLIC_DOMAIN: string
  readonly VITE_PROTECTED_DOMAIN: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
