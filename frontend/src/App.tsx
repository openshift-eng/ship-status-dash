import CssBaseline from '@mui/material/CssBaseline'
import { ThemeProvider, createTheme } from '@mui/material/styles'
import { StylesProvider } from '@mui/styles'
import { useEffect, useMemo, useState } from 'react'
import { Route, BrowserRouter as Router, Routes, useLocation } from 'react-router-dom'

import ComponentDetailsPage from './components/component/ComponentDetailsPage'
import ComponentStatusList from './components/ComponentStatusList'
import Header from './components/Header'
import OutageDetailsPage from './components/outage/OutageDetailsPage'
import SubComponentDetails from './components/sub-component/SubComponentDetails'
import { AuthProvider } from './contexts/AuthContext'
import { getProtectedDomain, getPublicDomain } from './utils/endpoints'

const baseLightTheme = createTheme({ palette: { mode: 'light' } })
const baseDarkTheme = createTheme({ palette: { mode: 'dark' } })

// Light and dark themes with status colors
const lightTheme = createTheme(baseLightTheme, {
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
        main: baseLightTheme.palette.info.main,
        light: baseLightTheme.palette.info.light,
        dark: baseLightTheme.palette.info.dark,
        background: baseLightTheme.palette.info.light,
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

const darkTheme = createTheme(baseDarkTheme, {
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
        main: baseDarkTheme.palette.info.main,
        light: baseDarkTheme.palette.info.light,
        dark: baseDarkTheme.palette.info.dark,
        background: baseDarkTheme.palette.info.light,
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

// This component is used to redirect the user to the public domain if they are on the protected domain
// This is necessary because the oauth proxy will redirect the user to the protected domain after authentication
// and we need to redirect them back to the public domain to avoid a redirect loop
function RedirectIfProtected() {
  const location = useLocation()

  useEffect(() => {
    const currentHost = window.location.hostname
    const protectedDomain = getProtectedDomain()
      .replace(/^https?:\/\//, '')
      .split('/')[0]
    const publicDomain = getPublicDomain()
      .replace(/^https?:\/\//, '')
      .split('/')[0]

    if (currentHost === protectedDomain) {
      const publicUrl = `${window.location.protocol}//${publicDomain}${location.pathname}${location.search}${location.hash}`
      console.log(`redirecting to: ${publicUrl}`)
      window.location.replace(publicUrl)
    }
  }, [location])

  return null
}

function App() {
  const [isDarkMode, setIsDarkMode] = useState(() => {
    const saved = localStorage.getItem('theme')
    if (saved) return saved === 'dark'
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  const theme = useMemo(() => {
    return isDarkMode ? darkTheme : lightTheme
  }, [isDarkMode])

  const toggleTheme = () => {
    const newMode = !isDarkMode
    setIsDarkMode(newMode)
    localStorage.setItem('theme', newMode ? 'dark' : 'light')
    // Dispatch custom event to notify other components
    window.dispatchEvent(new CustomEvent('themeChanged'))
  }

  return (
    <StylesProvider injectFirst>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <AuthProvider>
          <Router>
            <RedirectIfProtected />
            <Header onToggleTheme={toggleTheme} isDarkMode={isDarkMode} />
            <Routes>
              <Route path="/" element={<ComponentStatusList />} />
              <Route path="/:componentSlug" element={<ComponentDetailsPage />} />
              <Route path="/:componentSlug/:subComponentSlug" element={<SubComponentDetails />} />
              <Route
                path="/:componentSlug/:subComponentSlug/outages/:outageId"
                element={<OutageDetailsPage />}
              />
            </Routes>
          </Router>
        </AuthProvider>
      </ThemeProvider>
    </StylesProvider>
  )
}

export default App
