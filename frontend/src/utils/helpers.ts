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
      return theme.palette.mode === 'dark'
        ? theme.palette.warning.dark
        : theme.palette.warning.light
    case 'Unknown':
      return theme.palette.mode === 'dark' ? theme.palette.grey[700] : theme.palette.grey[300]
    default:
      return theme.palette.mode === 'dark' ? theme.palette.grey[800] : theme.palette.grey[100]
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
      return theme.palette.mode === 'dark'
        ? theme.palette.warning.light
        : theme.palette.warning.dark
    case 'Unknown':
      return theme.palette.mode === 'dark' ? theme.palette.grey[400] : theme.palette.grey[600]
    default:
      return theme.palette.mode === 'dark' ? theme.palette.grey[300] : theme.palette.grey[500]
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

// Helper function to format dates to second precision
export const formatDateToSeconds = (dateString: string) => {
  if (!dateString) return ''
  const date = new Date(dateString)
  return date.toISOString().replace(/\.\d{3}Z$/, 'Z')
}

// relativeTime shows a plain English rendering of a time, e.g. "30 minutes ago".
// This is because the ES6 Intl.RelativeTime isn't available in all environments yet,
// e.g. Safari and NodeJS.
export const relativeTime = (date: Date, startDate: Date) => {
  const minute = 1000 * 60 // Milliseconds in a minute
  const hour = 60 * minute // Milliseconds in an hour
  const day = 24 * hour // Milliseconds in a day

  const millisAgo = date.getTime() - startDate.getTime()
  if (Math.abs(millisAgo) < hour) {
    return Math.round(Math.abs(millisAgo) / minute) + ' minutes ago'
  } else if (Math.abs(millisAgo) < day) {
    const hours = Math.round(Math.abs(millisAgo) / hour)
    return `${hours} ${hours === 1 ? 'hour' : 'hours'} ago`
  } else if (Math.abs(millisAgo) < 1.5 * day) {
    return 'about a day ago'
  } else {
    return Math.round(Math.abs(millisAgo) / day) + ' days ago'
  }
}

// relativeDuration shows a plain English rendering of a duration, e.g. "30 minutes".
export const relativeDuration = (secondsAgo: number) => {
  if (secondsAgo === undefined) {
    return { value: 'N/A', units: 'N/A' }
  }

  const minute = 60
  const hour = 60 * minute
  const day = 24 * hour

  if (Math.abs(secondsAgo) < hour) {
    return { value: Math.abs(secondsAgo) / minute, units: 'minutes' }
  } else if (Math.abs(secondsAgo) < day) {
    const hours = Math.abs(secondsAgo) / hour
    return { value: hours, units: hours === 1 ? 'hour' : 'hours' }
  } else if (Math.abs(secondsAgo) < 1.5 * day) {
    return { value: 1, units: 'day' }
  } else {
    const days = Math.abs(secondsAgo) / day
    return { value: days, units: days === 1 ? 'day' : 'days' }
  }
}

// formatDuration formats a duration between start time and optional end time as a human-readable string
export const formatDuration = (
  startTime: string,
  endTime?: { Time: string; Valid: boolean },
): string => {
  const start = new Date(startTime)
  const end = endTime?.Valid ? new Date(endTime.Time) : new Date()
  const durationSeconds = Math.floor((end.getTime() - start.getTime()) / 1000)
  const duration = relativeDuration(durationSeconds)
  return `${Math.round(Number(duration.value))} ${duration.units}`
}

// Helper function to get current local time in datetime-local format
export const getCurrentLocalTime = () => {
  return formatDateForDateTimeLocal(new Date())
}

// Helper function to format a date to datetime-local format (YYYY-MM-DDTHH:mm)
// This preserves the local timezone instead of converting to UTC
export const formatDateForDateTimeLocal = (date: Date) => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day}T${hours}:${minutes}`
}
