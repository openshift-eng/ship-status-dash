import path from 'node:path'
import { fileURLToPath } from 'node:url'

import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

const LEGACY_PUBLIC = 'http://localhost:8080'
const DEFAULT_PUBLIC = 'http://localhost:8180'
const DEFAULT_PROTECTED = 'http://localhost:8443'
const DEFAULT_PROXY_TARGET = 'http://127.0.0.1:8443'
const DEFAULT_DEV_HOST = '::'

function viteCacheProfile(): string {
  const raw = process.env.SHIP_STATUS_VITE_CACHE_PROFILE?.trim()
  if (raw && /^[a-zA-Z0-9_-]{1,32}$/.test(raw)) {
    return raw
  }
  return 'local'
}

function resolvePublicDomain(mode: string, raw: string | undefined): string {
  const v = raw?.trim()
  if (v === LEGACY_PUBLIC) {
    return DEFAULT_PUBLIC
  }
  if (v) {
    return v
  }
  return mode === 'development' ? DEFAULT_PUBLIC : ''
}

function resolveProtectedDomain(mode: string, raw: string | undefined): string {
  const v = raw?.trim()
  if (v) {
    return v
  }
  return mode === 'development' ? DEFAULT_PROTECTED : ''
}

function resolveDevHost(raw: string | undefined): string {
  const host = raw?.trim()
  if (host) {
    return host
  }
  return DEFAULT_DEV_HOST
}

function resolveProxyTarget(raw: string | undefined): string {
  const port = Number(raw?.trim())
  if (Number.isInteger(port) && port > 0 && port <= 65535) {
    return `http://127.0.0.1:${port}`
  }
  return DEFAULT_PROXY_TARGET
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, __dirname, '')
  const vitePublic = resolvePublicDomain(mode, env.VITE_PUBLIC_DOMAIN)
  const viteProtected = resolveProtectedDomain(mode, env.VITE_PROTECTED_DOMAIN)
  const viteDevHost = resolveDevHost(env.VITE_DEV_HOST)
  const viteProxyTarget = resolveProxyTarget(env.VITE_PROXY_PORT)

  return {
    plugins: [react()],
    define: {
      'import.meta.env.VITE_PUBLIC_DOMAIN': JSON.stringify(vitePublic),
      'import.meta.env.VITE_PROTECTED_DOMAIN': JSON.stringify(viteProtected),
    },
    cacheDir: path.join(__dirname, 'node_modules', `.vite-${viteCacheProfile()}`),
    server: {
      // localhost resolves to ::1 first. Vite takes one host, so 127.0.0.1 refuses that
      // address. :: accepts both ::1 and 127.0.0.1. Set VITE_DEV_HOST to override.
      host: viteDevHost,
      port: 3030,
      proxy: {
        '/oauth': viteProxyTarget,
        '/api': viteProxyTarget,
      },
    },
    build: {
      outDir: 'build',
      chunkSizeWarningLimit: 2000,
    },
  }
})
