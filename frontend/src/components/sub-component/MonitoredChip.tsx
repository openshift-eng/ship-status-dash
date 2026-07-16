import { Sensors } from '@mui/icons-material'
import { Chip, Tooltip, styled } from '@mui/material'

import type { Monitoring } from '../../types'
import { relativeTime } from '../../utils/helpers'

const StyledMonitoredChip = styled(Chip)<{ size?: 'small' | 'medium' }>(({ theme, size }) => ({
  backgroundColor:
    theme.palette.mode === 'dark' ? `${theme.palette.info.main}25` : `${theme.palette.info.main}15`,
  color: theme.palette.mode === 'dark' ? theme.palette.info.light : theme.palette.info.dark,
  border: `1px solid ${theme.palette.info.main}40`,
  ...(size === 'small' && {
    fontSize: '0.65rem',
    height: '20px',
    '& .MuiChip-label': {
      padding: '0 8px',
    },
    '& .MuiChip-icon': {
      fontSize: '0.875rem',
      marginLeft: '4px',
    },
  }),
}))

interface MonitoredChipProps {
  monitoring: Monitoring
  lastPingTime?: string
  size?: 'small' | 'medium'
}

const MonitoredChip = ({ monitoring, lastPingTime, size = 'small' }: MonitoredChipProps) => {
  const pingLabel = lastPingTime
    ? relativeTime(new Date(lastPingTime), new Date())
    : 'awaiting first ping'

  const tooltip = [
    'This sub-component is automatically monitored.',
    `Last ping: ${pingLabel}.`,
    `Expected every ${monitoring.frequency}.`,
  ].join(' ')

  return (
    <Tooltip title={tooltip} arrow placement="top">
      <StyledMonitoredChip
        icon={<Sensors />}
        label={lastPingTime ? `Monitored · ${pingLabel}` : 'Monitored'}
        size={size}
        data-tour="subcomponent-monitored"
        onClick={(e) => e.stopPropagation()}
      />
    </Tooltip>
  )
}

export default MonitoredChip
