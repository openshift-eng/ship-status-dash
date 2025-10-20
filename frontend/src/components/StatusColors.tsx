import { Chip } from '@mui/material'
import { styled } from '@mui/material/styles'

import { getStatusChipColor, getSeverityColor } from '../utils/helpers'

export const StatusChip = styled(Chip)<{ status: string }>(({ theme, status }) => {
  const color = getStatusChipColor(theme, status)

  return {
    backgroundColor: color,
    color: theme.palette.getContrastText(color),
    '&.MuiChip-outlined': {
      borderColor: color,
      color: color,
      backgroundColor: 'transparent',
    },
  }
})

export const SeverityChip = styled(Chip)<{ severity: string }>(({ theme, severity }) => {
  const color = getSeverityColor(theme, severity)

  return {
    borderColor: color,
    color: color,
    backgroundColor: 'transparent',
  }
})
