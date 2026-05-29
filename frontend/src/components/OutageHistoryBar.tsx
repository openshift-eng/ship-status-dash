import { Box, Divider, Skeleton, Tooltip, Typography, styled, useTheme } from '@mui/material'
import { useMemo } from 'react'

import type { Outage } from '../types'
import { formatStatusSeverityText, getSeverityColor } from '../utils/helpers'

// Severity priority for bucketing multiple outages in a single day.
// Higher index = higher priority (worse).
const SEVERITY_PRIORITY = ['Suspected', 'Partial', 'Degraded', 'CapacityExhausted', 'Down']

// Minimum visible fill fraction for short outages so they remain visible at day-bar scale.
const MIN_VISIBLE_FRACTION = 0.15

const BarContainer = styled(Box)(() => ({
  width: '100%',
}))

const SegmentsRow = styled(Box)(() => ({
  display: 'flex',
  width: '100%',
  gap: '2px',
  alignItems: 'stretch',
}))

const Segment = styled(Box)<{ segmentbg: string }>(({ segmentbg }) => ({
  flex: 1,
  height: '28px',
  background: segmentbg,
  borderRadius: '2px',
  cursor: 'default',
  transition: 'opacity 0.15s',
  '&:hover': {
    opacity: 0.75,
  },
}))

const LabelsRow = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  marginTop: theme.spacing(0.5),
  gap: theme.spacing(1),
}))

const LabelText = styled(Typography)(({ theme }) => ({
  fontSize: '0.7rem',
  color: theme.palette.text.secondary,
  whiteSpace: 'nowrap',
}))

const LabelDivider = styled(Box)(({ theme }) => ({
  flex: 1,
  height: '1px',
  backgroundColor: theme.palette.divider,
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
  totalOutageMinutes: number
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
    let totalOutageMs = 0

    for (const outage of dayOutages) {
      const priority = SEVERITY_PRIORITY.indexOf(outage.severity)
      if (priority === -1) {
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

      // Clip the outage to the day boundary to count only time within this day.
      const clippedStart = Math.max(new Date(outage.start_time).getTime(), dayStart.getTime())
      const outageEnd = outage.end_time?.Valid ? new Date(outage.end_time.Time) : now
      const clippedEnd = Math.min(outageEnd.getTime(), dayEnd.getTime())
      totalOutageMs += Math.max(0, clippedEnd - clippedStart)
    }

    buckets.push({
      date: dayStart,
      worstSeverity,
      totalOutageMinutes: totalOutageMs / 60000,
      outages: dayOutages,
    })
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

  const healthyColor = getSeverityColor(theme, 'Healthy')

  const segmentBackground = (bucket: DayBucket): string => {
    if (bucket.worstSeverity === null) {
      return healthyColor
    }
    const rawFraction = bucket.totalOutageMinutes / (24 * 60)
    const displayFraction = Math.max(MIN_VISIBLE_FRACTION, Math.min(1, rawFraction))
    const severityColor = getSeverityColor(theme, bucket.worstSeverity)
    // Severity color rises from the bottom, healthy fills the rest.
    return `linear-gradient(to top, ${severityColor} ${displayFraction * 100}%, ${healthyColor} ${displayFraction * 100}%)`
  }

  const formatDuration = (startStr: string, endTime: Outage['end_time']): string => {
    if (!endTime.Valid) return 'Ongoing'
    const ms = new Date(endTime.Time).getTime() - new Date(startStr).getTime()
    const hours = Math.floor(ms / 3600000)
    const mins = Math.floor((ms % 3600000) / 60000)
    const secs = Math.floor((ms % 60000) / 1000)
    if (hours > 0) return `${hours}h ${mins}m`
    if (mins > 0) return `${mins}m`
    return `${secs}s`
  }

  const formatMinutes = (minutes: number): string => {
    const hours = Math.floor(minutes / 60)
    const mins = Math.floor(minutes % 60)
    const secs = Math.floor((minutes % 1) * 60)
    if (hours > 0) return `${hours}h ${mins}m`
    if (mins > 0) return `${mins}m`
    return `${secs}s`
  }

  const tooltipNode = (bucket: DayBucket): React.ReactNode => {
    const dateStr = bucket.date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
    const single = bucket.outages.length === 1
    return (
      <TooltipContainer>
        <TooltipDate variant="caption">{dateStr}</TooltipDate>
        {bucket.outages.length === 0 ? (
          <TooltipNoIncidents variant="caption">No incidents</TooltipNoIncidents>
        ) : (
          <>
            <TooltipDivider />
            {single ? (
              <Box>
                <TooltipOutageRow>
                  <TooltipSeverity variant="caption">
                    {formatStatusSeverityText(bucket.outages[0].severity)}
                  </TooltipSeverity>
                  <TooltipDuration variant="caption">
                    {formatDuration(bucket.outages[0].start_time, bucket.outages[0].end_time)}
                  </TooltipDuration>
                </TooltipOutageRow>
                {bucket.outages[0].description && (
                  <TooltipDescription variant="caption">
                    {bucket.outages[0].description}
                  </TooltipDescription>
                )}
              </Box>
            ) : (
              <Box>
                <TooltipOutageRow>
                  <TooltipSeverity variant="caption">
                    {bucket.outages.length} incidents
                  </TooltipSeverity>
                  <TooltipDuration variant="caption">
                    {formatMinutes(bucket.totalOutageMinutes)} total
                  </TooltipDuration>
                </TooltipOutageRow>
                <TooltipDescription variant="caption">
                  Worst: {formatStatusSeverityText(bucket.worstSeverity ?? '')}
                </TooltipDescription>
              </Box>
            )}
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
          <LabelDivider />
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
            <Segment segmentbg={segmentBackground(bucket)} />
          </Tooltip>
        ))}
      </SegmentsRow>
      <LabelsRow>
        <LabelText>{startLabel}</LabelText>
        <LabelDivider />
        <LabelText>{uptimePercent}% uptime</LabelText>
        <LabelDivider />
        <LabelText>Today</LabelText>
      </LabelsRow>
    </BarContainer>
  )
}

export default OutageHistoryBar
