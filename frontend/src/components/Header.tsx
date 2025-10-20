import { AppBar, Toolbar, Box } from '@mui/material'
import React from 'react'

const Header: React.FC = () => {
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
          sx={{
            height: 40,
            width: 'auto',
            maxWidth: 200,
          }}
        />
      </Toolbar>
    </AppBar>
  )
}

export default Header
