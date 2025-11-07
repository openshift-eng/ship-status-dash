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
import { getProtectedDomain, getPublicDomain } from './utils/endpoints'

// Create light and dark themes
const lightTheme = createTheme({
  palette: {
    mode: 'light',
  },
})

const darkTheme = createTheme({
  palette: {
    mode: 'dark',
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
        <Router>
          <RedirectIfProtected />
          <Header onToggleTheme={toggleTheme} isDarkMode={isDarkMode} />
          <Routes>
            <Route path="/" element={<ComponentStatusList />} />
            <Route path="/:componentName" element={<ComponentDetailsPage />} />
            <Route path="/:componentName/:subComponentName" element={<SubComponentDetails />} />
            <Route
              path="/:componentName/:subComponentName/outages/:outageId"
              element={<OutageDetailsPage />}
            />
          </Routes>
        </Router>
      </ThemeProvider>
    </StylesProvider>
  )
}

export default App
