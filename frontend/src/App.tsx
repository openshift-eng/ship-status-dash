import CssBaseline from '@mui/material/CssBaseline'
import { ThemeProvider, createTheme } from '@mui/material/styles'
import { StylesProvider } from '@mui/styles'
import React, { useState, useMemo } from 'react'
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'

import ComponentDetailsPage from './components/component/ComponentDetailsPage'
import ComponentStatusList from './components/ComponentStatusList'
import Header from './components/Header'
import OutageDetailsPage from './components/outage/OutageDetailsPage'
import SubComponentDetails from './components/sub-component/SubComponentDetails'

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
