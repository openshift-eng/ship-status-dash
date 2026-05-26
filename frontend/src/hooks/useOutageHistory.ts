import { useEffect, useState } from 'react'

import type { Outage } from '../types'
import { getOutagesDuringEndpoint } from '../utils/endpoints'

interface UseOutageHistoryResult {
  outages: Outage[]
  loading: boolean
  error: string | null
}

const useOutageHistory = (
  componentName: string,
  subComponentName?: string,
  days: number = 90,
): UseOutageHistoryResult => {
  const [outages, setOutages] = useState<Outage[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const end = new Date()
    const start = new Date(end)
    start.setDate(start.getDate() - days)

    fetch(getOutagesDuringEndpoint(componentName, subComponentName, start, end))
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then((data: Outage[]) => {
        setOutages(data ?? [])
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to fetch outage history')
        setOutages([])
      })
      .finally(() => {
        setLoading(false)
      })
  }, [componentName, subComponentName, days])

  return { outages, loading, error }
}

export default useOutageHistory
