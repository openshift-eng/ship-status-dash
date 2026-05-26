import {
  Box,
  CircularProgress,
  Container,
  Divider,
  Link,
  styled,
  Typography,
  useTheme,
} from '@mui/material'
import { useEffect, useState } from 'react'
import { Link as RouterLink } from 'react-router-dom'

import { FULL_OUTAGE_HISTORY_DAYS } from '../constants/history'
import useOutageHistory from '../hooks/useOutageHistory'
import type { Component, ComponentStatus, SubComponent } from '../types'
import { getComponentsEndpoint, getOverallStatusEndpoint } from '../utils/endpoints'
import { formatStatusSeverityText, getStatusChipColor } from '../utils/helpers'

import OutageHistoryBar from './OutageHistoryBar'
import { StatusChip } from './StatusColors'

const PageContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
  marginBottom: theme.spacing(4),
}))

const PageTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 700,
  marginBottom: theme.spacing(0.5),
}))

const PageNote = styled(Typography)(({ theme }) => ({
  color: theme.palette.text.secondary,
  fontSize: '0.8rem',
  marginBottom: theme.spacing(2),
}))

const StatusBanner = styled(Box)<{ bannercolor: string }>(({ theme, bannercolor }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: theme.spacing(1.5, 2),
  marginBottom: theme.spacing(4),
  borderRadius: theme.spacing(1),
  backgroundColor: bannercolor,
  color: theme.palette.getContrastText(bannercolor),
}))

const ComponentSection = styled(Box)(({ theme }) => ({
  marginBottom: theme.spacing(2),
}))

const ComponentTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  marginBottom: theme.spacing(1),
  color: theme.palette.text.secondary,
  textTransform: 'uppercase',
  fontSize: '0.75rem',
  letterSpacing: '0.08em',
}))

const SubComponentBlock = styled(Box)(({ theme }) => ({
  padding: theme.spacing(2, 0),
  borderTop: `1px solid ${theme.palette.divider}`,
  '&:first-of-type': {
    borderTop: 'none',
  },
}))

const SubComponentHeader = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: 8,
}))

const SubComponentLink = styled(Link)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '0.95rem',
  color: theme.palette.text.primary,
  textDecoration: 'none',
  cursor: 'pointer',
  '&:hover': {
    textDecoration: 'underline',
    color: theme.palette.primary.main,
  },
}))

const LoadingBox = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'center',
  alignItems: 'center',
  minHeight: '200px',
}))

interface SubComponentHistoryBlockProps {
  componentSlug: string
  subComponent: SubComponent
}

const SubComponentHistoryBlock = ({
  componentSlug,
  subComponent,
}: SubComponentHistoryBlockProps) => {
  const { outages, loading, error } = useOutageHistory(
    componentSlug,
    subComponent.slug,
    FULL_OUTAGE_HISTORY_DAYS,
  )

  return (
    <SubComponentBlock>
      <SubComponentHeader>
        <SubComponentLink
          component={RouterLink}
          to={`/${componentSlug}/${subComponent.slug}`}
          title={subComponent.name}
        >
          {subComponent.name}
        </SubComponentLink>
        {subComponent.status && (
          <StatusChip
            label={formatStatusSeverityText(subComponent.status)}
            status={subComponent.status}
            size="small"
            variant="filled"
          />
        )}
      </SubComponentHeader>
      {error ? (
        <Typography variant="caption" color="text.secondary">
          Could not load history
        </Typography>
      ) : (
        <OutageHistoryBar
          componentName={componentSlug}
          subComponentName={subComponent.slug}
          outages={outages}
          loading={loading}
          days={FULL_OUTAGE_HISTORY_DAYS}
        />
      )}
    </SubComponentBlock>
  )
}

const STATUS_PRIORITY = ['Unknown', 'Suspected', 'Partial', 'Degraded', 'CapacityExhausted', 'Down']
const HEALTHY_STATUSES = new Set(['Healthy', 'Unknown'])

const StatusHistoryPage = () => {
  const theme = useTheme()
  const [components, setComponents] = useState<Component[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([
      fetch(getComponentsEndpoint()).then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json() as Promise<Component[]>
      }),
      fetch(getOverallStatusEndpoint()).then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json() as Promise<ComponentStatus[]>
      }),
    ])
      .then(([componentsData, statusData]) => {
        // Build a map of componentName -> subComponentSlug -> worst active outage severity.
        // ComponentStatus uses display names and sub_component_name is the slug.
        const subStatusMap = new Map<string, Map<string, string>>()
        for (const cs of statusData) {
          const subMap = new Map<string, string>()
          for (const outage of cs.active_outages ?? []) {
            const current = subMap.get(outage.sub_component_name)
            const currentIdx = current ? STATUS_PRIORITY.indexOf(current) : -1
            const severityIdx = STATUS_PRIORITY.indexOf(outage.severity)
            if (severityIdx === -1) {
              // Unknown severity: still an active outage — record only if nothing is set yet.
              if (current === undefined) subMap.set(outage.sub_component_name, outage.severity)
            } else if (severityIdx > currentIdx) {
              subMap.set(outage.sub_component_name, outage.severity)
            }
          }
          subStatusMap.set(cs.component_name, subMap)
        }

        // Merge derived sub-component statuses. Default to Healthy when no active outage.
        return componentsData.map((c) => ({
          ...c,
          sub_components: c.sub_components.map((s) => ({
            ...s,
            status: (subStatusMap.get(c.name)?.get(s.slug) ?? 'Healthy') as SubComponent['status'],
          })),
        }))
      })
      .then((data) => setComponents(data))
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load components'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <PageContainer maxWidth="lg">
        <LoadingBox>
          <CircularProgress />
        </LoadingBox>
      </PageContainer>
    )
  }

  if (error) {
    return (
      <PageContainer maxWidth="lg">
        <Typography color="error">{error}</Typography>
      </PageContainer>
    )
  }

  const affectedSubs = components.flatMap((c) =>
    c.sub_components.filter((s) => s.status && !HEALTHY_STATUSES.has(s.status)),
  )
  const affectedCount = affectedSubs.length
  const allHealthy = affectedCount === 0

  const worstStatus = affectedSubs.reduce((worst, s) => {
    if (!s.status) return worst
    const currentIndex = STATUS_PRIORITY.indexOf(s.status)
    const worstIndex = STATUS_PRIORITY.indexOf(worst)
    return currentIndex > worstIndex ? s.status : worst
  }, 'Healthy')
  const bannerColor = getStatusChipColor(theme, worstStatus)

  return (
    <PageContainer maxWidth="lg">
      <PageTitle variant="h4">Status History</PageTitle>
      <PageNote>Uptime over the past {FULL_OUTAGE_HISTORY_DAYS} days.</PageNote>

      <StatusBanner bannercolor={bannerColor}>
        <Typography variant="body1" fontWeight={600}>
          {allHealthy
            ? 'All Systems Operational'
            : `${affectedCount} ${affectedCount === 1 ? 'service' : 'services'} experiencing issues`}
        </Typography>
      </StatusBanner>

      {components.map((component, i) => (
        <ComponentSection key={component.slug}>
          <ComponentTitle>{component.name}</ComponentTitle>
          {component.sub_components.map((sub) => (
            <SubComponentHistoryBlock
              key={sub.slug}
              componentSlug={component.slug}
              subComponent={sub}
            />
          ))}
          {i < components.length - 1 && <Divider sx={{ mt: 3, mb: 3 }} />}
        </ComponentSection>
      ))}
    </PageContainer>
  )
}

export default StatusHistoryPage
