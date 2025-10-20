import React, { useState, useEffect } from 'react'
import { Box, Card, CardContent, Typography, styled } from '@mui/material'
import { StatusChip } from './StatusColors'
import { getStatusChipColor } from '../utils/helpers'
import { SubComponent } from '../types'
import { getSubComponentStatusEndpoint } from '../utils/endpoints'
import OutageModal from './OutageModal'

const SubComponentCard = styled(Card)<{ status: string }>(({ theme, status }) => {
  const color = getStatusChipColor(theme, status)
  
  // Create a faint version of the status color
  const getFaintColor = (statusColor: string) => {
    switch (statusColor) {
      case theme.palette.success.main:
        return theme.palette.success.light
      case theme.palette.error.main:
        return theme.palette.error.light
      case theme.palette.warning.main:
        return theme.palette.warning.light
      case theme.palette.info.main:
        return theme.palette.info.light
      default:
        return theme.palette.grey[100]
    }
  }

  return {
    border: `1px solid ${color}`,
    borderRadius: theme.spacing(1.5),
    cursor: 'pointer',
    transition: 'all 0.2s ease-in-out',
    backgroundColor: theme.palette.background.paper,
    minHeight: '120px',
    display: 'flex',
    flexDirection: 'column',
    '&:hover': {
      boxShadow: theme.shadows[4],
      transform: 'translateY(-1px)',
      borderColor: color,
      backgroundColor: getFaintColor(color),
      '& .MuiChip-root': {
        color: 'white',
        borderColor: 'white',
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

const StatusChipBox = styled(Box)(({ theme }) => ({
  flexShrink: 0,
}))

interface SubComponentCardProps {
  subComponent: SubComponent
  componentName: string
}

const SubComponentCardComponent: React.FC<SubComponentCardProps> = ({
  subComponent,
  componentName,
}) => {
  const [modalOpen, setModalOpen] = useState(false)
  const [subComponentWithStatus, setSubComponentWithStatus] = useState<SubComponent>(subComponent)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch(getSubComponentStatusEndpoint(componentName, subComponent.name))
      .then((res) => res.json())
      .catch(() => ({ status: 'Unknown', active_outages: [] }))
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
    setModalOpen(true)
  }

  const handleCloseModal = () => {
    setModalOpen(false)
  }

  return (
    <>
      <SubComponentCard status={subComponentWithStatus.status || 'Unknown'} onClick={handleClick}>
        <StyledCardContent>
          <CardHeader>
            <SubComponentTitle>
              {subComponent.name}
            </SubComponentTitle>
            <StatusChipBox>
              <StatusChip
                label={loading ? 'Loading...' : subComponentWithStatus.status || 'Unknown'}
                status={subComponentWithStatus.status || 'Unknown'}
                size="small"
                variant="outlined"
              />
            </StatusChipBox>
          </CardHeader>
          <SubComponentDescription>
            {subComponent.description}
          </SubComponentDescription>
        </StyledCardContent>
      </SubComponentCard>

      <OutageModal
        open={modalOpen}
        onClose={handleCloseModal}
        selectedSubComponent={subComponentWithStatus}
        componentName={componentName}
      />
    </>
  )
}

export default SubComponentCardComponent
