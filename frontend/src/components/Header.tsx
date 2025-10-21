import { Brightness4, Brightness7 } from '@mui/icons-material'
import { AppBar, Toolbar, Box, IconButton, Tooltip } from '@mui/material'
import React from 'react'
import { useNavigate } from 'react-router-dom'

interface HeaderProps {
  onToggleTheme: () => void
  isDarkMode: boolean
}

const Header: React.FC<HeaderProps> = ({ onToggleTheme, isDarkMode }) => {
  const navigate = useNavigate()

  const handleLogoClick = () => {
    navigate('/')
  }

  return (
    <AppBar
      position="sticky"
      sx={{
        backgroundColor: 'background.paper',
        boxShadow: 1,
        zIndex: 1000,
      }}
    >
      <Toolbar sx={{ justifyContent: 'space-between' }}>
        <Box
          component="img"
          src="/logo.svg"
          alt="Logo"
          onClick={handleLogoClick}
          sx={{
            height: 40,
            width: 'auto',
            maxWidth: 200,
            cursor: 'pointer',
            '&:hover': {
              opacity: 0.8,
            },
          }}
        />

        <Tooltip title={isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'}>
          <IconButton
            onClick={onToggleTheme}
            sx={{
              color: 'text.primary',
              backgroundColor: 'action.hover',
              '&:hover': {
                backgroundColor: 'action.selected',
              },
            }}
          >
            {isDarkMode ? <Brightness7 /> : <Brightness4 />}
          </IconButton>
        </Tooltip>
      </Toolbar>
    </AppBar>
  )
}

export default Header
