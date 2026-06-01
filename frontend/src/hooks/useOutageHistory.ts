import { useEffect, useState } from 'react'

import type { OutageDayBucket } from '../types'
import { getSubComponentHistoryEndpoint } from '../utils/endpoints'

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

  useEffect(() => {
    const controller = new AbortController()
    // Reset before each fetch so stale data isn't shown as settled while the new request is in flight.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true)
    setError(null)

    fetch(getSubComponentHistoryEndpoint(componentName, subComponentName, days), {
      signal: controller.signal,
    })
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then((data: OutageDayBucket[]) => {
        setBuckets(data ?? [])
      })
      .catch((err) => {
        if (err instanceof DOMException && err.name === 'AbortError') return
        setError(err instanceof Error ? err.message : 'Failed to fetch outage history')
        setBuckets([])
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false)
        }
      })

    return () => controller.abort()
  }, [componentName, subComponentName, days])

  return { buckets, loading, error }
}

export default useOutageHistory
