import { ArrowBack } from '@mui/icons-material'
import { Button, Container, Paper, styled, Typography } from '@mui/material'
import { useNavigate, useParams } from 'react-router-dom'

import { getTeamColor } from '../../utils/teamColor'
import SubComponentList from '../sub-component/SubComponentList'

const StyledContainer = styled(Container)(({ theme }) => ({
  marginTop: theme.spacing(4),
  marginBottom: theme.spacing(4),
}))

const BackButton = styled(Button)(({ theme }) => ({
  marginBottom: theme.spacing(3),
}))

const TeamHeader = styled(Paper)<{ teamColor?: string }>(({ theme, teamColor }) => {
  const isDark = theme.palette.mode === 'dark'
  return {
    padding: theme.spacing(4),
    marginBottom: theme.spacing(4),
    borderRadius: theme.spacing(2),
    backgroundColor: teamColor
      ? `${teamColor}${isDark ? '25' : '15'}`
      : isDark
        ? theme.palette.grey[800]
        : theme.palette.grey[100],
    border: teamColor ? `2px solid ${teamColor}40` : undefined,
  }
})

const TeamTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '2rem',
  color: theme.palette.text.primary,
  [theme.breakpoints.down('md')]: {
    fontSize: '1.75rem',
  },
  [theme.breakpoints.down('sm')]: {
    fontSize: '1.5rem',
  },
}))

const TeamPage = () => {
  const navigate = useNavigate()
  const { team } = useParams<{ team: string }>()

  if (!team) return null

  const decodedTeam = decodeURIComponent(team)
  const teamColor = getTeamColor(decodedTeam)

  return (
    <StyledContainer maxWidth="lg">
      <BackButton variant="outlined" startIcon={<ArrowBack />} onClick={() => navigate('/')}>
        Main Dashboard
      </BackButton>

      <TeamHeader elevation={2} teamColor={teamColor}>
        <TeamTitle>{decodedTeam} Sub Components</TeamTitle>
      </TeamHeader>

      <SubComponentList filters={{ team: decodedTeam }} />
    </StyledContainer>
  )
}

export default TeamPage
