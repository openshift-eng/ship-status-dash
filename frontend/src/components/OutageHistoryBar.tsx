import { Box, Divider, Skeleton, Tooltip, Typography, styled, useTheme } from '@mui/material'
import { useMemo } from 'react'

import type { Outage } from '../types'
import { formatStatusSeverityText, getSeverityColor } from '../utils/helpers'

// Severity priority for bucketing multiple outages in a single day.
// Higher index = higher priority (worse).
const SEVERITY_PRIORITY = ['Suspected', 'Partial', 'Degraded', 'CapacityExhausted', 'Down']

const BarContainer = styled(Box)(() => ({
  width: '100%',
}))

const SegmentsRow = styled(Box)(() => ({
  display: 'flex',
  width: '100%',
  gap: '2px',
  alignItems: 'stretch',
}))

const Segment = styled(Box)<{ segmentcolor: string }>(({ segmentcolor }) => ({
  flex: 1,
  height: '28px',
  backgroundColor: segmentcolor,
  borderRadius: '2px',
  cursor: 'default',
  transition: 'opacity 0.15s',
  '&:hover': {
    opacity: 0.75,
  },
}))

const LabelsRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginTop: theme.spacing(0.5),
}))

const LabelText = styled(Typography)(({ theme }) => ({
  fontSize: '0.7rem',
  color: theme.palette.text.secondary,
}))

const TooltipContainer = styled(Box)(() => ({
  minWidth: 180,
  maxWidth: 280,
  padding: 4,
}))

const TooltipDate = styled(Typography)(() => ({
  fontWeight: 700,
  display: 'block',
  marginBottom: 4,
}))

const TooltipNoIncidents = styled(Typography)(() => ({
  color: 'success.light',
}))

const TooltipDivider = styled(Divider)(() => ({
  marginBottom: 8,
  borderColor: 'rgba(255,255,255,0.2)',
}))

const TooltipOutageRow = styled(Box)(() => ({
  display: 'flex',
  justifyContent: 'space-between',
  gap: 8,
}))

const TooltipSeverity = styled(Typography)(() => ({
  fontWeight: 600,
}))

const TooltipDuration = styled(Typography)(() => ({
  opacity: 0.8,
}))

const TooltipDescription = styled(Typography)(() => ({
  display: 'block',
  opacity: 0.75,
  fontSize: '0.7rem',
}))

interface DayBucket {
  date: Date
  worstSeverity: string | null
  outages: Outage[]
}

const buildDayBuckets = (outages: Outage[], days: number): DayBucket[] => {
  const now = new Date()
  const buckets: DayBucket[] = []

  for (let i = days - 1; i >= 0; i--) {
    const dayStart = new Date(now)
    dayStart.setDate(dayStart.getDate() - i)
    dayStart.setHours(0, 0, 0, 0)

    const dayEnd = new Date(dayStart)
    dayEnd.setHours(23, 59, 59, 999)

    const dayOutages = outages.filter((o) => {
      const start = new Date(o.start_time)
      const end = o.end_time?.Valid ? new Date(o.end_time.Time) : null
      return start <= dayEnd && (end === null || end >= dayStart)
    })

    let worstSeverity: string | null = null
    for (const outage of dayOutages) {
      const priority = SEVERITY_PRIORITY.indexOf(outage.severity)
      if (priority === -1) {
        // Unknown severity: still counts as an outage — record at lowest priority if nothing better is set.
        if (worstSeverity === null) worstSeverity = outage.severity
      } else {
        const currentPriority =
          worstSeverity !== null
            ? SEVERITY_PRIORITY.indexOf(worstSeverity) === -1
              ? 0
              : SEVERITY_PRIORITY.indexOf(worstSeverity)
            : -1
        if (priority > currentPriority) worstSeverity = outage.severity
      }
    }

    buckets.push({ date: dayStart, worstSeverity, outages: dayOutages })
  }

  return buckets
}

const computeUptimePercent = (buckets: DayBucket[]): number => {
  if (buckets.length === 0) return 100
  const healthyDays = buckets.filter((b) => b.worstSeverity === null).length
  const percent = (healthyDays / buckets.length) * 100
  return Math.round(percent * 100) / 100 // round to 2 decimal places
}

interface OutageHistoryBarProps {
  componentName: string
  subComponentName: string
  outages: Outage[]
  loading: boolean
  days?: number
}

const OutageHistoryBar = ({
  componentName,
  subComponentName,
  outages,
  loading,
  days = 90,
}: OutageHistoryBarProps) => {
  const theme = useTheme()

  const buckets = useMemo(() => buildDayBuckets(outages, days), [outages, days])
  const uptimePercent = useMemo(() => computeUptimePercent(buckets), [buckets])

  const segmentColor = (severity: string | null): string => {
    return getSeverityColor(theme, severity ?? 'Healthy')
  }

  const formatDuration = (startStr: string, endTime: Outage['end_time']): string => {
    if (!endTime.Valid) return 'Ongoing'
    const ms = new Date(endTime.Time).getTime() - new Date(startStr).getTime()
    const hours = Math.floor(ms / 3600000)
    const mins = Math.floor((ms % 3600000) / 60000)
    if (hours > 0) return `${hours}h ${mins}m`
    return `${mins}m`
  }

  const tooltipNode = (bucket: DayBucket): React.ReactNode => {
    const dateStr = bucket.date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
    return (
      <TooltipContainer>
        <TooltipDate variant="caption">{dateStr}</TooltipDate>
        {bucket.outages.length === 0 ? (
          <TooltipNoIncidents variant="caption">No incidents</TooltipNoIncidents>
        ) : (
          <>
            <TooltipDivider />
            {bucket.outages.map((o, idx) => (
              <Box key={o.ID} mb={idx < bucket.outages.length - 1 ? 1 : 0}>
                <TooltipOutageRow>
                  <TooltipSeverity variant="caption">
                    {formatStatusSeverityText(o.severity)}
                  </TooltipSeverity>
                  <TooltipDuration variant="caption">
                    {formatDuration(o.start_time, o.end_time)}
                  </TooltipDuration>
                </TooltipOutageRow>
                {o.description && (
                  <TooltipDescription variant="caption">{o.description}</TooltipDescription>
                )}
              </Box>
            ))}
          </>
        )}
      </TooltipContainer>
    )
  }

  const startLabel = `${days} days ago`

  if (loading) {
    return (
      <BarContainer>
        <Skeleton variant="rectangular" width="100%" height={28} sx={{ borderRadius: '2px' }} />
        <LabelsRow>
          <LabelText>{startLabel}</LabelText>
          <LabelText>Today</LabelText>
        </LabelsRow>
      </BarContainer>
    )
  }

  return (
    <BarContainer
      onClick={(e) => e.stopPropagation()}
      aria-label={`Outage history for ${componentName} ${subComponentName}`}
    >
      <SegmentsRow>
        {buckets.map((bucket, i) => (
          <Tooltip key={i} title={tooltipNode(bucket)} arrow placement="top" enterDelay={200}>
            <Segment segmentcolor={segmentColor(bucket.worstSeverity)} />
          </Tooltip>
        ))}
      </SegmentsRow>
      <LabelsRow>
        <LabelText>{startLabel}</LabelText>
        <LabelText>{uptimePercent}% uptime</LabelText>
        <LabelText>Today</LabelText>
      </LabelsRow>
    </BarContainer>
  )
}

export default OutageHistoryBar
