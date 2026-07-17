import { Box, Card, CardContent, Tooltip, Typography, styled } from '@mui/material'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { CARD_OUTAGE_HISTORY_DAYS } from '../../constants/history'
import { useTags } from '../../contexts/TagsContext'
import useIntervalRefresh from '../../hooks/useIntervalRefresh'
import useOutageHistory from '../../hooks/useOutageHistory'
import type { SubComponent } from '../../types'
import { deferMountFetch } from '../../utils/deferMountFetch'
import { getSubComponentStatusEndpoint } from '../../utils/endpoints'
import { formatStatusSeverityText } from '../../utils/helpers'
import { deslugify, slugify } from '../../utils/slugify'
import { getStatusTintStyles } from '../../utils/styles'
import OutageHistoryBar from '../OutageHistoryBar'
import { StatusChip } from '../StatusColors'
import TagChip from '../tags/TagChip'

import MonitoredChip from './MonitoredChip'

const SubComponentCard = styled(Card)<{ status: string }>(({ theme, status }) => ({
  ...getStatusTintStyles(theme, status, 1.5),
  ...(theme.palette.mode === 'dark' && { backgroundColor: theme.palette.grey[900] }),
  borderRadius: theme.spacing(1.5),
  cursor: 'pointer',
  transition: 'all 0.2s ease-in-out',
  minHeight: '160px',
  display: 'flex',
  flexDirection: 'column',
  '&:hover': {
    boxShadow: theme.shadows[4],
    transform: 'translateY(-1px)',
    '& .MuiChip-root': {
      opacity: 0.9,
    },
  },
}))

const StyledCardContent = styled(CardContent)(({ theme }) => ({
  padding: theme.spacing(2.5),
  flex: 1,
  display: 'flex',
  flexDirection: 'column',
  '&:last-child': {
    paddingBottom: theme.spacing(2.5),
  },
}))

const CardHeader = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  marginBottom: theme.spacing(1.5),
}))

const SubComponentTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1rem',
  color: theme.palette.text.primary,
  flex: 1,
  marginRight: theme.spacing(1),
}))

const SubComponentDescription = styled(Typography)(({ theme }) => ({
  fontSize: '0.875rem',
  color: theme.palette.text.secondary,
  lineHeight: 1.5,
  flex: 1,
  marginBottom: theme.spacing(1),
}))

const StatusChipBox = styled(Box)(() => ({
  flexShrink: 0,
}))

const CardFooter = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  marginTop: theme.spacing(1.5),
  paddingTop: theme.spacing(1.5),
  borderTop: `1px solid ${theme.palette.divider}`,
}))

const HistoryBarWrapper = styled(Box)(({ theme }) => ({
  marginTop: theme.spacing(1.5),
  paddingTop: theme.spacing(1.5),
  borderTop: `1px solid ${theme.palette.divider}`,
}))

const MonitoredChipRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  marginTop: theme.spacing(1),
}))

const TagsContainer = styled(Box)(({ theme }) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: theme.spacing(0.5),
  flex: 1,
}))

interface SubComponentCardProps {
  subComponent: SubComponent
  componentName: string
}

const SubComponentCardComponent = ({ subComponent, componentName }: SubComponentCardProps) => {
  const navigate = useNavigate()
  const { getTag } = useTags()
  const [subComponentWithStatus, setSubComponentWithStatus] = useState<SubComponent>(subComponent)
  const [lastPingTime, setLastPingTime] = useState<string | null | undefined>(undefined)
  const [loading, setLoading] = useState(true)
  const { buckets: historyBuckets, loading: historyLoading } = useOutageHistory(
    componentName,
    subComponent.name,
    CARD_OUTAGE_HISTORY_DAYS,
  )
  const abortRef = useRef<AbortController | null>(null)
  const subComponentRef = useRef(subComponent)
  const subComponentName = subComponent.name

  useEffect(() => {
    subComponentRef.current = subComponent
  }, [subComponent])

  const fetchStatus = useCallback(
    (silent: boolean) => {
      abortRef.current?.abort()
      const controller = new AbortController()
      abortRef.current = controller
      const current = subComponentRef.current

      if (!silent) {
        setSubComponentWithStatus(current)
        setLastPingTime(undefined)
        setLoading(true)
      }

      fetch(getSubComponentStatusEndpoint(componentName, subComponentName), {
        signal: controller.signal,
      })
        .then((res) => {
          if (!res.ok) {
            throw new Error(`Failed to fetch status: ${res.statusText}`)
          }
          return res.json()
        })
        .then((subStatus) => {
          setSubComponentWithStatus({
            ...current,
            status: subStatus.status,
            active_outages: subStatus.active_outages,
          })
          setLastPingTime(subStatus.last_ping_time ?? null)
        })
        .catch((err) => {
          if (err instanceof DOMException && err.name === 'AbortError') {
            return
          }
          if (!silent) {
            setSubComponentWithStatus({
              ...current,
              status: 'Unknown',
              active_outages: [],
            })
            setLastPingTime(undefined)
          }
        })
        .finally(() => {
          if (!controller.signal.aborted && !silent) {
            setLoading(false)
          }
        })
    },
    [componentName, subComponentName],
  )

  useEffect(() => {
    deferMountFetch(() => {
      fetchStatus(false)
    })
    return () => {
      abortRef.current?.abort()
    }
  }, [fetchStatus])

  useIntervalRefresh(() => fetchStatus(true))

  const handleClick = () => {
    const status = subComponentWithStatus.status || 'Unknown'
    const activeOutages = subComponentWithStatus.active_outages || []
    const isHealthy = status === 'Healthy' || activeOutages.length === 0

    if (isHealthy || activeOutages.length > 1) {
      navigate(`/${slugify(componentName)}/${slugify(subComponentWithStatus.name)}`)
    } else if (activeOutages.length === 1) {
      navigate(
        `/${slugify(componentName)}/${slugify(subComponentWithStatus.name)}/outages/${activeOutages[0].ID}`,
      )
    }
  }

  const cardContent = (
    <SubComponentCard
      status={subComponentWithStatus.status || 'Unknown'}
      onClick={handleClick}
      data-tour="subcomponent-card"
    >
      <StyledCardContent>
        <CardHeader>
          <SubComponentTitle>{deslugify(subComponent.name)}</SubComponentTitle>
          <StatusChipBox>
            <StatusChip
              label={
                loading
                  ? 'Loading...'
                  : formatStatusSeverityText(subComponentWithStatus.status || 'Unknown')
              }
              status={subComponentWithStatus.status || 'Unknown'}
              size="small"
              variant="filled"
            />
          </StatusChipBox>
        </CardHeader>
        <SubComponentDescription>{subComponent.description}</SubComponentDescription>
        {subComponent.tags && subComponent.tags.length > 0 && (
          <CardFooter data-tour="subcomponent-tags">
            <TagsContainer>
              {subComponent.tags.map((tag) => (
                <TagChip key={tag} tag={tag} size="small" color={getTag(tag)?.color} />
              ))}
            </TagsContainer>
          </CardFooter>
        )}
        {subComponent.monitoring && (
          <MonitoredChipRow>
            <MonitoredChip
              monitoring={subComponent.monitoring}
              lastPingTime={lastPingTime}
              size="small"
            />
          </MonitoredChipRow>
        )}
        <HistoryBarWrapper>
          <OutageHistoryBar
            componentName={componentName}
            subComponentName={subComponent.name}
            buckets={historyBuckets}
            loading={historyLoading}
            days={CARD_OUTAGE_HISTORY_DAYS}
          />
        </HistoryBarWrapper>
      </StyledCardContent>
    </SubComponentCard>
  )

  if (subComponent.long_description) {
    return (
      <Tooltip
        title={subComponent.long_description}
        arrow
        placement="top"
        enterDelay={300}
        leaveDelay={100}
        slotProps={{
          tooltip: {
            sx: (theme) =>
              theme.palette.mode === 'light'
                ? {
                    backgroundColor: '#ffffff',
                    color: '#000000',
                    border: `1px solid ${theme.palette.grey[700]}`,
                  }
                : { border: '1px solid #ffffff' },
          },
        }}
      >
        {cardContent}
      </Tooltip>
    )
  }

  return cardContent
}

export default SubComponentCardComponent
