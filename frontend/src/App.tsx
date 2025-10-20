import CssBaseline from '@mui/material/CssBaseline'
import { ThemeProvider, createTheme } from '@mui/material/styles'
import { StylesProvider } from '@mui/styles'
import React from 'react'
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'

import ComponentDetailsPage from './components/ComponentDetailsPage'
import ComponentStatusList from './components/ComponentStatusList'
import Header from './components/Header'

const theme = createTheme()

function App() {
  return (
    <StylesProvider injectFirst>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Router>
          <Header />
          <Routes>
            <Route path="/" element={<ComponentStatusList />} />
            <Route path="/component/:componentName" element={<ComponentDetailsPage />} />
          </Routes>
        </Router>
      </ThemeProvider>
    </StylesProvider>
  )
}

export default App
