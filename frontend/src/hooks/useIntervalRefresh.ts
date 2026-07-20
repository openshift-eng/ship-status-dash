import { useEffect, useRef } from 'react'

import { DASHBOARD_REFRESH_INTERVAL_MS } from '../constants/refresh'

/**
 * Invokes `callback` after `intervalMs` of wall-clock time while the tab is visible
 * and `enabled` is true. Time continues to elapse while the tab is hidden; when the
 * tab becomes visible again, any overdue refresh runs immediately.
 * Does not invoke immediately on mount; pair with a separate mount fetch.
 */
const useIntervalRefresh = (
  callback: () => void,
  intervalMs: number = DASHBOARD_REFRESH_INTERVAL_MS,
  enabled: boolean = true,
): void => {
  const callbackRef = useRef(callback)

  useEffect(() => {
    callbackRef.current = callback
  }, [callback])

  useEffect(() => {
    if (!enabled || intervalMs <= 0) {
      return
    }

    let lastRefreshAt = Date.now()
    let timeoutId: ReturnType<typeof setTimeout> | null = null

    const clear = () => {
      if (timeoutId !== null) {
        clearTimeout(timeoutId)
        timeoutId = null
      }
    }

    const schedule = () => {
      clear()
      if (document.visibilityState !== 'visible') {
        return
      }

      const remaining = Math.max(0, intervalMs - (Date.now() - lastRefreshAt))
      timeoutId = setTimeout(() => {
        if (document.visibilityState !== 'visible') {
          return
        }
        lastRefreshAt = Date.now()
        callbackRef.current()
        schedule()
      }, remaining)
    }

    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        schedule()
      } else {
        clear()
      }
    }

    if (document.visibilityState === 'visible') {
      schedule()
    }

    document.addEventListener('visibilitychange', handleVisibility)
    return () => {
      clear()
      document.removeEventListener('visibilitychange', handleVisibility)
    }
  }, [intervalMs, enabled])
}

export default useIntervalRefresh
