import { Alert, Box, CircularProgress, styled, Typography } from '@mui/material'
import { useCallback, useEffect, useRef, useState } from 'react'

import useIntervalRefresh from '../../hooks/useIntervalRefresh'
import type { SubComponent, SubComponentListParams, SubComponentListItem } from '../../types'
import { deferMountFetch } from '../../utils/deferMountFetch'
import { getListSubComponentsEndpoint } from '../../utils/endpoints'

import SubComponentCard from './SubComponentCard'

const SubComponentsGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
  gap: theme.spacing(3),
}))

const LoadingBox = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  minHeight: '200px',
}))

function toSubComponent(item: SubComponentListItem): SubComponent {
  const rest = Object.fromEntries(
    Object.entries(item).filter(([key]) => key !== 'component_name'),
  ) as SubComponent
  return rest
}

interface SubComponentListProps {
  filters: SubComponentListParams
}

const SubComponentList = ({ filters }: SubComponentListProps) => {
  const [items, setItems] = useState<SubComponentListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  // Stable across identical filter values even when parents pass a new object each render.
  const filterKey = JSON.stringify(
    Object.entries(filters as Record<string, unknown>)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, value]) => [key, value ?? null]),
  )

  const fetchItems = useCallback(
    (silent: boolean) => {
      abortRef.current?.abort()
      const controller = new AbortController()
      abortRef.current = controller

      if (!silent) {
        setLoading(true)
        setError(null)
      }

      fetch(getListSubComponentsEndpoint(filters), { signal: controller.signal })
        .then((res) => {
          if (!res.ok) throw new Error('Failed to load sub-components')
          return res.json()
        })
        .then((data) => {
          if (controller.signal.aborted) {
            return
          }
          setItems(data ?? [])
          if (silent) {
            setError(null)
          }
        })
        .catch((err) => {
          if (err instanceof DOMException && err.name === 'AbortError') {
            return
          }
          if (!silent) {
            setError(err instanceof Error ? err.message : 'Failed to load sub-components')
          }
        })
        .finally(() => {
          if (!controller.signal.aborted && !silent) {
            setLoading(false)
          }
        })
    },
    // filterKey stands in for filters so inline filter objects from parents do not refetch every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [filterKey],
  )

  useEffect(() => {
    deferMountFetch(() => {
      fetchItems(false)
    })
    return () => {
      abortRef.current?.abort()
    }
  }, [fetchItems])

  useIntervalRefresh(() => fetchItems(true))

  return (
    <>
      {loading && (
        <LoadingBox>
          <CircularProgress />
        </LoadingBox>
      )}

      {error && <Alert severity="error">{error}</Alert>}

      {!loading && !error && (
        <>
          {items.length === 0 ? (
            <Typography color="text.secondary">
              No sub-components match the current filters.
            </Typography>
          ) : (
            <SubComponentsGrid>
              {items.map((item) => (
                <SubComponentCard
                  key={`${item.component_name}-${item.name}`}
                  subComponent={toSubComponent(item)}
                  componentName={item.component_name}
                />
              ))}
            </SubComponentsGrid>
          )}
        </>
      )}
    </>
  )
}

export default SubComponentList
