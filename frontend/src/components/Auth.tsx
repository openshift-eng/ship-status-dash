import { Login, Person } from '@mui/icons-material'
import { Box, Button, styled, Typography } from '@mui/material'
import { useEffect, useState } from 'react'

import { getProtectedDomain, getUserEndpoint } from '../utils/endpoints'

const LoginButton = styled(Button)(({ theme }) => ({
  color: theme.palette.text.primary,
  borderColor: theme.palette.divider,
  textTransform: 'none',
  '&:hover': {
    borderColor: theme.palette.primary.main,
    backgroundColor: theme.palette.action.hover,
  },
}))

const UserDisplay = styled(Box)(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
  padding: theme.spacing(0.75, 1.5),
  borderRadius: theme.shape.borderRadius,
  backgroundColor:
    theme.palette.mode === 'dark' ? theme.palette.grey[800] : theme.palette.grey[100],
  border: `1px solid ${theme.palette.divider}`,
}))

const UserName = styled(Typography)(({ theme }) => ({
  fontSize: '0.875rem',
  color: theme.palette.text.primary,
  fontWeight: 500,
}))

const Auth = () => {
  const [user, setUser] = useState<{ user?: string } | null>(null)

  useEffect(() => {
    fetch(getUserEndpoint())
      .then((response) => {
        if (response.ok) {
          return response.json()
        }
        return null
      })
      .then((userData) => {
        if (userData) {
          setUser(userData)
        }
      })
      .catch(() => {
        setUser(null)
      })
  }, [])

  const handleLoginClick = () => {
    // we need to store the redirect url in local storage because the oauth proxy will redirect to the callback url after authentication
    localStorage.setItem('oauth_redirect', window.location.href)
    window.location.href = `${getProtectedDomain()}/oauth/start`
  }

  if (user) {
    return (
      <UserDisplay>
        <Person fontSize="small" sx={{ color: 'text.secondary' }} />
        <UserName>{user.user}</UserName>
      </UserDisplay>
    )
  }

  return (
    <LoginButton variant="outlined" startIcon={<Login />} onClick={handleLoginClick}>
      Login
    </LoginButton>
  )
}

export default Auth
