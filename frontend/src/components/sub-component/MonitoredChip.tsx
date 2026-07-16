import { Sensors } from '@mui/icons-material'
import { Box, Chip, Tooltip, styled } from '@mui/material'

import type { Monitoring } from '../../types'
import { relativeTime } from '../../utils/helpers'

const StyledMonitoredChip = styled(Chip)<{ size?: 'small' | 'medium' }>(({ theme, size }) => ({
  backgroundColor:
    theme.palette.mode === 'dark' ? `${theme.palette.info.main}25` : `${theme.palette.info.main}15`,
  color: theme.palette.mode === 'dark' ? theme.palette.info.light : theme.palette.info.dark,
  border: `1px solid ${theme.palette.info.main}40`,
  ...(size === 'small' && {
    fontSize: theme.typography.caption.fontSize,
    height: theme.spacing(2.5),
    '& .MuiChip-label': {
      padding: `0 ${theme.spacing(1)}`,
    },
    '& .MuiChip-icon': {
      fontSize: theme.typography.body2.fontSize,
      marginLeft: theme.spacing(0.5),
    },
  }),
}))

const ChipGuard = styled(Box)({
  display: 'inline-flex',
})

interface MonitoredChipProps {
  monitoring: Monitoring
  /** Confirmed ping time, `null` when absent after a successful status fetch, `undefined` while unresolved. */
  lastPingTime?: string | null
  size?: 'small' | 'medium'
}

const MonitoredChip = ({ monitoring, lastPingTime, size = 'small' }: MonitoredChipProps) => {
  const hasConfirmedPing = lastPingTime != null
  const pingConfirmedAbsent = lastPingTime === null
  const pingLabel = hasConfirmedPing
    ? relativeTime(new Date(lastPingTime), new Date())
    : pingConfirmedAbsent
      ? 'awaiting first ping'
      : 'checking'

  const tooltip = [
    'This sub-component is automatically monitored.',
    `Last ping: ${pingLabel}.`,
    `Expected every ${monitoring.frequency}.`,
  ].join(' ')

  return (
    <Tooltip title={tooltip} arrow placement="top">
      <ChipGuard onClick={(e) => e.stopPropagation()}>
        <StyledMonitoredChip
          icon={<Sensors />}
          label={hasConfirmedPing ? `Monitored · ${pingLabel}` : 'Monitored'}
          size={size}
          data-tour="subcomponent-monitored"
        />
      </ChipGuard>
    </Tooltip>
  )
}

export default MonitoredChip
