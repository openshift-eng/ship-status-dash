import {
  Accessibility,
  Brightness4,
  Brightness7,
  Dashboard,
  HelpOutline,
  History,
  Insights,
  Menu as MenuIcon,
} from '@mui/icons-material'
import {
  AppBar,
  Box,
  Divider,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  styled,
  Toolbar,
} from '@mui/material'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { EXTERNAL_PAGES_PATH_PREFIX, externalPages } from '../constants/externalPages'

import Auth from './Auth'
import { TOUR_RESTART_EVENT, useHasTour } from './tour/AppTour'

interface HeaderProps {
  onToggleTheme: () => void
  isDarkMode: boolean
  onToggleAccessibility: () => void
  isAccessibilityMode: boolean
}

const HeaderIconButton = styled(IconButton)(({ theme }) => ({
  color: theme.palette.text.primary,
  backgroundColor: theme.palette.action.hover,
  '&:hover': {
    backgroundColor: theme.palette.action.selected,
  },
}))

const Header = ({
  onToggleTheme,
  isDarkMode,
  onToggleAccessibility,
  isAccessibilityMode,
}: HeaderProps) => {
  const navigate = useNavigate()
  const hasTour = useHasTour()
  const spcPage = externalPages.find((p) => p.slug === 'spc-dashboard')
  const [menuAnchor, setMenuAnchor] = useState<null | HTMLElement>(null)

  const handleMenuOpen = (e: React.MouseEvent<HTMLElement>) => setMenuAnchor(e.currentTarget)
  const handleMenuClose = () => setMenuAnchor(null)

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
          onClick={() => navigate('/')}
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
          <Auth />
          <HeaderIconButton onClick={handleMenuOpen} aria-label="Open menu">
            <MenuIcon />
          </HeaderIconButton>
          <Menu anchorEl={menuAnchor} open={Boolean(menuAnchor)} onClose={handleMenuClose}>
            <MenuItem
              onClick={() => {
                navigate('/')
                handleMenuClose()
              }}
            >
              <ListItemIcon>
                <Dashboard fontSize="small" />
              </ListItemIcon>
              <ListItemText>Component Status</ListItemText>
            </MenuItem>
            <MenuItem
              onClick={() => {
                navigate('/status-history')
                handleMenuClose()
              }}
            >
              <ListItemIcon>
                <History fontSize="small" />
              </ListItemIcon>
              <ListItemText>Incident History</ListItemText>
            </MenuItem>
            {spcPage && (
              <MenuItem
                onClick={() => {
                  navigate(`${EXTERNAL_PAGES_PATH_PREFIX}/${spcPage.slug}`)
                  handleMenuClose()
                }}
              >
                <ListItemIcon>
                  <Insights fontSize="small" />
                </ListItemIcon>
                <ListItemText>{spcPage.label}</ListItemText>
              </MenuItem>
            )}
            <Divider />
            <MenuItem
              onClick={() => {
                onToggleAccessibility()
                handleMenuClose()
              }}
            >
              <ListItemIcon>
                <Accessibility
                  fontSize="small"
                  color={isAccessibilityMode ? 'primary' : 'inherit'}
                />
              </ListItemIcon>
              <ListItemText>
                {isAccessibilityMode ? 'Disable accessibility mode' : 'Enable accessibility mode'}
              </ListItemText>
            </MenuItem>
            <MenuItem
              onClick={() => {
                onToggleTheme()
                handleMenuClose()
              }}
            >
              <ListItemIcon>
                {isDarkMode ? <Brightness7 fontSize="small" /> : <Brightness4 fontSize="small" />}
              </ListItemIcon>
              <ListItemText>
                {isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'}
              </ListItemText>
            </MenuItem>
            <Divider />
            <MenuItem
              disabled={!hasTour}
              data-tour="page-tour-button"
              onClick={() => {
                window.dispatchEvent(new CustomEvent(TOUR_RESTART_EVENT))
                handleMenuClose()
              }}
            >
              <ListItemIcon>
                <HelpOutline fontSize="small" />
              </ListItemIcon>
              <ListItemText>Page tour</ListItemText>
            </MenuItem>
          </Menu>
        </Box>
      </Toolbar>
    </AppBar>
  )
}

export default Header
