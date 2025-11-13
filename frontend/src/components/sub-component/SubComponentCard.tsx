import { Box, Card, CardContent, Typography, styled } from '@mui/material'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import type { SubComponent } from '../../types'
import { getSubComponentStatusEndpoint } from '../../utils/endpoints'
import { getStatusBackgroundColor, getStatusChipColor } from '../../utils/helpers'
import { slugify } from '../../utils/slugify'
import { StatusChip } from '../StatusColors'

const SubComponentCard = styled(Card)<{ status: string; useBackgroundColor?: boolean }>(({
  theme,
  status,
  useBackgroundColor = false,
}) => {
  const color = getStatusChipColor(theme, status)
  const backgroundColor = useBackgroundColor
    ? getStatusBackgroundColor(theme, status)
    : theme.palette.background.paper

  return {
    border: `2px solid ${color}`,
    borderRadius: theme.spacing(1.5),
    cursor: 'pointer',
    transition: 'all 0.2s ease-in-out',
    backgroundColor: backgroundColor,
    minHeight: '120px',
    display: 'flex',
    flexDirection: 'column',
    '&:hover': {
      boxShadow: theme.shadows[4],
      transform: 'translateY(-1px)',
      '& .MuiChip-root': {
        backgroundColor: 'white',
        color: color,
        borderColor: color,
      },
    },
  }
})

const StyledCardContent = styled(CardContent)(({ theme }) => ({
  padding: theme.spacing(2.5),
  flex: 1,
  display: 'flex',
  flexDirection: 'column',
  '&:last-child': {
    paddingBottom: theme.spacing(2.5),
  },
}))

const CardHeader = styled(Box)(({ theme }) => ({
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'flex-start',
  marginBottom: theme.spacing(1),
}))

const SubComponentTitle = styled(Typography)(({ theme }) => ({
  fontWeight: 600,
  fontSize: '1rem',
  color: theme.palette.text.primary,
  flex: 1,
  marginRight: theme.spacing(1),
}))

const SubComponentDescription = styled(Typography)(({ theme }) => ({
  fontSize: '0.875rem',
  color: theme.palette.text.secondary,
  lineHeight: 1.5,
  flex: 1,
}))

const StatusChipBox = styled(Box)(() => ({
  flexShrink: 0,
}))

interface SubComponentCardProps {
  subComponent: SubComponent
  componentName: string
  useBackgroundColor?: boolean
}

const SubComponentCardComponent = ({
  subComponent,
  componentName,
  useBackgroundColor = false,
}: SubComponentCardProps) => {
  const navigate = useNavigate()
  const [subComponentWithStatus, setSubComponentWithStatus] = useState<SubComponent>(subComponent)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch(getSubComponentStatusEndpoint(componentName, subComponent.name))
      .then((res) => res.json().catch(() => ({ status: 'Unknown', active_outages: [] })))
      .then((subStatus) => {
        setSubComponentWithStatus({
          ...subComponent,
          status: subStatus.status,
          active_outages: subStatus.active_outages,
        })
      })
      .finally(() => {
        setLoading(false)
      })
  }, [componentName, subComponent])

  const handleClick = () => {
    const status = subComponentWithStatus.status || 'Unknown'
    const activeOutages = subComponentWithStatus.active_outages || []
    const isHealthy = status === 'Healthy' || activeOutages.length === 0

    if (isHealthy || activeOutages.length > 1) {
      navigate(`/${slugify(componentName)}/${slugify(subComponentWithStatus.name)}`)
    } else if (activeOutages.length === 1) {
      navigate(
        `/${slugify(componentName)}/${slugify(subComponentWithStatus.name)}/outages/${activeOutages[0].id}`,
      )
    }
  }

  return (
    <SubComponentCard
      status={subComponentWithStatus.status || 'Unknown'}
      useBackgroundColor={useBackgroundColor}
      onClick={handleClick}
    >
      <StyledCardContent>
        <CardHeader>
          <SubComponentTitle>{subComponent.name}</SubComponentTitle>
          <StatusChipBox>
            <StatusChip
              label={loading ? 'Loading...' : subComponentWithStatus.status || 'Unknown'}
              status={subComponentWithStatus.status || 'Unknown'}
              size="small"
              variant="outlined"
            />
          </StatusChipBox>
        </CardHeader>
        <SubComponentDescription>{subComponent.description}</SubComponentDescription>
      </StyledCardContent>
    </SubComponentCard>
  )
}

export default SubComponentCardComponent
