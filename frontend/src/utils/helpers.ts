import type { Theme } from '@mui/material/styles'

export const getStatusBackgroundColor = (theme: Theme, status: string) => {
  switch (status) {
    case 'Healthy':
      return theme.palette.success.light
    case 'Degraded':
      return theme.palette.warning.light
    case 'Down':
      return theme.palette.error.light
    case 'Suspected':
      return theme.palette.info.light
    case 'Partial':
      return '#FFB366' // More vibrant orange
    case 'Unknown':
      return theme.palette.grey[300]
    default:
      return theme.palette.grey[100]
  }
}

export const getStatusChipColor = (theme: Theme, status: string) => {
  switch (status) {
    case 'Healthy':
      return theme.palette.success.main
    case 'Degraded':
      return theme.palette.warning.main
    case 'Down':
      return theme.palette.error.main
    case 'Suspected':
      return theme.palette.info.main
    case 'Partial':
      return '#FF8C00' // Vibrant orange for better contrast
    case 'Unknown':
      return theme.palette.grey[600]
    default:
      return theme.palette.grey[500]
  }
}

export const getSeverityColor = (theme: Theme, severity: string) => {
  switch (severity) {
    case 'Down':
      return theme.palette.error.main
    case 'Degraded':
      return theme.palette.warning.main
    default:
      return theme.palette.info.main
  }
}
