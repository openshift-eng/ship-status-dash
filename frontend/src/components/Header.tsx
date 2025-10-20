import { AppBar, Toolbar, Box } from '@mui/material'
import React from 'react'
import { useNavigate } from 'react-router-dom'

const Header: React.FC = () => {
  const navigate = useNavigate()

  const handleLogoClick = () => {
    navigate('/')
  }

  return (
    <AppBar
      position="sticky"
      sx={{
        backgroundColor: 'white',
        boxShadow: '0 2px 4px rgba(0,0,0,0.1)',
        zIndex: 1000,
      }}
    >
      <Toolbar>
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
      </Toolbar>
    </AppBar>
  )
}

export default Header
