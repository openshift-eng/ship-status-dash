import { useEffect } from 'react'

const OAuthCallback = () => {
  useEffect(() => {
    const redirectUrl = localStorage.getItem('oauth_redirect') || '/'
    localStorage.removeItem('oauth_redirect')
    window.location.href = redirectUrl
  }, [])

  return null
}

export default OAuthCallback
