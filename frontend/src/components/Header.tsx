import { Brightness4, Brightness7 } from '@mui/icons-material'
import { AppBar, Box, IconButton, styled, Toolbar, Tooltip } from '@mui/material'
import { useNavigate } from 'react-router-dom'

import Auth from './Auth'

interface HeaderProps {
  onToggleTheme: () => void
  isDarkMode: boolean
}

const DarkModeToggle = styled(IconButton)(({ theme }) => ({
  color: theme.palette.text.primary,
  backgroundColor: theme.palette.action.hover,
  '&:hover': {
    backgroundColor: theme.palette.action.selected,
  },
}))

const Header = ({ onToggleTheme, isDarkMode }: HeaderProps) => {
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
          <Tooltip title={isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'}>
            <DarkModeToggle onClick={onToggleTheme}>
              {isDarkMode ? <Brightness7 /> : <Brightness4 />}
            </DarkModeToggle>
          </Tooltip>
          <Auth />
        </Box>
      </Toolbar>
    </AppBar>
  )
}

export default Header
