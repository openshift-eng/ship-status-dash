import { Box, CircularProgress, Container, Divider, Link, styled, Typography } from '@mui/material'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link as RouterLink } from 'react-router-dom'

import { FULL_OUTAGE_HISTORY_DAYS } from '../constants/history'
import useIntervalRefresh from '../hooks/useIntervalRefresh'
import useOutageHistory from '../hooks/useOutageHistory'
import type { Component } from '../types'
import { deferMountFetch } from '../utils/deferMountFetch'
import { getComponentsEndpoint } from '../utils/endpoints'

import OutageHistoryBar from './OutageHistoryBar'

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
  subComponentSlug: string
  subComponentName: string
}

const SubComponentHistoryBlock = ({
  componentSlug,
  subComponentSlug,
  subComponentName,
}: SubComponentHistoryBlockProps) => {
  const { buckets, loading, error } = useOutageHistory(
    componentSlug,
    subComponentSlug,
    FULL_OUTAGE_HISTORY_DAYS,
  )

  return (
    <SubComponentBlock data-tour="status-history-bar">
      <SubComponentHeader>
        <SubComponentLink
          component={RouterLink}
          to={`/${componentSlug}/${subComponentSlug}`}
          title={subComponentName}
        >
          {subComponentName}
        </SubComponentLink>
      </SubComponentHeader>
      {error ? (
        <Typography variant="caption" color="text.secondary">
          Could not load history
        </Typography>
      ) : (
        <OutageHistoryBar
          componentName={componentSlug}
          subComponentName={subComponentSlug}
          buckets={buckets}
          loading={loading}
          days={FULL_OUTAGE_HISTORY_DAYS}
        />
      )}
    </SubComponentBlock>
  )
}

const StatusHistoryPage = () => {
  const [components, setComponents] = useState<Component[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const fetchComponents = useCallback((silent: boolean) => {
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    if (!silent) {
      setLoading(true)
      setError(null)
    }

    fetch(getComponentsEndpoint(), { signal: controller.signal })
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json() as Promise<Component[]>
      })
      .then((data) => {
        if (controller.signal.aborted) {
          return
        }
        setComponents(data)
        if (silent) {
          setError(null)
        }
      })
      .catch((err) => {
        if (err instanceof DOMException && err.name === 'AbortError') {
          return
        }
        if (!silent) {
          setError(err instanceof Error ? err.message : 'Failed to load components')
        }
      })
      .finally(() => {
        if (!controller.signal.aborted && !silent) {
          setLoading(false)
        }
      })
  }, [])

  useEffect(() => {
    deferMountFetch(() => {
      fetchComponents(false)
    })
    return () => {
      abortRef.current?.abort()
    }
  }, [fetchComponents])

  useIntervalRefresh(() => fetchComponents(true))

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

  return (
    <PageContainer maxWidth="lg">
      <PageTitle variant="h4" data-tour="status-history-heading">
        Incident History
      </PageTitle>
      <PageNote>Uptime over the past {FULL_OUTAGE_HISTORY_DAYS} days.</PageNote>

      {components.map((component, i) => (
        <ComponentSection key={component.slug}>
          <ComponentTitle>{component.name}</ComponentTitle>
          {component.sub_components.map((sub) => (
            <SubComponentHistoryBlock
              key={sub.slug}
              componentSlug={component.slug}
              subComponentSlug={sub.slug}
              subComponentName={sub.name}
            />
          ))}
          {i < components.length - 1 && <Divider sx={{ mt: 3, mb: 3 }} />}
        </ComponentSection>
      ))}
    </PageContainer>
  )
}

export default StatusHistoryPage
