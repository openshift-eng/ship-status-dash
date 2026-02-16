import { Alert, Box, CircularProgress, styled, Typography } from '@mui/material'
import { useEffect, useState } from 'react'

import type { SubComponent, SubComponentListParams, SubComponentListItem } from '../../types'
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

  useEffect(() => {
    let cancelled = false
    queueMicrotask(() => {
      if (!cancelled) {
        setLoading(true)
        setError(null)
      }
    })
    fetch(getListSubComponentsEndpoint(filters))
      .then((res) => {
        if (!res.ok) throw new Error('Failed to load sub-components')
        return res.json()
      })
      .then((data) => {
        if (!cancelled) setItems(data ?? [])
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load sub-components')
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [filters])

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
