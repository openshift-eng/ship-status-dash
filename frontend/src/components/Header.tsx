import { Brightness4, Brightness7, Login } from '@mui/icons-material'
import { AppBar, Box, Button, IconButton, Toolbar, Tooltip } from '@mui/material'
import { useNavigate } from 'react-router-dom'

import { getProtectedDomain } from '../utils/endpoints'

interface HeaderProps {
  onToggleTheme: () => void
  isDarkMode: boolean
}

const Header = ({ onToggleTheme, isDarkMode }: HeaderProps) => {
  const navigate = useNavigate()

  const handleLogoClick = () => {
    navigate('/')
  }

  const handleLoginClick = () => {
    // we need to store the redirect url in local storage because the oauth proxy will redirect to the callback url after authentication
    localStorage.setItem('oauth_redirect', window.location.href)
    window.location.href = `${getProtectedDomain()}/oauth/start`
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
          src={isDarkMode ? '/logo-dark.svg' : '/logo.svg'}
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

        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Button
            variant="outlined"
            startIcon={<Login />}
            onClick={handleLoginClick}
            sx={{
              color: 'text.primary',
              borderColor: 'divider',
              textTransform: 'none',
              '&:hover': {
                borderColor: 'primary.main',
                backgroundColor: 'action.hover',
              },
            }}
          >
            Login
          </Button>
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
        </Box>
      </Toolbar>
    </AppBar>
  )
}

export default Header
