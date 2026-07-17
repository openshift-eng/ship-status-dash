import { useEffect, useRef } from 'react'

import { DASHBOARD_REFRESH_INTERVAL_MS } from '../constants/refresh'

/**
 * Invokes `callback` on a fixed interval while the tab is visible and `enabled` is true.
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

    let intervalId: ReturnType<typeof setInterval> | null = null

    const clear = () => {
      if (intervalId !== null) {
        clearInterval(intervalId)
        intervalId = null
      }
    }

    const start = () => {
      clear()
      intervalId = setInterval(() => {
        callbackRef.current()
      }, intervalMs)
    }

    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        start()
      } else {
        clear()
      }
    }

    if (document.visibilityState === 'visible') {
      start()
    }

    document.addEventListener('visibilitychange', handleVisibility)
    return () => {
      clear()
      document.removeEventListener('visibilitychange', handleVisibility)
    }
  }, [intervalMs, enabled])
}

export default useIntervalRefresh
