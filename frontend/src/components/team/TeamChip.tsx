import { Chip, styled } from '@mui/material'
import { lighten } from '@mui/material/styles'
import { Link } from 'react-router-dom'

import { getTeamColor } from '../../utils/teamColor'

const StyledTeamChip = styled(Chip)<{ size?: 'small' | 'medium'; teamColor: string }>(({
  theme,
  size,
  teamColor,
}) => {
  const isDark = theme.palette.mode === 'dark'
  const textColor = isDark ? lighten(teamColor, 0.4) : teamColor

  return {
    backgroundColor: `${teamColor}${isDark ? '25' : '15'}`,
    color: textColor,
    border: `1px solid ${teamColor}40`,
    transition: 'transform 0.2s ease, box-shadow 0.2s ease',
    cursor: 'pointer',
    ...(size === 'small' && {
      fontSize: '0.65rem',
      height: '20px',
      '& .MuiChip-label': {
        padding: '0 8px',
      },
    }),
    '&:hover': {
      transform: 'scale(1.1)',
      boxShadow: theme.shadows[4],
    },
  }
})

interface TeamChipProps {
  team: string
  size?: 'small' | 'medium'
}

const TeamChip = ({ team, size = 'medium' }: TeamChipProps) => (
  <Link
    to={`/team/${encodeURIComponent(team)}`}
    style={{ textDecoration: 'none' }}
    data-tour="component-team"
    onClick={(e) => e.stopPropagation()}
  >
    <StyledTeamChip label={team} size={size} teamColor={getTeamColor(team)} clickable />
  </Link>
)

export default TeamChip
