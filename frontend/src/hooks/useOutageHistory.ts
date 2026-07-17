import { useCallback, useEffect, useRef, useState } from 'react'

import type { OutageDayBucket } from '../types'
import { deferMountFetch } from '../utils/deferMountFetch'
import { getSubComponentHistoryEndpoint } from '../utils/endpoints'

import useIntervalRefresh from './useIntervalRefresh'

interface UseOutageHistoryResult {
  buckets: OutageDayBucket[]
  loading: boolean
  error: string | null
}

const useOutageHistory = (
  componentName: string,
  subComponentName: string,
  days: number = 90,
): UseOutageHistoryResult => {
  const [buckets, setBuckets] = useState<OutageDayBucket[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const fetchHistory = useCallback(
    (silent: boolean) => {
      abortRef.current?.abort()
      const controller = new AbortController()
      abortRef.current = controller

      if (!silent) {
        setLoading(true)
        setError(null)
      }

      fetch(getSubComponentHistoryEndpoint(componentName, subComponentName, days), {
        signal: controller.signal,
      })
        .then((res) => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`)
          return res.json()
        })
        .then((data: OutageDayBucket[]) => {
          setBuckets(data ?? [])
          if (silent) {
            setError(null)
          }
        })
        .catch((err) => {
          if (err instanceof DOMException && err.name === 'AbortError') return
          if (!silent) {
            setError(err instanceof Error ? err.message : 'Failed to fetch outage history')
            setBuckets([])
          }
        })
        .finally(() => {
          if (!controller.signal.aborted && !silent) {
            setLoading(false)
          }
        })
    },
    [componentName, subComponentName, days],
  )

  useEffect(() => {
    deferMountFetch(() => {
      fetchHistory(false)
    })
    return () => {
      abortRef.current?.abort()
    }
  }, [fetchHistory])

  useIntervalRefresh(() => fetchHistory(true))

  return { buckets, loading, error }
}

export default useOutageHistory
