const DEFAULT_DASHBOARD_REFRESH_INTERVAL_MS = 10 * 60 * 1000

function resolveRefreshIntervalMs(): number {
  const raw = import.meta.env.VITE_DASHBOARD_REFRESH_INTERVAL_MS
  if (raw === undefined || raw === '') {
    return DEFAULT_DASHBOARD_REFRESH_INTERVAL_MS
  }
  const parsed = Number(raw)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return DEFAULT_DASHBOARD_REFRESH_INTERVAL_MS
  }
  return parsed
}

/** How often active dashboard pages re-fetch live data. Overridable via VITE_DASHBOARD_REFRESH_INTERVAL_MS. */
export const DASHBOARD_REFRESH_INTERVAL_MS = resolveRefreshIntervalMs()
