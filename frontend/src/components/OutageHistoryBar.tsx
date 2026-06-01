import { Box, Divider, Skeleton, Tooltip, Typography, styled, useTheme } from '@mui/material'

import type { OutageDayBucket } from '../types'
import { formatMinutes, formatStatusSeverityText, getSeverityColor } from '../utils/helpers'

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
  borderTop: `1px dashed ${theme.palette.divider}`,
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

const computeUptimePercent = (buckets: OutageDayBucket[]): number => {
  if (buckets.length === 0) return 100
  const healthyDays = buckets.filter((b) => b.highest_severity === null).length
  const percent = (healthyDays / buckets.length) * 100
  return Math.round(percent * 100) / 100
}

interface OutageHistoryBarProps {
  componentName: string
  subComponentName: string
  buckets: OutageDayBucket[]
  loading: boolean
  days?: number
}

const OutageHistoryBar = ({
  componentName,
  subComponentName,
  buckets,
  loading,
  days = 90,
}: OutageHistoryBarProps) => {
  const theme = useTheme()
  const uptimePercent = computeUptimePercent(buckets)
  const healthyColor = getSeverityColor(theme, 'Healthy')

  const segmentBackground = (bucket: OutageDayBucket): string => {
    if (bucket.highest_severity === null) {
      return healthyColor
    }
    const rawFraction = bucket.total_outage_minutes / (24 * 60)
    const displayFraction = Math.max(MIN_VISIBLE_FRACTION, Math.min(1, rawFraction))
    const severityColor = getSeverityColor(theme, bucket.highest_severity)
    // Severity color rises from the bottom, healthy fills the rest.
    return `linear-gradient(to top, ${severityColor} ${displayFraction * 100}%, ${healthyColor} ${displayFraction * 100}%)`
  }

  const tooltipNode = (bucket: OutageDayBucket): React.ReactNode => {
    const dateStr = bucket.date
      ? new Date(bucket.date + 'T00:00:00').toLocaleDateString('en-US', {
          month: 'short',
          day: 'numeric',
          year: 'numeric',
        })
      : ''
    return (
      <TooltipContainer>
        <TooltipDate variant="caption">{dateStr}</TooltipDate>
        {bucket.outage_count === 0 ? (
          <TooltipNoIncidents variant="caption">No incidents</TooltipNoIncidents>
        ) : (
          <>
            <TooltipDivider />
            <TooltipOutageRow>
              <TooltipSeverity variant="caption">
                {bucket.outage_count === 1 ? '1 incident' : `${bucket.outage_count} incidents`}
              </TooltipSeverity>
              <TooltipDuration variant="caption">
                {formatMinutes(bucket.total_outage_minutes)} total
              </TooltipDuration>
            </TooltipOutageRow>
            <TooltipDescription variant="caption">
              Highest Severity: {formatStatusSeverityText(bucket.highest_severity ?? '')}
            </TooltipDescription>
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
