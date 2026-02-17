import { createTheme } from '@mui/material/styles'

const baseLightTheme = createTheme({
  palette: {
    mode: 'light',
    tagBorderColor: '#000000',
    tagBackgroundColor: '#ffffff',
    tagTextColor: '#000000',
  },
})
const baseDarkTheme = createTheme({
  palette: {
    mode: 'dark',
    tagBorderColor: '#ffffff',
    tagBackgroundColor: '#000000',
    tagTextColor: '#ffffff',
  },
})

// Light and dark themes with status colors
export const lightTheme = createTheme(baseLightTheme, {
  palette: {
    status: {
      healthy: {
        main: baseLightTheme.palette.success.main,
        light: baseLightTheme.palette.success.light,
        dark: baseLightTheme.palette.success.dark,
        background: baseLightTheme.palette.success.light,
      },
      degraded: {
        main: baseLightTheme.palette.warning.main,
        light: baseLightTheme.palette.warning.light,
        dark: baseLightTheme.palette.warning.dark,
        background: baseLightTheme.palette.warning.light,
      },
      down: {
        main: baseLightTheme.palette.error.main,
        light: baseLightTheme.palette.error.light,
        dark: baseLightTheme.palette.error.dark,
        background: baseLightTheme.palette.error.light,
      },
      capacityExhausted: {
        main: '#f57c00',
        light: '#ffb74d',
        dark: '#d68910',
        background: '#ffb74d',
      },
      suspected: {
        main: '#8d6e63',
        light: '#a1887f',
        dark: '#6d4c41',
        background: '#a1887f',
      },
      partial: {
        main: baseLightTheme.palette.warning.main,
        light: baseLightTheme.palette.warning.light,
        dark: baseLightTheme.palette.warning.dark,
        background: baseLightTheme.palette.warning.light,
      },
      unknown: {
        main: baseLightTheme.palette.grey[600],
        light: baseLightTheme.palette.grey[400],
        dark: baseLightTheme.palette.grey[700],
        background: baseLightTheme.palette.grey[300],
      },
    },
  },
})

export const darkTheme = createTheme(baseDarkTheme, {
  palette: {
    status: {
      healthy: {
        main: baseDarkTheme.palette.success.main,
        light: baseDarkTheme.palette.success.light,
        dark: baseDarkTheme.palette.success.dark,
        background: baseDarkTheme.palette.success.light,
      },
      degraded: {
        main: baseDarkTheme.palette.warning.main,
        light: baseDarkTheme.palette.warning.light,
        dark: baseDarkTheme.palette.warning.dark,
        background: baseDarkTheme.palette.warning.light,
      },
      down: {
        main: baseDarkTheme.palette.error.main,
        light: baseDarkTheme.palette.error.light,
        dark: baseDarkTheme.palette.error.dark,
        background: baseDarkTheme.palette.error.light,
      },
      capacityExhausted: {
        main: '#d68910',
        light: '#ffb74d',
        dark: '#b8620b',
        background: '#ffb74d',
      },
      suspected: {
        main: '#a1887f',
        light: '#bcaaa4',
        dark: '#8d6e63',
        background: '#bcaaa4',
      },
      partial: {
        main: baseDarkTheme.palette.warning.main,
        light: baseDarkTheme.palette.warning.light,
        dark: baseDarkTheme.palette.warning.dark,
        background: baseDarkTheme.palette.warning.dark,
      },
      unknown: {
        main: baseDarkTheme.palette.grey[400],
        light: baseDarkTheme.palette.grey[300],
        dark: baseDarkTheme.palette.grey[500],
        background: baseDarkTheme.palette.grey[700],
      },
    },
  },
})

// Accessibility-friendly themes with colorblind-safe colors
// Uses shades of blue for greens (healthy) and shades of orange for reds (down)
export const lightAccessibilityTheme = createTheme(baseLightTheme, {
  palette: {
    status: {
      healthy: {
        main: '#1976d2',
        light: '#42a5f5',
        dark: '#1565c0',
        background: '#42a5f5',
      },
      degraded: {
        main: '#e65100',
        light: '#ff6f00',
        dark: '#bf360c',
        background: '#ff6f00',
      },
      down: {
        main: '#f57c00',
        light: '#ff9800',
        dark: '#e65100',
        background: '#ff9800',
      },
      capacityExhausted: {
        main: '#e65100',
        light: '#ff6f00',
        dark: '#bf360c',
        background: '#ff6f00',
      },
      suspected: {
        main: '#8d6e63',
        light: '#a1887f',
        dark: '#6d4c41',
        background: '#a1887f',
      },
      partial: {
        main: '#ff9800',
        light: '#ffb74d',
        dark: '#f57c00',
        background: '#ffb74d',
      },
      unknown: {
        main: baseLightTheme.palette.grey[600],
        light: baseLightTheme.palette.grey[400],
        dark: baseLightTheme.palette.grey[700],
        background: baseLightTheme.palette.grey[300],
      },
    },
  },
})

export const darkAccessibilityTheme = createTheme(baseDarkTheme, {
  palette: {
    status: {
      healthy: {
        main: '#42a5f5',
        light: '#64b5f6',
        dark: '#1976d2',
        background: '#64b5f6',
      },
      degraded: {
        main: '#ff6f00',
        light: '#ff8f00',
        dark: '#e65100',
        background: '#ff8f00',
      },
      down: {
        main: '#ff9800',
        light: '#ffb74d',
        dark: '#f57c00',
        background: '#ffb74d',
      },
      capacityExhausted: {
        main: '#ff6f00',
        light: '#ff8f00',
        dark: '#e65100',
        background: '#ff8f00',
      },
      suspected: {
        main: '#a1887f',
        light: '#bcaaa4',
        dark: '#8d6e63',
        background: '#bcaaa4',
      },
      partial: {
        main: '#ffb74d',
        light: '#ffcc80',
        dark: '#ff9800',
        background: '#ffcc80',
      },
      unknown: {
        main: baseDarkTheme.palette.grey[400],
        light: baseDarkTheme.palette.grey[300],
        dark: baseDarkTheme.palette.grey[500],
        background: baseDarkTheme.palette.grey[700],
      },
    },
  },
})
