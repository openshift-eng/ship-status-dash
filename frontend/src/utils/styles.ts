import type { Theme } from '@mui/material/styles'

import { getStatusBackgroundColor } from './helpers'

/**
 * Returns style object for status-colored border and 5% tint overlay.
 * Use in styled components that need consistent status styling (e.g. ComponentWell, ComponentHeader).
 *
 * @param borderRadius - Theme spacing unit (e.g. 2) or CSS value (e.g. 'inherit')
 */
export const getStatusTintStyles = (
  theme: Theme,
  status: string,
  borderRadius: number | string = 2,
) => {
  const statusColor = getStatusBackgroundColor(theme, status)
  const radius =
    typeof borderRadius === 'number' ? theme.spacing(borderRadius) : borderRadius

  return {
    backgroundColor: theme.palette.background.paper,
    border: `1px solid ${statusColor}`,
    position: 'relative' as const,
    '&::before': {
      content: '""',
      position: 'absolute',
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      background: statusColor,
      opacity: 0.05,
      borderRadius: radius,
      pointerEvents: 'none',
      zIndex: 0,
    },
    '& > *': {
      position: 'relative',
      zIndex: 1,
    },
  }
}
