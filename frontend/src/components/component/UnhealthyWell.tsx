import { Alert, Box, Card, CardContent, styled, Typography } from '@mui/material'
import React, { useEffect, useMemo, useState } from 'react'

import type { Status, SubComponent, SubComponentListItem } from '../../types'
import { getListSubComponentsEndpoint } from '../../utils/endpoints'
import { getStatusChipColor } from '../../utils/helpers'
import { getStatusTintStyles } from '../../utils/styles'
import SubComponentCard from '../sub-component/SubComponentCard'

/** Sub-component statuses that are not Healthy (Partial is component-level only). */
export const UNHEALTHY_STATUSES: Status[] = ['Degraded', 'Down', 'Suspected', 'CapacityExhausted']

const STATUS_RANK: Record<string, number> = {
  Down: 5,
  CapacityExhausted: 4,
  Degraded: 3,
  Suspected: 2,
  Unknown: 1,
  Healthy: 0,
}

const Section = styled(Box)(({ theme }) => ({
  marginBottom: theme.spacing(5),
  paddingBottom: theme.spacing(5),
  borderBottom: `2px solid ${theme.palette.divider}`,
}))

const Well = styled(Card)(({ theme }) => ({
  ...getStatusTintStyles(theme, 'Down', 2),
  borderRadius: theme.spacing(2),
  border: `2px solid ${getStatusChipColor(theme, 'Down')}66`,
}))

const SubComponentsGrid = styled(Box)(({ theme }) => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
  gap: theme.spacing(2),
  marginTop: theme.spacing(2),
}))

const HeaderBox = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  marginBottom: theme.spacing(2),
  paddingBottom: theme.spacing(2),
  borderBottom: `1px solid ${theme.palette.divider}`,
}))

const TitleGroup = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'flex-start',
  gap: theme.spacing(0.5),
  minWidth: 0,
}))

const WellTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1.5rem',
  color: theme.palette.text.primary,
}))

const WellDescription = styled(Typography)(({ theme }) => ({
  fontSize: '0.95rem',
  color: theme.palette.text.secondary,
}))

const CardWrapper = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  gap: theme.spacing(0.75),
}))

const ComponentLabel = styled(Typography)(({ theme }) => ({
  fontSize: '0.75rem',
  fontWeight: 600,
  color: theme.palette.text.secondary,
  textTransform: 'uppercase',
  letterSpacing: '0.04em',
}))

const toSubComponent = (item: SubComponentListItem): SubComponent => {
  const rest = Object.fromEntries(
    Object.entries(item).filter(([key]) => key !== 'component_name'),
  ) as SubComponent
  return rest
}

interface UnhealthyWellProps {
  onHasOutagesChange?: (hasOutages: boolean) => void
}

const UnhealthyWell: React.FC<UnhealthyWellProps> = ({ onHasOutagesChange }) => {
  const [items, setItems] = useState<SubComponentListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    fetch(getListSubComponentsEndpoint({ status: UNHEALTHY_STATUSES }), {
      signal: controller.signal,
    })
      .then((res) => {
        if (!res.ok) throw new Error('Failed to load unhealthy sub-components')
        return res.json()
      })
      .then((data: SubComponentListItem[]) => {
        if (!controller.signal.aborted) setItems(data ?? [])
      })
      .catch((err) => {
        if (err instanceof DOMException && err.name === 'AbortError') return
        if (!controller.signal.aborted) {
          setError(err instanceof Error ? err.message : 'Failed to load unhealthy sub-components')
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [])

  useEffect(() => {
    if (loading) return
    onHasOutagesChange?.(!error && items.length > 0)
  }, [loading, error, items.length, onHasOutagesChange])

  useEffect(() => {
    return () => onHasOutagesChange?.(false)
  }, [onHasOutagesChange])

  const sortedItems = useMemo(() => {
    return [...items].sort((a, b) => {
      const rankDiff = (STATUS_RANK[b.status ?? ''] ?? 0) - (STATUS_RANK[a.status ?? ''] ?? 0)
      if (rankDiff !== 0) return rankDiff
      const componentDiff = a.component_name.localeCompare(b.component_name)
      if (componentDiff !== 0) return componentDiff
      return a.name.localeCompare(b.name)
    })
  }, [items])

  if (error) {
    return <Alert severity="error">{error}</Alert>
  }

  if (loading || sortedItems.length === 0) {
    return null
  }

  return (
    <Section data-tour="unhealthy-well">
      <Well>
        <CardContent>
          <HeaderBox>
            <TitleGroup>
              <WellTitle>In Outage</WellTitle>
              <WellDescription>Sub-components that are not currently healthy.</WellDescription>
            </TitleGroup>
          </HeaderBox>

          <SubComponentsGrid>
            {sortedItems.map((item) => (
              <CardWrapper key={`${item.component_name}-${item.name}`}>
                <ComponentLabel>{item.component_name}</ComponentLabel>
                <SubComponentCard
                  subComponent={toSubComponent(item)}
                  componentName={item.component_name}
                />
              </CardWrapper>
            ))}
          </SubComponentsGrid>
        </CardContent>
      </Well>
    </Section>
  )
}

export default UnhealthyWell
